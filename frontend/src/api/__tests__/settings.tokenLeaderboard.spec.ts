import { describe, expect, it, vi } from "vitest";

import { apiClient } from "@/api/client";
import {
  updateTokenLeaderboardSettings,
} from "@/api/admin/settings";

describe("admin token leaderboard settings API", () => {
  it("uses the isolated token leaderboard endpoint", async () => {
    const put = vi.spyOn(apiClient, "put").mockResolvedValue({
      data: {
        token_leaderboard_common_group_id: 21,
        token_leaderboard_tier_tooltip: "说明",
      },
    });

    await updateTokenLeaderboardSettings({
      token_leaderboard_common_group_id: 21,
      token_leaderboard_tier_tooltip: "说明",
    });

    expect(put).toHaveBeenCalledWith("/admin/settings/token-leaderboard", {
      token_leaderboard_common_group_id: 21,
      token_leaderboard_tier_tooltip: "说明",
    });
  });
});
