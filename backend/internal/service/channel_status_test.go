package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubChannelStatusSettings struct {
	runtime ChannelMonitorRuntime
}

func (s stubChannelStatusSettings) GetChannelMonitorRuntime(context.Context) ChannelMonitorRuntime {
	return s.runtime
}

type stubChannelStatusGroups struct {
	groups []Group
	err    error
}

func (s stubChannelStatusGroups) GetAvailableGroups(context.Context, int64) ([]Group, error) {
	return s.groups, s.err
}

type stubChannelStatusV1 struct {
	views []*UserMonitorView
	err   error
}

func (s stubChannelStatusV1) ListUserView(context.Context) ([]*UserMonitorView, error) {
	return s.views, s.err
}

type stubChannelStatusV2 struct {
	matrix *ChannelMonitorV2Matrix
	err    error

	lastRange   string
	lastFilter  ChannelMonitorV2Filter
	lastGroupBy ChannelMonitorV2GroupBy
	lastAdmin   bool
}

func (s *stubChannelStatusV2) ParseFilter(rangeValue string, _ []string, _ []string, _ []int64) (ChannelMonitorV2Filter, error) {
	s.lastRange = rangeValue
	return ChannelMonitorV2Filter{}, nil
}

func (s *stubChannelStatusV2) Matrix(_ context.Context, filter ChannelMonitorV2Filter, groupBy ChannelMonitorV2GroupBy, admin bool) (*ChannelMonitorV2Matrix, error) {
	s.lastFilter = filter
	s.lastGroupBy = groupBy
	s.lastAdmin = admin
	return s.matrix, s.err
}

func newChannelStatusTestService() *ChannelStatusService {
	fixed := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	return &ChannelStatusService{now: func() time.Time { return fixed }}
}

func TestChannelStatusServiceDisabledReturnsUnknown(t *testing.T) {
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: false, Mode: ChannelMonitorModeV1}}
	id := int64(12)
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "plus"})
	require.NoError(t, err)
	require.Equal(t, 2, snap.SchemaVersion)
	require.Nil(t, snap.Connected)
	require.Equal(t, ChannelStatusStatusUnknown, snap.Status)
	require.Empty(t, snap.Items)
	require.Equal(t, id, *snap.GroupID)
	require.Equal(t, "plus", snap.GroupName)
	requireChannelStatusJSONOmitsItems(t, snap)
}

func TestChannelStatusServiceDefaultReturnsOnlyKeyGroup(t *testing.T) {
	plusID := int64(12)
	proID := int64(13)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{
		{ID: plusID, Name: "plus"},
		{ID: proID, Name: "pro"},
	}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{
		{Name: "Plus probe", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
		{Name: "Pro probe", GroupName: "pro", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusFailed, PrimaryErrorCategory: MonitorErrorCategoryRateOrCapacity},
		{Name: "Hidden", GroupName: "admin-only", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
		{Name: "No group", GroupName: "", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
	}}

	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 7, KeyGroupID: &plusID, KeyGroupName: "plus"})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.True(t, *snap.Connected)
	require.Equal(t, MonitorStatusOperational, snap.Status)
	require.Equal(t, plusID, *snap.GroupID)
	require.Empty(t, snap.Items)
	requireChannelStatusJSONOmitsItems(t, snap)
	requireChannelStatusJSONRedacted(t, snap)
}

func TestChannelStatusServiceVisibleIncludesOtherGroups(t *testing.T) {
	plusID := int64(12)
	proID := int64(13)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{
		{ID: plusID, Name: "plus"},
		{ID: proID, Name: "pro"},
	}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{
		{Name: "Plus probe", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
		{Name: "Pro probe", GroupName: "pro", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusFailed},
		{Name: "Hidden", GroupName: "admin-only", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
	}}

	snap, err := svc.Get(context.Background(), ChannelStatusQuery{
		UserID: 7, KeyGroupID: &plusID, KeyGroupName: "plus", IncludeVisible: true,
	})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.True(t, *snap.Connected)
	require.Equal(t, MonitorStatusOperational, snap.Status)
	require.Len(t, snap.Items, 2)
	require.Equal(t, "plus", snap.Items[0].GroupName)
	require.True(t, snap.Items[0].Connected)
	require.Equal(t, "pro", snap.Items[1].GroupName)
	require.False(t, snap.Items[1].Connected)
	for _, item := range snap.Items {
		require.NotEqual(t, "admin-only", item.GroupName)
	}
}

func TestChannelStatusServiceV1RateOrCapacityNotConnected(t *testing.T) {
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: 1, Name: "plus"}}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{{
		Name: "Plus", GroupName: "plus", Provider: MonitorProviderOpenAI,
		PrimaryStatus: MonitorStatusOperational, PrimaryErrorCategory: MonitorErrorCategoryRateOrCapacity,
	}}}
	id := int64(1)
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "plus"})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.False(t, *snap.Connected)
	require.Equal(t, MonitorStatusOperational, snap.Status)
}

