package service

import (
	"context"
	"errors"
	"strconv"
	"time"
)

const (
	channelStatusObject        = "sub2api.channel_status"
	channelStatusSchemaVersion = 2
	channelStatusV2Range       = "90m"
	ChannelStatusScopeKey      = "key"
	ChannelStatusScopeVisible  = "visible"
	ChannelStatusStatusUnknown = "unknown"
)

type channelStatusSettings interface {
	GetChannelMonitorRuntime(ctx context.Context) ChannelMonitorRuntime
}

type channelStatusGroups interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
}

type channelStatusV1 interface {
	ListUserView(ctx context.Context) ([]*UserMonitorView, error)
}

type channelStatusV2 interface {
	ParseFilter(rangeValue string, platforms, models []string, groupIDs []int64) (ChannelMonitorV2Filter, error)
	Matrix(ctx context.Context, filter ChannelMonitorV2Filter, groupBy ChannelMonitorV2GroupBy, admin bool) (*ChannelMonitorV2Matrix, error)
}

// ChannelStatusQuery GET /v1/sub2api/channel-status 的查询条件。
type ChannelStatusQuery struct {
	UserID         int64
	KeyGroupID     *int64
	KeyGroupName   string
	IncludeVisible bool
}

// ChannelStatusItem 可见分组的一条渠道状态。仅 ?scope=visible 返回。
type ChannelStatusItem struct {
	GroupID   *int64 `json:"group_id,omitempty"`
	GroupName string `json:"group_name"`
	Connected bool   `json:"connected"`
	Status    string `json:"status"`
}

// ChannelStatusSnapshot GET /v1/sub2api/channel-status 成功响应。
// 顶层字段描述当前 API Key 绑定分组；无监控时 connected 为 null、status 为 unknown。
type ChannelStatusSnapshot struct {
	Object        string              `json:"object"`
	SchemaVersion int                 `json:"schema_version"`
	CheckedAt     time.Time           `json:"checked_at"`
	GroupID       *int64              `json:"group_id"`
	GroupName     string              `json:"group_name"`
	Connected     *bool               `json:"connected"`
	Status        string              `json:"status"`
	Items         []ChannelStatusItem `json:"items,omitempty"`
}

// ChannelStatusService 汇总当前 Key 绑定分组的只读渠道状态。
type ChannelStatusService struct {
	settings channelStatusSettings
	groups   channelStatusGroups
	v1       channelStatusV1
	v2       channelStatusV2
	now      func() time.Time
}

func NewChannelStatusService(
	settings *SettingService,
	apiKeyService *APIKeyService,
	v1 *ChannelMonitorService,
	v2 *ChannelMonitorV2Service,
) *ChannelStatusService {
	svc := &ChannelStatusService{now: func() time.Time { return time.Now().UTC() }}
	if settings != nil {
		svc.settings = settings
	}
	if apiKeyService != nil {
		svc.groups = apiKeyService
	}
	if v1 != nil {
		svc.v1 = v1
	}
	if v2 != nil {
		svc.v2 = v2
	}
	return svc
}

func (s *ChannelStatusService) Get(ctx context.Context, q ChannelStatusQuery) (*ChannelStatusSnapshot, error) {
	snap := emptyChannelStatusSnapshot(s.nowFn(), q.KeyGroupID, q.KeyGroupName)
	if s == nil || s.settings == nil {
		return snap, nil
	}
	runtime := s.settings.GetChannelMonitorRuntime(ctx)
	if !runtime.Enabled {
		return snap, nil
	}

	var (
		items []ChannelStatusItem
		err   error
	)
	switch runtime.Mode {
	case ChannelMonitorModeV2:
		items, err = s.v2Items(ctx, q)
	default:
		items, err = s.v1Items(ctx, q)
	}
	if err != nil {
		if errors.Is(err, ErrChannelMonitorDisabled) {
			return snap, nil
		}
		return nil, err
	}
	items = aggregateChannelStatusByGroup(items)
	applyChannelStatusKeyGroup(snap, items)
	if q.IncludeVisible {
		if items == nil {
			items = []ChannelStatusItem{}
		}
		snap.Items = items
	}
	return snap, nil
}

func (s *ChannelStatusService) nowFn() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *ChannelStatusService) availableGroups(ctx context.Context, userID int64) ([]Group, error) {
	if s.groups == nil {
		return nil, nil
	}
	return s.groups.GetAvailableGroups(ctx, userID)
}

func (s *ChannelStatusService) v1Items(ctx context.Context, q ChannelStatusQuery) ([]ChannelStatusItem, error) {
	if s.v1 == nil {
		return []ChannelStatusItem{}, nil
	}
	groups, err := s.availableGroups(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]Group, len(groups))
	for i := range groups {
		if groups[i].Name == "" {
			continue
		}
		byName[groups[i].Name] = groups[i]
	}
	views, err := s.v1.ListUserView(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ChannelStatusItem, 0, len(views))
	for _, view := range views {
		if view == nil || view.GroupName == "" {
			continue
		}
		groupID, ok := resolveChannelStatusGroupID(view.GroupName, byName, q.KeyGroupID, q.KeyGroupName)
		if !ok {
			continue
		}
		id := groupID
		if !q.IncludeVisible && !isChannelStatusKeyGroup(&id, view.GroupName, q.KeyGroupID, q.KeyGroupName) {
			continue
		}
		items = append(items, ChannelStatusItem{
			GroupID:   &id,
			GroupName: view.GroupName,
			Connected: channelStatusV1Connected(view.PrimaryStatus, view.PrimaryErrorCategory),
			Status:    view.PrimaryStatus,
		})
	}
	return items, nil
}

