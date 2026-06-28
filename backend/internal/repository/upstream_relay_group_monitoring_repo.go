package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type upstreamRelayRepository struct {
	db *sql.DB
}

const (
	upstreamRelayHealthSnapshotWindowMinutes = 24 * 60
	upstreamRelayHealthSnapshotSampleLimit   = 20
)

func NewUpstreamRelayRepository(db *sql.DB) service.UpstreamRelayRepository {
	return &upstreamRelayRepository{db: db}
}

func (r *upstreamRelayRepository) ListConnectors(ctx context.Context, params pagination.PaginationParams, filters service.UpstreamRelayConnectorListFilters) ([]service.UpstreamRelayConnector, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	conditions := []string{"deleted_at IS NULL"}
	args := []any{}
	if filters.Status != "" {
		args = append(args, filters.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if filters.Search != "" {
		args = append(args, "%"+strings.ToLower(filters.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(base_url) LIKE $%d)", len(args), len(args)))
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := scanSingleRow(ctx, r.db, "SELECT COUNT(*) FROM upstream_relay_connectors WHERE "+where, args, &total); err != nil {
		return nil, nil, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, base_url, auth_mode, COALESCE(bearer_token_encrypted, ''),
		       COALESCE(refresh_token_encrypted, ''), COALESCE(login_email_encrypted, ''),
		       COALESCE(cookie_encrypted, ''), COALESCE(user_agent_encrypted, ''),
		       upstream_account_balance, upstream_account_balance_checked_at,
		       status, credential_version, last_verified_at, last_synced_at, COALESCE(last_error, ''),
		       COALESCE(created_by, 0), created_at, updated_at
		FROM upstream_relay_connectors
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelayConnectors(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, relayPage(total, page, pageSize), nil
}

func (r *upstreamRelayRepository) GetConnector(ctx context.Context, id int64) (*service.UpstreamRelayConnector, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, base_url, auth_mode, COALESCE(bearer_token_encrypted, ''),
		       COALESCE(refresh_token_encrypted, ''), COALESCE(login_email_encrypted, ''),
		       COALESCE(cookie_encrypted, ''), COALESCE(user_agent_encrypted, ''),
		       upstream_account_balance, upstream_account_balance_checked_at,
		       status, credential_version, last_verified_at, last_synced_at, COALESCE(last_error, ''),
		       COALESCE(created_by, 0), created_at, updated_at
		FROM upstream_relay_connectors
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelayConnectors(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, service.ErrUpstreamRelayConnectorNotFound
	}
	return &items[0], nil
}

func (r *upstreamRelayRepository) CreateConnector(ctx context.Context, connector *service.UpstreamRelayConnector) (*service.UpstreamRelayConnector, error) {
	var id int64
	if err := scanSingleRow(ctx, r.db, `
		INSERT INTO upstream_relay_connectors (
			name, base_url, auth_mode, bearer_token_encrypted, refresh_token_encrypted,
			login_email_encrypted, cookie_encrypted, user_agent_encrypted, status,
			created_by, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		RETURNING id
	`, []any{
		connector.Name, connector.BaseURL, connector.AuthMode, nullStringIfEmpty(connector.BearerTokenEncrypted),
		nullStringIfEmpty(connector.RefreshTokenEncrypted), nullStringIfEmpty(connector.LoginEmailEncrypted),
		nullStringIfEmpty(connector.CookieEncrypted), nullStringIfEmpty(connector.UserAgentEncrypted),
		connector.Status, nullableInt64Value(&connector.CreatedBy),
	}, &id); err != nil {
		return nil, err
	}
	return r.GetConnector(ctx, id)
}

func (r *upstreamRelayRepository) UpdateConnector(ctx context.Context, connector *service.UpstreamRelayConnector, credentialsUpdated bool) (*service.UpstreamRelayConnector, error) {
	versionExpr := "credential_version"
	if credentialsUpdated {
		versionExpr = "credential_version + 1"
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE upstream_relay_connectors
		SET name=$2, base_url=$3, auth_mode=$4, bearer_token_encrypted=$5,
		    refresh_token_encrypted=$6, login_email_encrypted=$7,
		    cookie_encrypted=$8, user_agent_encrypted=$9, status=$10,
		    credential_version=`+versionExpr+`, updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, connector.ID, connector.Name, connector.BaseURL, connector.AuthMode,
		nullStringIfEmpty(connector.BearerTokenEncrypted), nullStringIfEmpty(connector.RefreshTokenEncrypted),
		nullStringIfEmpty(connector.LoginEmailEncrypted), nullStringIfEmpty(connector.CookieEncrypted),
		nullStringIfEmpty(connector.UserAgentEncrypted), connector.Status)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, service.ErrUpstreamRelayConnectorNotFound
	}
	return r.GetConnector(ctx, connector.ID)
}

func (r *upstreamRelayRepository) SoftDeleteConnector(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE upstream_relay_connectors SET deleted_at=NOW(), updated_at=NOW(), status=$2 WHERE id=$1 AND deleted_at IS NULL`, id, service.UpstreamRelayConnectorStatusPaused)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return service.ErrUpstreamRelayConnectorNotFound
	}
	return nil
}

func (r *upstreamRelayRepository) UpsertSnapshots(ctx context.Context, connectorID int64, snapshots []service.UpstreamRelayGroupRateSnapshot) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	previous, err := listRelaySnapshotsTx(ctx, tx, connectorID)
	if err != nil {
		return err
	}
	changes := buildRelaySnapshotChanges(connectorID, previous, snapshots)
	for _, change := range changes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_relay_group_rate_snapshot_changes (
				connector_id, upstream_group_id, group_name, platform, change_type,
				old_final_rate_multiplier, new_final_rate_multiplier,
				old_status, new_status, source, changed_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		`, change.ConnectorID, change.UpstreamGroupID, change.GroupName, change.Platform, change.ChangeType,
			nullableFloat64Value(change.OldFinalRateMultiplier), nullableFloat64Value(change.NewFinalRateMultiplier),
			change.OldStatus, change.NewStatus, change.Source); err != nil {
			return err
		}
	}
	seenGroupIDs := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		seenGroupIDs = append(seenGroupIDs, snapshot.UpstreamGroupID)
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO upstream_relay_group_rate_snapshots (
					connector_id, upstream_group_id, name, platform, status,
					default_rate_multiplier, override_rate_multiplier, final_rate_multiplier,
					today_actual_cost, today_total_tokens, today_usage_checked_at,
					source, last_seen_at, created_at, updated_at
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW(),NOW())
				ON CONFLICT (connector_id, upstream_group_id) DO UPDATE SET
					name=EXCLUDED.name,
					platform=EXCLUDED.platform,
					status=EXCLUDED.status,
					default_rate_multiplier=EXCLUDED.default_rate_multiplier,
					override_rate_multiplier=EXCLUDED.override_rate_multiplier,
					final_rate_multiplier=EXCLUDED.final_rate_multiplier,
					today_actual_cost=EXCLUDED.today_actual_cost,
					today_total_tokens=EXCLUDED.today_total_tokens,
					today_usage_checked_at=EXCLUDED.today_usage_checked_at,
					source=EXCLUDED.source,
					last_seen_at=EXCLUDED.last_seen_at,
					updated_at=NOW()
			`, connectorID, snapshot.UpstreamGroupID, snapshot.Name, snapshot.Platform, snapshot.Status,
			snapshot.DefaultRateMultiplier, nullableFloat64Value(snapshot.OverrideRateMultiplier),
			snapshot.FinalRateMultiplier, nullableFloat64Value(snapshot.TodayActualCost),
			nullableInt64Value(snapshot.TodayTotalTokens), snapshot.TodayUsageCheckedAt,
			snapshot.Source, snapshot.LastSeenAt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_relay_group_rate_snapshots
		SET status='stale', updated_at=NOW()
		WHERE connector_id=$1
		  AND NOT (upstream_group_id = ANY($2))
	`, connectorID, pq.Array(seenGroupIDs)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *upstreamRelayRepository) ListSnapshots(ctx context.Context, connectorID int64) ([]service.UpstreamRelayGroupRateSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
			SELECT id, connector_id, upstream_group_id, name, platform, status,
			       default_rate_multiplier, override_rate_multiplier, final_rate_multiplier,
			       today_actual_cost, today_total_tokens, today_usage_checked_at,
			       source, last_seen_at, created_at, updated_at
			FROM upstream_relay_group_rate_snapshots
			WHERE connector_id = $1
			ORDER BY platform ASC, name ASC, upstream_group_id ASC
	`, connectorID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRelaySnapshots(rows)
}

func (r *upstreamRelayRepository) ListSnapshotChanges(ctx context.Context, params pagination.PaginationParams, filters service.UpstreamRelaySnapshotChangeListFilters) ([]service.UpstreamRelayGroupRateSnapshotChange, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	conditions := []string{"1=1"}
	args := []any{}
	if filters.ConnectorID > 0 {
		args = append(args, filters.ConnectorID)
		conditions = append(conditions, fmt.Sprintf("ch.connector_id = $%d", len(args)))
	}
	if filters.ChangeType != "" {
		args = append(args, filters.ChangeType)
		conditions = append(conditions, fmt.Sprintf("ch.change_type = $%d", len(args)))
	}
	if strings.TrimSpace(filters.Search) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(filters.Search))+"%")
		conditions = append(conditions, fmt.Sprintf("(LOWER(COALESCE(c.name, '')) LIKE $%d OR LOWER(ch.upstream_group_id) LIKE $%d OR LOWER(ch.group_name) LIKE $%d OR LOWER(ch.platform) LIKE $%d)", len(args), len(args), len(args), len(args)))
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := scanSingleRow(ctx, r.db, `
		SELECT COUNT(*)
		FROM upstream_relay_group_rate_snapshot_changes ch
		LEFT JOIN upstream_relay_connectors c ON c.id = ch.connector_id
		WHERE `+where, args, &total); err != nil {
		return nil, nil, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT ch.id, ch.connector_id, COALESCE(c.name, ''), ch.upstream_group_id,
		       ch.group_name, ch.platform, ch.change_type,
		       ch.old_final_rate_multiplier, ch.new_final_rate_multiplier,
		       ch.old_status, ch.new_status, ch.source, ch.changed_at
		FROM upstream_relay_group_rate_snapshot_changes ch
		LEFT JOIN upstream_relay_connectors c ON c.id = ch.connector_id
		WHERE `+where+`
		ORDER BY ch.changed_at DESC, ch.id DESC
		LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelaySnapshotChanges(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, relayPage(total, page, pageSize), nil
}

func (r *upstreamRelayRepository) MarkConnectorSync(ctx context.Context, connectorID int64, status string, errMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE upstream_relay_connectors
		SET status=$2, last_verified_at=NOW(),
		    last_synced_at=CASE WHEN $3 = '' THEN NOW() ELSE last_synced_at END,
		    last_error=NULLIF($3, ''), updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, connectorID, status, errMessage)
	return err
}

func (r *upstreamRelayRepository) UpdateConnectorAccountBalance(ctx context.Context, connectorID int64, balance *float64, checkedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE upstream_relay_connectors
		SET upstream_account_balance=$2,
		    upstream_account_balance_checked_at=$3,
		    updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, connectorID, nullableFloat64Value(balance), checkedAt)
	return err
}

func (r *upstreamRelayRepository) UpdateSnapshotTodayUsage(ctx context.Context, connectorID int64, usageByGroup map[string]service.UpstreamRelayGroupTodayUsage, checkedAt *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if usageByGroup == nil || checkedAt == nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE upstream_relay_group_rate_snapshots
			SET today_actual_cost=NULL,
			    today_total_tokens=NULL,
			    today_usage_checked_at=NULL,
			    updated_at=NOW()
			WHERE connector_id=$1
		`, connectorID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_relay_group_rate_snapshots
		SET today_actual_cost=0,
		    today_total_tokens=0,
		    today_usage_checked_at=$2,
		    updated_at=NOW()
		WHERE connector_id=$1
	`, connectorID, checkedAt); err != nil {
		return err
	}
	for groupID, usage := range usageByGroup {
		if _, err := tx.ExecContext(ctx, `
			UPDATE upstream_relay_group_rate_snapshots
			SET today_actual_cost=$3,
			    today_total_tokens=$4,
			    today_usage_checked_at=$5,
			    updated_at=NOW()
			WHERE connector_id=$1 AND upstream_group_id=$2
		`, connectorID, groupID, usage.ActualCost, usage.TotalTokens, checkedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *upstreamRelayRepository) ListCandidateUsageBindings(ctx context.Context, connectorID int64) ([]service.UpstreamRelayCandidateUsageBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, connector_id, account_id, upstream_group_id,
		       COALESCE(upstream_api_key_id, 0), COALESCE(upstream_api_key_name, '')
		FROM upstream_relay_candidates
		WHERE connector_id=$1
		  AND deleted_at IS NULL
		ORDER BY id ASC
	`, connectorID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []service.UpstreamRelayCandidateUsageBinding{}
	for rows.Next() {
		var item service.UpstreamRelayCandidateUsageBinding
		if err := rows.Scan(&item.CandidateID, &item.ConnectorID, &item.AccountID, &item.UpstreamGroupID, &item.UpstreamAPIKeyID, &item.UpstreamAPIKeyName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *upstreamRelayRepository) ListCandidates(ctx context.Context, params pagination.PaginationParams, filters service.UpstreamRelayCandidateListFilters) ([]service.UpstreamRelayCandidate, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	conditions := []string{"c.deleted_at IS NULL"}
	args := []any{}
	if filters.ConnectorID > 0 {
		args = append(args, filters.ConnectorID)
		conditions = append(conditions, fmt.Sprintf("c.connector_id = $%d", len(args)))
	}
	if filters.Enabled != nil {
		args = append(args, *filters.Enabled)
		conditions = append(conditions, fmt.Sprintf("c.enabled = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := scanSingleRow(ctx, r.db, "SELECT COUNT(*) FROM upstream_relay_candidates c WHERE "+where, args, &total); err != nil {
		return nil, nil, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, relayCandidateSelect()+`
		WHERE `+where+`
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelayCandidates(rows)
	if err != nil {
		return nil, nil, err
	}
	if err := r.decorateRelayCandidates(ctx, items); err != nil {
		return nil, nil, err
	}
	return items, relayPage(total, page, pageSize), nil
}

func (r *upstreamRelayRepository) GetCandidate(ctx context.Context, id int64) (*service.UpstreamRelayCandidate, error) {
	rows, err := r.db.QueryContext(ctx, relayCandidateSelect()+` WHERE c.id = $1 AND c.deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelayCandidates(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, service.ErrUpstreamRelayCandidateNotFound
	}
	if err := r.decorateRelayCandidates(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *upstreamRelayRepository) CreateCandidate(ctx context.Context, candidate *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	if err := r.validateCandidateBinding(ctx, candidate.ConnectorID, candidate.AccountID); err != nil {
		return nil, err
	}
	var id int64
	if err := scanSingleRow(ctx, r.db, `
		INSERT INTO upstream_relay_candidates (
			connector_id, account_id, upstream_group_id, probe_model, probe_protocol,
			upstream_api_key_id, upstream_api_key_name, upstream_api_key_masked,
			enabled, notes, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING id
	`, []any{
		candidate.ConnectorID, candidate.AccountID, candidate.UpstreamGroupID,
		candidate.ProbeModel, candidate.ProbeProtocol, nullableInt64Value(candidate.UpstreamAPIKeyID),
		candidate.UpstreamAPIKeyName, candidate.UpstreamAPIKeyMasked, candidate.Enabled,
		candidate.Notes, nullableInt64Value(&candidate.CreatedBy),
	}, &id); err != nil {
		return nil, translateRelayWriteError(err)
	}
	return r.GetCandidate(ctx, id)
}

func (r *upstreamRelayRepository) UpdateCandidate(ctx context.Context, candidate *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	if err := r.validateCandidateBinding(ctx, candidate.ConnectorID, candidate.AccountID); err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE upstream_relay_candidates c
		SET connector_id=$2, account_id=$3, upstream_group_id=$4, probe_model=$5,
		    probe_protocol=$6, upstream_api_key_id=$7, upstream_api_key_name=$8,
		    upstream_api_key_masked=$9, enabled=$10, notes=$11, updated_at=NOW()
		WHERE c.id=$1 AND c.deleted_at IS NULL
	`, candidate.ID, candidate.ConnectorID, candidate.AccountID, candidate.UpstreamGroupID,
		candidate.ProbeModel, candidate.ProbeProtocol, nullableInt64Value(candidate.UpstreamAPIKeyID),
		candidate.UpstreamAPIKeyName, candidate.UpstreamAPIKeyMasked, candidate.Enabled, candidate.Notes)
	if err != nil {
		return nil, translateRelayWriteError(err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, service.ErrUpstreamRelayCandidateNotFound
	}
	return r.GetCandidate(ctx, candidate.ID)
}

func (r *upstreamRelayRepository) validateCandidateBinding(ctx context.Context, connectorID, accountID int64) error {
	var connectorExists bool
	if err := scanSingleRow(ctx, r.db, `
		SELECT EXISTS (
			SELECT 1
			FROM upstream_relay_connectors
			WHERE id=$1 AND deleted_at IS NULL
		)
	`, []any{connectorID}, &connectorExists); err != nil {
		return err
	}
	if !connectorExists {
		return infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_CONNECTOR_MISSING", "candidate connector must exist")
	}
	var accountExists bool
	if err := scanSingleRow(ctx, r.db, `
		SELECT EXISTS (
			SELECT 1
			FROM accounts
			WHERE id=$1
		)
	`, []any{accountID}, &accountExists); err != nil {
		return err
	}
	if !accountExists {
		return infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_ACCOUNT_MISSING", "candidate account must exist")
	}
	return nil
}

func (r *upstreamRelayRepository) SoftDeleteCandidate(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE upstream_relay_candidates SET deleted_at=NOW(), enabled=FALSE, updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return service.ErrUpstreamRelayCandidateNotFound
	}
	return nil
}

func (r *upstreamRelayRepository) InsertProbeResult(ctx context.Context, result service.UpstreamRelayProbeResult) (*service.UpstreamRelayProbeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if err := scanSingleRow(ctx, tx, `
		INSERT INTO upstream_relay_probe_results (
			candidate_id, success, latency_ms, http_status, error_class, error_message, probed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())
		RETURNING id
	`, []any{
		result.CandidateID, result.Success, nullableIntValue(result.LatencyMs), nullableIntValue(result.HTTPStatus),
		result.ErrorClass, result.ErrorMessage,
	}, &id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE upstream_relay_candidates SET last_probe_result_id=$2, updated_at=NOW() WHERE id=$1`, result.CandidateID, id); err != nil {
		return nil, err
	}
	if err := r.refreshCandidateHealthSnapshot(ctx, tx, result.CandidateID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	candidate, err := r.GetCandidate(ctx, result.CandidateID)
	if err != nil {
		return nil, err
	}
	return candidate.LatestProbe, nil
}

func (r *upstreamRelayRepository) refreshCandidateHealthSnapshot(ctx context.Context, tx *sql.Tx, candidateID int64) error {
	probes, err := r.recentCandidateProbeRows(ctx, tx, []int64{candidateID}, upstreamRelayHealthSnapshotWindowMinutes, upstreamRelayHealthSnapshotSampleLimit)
	if err != nil {
		return err
	}
	health := buildRelayHealth(probes[candidateID], upstreamRelayHealthSnapshotWindowMinutes)
	if health == nil {
		health = &service.UpstreamRelayCandidateHealth{WindowMinutes: upstreamRelayHealthSnapshotWindowMinutes}
	}
	health.WindowMinutes = upstreamRelayHealthSnapshotWindowMinutes
	if health.CalculatedAt.IsZero() {
		health.CalculatedAt = time.Now()
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO upstream_relay_candidate_health_snapshots (
			candidate_id, probe_count, success_count, success_rate, avg_latency_ms, p95_latency_ms,
			consecutive_successes, consecutive_failures, last_error_class, last_success_at,
			window_minutes, sample_size, calculated_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (candidate_id) DO UPDATE SET
			probe_count=EXCLUDED.probe_count,
			success_count=EXCLUDED.success_count,
			success_rate=EXCLUDED.success_rate,
			avg_latency_ms=EXCLUDED.avg_latency_ms,
			p95_latency_ms=EXCLUDED.p95_latency_ms,
			consecutive_successes=EXCLUDED.consecutive_successes,
			consecutive_failures=EXCLUDED.consecutive_failures,
			last_error_class=EXCLUDED.last_error_class,
			last_success_at=EXCLUDED.last_success_at,
			window_minutes=EXCLUDED.window_minutes,
			sample_size=EXCLUDED.sample_size,
			calculated_at=EXCLUDED.calculated_at,
			updated_at=NOW()
	`, candidateID, health.ProbeCount, health.SuccessCount, health.SuccessRate,
		nullableIntValue(health.AvgLatencyMs), nullableIntValue(health.P95LatencyMs),
		health.ConsecutiveSuccesses, health.ConsecutiveFailures, health.LastErrorClass,
		health.LastSuccessAt, health.WindowMinutes, health.SampleSize, health.CalculatedAt)
	return err
}

func (r *upstreamRelayRepository) InsertUsageDeltaSample(ctx context.Context, sample service.UpstreamRelayUsageDeltaSample) (*service.UpstreamRelayUsageDeltaSample, error) {
	var id int64
	if err := scanSingleRow(ctx, r.db, `
		INSERT INTO upstream_relay_usage_delta_samples (
			candidate_id, probe_result_id, model, status,
			before_cost, before_actual_cost, after_cost, after_actual_cost,
			cost_delta, actual_cost_delta, derived_rate_multiplier,
			unreliable_reason, sampled_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())
		RETURNING id
	`, []any{
		sample.CandidateID, nullableInt64Value(sample.ProbeResultID), sample.Model, sample.Status,
		nullableFloat64Value(sample.BeforeCost), nullableFloat64Value(sample.BeforeActualCost),
		nullableFloat64Value(sample.AfterCost), nullableFloat64Value(sample.AfterActualCost),
		nullableFloat64Value(sample.CostDelta), nullableFloat64Value(sample.ActualCostDelta),
		nullableFloat64Value(sample.DerivedRateMultiplier), sample.UnreliableReason,
	}, &id); err != nil {
		return nil, err
	}
	samples, err := r.latestUsageDeltaSamples(ctx, []int64{sample.CandidateID})
	if err != nil {
		return nil, err
	}
	if got := samples[sample.CandidateID]; got != nil {
		return got, nil
	}
	sample.ID = id
	return &sample, nil
}

func (r *upstreamRelayRepository) ListRecommendationInputs(ctx context.Context) ([]service.UpstreamRelayCandidate, error) {
	rows, err := r.db.QueryContext(ctx, relayCandidateSelect()+`
		WHERE c.deleted_at IS NULL
		  AND c.enabled = TRUE
		  AND rc.status = $1
	`, service.UpstreamRelayConnectorStatusActive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanRelayCandidates(rows)
	if err != nil {
		return nil, err
	}
	if err := r.decorateRelayCandidates(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *upstreamRelayRepository) GetMonitoringPolicy(ctx context.Context) (*service.UpstreamRelayMonitoringPolicy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT auto_sync_enabled, sync_interval_minutes, auto_probe_enabled, probe_interval_minutes,
		       failure_retry_interval_minutes, sync_concurrency, probe_concurrency,
		       COALESCE(updated_by, 0), created_at, updated_at
		FROM upstream_relay_monitoring_policy
		WHERE id = 1
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, nil
	}
	policy, err := scanRelayMonitoringPolicy(rows)
	if err != nil {
		return nil, err
	}
	return &policy, rows.Err()
}

func (r *upstreamRelayRepository) UpsertMonitoringPolicy(ctx context.Context, policy service.UpstreamRelayMonitoringPolicy, operatorID int64) (*service.UpstreamRelayMonitoringPolicy, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO upstream_relay_monitoring_policy (
			id, auto_sync_enabled, sync_interval_minutes, auto_probe_enabled, probe_interval_minutes,
			failure_retry_interval_minutes, sync_concurrency, probe_concurrency,
			updated_by, created_at, updated_at
		)
		VALUES (1,$1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE SET
			auto_sync_enabled=EXCLUDED.auto_sync_enabled,
			sync_interval_minutes=EXCLUDED.sync_interval_minutes,
			auto_probe_enabled=EXCLUDED.auto_probe_enabled,
			probe_interval_minutes=EXCLUDED.probe_interval_minutes,
			failure_retry_interval_minutes=EXCLUDED.failure_retry_interval_minutes,
			sync_concurrency=EXCLUDED.sync_concurrency,
			probe_concurrency=EXCLUDED.probe_concurrency,
			updated_by=EXCLUDED.updated_by,
			updated_at=NOW()
	`, policy.AutoSyncEnabled, policy.SyncIntervalMinutes, policy.AutoProbeEnabled, policy.ProbeIntervalMinutes,
		policy.FailureRetryIntervalMinutes, policy.SyncConcurrency, policy.ProbeConcurrency, operatorID); err != nil {
		return nil, err
	}
	return r.GetMonitoringPolicy(ctx)
}

func (r *upstreamRelayRepository) GetRecommendationPolicy(ctx context.Context) (*service.UpstreamRelayRecommendationPolicy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT snapshot_freshness_minutes, usage_delta_freshness_minutes, probe_freshness_minutes,
		       min_success_rate, min_sample_size, exclude_consecutive_failures,
		       priority_start, priority_step, sort_fields, COALESCE(updated_by, 0),
		       created_at, updated_at
		FROM upstream_relay_recommendation_policy
		WHERE id = 1
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, nil
	}
	policy, err := scanRelayRecommendationPolicy(rows)
	if err != nil {
		return nil, err
	}
	return &policy, rows.Err()
}

func (r *upstreamRelayRepository) UpsertRecommendationPolicy(ctx context.Context, policy service.UpstreamRelayRecommendationPolicy, operatorID int64) (*service.UpstreamRelayRecommendationPolicy, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO upstream_relay_recommendation_policy (
			id, snapshot_freshness_minutes, usage_delta_freshness_minutes, probe_freshness_minutes,
			min_success_rate, min_sample_size, exclude_consecutive_failures,
			priority_start, priority_step, sort_fields, updated_by, created_at, updated_at
		)
		VALUES (1,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE SET
			snapshot_freshness_minutes=EXCLUDED.snapshot_freshness_minutes,
			usage_delta_freshness_minutes=EXCLUDED.usage_delta_freshness_minutes,
			probe_freshness_minutes=EXCLUDED.probe_freshness_minutes,
			min_success_rate=EXCLUDED.min_success_rate,
			min_sample_size=EXCLUDED.min_sample_size,
			exclude_consecutive_failures=EXCLUDED.exclude_consecutive_failures,
			priority_start=EXCLUDED.priority_start,
			priority_step=EXCLUDED.priority_step,
			sort_fields=EXCLUDED.sort_fields,
			updated_by=EXCLUDED.updated_by,
			updated_at=NOW()
	`, policy.SnapshotFreshnessMinutes, policy.UsageDeltaFreshnessMinutes, policy.ProbeFreshnessMinutes,
		policy.MinSuccessRate, policy.MinSampleSize, policy.ExcludeConsecutiveFailures,
		policy.PriorityStart, policy.PriorityStep, pq.Array(policy.SortFields), operatorID); err != nil {
		return nil, err
	}
	return r.GetRecommendationPolicy(ctx)
}

func (r *upstreamRelayRepository) CreateRecommendationRun(ctx context.Context, run service.UpstreamRelayRecommendationRun, suggestions []service.UpstreamRelayRecommendationSuggestion) (*service.UpstreamRelayRecommendationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var runID int64
	if err := scanSingleRow(ctx, tx, `
		INSERT INTO upstream_relay_recommendation_runs (
			status, total_candidates, suggestion_count, error_message, created_by, created_at
		)
		VALUES ($1,$2,$3,$4,$5,NOW())
		RETURNING id
	`, []any{run.Status, run.TotalCandidates, len(suggestions), nullStringIfEmpty(run.ErrorMessage), nullableInt64Value(&run.CreatedBy)}, &runID); err != nil {
		return nil, err
	}
	for _, suggestion := range suggestions {
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO upstream_relay_recommendation_suggestions (
					run_id, candidate_id, connector_id, account_id, upstream_group_id,
					old_priority, new_priority, final_rate_multiplier,
					health_status, reason_code, confidence, health_summary, rate_source,
					reason, applied, created_at
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,FALSE,NOW())
			`, runID, suggestion.CandidateID, suggestion.ConnectorID, suggestion.AccountID,
			suggestion.UpstreamGroupID, nullableIntValue(suggestion.OldPriority), suggestion.NewPriority,
			suggestion.FinalRateMultiplier, suggestion.HealthStatus,
			suggestion.ReasonCode, suggestion.Confidence, suggestion.HealthSummary, suggestion.RateSource,
			suggestion.Reason); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRecommendationRun(ctx, runID)
}

func (r *upstreamRelayRepository) GetRecommendationRun(ctx context.Context, id int64) (*service.UpstreamRelayRecommendationRun, error) {
	run, err := r.getRelayRunSummary(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	suggestions, err := r.listRelaySuggestions(ctx, id)
	if err != nil {
		return nil, err
	}
	run.Suggestions = suggestions
	return run, nil
}

func (r *upstreamRelayRepository) ListRecommendationRuns(ctx context.Context, params pagination.PaginationParams) ([]service.UpstreamRelayRecommendationRun, *pagination.PaginationResult, error) {
	page, pageSize := normalizePolicyPagination(params)
	var total int64
	if err := scanSingleRow(ctx, r.db, `SELECT COUNT(*) FROM upstream_relay_recommendation_runs`, nil, &total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, status, total_candidates, suggestion_count, applied, applied_by,
		       applied_at, COALESCE(error_message, ''), COALESCE(created_by, 0), created_at
		FROM upstream_relay_recommendation_runs
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	runs := []service.UpstreamRelayRecommendationRun{}
	for rows.Next() {
		run, err := scanRelayRun(rows)
		if err != nil {
			return nil, nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return runs, relayPage(total, page, pageSize), nil
}

func (r *upstreamRelayRepository) ApplyRecommendationRun(ctx context.Context, runID, operatorID int64) (*service.UpstreamRelayRecommendationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var applied bool
	if err := scanSingleRow(ctx, tx, `
		SELECT status, applied FROM upstream_relay_recommendation_runs WHERE id=$1 FOR UPDATE
	`, []any{runID}, &status, &applied); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUpstreamRelayRunNotFound
		}
		return nil, err
	}
	if status != service.UpstreamRelayRunStatusSuccess {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_RUN_NOT_SUCCESS", "only successful recommendation runs can be applied")
	}
	if applied {
		return nil, infraerrors.Conflict("UPSTREAM_RELAY_RUN_ALREADY_APPLIED", "recommendation run already applied")
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT candidate_id, account_id, old_priority, new_priority
		FROM upstream_relay_recommendation_suggestions
		WHERE run_id=$1 AND applied=FALSE
		ORDER BY new_priority ASC, candidate_id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	type row struct {
		candidateID int64
		accountID   int64
		oldPriority sql.NullInt64
		newPriority int
	}
	suggestions := []row{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.candidateID, &item.accountID, &item.oldPriority, &item.newPriority); err != nil {
			_ = rows.Close()
			return nil, err
		}
		suggestions = append(suggestions, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, service.ErrUpstreamRelayNoPendingSuggestions
	}
	affectedAccounts := map[int64]struct{}{}
	for _, suggestion := range suggestions {
		oldPriority := nullableInt64Value(nullInt64Ptr(suggestion.oldPriority))
		res, err := tx.ExecContext(ctx, `
			UPDATE accounts
			SET priority=$2, updated_at=NOW()
			WHERE id=$1 AND priority IS NOT DISTINCT FROM $3
		`, suggestion.accountID, suggestion.newPriority, oldPriority)
		if err != nil {
			return nil, err
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return nil, infraerrors.Conflict("UPSTREAM_RELAY_RECOMMENDATION_STALE", "recommendation target changed after this run")
		}
		affectedAccounts[suggestion.accountID] = struct{}{}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_relay_recommendation_suggestions
		SET applied=TRUE, applied_by=$2, applied_at=NOW()
		WHERE run_id=$1 AND applied=FALSE
	`, runID, operatorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE upstream_relay_recommendation_runs
		SET applied=TRUE, applied_by=$2, applied_at=NOW()
		WHERE id=$1
	`, runID, operatorID); err != nil {
		return nil, err
	}
	for accountID := range affectedAccounts {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRecommendationRun(ctx, runID)
}

func (r *upstreamRelayRepository) DeleteRecommendationRun(ctx context.Context, runID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var applied bool
	if err := scanSingleRow(ctx, tx, `
		SELECT applied FROM upstream_relay_recommendation_runs WHERE id=$1 FOR UPDATE
	`, []any{runID}, &applied); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUpstreamRelayRunNotFound
		}
		return err
	}
	if applied {
		return service.ErrUpstreamRelayAppliedRunDelete
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM upstream_relay_recommendation_suggestions WHERE run_id=$1
	`, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM upstream_relay_recommendation_runs WHERE id=$1
	`, runID); err != nil {
		return err
	}
	return tx.Commit()
}

func relayCandidateSelect() string {
	return `
		SELECT c.id, c.connector_id, COALESCE(rc.name, ''), rc.status, c.account_id,
		       s.today_actual_cost, s.today_total_tokens, s.today_usage_checked_at,
		       COALESCE(a.name, ''), COALESCE(a.platform, ''), c.upstream_group_id,
		       COALESCE(s.name, ''), c.upstream_api_key_id,
		       COALESCE(c.upstream_api_key_name, ''), COALESCE(c.upstream_api_key_masked, ''),
		       c.probe_model, c.probe_protocol, a.priority,
		       c.enabled, c.notes,
		       c.last_probe_result_id, COALESCE(c.created_by, 0), c.created_at, c.updated_at,
		       pr.id, pr.success, pr.latency_ms, pr.http_status, COALESCE(pr.error_class, ''),
		       COALESCE(pr.error_message, ''), pr.probed_at,
		       s.id, s.default_rate_multiplier, s.override_rate_multiplier, s.final_rate_multiplier,
		       s.today_actual_cost, s.today_total_tokens, s.today_usage_checked_at,
		       COALESCE(s.platform, ''), COALESCE(s.status, ''), COALESCE(s.source, ''), s.last_seen_at
		FROM upstream_relay_candidates c
		JOIN upstream_relay_connectors rc ON rc.id = c.connector_id AND rc.deleted_at IS NULL
		JOIN accounts a ON a.id = c.account_id
		LEFT JOIN upstream_relay_probe_results pr ON pr.id = c.last_probe_result_id
		LEFT JOIN upstream_relay_group_rate_snapshots s ON s.connector_id = c.connector_id AND s.upstream_group_id = c.upstream_group_id
	`
}

func scanRelayConnectors(rows *sql.Rows) ([]service.UpstreamRelayConnector, error) {
	items := []service.UpstreamRelayConnector{}
	for rows.Next() {
		var item service.UpstreamRelayConnector
		var upstreamBalance sql.NullFloat64
		var upstreamBalanceCheckedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.AuthMode, &item.BearerTokenEncrypted,
			&item.RefreshTokenEncrypted, &item.LoginEmailEncrypted, &item.CookieEncrypted, &item.UserAgentEncrypted,
			&upstreamBalance, &upstreamBalanceCheckedAt,
			&item.Status, &item.CredentialVersion,
			&item.LastVerifiedAt, &item.LastSyncedAt, &item.LastError, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.UpstreamAccountBalance = nullableFloat64Ptr(upstreamBalance)
		if upstreamBalanceCheckedAt.Valid {
			item.UpstreamAccountBalanceCheckedAt = &upstreamBalanceCheckedAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRelaySnapshots(rows *sql.Rows) ([]service.UpstreamRelayGroupRateSnapshot, error) {
	items := []service.UpstreamRelayGroupRateSnapshot{}
	for rows.Next() {
		var item service.UpstreamRelayGroupRateSnapshot
		var override sql.NullFloat64
		var todayActualCost sql.NullFloat64
		var todayTotalTokens sql.NullInt64
		var todayUsageCheckedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ConnectorID, &item.UpstreamGroupID, &item.Name, &item.Platform,
			&item.Status, &item.DefaultRateMultiplier, &override, &item.FinalRateMultiplier,
			&todayActualCost, &todayTotalTokens, &todayUsageCheckedAt,
			&item.Source, &item.LastSeenAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.OverrideRateMultiplier = nullableFloat64Ptr(override)
		item.TodayActualCost = nullableFloat64Ptr(todayActualCost)
		item.TodayTotalTokens = nullInt64Ptr(todayTotalTokens)
		if todayUsageCheckedAt.Valid {
			item.TodayUsageCheckedAt = &todayUsageCheckedAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRelaySnapshotChanges(rows *sql.Rows) ([]service.UpstreamRelayGroupRateSnapshotChange, error) {
	items := []service.UpstreamRelayGroupRateSnapshotChange{}
	for rows.Next() {
		var item service.UpstreamRelayGroupRateSnapshotChange
		var oldRate, newRate sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.ConnectorID, &item.ConnectorName, &item.UpstreamGroupID,
			&item.GroupName, &item.Platform, &item.ChangeType, &oldRate, &newRate,
			&item.OldStatus, &item.NewStatus, &item.Source, &item.ChangedAt); err != nil {
			return nil, err
		}
		item.OldFinalRateMultiplier = nullableFloat64Ptr(oldRate)
		item.NewFinalRateMultiplier = nullableFloat64Ptr(newRate)
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRelayCandidates(rows *sql.Rows) ([]service.UpstreamRelayCandidate, error) {
	items := []service.UpstreamRelayCandidate{}
	for rows.Next() {
		var item service.UpstreamRelayCandidate
		var priority sql.NullInt64
		var probeID, probeLatency, probeHTTP sql.NullInt64
		var probeSuccess sql.NullBool
		var probeErrorClass, probeErrorMessage sql.NullString
		var probeAt sql.NullTime
		var snapshotID sql.NullInt64
		var defaultRate, overrideRate, finalRate sql.NullFloat64
		var todayActualCost, snapshotTodayActualCost sql.NullFloat64
		var todayTotalTokens, snapshotTodayTotalTokens sql.NullInt64
		var todayUsageCheckedAt, snapshotTodayUsageCheckedAt sql.NullTime
		var snapshotPlatform, snapshotStatus, snapshotSource sql.NullString
		var snapshotSeen sql.NullTime
		var upstreamAPIKeyID sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ConnectorID, &item.ConnectorName, &item.ConnectorStatus, &item.AccountID,
			&todayActualCost, &todayTotalTokens, &todayUsageCheckedAt,
			&item.AccountName, &item.AccountPlatform, &item.UpstreamGroupID,
			&item.UpstreamGroupName, &upstreamAPIKeyID, &item.UpstreamAPIKeyName, &item.UpstreamAPIKeyMasked,
			&item.ProbeModel, &item.ProbeProtocol, &priority,
			&item.Enabled, &item.Notes,
			&item.LastProbeResultID, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
			&probeID, &probeSuccess, &probeLatency, &probeHTTP, &probeErrorClass,
			&probeErrorMessage, &probeAt,
			&snapshotID, &defaultRate, &overrideRate, &finalRate,
			&snapshotTodayActualCost, &snapshotTodayTotalTokens, &snapshotTodayUsageCheckedAt,
			&snapshotPlatform, &snapshotStatus, &snapshotSource, &snapshotSeen,
		); err != nil {
			return nil, err
		}
		item.TodayActualCost = nullableFloat64Ptr(todayActualCost)
		item.TodayTotalTokens = nullInt64Ptr(todayTotalTokens)
		if todayUsageCheckedAt.Valid {
			item.TodayUsageCheckedAt = &todayUsageCheckedAt.Time
		}
		item.UpstreamAPIKeyID = nullableInt64Ptr(upstreamAPIKeyID)
		item.CurrentPriority = nullableIntFromSQL(priority)
		if probeID.Valid {
			latency := nullableIntFromSQL(probeLatency)
			httpStatus := nullableIntFromSQL(probeHTTP)
			item.LatestProbe = &service.UpstreamRelayProbeResult{
				ID:           probeID.Int64,
				CandidateID:  item.ID,
				Success:      probeSuccess.Valid && probeSuccess.Bool,
				LatencyMs:    latency,
				HTTPStatus:   httpStatus,
				ErrorClass:   probeErrorClass.String,
				ErrorMessage: probeErrorMessage.String,
				ProbedAt:     probeAt.Time,
			}
		}
		if snapshotID.Valid {
			item.LatestSnapshot = &service.UpstreamRelayGroupRateSnapshot{
				ID:                     snapshotID.Int64,
				ConnectorID:            item.ConnectorID,
				UpstreamGroupID:        item.UpstreamGroupID,
				Name:                   item.UpstreamGroupName,
				Platform:               snapshotPlatform.String,
				Status:                 snapshotStatus.String,
				DefaultRateMultiplier:  defaultRate.Float64,
				OverrideRateMultiplier: nullableFloat64Ptr(overrideRate),
				FinalRateMultiplier:    finalRate.Float64,
				TodayActualCost:        nullableFloat64Ptr(snapshotTodayActualCost),
				TodayTotalTokens:       nullInt64Ptr(snapshotTodayTotalTokens),
				Source:                 snapshotSource.String,
				LastSeenAt:             snapshotSeen.Time,
			}
			if snapshotTodayUsageCheckedAt.Valid {
				item.LatestSnapshot.TodayUsageCheckedAt = &snapshotTodayUsageCheckedAt.Time
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRelayMonitoringPolicy(rows *sql.Rows) (service.UpstreamRelayMonitoringPolicy, error) {
	var policy service.UpstreamRelayMonitoringPolicy
	if err := rows.Scan(
		&policy.AutoSyncEnabled,
		&policy.SyncIntervalMinutes,
		&policy.AutoProbeEnabled,
		&policy.ProbeIntervalMinutes,
		&policy.FailureRetryIntervalMinutes,
		&policy.SyncConcurrency,
		&policy.ProbeConcurrency,
		&policy.UpdatedBy,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return policy, err
	}
	return policy, nil
}

func scanRelayRecommendationPolicy(rows *sql.Rows) (service.UpstreamRelayRecommendationPolicy, error) {
	var policy service.UpstreamRelayRecommendationPolicy
	sortFields := []string{}
	if err := rows.Scan(
		&policy.SnapshotFreshnessMinutes,
		&policy.UsageDeltaFreshnessMinutes,
		&policy.ProbeFreshnessMinutes,
		&policy.MinSuccessRate,
		&policy.MinSampleSize,
		&policy.ExcludeConsecutiveFailures,
		&policy.PriorityStart,
		&policy.PriorityStep,
		pq.Array(&sortFields),
		&policy.UpdatedBy,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return policy, err
	}
	policy.SortFields = sortFields
	return policy, nil
}

func (r *upstreamRelayRepository) getRelayRunSummary(ctx context.Context, q sqlQueryer, id int64) (*service.UpstreamRelayRecommendationRun, error) {
	row, err := q.QueryContext(ctx, `
		SELECT id, status, total_candidates, suggestion_count, applied, applied_by,
		       applied_at, COALESCE(error_message, ''), COALESCE(created_by, 0), created_at
		FROM upstream_relay_recommendation_runs
		WHERE id=$1
	`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = row.Close() }()
	if !row.Next() {
		return nil, service.ErrUpstreamRelayRunNotFound
	}
	run, err := scanRelayRun(row)
	if err != nil {
		return nil, err
	}
	return &run, row.Err()
}

func scanRelayRun(rows *sql.Rows) (service.UpstreamRelayRecommendationRun, error) {
	var run service.UpstreamRelayRecommendationRun
	var appliedBy sql.NullInt64
	if err := rows.Scan(&run.ID, &run.Status, &run.TotalCandidates, &run.SuggestionCount, &run.Applied,
		&appliedBy, &run.AppliedAt, &run.ErrorMessage, &run.CreatedBy, &run.CreatedAt); err != nil {
		return run, err
	}
	run.AppliedBy = nullInt64Ptr(appliedBy)
	return run, nil
}

func (r *upstreamRelayRepository) listRelaySuggestions(ctx context.Context, runID int64) ([]service.UpstreamRelayRecommendationSuggestion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.run_id, s.candidate_id, s.connector_id, COALESCE(rc.name, ''),
		       s.account_id, COALESCE(a.name, ''), s.upstream_group_id, COALESCE(gs.name, ''),
		       s.old_priority, s.new_priority,
		       s.final_rate_multiplier, s.health_status, COALESCE(s.reason_code, ''),
		       COALESCE(s.confidence, ''), COALESCE(s.health_summary, ''), COALESCE(s.rate_source, ''),
		       s.reason, s.applied, s.applied_by,
		       s.applied_at, s.created_at
		FROM upstream_relay_recommendation_suggestions s
		LEFT JOIN upstream_relay_connectors rc ON rc.id = s.connector_id
		LEFT JOIN accounts a ON a.id = s.account_id
		LEFT JOIN upstream_relay_group_rate_snapshots gs ON gs.connector_id=s.connector_id AND gs.upstream_group_id=s.upstream_group_id
		WHERE s.run_id=$1
		ORDER BY s.new_priority ASC, s.candidate_id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []service.UpstreamRelayRecommendationSuggestion{}
	for rows.Next() {
		var item service.UpstreamRelayRecommendationSuggestion
		var oldPriority, appliedBy sql.NullInt64
		if err := rows.Scan(&item.ID, &item.RunID, &item.CandidateID, &item.ConnectorID, &item.ConnectorName,
			&item.AccountID, &item.AccountName, &item.UpstreamGroupID, &item.UpstreamGroupName,
			&oldPriority, &item.NewPriority, &item.FinalRateMultiplier, &item.HealthStatus, &item.ReasonCode, &item.Confidence,
			&item.HealthSummary, &item.RateSource, &item.Reason, &item.Applied, &appliedBy,
			&item.AppliedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.OldPriority = nullableIntFromSQL(oldPriority)
		item.AppliedBy = nullInt64Ptr(appliedBy)
		items = append(items, item)
	}
	return items, rows.Err()
}

type relayProbeRow struct {
	candidateID int64
	success     bool
	latency     sql.NullInt64
	errorClass  string
	probedAt    time.Time
}

func (r *upstreamRelayRepository) decorateRelayCandidates(ctx context.Context, items []service.UpstreamRelayCandidate) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	health, err := r.candidateHealthSnapshots(ctx, ids)
	if err != nil {
		return err
	}
	usageSamples, err := r.latestUsageDeltaSamples(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].Health = health[items[i].ID]
		items[i].LatestUsageDelta = usageSamples[items[i].ID]
	}
	return nil
}

func (r *upstreamRelayRepository) candidateHealthSummaries(ctx context.Context, ids []int64) (map[int64]*service.UpstreamRelayCandidateHealth, error) {
	grouped, err := r.recentCandidateProbeRows(ctx, r.db, ids, 30, 20)
	if err != nil {
		return nil, err
	}
	out := map[int64]*service.UpstreamRelayCandidateHealth{}
	for id, probes := range grouped {
		out[id] = buildRelayHealth(probes, 30)
	}
	return out, nil
}

func (r *upstreamRelayRepository) recentCandidateProbeRows(ctx context.Context, q sqlQueryer, ids []int64, windowMinutes, sampleLimit int) (map[int64][]relayProbeRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT candidate_id, success, latency_ms, COALESCE(error_class, ''), probed_at
		FROM (
			SELECT candidate_id, success, latency_ms, error_class, probed_at,
			       ROW_NUMBER() OVER (PARTITION BY candidate_id ORDER BY probed_at DESC, id DESC) AS rn
			FROM upstream_relay_probe_results
			WHERE candidate_id = ANY($1)
			  AND probed_at >= NOW() - ($2::INT * INTERVAL '1 minute')
		) recent
		WHERE rn <= $3
		ORDER BY candidate_id ASC, probed_at DESC
	`, pq.Array(ids), windowMinutes, sampleLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	grouped := map[int64][]relayProbeRow{}
	for rows.Next() {
		var row relayProbeRow
		if err := rows.Scan(&row.candidateID, &row.success, &row.latency, &row.errorClass, &row.probedAt); err != nil {
			return nil, err
		}
		grouped[row.candidateID] = append(grouped[row.candidateID], row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grouped, nil
}

func (r *upstreamRelayRepository) candidateHealthSnapshots(ctx context.Context, ids []int64) (map[int64]*service.UpstreamRelayCandidateHealth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT candidate_id, probe_count, success_count, success_rate,
		       avg_latency_ms, p95_latency_ms, consecutive_successes, consecutive_failures,
		       COALESCE(last_error_class, ''), last_success_at, window_minutes, sample_size, calculated_at
		FROM upstream_relay_candidate_health_snapshots
		WHERE candidate_id = ANY($1)
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]*service.UpstreamRelayCandidateHealth{}
	for rows.Next() {
		var candidateID int64
		var health service.UpstreamRelayCandidateHealth
		var avgLatency, p95Latency sql.NullInt64
		var lastSuccessAt sql.NullTime
		if err := rows.Scan(
			&candidateID, &health.ProbeCount, &health.SuccessCount, &health.SuccessRate,
			&avgLatency, &p95Latency, &health.ConsecutiveSuccesses, &health.ConsecutiveFailures,
			&health.LastErrorClass, &lastSuccessAt, &health.WindowMinutes, &health.SampleSize, &health.CalculatedAt,
		); err != nil {
			return nil, err
		}
		health.AvgLatencyMs = nullableIntFromSQL(avgLatency)
		health.P95LatencyMs = nullableIntFromSQL(p95Latency)
		if lastSuccessAt.Valid {
			health.LastSuccessAt = &lastSuccessAt.Time
		}
		out[candidateID] = &health
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == len(ids) {
		return out, nil
	}
	fallback, err := r.candidateHealthSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, health := range fallback {
		if _, ok := out[id]; !ok {
			out[id] = health
		}
	}
	return out, nil
}

func buildRelayHealth(probes []relayProbeRow, windowMinutes int) *service.UpstreamRelayCandidateHealth {
	health := &service.UpstreamRelayCandidateHealth{ProbeCount: len(probes), WindowMinutes: windowMinutes, SampleSize: len(probes)}
	if len(probes) == 0 {
		return health
	}
	latencies := []int{}
	countingStreak := true
	for i, probe := range probes {
		if probe.success {
			health.SuccessCount++
			if health.LastSuccessAt == nil {
				t := probe.probedAt
				health.LastSuccessAt = &t
			}
		} else if health.LastErrorClass == "" {
			health.LastErrorClass = probe.errorClass
		}
		if probe.latency.Valid {
			latencies = append(latencies, int(probe.latency.Int64))
		}
		if i == 0 {
			if probe.success {
				health.ConsecutiveSuccesses = 1
			} else {
				health.ConsecutiveFailures = 1
			}
			continue
		}
		if !countingStreak {
			continue
		}
		if probe.success == probes[0].success {
			if probe.success {
				health.ConsecutiveSuccesses++
			} else {
				health.ConsecutiveFailures++
			}
			continue
		}
		countingStreak = false
	}
	health.SuccessRate = float64(health.SuccessCount) / float64(len(probes))
	if len(latencies) > 0 {
		total := 0
		for _, latency := range latencies {
			total += latency
		}
		avg := int(math.Round(float64(total) / float64(len(latencies))))
		sort.Ints(latencies)
		p95Index := int(math.Ceil(float64(len(latencies))*0.95)) - 1
		if p95Index < 0 {
			p95Index = 0
		}
		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}
		p95 := latencies[p95Index]
		health.AvgLatencyMs = &avg
		health.P95LatencyMs = &p95
	}
	return health
}

func (r *upstreamRelayRepository) latestUsageDeltaSamples(ctx context.Context, ids []int64) (map[int64]*service.UpstreamRelayUsageDeltaSample, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, candidate_id, probe_result_id, model, status,
		       before_cost, before_actual_cost, after_cost, after_actual_cost,
		       cost_delta, actual_cost_delta, derived_rate_multiplier,
		       COALESCE(unreliable_reason, ''), sampled_at
		FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY candidate_id ORDER BY sampled_at DESC, id DESC) AS rn
			FROM upstream_relay_usage_delta_samples
			WHERE candidate_id = ANY($1)
		) latest
		WHERE rn = 1
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]*service.UpstreamRelayUsageDeltaSample{}
	for rows.Next() {
		sample, err := scanUsageDeltaSample(rows)
		if err != nil {
			return nil, err
		}
		out[sample.CandidateID] = sample
	}
	return out, rows.Err()
}

func scanUsageDeltaSample(rows *sql.Rows) (*service.UpstreamRelayUsageDeltaSample, error) {
	var item service.UpstreamRelayUsageDeltaSample
	var probeID sql.NullInt64
	var beforeCost, beforeActualCost, afterCost, afterActualCost sql.NullFloat64
	var costDelta, actualCostDelta, derived sql.NullFloat64
	if err := rows.Scan(
		&item.ID, &item.CandidateID, &probeID, &item.Model, &item.Status,
		&beforeCost, &beforeActualCost, &afterCost, &afterActualCost,
		&costDelta, &actualCostDelta, &derived,
		&item.UnreliableReason, &item.SampledAt,
	); err != nil {
		return nil, err
	}
	item.ProbeResultID = nullInt64Ptr(probeID)
	item.BeforeCost = nullableFloat64Ptr(beforeCost)
	item.BeforeActualCost = nullableFloat64Ptr(beforeActualCost)
	item.AfterCost = nullableFloat64Ptr(afterCost)
	item.AfterActualCost = nullableFloat64Ptr(afterActualCost)
	item.CostDelta = nullableFloat64Ptr(costDelta)
	item.ActualCostDelta = nullableFloat64Ptr(actualCostDelta)
	item.DerivedRateMultiplier = nullableFloat64Ptr(derived)
	return &item, nil
}

func listRelaySnapshotsTx(ctx context.Context, tx *sql.Tx, connectorID int64) ([]service.UpstreamRelayGroupRateSnapshot, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, connector_id, upstream_group_id, name, platform, status,
		       default_rate_multiplier, override_rate_multiplier, final_rate_multiplier,
		       today_actual_cost, today_total_tokens, today_usage_checked_at,
		       source, last_seen_at, created_at, updated_at
		FROM upstream_relay_group_rate_snapshots
		WHERE connector_id = $1
	`, connectorID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRelaySnapshots(rows)
}

func buildRelaySnapshotChanges(connectorID int64, previous, next []service.UpstreamRelayGroupRateSnapshot) []service.UpstreamRelayGroupRateSnapshotChange {
	if len(previous) == 0 {
		return nil
	}
	previousByGroup := make(map[string]service.UpstreamRelayGroupRateSnapshot, len(previous))
	for _, snapshot := range previous {
		previousByGroup[snapshot.UpstreamGroupID] = snapshot
	}
	nextByGroup := make(map[string]service.UpstreamRelayGroupRateSnapshot, len(next))
	for _, snapshot := range next {
		nextByGroup[snapshot.UpstreamGroupID] = snapshot
	}
	changes := []service.UpstreamRelayGroupRateSnapshotChange{}
	for _, snapshot := range next {
		old, exists := previousByGroup[snapshot.UpstreamGroupID]
		if !exists {
			newRate := roundRelayRate(snapshot.FinalRateMultiplier)
			changes = append(changes, service.UpstreamRelayGroupRateSnapshotChange{
				ConnectorID:            connectorID,
				UpstreamGroupID:        snapshot.UpstreamGroupID,
				GroupName:              snapshot.Name,
				Platform:               snapshot.Platform,
				ChangeType:             service.UpstreamRelaySnapshotChangeAdded,
				NewFinalRateMultiplier: &newRate,
				NewStatus:              snapshot.Status,
				Source:                 snapshot.Source,
			})
			continue
		}
		oldRate := roundRelayRate(old.FinalRateMultiplier)
		newRate := roundRelayRate(snapshot.FinalRateMultiplier)
		if old.Status == "stale" {
			changes = append(changes, service.UpstreamRelayGroupRateSnapshotChange{
				ConnectorID:            connectorID,
				UpstreamGroupID:        snapshot.UpstreamGroupID,
				GroupName:              snapshot.Name,
				Platform:               snapshot.Platform,
				ChangeType:             service.UpstreamRelaySnapshotChangeAdded,
				OldFinalRateMultiplier: &oldRate,
				NewFinalRateMultiplier: &newRate,
				OldStatus:              old.Status,
				NewStatus:              snapshot.Status,
				Source:                 snapshot.Source,
			})
			continue
		}
		if oldRate == newRate {
			continue
		}
		changes = append(changes, service.UpstreamRelayGroupRateSnapshotChange{
			ConnectorID:            connectorID,
			UpstreamGroupID:        snapshot.UpstreamGroupID,
			GroupName:              snapshot.Name,
			Platform:               snapshot.Platform,
			ChangeType:             service.UpstreamRelaySnapshotChangeRateChanged,
			OldFinalRateMultiplier: &oldRate,
			NewFinalRateMultiplier: &newRate,
			OldStatus:              old.Status,
			NewStatus:              snapshot.Status,
			Source:                 snapshot.Source,
		})
	}
	for _, snapshot := range previous {
		if snapshot.Status == "stale" {
			continue
		}
		if _, exists := nextByGroup[snapshot.UpstreamGroupID]; exists {
			continue
		}
		oldRate := roundRelayRate(snapshot.FinalRateMultiplier)
		changes = append(changes, service.UpstreamRelayGroupRateSnapshotChange{
			ConnectorID:            connectorID,
			UpstreamGroupID:        snapshot.UpstreamGroupID,
			GroupName:              snapshot.Name,
			Platform:               snapshot.Platform,
			ChangeType:             service.UpstreamRelaySnapshotChangeRemoved,
			OldFinalRateMultiplier: &oldRate,
			OldStatus:              snapshot.Status,
			NewStatus:              "stale",
			Source:                 snapshot.Source,
		})
	}
	return changes
}

func roundRelayRate(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}

func nullableIntFromSQL(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int64)
	return &out
}

func nullableInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	out := v.Int64
	return &out
}

func relayPage(total int64, page, pageSize int) *pagination.PaginationResult {
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}
}

func translateRelayWriteError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "upstream_relay_candidates") && strings.Contains(msg, "accounts") {
		return infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_ACCOUNT_MISSING", "candidate account must exist")
	}
	if strings.Contains(msg, "upstream_relay_candidates") && strings.Contains(msg, "upstream_relay_connectors") {
		return infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_CONNECTOR_MISSING", "candidate connector must exist")
	}
	if strings.Contains(msg, "duplicate key") {
		return infraerrors.Conflict("UPSTREAM_RELAY_DUPLICATE", "upstream relay mapping already exists")
	}
	return err
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
