package service

import (
	"context"
	"errors"
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
	if first.RankRewardAmountCents != 48_000 || second.RankRewardAmountCents != 32_000 {
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

func TestCalculateCampaignRewardsRedistributesVacantRankWeights(t *testing.T) {
	cfg := campaignTestConfig(0)
	rows := []CampaignLeaderboardRow{
		campaignTestLeaderboardRow(1, 1, 10_000),
		campaignTestLeaderboardRow(2, 1, 10_000),
		campaignTestLeaderboardRow(3, 1, 10_000),
		campaignTestLeaderboardRow(4, 1, 10_000),
		campaignTestLeaderboardRow(5, 1, 10_000),
	}

	summary := calculateCampaignRewards(7, cfg, 100_000, rows, CampaignCalculationFinal, "partial-ranking")
	var totalRankReward int64
	for _, result := range summary.Results {
		totalRankReward += result.RankRewardAmountCents
	}
	if totalRankReward+summary.TotalRoundingResidualCents != summary.RankPoolCents {
		t.Fatalf("空缺名次奖金应由实际五人瓜分：rank=%d residual=%d pool=%d", totalRankReward, summary.TotalRoundingResidualCents, summary.RankPoolCents)
	}

	for userID := int64(6); userID <= 10; userID++ {
		rows = append(rows, campaignTestLeaderboardRow(userID, 1, 10_000))
	}
	full := calculateCampaignRewards(7, cfg, 100_000, rows, CampaignCalculationFinal, "full-ranking")
	if full.Results[0].RankRewardAmountCents != 24_000 || full.Results[9].RankRewardAmountCents != 1_600 {
		t.Fatalf("满榜时应保持原权重：first=%d tenth=%d", full.Results[0].RankRewardAmountCents, full.Results[9].RankRewardAmountCents)
	}
}

func TestCalculateCampaignRewardsPreservesFractionalInviteCounts(t *testing.T) {
	cfg := campaignTestConfig(0)
	rows := []CampaignLeaderboardRow{
		campaignTestLeaderboardRow(1, 1, 100),
		campaignTestLeaderboardRow(2, 1, 100),
	}
	rows[0].ValidInviteCount = 1.25
	rows[1].ValidInviteCount = 1.5

	summary := calculateCampaignRewards(7, cfg, 10_000, rows, CampaignCalculationPreview, "fractional")
	if len(summary.Results) != 2 || summary.Results[0].UserID != 2 {
		t.Fatalf("小数有效邀请次数应完整参与排序：%+v", summary.Results)
	}
	if !summary.Results[0].ContributionWeight.GreaterThan(summary.Results[1].ContributionWeight) {
		t.Fatalf("小数有效邀请次数应完整参与贡献权重计算")
	}
}

func TestBuildInitialCampaignConfigRejectsHistoricalInviteRatioOutsideRange(t *testing.T) {
	input := CampaignCreateInput{
		StartAt:                  time.Now(),
		RechargeThresholdCents:   2_000,
		HistoricalInviteRatio:    decimal.RequireFromString("1.01"),
		PoolInjectionRate:        decimal.RequireFromString("0.1"),
		RankPoolRatio:            decimal.RequireFromString("0.8"),
		ContributionPoolRatio:    decimal.RequireFromString("0.2"),
		RankRewardCount:          10,
		RankWeights:              append([]int64(nil), defaultCampaignRankWeights...),
		MinPayoutAmountCents:     100,
		AllowAccumulatedRecharge: true,
	}
	if _, err := buildInitialCampaignConfig(input); !errors.Is(err, ErrCampaignInvalidConfig) {
		t.Fatalf("超出范围的历史邀请折算比例应被拒绝，err=%v", err)
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

func TestCalculateCampaignRewardsTracksRoundingResidual(t *testing.T) {
	cfg := campaignTestConfig(0)
	rows := []CampaignLeaderboardRow{
		campaignTestLeaderboardRow(1, 1, 10_000),
		campaignTestLeaderboardRow(2, 1, 10_000),
		campaignTestLeaderboardRow(3, 1, 10_000),
	}

	summary := calculateCampaignRewards(7, cfg, 101, rows, CampaignCalculationFinal, "batch")

	if summary.TotalRoundingResidualCents <= 0 {
		t.Fatalf("向下取整产生的尾差应被记录，实际=%d", summary.TotalRoundingResidualCents)
	}
	if summary.TotalGrossRewardCents+summary.TotalRoundingResidualCents != summary.FinalPoolCents {
		t.Fatalf("总奖励与尾差应可对账：gross=%d residual=%d pool=%d", summary.TotalGrossRewardCents, summary.TotalRoundingResidualCents, summary.FinalPoolCents)
	}
}

func TestCampaignPayoutRejectsAlreadyPaidRewardResult(t *testing.T) {
	repo := &campaignPayoutRepoStub{
		results: []CampaignRewardResult{{
			ID:                     101,
			CampaignID:             7,
			UserID:                 21,
			FinalPayoutAmountCents: 500,
		}},
		existingResultBatch: &CampaignPayoutBatch{ID: 9, CampaignID: 7, Status: "failed"},
	}
	svc := NewCampaignService(repo, nil)
	svc.balanceGrant = campaignGrantStub{}

	_, err := svc.Payout(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignAlreadyPaid) {
		t.Fatalf("已有 reward_result 发放记录时应拒绝重复发放，实际 err=%v", err)
	}
}

func TestCampaignLeaderboardAdjustmentInvalidatesFinalSettlement(t *testing.T) {
	repo := &campaignManualAdjustmentRepoStub{campaign: &Campaign{ID: 7, Status: CampaignStatusPublicizing}}
	svc := NewCampaignService(repo, nil)

	adjustment, err := svc.AddLeaderboardAdjustment(context.Background(), 7, CampaignLeaderboardAdjustmentInput{
		UserID:                   21,
		ValidInviteDelta:         2,
		RechargeAmountDeltaCents: 10_000,
		Reason:                   "补录线下确认邀请",
	})
	if err != nil {
		t.Fatalf("手动调整榜单失败：%v", err)
	}
	if adjustment == nil || repo.added == nil || repo.invalidatedCampaignID != 7 {
		t.Fatalf("调整后应写入增量并失效最终结算，adjustment=%+v added=%+v invalidated=%d", adjustment, repo.added, repo.invalidatedCampaignID)
	}
}

func TestCampaignLeaderboardAdjustmentRejectsPaidCampaign(t *testing.T) {
	repo := &campaignManualAdjustmentRepoStub{campaign: &Campaign{ID: 7, Status: CampaignStatusPaid}}
	svc := NewCampaignService(repo, nil)

	_, err := svc.AddLeaderboardAdjustment(context.Background(), 7, CampaignLeaderboardAdjustmentInput{UserID: 21, ValidInviteDelta: 1})
	if !errors.Is(err, ErrCampaignAlreadyPaid) {
		t.Fatalf("已派奖活动应拒绝手动调整，实际 err=%v", err)
	}
	if repo.added != nil || repo.invalidatedCampaignID != 0 {
		t.Fatalf("拒绝调整时不应写入或失效结算，added=%+v invalidated=%d", repo.added, repo.invalidatedCampaignID)
	}
}

func TestCampaignSettlementInputsRejectAfterPayoutBatch(t *testing.T) {
	repo := &campaignManualAdjustmentRepoStub{
		campaign:         &Campaign{ID: 7, Status: CampaignStatusPublicizing},
		successfulPayout: &CampaignPayoutBatch{ID: 9, CampaignID: 7, Status: "failed"},
	}
	svc := NewCampaignService(repo, nil)

	if err := svc.AddPoolAdjustment(context.Background(), 7, CampaignPoolAdjustmentInput{AdjustmentType: "additional_bonus", AmountCents: 100}); !errors.Is(err, ErrCampaignAlreadyPaid) {
		t.Fatalf("已有发放批次后应拒绝奖池调整，实际 err=%v", err)
	}
	if _, err := svc.AdjustInviteRecord(context.Background(), 7, CampaignInviteRecordAdjustmentInput{RecordID: 11, Status: CampaignInviteStatusEffective}); !errors.Is(err, ErrCampaignAlreadyPaid) {
		t.Fatalf("已有发放批次后应拒绝邀请记录调整，实际 err=%v", err)
	}
	if _, err := svc.AddLeaderboardAdjustment(context.Background(), 7, CampaignLeaderboardAdjustmentInput{UserID: 21, ValidInviteDelta: 1}); !errors.Is(err, ErrCampaignAlreadyPaid) {
		t.Fatalf("已有发放批次后应拒绝排行榜调整，实际 err=%v", err)
	}
	if repo.added != nil || repo.addedPool != nil || repo.adjustedInvite != nil {
		t.Fatalf("拒绝调整时不应写入任何结算输入，leaderboard=%+v pool=%+v invite=%+v", repo.added, repo.addedPool, repo.adjustedInvite)
	}
}

func TestCampaignPoolAdjustmentInvalidatesFinalBeforePayout(t *testing.T) {
	repo := &campaignManualAdjustmentRepoStub{campaign: &Campaign{ID: 7, Status: CampaignStatusPublicizing}}
	svc := NewCampaignService(repo, nil)
	svc.balanceGrant = campaignGrantStub{}

	err := svc.AddPoolAdjustment(context.Background(), 7, CampaignPoolAdjustmentInput{AdjustmentType: "additional_bonus", AmountCents: 100, Reason: "补充奖池"})
	if err != nil {
		t.Fatalf("奖池调整失败：%v", err)
	}
	if repo.addedPool == nil {
		t.Fatalf("奖池调整应写入仓储")
	}
	if _, err := svc.Payout(context.Background(), 7, nil); !errors.Is(err, ErrCampaignNoFinalSettlement) {
		t.Fatalf("final 被奖池调整失效后应拒绝发放，实际 err=%v", err)
	}
}

func TestCampaignFinalRecalculationRejectsBeforeCampaignEnd(t *testing.T) {
	repo := &campaignRecalculateRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusActive, EndAt: time.Now().Add(time.Hour)},
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.RecalculateRewards(context.Background(), 7, CampaignCalculationFinal)
	if !errors.Is(err, ErrCampaignInvalidConfig) {
		t.Fatalf("活动结束前应拒绝 final 结算，实际 err=%v", err)
	}
	if repo.saved {
		t.Fatalf("拒绝 final 时不应保存结算结果")
	}
}

func TestCampaignRecordRechargeAllUsersInjectsPoolWithoutInviteRecord(t *testing.T) {
	successAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	repo := &campaignRechargeRepoStub{
		campaign: &Campaign{ID: 7, StartAt: successAt.Add(-time.Hour), EndAt: successAt.Add(time.Hour)},
		cfg: &CampaignConfigVersion{
			ID:                 11,
			PoolInjectionRate:  decimal.RequireFromString("0.10"),
			PoolInjectionScope: CampaignPoolInjectionScopeAllUsers,
		},
		recordRechargeErr: ErrCampaignNotFound,
	}
	svc := NewCampaignService(repo, nil)

	invite, err := svc.RecordRecharge(context.Background(), CampaignRechargeInput{
		InviteeUserID:       99,
		SourceType:          "payment_order",
		SourceID:            "order-1",
		SourceSuccessAt:     successAt,
		RechargeAmountCents: 5_000,
	})
	if err != nil {
		t.Fatalf("全员充值入池不应因没有邀请记录失败：%v", err)
	}
	if invite != nil {
		t.Fatalf("非受邀用户充值只注入奖池，不应返回邀请记录：%+v", invite)
	}
	if !repo.insertPoolCalled || repo.insertedInvite != nil || repo.insertedPoolAmount != 500 {
		t.Fatalf("非受邀用户充值应写入无邀请记录的奖池流水，called=%v invite=%+v pool=%d", repo.insertPoolCalled, repo.insertedInvite, repo.insertedPoolAmount)
	}
}

func TestCampaignRecordRechargeInviteesOnlyIgnoresRechargeWithoutInviteRecord(t *testing.T) {
	successAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	repo := &campaignRechargeRepoStub{
		campaign: &Campaign{ID: 7, StartAt: successAt.Add(-time.Hour), EndAt: successAt.Add(time.Hour)},
		cfg: &CampaignConfigVersion{
			ID:                 11,
			PoolInjectionRate:  decimal.RequireFromString("0.10"),
			PoolInjectionScope: CampaignPoolInjectionScopeInviteesOnly,
		},
		recordRechargeErr: ErrCampaignNotFound,
	}
	svc := NewCampaignService(repo, nil)

	invite, err := svc.RecordRecharge(context.Background(), CampaignRechargeInput{
		InviteeUserID:       99,
		SourceType:          "payment_order",
		SourceID:            "order-1",
		SourceSuccessAt:     successAt,
		RechargeAmountCents: 5_000,
	})
	if err != nil || invite != nil {
		t.Fatalf("仅受邀新用户模式下无邀请记录充值应被忽略，invite=%+v err=%v", invite, err)
	}
	if repo.insertPoolCalled {
		t.Fatalf("仅受邀新用户模式下不应写入非受邀充值奖池流水")
	}
}

func TestCampaignDeleteRemovesEmptyDraft(t *testing.T) {
	repo := &campaignDeleteRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusDraft},
		impact:   CampaignDeleteImpact{},
	}
	svc := NewCampaignService(repo, nil)

	result, err := svc.DeleteCampaign(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("删除空草稿活动失败：%v", err)
	}
	if result.Action != "deleted" || !repo.deleted {
		t.Fatalf("空草稿活动应硬删除，action=%q deleted=%v", result.Action, repo.deleted)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("硬删除时不应更新状态，status=%q", repo.updatedStatus)
	}
}

func TestCampaignDeleteArchivesActiveCampaignWithBusinessData(t *testing.T) {
	operatorID := int64(99)
	repo := &campaignDeleteRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusActive},
		impact:   CampaignDeleteImpact{Participants: 1},
	}
	svc := NewCampaignService(repo, nil)

	result, err := svc.DeleteCampaign(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("归档有数据活动失败：%v", err)
	}
	if result.Action != "archived" || result.Campaign == nil || result.Campaign.Status != CampaignStatusTerminated {
		t.Fatalf("进行中活动应终止归档，result=%+v", result)
	}
	if repo.deleted {
		t.Fatalf("有业务数据的活动不能硬删除")
	}
	if repo.updatedStatus != CampaignStatusTerminated || repo.updatedOperator == nil || *repo.updatedOperator != operatorID {
		t.Fatalf("终止状态或操作者未记录，status=%q operator=%v", repo.updatedStatus, repo.updatedOperator)
	}
}

