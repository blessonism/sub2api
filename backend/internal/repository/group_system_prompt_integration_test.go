//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupSystemPromptPersistenceProjectionAndCache(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newGroupRepositoryWithSQL(client, tx)
	suffix := time.Now().UnixNano()
	group := &service.Group{Name: fmt.Sprintf("system-prompt-%d", suffix), Platform: service.PlatformOpenAI, RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
	require.NoError(t, repo.Create(ctx, group))
	loaded, err := repo.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, service.GroupSystemPromptInjection{Mode: "append"}, loaded.SystemPromptInjection)

	want := service.GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "  自定义系统提示词\n"}
	loaded.SystemPromptInjection = want
	require.NoError(t, repo.Update(ctx, loaded))
	loaded, err = repo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, want, loaded.SystemPromptInjection)

	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("system-prompt-%d@example.com", suffix), Concurrency: 5})
	apiKeyRepo := NewAPIKeyRepository(client, nil)
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: fmt.Sprintf("sk-system-prompt-%d", suffix), Name: "prompt", Status: service.StatusActive}
	require.NoError(t, apiKeyRepo.Create(ctx, key))
	authKey, err := apiKeyRepo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, authKey.Group)
	require.Equal(t, want, authKey.Group.SystemPromptInjection, "认证查询的显式投影不得漏列")

	cache := NewAPIKeyCache(testRedis(t))
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, repo, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
	t.Cleanup(apiKeyService.StopAuthCacheInvalidationSubscriber)
	first, err := apiKeyService.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, want, first.Group.SystemPromptInjection)

	// 故意在写库后延迟失效，证明第二次认证确实使用 JSON 缓存快照。
	disabled := want
	disabled.Enabled = false
	loaded.SystemPromptInjection = disabled
	require.NoError(t, repo.Update(ctx, loaded))
	cached, err := apiKeyService.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, want, cached.Group.SystemPromptInjection)
	apiKeyService.InvalidateAuthCacheByGroupID(ctx, group.ID)
	refreshed, err := apiKeyService.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, disabled, refreshed.Group.SystemPromptInjection, "失效后新请求必须取得关闭后的正文和模式")

	copy := *loaded
	copy.ID = 0
	copy.Name += "-copy"
	require.NoError(t, repo.CreateFromSource(ctx, &copy, group.ID))
	copied, err := repo.GetByIDLite(ctx, copy.ID)
	require.NoError(t, err)
	require.Equal(t, disabled, copied.SystemPromptInjection)
}

func TestGroupSystemPromptMigrationDefaultAndDurableInvalidation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	suffix := time.Now().UnixNano()
	// 原生 INSERT 不依赖 Ent 默认值，确认迁移给旧分组和数据库写入者的默认契约。
	rows, err := tx.QueryContext(ctx, "INSERT INTO groups (name, platform) VALUES ($1, 'openai') RETURNING id, system_prompt_injection", fmt.Sprintf("system-prompt-default-%d", suffix))
	require.NoError(t, err)
	require.True(t, rows.Next())
	var groupID int64
	var payload []byte
	require.NoError(t, rows.Scan(&groupID, &payload))
	require.NoError(t, rows.Close())
	var defaults service.GroupSystemPromptInjection
	require.NoError(t, json.Unmarshal(payload, &defaults))
	require.Equal(t, service.GroupSystemPromptInjection{Mode: "append"}, defaults)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("system-prompt-trigger-%d@example.com", suffix)})
	apiKeyRepo := NewAPIKeyRepository(client, nil)
	key := &service.APIKey{UserID: user.ID, GroupID: &groupID, Key: fmt.Sprintf("sk-system-prompt-trigger-%d", suffix), Name: "trigger", Status: service.StatusActive}
	require.NoError(t, apiKeyRepo.Create(ctx, key))
	sum := sha256.Sum256([]byte(key.Key))
	cacheKey := hex.EncodeToString(sum[:])
	_, err = tx.ExecContext(ctx, "DELETE FROM auth_cache_invalidation_outbox WHERE cache_key=$1", cacheKey)
	require.NoError(t, err)

	countInvalidations := func() int {
		t.Helper()
		rows, err := tx.QueryContext(ctx, "SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key=$1", cacheKey)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()
		require.True(t, rows.Next())
		var count int
		require.NoError(t, rows.Scan(&count))
		return count
	}
	_, err = tx.ExecContext(ctx, "UPDATE groups SET system_prompt_injection=system_prompt_injection WHERE id=$1", groupID)
	require.NoError(t, err)
	require.Zero(t, countInvalidations(), "无实际变化不应触发失效")
	_, err = tx.ExecContext(ctx, `UPDATE groups SET system_prompt_injection='{"enabled":true,"mode":"prepend","content":"new prompt"}'::jsonb WHERE id=$1`, groupID)
	require.NoError(t, err)
	require.Equal(t, 1, countInvalidations(), "配置更新必须进入认证缓存失效 outbox")
}
