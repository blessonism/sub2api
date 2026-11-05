package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGroupSystemPromptInjectionRoundtrip(t *testing.T) {
	groupID := int64(60)
	config := GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "  自定义\n"}
	apiKey := &APIKey{
		ID: 92, UserID: 46, GroupID: &groupID, Key: "sk-system-prompt-roundtrip", Status: StatusActive,
		User:  &User{ID: 46, Status: StatusActive},
		Group: &Group{ID: groupID, Name: "system-prompt", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, SystemPromptInjection: config, ImageCountBilling: true},
	}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, config, materialized.Group.SystemPromptInjection)
	require.True(t, materialized.Group.ImageCountBilling)
	require.Equal(t, apiKeyAuthSnapshotVersion, cached.Snapshot.Version)
	require.GreaterOrEqual(t, cached.Snapshot.Version, 27, "合并后的缓存必须区别于双方各自发布的快照")

	for _, version := range []int{24, 25, 26} {
		cached.Snapshot.Version = version
		materialized, used, err = svc.applyAuthCacheEntry(apiKey.Key, &cached)
		require.NoError(t, err)
		require.False(t, used, "旧缓存不含合并后的完整配置，必须回源：v%d", version)
		require.Nil(t, materialized)
	}
}
