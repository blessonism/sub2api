package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/modeliqproberesult"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// modelIQProbeRepository 实现 service.ModelIQProbeRepository。
//
// 选型说明（与 channel_monitor_repo 一致）：
//   - 写入/改判/删除走 ent，复用事务上下文支持
//   - 分页查询（生效状态 = override_status ?? status 的过滤）与最近记录拉取
//     走原生 SQL，避免 ent 在 OR + IS NULL 组合上的样板代码
type modelIQProbeRepository struct {
	client *dbent.Client
	db     *sql.DB
}

// NewModelIQProbeRepository 创建仓储实例。
func NewModelIQProbeRepository(client *dbent.Client, db *sql.DB) service.ModelIQProbeRepository {
	return &modelIQProbeRepository{client: client, db: db}
}

// modelIQProbeInsertColumns 与 modelIQProbeSelectColumns 列顺序一一对应，
// scan 目标顺序必须与此保持一致（有测试断言）。
const (
	modelIQProbeSelectColumns = `id, account_id, account_name, model, protocol, status,
		override_status, error_category, error_message, expected_answer, actual_answer,
		latency_ms, input_tokens, output_tokens, cache_read_tokens, created_at`
)

// Insert 写入一条探测结果。
func (r *modelIQProbeRepository) Insert(ctx context.Context, result *service.ModelIQProbeResult) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ModelIQProbeResult.Create().
		SetNillableAccountID(result.AccountID).
		SetAccountName(result.AccountName).
		SetModel(result.Model).
		SetProtocol(result.Protocol).
		SetStatus(modeliqproberesult.Status(result.Status)).
		SetExpectedAnswer(result.ExpectedAnswer).
		SetActualAnswer(result.ActualAnswer).
		SetErrorCategory(result.ErrorCategory).
		SetErrorMessage(result.ErrorMessage).
		SetNillableLatencyMs(nillableInt64(result.LatencyMs)).
		SetNillableInputTokens(result.InputTokens).
		SetNillableOutputTokens(result.OutputTokens).
		SetNillableCacheReadTokens(result.CacheReadTokens)
	if result.OverrideStatus != nil {
		builder = builder.SetOverrideStatus(modeliqproberesult.OverrideStatus(*result.OverrideStatus))
	}
	if !result.CreatedAt.IsZero() {
		builder = builder.SetCreatedAt(result.CreatedAt)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	result.ID = created.ID
	result.CreatedAt = created.CreatedAt
	return nil
}

