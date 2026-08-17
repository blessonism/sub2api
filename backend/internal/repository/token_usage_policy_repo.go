package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type tokenUsagePolicyRepository struct {
	db *sql.DB
}

const tokenUsagePolicyRunningStaleAfter = 30 * time.Minute

// tokenUsageAutoTotalsRefreshLockID 串行化用户累计用量刷新的事务级咨询锁 ID，
// 避免不同策略并发执行时基于同一水位重复累加同一批 usage_logs。
const tokenUsageAutoTotalsRefreshLockID int64 = 694208311321144028

func NewTokenUsageAutoPolicyRepository(db *sql.DB) service.TokenUsageAutoPolicyRepository {
	return &tokenUsagePolicyRepository{db: db}
}

func (r *tokenUsagePolicyRepository) ListPolicies(ctx context.Context, params pagination.PaginationParams, filters service.TokenUsageAutoPolicyListFilters) ([]service.TokenUsageAutoPolicy, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	conditions := []string{"1=1"}
	args := []any{}
	if filters.Enabled != nil {
		args = append(args, *filters.Enabled)
		conditions = append(conditions, fmt.Sprintf("p.enabled = $%d", len(args)))
	}
	if filters.TargetGroupID > 0 {
		args = append(args, filters.TargetGroupID)
		conditions = append(conditions, fmt.Sprintf("p.target_group_id = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	if err := scanSingleRow(ctx, r.db, "SELECT COUNT(*) FROM token_usage_auto_policies p WHERE "+where, args, &total); err != nil {
		return nil, nil, err
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.enabled, p.window_days, p.target_group_id, COALESCE(g.name, ''),
		       p.action_mode, p.conflict_mode, p.schedule_frequency,
		       p.filter_group_id, p.filter_model, p.filter_request_type, p.filter_billing_type,
		       p.last_run_at, p.next_run_at, p.created_at, p.updated_at
		FROM token_usage_auto_policies p
		LEFT JOIN groups g ON g.id = p.target_group_id
		WHERE `+where+`
		ORDER BY p.created_at DESC
		LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	policies, err := scanTokenUsagePolicies(rows)
	if err != nil {
		return nil, nil, err
	}
	if err := r.attachPolicyChildren(ctx, policies); err != nil {
		return nil, nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return policies, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (r *tokenUsagePolicyRepository) GetPolicyByID(ctx context.Context, id int64) (*service.TokenUsageAutoPolicy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.enabled, p.window_days, p.target_group_id, COALESCE(g.name, ''),
		       p.action_mode, p.conflict_mode, p.schedule_frequency,
		       p.filter_group_id, p.filter_model, p.filter_request_type, p.filter_billing_type,
		       p.last_run_at, p.next_run_at, p.created_at, p.updated_at
		FROM token_usage_auto_policies p
		LEFT JOIN groups g ON g.id = p.target_group_id
		WHERE p.id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	policies, err := scanTokenUsagePolicies(rows)
	if err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return nil, tokenUsagePolicyNotFoundError(sql.ErrNoRows)
	}
	if err := r.attachPolicyChildren(ctx, policies); err != nil {
		return nil, err
	}
	return &policies[0], nil
}

func (r *tokenUsagePolicyRepository) CreatePolicy(ctx context.Context, policy *service.TokenUsageAutoPolicy, tiers []service.TokenUsageAutoPolicyTier) (*service.TokenUsageAutoPolicy, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	err = scanSingleRow(ctx, tx, `
		INSERT INTO token_usage_auto_policies (
			name, enabled, window_days, target_group_id, action_mode, conflict_mode, schedule_frequency,
			filter_group_id, filter_model, filter_request_type, filter_billing_type, next_run_at, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW())
		RETURNING id
	`, []any{
		policy.Name, policy.Enabled, policy.WindowDays, policy.TargetGroupID, policy.ActionMode, policy.ConflictMode, policy.ScheduleFrequency,
		policy.Filters.GroupID, policy.Filters.Model, policy.Filters.RequestType, policy.Filters.BillingType, policy.NextRunAt,
	}, &id)
	if err != nil {
		return nil, translateTokenUsagePolicyWriteError(err)
	}
	if err := insertTokenUsagePolicyTiers(ctx, tx, id, tiers); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPolicyByID(ctx, id)
}

func (r *tokenUsagePolicyRepository) UpdatePolicy(ctx context.Context, policy *service.TokenUsageAutoPolicy, tiers []service.TokenUsageAutoPolicyTier) (*service.TokenUsageAutoPolicy, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	nextRunAt := policy.NextRunAt
	if !policy.Enabled {
		nextRunAt = nil
	}
	res, err := tx.ExecContext(ctx, `
			UPDATE token_usage_auto_policies
			SET name=$2, enabled=$3, window_days=$4, target_group_id=$5, action_mode=$6, conflict_mode=$7,
			    schedule_frequency=$8, filter_group_id=$9, filter_model=$10, filter_request_type=$11,
		    filter_billing_type=$12, next_run_at=$13, updated_at=NOW()
		WHERE id=$1
	`, policy.ID, policy.Name, policy.Enabled, policy.WindowDays, policy.TargetGroupID, policy.ActionMode, policy.ConflictMode,
		policy.ScheduleFrequency, policy.Filters.GroupID, policy.Filters.Model, policy.Filters.RequestType, policy.Filters.BillingType, nextRunAt)
	if err != nil {
		return nil, translateTokenUsagePolicyWriteError(err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, tokenUsagePolicyNotFoundError(sql.ErrNoRows)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM token_usage_auto_policy_tiers WHERE policy_id = $1`, policy.ID); err != nil {
		return nil, err
	}
	if err := insertTokenUsagePolicyTiers(ctx, tx, policy.ID, tiers); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPolicyByID(ctx, policy.ID)
}

func (r *tokenUsagePolicyRepository) DeletePolicy(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM token_usage_auto_policies WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return tokenUsagePolicyNotFoundError(sql.ErrNoRows)
	}
	return nil
}

func (r *tokenUsagePolicyRepository) CountAssignments(ctx context.Context, policyID int64) (int64, error) {
	var count int64
	err := scanSingleRow(ctx, r.db, `
		SELECT COUNT(*)
		FROM token_usage_auto_assignments
		WHERE policy_id = $1
		  AND (last_rate_multiplier IS NOT NULL OR group_granted_by_policy = TRUE OR last_applied_at IS NOT NULL)
	`, []any{policyID}, &count)
	return count, err
}

func (r *tokenUsagePolicyRepository) ListDuePolicies(ctx context.Context, now time.Time, limit int) ([]service.TokenUsageAutoPolicy, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.enabled, p.window_days, p.target_group_id, COALESCE(g.name, ''),
		       p.action_mode, p.conflict_mode, p.schedule_frequency,
		       p.filter_group_id, p.filter_model, p.filter_request_type, p.filter_billing_type,
		       p.last_run_at, p.next_run_at, p.created_at, p.updated_at
		FROM token_usage_auto_policies p
		LEFT JOIN groups g ON g.id = p.target_group_id
		WHERE p.enabled = TRUE AND p.next_run_at IS NOT NULL AND p.next_run_at <= $1
		ORDER BY p.next_run_at ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	policies, err := scanTokenUsagePolicies(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachPolicyChildren(ctx, policies); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *tokenUsagePolicyRepository) TryLockPolicy(ctx context.Context, policyID int64) (bool, error) {
	cutoff := time.Now().UTC().Add(-tokenUsagePolicyRunningStaleAfter)
	if err := r.recoverStalePolicyRuns(ctx, policyID, cutoff); err != nil {
		return false, err
	}
	var exists bool
	if err := scanSingleRow(ctx, r.db, `
		SELECT EXISTS (
			SELECT 1 FROM token_usage_auto_runs
			WHERE policy_id = $1 AND status = $2
		)
	`, []any{policyID, service.TokenUsagePolicyRunStatusRunning}, &exists); err != nil {
		return false, err
	}
	return !exists, nil
}

func (r *tokenUsagePolicyRepository) AggregatePolicyUsage(ctx context.Context, policy service.TokenUsageAutoPolicy, since time.Time) ([]service.TokenUsageAutoPolicyUsageRow, error) {
	conditions := []string{"ul.created_at >= $1", "ul.actual_cost > 0", "ul.user_id IS NOT NULL", "u.deleted_at IS NULL"}
	args := []any{since}
	if policy.Filters.GroupID != nil {
		args = append(args, *policy.Filters.GroupID)
		conditions = append(conditions, fmt.Sprintf("ul.group_id = $%d", len(args)))
	}
	if policy.Filters.Model != nil && strings.TrimSpace(*policy.Filters.Model) != "" {
		args = append(args, strings.TrimSpace(*policy.Filters.Model))
		conditions = append(conditions, fmt.Sprintf("ul.model = $%d", len(args)))
	}
	if policy.Filters.RequestType != nil {
		args = append(args, *policy.Filters.RequestType)
		conditions = append(conditions, fmt.Sprintf("ul.request_type = $%d", len(args)))
	}
	if policy.Filters.BillingType != nil {
		args = append(args, *policy.Filters.BillingType)
		conditions = append(conditions, fmt.Sprintf("ul.billing_type = $%d", len(args)))
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT ul.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''),
		       COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0)::bigint AS token_usage,
		       COALESCE(SUM(ul.actual_cost), 0)::double precision AS actual_cost
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		WHERE `+strings.Join(conditions, " AND ")+`
		GROUP BY ul.user_id, u.username, u.email
		ORDER BY token_usage DESC, ul.user_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []service.TokenUsageAutoPolicyUsageRow{}
	for rows.Next() {
		var row service.TokenUsageAutoPolicyUsageRow
		if err := rows.Scan(&row.UserID, &row.UserName, &row.UserEmail, &row.TokenUsage, &row.ActualCost); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// RefreshUserUsageTotals 按 usage_logs.id 水位增量刷新用户全历史累计用量，并返回最新累计值。
// 首次遇到用户时 last_processed_id=0，等价于全历史计算一次；后续只处理新增日志。
func (r *tokenUsagePolicyRepository) RefreshUserUsageTotals(ctx context.Context, userIDs []int64) (map[int64]service.TokenUsageAutoUserTotal, error) {
	result := map[int64]service.TokenUsageAutoUserTotal{}
	unique := uniquePositiveInt64(userIDs)
	if len(unique) == 0 {
		return result, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 事务级咨询锁：先锁后算，保证并发策略执行时水位读取与增量累加原子可见。
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, tokenUsageAutoTotalsRefreshLockID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		WITH delta AS (
			SELECT ul.user_id,
			       COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0)
			                    + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0)::bigint AS add_tokens,
			       COALESCE(SUM(ul.actual_cost), 0)::double precision AS add_cost,
			       MAX(ul.id) AS max_id
			FROM usage_logs ul
			JOIN users u ON u.id = ul.user_id AND u.deleted_at IS NULL
			LEFT JOIN token_usage_auto_user_totals t ON t.user_id = ul.user_id
			WHERE ul.user_id = ANY($1)
			  AND ul.actual_cost > 0
			  AND (t.user_id IS NULL OR ul.id > t.last_processed_id)
			GROUP BY ul.user_id
		)
		INSERT INTO token_usage_auto_user_totals (user_id, total_tokens, total_actual_cost, last_processed_id, updated_at)
		SELECT user_id, add_tokens, add_cost, max_id, NOW() FROM delta
		ON CONFLICT (user_id) DO UPDATE SET
			total_tokens      = token_usage_auto_user_totals.total_tokens + EXCLUDED.total_tokens,
			total_actual_cost = token_usage_auto_user_totals.total_actual_cost + EXCLUDED.total_actual_cost,
			last_processed_id = GREATEST(token_usage_auto_user_totals.last_processed_id, EXCLUDED.last_processed_id),
			updated_at        = NOW()
	`, pq.Array(unique)); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT user_id, total_tokens, total_actual_cost
		FROM token_usage_auto_user_totals
		WHERE user_id = ANY($1)
	`, pq.Array(unique))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var total service.TokenUsageAutoUserTotal
		if err := rows.Scan(&total.UserID, &total.TotalTokenUsage, &total.TotalActualCost); err != nil {
			return nil, err
		}
		result[total.UserID] = total
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *tokenUsagePolicyRepository) ListPolicyStates(ctx context.Context, policyID, targetGroupID int64, userIDs []int64) (map[int64]service.TokenUsageAutoPolicyState, error) {
	result := map[int64]service.TokenUsageAutoPolicyState{}
	unique := uniquePositiveInt64(userIDs)
	if len(unique) > 0 {
		rows, err := r.db.QueryContext(ctx, `
			SELECT u.id, COALESCE(u.username, ''), COALESCE(u.email, ''), ugr.rate_multiplier,
				       EXISTS (
				           SELECT 1 FROM user_allowed_groups uag
				           WHERE uag.user_id = u.id AND uag.group_id = $1
				       ) AS has_allowed_group
			FROM users u
			LEFT JOIN user_group_rate_multipliers ugr ON ugr.user_id = u.id AND ugr.group_id = $1
			WHERE u.id = ANY($2) AND u.deleted_at IS NULL
		`, targetGroupID, pq.Array(unique))
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var userID int64
			var username, email string
			var rate sql.NullFloat64
			var hasAllowed bool
			if err := rows.Scan(&userID, &username, &email, &rate, &hasAllowed); err != nil {
				return nil, err
			}
			state := service.TokenUsageAutoPolicyState{UserID: userID, UserName: username, UserEmail: email, HasAllowedGroup: hasAllowed}
			if rate.Valid {
				v := rate.Float64
				state.CurrentRate = &v
			}
			result[userID] = state
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.policy_id, a.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''),
		       a.target_group_id, a.tier_id, a.last_token_usage, a.last_actual_cost,
		       a.resident_tier_id, a.last_total_token_usage, a.last_total_actual_cost, a.last_rate_multiplier,
		       a.group_granted_by_policy, a.previous_rate_multiplier, a.manual_takeover,
		       COALESCE(a.manual_takeover_reason, ''), a.manual_takeover_at, a.last_applied_at,
		       a.created_at, a.updated_at,
		       ugr.rate_multiplier,
			       EXISTS (
			           SELECT 1 FROM user_allowed_groups uag
			           WHERE uag.user_id = a.user_id AND uag.group_id = a.target_group_id
			       ) AS has_allowed_group
		FROM token_usage_auto_assignments a
			JOIN users u ON u.id = a.user_id AND u.deleted_at IS NULL
			LEFT JOIN user_group_rate_multipliers ugr ON ugr.user_id = a.user_id AND ugr.group_id = a.target_group_id
			WHERE a.policy_id = $1
			  AND a.target_group_id = $2
		`, policyID, targetGroupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		assignment, currentRate, hasAllowed, err := scanTokenUsageAssignmentState(rows)
		if err != nil {
			return nil, err
		}
		state := result[assignment.UserID]
		state.UserID = assignment.UserID
		state.UserName = assignment.UserName
		state.UserEmail = assignment.UserEmail
		state.CurrentRate = currentRate
		state.HasAllowedGroup = hasAllowed
		state.Assignment = &assignment
		result[assignment.UserID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *tokenUsagePolicyRepository) ListPolicyAssignmentStates(ctx context.Context, policyID int64) (map[int64]service.TokenUsageAutoPolicyState, error) {
	result := map[int64]service.TokenUsageAutoPolicyState{}
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.policy_id, a.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''),
		       a.target_group_id, a.tier_id, a.last_token_usage, a.last_actual_cost,
		       a.resident_tier_id, a.last_total_token_usage, a.last_total_actual_cost, a.last_rate_multiplier,
		       a.group_granted_by_policy, a.previous_rate_multiplier, a.manual_takeover,
		       COALESCE(a.manual_takeover_reason, ''), a.manual_takeover_at, a.last_applied_at,
		       a.created_at, a.updated_at,
		       ugr.rate_multiplier,
		       EXISTS (
		           SELECT 1 FROM user_allowed_groups uag
		           WHERE uag.user_id = a.user_id AND uag.group_id = a.target_group_id
		       ) AS has_allowed_group
		FROM token_usage_auto_assignments a
			LEFT JOIN users u ON u.id = a.user_id
			LEFT JOIN user_group_rate_multipliers ugr ON ugr.user_id = a.user_id AND ugr.group_id = a.target_group_id
		WHERE a.policy_id = $1
		ORDER BY a.user_id ASC
	`, policyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		assignment, currentRate, hasAllowed, err := scanTokenUsageAssignmentState(rows)
		if err != nil {
			return nil, err
		}
		result[assignment.UserID] = service.TokenUsageAutoPolicyState{
			UserID:          assignment.UserID,
			UserName:        assignment.UserName,
			UserEmail:       assignment.UserEmail,
			CurrentRate:     currentRate,
			HasAllowedGroup: hasAllowed,
			Assignment:      &assignment,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *tokenUsagePolicyRepository) ApplyPolicyChanges(ctx context.Context, policy service.TokenUsageAutoPolicy, changes []service.TokenUsageAutoPolicyChange, nextRunAt *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.applyPolicyChangesInTx(ctx, tx, policy, changes, nextRunAt, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *tokenUsagePolicyRepository) ApplyPolicyChangesAndFinishRun(ctx context.Context, runID int64, policy service.TokenUsageAutoPolicy, changes []service.TokenUsageAutoPolicyChange, stats service.TokenUsageAutoPolicyRunStats, nextRunAt *time.Time, disablePolicy bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.applyPolicyChangesInTx(ctx, tx, policy, changes, nextRunAt, disablePolicy); err != nil {
		return err
	}
	if err := insertTokenUsageRunChanges(ctx, tx, runID, policy.ID, changes); err != nil {
		return err
	}
	if err := r.finishPolicyRunSummary(ctx, tx, runID, service.TokenUsagePolicyRunStatusSuccess, stats, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *tokenUsagePolicyRepository) applyPolicyChangesInTx(ctx context.Context, tx *sql.Tx, policy service.TokenUsageAutoPolicy, changes []service.TokenUsageAutoPolicyChange, nextRunAt *time.Time, disablePolicy bool) error {
	now := time.Now().UTC()
	for i := range changes {
		change := &changes[i]
		switch change.ChangeType {
		case service.TokenUsagePolicyChangeCreate, service.TokenUsagePolicyChangeUpdate, service.TokenUsagePolicyChangeDowngrade:
			if change.NewRateMultiplier == nil || change.TierID == nil {
				continue
			}
			grantByPolicy := false
			if policy.ActionMode == service.TokenUsagePolicyActionGrantGroupAndRate {
				res, err := tx.ExecContext(ctx, `
							INSERT INTO user_allowed_groups (user_id, group_id, created_at)
							VALUES ($1, $2, NOW())
							ON CONFLICT (user_id, group_id) DO NOTHING
						`, change.UserID, policy.TargetGroupID)
				if err != nil {
					return err
				}
				if affected, _ := res.RowsAffected(); affected > 0 {
					grantByPolicy = true
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, created_at, updated_at)
				VALUES ($1, $2, $3, NOW(), NOW())
				ON CONFLICT (user_id, group_id)
				DO UPDATE SET rate_multiplier = EXCLUDED.rate_multiplier, updated_at = EXCLUDED.updated_at
			`, change.UserID, policy.TargetGroupID, *change.NewRateMultiplier); err != nil {
				return err
			}
			previousRate := change.OldRateMultiplier
			var existingPrevious sql.NullFloat64
			_ = scanSingleRow(ctx, tx, `SELECT previous_rate_multiplier FROM token_usage_auto_assignments WHERE policy_id=$1 AND user_id=$2`, []any{policy.ID, change.UserID}, &existingPrevious)
			if existingPrevious.Valid {
				v := existingPrevious.Float64
				previousRate = &v
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO token_usage_auto_assignments (
					policy_id, user_id, target_group_id, tier_id, last_token_usage, last_actual_cost,
					last_total_token_usage, last_total_actual_cost, resident_tier_id, last_rate_multiplier,
					group_granted_by_policy, previous_rate_multiplier, manual_takeover, manual_takeover_reason,
					manual_takeover_at, last_applied_at, created_at, updated_at
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,FALSE,NULL,NULL,$13,NOW(),NOW())
				ON CONFLICT (policy_id, user_id)
				DO UPDATE SET
					target_group_id = EXCLUDED.target_group_id,
					tier_id = EXCLUDED.tier_id,
					last_token_usage = EXCLUDED.last_token_usage,
					last_actual_cost = EXCLUDED.last_actual_cost,
					last_total_token_usage = EXCLUDED.last_total_token_usage,
					last_total_actual_cost = EXCLUDED.last_total_actual_cost,
					resident_tier_id = EXCLUDED.resident_tier_id,
					last_rate_multiplier = EXCLUDED.last_rate_multiplier,
					group_granted_by_policy = token_usage_auto_assignments.group_granted_by_policy OR EXCLUDED.group_granted_by_policy,
					previous_rate_multiplier = COALESCE(token_usage_auto_assignments.previous_rate_multiplier, EXCLUDED.previous_rate_multiplier),
					manual_takeover = FALSE,
					manual_takeover_reason = NULL,
					manual_takeover_at = NULL,
					last_applied_at = EXCLUDED.last_applied_at,
					updated_at = NOW()
			`, policy.ID, change.UserID, policy.TargetGroupID, *change.TierID, change.TokenUsage, change.ActualCost,
				change.TotalTokenUsage, change.TotalActualCost, change.ResidentTierID, *change.NewRateMultiplier, grantByPolicy, previousRate, now); err != nil {
				return err
			}
		case service.TokenUsagePolicyChangeClear:
			if err := r.applyClearChange(ctx, tx, policy, change); err != nil {
				return err
			}
		case service.TokenUsagePolicyChangeSkipManual:
			clearPolicyGrantedGroupOwnership := shouldClearPolicyGrantedGroupOwnership(change)
			if _, err := tx.ExecContext(ctx, `
					INSERT INTO token_usage_auto_assignments (
						policy_id, user_id, target_group_id, tier_id, last_token_usage, last_actual_cost,
						last_total_token_usage, last_total_actual_cost, resident_tier_id, last_rate_multiplier,
						group_granted_by_policy, previous_rate_multiplier, manual_takeover, manual_takeover_reason,
						manual_takeover_at, last_applied_at, created_at, updated_at
					)
						VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,FALSE,$10,TRUE,$11,NOW(),NULL,NOW(),NOW())
						ON CONFLICT (policy_id, user_id)
						DO UPDATE SET
							target_group_id = EXCLUDED.target_group_id,
							tier_id = EXCLUDED.tier_id,
							last_token_usage = EXCLUDED.last_token_usage,
							last_actual_cost = EXCLUDED.last_actual_cost,
							last_total_token_usage = EXCLUDED.last_total_token_usage,
							last_total_actual_cost = EXCLUDED.last_total_actual_cost,
							resident_tier_id = EXCLUDED.resident_tier_id,
							previous_rate_multiplier = COALESCE(token_usage_auto_assignments.previous_rate_multiplier, EXCLUDED.previous_rate_multiplier),
							group_granted_by_policy = CASE
							WHEN $12 THEN FALSE
								ELSE token_usage_auto_assignments.group_granted_by_policy
							END,
						manual_takeover = TRUE,
						manual_takeover_reason = EXCLUDED.manual_takeover_reason,
						manual_takeover_at = COALESCE(token_usage_auto_assignments.manual_takeover_at, NOW()),
						updated_at = NOW()
				`, policy.ID, change.UserID, policy.TargetGroupID, change.TierID, change.TokenUsage, change.ActualCost,
					change.TotalTokenUsage, change.TotalActualCost, change.ResidentTierID, change.OldRateMultiplier, change.Reason, clearPolicyGrantedGroupOwnership); err != nil {
				return err
			}
		}
	}

	if disablePolicy {
		if _, err := tx.ExecContext(ctx, `
			UPDATE token_usage_auto_policies
			SET enabled = FALSE, last_run_at = $2, next_run_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, policy.ID, now); err != nil {
			return err
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE token_usage_auto_policies
		SET last_run_at = $2, next_run_at = $3, updated_at = NOW()
		WHERE id = $1
	`, policy.ID, now, nextRunAt); err != nil {
		return err
	}
	return nil
}

func (r *tokenUsagePolicyRepository) applyClearChange(ctx context.Context, tx *sql.Tx, policy service.TokenUsageAutoPolicy, change *service.TokenUsageAutoPolicyChange) error {
	var previous sql.NullFloat64
	var lastAutoRate sql.NullFloat64
	var currentRate sql.NullFloat64
	var granted bool
	var targetGroupID int64
	err := scanSingleRow(ctx, tx, `
		SELECT a.previous_rate_multiplier, a.last_rate_multiplier, ugr.rate_multiplier,
		       a.group_granted_by_policy, a.target_group_id
		FROM token_usage_auto_assignments a
		LEFT JOIN user_group_rate_multipliers ugr ON ugr.user_id = a.user_id AND ugr.group_id = a.target_group_id
		WHERE a.policy_id = $1 AND a.user_id = $2
	`, []any{policy.ID, change.UserID}, &previous, &lastAutoRate, &currentRate, &granted, &targetGroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if targetGroupID <= 0 {
		targetGroupID = policy.TargetGroupID
	}
	change.TargetGroupID = targetGroupID
	change.GroupGranted = granted
	if currentRate.Valid {
		v := currentRate.Float64
		change.OldRateMultiplier = &v
	}
	manualTakeover := change.ManualTakeover
	if lastAutoRate.Valid && !sameSQLFloat(currentRate, lastAutoRate) {
		manualTakeover = true
		change.ManualTakeover = true
		if change.Reason == "" || change.Reason == "policy cleared by admin" {
			change.Reason = service.TokenUsagePolicyManualTakeoverPreservedReason
		}
	}
	if !manualTakeover {
		if previous.Valid {
			res, err := tx.ExecContext(ctx, `
				UPDATE user_group_rate_multipliers
				SET rate_multiplier = $3, updated_at = NOW()
				WHERE user_id = $1 AND group_id = $2
				  AND (($4::decimal IS NULL AND rate_multiplier IS NULL) OR rate_multiplier = $4::decimal)
			`, change.UserID, targetGroupID, previous.Float64, nullableSQLFloat(lastAutoRate))
			if err != nil {
				return err
			}
			if markManualTakeoverWhenNoRowsAffected(res, change) {
				manualTakeover = true
			}
		} else {
			res, err := tx.ExecContext(ctx, `
				UPDATE user_group_rate_multipliers
				SET rate_multiplier = NULL, updated_at = NOW()
				WHERE user_id = $1 AND group_id = $2
				  AND (($3::decimal IS NULL AND rate_multiplier IS NULL) OR rate_multiplier = $3::decimal)
			`, change.UserID, targetGroupID, nullableSQLFloat(lastAutoRate))
			if err != nil {
				return err
			}
			if markManualTakeoverWhenNoRowsAffected(res, change) {
				manualTakeover = true
			} else {
				if _, err := tx.ExecContext(ctx, `
						DELETE FROM user_group_rate_multipliers
						WHERE user_id = $1 AND group_id = $2
						  AND rate_multiplier IS NULL
						  AND visible_rate_multiplier IS NULL
						  AND rpm_override IS NULL
					`, change.UserID, targetGroupID); err != nil {
					return err
				}
			}
		}
	}
	if granted {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM user_allowed_groups
			WHERE user_id = $1 AND group_id = $2
		`, change.UserID, targetGroupID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM token_usage_auto_assignments WHERE policy_id = $1 AND user_id = $2`, policy.ID, change.UserID)
	return err
}

func shouldClearPolicyGrantedGroupOwnership(change *service.TokenUsageAutoPolicyChange) bool {
	return change.GroupGranted || change.Reason == service.TokenUsagePolicyManualGroupRemovedReason
}

func nullableSQLFloat(value sql.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func markManualTakeoverWhenNoRowsAffected(res sql.Result, change *service.TokenUsageAutoPolicyChange) bool {
	affected, err := res.RowsAffected()
	if err != nil || affected > 0 {
		return false
	}
	change.ManualTakeover = true
	if change.Reason == "" || change.Reason == "policy cleared by admin" {
		change.Reason = service.TokenUsagePolicyManualTakeoverPreservedReason
	}
	return true
}

func (r *tokenUsagePolicyRepository) BeginPolicyRun(ctx context.Context, policyID int64, runType string) (*service.TokenUsageAutoPolicyRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	cutoff := time.Now().UTC().Add(-tokenUsagePolicyRunningStaleAfter)
	if err := r.recoverStalePolicyRunsInTx(ctx, tx, policyID, cutoff); err != nil {
		return nil, err
	}

	run := &service.TokenUsageAutoPolicyRun{}
	err = scanSingleRow(ctx, tx, `
			INSERT INTO token_usage_auto_runs (policy_id, run_type, status, started_at, created_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			RETURNING id, policy_id, run_type, status, total_users, create_count, update_count, downgrade_count, clear_count, skip_count, COALESCE(error_message, ''), started_at, finished_at, created_at
	`, []any{policyID, runType, service.TokenUsagePolicyRunStatusRunning}, scanTokenUsageRunDest(run)...)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, infraerrors.Conflict("POLICY_ALREADY_RUNNING", "policy is already running")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *tokenUsagePolicyRepository) FinishPolicyRun(ctx context.Context, runID int64, status string, stats service.TokenUsageAutoPolicyRunStats, changes []service.TokenUsageAutoPolicyChange, errMessage string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var policyID int64
	if err := scanSingleRow(ctx, tx, `SELECT policy_id FROM token_usage_auto_runs WHERE id = $1`, []any{runID}, &policyID); err != nil {
		return err
	}

	if err := insertTokenUsageRunChanges(ctx, tx, runID, policyID, changes); err != nil {
		return err
	}
	if err := r.finishPolicyRunSummary(ctx, tx, runID, status, stats, errMessage); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *tokenUsagePolicyRepository) finishPolicyRunSummary(ctx context.Context, exec sqlExecutor, runID int64, status string, stats service.TokenUsageAutoPolicyRunStats, errMessage string) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE token_usage_auto_runs
		SET status=$2, total_users=$3, create_count=$4, update_count=$5, downgrade_count=$6,
		    clear_count=$7, skip_count=$8, error_message=NULLIF($9, ''), finished_at=NOW()
		WHERE id=$1
	`, runID, status, stats.TotalUsers, stats.CreateCount, stats.UpdateCount, stats.DowngradeCount, stats.ClearCount, stats.SkipCount, errMessage)
	return err
}

func (r *tokenUsagePolicyRepository) ListPolicyRuns(ctx context.Context, policyID int64, params pagination.PaginationParams) ([]service.TokenUsageAutoPolicyRun, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	var total int64
	if err := scanSingleRow(ctx, r.db, `SELECT COUNT(*) FROM token_usage_auto_runs WHERE policy_id = $1`, []any{policyID}, &total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, policy_id, run_type, status, total_users, create_count, update_count, downgrade_count, clear_count, skip_count,
		       COALESCE(error_message, ''), started_at, finished_at, created_at
		FROM token_usage_auto_runs
		WHERE policy_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, policyID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	runs := []service.TokenUsageAutoPolicyRun{}
	for rows.Next() {
		var run service.TokenUsageAutoPolicyRun
		if err := rows.Scan(scanTokenUsageRunDest(&run)...); err != nil {
			return nil, nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return runs, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (r *tokenUsagePolicyRepository) ListPolicyRunChanges(ctx context.Context, policyID, runID int64, params pagination.PaginationParams) ([]service.TokenUsageAutoPolicyChange, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	var exists bool
	if err := scanSingleRow(ctx, r.db, `
		SELECT EXISTS (
			SELECT 1 FROM token_usage_auto_runs
			WHERE id = $1 AND policy_id = $2
		)
	`, []any{runID, policyID}, &exists); err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, infraerrors.BadRequest("INVALID_POLICY_RUN_ID", "invalid policy run id")
	}

	var total int64
	if err := scanSingleRow(ctx, r.db, `
		SELECT COUNT(*)
		FROM token_usage_auto_run_changes
		WHERE policy_id = $1 AND run_id = $2
	`, []any{policyID, runID}, &total); err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
			SELECT run_id, change_type, user_id, COALESCE(user_name, ''), COALESCE(user_email, ''),
			       token_usage, actual_cost, total_token_usage, total_actual_cost,
			       target_group_id, tier_id, tier_min_tokens, tier_condition_mode, tier_min_actual_cost,
			       resident_tier_id, resident_tier_min_tokens, resident_tier_condition_mode, resident_tier_min_actual_cost,
		       old_rate_multiplier, new_rate_multiplier, COALESCE(reason, ''),
		       group_granted, manual_takeover
		FROM token_usage_auto_run_changes
		WHERE policy_id = $1 AND run_id = $2
		ORDER BY id
		LIMIT $3 OFFSET $4
	`, policyID, runID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	changes := []service.TokenUsageAutoPolicyChange{}
	for rows.Next() {
		var rowRunID int64
		change, err := scanTokenUsageRunChange(rows, &rowRunID)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return changes, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func insertTokenUsageRunChanges(ctx context.Context, exec sqlExecutor, runID, policyID int64, changes []service.TokenUsageAutoPolicyChange) error {
	if len(changes) == 0 {
		return nil
	}
	for _, change := range changes {
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO token_usage_auto_run_changes (
				run_id, policy_id, change_type, user_id, user_name, user_email, token_usage, actual_cost,
				total_token_usage, total_actual_cost, target_group_id,
				tier_id, tier_min_tokens, tier_condition_mode, tier_min_actual_cost,
				resident_tier_id, resident_tier_min_tokens, resident_tier_condition_mode, resident_tier_min_actual_cost,
				old_rate_multiplier, new_rate_multiplier, reason,
				group_granted, manual_takeover, created_at
			)
			VALUES ($1,$2,$3,$4,NULLIF($5, ''),NULLIF($6, ''),$7,$8,$9,$10,$11,$12,$13,NULLIF($14, ''),$15,$16,$17,NULLIF($18, ''),$19,$20,$21,NULLIF($22, ''),$23,$24,NOW())
		`, runID, policyID, change.ChangeType, change.UserID, change.UserName, change.UserEmail, change.TokenUsage, change.ActualCost,
			change.TotalTokenUsage, change.TotalActualCost, change.TargetGroupID,
			change.TierID, change.TierMinTokens, nullIfEmpty(change.TierConditionMode), change.TierMinActualCost,
			change.ResidentTierID, change.ResidentTierMinTokens, nullIfEmpty(change.ResidentTierConditionMode), change.ResidentTierMinActualCost,
			change.OldRateMultiplier, change.NewRateMultiplier, change.Reason,
			change.GroupGranted, change.ManualTakeover); err != nil {
			return err
		}
	}
	return nil
}

func insertTokenUsagePolicyTiers(ctx context.Context, tx *sql.Tx, policyID int64, tiers []service.TokenUsageAutoPolicyTier) error {
	for _, tier := range tiers {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO token_usage_auto_policy_tiers (policy_id, condition_mode, min_tokens, min_actual_cost, is_resident, rate_multiplier, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		`, policyID, tier.ConditionMode, tier.MinTokens, tier.MinActualCost, tier.IsResident, tier.RateMultiplier, tier.SortOrder); err != nil {
			return err
		}
	}
	return nil
}

func (r *tokenUsagePolicyRepository) attachPolicyChildren(ctx context.Context, policies []service.TokenUsageAutoPolicy) error {
	if len(policies) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(policies))
	index := make(map[int64]int, len(policies))
	for i := range policies {
		ids = append(ids, policies[i].ID)
		index[policies[i].ID] = i
		policies[i].Tiers = []service.TokenUsageAutoPolicyTier{}
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, policy_id, condition_mode, min_tokens, min_actual_cost, is_resident, rate_multiplier, sort_order, created_at, updated_at
		FROM token_usage_auto_policy_tiers
		WHERE policy_id = ANY($1)
		ORDER BY policy_id, sort_order, min_tokens
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	for rows.Next() {
		var tier service.TokenUsageAutoPolicyTier
		if err := rows.Scan(&tier.ID, &tier.PolicyID, &tier.ConditionMode, &tier.MinTokens, &tier.MinActualCost, &tier.IsResident, &tier.RateMultiplier, &tier.SortOrder, &tier.CreatedAt, &tier.UpdatedAt); err != nil {
			_ = rows.Close()
			return err
		}
		if i, ok := index[tier.PolicyID]; ok {
			policies[i].Tiers = append(policies[i].Tiers, tier)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	runRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (policy_id)
		       id, policy_id, run_type, status, total_users, create_count, update_count, downgrade_count, clear_count, skip_count,
		       COALESCE(error_message, ''), started_at, finished_at, created_at
		FROM token_usage_auto_runs
		WHERE policy_id = ANY($1)
		ORDER BY policy_id, created_at DESC
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = runRows.Close() }()
	for runRows.Next() {
		var run service.TokenUsageAutoPolicyRun
		if err := runRows.Scan(scanTokenUsageRunDest(&run)...); err != nil {
			return err
		}
		if i, ok := index[run.PolicyID]; ok {
			policies[i].LatestRun = &run
		}
	}
	return runRows.Err()
}

func scanTokenUsagePolicies(rows *sql.Rows) ([]service.TokenUsageAutoPolicy, error) {
	policies := []service.TokenUsageAutoPolicy{}
	for rows.Next() {
		var p service.TokenUsageAutoPolicy
		var filterGroup sql.NullInt64
		var filterModel sql.NullString
		var filterRequest sql.NullInt16
		var filterBilling sql.NullInt16
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Enabled, &p.WindowDays, &p.TargetGroupID, &p.TargetGroupName,
			&p.ActionMode, &p.ConflictMode, &p.ScheduleFrequency,
			&filterGroup, &filterModel, &filterRequest, &filterBilling,
			&p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if filterGroup.Valid {
			v := filterGroup.Int64
			p.Filters.GroupID = &v
		}
		if filterModel.Valid {
			v := filterModel.String
			p.Filters.Model = &v
		}
		if filterRequest.Valid {
			v := int16(filterRequest.Int16)
			p.Filters.RequestType = &v
		}
		if filterBilling.Valid {
			v := int8(filterBilling.Int16)
			p.Filters.BillingType = &v
		}
		policies = append(policies, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

func scanTokenUsageAssignmentState(rows *sql.Rows) (service.TokenUsageAutoAssignment, *float64, bool, error) {
	var a service.TokenUsageAutoAssignment
	var tierID sql.NullInt64
	var residentTierID sql.NullInt64
	var lastRate sql.NullFloat64
	var previousRate sql.NullFloat64
	var takeoverReason string
	var currentRate sql.NullFloat64
	var hasAllowed bool
	if err := rows.Scan(
		&a.ID, &a.PolicyID, &a.UserID, &a.UserName, &a.UserEmail, &a.TargetGroupID,
		&tierID, &a.LastTokenUsage, &a.LastActualCost,
		&residentTierID, &a.LastTotalTokenUsage, &a.LastTotalActualCost, &lastRate, &a.GroupGrantedByPolicy, &previousRate,
		&a.ManualTakeover, &takeoverReason, &a.ManualTakeoverAt, &a.LastAppliedAt,
		&a.CreatedAt, &a.UpdatedAt, &currentRate, &hasAllowed,
	); err != nil {
		return a, nil, false, err
	}
	if tierID.Valid {
		v := tierID.Int64
		a.TierID = &v
	}
	if residentTierID.Valid {
		v := residentTierID.Int64
		a.ResidentTierID = &v
	}
	if lastRate.Valid {
		v := lastRate.Float64
		a.LastRateMultiplier = &v
	}
	if previousRate.Valid {
		v := previousRate.Float64
		a.PreviousRateMultiplier = &v
	}
	a.ManualTakeoverReason = takeoverReason
	var current *float64
	if currentRate.Valid {
		v := currentRate.Float64
		current = &v
	}
	return a, current, hasAllowed, nil
}

func scanTokenUsageRunDest(run *service.TokenUsageAutoPolicyRun) []any {
	return []any{
		&run.ID, &run.PolicyID, &run.RunType, &run.Status,
		&run.TotalUsers, &run.CreateCount, &run.UpdateCount, &run.DowngradeCount, &run.ClearCount, &run.SkipCount,
		&run.ErrorMessage, &run.StartedAt, &run.FinishedAt, &run.CreatedAt,
	}
}

func scanTokenUsageRunChange(rows *sql.Rows, runID *int64) (service.TokenUsageAutoPolicyChange, error) {
	var change service.TokenUsageAutoPolicyChange
	var tierID sql.NullInt64
	var tierMin sql.NullInt64
	var tierConditionMode sql.NullString
	var tierMinActualCost sql.NullFloat64
	var residentTierID sql.NullInt64
	var residentTierMin sql.NullInt64
	var residentTierConditionMode sql.NullString
	var residentTierMinActualCost sql.NullFloat64
	var oldRate sql.NullFloat64
	var newRate sql.NullFloat64
	if err := rows.Scan(
		runID, &change.ChangeType, &change.UserID, &change.UserName, &change.UserEmail,
		&change.TokenUsage, &change.ActualCost, &change.TotalTokenUsage, &change.TotalActualCost,
		&change.TargetGroupID, &tierID, &tierMin, &tierConditionMode, &tierMinActualCost,
		&residentTierID, &residentTierMin, &residentTierConditionMode, &residentTierMinActualCost,
		&oldRate, &newRate, &change.Reason, &change.GroupGranted, &change.ManualTakeover,
	); err != nil {
		return change, err
	}
	if tierID.Valid {
		v := tierID.Int64
		change.TierID = &v
	}
	if tierMin.Valid {
		v := tierMin.Int64
		change.TierMinTokens = &v
	}
	if tierConditionMode.Valid {
		change.TierConditionMode = tierConditionMode.String
	}
	if tierMinActualCost.Valid {
		v := tierMinActualCost.Float64
		change.TierMinActualCost = &v
	}
	if residentTierID.Valid {
		v := residentTierID.Int64
		change.ResidentTierID = &v
	}
	if residentTierMin.Valid {
		v := residentTierMin.Int64
		change.ResidentTierMinTokens = &v
	}
	if residentTierConditionMode.Valid {
		change.ResidentTierConditionMode = residentTierConditionMode.String
	}
	if residentTierMinActualCost.Valid {
		v := residentTierMinActualCost.Float64
		change.ResidentTierMinActualCost = &v
	}
	if oldRate.Valid {
		v := oldRate.Float64
		change.OldRateMultiplier = &v
	}
	if newRate.Valid {
		v := newRate.Float64
		change.NewRateMultiplier = &v
	}
	return change, nil
}

func sameSQLFloat(a, b sql.NullFloat64) bool {
	if !a.Valid || !b.Valid {
		return a.Valid == b.Valid
	}
	delta := a.Float64 - b.Float64
	if delta < 0 {
		delta = -delta
	}
	return delta < 0.00001
}

func normalizePolicyPagination(params pagination.PaginationParams) (int, int) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return page, pageSize
}

func uniquePositiveInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func tokenUsagePolicyNotFoundError(cause error) error {
	return infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id").WithCause(cause)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func duplicateEnabledTokenUsagePolicyError(cause error) error {
	return infraerrors.Conflict("DUPLICATE_ENABLED_TARGET_GROUP", "an enabled policy already targets this group").WithCause(cause)
}

func translateTokenUsagePolicyWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return duplicateEnabledTokenUsagePolicyError(err)
		case "23503":
			switch pgErr.Constraint {
			case "token_usage_auto_policies_target_group_id_fkey", "token_usage_auto_assignments_target_group_id_fkey":
				return infraerrors.BadRequest("INVALID_TARGET_GROUP", "invalid target group").WithCause(err)
			case "token_usage_auto_policies_filter_group_id_fkey":
				return infraerrors.BadRequest("INVALID_FILTER_GROUP", "invalid filter group").WithCause(err)
			}
		}
	}
	if isUniqueConstraintViolation(err) {
		return duplicateEnabledTokenUsagePolicyError(err)
	}
	return err
}

func (r *tokenUsagePolicyRepository) recoverStalePolicyRuns(ctx context.Context, policyID int64, cutoff time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE token_usage_auto_runs
		SET status = $3,
		    error_message = COALESCE(error_message, 'stale running policy run recovered'),
		    finished_at = NOW()
		WHERE policy_id = $1
		  AND status = $2
		  AND started_at < $4
	`, policyID, service.TokenUsagePolicyRunStatusRunning, service.TokenUsagePolicyRunStatusFailed, cutoff)
	return err
}

func (r *tokenUsagePolicyRepository) recoverStalePolicyRunsInTx(ctx context.Context, tx *sql.Tx, policyID int64, cutoff time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE token_usage_auto_runs
		SET status = $3,
		    error_message = COALESCE(error_message, 'stale running policy run recovered'),
		    finished_at = NOW()
		WHERE policy_id = $1
		  AND status = $2
		  AND started_at < $4
	`, policyID, service.TokenUsagePolicyRunStatusRunning, service.TokenUsagePolicyRunStatusFailed, cutoff)
	return err
}
