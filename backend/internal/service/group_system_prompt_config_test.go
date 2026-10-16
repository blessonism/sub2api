//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupSystemPromptInjection(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		config   GroupSystemPromptInjection
		wantMode string
		wantErr  bool
	}{
		{name: "default", platform: PlatformOpenAI, wantMode: "append"},
		{name: "disabled preserves whitespace", platform: PlatformGemini, config: GroupSystemPromptInjection{Mode: "replace", Content: " \n "}, wantMode: "replace"},
		{name: "openai prepend", platform: PlatformOpenAI, config: GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "  自定义\n "}, wantMode: "prepend"},
		{name: "anthropic append", platform: PlatformAnthropic, config: GroupSystemPromptInjection{Enabled: true, Content: "custom"}, wantMode: "append"},
		{name: "replace", platform: PlatformAnthropic, config: GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "custom"}, wantMode: "replace"},
		{name: "invalid mode even disabled", platform: PlatformOpenAI, config: GroupSystemPromptInjection{Mode: "invalid"}, wantErr: true},
		{name: "blank enabled", platform: PlatformOpenAI, config: GroupSystemPromptInjection{Enabled: true, Content: "\n\t　"}, wantErr: true},
		{name: "unsupported platform", platform: PlatformGemini, config: GroupSystemPromptInjection{Enabled: true, Content: "custom"}, wantErr: true},
		{name: "composite unsupported", platform: PlatformComposite, config: GroupSystemPromptInjection{Enabled: true, Content: "custom"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeGroupSystemPromptInjection(tt.platform, tt.config)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, int32(http.StatusBadRequest), infraerrors.FromError(err).Code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantMode, got.Mode)
			require.Equal(t, tt.config.Enabled, got.Enabled)
			require.Equal(t, tt.config.Content, got.Content)
		})
	}
}

func TestAdminGroupSystemPromptInjectionLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := &groupRepoStubForAdmin{createID: 51}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{groupRepo: repo, authCacheInvalidator: invalidator}

	created, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "injection", Platform: PlatformOpenAI, RateMultiplier: 1})
	require.NoError(t, err)
	require.Equal(t, GroupSystemPromptInjection{Mode: "append"}, created.SystemPromptInjection)

	repo.getByID = created
	config := GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "  自定义 prompt\n"}
	updated, err := svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{SystemPromptInjection: &config})
	require.NoError(t, err)
	require.Equal(t, config, updated.SystemPromptInjection)
	require.Equal(t, []int64{created.ID}, invalidator.groupIDs)

	updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.Equal(t, config, updated.SystemPromptInjection, "省略配置必须保留正文和模式")

	config.Enabled = false
	updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{SystemPromptInjection: &config})
	require.NoError(t, err)
	require.Equal(t, config, updated.SystemPromptInjection, "关闭时保留正文")

	copyRepo := newDuplicateGroupRepoStub(updated)
	copyService := &adminServiceImpl{groupRepo: copyRepo, groupDuplicateRepo: copyRepo}
	duplicated, err := copyService.DuplicateGroup(ctx, updated.ID, "admin:1", "")
	require.NoError(t, err)
	require.Equal(t, config, duplicated.SystemPromptInjection)
}

func TestAdminGroupSystemPromptInjectionRejectsInvalidWrites(t *testing.T) {
	ctx := context.Background()
	invalid := GroupSystemPromptInjection{Enabled: true, Content: " \n"}
	repo := &groupRepoStubForAdmin{}
	svc := &adminServiceImpl{groupRepo: repo}
	_, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "invalid", Platform: PlatformAnthropic, RateMultiplier: 1, SystemPromptInjection: invalid})
	require.Error(t, err)
	require.Nil(t, repo.created)

	repo.getByID = &Group{ID: 7, Platform: PlatformAnthropic}
	_, err = svc.UpdateGroup(ctx, 7, &UpdateGroupInput{SystemPromptInjection: &invalid})
	require.Error(t, err)
	require.Nil(t, repo.updated)

	repo.getByID = &Group{ID: 7, Platform: PlatformAnthropic, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "valid"}}
	_, err = svc.UpdateGroup(ctx, 7, &UpdateGroupInput{Platform: PlatformGemini})
	require.Error(t, err, "切换到不支持的平台时必须先关闭注入")
	require.Nil(t, repo.updated)
}

func TestAdminGroupSystemPromptInjectionSimpleMode(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			ctx := context.Background()
			repo := &groupRepoStubForAdmin{createID: 52}
			svc := &adminServiceImpl{cfg: &config.Config{RunMode: config.RunModeSimple}, groupRepo: repo}
			want := GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "  custom prompt\n"}
			created, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "simple", Platform: platform, SystemPromptInjection: want})
			require.NoError(t, err)
			require.Equal(t, want, created.SystemPromptInjection, "简单模式创建不能丢弃注入配置")
			repo.getByID = created

			want.Mode = "replace"
			updated, err := svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{SystemPromptInjection: &want})
			require.NoError(t, err)
			require.Equal(t, want, updated.SystemPromptInjection, "简单模式编辑不能丢弃注入配置")
			updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{Name: "renamed"})
			require.NoError(t, err)
			require.Equal(t, want, updated.SystemPromptInjection)
			want.Enabled = false
			updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{SystemPromptInjection: &want})
			require.NoError(t, err)
			require.Equal(t, want, updated.SystemPromptInjection, "简单模式关闭时保留正文和模式")
		})
	}
}

func TestAdminGroupSystemPromptInjectionSimpleModeRejectsInvalidWrites(t *testing.T) {
	ctx := context.Background()
	blank := GroupSystemPromptInjection{Enabled: true, Content: " \n"}
	repo := &groupRepoStubForAdmin{getByID: &Group{ID: 53, Platform: PlatformOpenAI}}
	svc := &adminServiceImpl{cfg: &config.Config{RunMode: config.RunModeSimple}, groupRepo: repo}
	_, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "invalid", Platform: PlatformOpenAI, SystemPromptInjection: blank})
	require.Error(t, err, "简单模式不能在校验前丢弃非法配置")
	require.Nil(t, repo.created)
	_, err = svc.UpdateGroup(ctx, 53, &UpdateGroupInput{SystemPromptInjection: &blank})
	require.Error(t, err)
	require.Nil(t, repo.updated)
}
