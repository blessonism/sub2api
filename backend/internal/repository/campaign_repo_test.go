package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

func TestCampaignRepositoryRecordRechargeIsSourceIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	startAt := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	successAt := startAt.Add(time.Hour)
	campaign := &service.Campaign{ID: 7, StartAt: startAt, EndAt: startAt.Add(7 * 24 * time.Hour)}
	cfg := &service.CampaignConfigVersion{ID: 11, PoolInjectionRate: decimal.RequireFromString("0.10")}
	input := service.CampaignRechargeInput{
		InviteeUserID:       22,
		SourceType:          "redeem_code",
		SourceID:            "redeem-1",
		SourceSuccessAt:     successAt,
		RechargeAmountCents: 5_000,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FROM campaign_invite_records")).
		WithArgs(campaign.ID, input.InviteeUserID, campaign.StartAt, campaign.EndAt).
		WillReturnRows(campaignInviteRows().AddRow(
			int64(31), campaign.ID, cfg.ID, int64(21), input.InviteeUserID, "affiliate_code",
			int64(2_000), startAt, nil, int64(0), "registered", "low", "", "not_reviewed", nil, nil, "",
		))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO campaign_pool_entries")).
		WithArgs(campaign.ID, cfg.ID, int64(31), input.InviteeUserID, input.SourceType, input.SourceID, input.SourceSuccessAt, input.RechargeAmountCents, cfg.PoolInjectionRate.String(), int64(500)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE campaign_invite_records")).
		WithArgs(campaign.ID, input.InviteeUserID, input.RechargeAmountCents, input.SourceSuccessAt, campaign.StartAt, campaign.EndAt).
		WillReturnRows(campaignInviteRows().AddRow(
			int64(31), campaign.ID, cfg.ID, int64(21), input.InviteeUserID, "affiliate_code",
			int64(2_000), startAt, successAt, int64(5_000), "effective", "low", "", "approved", nil, nil, "",
		))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE campaign_participants")).
		WithArgs(campaign.ID, int64(21)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	first, inserted, err := repo.RecordRecharge(ctx, campaign, cfg, input, 500)
	if err != nil {
		t.Fatalf("first record recharge: %v", err)
	}
	if !inserted || first.EffectiveRechargeAmountCents != 5_000 {
		t.Fatalf("首次记录应写入并累加充值金额，inserted=%v amount=%d", inserted, first.EffectiveRechargeAmountCents)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FROM campaign_invite_records")).
		WithArgs(campaign.ID, input.InviteeUserID, campaign.StartAt, campaign.EndAt).
		WillReturnRows(campaignInviteRows().AddRow(
			int64(31), campaign.ID, cfg.ID, int64(21), input.InviteeUserID, "affiliate_code",
			int64(2_000), startAt, successAt, int64(5_000), "effective", "low", "", "approved", nil, nil, "",
		))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO campaign_pool_entries")).
		WithArgs(campaign.ID, cfg.ID, int64(31), input.InviteeUserID, input.SourceType, input.SourceID, input.SourceSuccessAt, input.RechargeAmountCents, cfg.PoolInjectionRate.String(), int64(500)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	second, inserted, err := repo.RecordRecharge(ctx, campaign, cfg, input, 500)
	if err != nil {
		t.Fatalf("duplicate record recharge: %v", err)
	}
	if inserted || second.EffectiveRechargeAmountCents != 5_000 {
		t.Fatalf("重复 source 不应再次累加，inserted=%v amount=%d", inserted, second.EffectiveRechargeAmountCents)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryInsertPoolEntryAllowsNilInviteRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	successAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	campaign := &service.Campaign{ID: 7}
	cfg := &service.CampaignConfigVersion{ID: 11, PoolInjectionRate: decimal.RequireFromString("0.10")}
	input := service.CampaignRechargeInput{
		InviteeUserID:       99,
		SourceType:          "payment_order",
		SourceID:            "order-1",
		SourceSuccessAt:     successAt,
		RechargeAmountCents: 5_000,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO campaign_pool_entries")).
		WithArgs(campaign.ID, cfg.ID, nil, input.InviteeUserID, input.SourceType, input.SourceID, input.SourceSuccessAt, input.RechargeAmountCents, cfg.PoolInjectionRate.String(), int64(500)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	inserted, err := repo.InsertPoolEntry(ctx, campaign, cfg, nil, input, 500)
	if err != nil {
		t.Fatalf("写入非邀请充值奖池流水失败：%v", err)
	}
	if !inserted {
		t.Fatalf("首次非邀请充值奖池流水应写入成功")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryAddLeaderboardAdjustmentRollsBackWhenFinalInvalidationFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	input := service.CampaignLeaderboardAdjustmentInput{
		UserID:                   22,
		ValidInviteDelta:         1,
		RechargeAmountDeltaCents: 5_000,
		Reason:                   "补录邀请",
	}
	invalidateErr := errors.New("delete final failed")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO campaign_leaderboard_adjustments")).
		WithArgs(int64(7), input.UserID, input.ValidInviteDelta, input.RechargeAmountDeltaCents, input.Reason, nil).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "campaign_id", "user_id", "adjustment_type", "valid_invite_delta", "recharge_amount_delta_cents", "reason", "operator_id", "created_at",
		}).AddRow(int64(41), int64(7), input.UserID, "manual_delta", input.ValidInviteDelta, input.RechargeAmountDeltaCents, input.Reason, nil, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_reward_adjustments")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_reward_results")).
		WithArgs(int64(7)).
		WillReturnError(invalidateErr)
	mock.ExpectRollback()

	_, err = repo.AddLeaderboardAdjustment(ctx, 7, input)
	if !errors.Is(err, invalidateErr) {
		t.Fatalf("final 失效失败应回滚并返回错误，实际 err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryAddPoolAdjustmentInvalidatesFinalSettlement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	input := service.CampaignPoolAdjustmentInput{
		AdjustmentType: "additional_bonus",
		AmountCents:    10_000,
		Reason:         "补充活动奖池",
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO campaign_pool_adjustments")).
		WithArgs(int64(7), input.AdjustmentType, input.AmountCents, input.Reason, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_reward_adjustments")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_reward_results")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_leaderboard_snapshots")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.AddPoolAdjustment(ctx, 7, input); err != nil {
		t.Fatalf("奖池调整应失效 final 并提交事务：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryAddPoolAdjustmentRollsBackWhenFinalInvalidationFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	input := service.CampaignPoolAdjustmentInput{
		AdjustmentType: "exception_deduction",
		AmountCents:    -1_000,
		Reason:         "异常扣减",
	}
	invalidateErr := errors.New("delete final failed")

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO campaign_pool_adjustments")).
		WithArgs(int64(7), input.AdjustmentType, input.AmountCents, input.Reason, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaign_reward_adjustments")).
		WithArgs(int64(7)).
		WillReturnError(invalidateErr)
	mock.ExpectRollback()

	err = repo.AddPoolAdjustment(ctx, 7, input)
	if !errors.Is(err, invalidateErr) {
		t.Fatalf("final 失效失败应回滚奖池调整，实际 err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryGetCampaignDeleteImpactCountsDependencies(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	campaignID := int64(7)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT\n\t(SELECT COUNT(*) FROM campaign_participants WHERE campaign_id = $1)")).
		WithArgs(campaignID).
		WillReturnRows(sqlmock.NewRows([]string{
			"participants", "invite_records", "pool_entries", "pool_adjustments",
			"leaderboard_snapshots", "reward_results", "payout_batches", "payout_items",
		}).AddRow(int64(1), int64(2), int64(3), int64(4), int64(5), int64(6), int64(7), int64(8)))

	impact, err := repo.GetCampaignDeleteImpact(ctx, campaignID)
	if err != nil {
		t.Fatalf("统计删除影响失败：%v", err)
	}
	if impact.Participants != 1 || impact.InviteRecords != 2 || impact.PayoutItems != 8 {
		t.Fatalf("删除影响统计不符合预期：%+v", impact)
	}
	if !impact.HasBusinessData() {
		t.Fatalf("存在依赖数据时应识别为有业务数据")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryDeleteCampaignDeletesRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	campaignID := int64(7)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM campaigns WHERE id = $1")).
		WithArgs(campaignID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.DeleteCampaign(ctx, campaignID); err != nil {
		t.Fatalf("删除活动失败：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCampaignRepositoryListLeaderboardRowsScansPendingInviteCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewCampaignRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("WITH invite_base AS (")).
		WithArgs(int64(7), 50).
		WillReturnRows(campaignLeaderboardRows().AddRow(
			1, int64(21), "alpha@example.com", "alpha", 3, 2, int64(8_000),
			0, int64(0), false, now, now.Add(-time.Hour), int64(0), int64(0),
		))

	rows, err := repo.ListLeaderboardRows(ctx, 7, 50)
	if err != nil {
		t.Fatalf("读取活动排行榜失败：%v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("排行榜行数=%d", len(rows))
	}
	if rows[0].PendingInviteCount != 2 {
		t.Fatalf("待充值邀请数未正确扫描，实际=%d", rows[0].PendingInviteCount)
	}
	if rows[0].MaskedEmail != "a***@example.com" {
		t.Fatalf("用户侧邮箱脱敏不符合预期：%q", rows[0].MaskedEmail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func campaignInviteRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "campaign_id", "config_version_id", "inviter_user_id", "invitee_user_id", "invite_source",
		"threshold_snapshot_cents", "registered_at", "qualified_at", "effective_recharge_amount_cents",
		"status", "risk_level", "invalid_reason", "audit_status", "audit_by", "audit_at", "audit_note",
	})
}

func campaignLeaderboardRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"rank", "user_id", "email", "username", "valid_invite_count", "pending_invite_count",
		"recharge_amount", "manual_valid_invite_delta", "manual_recharge_delta", "has_manual_adjustment",
		"reached_count_at", "joined_at", "estimated_reward_cents", "final_reward_cents",
	})
}
