package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateAdminUsageCalibrationRejectsTokenTargetWithoutOriginalUsage(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}

	input := service.AdminUsageCalibrationCreateInput{
		TargetUserID: 42,
		AdminUserID:  1,
		Reason:       "校准历史统计",
		Token: &service.AdminUsageTokenCalibrationInput{
			Mode:      service.AdminUsageCalibrationModeTarget,
			Value:     1000,
			StartDate: "2026-06-01",
			EndDate:   "2026-06-02",
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectQuery("TO_CHAR\\(\\(created_at AT TIME ZONE").
		WithArgs(int64(42), "2026-06-01", "2026-06-03", "UTC").
		WillReturnRows(sqlmock.NewRows([]string{"allocation_date", "tokens"}))
	mock.ExpectRollback()

	got, err := repo.CreateAdminUsageCalibration(context.Background(), input)
	require.Nil(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无法按比例分摊")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAdminUsageCalibrationRejectsNegativeBalanceWithoutAuditInsert(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}

	input := service.AdminUsageCalibrationCreateInput{
		TargetUserID: 42,
		AdminUserID:  1,
		Reason:       "扣回余额",
		Balance: &service.AdminUsageBalanceCalibrationInput{
			Mode:  service.AdminUsageCalibrationModeDelta,
			Value: -20,
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectRollback()

	got, err := repo.CreateAdminUsageCalibration(context.Background(), input)
	require.Nil(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminUsageCalibrationSumBalanceSpentUsesOnlyNegativeDelta(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}
	start := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(-balance_delta\\), 0\\) FROM admin_usage_calibrations WHERE balance_delta < 0 AND target_user_id = \\$1 AND created_at >= \\$2 AND created_at < \\$3").
		WithArgs(int64(42), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3.75))

	got, err := repo.SumBalanceSpent(context.Background(), 42, start, end)

	require.NoError(t, err)
	require.InDelta(t, 3.75, got, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAllocateTokenDeltaKeepsExactTotalWithDeterministicRemainder(t *testing.T) {
	rows := []rawDailyTokenRow{
		{Date: "2026-06-01", Tokens: 100},
		{Date: "2026-06-02", Tokens: 300},
		{Date: "2026-06-03", Tokens: 600},
	}

	got := allocateTokenDelta(rows, 1000, 101)

	require.Equal(t, []allocationPlanRow{
		{Date: "2026-06-01", OriginalTokens: 100, TokenDelta: 10},
		{Date: "2026-06-02", OriginalTokens: 300, TokenDelta: 30},
		{Date: "2026-06-03", OriginalTokens: 600, TokenDelta: 61},
	}, got)
	require.Equal(t, int64(101), sumAllocationTokenDelta(got))
}

func TestAllocateTokenDeltaKeepsExactTotalForNegativeDelta(t *testing.T) {
	rows := []rawDailyTokenRow{
		{Date: "2026-06-01", Tokens: 1},
		{Date: "2026-06-02", Tokens: 1},
		{Date: "2026-06-03", Tokens: 1},
	}

	got := allocateTokenDelta(rows, 3, -2)

	require.Equal(t, []allocationPlanRow{
		{Date: "2026-06-01", OriginalTokens: 1, TokenDelta: -1},
		{Date: "2026-06-02", OriginalTokens: 1, TokenDelta: -1},
	}, got)
	require.Equal(t, int64(-2), sumAllocationTokenDelta(got))
}

func sumAllocationTokenDelta(rows []allocationPlanRow) int64 {
	var total int64
	for _, row := range rows {
		total += row.TokenDelta
	}
	return total
}
