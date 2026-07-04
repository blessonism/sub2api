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
			CreatedBy:     ptrInt64(1),
			UpdatedBy:     ptrInt64(2),
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
		t.Fatalf("补位排名行应同步我的有效邀请与充值金额，valid=%d recharge=%d", got.ValidInviteCount, got.InviteeRechargeAmountCents)
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
	return nil, ErrCampaignNotFound
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
		ValidInviteCount:           validInvites,
		InviteeRechargeAmountCents: rechargeCents,
		ReachedCountAt:             now,
		JoinedAt:                   now.Add(-time.Hour),
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}
