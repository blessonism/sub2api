package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMapperSystemPromptInjectionAdminOnly(t *testing.T) {
	config := service.GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "private prompt"}
	group := &service.Group{ID: 31, Platform: service.PlatformOpenAI, SystemPromptInjection: config}
	adminJSON, err := json.Marshal(GroupFromServiceAdmin(group))
	require.NoError(t, err)
	var admin struct {
		SystemPromptInjection service.GroupSystemPromptInjection `json:"system_prompt_injection"`
	}
	require.NoError(t, json.Unmarshal(adminJSON, &admin))
	require.Equal(t, config, admin.SystemPromptInjection)
	for _, mapper := range []func(*service.Group) *Group{GroupFromService, GroupFromServiceShallow, GroupFromServiceUserVisible} {
		userJSON, err := json.Marshal(mapper(group))
		require.NoError(t, err)
		require.NotContains(t, string(userJSON), "system_prompt_injection")
		require.NotContains(t, string(userJSON), config.Content)
	}
	defaults := GroupFromServiceAdmin(&service.Group{})
	require.Equal(t, "append", defaults.SystemPromptInjection.Mode)
}
