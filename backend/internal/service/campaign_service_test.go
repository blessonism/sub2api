package service

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCalculatePoolInjectionCentsUsesIntegerCents(t *testing.T) {
	got := calculatePoolInjectionCents(50_000, decimal.RequireFromString("0.10"))
	if got != 5_000 {
		t.Fatalf("500 元充值应入池 50 元，实际=%d 分", got)
	}
}

func TestCalculateCampaignRewardsUsesRankWeightsAndContributionSqrt(t *testing.T) {
	cfg := campaignTestConfig(100)
	rows := []CampaignLeaderboardRow{
		campaignTestLeaderboardRow(1, 9, 90_000),
		campaignTestLeaderboardRow(2, 4, 40_000),
	}

	summary := calculateCampaignRewards(7, cfg, 100_000, rows, CampaignCalculationFinal, "batch")

	if summary.RankPoolCents != 80_000 || summary.ContributionPoolCents != 20_000 {
		t.Fatalf("奖金池拆分不符合 80/20：rank=%d contribution=%d", summary.RankPoolCents, summary.ContributionPoolCents)
	}
	if len(summary.Results) != 2 {
		t.Fatalf("奖励结果数量=%d", len(summary.Results))
	}
	first := summary.Results[0]
	second := summary.Results[1]
	if first.RankRewardAmountCents != 24_000 || second.RankRewardAmountCents != 16_000 {
		t.Fatalf("Top 权重奖励不符合预期：first=%d second=%d", first.RankRewardAmountCents, second.RankRewardAmountCents)
	}

	expectedFirstContribution := int64(math.Floor(20_000 * 3 / 5))
	expectedSecondContribution := int64(math.Floor(20_000 * 2 / 5))
	if first.ContributionRewardAmountCents != expectedFirstContribution || second.ContributionRewardAmountCents != expectedSecondContribution {
		t.Fatalf("贡献奖励未按平方根权重分配：first=%d second=%d", first.ContributionRewardAmountCents, second.ContributionRewardAmountCents)
	}
	if summary.TotalFinalPayoutCents != first.FinalPayoutAmountCents+second.FinalPayoutAmountCents {
		t.Fatalf("最终发放汇总不等于明细合计")
	}
}

func TestCalculateCampaignRewardsWithholdsBelowMinimumPayout(t *testing.T) {
	cfg := campaignTestConfig(10_000)
	rows := []CampaignLeaderboardRow{
		campaignTestLeaderboardRow(1, 1, 2_000),
	}

	summary := calculateCampaignRewards(7, cfg, 1_000, rows, CampaignCalculationFinal, "batch")

	if len(summary.Results) != 1 {
		t.Fatalf("奖励结果数量=%d", len(summary.Results))
	}
	result := summary.Results[0]
	if result.GrossRewardAmountCents == 0 {
		t.Fatalf("测试数据应先产生一笔可回收奖励")
	}
	if result.FinalPayoutAmountCents != 0 {
		t.Fatalf("低于最低发放金额应不发放，实际=%d", result.FinalPayoutAmountCents)
	}
	if result.WithheldAmountCents != result.GrossRewardAmountCents || result.WithheldReason == "" {
		t.Fatalf("低额回收记录不完整：withheld=%d gross=%d reason=%q", result.WithheldAmountCents, result.GrossRewardAmountCents, result.WithheldReason)
	}
}

func campaignTestConfig(minPayoutCents int64) *CampaignConfigVersion {
	return &CampaignConfigVersion{
		ID:                       11,
		RechargeThresholdCents:   2_000,
		AllowAccumulatedRecharge: true,
		PoolInjectionRate:        decimal.RequireFromString("0.10"),
		RankPoolRatio:            decimal.RequireFromString("0.80"),
		ContributionPoolRatio:    decimal.RequireFromString("0.20"),
		RankRewardCount:          10,
		RankWeights:              []int64{30, 20, 15, 10, 8, 6, 4, 3, 2, 2},
		MinPayoutAmountCents:     minPayoutCents,
	}
}

func campaignTestLeaderboardRow(userID int64, validInvites int, rechargeCents int64) CampaignLeaderboardRow {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	return CampaignLeaderboardRow{
		UserID:                     userID,
		ValidInviteCount:           validInvites,
		InviteeRechargeAmountCents: rechargeCents,
		ReachedCountAt:             now,
		JoinedAt:                   now.Add(-time.Hour),
	}
}
