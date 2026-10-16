//go:build integration

package repository

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type monitorDisplayTestEncryptor struct{}

func (monitorDisplayTestEncryptor) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (monitorDisplayTestEncryptor) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "enc:"), nil
}

func TestChannelMonitorLinkedGroupLifecycle(t *testing.T) {
	ctx := context.Background()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "monitor-linked"})
	other := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "monitor-other"})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1,$2)", group.ID, other.ID)
	})
	repo := NewChannelMonitorRepository(integrationEntClient, integrationDB)
	svc := service.NewChannelMonitorService(repo, monitorDisplayTestEncryptor{})
	svc.SetGroupReader(NewGroupRepository(integrationEntClient, integrationDB))
	params := service.ChannelMonitorCreateParams{Name: "linked", Provider: service.MonitorProviderOpenAI,
		APIMode: service.MonitorAPIModeResponses, Endpoint: "https://93.184.216.34", APIKey: "test",
		PrimaryModel: "gpt-test", GroupName: "display only", GroupID: &group.ID, IntervalSeconds: 60}
	manualZero, cache, tps, availability, latency := 0.0, .625, 42.5, 98.5, 850
	params.DisplayOverrides = domain.ChannelMonitorDisplayOverrides{VisibleRateMultiplier: &manualZero, CacheRate24h: &cache, OutputTPS5m: &tps, PrimaryLatencyMs: &latency, Availability7d: &availability, Availability15d: &availability, Availability30d: &availability}
	created, err := svc.Create(ctx, params)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(ctx, created.ID) })
	loaded, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, &group.ID, loaded.GroupID)
	require.Equal(t, params.DisplayOverrides, loaded.DisplayOverrides)
	copy, err := svc.Duplicate(ctx, created.ID, 0, "test", "copy-linked")
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(ctx, copy.ID) })
	require.Equal(t, &group.ID, copy.GroupID)
	require.Equal(t, params.DisplayOverrides, copy.DisplayOverrides)
	require.Equal(t, "display only", copy.GroupName)
	require.False(t, copy.Enabled)
	_, err = svc.Update(ctx, created.ID, service.ChannelMonitorUpdateParams{GroupID: &other.ID})
	require.NoError(t, err)
	zero := int64(0)
	_, err = svc.Update(ctx, created.ID, service.ChannelMonitorUpdateParams{GroupID: &zero})
	require.NoError(t, err)
	loaded, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.GroupID)
	require.Equal(t, "display only", loaded.GroupName)
	require.Equal(t, params.DisplayOverrides, loaded.DisplayOverrides, "omitted overrides must be preserved")
	automatic := domain.ChannelMonitorDisplayOverrides{}
	_, err = svc.Update(ctx, created.ID, service.ChannelMonitorUpdateParams{DisplayOverrides: &automatic})
	require.NoError(t, err)
	loaded, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, automatic, loaded.DisplayOverrides)
	loaded, err = repo.GetByID(ctx, copy.ID)
	require.NoError(t, err)
	require.Equal(t, &group.ID, loaded.GroupID, "copy must be independent")
	require.Equal(t, params.DisplayOverrides, loaded.DisplayOverrides, "resetting the original must not reset its copy")
	// 删除统计分组不删除监控，FK 自动解绑。
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id=$1", group.ID)
	require.NoError(t, err)
	loaded, err = repo.GetByID(ctx, copy.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.GroupID)
	invalid := int64(-1)
	params.GroupID = &invalid
	_, err = svc.Create(ctx, params)
	require.ErrorIs(t, err, service.ErrGroupNotFound)
}

