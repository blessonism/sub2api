package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type adminUsageCalibrationRepository struct {
	sql sqlExecutor
}

type rawDailyTokenRow struct {
	Date   string
	Tokens int64
}

type allocationPlanRow struct {
	Date           string
	OriginalTokens int64
	TokenDelta     int64
	BalanceDelta   *float64
}

const adminUsageBalanceScale int64 = 1_000_000

func NewAdminUsageCalibrationRepository(sqlDB *sql.DB) service.AdminUsageCalibrationRepository {
	return &adminUsageCalibrationRepository{sql: sqlDB}
}

func (r *adminUsageCalibrationRepository) CreateAdminUsageCalibration(ctx context.Context, input service.AdminUsageCalibrationCreateInput) (*service.AdminUsageCalibration, error) {
	txStarter, ok := r.sql.(sqlTxStarter)
	if !ok {
		return nil, fmt.Errorf("admin usage calibration repository sql executor does not support transactions")
	}
	tx, err := txStarter.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin admin usage calibration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record, err := r.createAdminUsageCalibrationInTx(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit admin usage calibration transaction: %w", err)
	}
	return record, nil
}

func (r *adminUsageCalibrationRepository) createAdminUsageCalibrationInTx(ctx context.Context, tx *sql.Tx, input service.AdminUsageCalibrationCreateInput) (*service.AdminUsageCalibration, error) {
	var currentBalance float64
	if err := scanSingleRow(ctx, tx, `
		SELECT balance
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{input.TargetUserID}, &currentBalance); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUserNotFound
		}
		return nil, fmt.Errorf("lock target user balance: %w", err)
	}

	var (
		tokenMode            *string
		tokenInput           *int64
		tokenBefore          *int64
		tokenAfter           *int64
		tokenDelta           *int64
		tokenStartDate       *string
		tokenEndDate         *string
		tokenTimezone        *string
		allocationRows       []allocationPlanRow
		balanceMode          *string
		balanceInput         *float64
		balanceBefore        *float64
		balanceAfter         *float64
		balanceDelta         *float64
		consumptionMode      *string
		consumptionInput     *float64
		consumptionBefore    *float64
		consumptionAfter     *float64
		consumptionDelta     *float64
		consumptionStartDate *string
		consumptionEndDate   *string
		consumptionTimezone  *string
		effectiveBalance     = currentBalance
		rawTokenRows         []rawDailyTokenRow
		rawTokenTotal        int64
		tokenBeforeValue     int64
		tokenAfterValue      int64
	)

	if input.Token != nil {
		plan, rawRows, originalTotal, before, after, delta, err := r.planTokenCalibration(ctx, tx, input.TargetUserID, *input.Token)
		if err != nil {
			return nil, err
		}
		rawTokenRows = rawRows
		rawTokenTotal = originalTotal
		tokenBeforeValue = before
		tokenAfterValue = after
		if delta != 0 {
			mode := input.Token.Mode
			tz := strings.TrimSpace(input.Token.Timezone)
			if tz == "" {
				tz = "UTC"
			}
			tokenMode = &mode
			tokenInput = &input.Token.Value
			tokenBefore = &before
			tokenAfter = &after
			tokenDelta = &delta
			tokenStartDate = &input.Token.StartDate
			tokenEndDate = &input.Token.EndDate
			tokenTimezone = &tz
			allocationRows = plan
		}
	}

	if input.Balance != nil {
		before := currentBalance
		after := currentBalance
		switch input.Balance.Mode {
		case service.AdminUsageCalibrationModeDelta:
			after = currentBalance + input.Balance.Value
		case service.AdminUsageCalibrationModeTarget:
			after = input.Balance.Value
		default:
			return nil, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "balance.mode"})
		}
		if after < 0 {
			return nil, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NEGATIVE_BALANCE", "balance calibration cannot make user balance negative")
		}
		delta := after - currentBalance
		if delta != 0 {
			mode := input.Balance.Mode
			value := input.Balance.Value
			balanceMode = &mode
			balanceInput = &value
			balanceBefore = &before
			balanceAfter = &after
			balanceDelta = &delta
			effectiveBalance = after
		}
	}

	if input.Consumption != nil {
		tz := strings.TrimSpace(input.Consumption.Timezone)
		if tz == "" {
			tz = "UTC"
		}
		rows, originalTokens, err := queryOriginalDailyTokens(ctx, tx, input.TargetUserID, input.Consumption.StartDate, input.Consumption.EndDate, tz)
		if err != nil {
			return nil, err
		}
		startTime, endTime, err := calibrationTimeRange(input.Consumption.StartDate, input.Consumption.EndDate, tz)
		if err != nil {
			return nil, err
		}
		originalCost, err := queryOriginalUsageCost(ctx, tx, input.TargetUserID, startTime, endTime)
		if err != nil {
			return nil, err
		}
		existingDelta, err := sumBalanceSpentByTimeRange(ctx, tx, input.TargetUserID, startTime, endTime)
		if err != nil {
			return nil, err
		}
		before := originalCost + existingDelta
		delta, after, walletAfter, err := calculateConsumptionCalibration(input.Consumption.Mode, input.Consumption.Value, before, currentBalance)
		if err != nil {
			return nil, err
		}
		walletDelta := -delta
		if delta != 0 {
			walletMode := service.AdminUsageCalibrationModeDelta
			consumptionMode = &input.Consumption.Mode
			consumptionInput = &input.Consumption.Value
			consumptionBefore = &before
			consumptionAfter = &after
			consumptionDelta = &delta
			consumptionStartDate = &input.Consumption.StartDate
			consumptionEndDate = &input.Consumption.EndDate
			consumptionTimezone = &tz
			balanceMode = &walletMode
			balanceInput = &walletDelta
			balanceBefore = &currentBalance
			balanceAfter = &walletAfter
			balanceDelta = &walletDelta
			effectiveBalance = walletAfter
			if originalTokens > 0 {
				allocationRows = mergeAllocationPlans(allocationRows, allocateBalanceDelta(rows, originalTokens, walletDelta))
			} else {
				allocationRows = mergeAllocationPlans(allocationRows, allocateBalanceDeltaOnDate(input.Consumption.StartDate, walletDelta))
			}
		}
	}

	if input.Token != nil && input.Consumption == nil && balanceDelta != nil {
		if tokenDelta == nil {
			mode := input.Token.Mode
			tz := strings.TrimSpace(input.Token.Timezone)
			if tz == "" {
				tz = "UTC"
			}
			zero := int64(0)
			tokenMode = &mode
			tokenInput = &input.Token.Value
			tokenBefore = &tokenBeforeValue
			tokenAfter = &tokenAfterValue
			tokenDelta = &zero
			tokenStartDate = &input.Token.StartDate
			tokenEndDate = &input.Token.EndDate
			tokenTimezone = &tz
		}
		allocationRows = mergeAllocationPlans(allocationRows, allocateBalanceDelta(rawTokenRows, rawTokenTotal, *balanceDelta))
	}

	if (tokenDelta == nil || *tokenDelta == 0) && balanceDelta == nil {
		return nil, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NO_CHANGE", "calibration does not change token usage or balance")
	}

	var record service.AdminUsageCalibration
	insertQuery := `
		INSERT INTO admin_usage_calibrations (
			target_user_id,
			admin_user_id,
			reason,
			token_mode,
			token_input_value,
			token_before_value,
			token_after_value,
			token_delta,
			token_calculation_start_date,
			token_calculation_end_date,
			token_calculation_timezone,
			balance_mode,
			balance_input_value,
			balance_before_value,
			balance_after_value,
			balance_delta,
			consumption_mode,
			consumption_input_value,
			consumption_before_value,
			consumption_after_value,
			consumption_delta,
			consumption_start_date,
			consumption_end_date,
			consumption_timezone
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::date, $10::date, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22::date, $23::date, $24)
		RETURNING
			id,
			target_user_id,
			admin_user_id,
			reason,
			token_mode,
			token_input_value,
			token_before_value,
			token_after_value,
			token_delta,
			TO_CHAR(token_calculation_start_date, 'YYYY-MM-DD'),
			TO_CHAR(token_calculation_end_date, 'YYYY-MM-DD'),
			token_calculation_timezone,
			balance_mode,
			balance_input_value,
			balance_before_value,
			balance_after_value,
			balance_delta,
			consumption_mode,
			consumption_input_value,
			consumption_before_value,
			consumption_after_value,
			consumption_delta,
			TO_CHAR(consumption_start_date, 'YYYY-MM-DD'),
			TO_CHAR(consumption_end_date, 'YYYY-MM-DD'),
			consumption_timezone,
			created_at
	`
	if err := scanAdminUsageCalibrationRow(ctx, tx, insertQuery, []any{
		input.TargetUserID,
		input.AdminUserID,
		input.Reason,
		nullableStringValue(tokenMode),
		nullableInt64Value(tokenInput),
		nullableInt64Value(tokenBefore),
		nullableInt64Value(tokenAfter),
		nullableInt64Value(tokenDelta),
		nullableStringValue(tokenStartDate),
		nullableStringValue(tokenEndDate),
		nullableStringValue(tokenTimezone),
		nullableStringValue(balanceMode),
		nullableFloat64Value(balanceInput),
		nullableFloat64Value(balanceBefore),
		nullableFloat64Value(balanceAfter),
		nullableFloat64Value(balanceDelta),
		nullableStringValue(consumptionMode),
		nullableFloat64Value(consumptionInput),
		nullableFloat64Value(consumptionBefore),
		nullableFloat64Value(consumptionAfter),
		nullableFloat64Value(consumptionDelta),
		nullableStringValue(consumptionStartDate),
		nullableStringValue(consumptionEndDate),
		nullableStringValue(consumptionTimezone),
	}, &record); err != nil {
		return nil, fmt.Errorf("insert admin usage calibration: %w", err)
	}

	if len(allocationRows) > 0 {
		allocations, err := insertAdminUsageCalibrationAllocations(ctx, tx, record.ID, input.TargetUserID, allocationRows)
		if err != nil {
			return nil, err
		}
		record.Allocations = allocations
	}

	if balanceDelta != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET balance = $2, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, input.TargetUserID, effectiveBalance); err != nil {
			return nil, fmt.Errorf("update calibrated balance: %w", err)
		}
	}

	return &record, nil
}

func calculateConsumptionCalibration(mode string, value, currentConsumption, currentBalance float64) (delta, after, walletAfter float64, err error) {
	switch mode {
	case service.AdminUsageCalibrationModeDelta:
		delta = value
	case service.AdminUsageCalibrationModeTarget:
		delta = value - currentConsumption
	default:
		return 0, 0, 0, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.mode"})
	}
	delta = math.Round(delta*float64(adminUsageBalanceScale)) / float64(adminUsageBalanceScale)
	after = currentConsumption + delta
	if after < 0 {
		return 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NEGATIVE_CONSUMPTION", "consumption calibration cannot make range consumption negative")
	}
	walletAfter = currentBalance - delta
	if walletAfter < 0 {
		return 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NEGATIVE_BALANCE", "consumption calibration cannot make user balance negative")
	}
	return delta, after, walletAfter, nil
}

func (r *adminUsageCalibrationRepository) planTokenCalibration(ctx context.Context, exec sqlQueryer, userID int64, input service.AdminUsageTokenCalibrationInput) ([]allocationPlanRow, []rawDailyTokenRow, int64, int64, int64, int64, error) {
	tz := strings.TrimSpace(input.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	rows, originalTotal, err := queryOriginalDailyTokens(ctx, exec, userID, input.StartDate, input.EndDate, tz)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	if originalTotal == 0 {
		return nil, nil, 0, 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE", "该范围没有原始用量，无法按比例分摊")
	}
	existingDelta, err := sumTokenAllocationsByDateRange(ctx, exec, userID, input.StartDate, exclusiveDate(input.EndDate))
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	before := originalTotal + existingDelta
	if before < 0 {
		before = 0
	}
	var delta int64
	switch input.Mode {
	case service.AdminUsageCalibrationModeDelta:
		delta = input.Value
	case service.AdminUsageCalibrationModeTarget:
		delta = input.Value - before
	default:
		return nil, nil, 0, 0, 0, 0, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.mode"})
	}
	after := before + delta
	if after < 0 {
		return nil, nil, 0, 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NEGATIVE_TOKENS", "token calibration cannot make token usage negative")
	}
	if delta == 0 {
		return nil, rows, originalTotal, before, after, 0, nil
	}
	plan := allocateTokenDelta(rows, originalTotal, delta)
	return plan, rows, originalTotal, before, after, delta, nil
}

func queryOriginalDailyTokens(ctx context.Context, exec sqlQueryer, userID int64, startDate, endDate, tz string) ([]rawDailyTokenRow, int64, error) {
	endExclusive := exclusiveDate(endDate)
	query := `
		SELECT
			TO_CHAR((created_at AT TIME ZONE $4)::date, 'YYYY-MM-DD') AS allocation_date,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS tokens
		FROM usage_logs
		WHERE user_id = $1
		  AND created_at >= ($2::date::timestamp AT TIME ZONE $4)
		  AND created_at < ($3::date::timestamp AT TIME ZONE $4)
		GROUP BY 1
		HAVING COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) > 0
		ORDER BY 1 ASC
	`
	rows, err := exec.QueryContext(ctx, query, userID, startDate, endExclusive, tz)
	if err != nil {
		return nil, 0, fmt.Errorf("query original daily tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]rawDailyTokenRow, 0)
	var total int64
	for rows.Next() {
		var row rawDailyTokenRow
		if err := rows.Scan(&row.Date, &row.Tokens); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
		total += row.Tokens
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func queryOriginalUsageCost(ctx context.Context, exec sqlQueryer, userID int64, startTime, endTime time.Time) (float64, error) {
	var total float64
	if err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(SUM(actual_cost), 0)
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, []any{userID, startTime, endTime}, &total); err != nil {
		return 0, fmt.Errorf("query original usage cost: %w", err)
	}
	return total, nil
}

func calibrationTimeRange(startDate, endDate, timezoneName string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return time.Time{}, time.Time{}, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.timezone"})
	}
	start, err := time.ParseInLocation("2006-01-02", startDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.start_date"})
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, location)
	if err != nil || end.Before(start) {
		return time.Time{}, time.Time{}, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.date_range"})
	}
	return start, end.AddDate(0, 0, 1), nil
}

func allocateTokenDelta(rows []rawDailyTokenRow, originalTotal int64, delta int64) []allocationPlanRow {
	return allocateIntegerDelta(rows, originalTotal, delta)
}

func allocateIntegerDelta(rows []rawDailyTokenRow, originalTotal int64, delta int64) []allocationPlanRow {
	if delta == 0 || originalTotal <= 0 {
		return nil
	}
	negative := delta < 0
	absDelta := uint64(delta)
	if delta < 0 {
		absDelta = uint64(-(delta + 1)) + 1
	}
	type weightedRow struct {
		Date           string
		OriginalTokens int64
		Base           uint64
		Remainder      uint64
	}
	weighted := make([]weightedRow, 0, len(rows))
	var baseTotal uint64
	for _, row := range rows {
		if row.Tokens <= 0 {
			continue
		}
		hi, lo := bits.Mul64(absDelta, uint64(row.Tokens))
		base, remainder := bits.Div64(hi, lo, uint64(originalTotal))
		baseTotal += base
		weighted = append(weighted, weightedRow{
			Date:           row.Date,
			OriginalTokens: row.Tokens,
			Base:           base,
			Remainder:      remainder,
		})
	}
	remaining := absDelta - baseTotal
	sort.SliceStable(weighted, func(i, j int) bool {
		if weighted[i].Remainder != weighted[j].Remainder {
			return weighted[i].Remainder > weighted[j].Remainder
		}
		if weighted[i].OriginalTokens != weighted[j].OriginalTokens {
			return weighted[i].OriginalTokens > weighted[j].OriginalTokens
		}
		return weighted[i].Date < weighted[j].Date
	})
	for i := range weighted {
		if remaining == 0 {
			break
		}
		weighted[i].Base++
		remaining--
	}
	sort.SliceStable(weighted, func(i, j int) bool {
		return weighted[i].Date < weighted[j].Date
	})
	plan := make([]allocationPlanRow, 0, len(weighted))
	for _, row := range weighted {
		tokenDelta := int64(row.Base)
		if negative {
			tokenDelta = -tokenDelta
		}
		if tokenDelta == 0 {
			continue
		}
		plan = append(plan, allocationPlanRow{
			Date:           row.Date,
			OriginalTokens: row.OriginalTokens,
			TokenDelta:     tokenDelta,
		})
	}
	return plan
}

func allocateBalanceDelta(rows []rawDailyTokenRow, originalTotal int64, delta float64) []allocationPlanRow {
	if originalTotal <= 0 || delta == 0 {
		return nil
	}
	units := int64(math.Round(math.Abs(delta) * float64(adminUsageBalanceScale)))
	if units == 0 {
		return nil
	}
	if delta < 0 {
		units = -units
	}
	allocated := allocateIntegerDelta(rows, originalTotal, units)
	unitsByDate := make(map[string]int64, len(allocated))
	for _, row := range allocated {
		unitsByDate[row.Date] = row.TokenDelta
	}
	plan := make([]allocationPlanRow, 0, len(rows))
	for _, row := range rows {
		if row.Tokens <= 0 {
			continue
		}
		value := float64(unitsByDate[row.Date]) / float64(adminUsageBalanceScale)
		plan = append(plan, allocationPlanRow{Date: row.Date, OriginalTokens: row.Tokens, BalanceDelta: &value})
	}
	return plan
}

// allocateBalanceDeltaOnDate assigns the whole wallet delta to a single day when
// there is no original token usage to weight against (e.g. pure calibration spend).
func allocateBalanceDeltaOnDate(date string, delta float64) []allocationPlanRow {
	date = strings.TrimSpace(date)
	if date == "" || delta == 0 {
		return nil
	}
	value := math.Round(delta*float64(adminUsageBalanceScale)) / float64(adminUsageBalanceScale)
	if value == 0 {
		return nil
	}
	return []allocationPlanRow{{
		Date:           date,
		OriginalTokens: 0,
		BalanceDelta:   &value,
	}}
}

func mergeAllocationPlans(tokenRows, balanceRows []allocationPlanRow) []allocationPlanRow {
	byDate := make(map[string]allocationPlanRow, len(tokenRows)+len(balanceRows))
	for _, row := range tokenRows {
		byDate[row.Date] = row
	}
	for _, row := range balanceRows {
		if existing, ok := byDate[row.Date]; ok {
			existing.BalanceDelta = row.BalanceDelta
			byDate[row.Date] = existing
			continue
		}
		byDate[row.Date] = row
	}
	result := make([]allocationPlanRow, 0, len(byDate))
	for _, row := range byDate {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result
}

func insertAdminUsageCalibrationAllocations(ctx context.Context, exec sqlExecutor, calibrationID, targetUserID int64, rows []allocationPlanRow) ([]service.AdminUsageCalibrationDailyAllocation, error) {
	result := make([]service.AdminUsageCalibrationDailyAllocation, 0, len(rows))
	query := `
		INSERT INTO admin_usage_calibration_daily_allocations (
			calibration_id,
			target_user_id,
			allocation_date,
			original_tokens,
			token_delta,
			balance_delta
		)
		VALUES ($1, $2, $3::date, $4, $5, $6)
		RETURNING id, calibration_id, target_user_id, TO_CHAR(allocation_date, 'YYYY-MM-DD'), original_tokens, token_delta, balance_delta, created_at
	`
	for _, row := range rows {
		var out service.AdminUsageCalibrationDailyAllocation
		if err := scanSingleRow(
			ctx,
			exec,
			query,
			[]any{calibrationID, targetUserID, row.Date, row.OriginalTokens, row.TokenDelta, nullableFloat64Value(row.BalanceDelta)},
			&out.ID,
			&out.CalibrationID,
			&out.TargetUserID,
			&out.Date,
			&out.OriginalToken,
			&out.TokenDelta,
			&out.BalanceDelta,
			&out.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("insert admin usage calibration allocation: %w", err)
		}
		result = append(result, out)
	}
	return result, nil
}

func (r *adminUsageCalibrationRepository) ListAdminUsageCalibrations(ctx context.Context, filters service.AdminUsageCalibrationListFilters, params pagination.PaginationParams) ([]service.AdminUsageCalibration, *pagination.PaginationResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	conditions := make([]string, 0, 1)
	args := make([]any, 0, 3)
	if filters.TargetUserID > 0 {
		conditions = append(conditions, fmt.Sprintf("target_user_id = $%d", len(args)+1))
		args = append(args, filters.TargetUserID)
	}
	where := buildWhere(conditions)

	var total int64
	countQuery := "SELECT COUNT(*) FROM admin_usage_calibrations " + where
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, nil, err
	}

	offset := (params.Page - 1) * params.PageSize
	args = append(args, params.PageSize, offset)
	query := fmt.Sprintf(`
		SELECT
			id,
			target_user_id,
			admin_user_id,
			reason,
			token_mode,
			token_input_value,
			token_before_value,
			token_after_value,
			token_delta,
			TO_CHAR(token_calculation_start_date, 'YYYY-MM-DD'),
			TO_CHAR(token_calculation_end_date, 'YYYY-MM-DD'),
			token_calculation_timezone,
			balance_mode,
			balance_input_value,
			balance_before_value,
			balance_after_value,
			balance_delta,
			consumption_mode,
			consumption_input_value,
			consumption_before_value,
			consumption_after_value,
			consumption_delta,
			TO_CHAR(consumption_start_date, 'YYYY-MM-DD'),
			TO_CHAR(consumption_end_date, 'YYYY-MM-DD'),
			consumption_timezone,
			created_at
		FROM admin_usage_calibrations
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminUsageCalibration, 0, params.PageSize)
	for rows.Next() {
		var item service.AdminUsageCalibration
		if err := scanAdminUsageCalibrationFromRows(rows, &item); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := r.attachAllocations(ctx, items); err != nil {
		return nil, nil, err
	}

	pages := 0
	if params.PageSize > 0 {
		pages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	result := &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    pages,
	}
	return items, result, nil
}

