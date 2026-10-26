//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelGroupRoutesPersistenceAuthCacheAndUsage(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	groups := newGroupRepositoryWithSQL(client, tx)
	suffix := time.Now().UnixNano()
	target := &service.Group{Name: fmt.Sprintf("route-target-%d", suffix), Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3}
	require.NoError(t, groups.Create(ctx, target))
	source := &service.Group{Name: fmt.Sprintf("route-source-%d", suffix), Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.2,
		ModelGroupRoutes: []service.ModelGroupRoute{{Model: "gpt-6-luna", TargetGroupID: target.ID, Mode: "target_only", Enabled: true}}}
	require.NoError(t, groups.Create(ctx, source))
	loaded, err := groups.GetByIDLite(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, source.ModelGroupRoutes, loaded.ModelGroupRoutes)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("route-%d@example.com", suffix), Concurrency: 5})
	keys := NewAPIKeyRepository(client, nil)
	key := &service.APIKey{UserID: user.ID, GroupID: &source.ID, Key: fmt.Sprintf("sk-route-%d", suffix), Name: "route", Status: service.StatusActive}
	require.NoError(t, keys.Create(ctx, key))
	projected, err := keys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, source.ModelGroupRoutes, projected.Group.ModelGroupRoutes)
	cache := NewAPIKeyCache(testRedis(t))
	auth := service.NewAPIKeyService(keys, nil, groups, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
	t.Cleanup(auth.StopAuthCacheInvalidationSubscriber)
	first, err := auth.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, source.ModelGroupRoutes, first.Group.ModelGroupRoutes)
	loaded.ModelGroupRoutes[0].Mode = "prefer_local"
	require.NoError(t, groups.Update(ctx, loaded))
	auth.InvalidateAuthCacheByGroupID(ctx, source.ID)
	refreshed, err := auth.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, "prefer_local", refreshed.Group.ModelGroupRoutes[0].Mode)
	copy := *loaded
	copy.ID = 0
	copy.Name += "-copy"
	require.NoError(t, groups.CreateFromSource(ctx, &copy, source.ID))
	copied, err := groups.GetByIDLite(ctx, copy.ID)
	require.NoError(t, err)
	require.Equal(t, loaded.ModelGroupRoutes, copied.ModelGroupRoutes)
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("route-account-%d", suffix)})
	logs := newUsageLogRepositoryWithSQL(client, tx)
	entry := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: fmt.Sprintf("route-usage-%d", suffix), Model: "gpt-6-luna", GroupID: &target.ID, SourceGroupID: &source.ID, RateMultiplier: 0.3}
	_, err = logs.Create(ctx, entry)
	require.NoError(t, err)
	stored, err := logs.GetByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, entry.SourceGroupID, stored.SourceGroupID)
	require.Equal(t, entry.GroupID, stored.GroupID)
}

func TestModelGroupRoutesMigrationDefaultsAndSQLInvalidation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	suffix := time.Now().UnixNano()
	rows, err := tx.QueryContext(ctx, "INSERT INTO groups (name,platform) VALUES ($1,'openai') RETURNING id,model_group_routes", fmt.Sprintf("route-default-%d", suffix))
	require.NoError(t, err)
	require.True(t, rows.Next())
	var id int64
	var data []byte
	require.NoError(t, rows.Scan(&id, &data))
	require.NoError(t, rows.Close())
	var routes []service.ModelGroupRoute
	require.NoError(t, json.Unmarshal(data, &routes))
	require.Empty(t, routes)
	require.NotNil(t, routes)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("route-trigger-%d@example.com", suffix)})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &id, Key: fmt.Sprintf("sk-route-trigger-%d", suffix), Name: "route-trigger"})
	count := func() int {
		rows, err := tx.QueryContext(ctx, "SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')", key.Key)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var result int
		require.NoError(t, rows.Scan(&result))
		return result
	}
	before := count()
	_, err = tx.ExecContext(ctx, "UPDATE groups SET model_group_routes=model_group_routes WHERE id=$1", id)
	require.NoError(t, err)
	require.Equal(t, before, count())
	_, err = tx.ExecContext(ctx, `UPDATE groups SET model_group_routes='[{"model":"gpt-6-luna","target_group_id":40,"mode":"target_only","enabled":false}]'::jsonb WHERE id=$1`, id)
	require.NoError(t, err)
	require.Equal(t, before+1, count())
}
