package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type sqlTxStarter interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type userGroupRateRepository struct {
	sql sqlExecutor
}

// NewUserGroupRateRepository 创建用户专属分组倍率/RPM 仓储
func NewUserGroupRateRepository(sqlDB *sql.DB) service.UserGroupRateRepository {
	return &userGroupRateRepository{sql: sqlDB}
}

func (r *userGroupRateRepository) runInTx(ctx context.Context, fn func(exec sqlExecutor) error) error {
	txStarter, ok := r.sql.(sqlTxStarter)
	if !ok {
		return fmt.Errorf("user group rate repository sql executor does not support transactions")
	}

	tx, err := txStarter.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user group rate transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user group rate transaction: %w", err)
	}
	return nil
}

// GetByUserID 获取用户所有专属分组 rate_multiplier（仅返回非 NULL 的条目）
func (r *userGroupRateRepository) GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error) {
	query := `SELECT group_id, rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND rate_multiplier IS NOT NULL`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]float64)
	for rows.Next() {
		var groupID int64
		var rate float64
		if err := rows.Scan(&groupID, &rate); err != nil {
			return nil, err
		}
		result[groupID] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetVisibleByUserID 获取用户所有专属可见分组倍率（仅返回非 NULL 的条目）
func (r *userGroupRateRepository) GetVisibleByUserID(ctx context.Context, userID int64) (map[int64]float64, error) {
	query := `SELECT group_id, visible_rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND visible_rate_multiplier IS NOT NULL`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]float64)
	for rows.Next() {
		var groupID int64
		var rate float64
		if err := rows.Scan(&groupID, &rate); err != nil {
			return nil, err
		}
		result[groupID] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetVisibleByUserIDs 批量获取多个用户的专属可见分组倍率（仅返回非 NULL 的条目）
func (r *userGroupRateRepository) GetVisibleByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]float64, error) {
	result := make(map[int64]map[int64]float64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT user_id, group_id, visible_rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = ANY($1)
		  AND visible_rate_multiplier IS NOT NULL
	`
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		var groupID int64
		var rate float64
		if err := rows.Scan(&userID, &groupID, &rate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]float64)
		}
		result[userID][groupID] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByUserIDs 批量获取多个用户的专属分组 rate_multiplier（仅返回非 NULL 的条目）
func (r *userGroupRateRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]float64, error) {
	result := make(map[int64]map[int64]float64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		uniqueIDs = append(uniqueIDs, userID)
		result[userID] = make(map[int64]float64)
	}
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id, group_id, rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = ANY($1) AND rate_multiplier IS NOT NULL
	`, pq.Array(uniqueIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		var groupID int64
		var rate float64
		if err := rows.Scan(&userID, &groupID, &rate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]float64)
		}
		result[userID][groupID] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByGroupID 获取指定分组下所有用户的专属配置（rate 与 rpm_override 任一非 NULL 即返回）
func (r *userGroupRateRepository) GetByGroupID(ctx context.Context, groupID int64) ([]service.UserGroupRateEntry, error) {
	query := `
		SELECT ugr.user_id, u.username, u.email, COALESCE(u.notes, ''), u.status, ugr.rate_multiplier, ugr.visible_rate_multiplier, ugr.rpm_override
		FROM user_group_rate_multipliers ugr
		JOIN users u ON u.id = ugr.user_id AND u.deleted_at IS NULL
		WHERE ugr.group_id = $1
		  AND (ugr.rate_multiplier IS NOT NULL OR ugr.visible_rate_multiplier IS NOT NULL OR ugr.rpm_override IS NOT NULL)
		ORDER BY ugr.user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserGroupRateEntry
	for rows.Next() {
		var entry service.UserGroupRateEntry
		var rate sql.NullFloat64
		var visibleRate sql.NullFloat64
		var rpm sql.NullInt32
		if err := rows.Scan(&entry.UserID, &entry.UserName, &entry.UserEmail, &entry.UserNotes, &entry.UserStatus, &rate, &visibleRate, &rpm); err != nil {
			return nil, err
		}
		if rate.Valid {
			v := rate.Float64
			entry.RateMultiplier = &v
		}
		if visibleRate.Valid {
			v := visibleRate.Float64
			entry.VisibleRateMultiplier = &v
		}
		if rpm.Valid {
			v := int(rpm.Int32)
			entry.RPMOverride = &v
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByUserAndGroup 获取用户在特定分组的专属 rate_multiplier（NULL 返回 nil）
func (r *userGroupRateRepository) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	query := `SELECT rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, r.sql, query, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !rate.Valid {
		return nil, nil
	}
	v := rate.Float64
	return &v, nil
}

// GetVisibleByUserAndGroup 获取用户在特定分组的专属 visible_rate_multiplier（NULL 返回 nil）
func (r *userGroupRateRepository) GetVisibleByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	query := `SELECT visible_rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, r.sql, query, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !rate.Valid {
		return nil, nil
	}
	v := rate.Float64
	return &v, nil
}

// GetRPMOverrideByUserAndGroup 获取用户在特定分组的 rpm_override（NULL 返回 nil）
func (r *userGroupRateRepository) GetRPMOverrideByUserAndGroup(ctx context.Context, userID, groupID int64) (*int, error) {
	query := `SELECT rpm_override FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`
	var rpm sql.NullInt32
	err := scanSingleRow(ctx, r.sql, query, []any{userID, groupID}, &rpm)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !rpm.Valid {
		return nil, nil
	}
	v := int(rpm.Int32)
	return &v, nil
}

// SyncUserGroupRates 同步用户的分组专属 rate_multiplier。
//   - 传入空 map：清空该用户所有行的 rate_multiplier；若 rpm_override 也为 NULL 则整行删除。
//   - 值为 nil：清空对应行的 rate_multiplier（保留 rpm_override）。
//   - 值非 nil：upsert rate_multiplier（保留已有 rpm_override）。
func (r *userGroupRateRepository) SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error {
	return r.syncUserGroupRateColumn(ctx, userID, "rate_multiplier", rates)
}

// SyncUserGroupVisibleRates 同步用户的分组专属 visible_rate_multiplier。
//   - 传入空 map：清空该用户所有行的 visible_rate_multiplier；若其它配置也为 NULL 则整行删除。
//   - 值为 nil：清空对应行的 visible_rate_multiplier（保留 rate_multiplier/rpm_override）。
//   - 值非 nil：upsert visible_rate_multiplier（保留已有 rate_multiplier/rpm_override）。
func (r *userGroupRateRepository) SyncUserGroupVisibleRates(ctx context.Context, userID int64, rates map[int64]*float64) error {
	return r.syncUserGroupRateColumn(ctx, userID, "visible_rate_multiplier", rates)
}

func (r *userGroupRateRepository) syncUserGroupRateColumn(ctx context.Context, userID int64, column string, rates map[int64]*float64) error {
	if column != "rate_multiplier" && column != "visible_rate_multiplier" {
		return fmt.Errorf("unsupported user group rate column: %s", column)
	}
	if len(rates) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET `+column+` = NULL, updated_at = NOW()
			WHERE user_id = $1
		`, userID); err != nil {
			return err
		}
		_, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE user_id = $1 AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL`,
			userID)
		return err
	}

	var clearGroupIDs []int64
	upsertGroupIDs := make([]int64, 0, len(rates))
	upsertRates := make([]float64, 0, len(rates))
	for groupID, rate := range rates {
		if rate == nil {
			clearGroupIDs = append(clearGroupIDs, groupID)
		} else {
			upsertGroupIDs = append(upsertGroupIDs, groupID)
			upsertRates = append(upsertRates, *rate)
		}
	}

	if len(clearGroupIDs) > 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET `+column+` = NULL, updated_at = NOW()
			WHERE user_id = $1 AND group_id = ANY($2)
		`, userID, pq.Array(clearGroupIDs)); err != nil {
			return err
		}
		if _, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = ANY($2) AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL`,
			userID, pq.Array(clearGroupIDs)); err != nil {
			return err
		}
	}

	if len(upsertGroupIDs) > 0 {
		now := time.Now()
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, `+column+`, created_at, updated_at)
			SELECT
				$1::bigint,
				data.group_id,
				data.rate_multiplier,
				$2::timestamptz,
				$2::timestamptz
			FROM unnest($3::bigint[], $4::double precision[]) AS data(group_id, rate_multiplier)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				`+column+` = EXCLUDED.`+column+`,
				updated_at = EXCLUDED.updated_at
		`, userID, now, pq.Array(upsertGroupIDs), pq.Array(upsertRates))
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncGroupRateMultipliers 同步分组的真实/可见倍率部分（不触动 rpm_override）。
// 语义：
//   - 未出现在 entries 中的用户行：rate_multiplier 与 visible_rate_multiplier 归 NULL；若 rpm_override 也为 NULL 则整行删除。
//   - 出现的用户行：upsert rate_multiplier，并按条目设置 visible_rate_multiplier（nil 表示继承）。
func (r *userGroupRateRepository) SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []service.GroupRateMultiplierInput) error {
	return r.runInTx(ctx, func(exec sqlExecutor) error {
		return r.syncGroupRateMultipliersOnExec(ctx, exec, groupID, entries)
	})
}