func TestCampaignDeleteArchivesFrozenCampaignAsTerminated(t *testing.T) {
	operatorID := int64(99)
	repo := &campaignDeleteRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusFrozen},
		impact:   CampaignDeleteImpact{LeaderboardSnapshots: 1},
	}
	svc := NewCampaignService(repo, nil)

	result, err := svc.DeleteCampaign(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("归档已冻结活动失败：%v", err)
	}
	if result.Action != "archived" || result.Campaign == nil || result.Campaign.Status != CampaignStatusTerminated {
		t.Fatalf("已冻结活动应终止归档而非取消，result=%+v", result)
	}
	if repo.updatedStatus != CampaignStatusTerminated {
		t.Fatalf("已冻结活动删除应落到 terminated，实际 status=%q", repo.updatedStatus)
	}
}

func TestCampaignDeleteRejectsPaidCampaign(t *testing.T) {
	repo := &campaignDeleteRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusPaid},
		impact:   CampaignDeleteImpact{PayoutBatches: 1},
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.DeleteCampaign(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignDeleteBlocked) {
		t.Fatalf("已发放活动应拒绝删除，实际 err=%v", err)
	}
	if repo.deleted || repo.updatedStatus != "" {
		t.Fatalf("拒绝删除时不应变更活动，deleted=%v status=%q", repo.deleted, repo.updatedStatus)
	}
}

