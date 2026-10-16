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
		Group: &Group{ID: groupID, Name: "system-prompt", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, SystemPromptInjection: config},
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
	require.Equal(t, apiKeyAuthSnapshotVersion, cached.Snapshot.Version)

	cached.Snapshot.Version = 24
	materialized, used, err = svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.False(t, used, "旧缓存不含配置，必须回源")
	require.Nil(t, materialized)
}