func (r *adminUsageCalibrationRepository) attachAllocations(ctx context.Context, items []service.AdminUsageCalibration) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	byID := make(map[int64]*service.AdminUsageCalibration, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		byID[items[i].ID] = &items[i]
	}
	query := `
		SELECT id, calibration_id, target_user_id, TO_CHAR(allocation_date, 'YYYY-MM-DD'), original_tokens, token_delta, balance_delta, created_at
		FROM admin_usage_calibration_daily_allocations
		WHERE calibration_id = ANY($1)
		ORDER BY allocation_date ASC, id ASC
	`
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var allocation service.AdminUsageCalibrationDailyAllocation
		if err := rows.Scan(&allocation.ID, &allocation.CalibrationID, &allocation.TargetUserID, &allocation.Date, &allocation.OriginalToken, &allocation.TokenDelta, &allocation.BalanceDelta, &allocation.CreatedAt); err != nil {
			return err
		}
		if item, ok := byID[allocation.CalibrationID]; ok {
			item.Allocations = append(item.Allocations, allocation)
		}
	}
	return rows.Err()
}

func (r *adminUsageCalibrationRepository) SumTokenAllocations(ctx context.Context, userID int64, startDate, endDateExclusive string) (int64, error) {
	return sumTokenAllocationsByDateRange(ctx, r.sql, userID, startDate, endDateExclusive)
}