func TestCampaignFreezeUpdatesStatus(t *testing.T) {
	operatorID := int64(99)
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusActive},
		cfg:      campaignTestConfig(0),
		pool:     &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 1_000},
		rows:     []CampaignLeaderboardRow{campaignTestLeaderboardRow(1, 2, 10_000)},
	}
	svc := NewCampaignService(repo, nil)

	if err := svc.FreezeLeaderboard(context.Background(), 7, &operatorID); err != nil {
		t.Fatalf("冻结榜单失败：%v", err)
	}
	if !repo.snapshotSaved || repo.snapshotType != "end_frozen" {
		t.Fatalf("冻结时应保存榜单快照，saved=%v type=%q", repo.snapshotSaved, repo.snapshotType)
	}
	if repo.updatedStatus != CampaignStatusFrozen || repo.updatedOperator == nil || *repo.updatedOperator != operatorID {
		t.Fatalf("冻结后应更新为 frozen 并记录操作者，status=%q operator=%v", repo.updatedStatus, repo.updatedOperator)
	}
}

func TestCampaignPauseAndResumeWithinActiveWindow(t *testing.T) {
	now := time.Now()
	operatorID := int64(99)
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusActive, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
	}
	svc := NewCampaignService(repo, nil)

	paused, err := svc.PauseCampaign(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("暂停活动失败：%v", err)
	}
	if paused.Status != CampaignStatusPaused || repo.updatedStatus != CampaignStatusPaused {
		t.Fatalf("暂停后状态应为 paused，campaign=%+v updated=%q", paused, repo.updatedStatus)
	}

	resumed, err := svc.ResumeCampaign(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("恢复活动失败：%v", err)
	}
	if resumed.Status != CampaignStatusActive || repo.updatedStatus != CampaignStatusActive {
		t.Fatalf("活动期内恢复应回到 active，campaign=%+v updated=%q", resumed, repo.updatedStatus)
	}
}

func TestCampaignResumeReturnsToWarmupBeforeStart(t *testing.T) {
	now := time.Now()
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusPaused, StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour)},
	}
	svc := NewCampaignService(repo, nil)

	resumed, err := svc.ResumeCampaign(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("恢复预热期活动失败：%v", err)
	}
	if resumed.Status != CampaignStatusWarmup || repo.updatedStatus != CampaignStatusWarmup {
		t.Fatalf("开始前恢复应回到 warmup，campaign=%+v updated=%q", resumed, repo.updatedStatus)
	}
	if repo.hasActiveChecked {
		t.Fatal("恢复到 warmup 时不应检查 active 单例约束")
	}
}

func TestCampaignUnfreezeReturnsToActiveWithinWindow(t *testing.T) {
	now := time.Now()
	operatorID := int64(99)
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusFrozen, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
	}
	svc := NewCampaignService(repo, nil)

	unfrozen, err := svc.UnfreezeLeaderboard(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("取消冻结失败：%v", err)
	}
	if unfrozen.Status != CampaignStatusActive || repo.updatedStatus != CampaignStatusActive {
		t.Fatalf("活动期内取消冻结应回到 active，campaign=%+v updated=%q", unfrozen, repo.updatedStatus)
	}
}