func (r *userGroupRateRepository) syncGroupRateMultipliersOnExec(ctx context.Context, exec sqlExecutor, groupID int64, entries []service.GroupRateMultiplierInput) error {
	keepUserIDs := make([]int64, 0, len(entries))
	for _, e := range entries {
		keepUserIDs = append(keepUserIDs, e.UserID)
	}

	// 未在 entries 列表中的行：清空真实/可见倍率。
	if len(keepUserIDs) == 0 {
		if _, err := exec.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, visible_rate_multiplier = NULL, updated_at = NOW()
			WHERE group_id = $1
		`, groupID); err != nil {
			return err
		}
	} else {
		if _, err := exec.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, visible_rate_multiplier = NULL, updated_at = NOW()
			WHERE group_id = $1 AND user_id <> ALL($2)
		`, groupID, pq.Array(keepUserIDs)); err != nil {
			return err
		}
	}

	// 清空后若整行 NULL 则删除。
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1 AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL
	`, groupID); err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	userIDs := make([]int64, len(entries))
	rates := make([]float64, len(entries))
	rateSet := make([]bool, len(entries))
	rateHasValue := make([]bool, len(entries))
	visibleRates := make([]float64, len(entries))
	visibleRateSet := make([]bool, len(entries))
	visibleRateHasValue := make([]bool, len(entries))
	for i, e := range entries {
		userIDs[i] = e.UserID
		if e.RateMultiplier != nil {
			rates[i] = *e.RateMultiplier
			rateHasValue[i] = true
		}
		if e.RateMultiplierSet {
			rateSet[i] = true
		}
		if e.VisibleRateMultiplier != nil {
			visibleRates[i] = *e.VisibleRateMultiplier
			visibleRateHasValue[i] = true
		}
		if e.VisibleRateMultiplierSet {
			visibleRateSet[i] = true
		}
	}
	now := time.Now()
	_, err := exec.ExecContext(ctx, `
		INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, visible_rate_multiplier, created_at, updated_at)
		SELECT
			data.user_id,
			$1::bigint,
			CASE WHEN data.rate_set AND data.rate_has_value THEN data.rate_multiplier ELSE NULL END,
			CASE WHEN data.visible_rate_set AND data.visible_rate_has_value THEN data.visible_rate_multiplier ELSE NULL END,
			$2::timestamptz,
			$2::timestamptz
		FROM unnest($3::bigint[], $4::double precision[], $5::boolean[], $6::boolean[], $7::double precision[], $8::boolean[], $9::boolean[]) AS data(user_id, rate_multiplier, rate_set, rate_has_value, visible_rate_multiplier, visible_rate_set, visible_rate_has_value)
		ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = CASE WHEN data.rate_set THEN EXCLUDED.rate_multiplier ELSE user_group_rate_multipliers.rate_multiplier END,
				visible_rate_multiplier = CASE WHEN data.visible_rate_set THEN EXCLUDED.visible_rate_multiplier ELSE user_group_rate_multipliers.visible_rate_multiplier END,
				updated_at = EXCLUDED.updated_at
			WHERE (data.rate_set AND user_group_rate_multipliers.rate_multiplier IS DISTINCT FROM EXCLUDED.rate_multiplier)
			   OR (data.visible_rate_set AND user_group_rate_multipliers.visible_rate_multiplier IS DISTINCT FROM EXCLUDED.visible_rate_multiplier)
		`, groupID, now, pq.Array(userIDs), pq.Array(rates), pq.Array(rateSet), pq.Array(rateHasValue), pq.Array(visibleRates), pq.Array(visibleRateSet), pq.Array(visibleRateHasValue))
	return err
}

// SyncGroupRPMOverrides 同步分组的 rpm_override 部分（不触动 rate_multiplier）。
// 语义：
//   - 未出现的用户行：rpm_override 归 NULL；若 rate_multiplier 也为 NULL 则整行删除。
//   - 出现的用户行：若 RPMOverride 为 nil 则清空；非 nil 则 upsert。
func (r *userGroupRateRepository) SyncGroupRPMOverrides(ctx context.Context, groupID int64, entries []service.GroupRPMOverrideInput) error {
	keepUserIDs := make([]int64, 0, len(entries))
	var clearUserIDs []int64
	upsertUserIDs := make([]int64, 0, len(entries))
	upsertValues := make([]int32, 0, len(entries))
	for _, e := range entries {
		keepUserIDs = append(keepUserIDs, e.UserID)
		if e.RPMOverride == nil {
			clearUserIDs = append(clearUserIDs, e.UserID)
		} else {
			upsertUserIDs = append(upsertUserIDs, e.UserID)
			upsertValues = append(upsertValues, int32(*e.RPMOverride))
		}
	}

	// 未在 entries 列表中的行：清空 rpm_override。
	if len(keepUserIDs) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rpm_override = NULL, updated_at = NOW()
			WHERE group_id = $1
		`, groupID); err != nil {
			return err
		}
	} else {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rpm_override = NULL, updated_at = NOW()
			WHERE group_id = $1 AND user_id <> ALL($2)
		`, groupID, pq.Array(keepUserIDs)); err != nil {
			return err
		}
	}

	// 显式 clear 的行。
	if len(clearUserIDs) > 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rpm_override = NULL, updated_at = NOW()
			WHERE group_id = $1 AND user_id = ANY($2)
		`, groupID, pq.Array(clearUserIDs)); err != nil {
			return err
		}
	}

	// 清空后若整行 NULL 则删除。
	if _, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1 AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL
	`, groupID); err != nil {
		return err
	}

	if len(upsertUserIDs) > 0 {
		now := time.Now()
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rpm_override, created_at, updated_at)
			SELECT data.user_id, $1::bigint, data.rpm_override, $2::timestamptz, $2::timestamptz
			FROM unnest($3::bigint[], $4::integer[]) AS data(user_id, rpm_override)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET rpm_override = EXCLUDED.rpm_override, updated_at = EXCLUDED.updated_at
		`, groupID, now, pq.Array(upsertUserIDs), pq.Array(upsertValues))
		if err != nil {
			return err
		}
	}

	return nil
}

// ClearGroupRPMOverrides 清空指定分组所有行的 rpm_override。
func (r *userGroupRateRepository) ClearGroupRPMOverrides(ctx context.Context, groupID int64) error {
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rpm_override = NULL, updated_at = NOW()
		WHERE group_id = $1
	`, groupID); err != nil {
		return err
	}
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1 AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL
	`, groupID)
	return err
}

// ClearGroupRateMultipliers 清空指定分组所有真实/可见专属倍率，保留 rpm_override。
func (r *userGroupRateRepository) ClearGroupRateMultipliers(ctx context.Context, groupID int64) error {
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rate_multiplier = NULL, visible_rate_multiplier = NULL, updated_at = NOW()
		WHERE group_id = $1
	`, groupID); err != nil {
		return err
	}
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1 AND rate_multiplier IS NULL AND visible_rate_multiplier IS NULL AND rpm_override IS NULL
	`, groupID)
	return err
}

// DeleteByGroupID 删除指定分组的所有用户专属条目
func (r *userGroupRateRepository) DeleteByGroupID(ctx context.Context, groupID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE group_id = $1`, groupID)
	return err
}

// DeleteByUserID 删除指定用户的所有专属条目
func (r *userGroupRateRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE user_id = $1`, userID)
	return err
}
