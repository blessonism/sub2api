package service

import (
	"context"
	"errors"
	"time"
)

const (
	channelStatusObject        = "sub2api.channel_status"
	channelStatusSchemaVersion = 1
	channelStatusModeOff       = "off"
	channelStatusV2Range       = "90m"
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

// ChannelStatusItem 脚本可见的单条渠道状态。
type ChannelStatusItem struct {
	GroupID       *int64 `json:"group_id,omitempty"`
	GroupName     string `json:"group_name"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Status        string `json:"status"`
	Connected     bool   `json:"connected"`
	IsKeyGroup    bool   `json:"is_key_group"`
	ErrorCategory string `json:"error_category,omitempty"`
}

// ChannelStatusSnapshot GET /v1/sub2api/channel-status 成功响应。
type ChannelStatusSnapshot struct {
	Object            string              `json:"object"`
	SchemaVersion     int                 `json:"schema_version"`
	Mode              string              `json:"mode"`
	Connected         bool                `json:"connected"`
	KeyGroupID        *int64              `json:"key_group_id,omitempty"`
	KeyGroupName      string              `json:"key_group_name,omitempty"`
	KeyGroupConnected *bool               `json:"key_group_connected"`
	KeyGroupStatus    string              `json:"key_group_status,omitempty"`
	CheckedAt         time.Time           `json:"checked_at"`
	ItemCount         int                 `json:"item_count"`
	ConnectedCount    int                 `json:"connected_count"`
	Items             []ChannelStatusItem `json:"items"`
}

// ChannelStatusService 汇总用户可见分组的只读渠道状态。
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

func (s *ChannelStatusService) Get(ctx context.Context, userID int64, keyGroupID *int64, keyGroupName string) (*ChannelStatusSnapshot, error) {
	snap := emptyChannelStatusSnapshot(s.nowFn(), keyGroupID, keyGroupName)
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
		snap.Mode = ChannelMonitorModeV2
		items, err = s.v2Items(ctx, userID)
	default:
		snap.Mode = ChannelMonitorModeV1
		items, err = s.v1Items(ctx, userID)
	}
	if err != nil {
		if errors.Is(err, ErrChannelMonitorDisabled) {
			return snap, nil
		}
		return nil, err
	}
	if items == nil {
		items = []ChannelStatusItem{}
	}
	markChannelStatusKeyGroup(items, keyGroupID, keyGroupName)
	fillChannelStatusCounts(snap, items)
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

func (s *ChannelStatusService) v1Items(ctx context.Context, userID int64) ([]ChannelStatusItem, error) {
	if s.v1 == nil {
		return []ChannelStatusItem{}, nil
	}
	groups, err := s.availableGroups(ctx, userID)
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
	if len(byName) == 0 {
		return []ChannelStatusItem{}, nil
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
		group, ok := byName[view.GroupName]
		if !ok {
			continue
		}
		groupID := group.ID
		items = append(items, ChannelStatusItem{
			GroupID:       &groupID,
			GroupName:     view.GroupName,
			Name:          view.Name,
			Provider:      view.Provider,
			Status:        view.PrimaryStatus,
			Connected:     channelStatusV1Connected(view.PrimaryStatus, view.PrimaryErrorCategory),
			ErrorCategory: view.PrimaryErrorCategory,
		})
	}
	return items, nil
}

func (s *ChannelStatusService) v2Items(ctx context.Context, userID int64) ([]ChannelStatusItem, error) {
	if s.v2 == nil {
		return []ChannelStatusItem{}, nil
	}
	groups, err := s.availableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(groups))
	allowedIDs := make([]int64, 0, len(groups))
	for i := range groups {
		allowed[groups[i].ID] = struct{}{}
		allowedIDs = append(allowedIDs, groups[i].ID)
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
		name := row.GroupName
		if name == "" {
			name = row.Platform
		}
		items = append(items, ChannelStatusItem{
			GroupID:   row.GroupID,
			GroupName: row.GroupName,
			Name:      name,
			Provider:  row.Platform,
			Status:    row.Health.Overall,
			Connected: channelStatusV2Connected(row.Health.Overall),
		})
	}
	return items, nil
}

func emptyChannelStatusSnapshot(now time.Time, keyGroupID *int64, keyGroupName string) *ChannelStatusSnapshot {
	return &ChannelStatusSnapshot{
		Object:        channelStatusObject,
		SchemaVersion: channelStatusSchemaVersion,
		Mode:          channelStatusModeOff,
		CheckedAt:     now,
		KeyGroupID:    cloneInt64Ptr(keyGroupID),
		KeyGroupName:  keyGroupName,
		Items:         []ChannelStatusItem{},
	}
}

func fillChannelStatusCounts(snap *ChannelStatusSnapshot, items []ChannelStatusItem) {
	connectedCount := 0
	keyItems := make([]ChannelStatusItem, 0, 2)
	for i := range items {
		if items[i].Connected {
			connectedCount++
		}
		if items[i].IsKeyGroup {
			keyItems = append(keyItems, items[i])
		}
	}
	snap.Items = items
	snap.ItemCount = len(items)
	snap.ConnectedCount = connectedCount
	snap.Connected = snap.ItemCount > 0 && snap.ConnectedCount == snap.ItemCount
	if len(keyItems) == 0 {
		return
	}
	allUp := true
	status := keyItems[0].Status
	for i := range keyItems {
		if keyItems[i].Connected {
			continue
		}
		allUp = false
		status = keyItems[i].Status
		break
	}
	snap.KeyGroupConnected = &allUp
	snap.KeyGroupStatus = status
}

func markChannelStatusKeyGroup(items []ChannelStatusItem, keyGroupID *int64, keyGroupName string) {
	if keyGroupID == nil && keyGroupName == "" {
		return
	}
	for i := range items {
		if keyGroupID != nil && items[i].GroupID != nil && *items[i].GroupID == *keyGroupID {
			items[i].IsKeyGroup = true
			continue
		}
		if keyGroupName != "" && items[i].GroupName == keyGroupName {
			items[i].IsKeyGroup = true
		}
	}
}

func channelStatusV1Connected(status, errorCategory string) bool {
	return status == MonitorStatusOperational && errorCategory != MonitorErrorCategoryRateOrCapacity
}

func channelStatusV2Connected(overall string) bool {
	return overall == "healthy" || overall == "warning"
}