func TestCampaignUnfreezeReturnsToAuditingAfterEnd(t *testing.T) {
	now := time.Now()
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusFrozen, StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour)},
	}
	svc := NewCampaignService(repo, nil)

	unfrozen, err := svc.UnfreezeLeaderboard(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("活动结束后取消冻结失败：%v", err)
	}
	if unfrozen.Status != CampaignStatusAuditing || repo.updatedStatus != CampaignStatusAuditing {
		t.Fatalf("活动结束后取消冻结应回到 auditing，campaign=%+v updated=%q", unfrozen, repo.updatedStatus)
	}
	if repo.hasActiveChecked {
		t.Fatal("取消冻结到 auditing 时不应检查 active 单例约束")
	}
}

func TestCampaignUnfreezeRejectsWhenFinalSettlementExists(t *testing.T) {
	repo := &campaignStatusRepoStub{
		campaign: &Campaign{ID: 7, Status: CampaignStatusFrozen, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)},
		finalResults: []CampaignRewardResult{{
			ID:                101,
			CampaignID:        7,
			CalculationStatus: CampaignCalculationFinal,
		}},
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.UnfreezeLeaderboard(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignSettlementLocked) {
		t.Fatalf("存在最终结算结果时应拒绝取消冻结，实际 err=%v", err)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("拒绝取消冻结时不应更新状态，status=%q", repo.updatedStatus)
	}
}

func TestCampaignUnfreezeRejectsWhenPayoutBatchExists(t *testing.T) {
	repo := &campaignStatusRepoStub{
		campaign:    &Campaign{ID: 7, Status: CampaignStatusFrozen, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)},
		payoutBatch: &CampaignPayoutBatch{ID: 9, CampaignID: 7, Status: "failed"},
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.UnfreezeLeaderboard(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignSettlementLocked) {
		t.Fatalf("存在发放批次时应拒绝取消冻结，实际 err=%v", err)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("拒绝取消冻结时不应更新状态，status=%q", repo.updatedStatus)
	}
}

