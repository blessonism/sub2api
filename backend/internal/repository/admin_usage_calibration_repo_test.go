package repository

import (
	"context"
	"math"
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

func TestCreateAdminUsageCalibrationAllocatesBalanceAcrossTokenDates(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}
	createdAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	input := service.AdminUsageCalibrationCreateInput{
		TargetUserID: 42,
		AdminUserID:  1,
		Token: &service.AdminUsageTokenCalibrationInput{
			Mode:      service.AdminUsageCalibrationModeDelta,
			Value:     100,
			StartDate: "2026-06-01",
			EndDate:   "2026-06-02",
		},
		Balance: &service.AdminUsageBalanceCalibrationInput{
			Mode:  service.AdminUsageCalibrationModeDelta,
			Value: -4,
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectQuery("TO_CHAR\\(\\(created_at AT TIME ZONE").
		WithArgs(int64(42), "2026-06-01", "2026-06-03", "UTC").
		WillReturnRows(sqlmock.NewRows([]string{"allocation_date", "tokens"}).
			AddRow("2026-06-01", int64(100)).
			AddRow("2026-06-02", int64(300)))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(token_delta\\), 0\\) FROM admin_usage_calibration_daily_allocations").
		WithArgs(int64(42), "2026-06-01", "2026-06-03").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(0)))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibrations .*RETURNING").
		WithArgs(int64(42), int64(1), "", "delta", int64(100), int64(400), int64(500), int64(100), "2026-06-01", "2026-06-02", "UTC", "delta", -4.0, 10.0, 6.0, -4.0, nil, nil, nil, nil, nil, nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "target_user_id", "admin_user_id", "reason", "token_mode", "token_input_value", "token_before_value", "token_after_value", "token_delta",
			"token_calculation_start_date", "token_calculation_end_date", "token_calculation_timezone", "balance_mode", "balance_input_value", "balance_before_value", "balance_after_value", "balance_delta",
			"consumption_mode", "consumption_input_value", "consumption_before_value", "consumption_after_value", "consumption_delta", "consumption_start_date", "consumption_end_date", "consumption_timezone",
			"created_at",
		}).AddRow(int64(9), int64(42), int64(1), "", "delta", int64(100), int64(400), int64(500), int64(100), "2026-06-01", "2026-06-02", "UTC", "delta", -4.0, 10.0, 6.0, -4.0, nil, nil, nil, nil, nil, nil, nil, nil, createdAt))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibration_daily_allocations .*RETURNING").
		WithArgs(int64(9), int64(42), "2026-06-01", int64(100), int64(25), -1.0).
		WillReturnRows(calibrationAllocationRows().AddRow(int64(1), int64(9), int64(42), "2026-06-01", int64(100), int64(25), -1.0, createdAt))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibration_daily_allocations .*RETURNING").
		WithArgs(int64(9), int64(42), "2026-06-02", int64(300), int64(75), -3.0).
		WillReturnRows(calibrationAllocationRows().AddRow(int64(2), int64(9), int64(42), "2026-06-02", int64(300), int64(75), -3.0, createdAt))
	mock.ExpectExec("UPDATE users").WithArgs(int64(42), 6.0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.CreateAdminUsageCalibration(context.Background(), input)

	require.NoError(t, err)
	require.Len(t, got.Allocations, 2)
	require.InDelta(t, -1.0, *got.Allocations[0].BalanceDelta, 1e-9)
	require.InDelta(t, -3.0, *got.Allocations[1].BalanceDelta, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAdminUsageCalibrationTargetsRangeConsumptionAndUpdatesBalance(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}
	createdAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	input := service.AdminUsageCalibrationCreateInput{
		TargetUserID: 42,
		AdminUserID:  1,
		Consumption: &service.AdminUsageConsumptionCalibrationInput{
			Mode:      service.AdminUsageCalibrationModeTarget,
			Value:     14,
			StartDate: "2026-06-01",
			EndDate:   "2026-06-02",
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(20.0))
	mock.ExpectQuery("TO_CHAR\\(\\(created_at AT TIME ZONE").
		WithArgs(int64(42), "2026-06-01", "2026-06-03", "UTC").
		WillReturnRows(sqlmock.NewRows([]string{"allocation_date", "tokens"}).
			AddRow("2026-06-01", int64(100)).
			AddRow("2026-06-02", int64(300)))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(actual_cost\\), 0\\)").
		WithArgs(int64(42), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(8.0))
	mock.ExpectQuery("(?s)WITH balance_deltas AS .*SUM\\(consumption_delta\\)").
		WithArgs(int64(42), "2026-06-01", "2026-06-03", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2.0))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibrations .*RETURNING").
		WithArgs(int64(42), int64(1), "", nil, nil, nil, nil, nil, nil, nil, nil, "delta", -4.0, 20.0, 16.0, -4.0, "target", 14.0, 10.0, 14.0, 4.0, "2026-06-01", "2026-06-02", "UTC").
		WillReturnRows(calibrationAuditRows().AddRow(
			int64(10), int64(42), int64(1), "", nil, nil, nil, nil, nil, nil, nil, nil,
			"delta", -4.0, 20.0, 16.0, -4.0, "target", 14.0, 10.0, 14.0, 4.0,
			"2026-06-01", "2026-06-02", "UTC", createdAt,
		))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibration_daily_allocations .*RETURNING").
		WithArgs(int64(10), int64(42), "2026-06-01", int64(100), int64(0), -1.0).
		WillReturnRows(calibrationAllocationRows().AddRow(int64(3), int64(10), int64(42), "2026-06-01", int64(100), int64(0), -1.0, createdAt))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibration_daily_allocations .*RETURNING").
		WithArgs(int64(10), int64(42), "2026-06-02", int64(300), int64(0), -3.0).
		WillReturnRows(calibrationAllocationRows().AddRow(int64(4), int64(10), int64(42), "2026-06-02", int64(300), int64(0), -3.0, createdAt))
	mock.ExpectExec("UPDATE users").WithArgs(int64(42), 16.0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.CreateAdminUsageCalibration(context.Background(), input)

	require.NoError(t, err)
	require.InDelta(t, 4.0, *got.ConsumptionDelta, 1e-9)
	require.InDelta(t, -4.0, *got.BalanceDelta, 1e-9)
	require.Len(t, got.Allocations, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}


func TestCreateAdminUsageCalibrationAllowsConsumptionWithoutOriginalTokens(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}
	createdAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	start := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	input := service.AdminUsageCalibrationCreateInput{
		TargetUserID: 42,
		AdminUserID:  1,
		Consumption: &service.AdminUsageConsumptionCalibrationInput{
			Mode:      service.AdminUsageCalibrationModeTarget,
			Value:     0,
			StartDate: "2026-07-24",
			EndDate:   "2026-07-24",
			Timezone:  "UTC",
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectQuery("TO_CHAR\\(\\(created_at AT TIME ZONE").
		WithArgs(int64(42), "2026-07-24", "2026-07-25", "UTC").
		WillReturnRows(sqlmock.NewRows([]string{"allocation_date", "tokens"}))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(actual_cost\\), 0\\)").
		WithArgs(int64(42), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0.0))
	mock.ExpectQuery("(?s)WITH balance_deltas AS .*SUM\\(consumption_delta\\)").
		WithArgs(int64(42), "2026-07-24", "2026-07-25", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(70.0))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibrations .*RETURNING").
		WithArgs(int64(42), int64(1), "", nil, nil, nil, nil, nil, nil, nil, nil, "delta", 70.0, 10.0, 80.0, 70.0, "target", 0.0, 70.0, 0.0, -70.0, "2026-07-24", "2026-07-24", "UTC").
		WillReturnRows(calibrationAuditRows().AddRow(
			int64(11), int64(42), int64(1), "", nil, nil, nil, nil, nil, nil, nil, nil,
			"delta", 70.0, 10.0, 80.0, 70.0, "target", 0.0, 70.0, 0.0, -70.0,
			"2026-07-24", "2026-07-24", "UTC", createdAt,
		))
	mock.ExpectQuery("(?s)INSERT INTO admin_usage_calibration_daily_allocations .*RETURNING").
		WithArgs(int64(11), int64(42), "2026-07-24", int64(0), int64(0), 70.0).
		WillReturnRows(calibrationAllocationRows().AddRow(int64(5), int64(11), int64(42), "2026-07-24", int64(0), int64(0), 70.0, createdAt))
	mock.ExpectExec("UPDATE users").WithArgs(int64(42), 80.0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.CreateAdminUsageCalibration(context.Background(), input)

	require.NoError(t, err)
	require.InDelta(t, -70.0, *got.ConsumptionDelta, 1e-9)
	require.InDelta(t, 70.0, *got.BalanceDelta, 1e-9)
	require.Len(t, got.Allocations, 1)
	require.Equal(t, "2026-07-24", got.Allocations[0].Date)
	require.Equal(t, int64(0), got.Allocations[0].OriginalToken)
	require.Equal(t, int64(0), got.Allocations[0].TokenDelta)
	require.InDelta(t, 70.0, *got.Allocations[0].BalanceDelta, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAllocateBalanceDeltaOnDate(t *testing.T) {
	got := allocateBalanceDeltaOnDate("2026-07-24", 70)
	require.Len(t, got, 1)
	require.Equal(t, "2026-07-24", got[0].Date)
	require.Equal(t, int64(0), got[0].OriginalTokens)
	require.InDelta(t, 70.0, *got[0].BalanceDelta, 1e-12)
	require.Nil(t, allocateBalanceDeltaOnDate("", 1))
	require.Nil(t, allocateBalanceDeltaOnDate("2026-07-24", 0))
}


func calibrationAuditRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "target_user_id", "admin_user_id", "reason", "token_mode", "token_input_value", "token_before_value", "token_after_value", "token_delta",
		"token_calculation_start_date", "token_calculation_end_date", "token_calculation_timezone", "balance_mode", "balance_input_value", "balance_before_value", "balance_after_value", "balance_delta",
		"consumption_mode", "consumption_input_value", "consumption_before_value", "consumption_after_value", "consumption_delta", "consumption_start_date", "consumption_end_date", "consumption_timezone", "created_at",
	})
}

func calibrationAllocationRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "calibration_id", "target_user_id", "allocation_date", "original_tokens", "token_delta", "balance_delta", "created_at"})
}

