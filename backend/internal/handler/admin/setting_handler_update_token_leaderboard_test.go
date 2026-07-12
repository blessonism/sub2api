package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsRequestBindsTokenLeaderboardControls(t *testing.T) {
	payload := []byte(`{
		"token_leaderboard_user_visible": true,
		"token_leaderboard_common_group_id": 7,
		"token_leaderboard_tier_tooltip": "按用量匹配权重"
	}`)
	var req UpdateSettingsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.TokenLeaderboardUserVisible == nil || !*req.TokenLeaderboardUserVisible {
		t.Fatalf("token leaderboard visibility was not bound")
	}
	if req.TokenLeaderboardCommonGroupID == nil || *req.TokenLeaderboardCommonGroupID != 7 {
		t.Fatalf("common group id was not bound")
	}
	if req.TokenLeaderboardTierTooltip == nil || *req.TokenLeaderboardTierTooltip != "按用量匹配权重" {
		t.Fatalf("tier tooltip was not bound")
	}
}

func TestUpdateSettingsRejectsRequestWithoutExplicitReplaceMode(t *testing.T) {
	handler, repo := newDingTalkSettingsHandler()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewBufferString(`{
		"token_leaderboard_common_group_id": 7,
		"token_leaderboard_tier_tooltip": "说明"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.lastUpdates)
}

func TestUpdateSettingsRejectsIncompleteRequestWithReplaceMode(t *testing.T) {
	handler, repo := newDingTalkSettingsHandler()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewBufferString(`{
		"token_leaderboard_common_group_id": 7,
		"token_leaderboard_tier_tooltip": "说明"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(settingsReplaceModeHeader, "replace")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_SETTINGS_REPLACE_REQUEST")
	require.Nil(t, repo.lastUpdates)
}

func TestUpdateTokenLeaderboardSettingsOnlyWritesOwnedKeys(t *testing.T) {
	repo := &settingHandlerRepoStub{values: map[string]string{"site_name": "保留名称"}}
	svc := service.NewSettingService(repo, &config.Config{})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/token-leaderboard", bytes.NewBufferString(`{
		"token_leaderboard_common_group_id": 0,
		"token_leaderboard_tier_tooltip": "  说明  "
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateTokenLeaderboardSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "保留名称", repo.values["site_name"])
	require.Equal(t, map[string]string{
		service.SettingKeyTokenLeaderboardCommonGroupID: "0",
		service.SettingKeyTokenLeaderboardTierTooltip:   "说明",
	}, repo.lastUpdates)
}
