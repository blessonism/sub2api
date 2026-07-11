package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestGetActiveLotteryCampaignPrefersFeaturedCampaign(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode,
	entry_mode, usage_mode, threshold_tokens, entry_step_tokens, threshold_cost_microusd, entry_step_cost_microusd, max_entries_per_user, start_at, end_at, draw_at,
	daily_draw_time, created_by, updated_by, created_at, updated_at, is_featured
FROM lottery_campaigns
WHERE status = 'published'
  AND start_at <= $1
  AND end_at >= $1
ORDER BY is_featured DESC, start_at ASC, id ASC
LIMIT 1`)).
		WithArgs(now).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "rules_text", "status", "participation_mode", "draw_schedule_type",
			"prize_mode", "entry_mode", "usage_mode", "threshold_tokens", "entry_step_tokens", "threshold_cost_microusd", "entry_step_cost_microusd", "max_entries_per_user",
			"start_at", "end_at", "draw_at", "daily_draw_time", "created_by", "updated_by",
			"created_at", "updated_at", "is_featured",
		}).AddRow(
			int64(7), "Token 抽奖", "", "", "published", "auto", "single", "single", "daily_once", "token",
			int64(100), int64(0), int64(0), int64(0), 1, now.Add(-time.Hour), now.Add(time.Hour), now.Add(30*time.Minute),
			"", nil, nil, now.Add(-2*time.Hour), now.Add(-time.Hour), true,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, campaign_id, tier_name, winner_count, reward_amount_cents, sort_order, created_at
FROM lottery_prize_tiers
WHERE campaign_id = $1 AND is_active = TRUE
ORDER BY sort_order ASC, id ASC`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "campaign_id", "tier_name", "winner_count", "reward_amount_cents", "sort_order", "created_at"}).
			AddRow(int64(1), int64(7), "一等奖", 1, int64(1000), 1, now))

	campaign, err := repo.GetActiveLotteryCampaign(context.Background(), now)
	if err != nil {
		t.Fatalf("get active lottery campaign: %v", err)
	}
	if campaign == nil || !campaign.IsFeatured {
		t.Fatalf("campaign = %+v, want featured", campaign)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpsertLotteryEntryPersistsUSDUsageWithTypedStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	entryDate := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	now := entryDate.Add(time.Hour)
	mock.ExpectQuery("INSERT INTO lottery_entries").
		WithArgs(int64(7), int64(42), entryDate, int64(0), int64(1_250_000), 2, "enrolled").
		WillReturnRows(sqlmock.NewRows([]string{"id", "campaign_id", "user_id", "entry_date", "tokens", "cost_microusd", "entry_count", "status", "enrolled_at", "created_at", "updated_at"}).
			AddRow(int64(9), int64(7), int64(42), entryDate, int64(0), int64(1_250_000), 2, "enrolled", now, now, now))

	entry, err := repo.UpsertLotteryEntry(context.Background(), service.LotteryCampaign{ID: 7}, 42, entryDate, service.LotteryUsage{CostMicrousd: 1_250_000}, 2, true)
	if err != nil {
		t.Fatalf("upsert lottery entry: %v", err)
	}
	if entry.CostMicrousd != 1_250_000 || entry.EntryCount != 2 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLotteryCampaignRejectsUsageModeChangeAfterDraw(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewLotteryCampaignRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT usage_mode FROM lottery_campaigns").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"usage_mode"}).AddRow("token"))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	_, err = repo.UpdateLotteryCampaign(context.Background(), 7, service.LotteryCampaignInput{UsageMode: service.LotteryUsageUSD})
	if !errors.Is(err, service.ErrLotteryUsageModeLocked) {
		t.Fatalf("error = %v, want usage mode locked", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGrantLotteryWinnerBalanceCommitsUserBalanceAndWinnerAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT w.user_id, w.reward_amount_cents, w.status,
	w.balance_before_snapshot, w.balance_after_snapshot, u.balance
FROM lottery_winners w
JOIN users u ON u.id = w.user_id
WHERE w.id = $1
FOR UPDATE OF w, u`)).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "reward_amount_cents", "status", "balance_before_snapshot", "balance_after_snapshot", "balance"}).
			AddRow(int64(42), int64(1250), "pending", nil, nil, 10.0))
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE users
SET balance = $2, updated_at = NOW()
WHERE id = $1`)).
		WithArgs(int64(42), 22.5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO redeem_codes (code, type, value, status, used_by, used_at, notes, created_at)
VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())`)).
		WithArgs(sqlmock.AnyArg(), "admin_balance", 12.5, "used", int64(42), "lottery payout").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE lottery_winners
SET status = 'success', balance_before_snapshot = $2, balance_after_snapshot = $3,
	error_message = '', processed_at = NOW()
WHERE id = $1`)).
		WithArgs(int64(99), 10.0, 22.5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, claimed, err := repo.GrantLotteryWinnerBalance(context.Background(), 99, "lottery payout")
	if err != nil {
		t.Fatalf("grant lottery winner balance: %v", err)
	}
	if !claimed {
		t.Fatalf("claimed = false, want true")
	}
	if result == nil || result.UserID != 42 || result.BalanceBefore != 10 || result.BalanceAfter != 22.5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGrantLotteryWinnerBalanceSkipsAlreadySuccessfulWinner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("FOR UPDATE OF w, u").
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "reward_amount_cents", "status", "balance_before_snapshot", "balance_after_snapshot", "balance"}).
			AddRow(int64(42), int64(1250), "success", 10.0, 22.5, 22.5))
	mock.ExpectRollback()

	result, claimed, err := repo.GrantLotteryWinnerBalance(context.Background(), 99, "lottery payout")
	if err != nil {
		t.Fatalf("grant lottery winner balance: %v", err)
	}
	if claimed {
		t.Fatalf("claimed = true, want false")
	}
	if result == nil || result.BalanceBefore != 10 || result.BalanceAfter != 22.5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