func TestAdminUsageCalibrationSumBalanceSpentUsesOnlyNegativeDelta(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminUsageCalibrationRepository{sql: db}
	start := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery("(?s)WITH balance_deltas AS .*admin_usage_calibration_daily_allocations.*SELECT COALESCE\\(SUM\\(consumption_delta\\), 0\\) FROM balance_deltas").
		WithArgs(int64(42), "2026-06-20", "2026-06-21", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3.75))

	got, err := repo.SumBalanceSpent(context.Background(), 42, start, end)

	require.NoError(t, err)
	require.InDelta(t, 3.75, got, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAllocateBalanceDeltaKeepsExactMicroTotalWithDeterministicRemainder(t *testing.T) {
	rows := []rawDailyTokenRow{
		{Date: "2026-06-01", Tokens: 100},
		{Date: "2026-06-02", Tokens: 300},
		{Date: "2026-06-03", Tokens: 600},
	}

	got := allocateBalanceDelta(rows, 1000, -1.000001)

	require.Len(t, got, 3)
	require.InDelta(t, -0.1, *got[0].BalanceDelta, 1e-12)
	require.InDelta(t, -0.3, *got[1].BalanceDelta, 1e-12)
	require.InDelta(t, -0.600001, *got[2].BalanceDelta, 1e-12)
	require.Equal(t, int64(-1_000_001), sumAllocationBalanceMicroUnits(got))
}

func TestAllocateBalanceDeltaRetainsZeroDaysToMarkCompleteAllocation(t *testing.T) {
	rows := []rawDailyTokenRow{
		{Date: "2026-06-01", Tokens: 1},
		{Date: "2026-06-02", Tokens: 1},
		{Date: "2026-06-03", Tokens: 1},
	}

	got := allocateBalanceDelta(rows, 3, 0.000001)

	require.Len(t, got, 3)
	require.Equal(t, int64(1), sumAllocationBalanceMicroUnits(got))
	for _, row := range got {
		require.NotNil(t, row.BalanceDelta)
	}
}

func TestCalculateConsumptionCalibration(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		value          float64
		currentSpend   float64
		currentBalance float64
		wantDelta      float64
		wantSpend      float64
		wantBalance    float64
		wantErrorText  string
	}{
		{name: "target increase deducts wallet", mode: "target", value: 14, currentSpend: 10, currentBalance: 20, wantDelta: 4, wantSpend: 14, wantBalance: 16},
		{name: "delta decrease refunds wallet", mode: "delta", value: -4, currentSpend: 10, currentBalance: 20, wantDelta: -4, wantSpend: 6, wantBalance: 24},
		{name: "rejects negative spend", mode: "delta", value: -11, currentSpend: 10, currentBalance: 20, wantErrorText: "range consumption negative"},
		{name: "rejects insufficient wallet", mode: "delta", value: 21, currentSpend: 10, currentBalance: 20, wantErrorText: "user balance negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, spend, balance, err := calculateConsumptionCalibration(tt.mode, tt.value, tt.currentSpend, tt.currentBalance)
			if tt.wantErrorText != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrorText)
				return
			}
			require.NoError(t, err)
			require.InDelta(t, tt.wantDelta, delta, 1e-9)
			require.InDelta(t, tt.wantSpend, spend, 1e-9)
			require.InDelta(t, tt.wantBalance, balance, 1e-9)
		})
	}
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

func sumAllocationBalanceMicroUnits(rows []allocationPlanRow) int64 {
	var total int64
	for _, row := range rows {
		if row.BalanceDelta != nil {
			total += int64(math.Round(*row.BalanceDelta * float64(adminUsageBalanceScale)))
		}
	}
	return total
}