func TestCampaignUnfreezeRejectsDuplicateActiveCampaign(t *testing.T) {
	now := time.Now()
	repo := &campaignStatusRepoStub{
		campaign:       &Campaign{ID: 7, Status: CampaignStatusFrozen, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
		hasOtherActive: true,
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.UnfreezeLeaderboard(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignDuplicateActive) {
		t.Fatalf("已有其他 active 活动时应拒绝取消冻结，实际 err=%v", err)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("拒绝取消冻结时不应更新状态，status=%q", repo.updatedStatus)
	}
}

func TestCampaignResumeRejectsDuplicateActiveCampaign(t *testing.T) {
	now := time.Now()
	repo := &campaignStatusRepoStub{
		campaign:       &Campaign{ID: 7, Status: CampaignStatusPaused, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
		hasOtherActive: true,
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.ResumeCampaign(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignDuplicateActive) {
		t.Fatalf("已有其他 active 活动时应拒绝恢复，实际 err=%v", err)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("拒绝恢复时不应更新状态，status=%q", repo.updatedStatus)
	}
}

// TestCampaignResumePropagatesDuplicateActiveFromUpdate 覆盖 HasActiveCampaign 检查通过、
// 但落库瞬间被并发提升的 warmup 活动抢占单例索引的竞态：仓储层将唯一冲突翻译为
// ErrCampaignDuplicateActive，服务层应原样上抛而非吞掉或改写成 500。
func TestCampaignResumePropagatesDuplicateActiveFromUpdate(t *testing.T) {
	now := time.Now()
	repo := &campaignStatusRepoStub{
		campaign:        &Campaign{ID: 7, Status: CampaignStatusPaused, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
		updateStatusErr: ErrCampaignDuplicateActive,
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.ResumeCampaign(context.Background(), 7, nil)
	if !errors.Is(err, ErrCampaignDuplicateActive) {
		t.Fatalf("落库唯一冲突应上抛 ErrCampaignDuplicateActive，实际 err=%v", err)
	}
	if !repo.hasActiveChecked {
		t.Fatal("恢复到 active 前应检查 active 单例约束")
	}
	if repo.updatedStatus != "" {
		t.Fatalf("落库被拒时不应记录状态变更，status=%q", repo.updatedStatus)
	}
}

func TestCampaignCopyCreatesDraftFromLatestConfigWithoutBusinessData(t *testing.T) {
	operatorID := int64(99)
	startAt := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	warmupAt := startAt.Add(-24 * time.Hour)
	auditStartAt := startAt.Add(7 * 24 * time.Hour)
	auditEndAt := auditStartAt.Add(24 * time.Hour)
	cfg := campaignTestConfig(500)
	cfg.ID = 31
	cfg.RechargeThresholdCents = 8_800
	cfg.PoolInjectionRate = decimal.RequireFromString("0.15")
	repo := &campaignCopyRepoStub{
		source: &Campaign{
			ID:            7,
			Name:          "暑期邀请活动",
			Description:   "复用说明",
			CoverURL:      "https://example.com/cover.png",
			RulesText:     "复用规则",
			Status:        CampaignStatusPaid,
			WarmupStartAt: &warmupAt,
			StartAt:       startAt,
			EndAt:         startAt.Add(6 * 24 * time.Hour),
			AuditStartAt:  &auditStartAt,
			AuditEndAt:    &auditEndAt,
			CreatedBy:     campaignPtrInt64(1),
			UpdatedBy:     campaignPtrInt64(2),
			CreatedAt:     startAt.Add(-48 * time.Hour),
			UpdatedAt:     startAt.Add(-time.Hour),
		},
		cfg: cfg,
	}
	svc := NewCampaignService(repo, nil)

	campaign, version, err := svc.CopyCampaign(context.Background(), 7, &operatorID)
	if err != nil {
		t.Fatalf("复制活动失败：%v", err)
	}
	if campaign == nil || campaign.Status != CampaignStatusDraft || campaign.Name != "暑期邀请活动 副本" {
		t.Fatalf("复制活动应创建草稿副本，campaign=%+v", campaign)
	}
	if version == nil || version.RechargeThresholdCents != 8_800 || !version.PoolInjectionRate.Equal(decimal.RequireFromString("0.15")) {
		t.Fatalf("复制活动应复用最新奖励配置，version=%+v", version)
	}
	if repo.createdInput.InitialBonusCents != 0 {
		t.Fatalf("复制活动不应复制初始奖池调整，initial_bonus=%d", repo.createdInput.InitialBonusCents)
	}
	if repo.addPoolAdjustmentCalled {
		t.Fatalf("复制活动不应写入奖池调整流水")
	}
	if repo.createdInput.OperatorID == nil || *repo.createdInput.OperatorID != operatorID {
		t.Fatalf("复制活动应记录操作者，operator=%v", repo.createdInput.OperatorID)
	}
}

func TestCampaignUpdateAllowsTimelinePatchAndValidatesMergedTimeRange(t *testing.T) {
	startAt := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	endAt := startAt.Add(48 * time.Hour)
	auditStartAt := endAt.Add(2 * time.Hour)
	publicityEndAt := auditStartAt.Add(24 * time.Hour)
	operatorID := int64(99)
	var clearedWarmup *time.Time

	repo := &campaignUpdateRepoStub{
		campaign: &Campaign{
			ID:      7,
			Name:    "邀请活动",
			Status:  CampaignStatusActive,
			StartAt: startAt,
			EndAt:   endAt,
		},
	}
	svc := NewCampaignService(repo, nil)

	updated, err := svc.UpdateCampaign(context.Background(), 7, CampaignUpdateInput{
		Name:           campaignPtrString("夏季邀请活动"),
		WarmupStartAt:  &clearedWarmup,
		EndAt:          campaignPtrTime(endAt.Add(24 * time.Hour)),
		AuditStartAt:   ptrTimePatch(auditStartAt),
		PublicityEndAt: ptrTimePatch(publicityEndAt),
		PayoutDueAt:    ptrTimePatch(publicityEndAt.Add(48 * time.Hour)),
		OperatorID:     &operatorID,
	})
	if err != nil {
		t.Fatalf("更新活动时间线失败：%v", err)
	}
	if updated == nil || updated.Name != "夏季邀请活动" || !updated.EndAt.Equal(endAt.Add(24*time.Hour)) {
		t.Fatalf("更新后的活动不符合预期：%+v", updated)
	}
	if !repo.updateCalled {
		t.Fatalf("有效更新时间线时应调用仓储")
	}
	if repo.updatedInput.WarmupStartAt == nil || *repo.updatedInput.WarmupStartAt != nil {
		t.Fatalf("预热时间应支持清空，input=%+v", repo.updatedInput.WarmupStartAt)
	}
	if repo.updatedInput.AuditStartAt == nil || *repo.updatedInput.AuditStartAt == nil || !(*repo.updatedInput.AuditStartAt).Equal(auditStartAt) {
		t.Fatalf("审核开始时间未传递到仓储，input=%+v", repo.updatedInput.AuditStartAt)
	}
	if repo.updatedInput.PublicityEndAt == nil || *repo.updatedInput.PublicityEndAt == nil || !(*repo.updatedInput.PublicityEndAt).Equal(publicityEndAt) {
		t.Fatalf("公示结束时间未传递到仓储，input=%+v", repo.updatedInput.PublicityEndAt)
	}
	if repo.updatedInput.OperatorID == nil || *repo.updatedInput.OperatorID != operatorID {
		t.Fatalf("操作者未传递到仓储，operator=%v", repo.updatedInput.OperatorID)
	}

	repo.updateCalled = false
	invalidStart := endAt.Add(72 * time.Hour)
	_, err = svc.UpdateCampaign(context.Background(), 7, CampaignUpdateInput{StartAt: &invalidStart})
	if !errors.Is(err, ErrCampaignInvalidConfig) {
		t.Fatalf("开始/结束时间合并后无效时应返回 ErrCampaignInvalidConfig，实际=%v", err)
	}
	if repo.updateCalled {
		t.Fatalf("时间校验失败时不应调用仓储更新")
	}
}

func TestCampaignGetMyDataReturnsCurrentRankOutsideTop50(t *testing.T) {
	repo := &campaignMyDataRepoStub{
		campaign: &Campaign{ID: 7, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)},
		cfg:      campaignTestConfig(100),
		pool:     &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 100_000},
		topRows: []CampaignLeaderboardRow{
			campaignTestLeaderboardRow(1, 10, 100_000),
			campaignTestLeaderboardRow(2, 9, 90_000),
		},
		currentUserRow: campaignTestLeaderboardRow(99, 1, 20_000),
	}
	repo.currentUserRow.Rank = 57

	svc := NewCampaignService(repo, nil)
	got, err := svc.GetMyData(context.Background(), 7, 99)
	if err != nil {
		t.Fatalf("获取我的活动数据失败：%v", err)
	}
	if got.CurrentRank == nil || *got.CurrentRank != 57 {
		t.Fatalf("Top50 外用户应返回补位排名，实际=%v", got.CurrentRank)
	}
	if !repo.loadedCurrentUserRank {
		t.Fatalf("Top50 列表未命中当前用户时应查询当前用户排名补位")
	}
	if got.ValidInviteCount != 1 || got.InviteeRechargeAmountCents != 20_000 {
		t.Fatalf("补位排名行应同步我的有效邀请与充值金额，valid=%g recharge=%d", got.ValidInviteCount, got.InviteeRechargeAmountCents)
	}
}

func TestCampaignGetMyDataFallsBackToAffiliateIdentityWithoutParticipant(t *testing.T) {
	repo := &campaignMyDataRepoStub{
		campaign:  &Campaign{ID: 7, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)},
		cfg:       campaignTestConfig(100),
		pool:      &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 100_000},
		affiliate: &AffiliateSummary{UserID: 99, AffCode: "AFF99"},
	}

	svc := NewCampaignService(repo, nil)
	got, err := svc.GetMyData(context.Background(), 7, 99)
	if err != nil {
		t.Fatalf("获取我的活动数据失败：%v", err)
	}
	if got.InviteCode != "AFF99" || got.InviteLink != "/register?aff=AFF99" {
		t.Fatalf("无参与记录时应返回当前用户邀请码和邀请链接，code=%q link=%q", got.InviteCode, got.InviteLink)
	}
	if repo.ensureAffiliateCalls != 1 {
		t.Fatalf("无参与记录时应确保用户邀请档案，calls=%d", repo.ensureAffiliateCalls)
	}
}

func TestCampaignGetMyDataKeepsParticipantInviteSnapshot(t *testing.T) {
	repo := &campaignMyDataRepoStub{
		campaign: &Campaign{ID: 7, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)},
		cfg:      campaignTestConfig(100),
		pool:     &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 100_000},
		participant: &CampaignParticipant{
			CampaignID:         7,
			UserID:             99,
			InviteCodeSnapshot: "SNAP99",
			InviteLinkSnapshot: "/register?aff=SNAP99",
		},
		affiliate: &AffiliateSummary{UserID: 99, AffCode: "AFF99"},
	}

	svc := NewCampaignService(repo, nil)
	got, err := svc.GetMyData(context.Background(), 7, 99)
	if err != nil {
		t.Fatalf("获取我的活动数据失败：%v", err)
	}
	if got.InviteCode != "SNAP99" || got.InviteLink != "/register?aff=SNAP99" {
		t.Fatalf("已有参与快照时应优先使用快照，code=%q link=%q", got.InviteCode, got.InviteLink)
	}
	if repo.ensureAffiliateCalls != 0 {
		t.Fatalf("已有完整参与快照时不应重复确保邀请档案，calls=%d", repo.ensureAffiliateCalls)
	}
}