func (r *adminUsageCalibrationRepository) SumAllTokenAllocations(ctx context.Context, startDate, endDateExclusive string) (int64, error) {
	return sumTokenAllocationsByDateRange(ctx, r.sql, 0, startDate, endDateExclusive)
}

func (r *adminUsageCalibrationRepository) SumTokenAllocationsByDate(ctx context.Context, userID int64, startDate, endDateExclusive string) (map[string]int64, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("target_user_id = $%d", len(args)+1))
		args = append(args, userID)
	}
	if startDate != "" {
		conditions = append(conditions, fmt.Sprintf("allocation_date >= $%d::date", len(args)+1))
		args = append(args, startDate)
	}
	if endDateExclusive != "" {
		conditions = append(conditions, fmt.Sprintf("allocation_date < $%d::date", len(args)+1))
		args = append(args, endDateExclusive)
	}
	query := fmt.Sprintf(`
		SELECT TO_CHAR(allocation_date, 'YYYY-MM-DD') AS date, COALESCE(SUM(token_delta), 0) AS token_delta
		FROM admin_usage_calibration_daily_allocations
		%s
		GROUP BY allocation_date
		ORDER BY allocation_date ASC
	`, buildWhere(conditions))
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string]int64)
	for rows.Next() {
		var date string
		var delta int64
		if err := rows.Scan(&date, &delta); err != nil {
			return nil, err
		}
		result[date] = delta
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *adminUsageCalibrationRepository) SumBalanceSpent(ctx context.Context, userID int64, startTime, endTime time.Time) (float64, error) {
	return sumBalanceSpentByTimeRange(ctx, r.sql, userID, startTime, endTime)
}