func (s *ChannelStatusService) v2Items(ctx context.Context, q ChannelStatusQuery) ([]ChannelStatusItem, error) {
	if s.v2 == nil {
		return []ChannelStatusItem{}, nil
	}
	groups, err := s.availableGroups(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(groups)+1)
	allowedIDs := make([]int64, 0, len(groups)+1)
	for i := range groups {
		allowed[groups[i].ID] = struct{}{}
		allowedIDs = append(allowedIDs, groups[i].ID)
	}
	if q.KeyGroupID != nil {
		if _, ok := allowed[*q.KeyGroupID]; !ok {
			allowed[*q.KeyGroupID] = struct{}{}
			allowedIDs = append(allowedIDs, *q.KeyGroupID)
		}
	}
	if !q.IncludeVisible {
		if q.KeyGroupID == nil {
			return []ChannelStatusItem{}, nil
		}
		allowedIDs = []int64{*q.KeyGroupID}
		allowed = map[int64]struct{}{*q.KeyGroupID: {}}
	}
	if len(allowedIDs) == 0 {
		return []ChannelStatusItem{}, nil
	}
	filter, err := s.v2.ParseFilter(channelStatusV2Range, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	filter.RestrictGroups = true
	filter.AllowedGroupIDs = allowedIDs
	matrix, err := s.v2.Matrix(ctx, filter, ChannelMonitorV2GroupByPlatformGroup, false)
	if err != nil {
		return nil, err
	}
	if matrix == nil {
		return []ChannelStatusItem{}, nil
	}
	items := make([]ChannelStatusItem, 0, len(matrix.Items))
	for i := range matrix.Items {
		row := matrix.Items[i]
		if row.GroupID == nil {
			continue
		}
		if _, ok := allowed[*row.GroupID]; !ok {
			continue
		}
		items = append(items, ChannelStatusItem{
			GroupID:   row.GroupID,
			GroupName: row.GroupName,
			Connected: channelStatusV2Connected(row.Health.Overall),
			Status:    row.Health.Overall,
		})
	}
	return items, nil
}

func emptyChannelStatusSnapshot(now time.Time, keyGroupID *int64, keyGroupName string) *ChannelStatusSnapshot {
	return &ChannelStatusSnapshot{
		Object:        channelStatusObject,
		SchemaVersion: channelStatusSchemaVersion,
		CheckedAt:     now,
		GroupID:       cloneInt64Ptr(keyGroupID),
		GroupName:     keyGroupName,
		Status:        ChannelStatusStatusUnknown,
	}
}

func applyChannelStatusKeyGroup(snap *ChannelStatusSnapshot, items []ChannelStatusItem) {
	if snap == nil {
		return
	}
	for i := range items {
		if !isChannelStatusKeyGroup(items[i].GroupID, items[i].GroupName, snap.GroupID, snap.GroupName) {
			continue
		}
		connected := items[i].Connected
		snap.Connected = &connected
		snap.Status = items[i].Status
		return
	}
}

func aggregateChannelStatusByGroup(items []ChannelStatusItem) []ChannelStatusItem {
	if len(items) == 0 {
		return items
	}
	type acc struct {
		item ChannelStatusItem
	}
	order := make([]string, 0, len(items))
	byKey := make(map[string]*acc, len(items))
	for i := range items {
		key := channelStatusItemKey(items[i])
		existing, ok := byKey[key]
		if !ok {
			cp := items[i]
			byKey[key] = &acc{item: cp}
			order = append(order, key)
			continue
		}
		if existing.item.Connected && !items[i].Connected {
			existing.item.Connected = false
			existing.item.Status = items[i].Status
		}
	}
	out := make([]ChannelStatusItem, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key].item)
	}
	return out
}

func channelStatusItemKey(item ChannelStatusItem) string {
	if item.GroupID != nil {
		return "id:" + strconv.FormatInt(*item.GroupID, 10)
	}
	return "name:" + item.GroupName
}

func resolveChannelStatusGroupID(groupName string, byName map[string]Group, keyGroupID *int64, keyGroupName string) (int64, bool) {
	if group, ok := byName[groupName]; ok {
		return group.ID, true
	}
	if keyGroupName != "" && groupName == keyGroupName && keyGroupID != nil {
		return *keyGroupID, true
	}
	return 0, false
}

func isChannelStatusKeyGroup(groupID *int64, groupName string, keyGroupID *int64, keyGroupName string) bool {
	if keyGroupID == nil && keyGroupName == "" {
		return false
	}
	if keyGroupID != nil && groupID != nil && *groupID == *keyGroupID {
		return true
	}
	return keyGroupName != "" && groupName == keyGroupName
}

func channelStatusV1Connected(status, errorCategory string) bool {
	return status == MonitorStatusOperational && errorCategory != MonitorErrorCategoryRateOrCapacity
}

func channelStatusV2Connected(overall string) bool {
	return overall == "healthy" || overall == "warning"
}