func TestChannelMonitorDisplayMetricsAggregation(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	users := []*service.User{mustCreateUser(t, client, &service.User{}), mustCreateUser(t, client, &service.User{})}
	keys := []*service.APIKey{mustCreateApiKey(t, client, &service.APIKey{UserID: users[0].ID}), mustCreateApiKey(t, client, &service.APIKey{UserID: users[1].ID})}
	account := mustCreateAccount(t, client, &service.Account{Name: "display-metrics"})
	group := mustCreateGroup(t, client, &service.Group{Name: "display-metrics"})
	other := mustCreateGroup(t, client, &service.Group{Name: "display-other"})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id IN ($1,$2)", users[0].ID, users[1].ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE id IN ($1,$2)", keys[0].ID, keys[1].ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1,$2)", group.ID, other.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1,$2)", users[0].ID, users[1].ID)
	})
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	insert := func(user int, groupID int64, model string, requested any, at time.Time, input, create, read, output int, duration any, kind service.RequestType, images, imageTokens int, mode string) {
		t.Helper()
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs
  (user_id,api_key_id,account_id,group_id,request_id,model,requested_model,created_at,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,duration_ms,request_type,image_count,image_output_tokens,billing_mode,image_size)
  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,CASE WHEN $15>0 OR $16>0 OR $17::varchar='image' THEN '1K' ELSE NULL END)`, users[user].ID, keys[user].ID, account.ID, groupID, fmt.Sprintf("display-%d", time.Now().UnixNano()), model, requested, at, input, create, read, output, duration, kind, images, imageTokens, mode)
		require.NoError(t, err)
	}
	// TPS 为 (10+30+20+40)/4=25；两个用户均计入。requested_model 优先，空白时回退 model。
	insert(0, group.ID, "upstream", " gpt-test ", now.Add(-time.Minute), 100, 50, 50, 10, 1000, service.RequestTypeSync, 0, 0, "token")
	insert(1, group.ID, " gpt-test ", "  ", now.Add(-2*time.Minute), 0, 0, 200, 60, 2000, service.RequestTypeStream, 0, 0, "token")
	insert(0, group.ID, "gpt-test", nil, now.Add(-5*time.Minute), 0, 0, 0, 20, 1000, service.RequestTypeWSV2, 0, 0, "token")
	insert(0, group.ID, "gpt-test", nil, now.Add(-3*time.Minute), 0, 0, 0, 40, 1000, service.RequestTypeCyberBlocked, 0, 0, "token")
	// 24h 下界计入缓存，5m 下界之前不计 TPS。缓存率为 350/500=0.7。
	insert(0, group.ID, "gpt-test", nil, now.Add(-24*time.Hour), 0, 0, 100, 9000, 1000, service.RequestTypeSync, 0, 0, "token")
	insert(0, group.ID, "gpt-test", nil, now.Add(-5*time.Minute-time.Microsecond), 0, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 0, "token")
	for _, at := range []time.Time{now.Add(-24*time.Hour - time.Microsecond), now, now.Add(time.Second)} {
		insert(0, group.ID, "gpt-test", nil, at, 9000, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 0, "token")
	}
	insert(0, other.ID, "gpt-test", nil, now.Add(-time.Minute), 9000, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 0, "token")
	insert(0, group.ID, "gpt-test", "extra-model", now.Add(-time.Minute), 9000, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 0, "token")
	// 运维有效文本样本过滤：无耗时、非正耗时/输出、实时与图像均排除 TPS。
	for _, duration := range []any{nil, 0, -1} {
		insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, 9000, duration, service.RequestTypeSync, 0, 0, "token")
	}
	for _, output := range []int{0, -1} {
		insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, output, 1000, service.RequestTypeSync, 0, 0, "token")
	}
	for _, kind := range []service.RequestType{service.RequestTypeUnknown, service.RequestTypeLive} {
		insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, 9000, 1000, kind, 0, 0, "token")
	}
	insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, 9000, 1000, service.RequestTypeSync, 1, 0, "token")
	insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 1, "token")
	insert(0, group.ID, "gpt-test", nil, now.Add(-time.Minute), 0, 0, 0, 9000, 1000, service.RequestTypeSync, 0, 0, "image")
	insert(0, group.ID, "zero-cache", nil, now.Add(-time.Hour), 100, 0, 0, 0, 0, service.RequestTypeSync, 0, 0, "token")
	repo := NewChannelMonitorDisplayRepository(integrationDB)
	primary := service.ChannelMonitorMetricKey{GroupID: group.ID, Model: "gpt-test"}
	empty := service.ChannelMonitorMetricKey{GroupID: group.ID, Model: "no-samples"}
	zero := service.ChannelMonitorMetricKey{GroupID: group.ID, Model: "zero-cache"}
	metrics, err := repo.BatchMetrics(ctx, []service.ChannelMonitorMetricKey{primary, empty, zero}, now)
	require.NoError(t, err)
	require.InDelta(t, 0.7, *metrics[primary].CacheRate24h, 1e-9)
	require.InDelta(t, 25.0, *metrics[primary].OutputTPS5m, 1e-9)
	require.NotNil(t, metrics[empty].CacheRate24h)
	require.Zero(t, *metrics[empty].CacheRate24h)
	require.Nil(t, metrics[empty].OutputTPS5m)
	require.Equal(t, 0.0, *metrics[zero].CacheRate24h)
	require.Nil(t, metrics[zero].OutputTPS5m)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = repo.BatchMetrics(canceled, []service.ChannelMonitorMetricKey{primary}, now)
	require.ErrorIs(t, err, context.Canceled)
}

// 主动探测持续更新 last_checked_at；用户列表仍必须按配置 ID 固定排列。
func TestChannelMonitorEnabledOrderSurvivesProbeUpdates(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorRepository(integrationEntClient, integrationDB)
	ids := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		monitor := &service.ChannelMonitor{Name: fmt.Sprintf("stable-order-%d", i), Provider: service.MonitorProviderOpenAI,
			APIMode: service.MonitorAPIModeResponses, Endpoint: "https://example.test", APIKey: "enc:test",
			PrimaryModel: "gpt-test", Enabled: true, IntervalSeconds: 60}
		require.NoError(t, repo.Create(ctx, monitor))
		ids = append(ids, monitor.ID)
		t.Cleanup(func() { _ = repo.Delete(ctx, monitor.ID) })
	}
	assertOrder := func() {
		t.Helper()
		monitors, err := repo.ListEnabled(ctx)
		require.NoError(t, err)
		found := make([]int64, 0, len(ids))
		for _, monitor := range monitors {
			if slices.Contains(ids, monitor.ID) {
				found = append(found, monitor.ID)
			}
		}
		require.Equal(t, ids, found)
	}
	assertOrder()
	for _, id := range []int64{ids[1], ids[0], ids[2], ids[0]} {
		require.NoError(t, repo.MarkChecked(ctx, id, time.Now()))
		assertOrder()
	}
}
