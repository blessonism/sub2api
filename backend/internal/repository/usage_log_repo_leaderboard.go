package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func buildUsageLogFilterConditions(filters UsageLogFilters) ([]string, []any) {
	conditions := make([]string, 0, 9)
	args := make([]any, 0, 9)

	if filters.UserID > 0 {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", len(args)+1))
		args = append(args, filters.UserID)
	}
	if filters.APIKeyID > 0 {
		conditions = append(conditions, fmt.Sprintf("api_key_id = $%d", len(args)+1))
		args = append(args, filters.APIKeyID)
	}
	if filters.AccountID > 0 {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", len(args)+1))
		args = append(args, filters.AccountID)
	}
	if filters.GroupID > 0 {
		conditions = append(conditions, fmt.Sprintf("group_id = $%d", len(args)+1))
		args = append(args, filters.GroupID)
	}
	if requestID := strings.TrimSpace(filters.RequestID); requestID != "" {
		conditions = append(conditions, fmt.Sprintf("request_id = $%d", len(args)+1))
		args = append(args, requestID)
	}
	conditions, args = appendUsageLogModelWhereCondition(conditions, args, filters.Model, filters.ModelFilterSource)
	conditions, args = appendRequestTypeOrStreamWhereCondition(conditions, args, filters.RequestType, filters.Stream)
	if filters.BillingType != nil {
		conditions = append(conditions, fmt.Sprintf("billing_type = $%d", len(args)+1))
		args = append(args, int16(*filters.BillingType))
	}
	conditions, args = appendUsageLogBillingModeWhereCondition(conditions, args, filters.BillingMode)
	if filters.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, *filters.EndTime)
	}

	return conditions, args
}

func appendSharedIPUsersCondition(conditions []string) []string {
	baseWhereClause := buildWhere(conditions)
	ipConditions := make([]string, 0, 3)
	if baseWhereClause != "" {
		ipConditions = append(ipConditions, strings.TrimPrefix(baseWhereClause, "WHERE "))
	}
	ipConditions = append(ipConditions, "ip_address IS NOT NULL", "ip_address <> ''")
	conditions = append(conditions, fmt.Sprintf(`ip_address IN (
		SELECT ip_address
		FROM usage_logs
		%s
		GROUP BY ip_address
		HAVING COUNT(DISTINCT user_id) > 1
	)`, buildWhere(ipConditions)))
	return conditions
}

func (r *usageLogRepository) GetSharedIPUsersSummary(ctx context.Context, filters UsageLogFilters) (*usagestats.SharedIPUsersSummary, error) {
	conditions, args := buildUsageLogFilterConditions(filters)
	conditions = appendSharedIPUsersCondition(conditions)
	whereClause := buildWhere(conditions)

	query := "SELECT COUNT(DISTINCT ip_address), COUNT(DISTINCT user_id), COUNT(*) FROM usage_logs " + whereClause
	summary := &usagestats.SharedIPUsersSummary{}
	if err := scanSingleRow(ctx, r.sql, query, args, &summary.IPCount, &summary.UserCount, &summary.RecordCount); err != nil {
		return nil, err
	}
	ipGroups, ipGroupsTruncated, err := r.listSharedIPGroupSummaryItems(ctx, whereClause, args)
	if err != nil {
		return nil, err
	}
	summary.IPGroups = ipGroups
	summary.IPGroupsLimit = sharedIPGroupSummaryLimit
	summary.IPGroupsTruncated = ipGroupsTruncated || summary.IPCount > int64(len(ipGroups))
	if summary.IPCount > int64(len(ipGroups)) {
		summary.HiddenIPGroupCount = summary.IPCount - int64(len(ipGroups))
	}
	return summary, nil
}

const sharedIPGroupSummaryLimit = 50
const sharedIPGroupUserSummaryLimit = 20

