package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type upstreamCostCalibrationRepository struct {
	db *sql.DB
}

func NewUpstreamCostCalibrationRepository(db *sql.DB) service.UpstreamCostCalibrationRepository {
	return &upstreamCostCalibrationRepository{db: db}
}

func (r *upstreamCostCalibrationRepository) ListTasks(ctx context.Context, params pagination.PaginationParams, filters service.UpstreamCostCalibrationTaskListFilters) ([]service.UpstreamCostCalibrationTask, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	conditions := []string{"1=1"}
	args := []any{}
	if filters.Enabled != nil {
		args = append(args, *filters.Enabled)
		conditions = append(conditions, fmt.Sprintf("t.enabled = $%d", len(args)))
	}
	if filters.TargetGroupID > 0 {
		args = append(args, filters.TargetGroupID)
		conditions = append(conditions, fmt.Sprintf("t.target_group_id = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	if err := scanSingleRow(ctx, r.db, "SELECT COUNT(*) FROM upstream_cost_calibration_tasks t WHERE "+where, args, &total); err != nil {
		return nil, nil, err
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.enabled, t.target_group_id, COALESCE(g.name, ''),
		       t.model, t.adapter_type, t.unit, t.test_prompt, t.sample_count,
		       t.priority_start, t.priority_step, t.last_run_id, t.last_run_at, t.created_at, t.updated_at
		FROM upstream_cost_calibration_tasks t
		LEFT JOIN groups g ON g.id = t.target_group_id
		WHERE `+where+`
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	tasks, err := scanUpstreamCostCalibrationTasks(rows)
	if err != nil {
		return nil, nil, err
	}
	if err := r.attachCalibrationTaskChildren(ctx, tasks); err != nil {
		return nil, nil, err
	}
	return tasks, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pagesFromTotal(total, pageSize)}, nil
}

func (r *upstreamCostCalibrationRepository) GetTaskByID(ctx context.Context, id int64) (*service.UpstreamCostCalibrationTask, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.enabled, t.target_group_id, COALESCE(g.name, ''),
		       t.model, t.adapter_type, t.unit, t.test_prompt, t.sample_count,
		       t.priority_start, t.priority_step, t.last_run_id, t.last_run_at, t.created_at, t.updated_at
		FROM upstream_cost_calibration_tasks t
		LEFT JOIN groups g ON g.id = t.target_group_id
		WHERE t.id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tasks, err := scanUpstreamCostCalibrationTasks(rows)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, upstreamCostCalibrationNotFoundError(sql.ErrNoRows)
	}
	if err := r.attachCalibrationTaskChildren(ctx, tasks); err != nil {
		return nil, err
	}
	return &tasks[0], nil
}

func (r *upstreamCostCalibrationRepository) CreateTask(ctx context.Context, task *service.UpstreamCostCalibrationTask, accounts []service.UpstreamCostCalibrationAccount) (*service.UpstreamCostCalibrationTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	err = scanSingleRow(ctx, tx, `
		INSERT INTO upstream_cost_calibration_tasks (
			name, enabled, target_group_id, model, adapter_type, unit, test_prompt,
			sample_count, priority_start, priority_step, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		RETURNING id
	`, []any{
		task.Name, task.Enabled, task.TargetGroupID, task.Model, task.AdapterType, task.Unit, task.TestPrompt,
		task.SampleCount, task.PriorityStart, task.PriorityStep,
	}, &id)
	if err != nil {
		return nil, translateUpstreamCostCalibrationWriteError(err)
	}
	if err := insertCalibrationTaskAccounts(ctx, tx, id, task.TargetGroupID, accounts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTaskByID(ctx, id)
}

func (r *upstreamCostCalibrationRepository) UpdateTask(ctx context.Context, task *service.UpstreamCostCalibrationTask, accounts []service.UpstreamCostCalibrationAccount) (*service.UpstreamCostCalibrationTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE upstream_cost_calibration_tasks
		SET name=$2, enabled=$3, target_group_id=$4, model=$5, adapter_type=$6, unit=$7,
		    test_prompt=$8, sample_count=$9, priority_start=$10, priority_step=$11, updated_at=NOW()
		WHERE id=$1
	`, task.ID, task.Name, task.Enabled, task.TargetGroupID, task.Model, task.AdapterType, task.Unit,
		task.TestPrompt, task.SampleCount, task.PriorityStart, task.PriorityStep)
	if err != nil {
		return nil, translateUpstreamCostCalibrationWriteError(err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, upstreamCostCalibrationNotFoundError(sql.ErrNoRows)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upstream_cost_calibration_task_accounts WHERE task_id = $1`, task.ID); err != nil {
		return nil, err
	}
	if err := insertCalibrationTaskAccounts(ctx, tx, task.ID, task.TargetGroupID, accounts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTaskByID(ctx, task.ID)
}

func (r *upstreamCostCalibrationRepository) DeleteTask(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM upstream_cost_calibration_tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return upstreamCostCalibrationNotFoundError(sql.ErrNoRows)
	}
	return nil
}

func (r *upstreamCostCalibrationRepository) BeginRun(ctx context.Context, taskID, targetGroupID int64) (*service.UpstreamCostCalibrationRun, error) {
	run := &service.UpstreamCostCalibrationRun{}
	err := scanCalibrationRun(r.db.QueryRowContext(ctx, `
		INSERT INTO upstream_cost_calibration_runs (task_id, target_group_id, status, started_at, created_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, task_id, target_group_id, status, total_accounts, valid_accounts, invalid_accounts,
		          suggestion_count, applied, applied_by, applied_at, error_message, started_at, finished_at, created_at
	`, taskID, targetGroupID, service.UpstreamCostCalibrationRunStatusRunning), run)
	if err != nil {
		return nil, translateUpstreamCostCalibrationWriteError(err)
	}
	return run, nil
}

func (r *upstreamCostCalibrationRepository) FinishRun(ctx context.Context, task service.UpstreamCostCalibrationTask, runID int64, status string, stats service.UpstreamCostCalibrationRunStats, results []service.UpstreamCostCalibrationResult, suggestions []service.UpstreamCostCalibrationSuggestion, errMessage string) (*service.UpstreamCostCalibrationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	for i := range results {
		results[i].RunID = runID
		results[i].TaskID = task.ID
	}
	if err := insertCalibrationResults(ctx, tx, runID, task.ID, results); err != nil {
		return nil, err
	}
	for i := range suggestions {
		suggestions[i].RunID = runID
		suggestions[i].TaskID = task.ID
	}
	if err := insertCalibrationSuggestions(ctx, tx, runID, task.ID, suggestions); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_cost_calibration_runs
		SET status=$2, total_accounts=$3, valid_accounts=$4, invalid_accounts=$5,
		    suggestion_count=$6, error_message=$7, finished_at=NOW()
		WHERE id=$1 AND task_id=$8
	`, runID, status, stats.TotalAccounts, stats.ValidAccounts, stats.InvalidAccounts, stats.SuggestionCount, nullStringIfEmpty(errMessage), task.ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_cost_calibration_tasks
		SET last_run_id=$2, last_run_at=NOW(), updated_at=NOW()
		WHERE id=$1
	`, task.ID, runID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRun(ctx, task.ID, runID)
}

func (r *upstreamCostCalibrationRepository) GetRun(ctx context.Context, taskID, runID int64) (*service.UpstreamCostCalibrationRun, error) {
	run, err := r.getRunSummary(ctx, r.db, taskID, runID)
	if err != nil {
		return nil, err
	}
	results, err := r.listRunResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	suggestions, err := r.listRunSuggestions(ctx, runID)
	if err != nil {
		return nil, err
	}
	run.Results = results
	run.Suggestions = suggestions
	return run, nil
}

func (r *upstreamCostCalibrationRepository) ListRuns(ctx context.Context, taskID int64, params pagination.PaginationParams) ([]service.UpstreamCostCalibrationRun, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	var total int64
	if err := scanSingleRow(ctx, r.db, `SELECT COUNT(*) FROM upstream_cost_calibration_runs WHERE task_id = $1`, []any{taskID}, &total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, target_group_id, status, total_accounts, valid_accounts, invalid_accounts,
		       suggestion_count, applied, applied_by, applied_at, error_message, started_at, finished_at, created_at
		FROM upstream_cost_calibration_runs
		WHERE task_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, taskID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	runs := []service.UpstreamCostCalibrationRun{}
	for rows.Next() {
		var run service.UpstreamCostCalibrationRun
		if err := scanCalibrationRun(rows, &run); err != nil {
			return nil, nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return runs, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pagesFromTotal(total, pageSize)}, nil
}

func (r *upstreamCostCalibrationRepository) ApplyRunSuggestions(ctx context.Context, taskID, runID, operatorID int64) (*service.UpstreamCostCalibrationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		status  string
		applied bool
		groupID int64
	)
	if err := scanSingleRow(ctx, tx, `
		SELECT r.status, r.applied, r.target_group_id
		FROM upstream_cost_calibration_runs r
		WHERE r.id = $1 AND r.task_id = $2
		FOR UPDATE
	`, []any{runID, taskID}, &status, &applied, &groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, upstreamCostCalibrationRunNotFoundError(err)
		}
		return nil, err
	}
	if status != service.UpstreamCostCalibrationRunStatusSuccess {
		return nil, infraerrors.BadRequest("CALIBRATION_RUN_NOT_SUCCESS", "only successful calibration runs can be applied")
	}
	if applied {
		return nil, infraerrors.Conflict("CALIBRATION_RUN_ALREADY_APPLIED", "calibration run suggestions already applied")
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT account_id, old_priority, new_priority
		FROM upstream_cost_calibration_suggestions
		WHERE run_id = $1 AND task_id = $2 AND applied = FALSE
		ORDER BY new_priority ASC, account_id ASC
	`, runID, taskID)
	if err != nil {
		return nil, err
	}
	type suggestionRow struct {
		accountID   int64
		oldPriority sql.NullInt64
		newPriority int
	}
	suggestions := []suggestionRow{}
	for rows.Next() {
		var row suggestionRow
		if err := rows.Scan(&row.accountID, &row.oldPriority, &row.newPriority); err != nil {
			_ = rows.Close()
			return nil, err
		}
		suggestions = append(suggestions, row)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, infraerrors.BadRequest("CALIBRATION_RUN_NO_SUGGESTIONS", "calibration run has no pending suggestions")
	}
	for _, suggestion := range suggestions {
		oldPriority := nullableInt64Value(nullInt64Ptr(suggestion.oldPriority))
		res, err := tx.ExecContext(ctx, `
			UPDATE account_groups
			SET priority = $3
			WHERE account_id = $1 AND group_id = $2 AND priority IS NOT DISTINCT FROM $4
		`, suggestion.accountID, groupID, suggestion.newPriority, oldPriority)
		if err != nil {
			return nil, err
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			exists, err := calibrationAccountGroupExists(ctx, tx, suggestion.accountID, groupID)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, infraerrors.BadRequest("CALIBRATION_ACCOUNT_NOT_IN_GROUP", "calibration account is no longer in target group")
			}
			return nil, infraerrors.Conflict("CALIBRATION_PRIORITY_CHANGED", "calibration account priority changed after this run")
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_cost_calibration_suggestions
		SET applied = TRUE, applied_by = $3, applied_at = NOW()
		WHERE run_id = $1 AND task_id = $2
	`, runID, taskID, operatorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_cost_calibration_runs
		SET applied = TRUE, applied_by = $3, applied_at = NOW()
		WHERE id = $1 AND task_id = $2
	`, runID, taskID, operatorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRun(ctx, taskID, runID)
}

func insertCalibrationTaskAccounts(ctx context.Context, tx *sql.Tx, taskID, targetGroupID int64, accounts []service.UpstreamCostCalibrationAccount) error {
	for _, account := range accounts {
		raw, err := json.Marshal(account.AdapterConfig)
		if err != nil {
			return fmt.Errorf("marshal adapter config: %w", err)
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_cost_calibration_task_accounts (task_id, account_id, adapter_config, created_at)
			SELECT $1, ag.account_id, $4::jsonb, NOW()
			FROM account_groups ag
			WHERE ag.account_id = $2 AND ag.group_id = $3
		`, taskID, account.AccountID, targetGroupID, string(raw))
		if err != nil {
			return translateUpstreamCostCalibrationWriteError(err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return infraerrors.BadRequest("CALIBRATION_ACCOUNT_NOT_IN_GROUP", "calibration account must belong to target group")
		}
	}
	return nil
}

func insertCalibrationResults(ctx context.Context, tx *sql.Tx, runID, taskID int64, results []service.UpstreamCostCalibrationResult) error {
	for _, result := range results {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_cost_calibration_results (
				run_id, task_id, account_id, account_name, account_platform, current_priority,
				before_balance, after_balance, cost_delta, unit, test_status, latency_ms,
				valid, rank, suggested_priority, error_message, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,NOW())
		`, runID, taskID, result.AccountID, result.AccountName, result.AccountPlatform, nullableIntValue(result.CurrentPriority),
			nullableFloat64Value(result.BeforeBalance), nullableFloat64Value(result.AfterBalance), nullableFloat64Value(result.CostDelta),
			result.Unit, result.TestStatus, nullableIntValue(result.LatencyMs), result.Valid, nullableIntValue(result.Rank),
			nullableIntValue(result.SuggestedPriority), nullStringIfEmpty(result.ErrorMessage)); err != nil {
			return err
		}
	}
	return nil
}

func insertCalibrationSuggestions(ctx context.Context, tx *sql.Tx, runID, taskID int64, suggestions []service.UpstreamCostCalibrationSuggestion) error {
	for _, suggestion := range suggestions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_cost_calibration_suggestions (
				run_id, task_id, account_id, old_priority, new_priority, reason, applied, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,FALSE,NOW())
		`, runID, taskID, suggestion.AccountID, nullableIntValue(suggestion.OldPriority), suggestion.NewPriority, suggestion.Reason); err != nil {
			return err
		}
	}
	return nil
}

