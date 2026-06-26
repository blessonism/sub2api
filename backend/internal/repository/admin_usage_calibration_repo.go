package repository

import (
	"context"
	"database/sql"
	"fmt"
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
}

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
		tokenMode        *string
		tokenInput       *int64
		tokenBefore      *int64
		tokenAfter       *int64
		tokenDelta       *int64
		tokenStartDate   *string
		tokenEndDate     *string
		tokenTimezone    *string
		allocationRows   []allocationPlanRow
		balanceMode      *string
		balanceInput     *float64
		balanceBefore    *float64
		balanceAfter     *float64
		balanceDelta     *float64
		effectiveBalance = currentBalance
	)

	if input.Token != nil {
		plan, before, after, delta, err := r.planTokenCalibration(ctx, tx, input.TargetUserID, *input.Token)
		if err != nil {
			return nil, err
		}
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

	if tokenDelta == nil && balanceDelta == nil {
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
			balance_delta
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::date, $10::date, $11, $12, $13, $14, $15, $16)
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

func (r *adminUsageCalibrationRepository) planTokenCalibration(ctx context.Context, exec sqlQueryer, userID int64, input service.AdminUsageTokenCalibrationInput) ([]allocationPlanRow, int64, int64, int64, error) {
	tz := strings.TrimSpace(input.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	rows, originalTotal, err := queryOriginalDailyTokens(ctx, exec, userID, input.StartDate, input.EndDate, tz)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if originalTotal == 0 {
		return nil, 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE", "该范围没有原始用量，无法按比例分摊")
	}
	existingDelta, err := sumTokenAllocationsByDateRange(ctx, exec, userID, input.StartDate, exclusiveDate(input.EndDate))
	if err != nil {
		return nil, 0, 0, 0, err
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
		return nil, 0, 0, 0, service.ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.mode"})
	}
	after := before + delta
	if after < 0 {
		return nil, 0, 0, 0, infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_NEGATIVE_TOKENS", "token calibration cannot make token usage negative")
	}
	if delta == 0 {
		return nil, before, after, 0, nil
	}
	plan := allocateTokenDelta(rows, originalTotal, delta)
	return plan, before, after, delta, nil
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

func allocateTokenDelta(rows []rawDailyTokenRow, originalTotal int64, delta int64) []allocationPlanRow {
	if delta == 0 || originalTotal <= 0 {
		return nil
	}
	sign := int64(1)
	absDelta := delta
	if delta < 0 {
		sign = -1
		absDelta = -delta
	}
	type weightedRow struct {
		Date           string
		OriginalTokens int64
		Base           int64
		Remainder      int64
	}
	weighted := make([]weightedRow, 0, len(rows))
	var baseTotal int64
	for _, row := range rows {
		if row.Tokens <= 0 {
			continue
		}
		product := absDelta * row.Tokens
		base := product / originalTotal
		remainder := product % originalTotal
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
		if remaining <= 0 {
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
		tokenDelta := row.Base * sign
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

func insertAdminUsageCalibrationAllocations(ctx context.Context, exec sqlExecutor, calibrationID, targetUserID int64, rows []allocationPlanRow) ([]service.AdminUsageCalibrationDailyAllocation, error) {
	result := make([]service.AdminUsageCalibrationDailyAllocation, 0, len(rows))
	query := `
		INSERT INTO admin_usage_calibration_daily_allocations (
			calibration_id,
			target_user_id,
			allocation_date,
			original_tokens,
			token_delta
		)
		VALUES ($1, $2, $3::date, $4, $5)
		RETURNING id, calibration_id, target_user_id, TO_CHAR(allocation_date, 'YYYY-MM-DD'), original_tokens, token_delta, created_at
	`
	for _, row := range rows {
		var out service.AdminUsageCalibrationDailyAllocation
		if err := scanSingleRow(
			ctx,
			exec,
			query,
			[]any{calibrationID, targetUserID, row.Date, row.OriginalTokens, row.TokenDelta},
			&out.ID,
			&out.CalibrationID,
			&out.TargetUserID,
			&out.Date,
			&out.OriginalToken,
			&out.TokenDelta,
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
		SELECT id, calibration_id, target_user_id, TO_CHAR(allocation_date, 'YYYY-MM-DD'), original_tokens, token_delta, created_at
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
		if err := rows.Scan(&allocation.ID, &allocation.CalibrationID, &allocation.TargetUserID, &allocation.Date, &allocation.OriginalToken, &allocation.TokenDelta, &allocation.CreatedAt); err != nil {
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
	conditions := []string{"balance_delta < 0"}
	args := make([]any, 0, 3)
	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("target_user_id = $%d", len(args)+1))
		args = append(args, userID)
	}
	if !startTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, startTime)
	}
	if !endTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, endTime)
	}
	query := "SELECT COALESCE(SUM(-balance_delta), 0) FROM admin_usage_calibrations " + buildWhere(conditions)
	var total float64
	if err := scanSingleRow(ctx, r.sql, query, args, &total); err != nil {
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
	conditions := []string{"target_user_id = ANY($1)", "balance_delta < 0"}
	args := []any{pq.Array(ids)}
	if !startTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, startTime)
	}
	if !endTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, endTime)
	}
	query := `
		SELECT target_user_id, COALESCE(SUM(-balance_delta), 0) AS spent
		FROM admin_usage_calibrations
		` + buildWhere(conditions) + `
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
		tokenMode      sql.NullString
		tokenInput     sql.NullInt64
		tokenBefore    sql.NullInt64
		tokenAfter     sql.NullInt64
		tokenDelta     sql.NullInt64
		tokenStartDate sql.NullString
		tokenEndDate   sql.NullString
		tokenTimezone  sql.NullString
		balanceMode    sql.NullString
		balanceInput   sql.NullFloat64
		balanceBefore  sql.NullFloat64
		balanceAfter   sql.NullFloat64
		balanceDelta   sql.NullFloat64
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