func sumBalanceSpentByTimeRange(ctx context.Context, exec sqlQueryer, userID int64, startTime, endTime time.Time) (float64, error) {
	args := append([]any{userID}, balanceCalibrationRangeArgs(startTime, endTime)...)
	query := `
		WITH balance_deltas AS (
			SELECT a.target_user_id, -a.balance_delta AS consumption_delta
			FROM admin_usage_calibration_daily_allocations a
			JOIN admin_usage_calibrations c ON c.id = a.calibration_id
			WHERE a.balance_delta IS NOT NULL
			  AND (c.consumption_delta IS NOT NULL OR a.balance_delta < 0)
			  AND ($1::bigint = 0 OR a.target_user_id = $1)
			  AND ($2::date IS NULL OR allocation_date >= $2::date)
			  AND ($3::date IS NULL OR allocation_date < $3::date)
			UNION ALL
			SELECT c.target_user_id, COALESCE(c.consumption_delta, -c.balance_delta)
			FROM admin_usage_calibrations c
			WHERE (c.consumption_delta IS NOT NULL OR c.balance_delta < 0)
			  AND ($1::bigint = 0 OR c.target_user_id = $1)
			  AND ($4::timestamptz IS NULL OR c.created_at >= $4::timestamptz)
			  AND ($5::timestamptz IS NULL OR c.created_at < $5::timestamptz)
			  AND NOT EXISTS (
				SELECT 1 FROM admin_usage_calibration_daily_allocations a
				WHERE a.calibration_id = c.id AND a.balance_delta IS NOT NULL
			  )
		)
		SELECT COALESCE(SUM(consumption_delta), 0) FROM balance_deltas
	`
	var total float64
	if err := scanSingleRow(ctx, exec, query, args, &total); err != nil {
		return 0, fmt.Errorf("sum balance calibration spend: %w", err)
	}
	return total, nil
}