func (r *upstreamCostCalibrationRepository) attachCalibrationTaskChildren(ctx context.Context, tasks []service.UpstreamCostCalibrationTask) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(tasks))
	byID := make(map[int64]*service.UpstreamCostCalibrationTask, len(tasks))
	for i := range tasks {
		ids = append(ids, tasks[i].ID)
		byID[tasks[i].ID] = &tasks[i]
	}
	accountRows, err := r.db.QueryContext(ctx, `
		SELECT ta.task_id, ta.account_id, COALESCE(a.name, ''), COALESCE(a.platform, ''),
		       ag.priority, ta.adapter_config, ta.created_at
		FROM upstream_cost_calibration_task_accounts ta
		JOIN upstream_cost_calibration_tasks t ON t.id = ta.task_id
		LEFT JOIN accounts a ON a.id = ta.account_id
		LEFT JOIN account_groups ag ON ag.account_id = ta.account_id AND ag.group_id = t.target_group_id
		WHERE ta.task_id = ANY($1)
		ORDER BY ta.task_id ASC, ag.priority ASC NULLS LAST, ta.account_id ASC
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = accountRows.Close() }()
	for accountRows.Next() {
		var (
			taskID    int64
			account   service.UpstreamCostCalibrationAccount
			priority  sql.NullInt64
			configRaw []byte
		)
		if err := accountRows.Scan(&taskID, &account.AccountID, &account.AccountName, &account.Platform, &priority, &configRaw, &account.CreatedAt); err != nil {
			return err
		}
		if priority.Valid {
			v := int(priority.Int64)
			account.CurrentPriority = &v
		}
		account.AdapterConfig = map[string]any{}
		if len(configRaw) > 0 {
			_ = json.Unmarshal(configRaw, &account.AdapterConfig)
		}
		if task := byID[taskID]; task != nil {
			task.Accounts = append(task.Accounts, account)
		}
	}
	if err := accountRows.Err(); err != nil {
		return err
	}
	runRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (task_id)
		       id, task_id, target_group_id, status, total_accounts, valid_accounts, invalid_accounts,
		       suggestion_count, applied, applied_by, applied_at, error_message, started_at, finished_at, created_at
		FROM upstream_cost_calibration_runs
		WHERE task_id = ANY($1)
		ORDER BY task_id, created_at DESC, id DESC
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = runRows.Close() }()
	for runRows.Next() {
		var run service.UpstreamCostCalibrationRun
		if err := scanCalibrationRun(runRows, &run); err != nil {
			return err
		}
		if task := byID[run.TaskID]; task != nil {
			task.LatestRun = &run
		}
	}
	return runRows.Err()
}

func scanUpstreamCostCalibrationTasks(rows *sql.Rows) ([]service.UpstreamCostCalibrationTask, error) {
	tasks := []service.UpstreamCostCalibrationTask{}
	for rows.Next() {
		var task service.UpstreamCostCalibrationTask
		var lastRunID sql.NullInt64
		if err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Enabled,
			&task.TargetGroupID,
			&task.TargetGroupName,
			&task.Model,
			&task.AdapterType,
			&task.Unit,
			&task.TestPrompt,
			&task.SampleCount,
			&task.PriorityStart,
			&task.PriorityStep,
			&lastRunID,
			&task.LastRunAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastRunID.Valid {
			v := lastRunID.Int64
			task.LastRunID = &v
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *upstreamCostCalibrationRepository) getRunSummary(ctx context.Context, q sqlQueryer, taskID, runID int64) (*service.UpstreamCostCalibrationRun, error) {
	run := &service.UpstreamCostCalibrationRun{}
	rows, err := q.QueryContext(ctx, `
		SELECT id, task_id, target_group_id, status, total_accounts, valid_accounts, invalid_accounts,
		       suggestion_count, applied, applied_by, applied_at, error_message, started_at, finished_at, created_at
		FROM upstream_cost_calibration_runs
		WHERE task_id = $1 AND id = $2
	`, taskID, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, upstreamCostCalibrationRunNotFoundError(sql.ErrNoRows)
	}
	err = scanCalibrationRun(rows, run)
	if err != nil {
		return nil, err
	}
	return run, rows.Err()
}

func (r *upstreamCostCalibrationRepository) listRunResults(ctx context.Context, runID int64) ([]service.UpstreamCostCalibrationResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, task_id, account_id, account_name, account_platform, current_priority,
		       before_balance, after_balance, cost_delta, unit, test_status, latency_ms,
		       valid, rank, suggested_priority, error_message, created_at
		FROM upstream_cost_calibration_results
		WHERE run_id = $1
		ORDER BY rank ASC NULLS LAST, account_id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	results := []service.UpstreamCostCalibrationResult{}
	for rows.Next() {
		result, err := scanCalibrationResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (r *upstreamCostCalibrationRepository) listRunSuggestions(ctx context.Context, runID int64) ([]service.UpstreamCostCalibrationSuggestion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, task_id, account_id, old_priority, new_priority, reason,
		       applied, applied_by, applied_at, created_at
		FROM upstream_cost_calibration_suggestions
		WHERE run_id = $1
		ORDER BY new_priority ASC, account_id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	suggestions := []service.UpstreamCostCalibrationSuggestion{}
	for rows.Next() {
		var s service.UpstreamCostCalibrationSuggestion
		var oldPriority sql.NullInt64
		var appliedBy sql.NullInt64
		if err := rows.Scan(&s.ID, &s.RunID, &s.TaskID, &s.AccountID, &oldPriority, &s.NewPriority, &s.Reason, &s.Applied, &appliedBy, &s.AppliedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		if oldPriority.Valid {
			v := int(oldPriority.Int64)
			s.OldPriority = &v
		}
		if appliedBy.Valid {
			v := appliedBy.Int64
			s.AppliedBy = &v
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, rows.Err()
}

type calibrationRowScanner interface {
	Scan(dest ...any) error
}

func scanCalibrationRun(scanner calibrationRowScanner, run *service.UpstreamCostCalibrationRun) error {
	var appliedBy sql.NullInt64
	var errorMessage sql.NullString
	if err := scanner.Scan(
		&run.ID,
		&run.TaskID,
		&run.TargetGroupID,
		&run.Status,
		&run.TotalAccounts,
		&run.ValidAccounts,
		&run.InvalidAccounts,
		&run.SuggestionCount,
		&run.Applied,
		&appliedBy,
		&run.AppliedAt,
		&errorMessage,
		&run.StartedAt,
		&run.FinishedAt,
		&run.CreatedAt,
	); err != nil {
		return err
	}
	if appliedBy.Valid {
		v := appliedBy.Int64
		run.AppliedBy = &v
	}
	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}
	return nil
}

func scanCalibrationResult(rows *sql.Rows) (service.UpstreamCostCalibrationResult, error) {
	var result service.UpstreamCostCalibrationResult
	var currentPriority, latency, rank, suggested sql.NullInt64
	var before, after, delta sql.NullFloat64
	var errMessage sql.NullString
	if err := rows.Scan(
		&result.ID, &result.RunID, &result.TaskID, &result.AccountID, &result.AccountName, &result.AccountPlatform,
		&currentPriority, &before, &after, &delta, &result.Unit, &result.TestStatus, &latency,
		&result.Valid, &rank, &suggested, &errMessage, &result.CreatedAt,
	); err != nil {
		return result, err
	}
	result.CurrentPriority = calibrationNullIntPtr(currentPriority)
	result.BeforeBalance = calibrationNullFloat64Ptr(before)
	result.AfterBalance = calibrationNullFloat64Ptr(after)
	result.CostDelta = calibrationNullFloat64Ptr(delta)
	result.LatencyMs = calibrationNullIntPtr(latency)
	result.Rank = calibrationNullIntPtr(rank)
	result.SuggestedPriority = calibrationNullIntPtr(suggested)
	if errMessage.Valid {
		result.ErrorMessage = errMessage.String
	}
	return result, nil
}

func upstreamCostCalibrationNotFoundError(cause error) error {
	return infraerrors.BadRequest("INVALID_CALIBRATION_TASK_ID", "invalid calibration task id").WithCause(cause)
}

func upstreamCostCalibrationRunNotFoundError(cause error) error {
	return infraerrors.BadRequest("INVALID_CALIBRATION_RUN_ID", "invalid calibration run id").WithCause(cause)
}

func translateUpstreamCostCalibrationWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		switch pgErr.Constraint {
		case "upstream_cost_calibration_tasks_target_group_id_fkey":
			return infraerrors.BadRequest("INVALID_TARGET_GROUP", "invalid target group").WithCause(err)
		case "upstream_cost_calibration_task_accounts_account_id_fkey":
			return infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id").WithCause(err)
		}
	}
	return err
}

func pagesFromTotal(total int64, pageSize int) int {
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return pages
}

func nullableIntValue(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullStringIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func calibrationNullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int64)
	return &out
}

func calibrationNullFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func calibrationAccountGroupExists(ctx context.Context, q sqlQueryer, accountID, groupID int64) (bool, error) {
	var exists bool
	if err := scanSingleRow(ctx, q, `
		SELECT EXISTS (
			SELECT 1 FROM account_groups WHERE account_id = $1 AND group_id = $2
		)
	`, []any{accountID, groupID}, &exists); err != nil {
		return false, err
	}
	return exists, nil
}