func TestCampaignGetFinalRewardResultsSummarizesPersistedFinalResults(t *testing.T) {
	repo := &campaignFinalResultsRepoStub{
		cfg:  campaignTestConfig(100),
		pool: &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 100_000},
		results: []CampaignRewardResult{{
			ID:                     101,
			CampaignID:             7,
			CalculationBatchNo:     "final-1",
			UserID:                 21,
			GrossRewardAmountCents: 20_000,
			FinalPayoutAmountCents: 18_000,
			WithheldAmountCents:    2_000,
			RoundingResidualCents:  1,
			CalculationStatus:      CampaignCalculationFinal,
		}},
	}
	svc := NewCampaignService(repo, nil)

	got, err := svc.GetFinalRewardResults(context.Background(), 7)
	if err != nil {
		t.Fatalf("读取最终结算结果失败：%v", err)
	}
	if got.CalculationStatus != CampaignCalculationFinal || got.CalculationBatchNo != "final-1" {
		t.Fatalf("最终结算批次信息不正确：status=%s batch=%s", got.CalculationStatus, got.CalculationBatchNo)
	}
	if got.RankPoolCents != 80_000 || got.ContributionPoolCents != 20_000 {
		t.Fatalf("奖池拆分不正确：rank=%d contribution=%d", got.RankPoolCents, got.ContributionPoolCents)
	}
	if got.TotalFinalPayoutCents != 18_000 || got.TotalWithheldCents != 2_000 || got.TotalRoundingResidualCents != 1 {
		t.Fatalf("最终结算汇总不正确：payout=%d withheld=%d residual=%d", got.TotalFinalPayoutCents, got.TotalWithheldCents, got.TotalRoundingResidualCents)
	}
}