func (r *adminUsageCalibrationRepository) SumBalanceSpentByUsers(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]float64, error) {
	result := make(map[int64]float64)
	ids := normalizePositiveInt64IDs(userIDs)
	if len(ids) == 0 {
		return result, nil
	}
	args := append([]any{pq.Array(ids)}, balanceCalibrationRangeArgs(startTime, endTime)...)
	query := `
		WITH balance_deltas AS (
			SELECT a.target_user_id, -a.balance_delta AS consumption_delta
			FROM admin_usage_calibration_daily_allocations a
			JOIN admin_usage_calibrations c ON c.id = a.calibration_id
			WHERE a.target_user_id = ANY($1)
			  AND a.balance_delta IS NOT NULL
			  AND (c.consumption_delta IS NOT NULL OR a.balance_delta < 0)
			  AND ($2::date IS NULL OR allocation_date >= $2::date)
			  AND ($3::date IS NULL OR allocation_date < $3::date)
			UNION ALL
			SELECT c.target_user_id, COALESCE(c.consumption_delta, -c.balance_delta)
			FROM admin_usage_calibrations c
			WHERE c.target_user_id = ANY($1)
			  AND (c.consumption_delta IS NOT NULL OR c.balance_delta < 0)
			  AND ($4::timestamptz IS NULL OR c.created_at >= $4::timestamptz)
			  AND ($5::timestamptz IS NULL OR c.created_at < $5::timestamptz)
			  AND NOT EXISTS (
				SELECT 1 FROM admin_usage_calibration_daily_allocations a
				WHERE a.calibration_id = c.id AND a.balance_delta IS NOT NULL
			  )
		)
		SELECT target_user_id, COALESCE(SUM(consumption_delta), 0) AS spent
		FROM balance_deltas
		GROUP BY target_user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sum balance calibration spend by users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID int64
		var spent float64
		if err := rows.Scan(&userID, &spent); err != nil {
			return nil, err
		}
		result[userID] = spent
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func sumTokenAllocationsByDateRange(ctx context.Context, exec sqlQueryer, userID int64, startDate, endDateExclusive string) (int64, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("target_user_id = $%d", len(args)+1))
		args = append(args, userID)
	}
	if startDate != "" {
		conditions = append(conditions, fmt.Sprintf("allocation_date >= $%d::date", len(args)+1))
		args = append(args, startDate)
	}
	if endDateExclusive != "" {
		conditions = append(conditions, fmt.Sprintf("allocation_date < $%d::date", len(args)+1))
		args = append(args, endDateExclusive)
	}
	query := "SELECT COALESCE(SUM(token_delta), 0) FROM admin_usage_calibration_daily_allocations " + buildWhere(conditions)
	var total int64
	if err := scanSingleRow(ctx, exec, query, args, &total); err != nil {
		return 0, err
	}
	return total, nil
}

func sumBalanceCalibrationsByTimeRange(ctx context.Context, exec sqlQueryer, userID int64, startTime, endTime time.Time) (float64, error) {
	args := append([]any{userID}, balanceCalibrationRangeArgs(startTime, endTime)...)
	query := `
		WITH balance_deltas AS (
			SELECT target_user_id, balance_delta
			FROM admin_usage_calibration_daily_allocations
			WHERE balance_delta IS NOT NULL
			  AND ($1::bigint = 0 OR target_user_id = $1)
			  AND ($2::date IS NULL OR allocation_date >= $2::date)
			  AND ($3::date IS NULL OR allocation_date < $3::date)
			UNION ALL
			SELECT c.target_user_id, c.balance_delta
			FROM admin_usage_calibrations c
			WHERE c.balance_delta IS NOT NULL
			  AND ($1::bigint = 0 OR c.target_user_id = $1)
			  AND ($4::timestamptz IS NULL OR c.created_at >= $4::timestamptz)
			  AND ($5::timestamptz IS NULL OR c.created_at < $5::timestamptz)
			  AND NOT EXISTS (
				SELECT 1 FROM admin_usage_calibration_daily_allocations a
				WHERE a.calibration_id = c.id AND a.balance_delta IS NOT NULL
			  )
		)
		SELECT COALESCE(SUM(balance_delta), 0) FROM balance_deltas
	`
	var total float64
	if err := scanSingleRow(ctx, exec, query, args, &total); err != nil {
		return 0, err
	}
	return total, nil
}

func balanceCalibrationRangeArgs(startTime, endTime time.Time) []any {
	startDate, endDateExclusive := calibrationAllocationDateRange(startTime, endTime)
	return []any{
		nullableRangeString(startDate),
		nullableRangeString(endDateExclusive),
		nullableRangeTime(startTime),
		nullableRangeTime(endTime),
	}
}

func nullableRangeString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableRangeTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func scanAdminUsageCalibrationRow(ctx context.Context, exec sqlQueryer, query string, args []any, out *service.AdminUsageCalibration) error {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := scanAdminUsageCalibrationFromRows(rows, out); err != nil {
		return err
	}
	return rows.Err()
}

func scanAdminUsageCalibrationFromRows(rows *sql.Rows, out *service.AdminUsageCalibration) error {
	var (
		tokenMode            sql.NullString
		tokenInput           sql.NullInt64
		tokenBefore          sql.NullInt64
		tokenAfter           sql.NullInt64
		tokenDelta           sql.NullInt64
		tokenStartDate       sql.NullString
		tokenEndDate         sql.NullString
		tokenTimezone        sql.NullString
		balanceMode          sql.NullString
		balanceInput         sql.NullFloat64
		balanceBefore        sql.NullFloat64
		balanceAfter         sql.NullFloat64
		balanceDelta         sql.NullFloat64
		consumptionMode      sql.NullString
		consumptionInput     sql.NullFloat64
		consumptionBefore    sql.NullFloat64
		consumptionAfter     sql.NullFloat64
		consumptionDelta     sql.NullFloat64
		consumptionStartDate sql.NullString
		consumptionEndDate   sql.NullString
		consumptionTimezone  sql.NullString
	)
	if err := rows.Scan(
		&out.ID,
		&out.TargetUserID,
		&out.AdminUserID,
		&out.Reason,
		&tokenMode,
		&tokenInput,
		&tokenBefore,
		&tokenAfter,
		&tokenDelta,
		&tokenStartDate,
		&tokenEndDate,
		&tokenTimezone,
		&balanceMode,
		&balanceInput,
		&balanceBefore,
		&balanceAfter,
		&balanceDelta,
		&consumptionMode,
		&consumptionInput,
		&consumptionBefore,
		&consumptionAfter,
		&consumptionDelta,
		&consumptionStartDate,
		&consumptionEndDate,
		&consumptionTimezone,
		&out.CreatedAt,
	); err != nil {
		return err
	}
	out.TokenMode = nullStringPtr(tokenMode)
	out.TokenInputValue = nullInt64Ptr(tokenInput)
	out.TokenBeforeValue = nullInt64Ptr(tokenBefore)
	out.TokenAfterValue = nullInt64Ptr(tokenAfter)
	out.TokenDelta = nullInt64Ptr(tokenDelta)
	out.TokenCalculationStartDate = nullStringPtr(tokenStartDate)
	out.TokenCalculationEndDate = nullStringPtr(tokenEndDate)
	out.TokenCalculationTimezone = nullStringPtr(tokenTimezone)
	out.BalanceMode = nullStringPtr(balanceMode)
	out.BalanceInputValue = nullCalibrationFloat64Ptr(balanceInput)
	out.BalanceBeforeValue = nullCalibrationFloat64Ptr(balanceBefore)
	out.BalanceAfterValue = nullCalibrationFloat64Ptr(balanceAfter)
	out.BalanceDelta = nullCalibrationFloat64Ptr(balanceDelta)
	out.ConsumptionMode = nullStringPtr(consumptionMode)
	out.ConsumptionInputValue = nullCalibrationFloat64Ptr(consumptionInput)
	out.ConsumptionBeforeValue = nullCalibrationFloat64Ptr(consumptionBefore)
	out.ConsumptionAfterValue = nullCalibrationFloat64Ptr(consumptionAfter)
	out.ConsumptionDelta = nullCalibrationFloat64Ptr(consumptionDelta)
	out.ConsumptionStartDate = nullStringPtr(consumptionStartDate)
	out.ConsumptionEndDate = nullStringPtr(consumptionEndDate)
	out.ConsumptionTimezone = nullStringPtr(consumptionTimezone)
	return nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullCalibrationFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullableStringValue(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Value(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFloat64Value(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func exclusiveDate(date string) string {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, 1).Format("2006-01-02")
}