func TestChannelStatusServiceV2MatrixAndWarningConnected(t *testing.T) {
	plusID := int64(12)
	proID := int64(13)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV2}}
	svc.groups = stubChannelStatusGroups{groups: []Group{
		{ID: plusID, Name: "plus"},
		{ID: proID, Name: "pro"},
	}}
	v2 := &stubChannelStatusV2{matrix: &ChannelMonitorV2Matrix{Items: []ChannelMonitorV2MatrixRow{
		{Platform: MonitorProviderOpenAI, GroupID: &plusID, GroupName: "plus", Health: ChannelMonitorV2Health{Overall: "warning"}},
		{Platform: MonitorProviderOpenAI, GroupID: &proID, GroupName: "pro", Health: ChannelMonitorV2Health{Overall: "critical"}},
		{Platform: MonitorProviderOpenAI, GroupName: "unknown", Health: ChannelMonitorV2Health{Overall: "unknown"}},
	}}}
	svc.v2 = v2

	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 7, KeyGroupID: &plusID, KeyGroupName: "plus"})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.True(t, *snap.Connected)
	require.Equal(t, "warning", snap.Status)
	require.Empty(t, snap.Items)
	require.Equal(t, channelStatusV2Range, v2.lastRange)
	require.True(t, v2.lastFilter.RestrictGroups)
	require.Equal(t, []int64{plusID}, v2.lastFilter.AllowedGroupIDs)
	require.Equal(t, ChannelMonitorV2GroupByPlatformGroup, v2.lastGroupBy)
	require.False(t, v2.lastAdmin)
	requireChannelStatusJSONRedacted(t, snap)
}

func TestChannelStatusServiceUnboundKeyStaysUnknown(t *testing.T) {
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: 12, Name: "plus"}}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{{
		Name: "Plus", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational,
	}}}
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1})
	require.NoError(t, err)
	require.Nil(t, snap.GroupID)
	require.Empty(t, snap.GroupName)
	require.Nil(t, snap.Connected)
	require.Equal(t, ChannelStatusStatusUnknown, snap.Status)
	require.Empty(t, snap.Items)
}

func TestChannelStatusServiceMissingKeyGroupMonitorLeavesUnknown(t *testing.T) {
	id := int64(99)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: 12, Name: "plus"}}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{{
		Name: "Plus", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational,
	}}}
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "codex"})
	require.NoError(t, err)
	require.Equal(t, id, *snap.GroupID)
	require.Equal(t, "codex", snap.GroupName)
	require.Nil(t, snap.Connected)
	require.Equal(t, ChannelStatusStatusUnknown, snap.Status)
}

func TestChannelStatusServiceKeyGroupOutsideAvailableStillResolved(t *testing.T) {
	id := int64(40)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: 12, Name: "plus"}}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{{
		Name: "Luna", GroupName: "luna", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusDegraded,
	}}}
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "luna"})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.False(t, *snap.Connected)
	require.Equal(t, MonitorStatusDegraded, snap.Status)
}

func TestChannelStatusServiceV1MultipleMonitorsSameKeyGroup(t *testing.T) {
	plusID := int64(12)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: plusID, Name: "plus"}}}
	svc.v1 = stubChannelStatusV1{views: []*UserMonitorView{
		{Name: "Plus a", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusOperational},
		{Name: "Plus b", GroupName: "plus", Provider: MonitorProviderOpenAI, PrimaryStatus: MonitorStatusFailed},
	}}
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{
		UserID: 1, KeyGroupID: &plusID, KeyGroupName: "plus", IncludeVisible: true,
	})
	require.NoError(t, err)
	require.NotNil(t, snap.Connected)
	require.False(t, *snap.Connected)
	require.Equal(t, MonitorStatusFailed, snap.Status)
	require.Len(t, snap.Items, 1)
	require.False(t, snap.Items[0].Connected)
	require.Equal(t, MonitorStatusFailed, snap.Items[0].Status)
}

func TestChannelStatusServiceV2DisabledConfigReturnsUnknown(t *testing.T) {
	id := int64(12)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV2}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: id, Name: "plus"}}}
	svc.v2 = &stubChannelStatusV2{err: ErrChannelMonitorDisabled}
	snap, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "plus"})
	require.NoError(t, err)
	require.Nil(t, snap.Connected)
	require.Equal(t, ChannelStatusStatusUnknown, snap.Status)
	require.Empty(t, snap.Items)
	require.Equal(t, id, *snap.GroupID)
}

func TestChannelStatusServiceV2MatrixErrorPropagates(t *testing.T) {
	id := int64(1)
	svc := newChannelStatusTestService()
	svc.settings = stubChannelStatusSettings{runtime: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV2}}
	svc.groups = stubChannelStatusGroups{groups: []Group{{ID: 1, Name: "plus"}}}
	svc.v2 = &stubChannelStatusV2{err: errors.New("db down")}
	_, err := svc.Get(context.Background(), ChannelStatusQuery{UserID: 1, KeyGroupID: &id, KeyGroupName: "plus"})
	require.EqualError(t, err, "db down")
}

func requireChannelStatusJSONOmitsItems(t *testing.T, snap *ChannelStatusSnapshot) {
	t.Helper()
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"items"`)
	require.Contains(t, string(raw), `"schema_version":2`)
}

func requireChannelStatusJSONRedacted(t *testing.T, snap *ChannelStatusSnapshot) {
	t.Helper()
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	var payload any
	require.NoError(t, json.Unmarshal(raw, &payload))
	forbidden := map[string]struct{}{
		"quota": {}, "endpoint": {}, "api_key": {},
		"request_count": {}, "rpm": {}, "buckets": {}, "metrics": {},
	}
	var hits []string
	walkJSONKeys(payload, func(key string) {
		if _, ok := forbidden[strings.ToLower(key)]; ok {
			hits = append(hits, key)
		}
	})
	require.Empty(t, hits, "channel status JSON leaked forbidden keys: %s", strings.Join(hits, ","))
}

func walkJSONKeys(value any, fn func(string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			fn(key)
			walkJSONKeys(child, fn)
		}
	case []any:
		for _, child := range typed {
			walkJSONKeys(child, fn)
		}
	}
}
