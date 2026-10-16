//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheFullAccountKeepsGroupSystemPromptInjection(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	config := service.GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "custom"}
	account := &service.Account{ID: 131, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Groups:        []*service.Group{{ID: 5, SystemPromptInjection: config}},
		AccountGroups: []service.AccountGroup{{AccountID: 131, GroupID: 5, Group: &service.Group{ID: 5, SystemPromptInjection: config}}},
	}
	require.NoError(t, cache.SetAccount(ctx, account))
	got, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, config, got.Groups[0].SystemPromptInjection)
	require.Equal(t, config, got.AccountGroups[0].Group.SystemPromptInjection)
}