// ListPage 管理端分页查询（生效状态过滤：override_status 优先于 status）。
func (r *modelIQProbeRepository) ListPage(ctx context.Context, page, pageSize int, filters service.ModelIQProbeListFilters) ([]*service.ModelIQProbeResult, int64, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if filters.Model != "" {
		args = append(args, filters.Model)
		where = append(where, fmt.Sprintf("model = $%d", len(args)))
	}
	if filters.AccountID > 0 {
		args = append(args, filters.AccountID)
		where = append(where, fmt.Sprintf("account_id = $%d", len(args)))
	}
	if filters.Status != "" {
		args = append(args, filters.Status)
		where = append(where, fmt.Sprintf("((override_status IS NOT NULL AND override_status = $%d) OR (override_status IS NULL AND status = $%d))", len(args), len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	countSQL := "SELECT COUNT(*) FROM model_iq_probe_results WHERE " + whereSQL
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*service.ModelIQProbeResult{}, 0, nil
	}

	limit, offset := pageSize, (page-1)*pageSize
	args = append(args, limit, offset)
	querySQL := "SELECT " + modelIQProbeSelectColumns + " FROM model_iq_probe_results WHERE " +
		whereSQL + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := make([]*service.ModelIQProbeResult, 0, limit)
	for rows.Next() {
		item, err := scanModelIQProbeResult(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, item)
	}
	return results, total, rows.Err()
}

// ListRecent 按时间倒序拉取最近结果（用户端汇总用）。
func (r *modelIQProbeRepository) ListRecent(ctx context.Context, limit int) ([]*service.ModelIQProbeResult, error) {
	querySQL := "SELECT " + modelIQProbeSelectColumns + " FROM model_iq_probe_results ORDER BY created_at DESC, id DESC LIMIT $1"
	rows, err := r.db.QueryContext(ctx, querySQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*service.ModelIQProbeResult, 0, limit)
	for rows.Next() {
		item, err := scanModelIQProbeResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// 自动发现模式在本地历史中读取模型名，公开查询不会触发上游模型目录请求。
func (r *modelIQProbeRepository) ListSummaryModels(ctx context.Context, accountIDs []int64) ([]string, error) {
	models := []string{}
	if len(accountIDs) == 0 {
		return models, nil
	}
	rows, err := r.db.QueryContext(ctx, "SELECT DISTINCT model FROM model_iq_probe_results WHERE created_at >= $1 AND account_id = ANY($2) ORDER BY model",
		time.Now().AddDate(0, 0, -service.ModelIQProbeRetentionDays), pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

// 先限制精确目标对，再计算最新记录和时间线，退出范围的账号不会挤占历史槽位。
func (r *modelIQProbeRepository) ListForSummary(ctx context.Context, targets []service.ModelIQProbeTargetRef, recentPerModel int) ([]*service.ModelIQProbeResult, error) {
	if len(targets) == 0 {
		return []*service.ModelIQProbeResult{}, nil
	}
	accountIDs := make([]int64, 0, len(targets))
	models := make([]string, 0, len(targets))
	for _, target := range targets {
		accountIDs = append(accountIDs, target.AccountID)
		models = append(models, target.Model)
	}
	args := []any{time.Now().AddDate(0, 0, -service.ModelIQProbeRetentionDays), recentPerModel, pq.Array(accountIDs), pq.Array(models)}
	where := "created_at >= $1 AND (account_id, model) IN (SELECT * FROM unnest($3::bigint[], $4::text[]))"
	query := "WITH ranked AS (SELECT " + modelIQProbeSelectColumns +
		", ROW_NUMBER() OVER (PARTITION BY model, account_id ORDER BY created_at DESC, id DESC) AS target_rank, ROW_NUMBER() OVER (PARTITION BY model ORDER BY created_at DESC, id DESC) AS model_rank FROM model_iq_probe_results WHERE " + where +
		") SELECT " + modelIQProbeSelectColumns + " FROM ranked WHERE target_rank = 1 OR model_rank <= $2 ORDER BY created_at DESC, id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]*service.ModelIQProbeResult, 0)
	for rows.Next() {
		item, err := scanModelIQProbeResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// OverrideStatus 人工改判；status 为 nil 时清除改判恢复自动判定。
func (r *modelIQProbeRepository) OverrideStatus(ctx context.Context, id int64, status *string) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ModelIQProbeResult.Update().Where(modeliqproberesult.ID(id))
	if status != nil {
		builder = builder.SetOverrideStatus(modeliqproberesult.OverrideStatus(*status))
	} else {
		builder = builder.ClearOverrideStatus()
	}
	affected, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrModelIQProbeResultNotFound
	}
	return nil
}

// DeleteBefore 物理删除指定时间前的结果行，返回删除条数。
func (r *modelIQProbeRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	client := clientFromContext(ctx, r.client)
	deleted, err := client.ModelIQProbeResult.Delete().Where(modeliqproberesult.CreatedAtLT(cutoff)).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return int64(deleted), nil
}

// scanModelIQProbeResult 按 modelIQProbeSelectColumns 的列顺序扫描一行。
func scanModelIQProbeResult(rows *sql.Rows) (*service.ModelIQProbeResult, error) {
	var (
		r               service.ModelIQProbeResult
		accountID       sql.NullInt64
		overrideStatus  sql.NullString
		latencyMs       sql.NullInt64
		inputTokens     sql.NullInt64
		outputTokens    sql.NullInt64
		cacheReadTokens sql.NullInt64
		actualAnswer    sql.NullString
		createdAt       sql.NullTime
	)
	if err := rows.Scan(
		&r.ID,
		&accountID,
		&r.AccountName,
		&r.Model,
		&r.Protocol,
		&r.Status,
		&overrideStatus,
		&r.ErrorCategory,
		&r.ErrorMessage,
		&r.ExpectedAnswer,
		&actualAnswer,
		&latencyMs,
		&inputTokens,
		&outputTokens,
		&cacheReadTokens,
		&createdAt,
	); err != nil {
		return nil, err
	}
	if accountID.Valid {
		v := accountID.Int64
		r.AccountID = &v
	}
	if overrideStatus.Valid {
		v := overrideStatus.String
		r.OverrideStatus = &v
	}
	if latencyMs.Valid {
		r.LatencyMs = latencyMs.Int64
	}
	if inputTokens.Valid {
		v := int(inputTokens.Int64)
		r.InputTokens = &v
	}
	if outputTokens.Valid {
		v := int(outputTokens.Int64)
		r.OutputTokens = &v
	}
	if cacheReadTokens.Valid {
		v := int(cacheReadTokens.Int64)
		r.CacheReadTokens = &v
	}
	if actualAnswer.Valid {
		r.ActualAnswer = actualAnswer.String
	}
	if createdAt.Valid {
		r.CreatedAt = createdAt.Time
	}
	r.EffectiveStatus = r.ResolveEffectiveStatus()
	return &r, nil
}

// nillableInt64 零值转 NULL（latency 0 表示未知）。
func nillableInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
