package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type upstreamRelayRepository struct {
	db *sql.DB
}

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
	seenGroupIDs := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		seenGroupIDs = append(seenGroupIDs, snapshot.UpstreamGroupID)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_relay_group_rate_snapshots (
				connector_id, upstream_group_id, name, platform, status,
				default_rate_multiplier, override_rate_multiplier, final_rate_multiplier,
				source, last_seen_at, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
			ON CONFLICT (connector_id, upstream_group_id) DO UPDATE SET
				name=EXCLUDED.name,
				platform=EXCLUDED.platform,
				status=EXCLUDED.status,
				default_rate_multiplier=EXCLUDED.default_rate_multiplier,
				override_rate_multiplier=EXCLUDED.override_rate_multiplier,
				final_rate_multiplier=EXCLUDED.final_rate_multiplier,
				source=EXCLUDED.source,
				last_seen_at=EXCLUDED.last_seen_at,
				updated_at=NOW()
		`, connectorID, snapshot.UpstreamGroupID, snapshot.Name, snapshot.Platform, snapshot.Status,
			snapshot.DefaultRateMultiplier, nullableFloat64Value(snapshot.OverrideRateMultiplier),
			snapshot.FinalRateMultiplier, snapshot.Source, snapshot.LastSeenAt); err != nil {
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
	return &items[0], nil
}

func (r *upstreamRelayRepository) CreateCandidate(ctx context.Context, candidate *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	var id int64
	if err := scanSingleRow(ctx, r.db, `
		INSERT INTO upstream_relay_candidates (
			connector_id, account_id, upstream_group_id, probe_model, probe_protocol,
			target_group_id, enabled, notes, created_by, created_at, updated_at
		)
		SELECT $1, ag.account_id, $3, $4, $5, ag.group_id, $7, $8, $9, NOW(), NOW()
		FROM account_groups ag
		WHERE ag.account_id=$2 AND ag.group_id=$6
		RETURNING id
	`, []any{
		candidate.ConnectorID, candidate.AccountID, candidate.UpstreamGroupID,
		candidate.ProbeModel, candidate.ProbeProtocol, candidate.TargetGroupID,
		candidate.Enabled, candidate.Notes, nullableInt64Value(&candidate.CreatedBy),
	}, &id); err != nil {
		return nil, translateRelayWriteError(err)
	}
	return r.GetCandidate(ctx, id)
}

func (r *upstreamRelayRepository) UpdateCandidate(ctx context.Context, candidate *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE upstream_relay_candidates c
		SET connector_id=$2, account_id=$3, upstream_group_id=$4, probe_model=$5,
		    probe_protocol=$6, target_group_id=$7, enabled=$8, notes=$9, updated_at=NOW()
		WHERE c.id=$1 AND c.deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM account_groups ag WHERE ag.account_id=$3 AND ag.group_id=$7)
	`, candidate.ID, candidate.ConnectorID, candidate.AccountID, candidate.UpstreamGroupID,
		candidate.ProbeModel, candidate.ProbeProtocol, candidate.TargetGroupID, candidate.Enabled, candidate.Notes)
	if err != nil {
		return nil, translateRelayWriteError(err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_ACCOUNT_GROUP_MISSING", "candidate account must belong to target group")
	}
	return r.GetCandidate(ctx, candidate.ID)
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
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	candidate, err := r.GetCandidate(ctx, result.CandidateID)
	if err != nil {
		return nil, err
	}
	return candidate.LatestProbe, nil
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
	return scanRelayCandidates(rows)
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
				target_group_id, old_priority, new_priority, final_rate_multiplier,
				health_status, reason, applied, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,FALSE,NOW())
		`, runID, suggestion.CandidateID, suggestion.ConnectorID, suggestion.AccountID,
			suggestion.UpstreamGroupID, suggestion.TargetGroupID, nullableIntValue(suggestion.OldPriority),
			suggestion.NewPriority, suggestion.FinalRateMultiplier, suggestion.HealthStatus, suggestion.Reason); err != nil {
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
		SELECT candidate_id, account_id, target_group_id, old_priority, new_priority
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
		groupID     int64
		oldPriority sql.NullInt64
		newPriority int
	}
	suggestions := []row{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.candidateID, &item.accountID, &item.groupID, &item.oldPriority, &item.newPriority); err != nil {
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
	affectedGroups := map[int64][]int64{}
	for _, suggestion := range suggestions {
		oldPriority := nullableInt64Value(nullInt64Ptr(suggestion.oldPriority))
		res, err := tx.ExecContext(ctx, `
			UPDATE account_groups
			SET priority=$3
			WHERE account_id=$1 AND group_id=$2 AND priority IS NOT DISTINCT FROM $4
		`, suggestion.accountID, suggestion.groupID, suggestion.newPriority, oldPriority)
		if err != nil {
			return nil, err
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return nil, infraerrors.Conflict("UPSTREAM_RELAY_RECOMMENDATION_STALE", "recommendation target changed after this run")
		}
		affectedGroups[suggestion.accountID] = append(affectedGroups[suggestion.accountID], suggestion.groupID)
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
	for accountID, groupIDs := range affectedGroups {
		payload := buildSchedulerGroupPayload(groupIDs)
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountGroupsChanged, &accountID, nil, payload); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRecommendationRun(ctx, runID)
}

func relayCandidateSelect() string {
	return `
		SELECT c.id, c.connector_id, COALESCE(rc.name, ''), rc.status, c.account_id,
		       COALESCE(a.name, ''), COALESCE(a.platform, ''), c.upstream_group_id,
		       COALESCE(s.name, ''), c.probe_model, c.probe_protocol, c.target_group_id,
		       COALESCE(g.name, ''), ag.priority, c.enabled, c.notes,
		       c.last_probe_result_id, COALESCE(c.created_by, 0), c.created_at, c.updated_at,
		       pr.id, pr.success, pr.latency_ms, pr.http_status, COALESCE(pr.error_class, ''),
		       COALESCE(pr.error_message, ''), pr.probed_at,
		       s.id, s.default_rate_multiplier, s.override_rate_multiplier, s.final_rate_multiplier,
		       COALESCE(s.platform, ''), COALESCE(s.status, ''), COALESCE(s.source, ''), s.last_seen_at
		FROM upstream_relay_candidates c
		JOIN upstream_relay_connectors rc ON rc.id = c.connector_id AND rc.deleted_at IS NULL
		JOIN accounts a ON a.id = c.account_id
		JOIN groups g ON g.id = c.target_group_id
		JOIN account_groups ag ON ag.account_id = c.account_id AND ag.group_id = c.target_group_id
		LEFT JOIN upstream_relay_probe_results pr ON pr.id = c.last_probe_result_id
		LEFT JOIN upstream_relay_group_rate_snapshots s ON s.connector_id = c.connector_id AND s.upstream_group_id = c.upstream_group_id
	`
}

func scanRelayConnectors(rows *sql.Rows) ([]service.UpstreamRelayConnector, error) {
	items := []service.UpstreamRelayConnector{}
	for rows.Next() {
		var item service.UpstreamRelayConnector
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.AuthMode, &item.BearerTokenEncrypted,
			&item.RefreshTokenEncrypted, &item.LoginEmailEncrypted, &item.CookieEncrypted, &item.UserAgentEncrypted,
			&item.Status, &item.CredentialVersion,
			&item.LastVerifiedAt, &item.LastSyncedAt, &item.LastError, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
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
		if err := rows.Scan(&item.ID, &item.ConnectorID, &item.UpstreamGroupID, &item.Name, &item.Platform,
			&item.Status, &item.DefaultRateMultiplier, &override, &item.FinalRateMultiplier, &item.Source,
			&item.LastSeenAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.OverrideRateMultiplier = nullableFloat64Ptr(override)
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
		var snapshotPlatform, snapshotStatus, snapshotSource sql.NullString
		var snapshotSeen sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.ConnectorID, &item.ConnectorName, &item.ConnectorStatus, &item.AccountID,
			&item.AccountName, &item.AccountPlatform, &item.UpstreamGroupID,
			&item.UpstreamGroupName, &item.ProbeModel, &item.ProbeProtocol, &item.TargetGroupID,
			&item.TargetGroupName, &priority, &item.Enabled, &item.Notes,
			&item.LastProbeResultID, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
			&probeID, &probeSuccess, &probeLatency, &probeHTTP, &probeErrorClass,
			&probeErrorMessage, &probeAt,
			&snapshotID, &defaultRate, &overrideRate, &finalRate,
			&snapshotPlatform, &snapshotStatus, &snapshotSource, &snapshotSeen,
		); err != nil {
			return nil, err
		}
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
				Source:                 snapshotSource.String,
				LastSeenAt:             snapshotSeen.Time,
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
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
		       s.target_group_id, COALESCE(g.name, ''), s.old_priority, s.new_priority,
		       s.final_rate_multiplier, s.health_status, s.reason, s.applied, s.applied_by,
		       s.applied_at, s.created_at
		FROM upstream_relay_recommendation_suggestions s
		LEFT JOIN upstream_relay_connectors rc ON rc.id = s.connector_id
		LEFT JOIN accounts a ON a.id = s.account_id
		LEFT JOIN groups g ON g.id = s.target_group_id
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
			&item.TargetGroupID, &item.TargetGroupName, &oldPriority, &item.NewPriority,
			&item.FinalRateMultiplier, &item.HealthStatus, &item.Reason, &item.Applied, &appliedBy,
			&item.AppliedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.OldPriority = nullableIntFromSQL(oldPriority)
		item.AppliedBy = nullInt64Ptr(appliedBy)
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableIntFromSQL(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int64)
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
	if strings.Contains(msg, "upstream_relay_candidates") && strings.Contains(msg, "account_groups") {
		return infraerrors.BadRequest("UPSTREAM_RELAY_CANDIDATE_ACCOUNT_GROUP_MISSING", "candidate account must belong to target group")
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