func TestCampaignGetFinalRewardResultsRejectsMissingFinalSettlement(t *testing.T) {
	repo := &campaignFinalResultsRepoStub{
		cfg:     campaignTestConfig(100),
		pool:    &CampaignPoolSummary{CampaignID: 7, FinalPoolCents: 100_000},
		results: nil,
	}
	svc := NewCampaignService(repo, nil)

	_, err := svc.GetFinalRewardResults(context.Background(), 7)
	if !errors.Is(err, ErrCampaignNoFinalSettlement) {
		t.Fatalf("无最终结算结果时应返回 ErrCampaignNoFinalSettlement，实际=%v", err)
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

type campaignMyDataRepoStub struct {
	CampaignRepository
	campaign              *Campaign
	cfg                   *CampaignConfigVersion
	pool                  *CampaignPoolSummary
	participant           *CampaignParticipant
	affiliate             *AffiliateSummary
	ensureAffiliateCalls  int
	topRows               []CampaignLeaderboardRow
	currentUserRow        CampaignLeaderboardRow
	loadedCurrentUserRank bool
}

func (r *campaignMyDataRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignMyDataRepoStub) ListLeaderboardRows(context.Context, int64, int) ([]CampaignLeaderboardRow, error) {
	return append([]CampaignLeaderboardRow(nil), r.topRows...), nil
}

func (r *campaignMyDataRepoStub) GetPoolSummary(context.Context, int64) (*CampaignPoolSummary, error) {
	return r.pool, nil
}

func (r *campaignMyDataRepoStub) GetPublishedConfigVersion(context.Context, int64) (*CampaignConfigVersion, error) {
	return r.cfg, nil
}

func (r *campaignMyDataRepoStub) ListInviteRecords(context.Context, int64, int64, int, int) ([]CampaignInviteRecord, int64, error) {
	return nil, 0, nil
}

func (r *campaignMyDataRepoStub) GetParticipantStats(context.Context, int64, int64) (*CampaignParticipant, error) {
	if r.participant != nil {
		return r.participant, nil
	}
	return nil, ErrCampaignNotFound
}

func (r *campaignMyDataRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	r.ensureAffiliateCalls++
	if r.affiliate != nil {
		return r.affiliate, nil
	}
	return nil, ErrAffiliateProfileNotFound
}

func (r *campaignMyDataRepoStub) GetLeaderboardRowForUser(context.Context, int64, int64) (*CampaignLeaderboardRow, error) {
	r.loadedCurrentUserRank = true
	return &r.currentUserRow, nil
}

type campaignFinalResultsRepoStub struct {
	CampaignRepository
	cfg     *CampaignConfigVersion
	pool    *CampaignPoolSummary
	results []CampaignRewardResult
}

func (r *campaignFinalResultsRepoStub) GetPoolSummary(context.Context, int64) (*CampaignPoolSummary, error) {
	return r.pool, nil
}

func (r *campaignFinalResultsRepoStub) GetPublishedConfigVersion(context.Context, int64) (*CampaignConfigVersion, error) {
	return r.cfg, nil
}

func (r *campaignFinalResultsRepoStub) ListRewardResults(context.Context, int64, string) ([]CampaignRewardResult, error) {
	return append([]CampaignRewardResult(nil), r.results...), nil
}

type campaignPayoutRepoStub struct {
	CampaignRepository
	results             []CampaignRewardResult
	existingResultBatch *CampaignPayoutBatch
}

type campaignRechargeRepoStub struct {
	CampaignRepository
	campaign           *Campaign
	cfg                *CampaignConfigVersion
	recordRechargeErr  error
	insertPoolCalled   bool
	insertedInvite     *CampaignInviteRecord
	insertedPoolAmount int64
}

func (r *campaignRechargeRepoStub) GetActiveCampaign(context.Context, time.Time) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignRechargeRepoStub) GetLatestConfigVersionAt(context.Context, int64, time.Time) (*CampaignConfigVersion, error) {
	if r.cfg == nil {
		return nil, ErrCampaignNotFound
	}
	return r.cfg, nil
}

func (r *campaignRechargeRepoStub) RecordRecharge(context.Context, *Campaign, *CampaignConfigVersion, CampaignRechargeInput, int64) (*CampaignInviteRecord, bool, error) {
	if r.recordRechargeErr != nil {
		return nil, false, r.recordRechargeErr
	}
	return &CampaignInviteRecord{ID: 31}, true, nil
}

func (r *campaignRechargeRepoStub) InsertPoolEntry(_ context.Context, _ *Campaign, _ *CampaignConfigVersion, invite *CampaignInviteRecord, _ CampaignRechargeInput, poolAmountCents int64) (bool, error) {
	r.insertPoolCalled = true
	r.insertedInvite = invite
	r.insertedPoolAmount = poolAmountCents
	return true, nil
}

type campaignManualAdjustmentRepoStub struct {
	CampaignRepository
	campaign              *Campaign
	successfulPayout      *CampaignPayoutBatch
	added                 *CampaignLeaderboardAdjustmentInput
	addedPool             *CampaignPoolAdjustmentInput
	adjustedInvite        *CampaignInviteRecordAdjustmentInput
	invalidatedCampaignID int64
	results               []CampaignRewardResult
}

func (r *campaignManualAdjustmentRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignManualAdjustmentRepoStub) GetSuccessfulPayoutBatch(context.Context, int64) (*CampaignPayoutBatch, error) {
	if r.successfulPayout != nil {
		return r.successfulPayout, nil
	}
	return nil, ErrCampaignNotFound
}

func (r *campaignManualAdjustmentRepoStub) AddLeaderboardAdjustment(_ context.Context, campaignID int64, input CampaignLeaderboardAdjustmentInput) (*CampaignManualLeaderboardAdjustment, error) {
	r.added = &input
	r.invalidatedCampaignID = campaignID
	return &CampaignManualLeaderboardAdjustment{
		ID:                       1,
		CampaignID:               campaignID,
		UserID:                   input.UserID,
		AdjustmentType:           "manual_delta",
		ValidInviteDelta:         input.ValidInviteDelta,
		RechargeAmountDeltaCents: input.RechargeAmountDeltaCents,
	}, nil
}

func (r *campaignManualAdjustmentRepoStub) AddPoolAdjustment(_ context.Context, campaignID int64, input CampaignPoolAdjustmentInput) error {
	r.addedPool = &input
	r.invalidatedCampaignID = campaignID
	return nil
}

func (r *campaignManualAdjustmentRepoStub) AdjustInviteRecord(_ context.Context, campaignID int64, input CampaignInviteRecordAdjustmentInput) (*CampaignInviteRecord, error) {
	r.adjustedInvite = &input
	r.invalidatedCampaignID = campaignID
	return &CampaignInviteRecord{ID: input.RecordID, CampaignID: campaignID, Status: input.Status}, nil
}

func (r *campaignManualAdjustmentRepoStub) ListRewardResults(context.Context, int64, string) ([]CampaignRewardResult, error) {
	return append([]CampaignRewardResult(nil), r.results...), nil
}

func (r *campaignManualAdjustmentRepoStub) GetPayoutBatchForRewardResult(context.Context, int64, int64) (*CampaignPayoutBatch, error) {
	return nil, ErrCampaignNotFound
}

func (r *campaignPayoutRepoStub) GetSuccessfulPayoutBatch(context.Context, int64) (*CampaignPayoutBatch, error) {
	return nil, ErrCampaignNotFound
}

func (r *campaignPayoutRepoStub) ListRewardResults(context.Context, int64, string) ([]CampaignRewardResult, error) {
	return r.results, nil
}

func (r *campaignPayoutRepoStub) GetPayoutBatchForRewardResult(context.Context, int64, int64) (*CampaignPayoutBatch, error) {
	if r.existingResultBatch != nil {
		return r.existingResultBatch, nil
	}
	return nil, ErrCampaignNotFound
}

type campaignDeleteRepoStub struct {
	CampaignRepository
	campaign        *Campaign
	impact          CampaignDeleteImpact
	deleted         bool
	updatedStatus   string
	updatedOperator *int64
}

func (r *campaignDeleteRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignDeleteRepoStub) GetCampaignDeleteImpact(context.Context, int64) (*CampaignDeleteImpact, error) {
	return &r.impact, nil
}

func (r *campaignDeleteRepoStub) DeleteCampaign(context.Context, int64) error {
	r.deleted = true
	return nil
}

func (r *campaignDeleteRepoStub) UpdateCampaignStatus(_ context.Context, _ int64, status string, operatorID *int64) (*Campaign, error) {
	r.updatedStatus = status
	r.updatedOperator = operatorID
	updated := *r.campaign
	updated.Status = status
	r.campaign = &updated
	return &updated, nil
}

type campaignStatusRepoStub struct {
	CampaignRepository
	campaign         *Campaign
	cfg              *CampaignConfigVersion
	pool             *CampaignPoolSummary
	rows             []CampaignLeaderboardRow
	finalResults     []CampaignRewardResult
	payoutBatch      *CampaignPayoutBatch
	hasOtherActive   bool
	hasActiveChecked bool
	snapshotSaved    bool
	snapshotType     string
	updatedStatus    string
	updatedOperator  *int64
	updateStatusErr  error
}

func (r *campaignStatusRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignStatusRepoStub) ListLeaderboardRows(context.Context, int64, int) ([]CampaignLeaderboardRow, error) {
	return append([]CampaignLeaderboardRow(nil), r.rows...), nil
}

func (r *campaignStatusRepoStub) GetPoolSummary(context.Context, int64) (*CampaignPoolSummary, error) {
	if r.pool == nil {
		return &CampaignPoolSummary{CampaignID: r.campaign.ID}, nil
	}
	return r.pool, nil
}

func (r *campaignStatusRepoStub) GetPublishedConfigVersion(context.Context, int64) (*CampaignConfigVersion, error) {
	if r.cfg == nil {
		return nil, ErrCampaignNotFound
	}
	return r.cfg, nil
}

func (r *campaignStatusRepoStub) GetLatestConfigVersionAt(context.Context, int64, time.Time) (*CampaignConfigVersion, error) {
	if r.cfg == nil {
		return nil, ErrCampaignNotFound
	}
	return r.cfg, nil
}

func (r *campaignStatusRepoStub) SaveLeaderboardSnapshot(_ context.Context, _ int64, snapshotType string, _ []CampaignLeaderboardRow) error {
	r.snapshotSaved = true
	r.snapshotType = snapshotType
	return nil
}

func (r *campaignStatusRepoStub) UpdateCampaignStatus(_ context.Context, _ int64, status string, operatorID *int64) (*Campaign, error) {
	if r.updateStatusErr != nil && status == CampaignStatusActive {
		return nil, r.updateStatusErr
	}
	r.updatedStatus = status
	r.updatedOperator = operatorID
	updated := *r.campaign
	updated.Status = status
	r.campaign = &updated
	return &updated, nil
}

func (r *campaignStatusRepoStub) HasActiveCampaign(context.Context, int64) (bool, error) {
	r.hasActiveChecked = true
	return r.hasOtherActive, nil
}

func (r *campaignStatusRepoStub) ListRewardResults(context.Context, int64, string) ([]CampaignRewardResult, error) {
	return append([]CampaignRewardResult(nil), r.finalResults...), nil
}

func (r *campaignStatusRepoStub) GetSuccessfulPayoutBatch(context.Context, int64) (*CampaignPayoutBatch, error) {
	if r.payoutBatch != nil {
		return r.payoutBatch, nil
	}
	return nil, ErrCampaignNotFound
}

type campaignCopyRepoStub struct {
	CampaignRepository
	source                  *Campaign
	cfg                     *CampaignConfigVersion
	createdInput            CampaignCreateInput
	addPoolAdjustmentCalled bool
}

func (r *campaignCopyRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.source == nil {
		return nil, ErrCampaignNotFound
	}
	return r.source, nil
}

func (r *campaignCopyRepoStub) GetLatestConfigVersion(context.Context, int64) (*CampaignConfigVersion, error) {
	if r.cfg == nil {
		return nil, ErrCampaignNotFound
	}
	return r.cfg, nil
}

func (r *campaignCopyRepoStub) CreateCampaign(_ context.Context, input CampaignCreateInput, cfg CampaignConfigVersion) (*Campaign, *CampaignConfigVersion, error) {
	r.createdInput = input
	campaign := &Campaign{
		ID:            77,
		Name:          input.Name,
		Description:   input.Description,
		CoverURL:      input.CoverURL,
		RulesText:     input.RulesText,
		Status:        CampaignStatusDraft,
		WarmupStartAt: input.WarmupStartAt,
		StartAt:       input.StartAt,
		EndAt:         input.EndAt,
		AuditStartAt:  input.AuditStartAt,
		AuditEndAt:    input.AuditEndAt,
		CreatedBy:     input.OperatorID,
		UpdatedBy:     input.OperatorID,
	}
	cfg.ID = 88
	cfg.CampaignID = campaign.ID
	return campaign, &cfg, nil
}

func (r *campaignCopyRepoStub) AddPoolAdjustment(context.Context, int64, CampaignPoolAdjustmentInput) error {
	r.addPoolAdjustmentCalled = true
	return nil
}

type campaignUpdateRepoStub struct {
	CampaignRepository
	campaign     *Campaign
	updatedInput CampaignUpdateInput
	updateCalled bool
}

func (r *campaignUpdateRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignUpdateRepoStub) UpdateCampaign(_ context.Context, _ int64, input CampaignUpdateInput) (*Campaign, error) {
	r.updateCalled = true
	r.updatedInput = input
	updated := *r.campaign
	if input.Name != nil {
		updated.Name = *input.Name
	}
	if input.EndAt != nil {
		updated.EndAt = *input.EndAt
	}
	r.campaign = &updated
	return &updated, nil
}

type campaignRecalculateRepoStub struct {
	CampaignRepository
	campaign *Campaign
	saved    bool
}

func (r *campaignRecalculateRepoStub) GetCampaign(context.Context, int64) (*Campaign, error) {
	if r.campaign == nil {
		return nil, ErrCampaignNotFound
	}
	return r.campaign, nil
}

func (r *campaignRecalculateRepoStub) SaveRewardResults(context.Context, int64, string, string, []CampaignRewardResult) error {
	r.saved = true
	return nil
}

type campaignGrantStub struct{}

func (campaignGrantStub) ListUsers(context.Context, int, int, UserListFilters, string, string) ([]User, int64, error) {
	return nil, 0, nil
}

func (campaignGrantStub) GetUser(context.Context, int64) (*User, error) { return nil, nil }
func (campaignGrantStub) GetUserIncludeDeleted(context.Context, int64) (*User, error) {
	return nil, nil
}
func (campaignGrantStub) CreateUser(context.Context, *CreateUserInput) (*User, error) {
	return nil, nil
}
func (campaignGrantStub) UpdateUser(context.Context, int64, *UpdateUserInput) (*User, error) {
	return nil, nil
}
func (campaignGrantStub) DeleteUser(context.Context, int64) error { return nil }
func (campaignGrantStub) GrantUserBalances(context.Context, []BalanceGrantInput, string) ([]BalanceGrantResult, error) {
	return nil, nil
}

func campaignTestLeaderboardRow(userID int64, validInvites int, rechargeCents int64) CampaignLeaderboardRow {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	return CampaignLeaderboardRow{
		UserID:                     userID,
		ValidInviteCount:           float64(validInvites),
		PendingInviteCount:         1,
		InviteeRechargeAmountCents: rechargeCents,
		ReachedCountAt:             now,
		JoinedAt:                   now.Add(-time.Hour),
	}
}

func campaignPtrInt64(value int64) *int64 {
	return &value
}

func campaignPtrString(value string) *string {
	return &value
}

func campaignPtrTime(value time.Time) *time.Time {
	return &value
}

func ptrTimePatch(value time.Time) **time.Time {
	ptr := &value
	return &ptr
}
