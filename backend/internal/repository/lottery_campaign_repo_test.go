package repository

import (
	"context"
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
	entry_mode, threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at,
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
			"prize_mode", "entry_mode", "threshold_tokens", "entry_step_tokens", "max_entries_per_user",
			"start_at", "end_at", "draw_at", "daily_draw_time", "created_by", "updated_by",
			"created_at", "updated_at", "is_featured",
		}).AddRow(
			int64(7), "Token 抽奖", "", "", "published", "auto", "single", "single", "daily_once",
			int64(100), int64(0), 1, now.Add(-time.Hour), now.Add(time.Hour), now.Add(30*time.Minute),
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

func TestUpsertLotteryEntryUsesExplicitStatusParameterType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	entryDate := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO lottery_entries (campaign_id, user_id, entry_date, tokens, entry_count, status, enrolled_at, created_at, updated_at)
VALUES ($1, $2, $3::date, $4, $5, $6::varchar, CASE WHEN $6::varchar = 'enrolled' THEN NOW() ELSE NULL END, NOW(), NOW())
ON CONFLICT (campaign_id, user_id, entry_date) DO UPDATE
SET tokens = EXCLUDED.tokens,
	entry_count = EXCLUDED.entry_count,
	status = CASE WHEN lottery_entries.status = 'enrolled' THEN 'enrolled' ELSE EXCLUDED.status END,
	enrolled_at = CASE
		WHEN lottery_entries.enrolled_at IS NOT NULL THEN lottery_entries.enrolled_at
		WHEN EXCLUDED.status = 'enrolled' THEN NOW()
		ELSE NULL
	END,
	updated_at = NOW()
RETURNING id, campaign_id, user_id, entry_date, tokens, entry_count, status, enrolled_at, created_at, updated_at`)).
		WithArgs(int64(7), int64(42), entryDate, int64(2_000_000), 2, "enrolled").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "campaign_id", "user_id", "entry_date", "tokens", "entry_count", "status", "enrolled_at", "created_at", "updated_at",
		}).AddRow(int64(9), int64(7), int64(42), entryDate, int64(2_000_000), 2, "enrolled", now, now, now))

	entry, err := repo.UpsertLotteryEntry(context.Background(), service.LotteryCampaign{ID: 7}, 42, entryDate, 2_000_000, 2, true)
	if err != nil {
		t.Fatalf("upsert lottery entry: %v", err)
	}
	if entry.Status != "enrolled" || entry.EntryCount != 2 {
		t.Fatalf("entry = %+v, want enrolled entry with two chances", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCountLotteryParticipantsCountsOnlyEnrolledUsersInCurrentDraw(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewLotteryCampaignRepository(db)
	entryDate := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT COUNT(DISTINCT user_id)
FROM lottery_entries
WHERE campaign_id = $1 AND entry_date = $2::date AND status = 'enrolled'`)).
		WithArgs(int64(7), entryDate).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

	count, err := repo.CountLotteryParticipants(context.Background(), 7, entryDate)
	if err != nil {
		t.Fatalf("count lottery participants: %v", err)
	}
	if count != 3 {
		t.Fatalf("participant count = %d, want 3", count)
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
