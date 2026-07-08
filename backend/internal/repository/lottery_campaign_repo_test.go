package repository

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

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
