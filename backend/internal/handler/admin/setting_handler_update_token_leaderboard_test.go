package admin

import (
	"encoding/json"
	"testing"
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