func (r *usageLogRepository) listSharedIPGroupSummaryItems(ctx context.Context, whereClause string, args []any) ([]usagestats.SharedIPGroupSummaryItem, bool, error) {
	query := fmt.Sprintf(`
		WITH matched_logs AS (
			SELECT
				user_id,
				ip_address,
				created_at,
				input_tokens,
				output_tokens,
				cache_creation_tokens,
				cache_read_tokens,
				actual_cost
			FROM usage_logs
			`+whereClause+`
		),
		ip_summary AS (
			SELECT
				ip_address,
				COUNT(DISTINCT user_id) AS user_count,
				COUNT(*) AS record_count,
				MAX(created_at) AS last_used_at,
				COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
				COALESCE(SUM(actual_cost), 0) AS actual_cost
			FROM matched_logs
			GROUP BY ip_address
			ORDER BY record_count DESC, user_count DESC, last_used_at DESC, ip_address ASC
			LIMIT %d
		),
		user_summary AS (
			SELECT
				ml.ip_address,
				ml.user_id,
				COALESCE(u.email, '') AS email,
				u.deleted_at IS NOT NULL AS deleted,
				COUNT(*) AS record_count,
				MAX(ml.created_at) AS last_used_at,
				COALESCE(SUM(ml.input_tokens + ml.output_tokens + ml.cache_creation_tokens + ml.cache_read_tokens), 0) AS total_tokens,
				COALESCE(SUM(ml.actual_cost), 0) AS actual_cost,
				ROW_NUMBER() OVER (PARTITION BY ml.ip_address ORDER BY COUNT(*) DESC, MAX(ml.created_at) DESC, ml.user_id ASC) AS user_rank
			FROM matched_logs ml
			JOIN ip_summary ips ON ips.ip_address = ml.ip_address
			LEFT JOIN users u ON u.id = ml.user_id
			GROUP BY ml.ip_address, ml.user_id, u.email, u.deleted_at
		)
		SELECT
			ips.ip_address,
			ips.user_count,
			ips.record_count,
			ips.last_used_at,
			ips.total_tokens,
			ips.actual_cost,
			us.user_id,
			us.email,
			us.deleted,
			us.record_count,
			us.last_used_at,
			us.total_tokens,
			us.actual_cost
		FROM ip_summary ips
		JOIN user_summary us ON us.ip_address = ips.ip_address AND us.user_rank <= %d
		ORDER BY ips.record_count DESC, ips.user_count DESC, ips.last_used_at DESC, ips.ip_address ASC, us.user_rank ASC`, sharedIPGroupSummaryLimit+1, sharedIPGroupUserSummaryLimit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	groups := make([]usagestats.SharedIPGroupSummaryItem, 0)
	groupIndexByIP := make(map[string]int)
	for rows.Next() {
		var (
			group usagestats.SharedIPGroupSummaryItem
			user  usagestats.SharedIPGroupUserSummaryItem
		)
		if err := rows.Scan(
			&group.IPAddress,
			&group.UserCount,
			&group.RecordCount,
			&group.LastUsedAt,
			&group.TotalTokens,
			&group.ActualCost,
			&user.UserID,
			&user.Email,
			&user.Deleted,
			&user.RecordCount,
			&user.LastUsedAt,
			&user.TotalTokens,
			&user.ActualCost,
		); err != nil {
			return nil, false, err
		}

		groupIndex, ok := groupIndexByIP[group.IPAddress]
		if !ok {
			group.Users = make([]usagestats.SharedIPGroupUserSummaryItem, 0, minSharedIPSummaryInt(group.UserCount, int64(sharedIPGroupUserSummaryLimit)))
			group.UsersLimit = sharedIPGroupUserSummaryLimit
			group.UsersTruncated = group.UserCount > int64(sharedIPGroupUserSummaryLimit)
			if group.UsersTruncated {
				group.HiddenUserCount = group.UserCount - int64(sharedIPGroupUserSummaryLimit)
			}
			groups = append(groups, group)
			groupIndex = len(groups) - 1
			groupIndexByIP[group.IPAddress] = groupIndex
		}
		groups[groupIndex].Users = append(groups[groupIndex].Users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	truncated := len(groups) > sharedIPGroupSummaryLimit
	if truncated {
		groups = groups[:sharedIPGroupSummaryLimit]
	}
	return groups, truncated, nil
}

func minSharedIPSummaryInt(a, b int64) int {
	if a < b {
		return int(a)
	}
	return int(b)
}

// GetAdminTokenLeaderboard 返回管理员 Token 排行榜聚合结果。
func (r *usageLogRepository) GetAdminTokenLeaderboard(ctx context.Context, startTime, endTime time.Time, filters usagestats.AdminTokenLeaderboardFilters) (result *usagestats.AdminTokenLeaderboardResponse, err error) {
	if filters.Limit <= 0 {
		filters.Limit = 10
	}

	whereClause, args := buildAdminTokenLeaderboardWhere(startTime, endTime, filters, 0)
	calibrationStartDate, calibrationEndDateExclusive := calibrationAllocationDateRange(startTime, endTime)
	args = append(args, filters.Limit)
	limitPosition := len(args)
	includeCalibration := filters.GroupID == 0 && strings.TrimSpace(filters.Model) == ""
	calibrationCTE := ""
	userUsageCTE := `
		user_usage AS (
			SELECT
				user_id,
				email,
				username,
				status,
				registered_at,
				last_used_at,
				requests,
				tokens,
				cost,
				actual_cost,
				account_cost
			FROM raw_user_usage
		),`
	if includeCalibration {
		emailPosition, statusPosition := adminTokenLeaderboardCalibrationFilterPositions(filters, 0)
		args = append(args, calibrationStartDate, calibrationEndDateExclusive)
		calibrationWhere := buildAdminTokenLeaderboardCalibrationWhere(filters, emailPosition, statusPosition, len(args)-1, len(args))
		calibrationCTE = fmt.Sprintf(`,
		calibration_usage AS (
			SELECT
				acu.target_user_id AS user_id,
				COALESCE(SUM(acu.token_delta), 0) AS token_delta
			FROM admin_usage_calibration_daily_allocations acu
			LEFT JOIN users u ON u.id = acu.target_user_id
			WHERE %s
			GROUP BY acu.target_user_id
		)`, calibrationWhere)
		userUsageCTE = `
		user_usage AS (
			SELECT
				COALESCE(r.user_id, c.user_id) AS user_id,
				COALESCE(r.email, u.email, '') AS email,
				COALESCE(r.username, u.username, '') AS username,
				COALESCE(r.status, u.status, '') AS status,
				COALESCE(r.registered_at, u.created_at, to_timestamp(0)) AS registered_at,
				COALESCE(r.last_used_at, to_timestamp(0)) AS last_used_at,
				COALESCE(r.requests, 0) AS requests,
				COALESCE(r.tokens, 0) + COALESCE(c.token_delta, 0) AS tokens,
				COALESCE(r.cost, 0) AS cost,
				COALESCE(r.actual_cost, 0) AS actual_cost,
				COALESCE(r.account_cost, 0) AS account_cost
			FROM raw_user_usage r
			FULL OUTER JOIN calibration_usage c ON c.user_id = r.user_id
			LEFT JOIN users u ON u.id = COALESCE(r.user_id, c.user_id)
			WHERE COALESCE(r.requests, 0) > 0 OR COALESCE(r.tokens, 0) + COALESCE(c.token_delta, 0) <> 0
		),`
	}

	query := fmt.Sprintf(`
		WITH raw_user_usage AS (
			SELECT
				ul.user_id,
				COALESCE(u.email, '') as email,
				COALESCE(u.username, '') as username,
				COALESCE(u.status, '') as status,
				COALESCE(u.created_at, to_timestamp(0)) as registered_at,
				MAX(ul.created_at) as last_used_at,
				COUNT(*) as requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) as tokens,
				COALESCE(SUM(ul.total_cost), 0) as cost,
				COALESCE(SUM(ul.actual_cost), 0) as actual_cost,
				COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) as account_cost
			FROM usage_logs ul
			LEFT JOIN users u ON u.id = ul.user_id
			WHERE %s
			GROUP BY ul.user_id, u.email, u.username, u.status, u.created_at
		)%s,
		%s
		ranked AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY tokens DESC, actual_cost DESC, requests DESC, user_id ASC) as rank,
				user_id,
				email,
				username,
				status,
				registered_at,
				last_used_at,
				requests,
				tokens,
				cost,
				actual_cost,
				account_cost,
				COALESCE(SUM(requests) OVER (), 0) as total_requests,
				COALESCE(SUM(tokens) OVER (), 0) as total_tokens,
				COALESCE(SUM(cost) OVER (), 0) as total_cost,
				COALESCE(SUM(actual_cost) OVER (), 0) as total_actual_cost,
				COALESCE(SUM(account_cost) OVER (), 0) as total_account_cost
			FROM user_usage
		)
		SELECT
			rank,
			user_id,
			email,
			username,
			status,
			registered_at,
			last_used_at,
			requests,
			tokens,
			cost,
			actual_cost,
			account_cost,
			total_requests,
			total_tokens,
			total_cost,
			total_actual_cost,
			total_account_cost
		FROM ranked
		WHERE rank <= $%d
		ORDER BY rank ASC
	`, whereClause, calibrationCTE, userUsageCTE, limitPosition)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := &usagestats.AdminTokenLeaderboardResponse{
		Ranking: make([]usagestats.AdminTokenLeaderboardUser, 0, filters.Limit),
	}
	for rows.Next() {
		var row usagestats.AdminTokenLeaderboardUser
		if err = rows.Scan(
			&row.Rank,
			&row.UserID,
			&row.Email,
			&row.Username,
			&row.Status,
			&row.RegisteredAt,
			&row.LastUsedAt,
			&row.Requests,
			&row.Tokens,
			&row.Cost,
			&row.ActualCost,
			&row.AccountCost,
			&out.TotalRequests,
			&out.TotalTokens,
			&out.TotalCost,
			&out.TotalActualCost,
			&out.TotalAccountCost,
		); err != nil {
			return nil, err
		}
		out.Ranking = append(out.Ranking, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAdminTokenLeaderboardUserDetails 返回管理员排行榜中单个用户的展开明细。
func (r *usageLogRepository) GetAdminTokenLeaderboardUserDetails(ctx context.Context, startTime, endTime time.Time, userID int64, filters usagestats.AdminTokenLeaderboardFilters) (result *usagestats.AdminTokenLeaderboardUserDetails, err error) {
	out := &usagestats.AdminTokenLeaderboardUserDetails{}
	if userID <= 0 {
		return out, nil
	}

	if out.APIKeys, err = r.getAdminTokenLeaderboardAPIKeyDetails(ctx, startTime, endTime, userID, filters); err != nil {
		return nil, err
	}
	if out.Groups, err = r.getAdminTokenLeaderboardGroupDetails(ctx, startTime, endTime, userID, filters); err != nil {
		return nil, err
	}
	if out.Models, err = r.getAdminTokenLeaderboardModelDetails(ctx, startTime, endTime, userID, filters); err != nil {
		return nil, err
	}
	if filters.GroupID == 0 && strings.TrimSpace(filters.Model) == "" {
		startDate, endDateExclusive := calibrationAllocationDateRange(startTime, endTime)
		if out.CalibrationTokens, err = sumTokenAllocationsByDateRange(ctx, r.sql, userID, startDate, endDateExclusive); err != nil {
			return nil, err
		}
		if out.CalibrationBalanceDelta, err = sumBalanceCalibrationsByTimeRange(ctx, r.sql, userID, startTime, endTime); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func buildAdminTokenLeaderboardWhere(startTime, endTime time.Time, filters usagestats.AdminTokenLeaderboardFilters, userID int64) (string, []any) {
	conditions := []string{"ul.created_at >= $1", "ul.created_at < $2"}
	args := []any{startTime, endTime}

	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("ul.user_id = $%d", len(args)+1))
		args = append(args, userID)
	}
	if email := strings.TrimSpace(filters.Email); email != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(u.email, '')) LIKE $%d", len(args)+1))
		args = append(args, "%"+strings.ToLower(email)+"%")
	}
	if filters.GroupID > 0 {
		conditions = append(conditions, fmt.Sprintf("ul.group_id = $%d", len(args)+1))
		args = append(args, filters.GroupID)
	}
	if model := strings.TrimSpace(filters.Model); model != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", resolveModelDimensionExpression(filters.ModelType), len(args)+1))
		args = append(args, model)
	}
	if status := strings.TrimSpace(filters.UserStatus); status != "" {
		conditions = append(conditions, fmt.Sprintf("u.status = $%d", len(args)+1))
		args = append(args, status)
	}

	return strings.Join(conditions, " AND "), args
}

func calibrationAllocationDateRange(startTime, endTime time.Time) (string, string) {
	startDate := ""
	if !startTime.IsZero() {
		startDate = startTime.Format("2006-01-02")
	}
	endDateExclusive := ""
	if !endTime.IsZero() {
		endDateExclusive = calibrationExclusiveDateForTime(endTime)
	}
	return startDate, endDateExclusive
}

func calibrationExclusiveDateForTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format("2006-01-02")
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

func adminTokenLeaderboardCalibrationFilterPositions(filters usagestats.AdminTokenLeaderboardFilters, userID int64) (emailPosition, statusPosition int) {
	argPosition := 3
	if userID > 0 {
		argPosition++
	}
	if email := strings.TrimSpace(filters.Email); email != "" {
		emailPosition = argPosition
		argPosition++
	}
	if filters.GroupID > 0 {
		argPosition++
	}
	if model := strings.TrimSpace(filters.Model); model != "" {
		argPosition++
	}
	if status := strings.TrimSpace(filters.UserStatus); status != "" {
		statusPosition = argPosition
	}
	return emailPosition, statusPosition
}

func buildAdminTokenLeaderboardCalibrationWhere(filters usagestats.AdminTokenLeaderboardFilters, emailPosition, statusPosition, startDatePosition, endDatePosition int) string {
	conditions := []string{
		fmt.Sprintf("acu.allocation_date >= $%d::date", startDatePosition),
		fmt.Sprintf("acu.allocation_date < $%d::date", endDatePosition),
	}
	if email := strings.TrimSpace(filters.Email); email != "" && emailPosition > 0 {
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(u.email, '')) LIKE $%d", emailPosition))
	}
	if status := strings.TrimSpace(filters.UserStatus); status != "" && statusPosition > 0 {
		conditions = append(conditions, fmt.Sprintf("u.status = $%d", statusPosition))
	}
	return strings.Join(conditions, " AND ")
}

func (r *usageLogRepository) getAdminTokenLeaderboardAPIKeyDetails(ctx context.Context, startTime, endTime time.Time, userID int64, filters usagestats.AdminTokenLeaderboardFilters) (result []usagestats.AdminTokenLeaderboardAPIKeyUsage, err error) {
	whereClause, args := buildAdminTokenLeaderboardWhere(startTime, endTime, filters, userID)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(ul.api_key_id, 0) as api_key_id,
			COALESCE(k.name, '') as api_key_name,
			COUNT(*) as requests,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) as tokens,
			COALESCE(SUM(ul.total_cost), 0) as cost,
			COALESCE(SUM(ul.actual_cost), 0) as actual_cost,
			COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) as account_cost
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN api_keys k ON k.id = ul.api_key_id
		WHERE %s
		GROUP BY ul.api_key_id, k.name
		ORDER BY tokens DESC, actual_cost DESC, requests DESC, api_key_id ASC
	`, whereClause)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	result = make([]usagestats.AdminTokenLeaderboardAPIKeyUsage, 0)
	for rows.Next() {
		var row usagestats.AdminTokenLeaderboardAPIKeyUsage
		if err = rows.Scan(&row.APIKeyID, &row.APIKeyName, &row.Requests, &row.Tokens, &row.Cost, &row.ActualCost, &row.AccountCost); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *usageLogRepository) getAdminTokenLeaderboardGroupDetails(ctx context.Context, startTime, endTime time.Time, userID int64, filters usagestats.AdminTokenLeaderboardFilters) (result []usagestats.AdminTokenLeaderboardGroupUsage, err error) {
	whereClause, args := buildAdminTokenLeaderboardWhere(startTime, endTime, filters, userID)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(ul.group_id, 0) as group_id,
			COALESCE(g.name, '') as group_name,
			COUNT(*) as requests,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) as tokens,
			COALESCE(SUM(ul.total_cost), 0) as cost,
			COALESCE(SUM(ul.actual_cost), 0) as actual_cost,
			COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) as account_cost
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE %s
		GROUP BY ul.group_id, g.name
		ORDER BY tokens DESC, actual_cost DESC, requests DESC, group_id ASC
	`, whereClause)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	result = make([]usagestats.AdminTokenLeaderboardGroupUsage, 0)
	for rows.Next() {
		var row usagestats.AdminTokenLeaderboardGroupUsage
		if err = rows.Scan(&row.GroupID, &row.GroupName, &row.Requests, &row.Tokens, &row.Cost, &row.ActualCost, &row.AccountCost); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *usageLogRepository) getAdminTokenLeaderboardModelDetails(ctx context.Context, startTime, endTime time.Time, userID int64, filters usagestats.AdminTokenLeaderboardFilters) (result []usagestats.AdminTokenLeaderboardModelUsage, err error) {
	whereClause, args := buildAdminTokenLeaderboardWhere(startTime, endTime, filters, userID)
	modelExpr := resolveModelDimensionExpression(filters.ModelType)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(TRIM(%s), ''), '') as model,
			COUNT(*) as requests,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) as tokens,
			COALESCE(SUM(ul.total_cost), 0) as cost,
			COALESCE(SUM(ul.actual_cost), 0) as actual_cost,
			COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) as account_cost
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		WHERE %s
		GROUP BY 1
		ORDER BY tokens DESC, actual_cost DESC, requests DESC, 1 ASC
	`, modelExpr, whereClause)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	result = make([]usagestats.AdminTokenLeaderboardModelUsage, 0)
	for rows.Next() {
		var row usagestats.AdminTokenLeaderboardModelUsage
		if err = rows.Scan(&row.Model, &row.Requests, &row.Tokens, &row.Cost, &row.ActualCost, &row.AccountCost); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUserTokenLeaderboard 返回今日 Token 消耗榜单，并额外带回当前用户排名。
func (r *usageLogRepository) GetUserTokenLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) (result *usagestats.UserTokenLeaderboardRows, err error) {
	if limit <= 0 {
		limit = 10
	}
	calibrationStartDate, calibrationEndDateExclusive := calibrationAllocationDateRange(startTime, endTime)

	query := `
		WITH raw_usage AS (
			SELECT
				ul.user_id,
				COUNT(*) as requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) as tokens
			FROM usage_logs ul
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			GROUP BY ul.user_id
		),
		calibration_usage AS (
			SELECT
				target_user_id AS user_id,
				COALESCE(SUM(token_delta), 0) AS token_delta
			FROM admin_usage_calibration_daily_allocations
			WHERE allocation_date >= $5::date AND allocation_date < $6::date
			GROUP BY target_user_id
		),
		user_usage AS (
			SELECT
				COALESCE(r.user_id, c.user_id) AS user_id,
				COALESCE(u.email, '') AS email,
				COALESCE(r.requests, 0) AS requests,
				COALESCE(r.tokens, 0) + COALESCE(c.token_delta, 0) AS tokens
			FROM raw_usage r
			FULL OUTER JOIN calibration_usage c ON c.user_id = r.user_id
			LEFT JOIN users u ON u.id = COALESCE(r.user_id, c.user_id)
			WHERE COALESCE(r.requests, 0) > 0 OR COALESCE(r.tokens, 0) + COALESCE(c.token_delta, 0) <> 0
		),
			auto_multipliers AS (
				SELECT
					a.user_id,
					MIN(
						CASE
							WHEN ugr.visible_rate_multiplier IS NOT NULL THEN ugr.visible_rate_multiplier
							ELSE LEAST(ugr.rate_multiplier, COALESCE(target_group.visible_rate_multiplier, target_group.rate_multiplier))
						END
					) AS rate_multiplier
				FROM token_usage_auto_assignments a
				JOIN token_usage_auto_policies p ON p.id = a.policy_id AND p.enabled = TRUE
				JOIN groups target_group ON target_group.id = a.target_group_id AND target_group.status = '` + service.StatusActive + `'
				JOIN user_group_rate_multipliers ugr ON ugr.user_id = a.user_id AND ugr.group_id = a.target_group_id
				WHERE a.last_rate_multiplier IS NOT NULL
				  AND a.manual_takeover = FALSE
				  AND ugr.rate_multiplier = a.last_rate_multiplier
				GROUP BY a.user_id
			),
			common_multiplier AS (
				SELECT COALESCE(g.visible_rate_multiplier, g.rate_multiplier) AS rate_multiplier
				FROM settings s
				JOIN groups g ON g.id = CASE WHEN s.value ~ '^[0-9]+$' THEN s.value::bigint ELSE 0 END
				WHERE s.key = '` + service.SettingKeyTokenLeaderboardCommonGroupID + `'
				  AND g.status = '` + service.StatusActive + `'
				  AND g.subscription_type = '` + service.SubscriptionTypeStandard + `'
			  AND g.is_exclusive = FALSE
			LIMIT 1
		),
		ranked AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY uu.tokens DESC, uu.requests DESC, uu.user_id ASC) as rank,
				uu.user_id,
					uu.email,
					uu.requests,
					uu.tokens,
					COALESCE(am.rate_multiplier, (SELECT rate_multiplier FROM common_multiplier)) AS discount_rate_multiplier
				FROM user_usage uu
				LEFT JOIN auto_multipliers am ON am.user_id = uu.user_id
			),
		selected AS (
			SELECT 'top' as row_type, rank, user_id, email, requests, tokens, discount_rate_multiplier
			FROM ranked
			WHERE rank <= $3
			UNION ALL
			SELECT 'current' as row_type, rank, user_id, email, requests, tokens, discount_rate_multiplier
			FROM ranked
			WHERE user_id = $4
			  AND NOT EXISTS (
				SELECT 1 FROM ranked WHERE user_id = $4 AND rank <= $3
			  )
		)
		SELECT row_type, rank, user_id, email, requests, tokens, discount_rate_multiplier
		FROM selected
		ORDER BY CASE WHEN row_type = 'top' THEN 0 ELSE 1 END, rank ASC
	`

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, limit, currentUserID, calibrationStartDate, calibrationEndDateExclusive)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := &usagestats.UserTokenLeaderboardRows{
		Ranking: make([]usagestats.UserTokenLeaderboardRow, 0, limit),
	}
	for rows.Next() {
		var rowType string
		var row usagestats.UserTokenLeaderboardRow
		var discountRate sql.NullFloat64
		if err = rows.Scan(&rowType, &row.Rank, &row.UserID, &row.Email, &row.Requests, &row.Tokens, &discountRate); err != nil {
			return nil, err
		}
		row.DiscountRateMultiplier = nullFloat64Ptr(discountRate)
		if row.UserID == currentUserID {
			rowCopy := row
			out.MyRank = &rowCopy
		}
		if rowType == "top" {
			out.Ranking = append(out.Ranking, row)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
