package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type conversationCaptureRepository struct {
	db *sql.DB
}

func NewConversationCaptureRepository(db *sql.DB) service.ConversationRepository {
	return &conversationCaptureRepository{db: db}
}

func (r *conversationCaptureRepository) UpsertTurn(ctx context.Context, record service.ConversationTurnRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
INSERT INTO conversation_sessions (
    session_id, user_id, api_key_id, account_id, provider, model, request_path, status,
    turn_count, source_request_count, input_tokens, output_tokens, total_tokens, actual_cost,
    quality_status, exportable, capture_status, session_source, retention_until, started_at, ended_at, created_at, updated_at
) VALUES (
    $1,$2,$3,NULLIF($4,0),$5,$6,$7,'active',
    0,0,0,0,0,0,
    'unchecked',FALSE,'captured',$8,$9,$10,$10,$10,$10
) ON CONFLICT (session_id) DO NOTHING`, record.SessionID, record.UserID, record.APIKeyID, record.AccountID, record.Provider, record.Model, record.RequestPath, record.SessionSource, record.RetentionUntil, record.CreatedAt)
	if err != nil {
		return err
	}

	var currentTurnCount int
	err = tx.QueryRowContext(ctx, `SELECT turn_count FROM conversation_sessions WHERE session_id = $1 FOR UPDATE`, record.SessionID).Scan(&currentTurnCount)
	if err != nil {
		return err
	}

	record.TurnIndex = currentTurnCount + 1
	reqMessages, _ := json.Marshal(record.RequestMessages)
	respMessages, _ := json.Marshal(record.ResponseMessages)
	tools, _ := json.Marshal(record.Tools)
	usage, _ := json.Marshal(record.Usage)
	meta, _ := json.Marshal(record.Meta)
	qualityErrors, _ := json.Marshal(record.QualityErrors)
	insertResult, err := tx.ExecContext(ctx, `
INSERT INTO conversation_turns (
    session_id, request_id, upstream_request_id, client_request_id, turn_index, provider, model, request_path,
    request_messages, response_messages, tools, usage, meta, input_tokens, output_tokens, total_tokens,
    actual_cost, stream, client_disconnect, truncated, quality_status, quality_errors, exportable, parse_status, parse_error,
    dedupe_hash, raw_archive_key, payload_preview, retention_until, created_at
) VALUES (
    $1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,
    $9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13::jsonb,$14,$15,$16,
    $17,$18,$19,$20,$21,$22::jsonb,$23,$24,NULLIF($25,''),
    $26,NULLIF($27,''),$28,$29,$30
) ON CONFLICT (session_id, turn_index) DO NOTHING`,
		record.SessionID, record.RequestID, record.UpstreamRequestID, record.ClientRequestID, record.TurnIndex, record.Provider, record.Model, record.RequestPath,
		string(reqMessages), string(respMessages), string(tools), string(usage), string(meta), record.InputTokens, record.OutputTokens, record.TotalTokens,
		record.ActualCost, record.Stream, record.ClientDisconnect, record.Truncated, record.QualityStatus, string(qualityErrors), record.Exportable, record.ParseStatus, record.ParseError,
		record.DedupeHash, record.RawArchiveKey, record.PayloadPreview, record.RetentionUntil, record.CreatedAt)
	if err != nil {
		return err
	}
	insertedRows, err := insertResult.RowsAffected()
	if err != nil {
		return err
	}
	if insertedRows == 0 {
		return fmt.Errorf("conversation turn insert skipped for session_id=%s turn_index=%d", record.SessionID, record.TurnIndex)
	}

	if err := r.recalculateSessionSummaryTx(ctx, tx, record.SessionID, record.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) ListSessions(ctx context.Context, filters service.ConversationSessionFilters, page, pageSize int) ([]service.ConversationSession, int64, error) {
	where, args := buildConversationSessionWhere(filters, "s")
	countSQL := "SELECT COUNT(*) FROM conversation_sessions s " + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT s.id, s.session_id, s.user_id, s.api_key_id, s.account_id, s.provider, s.model, s.request_path, s.status,
       s.turn_count, s.source_request_count, s.input_tokens, s.output_tokens, s.total_tokens, s.actual_cost,
       s.quality_status, s.quality_errors, s.exportable,
       COALESCE((
           SELECT SUM(d.duplicate_count)
           FROM (
               SELECT COUNT(*) - 1 AS duplicate_count
               FROM conversation_turns dt
               WHERE dt.session_id = s.session_id AND dt.dedupe_hash <> ''
               GROUP BY dt.dedupe_hash
               HAVING COUNT(*) > 1
           ) d
       ), 0) AS duplicate_turn_count,
       s.capture_status, s.session_source, s.retention_until,
       s.started_at, s.ended_at, s.created_at, s.updated_at,
       COALESCE(u.email, '') AS user_email
FROM conversation_sessions s
LEFT JOIN users u ON u.id = s.user_id `+where+`
ORDER BY s.started_at DESC, s.id DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []service.ConversationSession{}
	for rows.Next() {
		item, err := scanConversationSession(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *conversationCaptureRepository) GetSessionByID(ctx context.Context, id int64) (*service.ConversationSession, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT s.id, s.session_id, s.user_id, s.api_key_id, s.account_id, s.provider, s.model, s.request_path, s.status,
       s.turn_count, s.source_request_count, s.input_tokens, s.output_tokens, s.total_tokens, s.actual_cost,
       s.quality_status, s.quality_errors, s.exportable,
       COALESCE((
           SELECT SUM(d.duplicate_count)
           FROM (
               SELECT COUNT(*) - 1 AS duplicate_count
               FROM conversation_turns dt
               WHERE dt.session_id = s.session_id AND dt.dedupe_hash <> ''
               GROUP BY dt.dedupe_hash
               HAVING COUNT(*) > 1
           ) d
       ), 0) AS duplicate_turn_count,
       s.capture_status, s.session_source, s.retention_until,
       s.started_at, s.ended_at, s.created_at, s.updated_at,
       COALESCE(u.email, '') AS user_email
FROM conversation_sessions s
LEFT JOIN users u ON u.id = s.user_id
WHERE s.id = $1`, id)
	item, err := scanConversationSession(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *conversationCaptureRepository) ListTurnsBySessionID(ctx context.Context, sessionID string, page, pageSize int) ([]service.ConversationTurnSummary, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`, sessionID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, conversationTurnSummarySelectSQL()+`
WHERE session_id = $1
ORDER BY turn_index ASC
LIMIT $2 OFFSET $3`, sessionID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanConversationTurnSummaries(rows)
	return items, total, err
}

func (r *conversationCaptureRepository) GetTurnByID(ctx context.Context, id int64) (*service.ConversationTurn, error) {
	row := r.db.QueryRowContext(ctx, conversationTurnSelectSQL()+`WHERE id = $1`, id)
	item, err := scanConversationTurn(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *conversationCaptureRepository) SetSessionExportable(ctx context.Context, id int64, exportable bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation_sessions SET exportable = $2, updated_at = NOW() WHERE id = $1`, id, exportable); err != nil {
		return err
	}
	if !exportable {
		_, err = tx.ExecContext(ctx, `UPDATE conversation_turns SET exportable = FALSE WHERE session_id = $1`, sessionID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `
UPDATE conversation_turns
SET exportable = $2
WHERE session_id = $1
  AND parse_status = 'success'
  AND truncated = FALSE
  AND client_disconnect = FALSE
  AND quality_status = 'clean'`, sessionID, exportable)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) SetTurnExportable(ctx context.Context, id int64, exportable bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation_turns SET exportable = $2 WHERE id = $1`, id, exportable); err != nil {
		return err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, sessionID, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) SetSessionQuality(ctx context.Context, id int64, status string, qualityErrors []service.QualityError, exportable *bool) error {
	errorsRaw, err := json.Marshal(qualityErrors)
	if err != nil {
		return err
	}
	nextExportable := exportable
	if status == service.ConversationQualityStatusRejected {
		value := false
		nextExportable = &value
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); err != nil {
		return err
	}
	if nextExportable == nil {
		if _, err := tx.ExecContext(ctx, `
UPDATE conversation_sessions
SET quality_status = $2, quality_errors = $3::jsonb, updated_at = NOW()
WHERE id = $1`, id, status, string(errorsRaw)); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
UPDATE conversation_sessions
SET quality_status = $2, quality_errors = $3::jsonb, exportable = $4, updated_at = NOW()
WHERE id = $1`, id, status, string(errorsRaw), *nextExportable); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
UPDATE conversation_turns
SET exportable = $2
WHERE session_id = $1
  AND parse_status = 'success'
  AND truncated = FALSE
  AND client_disconnect = FALSE
  AND quality_status = 'clean'`, sessionID, *nextExportable)
		if err != nil {
			return err
		}
	}
	if status == service.ConversationQualityStatusRejected {
		if _, err := tx.ExecContext(ctx, `UPDATE conversation_turns SET exportable = FALSE WHERE session_id = $1`, sessionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) SetTurnQuality(ctx context.Context, id int64, status string, qualityErrors []service.QualityError, exportable *bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	sessionID, err := r.setTurnQualityTx(ctx, tx, id, status, qualityErrors, exportable)
	if err != nil {
		return err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, sessionID, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) BulkSetQuality(ctx context.Context, req service.ConversationBulkQualityUpdateRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	affectedSessions := make([]string, 0, len(req.TurnIDs))
	seenAffectedSessions := make(map[string]struct{}, len(req.TurnIDs))
	for _, id := range req.TurnIDs {
		sessionID, err := r.setTurnQualityTx(ctx, tx, id, req.QualityStatus, req.QualityErrors, req.Exportable)
		if err != nil {
			return err
		}
		if _, ok := seenAffectedSessions[sessionID]; ok {
			continue
		}
		seenAffectedSessions[sessionID] = struct{}{}
		affectedSessions = append(affectedSessions, sessionID)
	}
	now := time.Now()
	for _, sessionID := range affectedSessions {
		if err := r.recalculateSessionSummaryTx(ctx, tx, sessionID, now); err != nil {
			return err
		}
	}
	for _, id := range req.SessionIDs {
		if err := r.setSessionQualityTx(ctx, tx, id, req.QualityStatus, req.QualityErrors, req.Exportable); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) MergeSessions(ctx context.Context, targetID int64, sourceIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var targetSessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, targetID).Scan(&targetSessionID); err != nil {
		return err
	}
	var nextIndex int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(turn_index), 0) FROM conversation_turns WHERE session_id = $1`, targetSessionID).Scan(&nextIndex); err != nil {
		return err
	}
	for _, sourceID := range sourceIDs {
		if sourceID == targetID {
			continue
		}
		var sourceSessionID string
		if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, sourceID).Scan(&sourceSessionID); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `
SELECT id
FROM conversation_turns
WHERE session_id = $1
ORDER BY turn_index ASC, created_at ASC, id ASC
FOR UPDATE`, sourceSessionID)
		if err != nil {
			return err
		}
		var turnIDs []int64
		for rows.Next() {
			var turnID int64
			if err := rows.Scan(&turnID); err != nil {
				rows.Close()
				return err
			}
			turnIDs = append(turnIDs, turnID)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, turnID := range turnIDs {
			nextIndex++
			if _, err := tx.ExecContext(ctx, `
UPDATE conversation_turns
SET session_id = $2, turn_index = $3
WHERE id = $1`, turnID, targetSessionID, nextIndex); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_sessions WHERE id = $1`, sourceID); err != nil {
			return err
		}
	}
	if err := r.reindexSessionTurnsTx(ctx, tx, targetSessionID); err != nil {
		return err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, targetSessionID, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) SplitSessionFromTurn(ctx context.Context, sessionID int64, turnID int64, newSessionID string, now time.Time) (*service.ConversationSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	source, err := scanConversationSession(tx.QueryRowContext(ctx, `
SELECT s.id, s.session_id, s.user_id, s.api_key_id, s.account_id, s.provider, s.model, s.request_path, s.status,
       s.turn_count, s.source_request_count, s.input_tokens, s.output_tokens, s.total_tokens, s.actual_cost,
       s.quality_status, s.quality_errors, s.exportable, 0::bigint AS duplicate_turn_count, s.capture_status, s.session_source, s.retention_until,
       s.started_at, s.ended_at, s.created_at, s.updated_at,
       COALESCE(u.email, '') AS user_email
FROM conversation_sessions s
LEFT JOIN users u ON u.id = s.user_id
WHERE s.id = $1 FOR UPDATE`, sessionID))
	if err != nil {
		return nil, err
	}
	var splitIndex int
	if err := tx.QueryRowContext(ctx, `
SELECT turn_index
FROM conversation_turns
WHERE id = $1 AND session_id = $2
FOR UPDATE`, turnID, source.SessionID).Scan(&splitIndex); err != nil {
		return nil, err
	}
	qualityErrors, _ := json.Marshal(source.QualityErrors)
	var accountID any
	if source.AccountID != nil {
		accountID = *source.AccountID
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO conversation_sessions (
    session_id, user_id, api_key_id, account_id, provider, model, request_path, status,
    turn_count, source_request_count, input_tokens, output_tokens, total_tokens, actual_cost,
    quality_status, quality_errors, exportable, capture_status, session_source, retention_until,
    started_at, ended_at, created_at, updated_at
) VALUES (
    $1,$2,$3,$4,$5,$6,$7,'active',
    0,0,0,0,0,0,
    $8,$9::jsonb,FALSE,$10,'manual_split',$11,
    $12,$12,$12,$12
)`, newSessionID, source.UserID, source.APIKeyID, accountID, source.Provider, source.Model, source.RequestPath,
		source.QualityStatus, string(qualityErrors), source.CaptureStatus, source.RetentionUntil, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE conversation_turns
SET session_id = $3
WHERE session_id = $1 AND turn_index >= $2`, source.SessionID, splitIndex, newSessionID); err != nil {
		return nil, err
	}
	if err := r.reindexSessionTurnsTx(ctx, tx, source.SessionID); err != nil {
		return nil, err
	}
	if err := r.reindexSessionTurnsTx(ctx, tx, newSessionID); err != nil {
		return nil, err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, source.SessionID, now); err != nil {
		return nil, err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, newSessionID, now); err != nil {
		return nil, err
	}
	item, err := scanConversationSession(tx.QueryRowContext(ctx, `
SELECT s.id, s.session_id, s.user_id, s.api_key_id, s.account_id, s.provider, s.model, s.request_path, s.status,
       s.turn_count, s.source_request_count, s.input_tokens, s.output_tokens, s.total_tokens, s.actual_cost,
       s.quality_status, s.quality_errors, s.exportable, 0::bigint AS duplicate_turn_count, s.capture_status, s.session_source, s.retention_until,
       s.started_at, s.ended_at, s.created_at, s.updated_at,
       COALESCE(u.email, '') AS user_email
FROM conversation_sessions s
LEFT JOIN users u ON u.id = s.user_id
WHERE s.session_id = $1`, newSessionID))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *conversationCaptureRepository) MoveTurn(ctx context.Context, turnID int64, targetSessionID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var target string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, targetSessionID).Scan(&target); err != nil {
		return err
	}
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`, turnID).Scan(&source); err != nil {
		return err
	}
	if source == target {
		return tx.Commit()
	}
	var nextIndex int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(turn_index), 0) + 1 FROM conversation_turns WHERE session_id = $1`, target).Scan(&nextIndex); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE conversation_turns
SET session_id = $2, turn_index = $3
WHERE id = $1`, turnID, target, nextIndex); err != nil {
		return err
	}
	now := time.Now()
	if err := r.reindexSessionTurnsTx(ctx, tx, source); err != nil {
		return err
	}
	if err := r.reindexSessionTurnsTx(ctx, tx, target); err != nil {
		return err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, source, now); err != nil {
		return err
	}
	if err := r.recalculateSessionSummaryTx(ctx, tx, target, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *conversationCaptureRepository) setSessionQualityTx(ctx context.Context, tx *sql.Tx, id int64, status string, qualityErrors []service.QualityError, exportable *bool) error {
	errorsRaw, err := json.Marshal(qualityErrors)
	if err != nil {
		return err
	}
	nextExportable := exportable
	if status == service.ConversationQualityStatusRejected {
		value := false
		nextExportable = &value
	}
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); err != nil {
		return err
	}
	if nextExportable == nil {
		_, err = tx.ExecContext(ctx, `
UPDATE conversation_sessions
SET quality_status = $2, quality_errors = $3::jsonb, updated_at = NOW()
WHERE id = $1`, id, status, string(errorsRaw))
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE conversation_sessions
SET quality_status = $2, quality_errors = $3::jsonb, exportable = $4, updated_at = NOW()
WHERE id = $1`, id, status, string(errorsRaw), *nextExportable); err != nil {
		return err
	}
	if status == service.ConversationQualityStatusRejected {
		_, err = tx.ExecContext(ctx, `UPDATE conversation_turns SET exportable = FALSE WHERE session_id = $1`, sessionID)
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE conversation_turns
SET exportable = $2
WHERE session_id = $1
  AND parse_status = 'success'
  AND truncated = FALSE
  AND client_disconnect = FALSE
  AND quality_status = 'clean'`, sessionID, *nextExportable)
	return err
}

func (r *conversationCaptureRepository) setTurnQualityTx(ctx context.Context, tx *sql.Tx, id int64, status string, qualityErrors []service.QualityError, exportable *bool) (string, error) {
	errorsRaw, err := json.Marshal(qualityErrors)
	if err != nil {
		return "", err
	}
	nextExportable := exportable
	if status == service.ConversationQualityStatusRejected || status == service.ConversationQualityStatusNeedsReview {
		value := false
		nextExportable = &value
	}
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); err != nil {
		return "", err
	}
	if nextExportable == nil {
		_, err = tx.ExecContext(ctx, `
UPDATE conversation_turns
SET quality_status = $2, quality_errors = $3::jsonb
WHERE id = $1`, id, status, string(errorsRaw))
		return sessionID, err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE conversation_turns
SET quality_status = $2, quality_errors = $3::jsonb, exportable = $4
WHERE id = $1`, id, status, string(errorsRaw), *nextExportable)
	return sessionID, err
}

func (r *conversationCaptureRepository) reindexSessionTurnsTx(ctx context.Context, tx *sql.Tx, sessionID string) error {
	if _, err := tx.ExecContext(ctx, `
WITH ranked AS (
    SELECT id, (-ROW_NUMBER() OVER (ORDER BY turn_index ASC, created_at ASC, id ASC))::int AS new_index
    FROM conversation_turns
    WHERE session_id = $1
)
UPDATE conversation_turns t
SET turn_index = ranked.new_index
FROM ranked
WHERE t.id = ranked.id`, sessionID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY turn_index DESC, created_at ASC, id ASC)::int AS new_index
    FROM conversation_turns
    WHERE session_id = $1
)
UPDATE conversation_turns t
SET turn_index = ranked.new_index
FROM ranked
WHERE t.id = ranked.id`, sessionID)
	return err
}

func (r *conversationCaptureRepository) recalculateSessionSummaryTx(ctx context.Context, tx *sql.Tx, sessionID string, now time.Time) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`, sessionID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM conversation_sessions WHERE session_id = $1`, sessionID)
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE conversation_sessions s
SET turn_count = agg.turn_count,
    source_request_count = agg.source_request_count,
    input_tokens = agg.input_tokens,
    output_tokens = agg.output_tokens,
    total_tokens = agg.total_tokens,
    actual_cost = agg.actual_cost,
    quality_status = CASE
        WHEN agg.rejected_count > 0 THEN 'rejected'
        WHEN agg.review_count > 0 OR agg.unchecked_count > 0 THEN 'needs_review'
        ELSE 'clean'
    END,
    quality_errors = COALESCE(errors.items, '[]'::jsonb),
    exportable = CASE
        WHEN agg.turn_count > 0
          AND agg.rejected_count = 0
          AND agg.review_count = 0
          AND agg.unchecked_count = 0
          AND agg.non_exportable_count = 0
        THEN TRUE
        ELSE FALSE
    END,
    started_at = agg.started_at,
    ended_at = agg.ended_at,
    retention_until = agg.retention_until,
    updated_at = $2,
    capture_status = CASE
        WHEN agg.truncated_count > 0 THEN 'truncated'
        WHEN agg.failed_count > 0 THEN 'parse_failed'
        ELSE 'captured'
    END
FROM (
    SELECT session_id,
           COUNT(*)::int AS turn_count,
           COUNT(*)::int AS source_request_count,
           COALESCE(SUM(input_tokens), 0)::bigint AS input_tokens,
           COALESCE(SUM(output_tokens), 0)::bigint AS output_tokens,
           COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
           COALESCE(SUM(actual_cost), 0)::numeric AS actual_cost,
           MIN(created_at) AS started_at,
           MAX(created_at) AS ended_at,
           MAX(retention_until) AS retention_until,
           COUNT(*) FILTER (WHERE truncated) AS truncated_count,
           COUNT(*) FILTER (WHERE parse_status <> 'success') AS failed_count,
           COUNT(*) FILTER (WHERE quality_status = 'rejected') AS rejected_count,
           COUNT(*) FILTER (WHERE quality_status = 'needs_review') AS review_count,
           COUNT(*) FILTER (WHERE quality_status IS NULL OR quality_status NOT IN ('clean','needs_review','rejected')) AS unchecked_count,
           COUNT(*) FILTER (WHERE exportable = FALSE) AS non_exportable_count
    FROM conversation_turns
    WHERE session_id = $1
    GROUP BY session_id
) agg
LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('code', code, 'message', message, 'source', source) ORDER BY code, message, source) AS items
    FROM (
        SELECT DISTINCT
               COALESCE(err.item->>'code', '') AS code,
               COALESCE(err.item->>'message', '') AS message,
               COALESCE(err.item->>'source', '') AS source
        FROM conversation_turns et
        CROSS JOIN LATERAL jsonb_array_elements(COALESCE(et.quality_errors, '[]'::jsonb)) AS err(item)
        WHERE et.session_id = agg.session_id
          AND COALESCE(err.item->>'code', '') <> ''
    ) deduped_errors
) errors ON TRUE
WHERE s.session_id = agg.session_id`, sessionID, now)
	return err
}

func (r *conversationCaptureRepository) ListExportableTurns(ctx context.Context, req service.ConversationExportMessagesJSONLRequest) ([]service.ConversationTurn, error) {
	sqlText, args := buildConversationExportTurnsSQL(req)
	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConversationTurns(rows)
}

func (r *conversationCaptureRepository) FindSessionIDByResponseID(ctx context.Context, responseID string) (string, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return "", sql.ErrNoRows
	}
	var sessionID string
	err := r.db.QueryRowContext(ctx, `
SELECT session_id
FROM conversation_turns
WHERE meta->>'response_id' = $1
ORDER BY created_at DESC, id DESC
LIMIT 1`, responseID).Scan(&sessionID)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (r *conversationCaptureRepository) CreateExportJob(ctx context.Context, job service.ConversationExportJob) (*service.ConversationExportJob, error) {
	filters, err := json.Marshal(job.Filters)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO conversation_export_jobs (
    status, filters, format, encoding, session_count, turn_count, file_size,
    s3_key, download_url_expires_at, expires_at, error_message, created_by,
    created_at, updated_at, started_at, completed_at
) VALUES (
    $1,$2::jsonb,$3,$4,$5,$6,$7,
    NULLIF($8,''),$9,$10,NULLIF($11,''),$12,
    $13,$13,$14,$15
)
RETURNING id, status, filters, format, encoding, session_count, turn_count, file_size,
          s3_key, download_url_expires_at, expires_at, error_message, created_by,
          created_at, updated_at, started_at, completed_at`,
		job.Status, string(filters), job.Format, job.Encoding, job.SessionCount, job.TurnCount, job.FileSize,
		stringPtrValue(job.S3Key), job.DownloadURLExpiresAt, job.ExpiresAt, stringPtrValue(job.ErrorMessage), job.CreatedBy,
		job.CreatedAt, job.StartedAt, job.CompletedAt)
	return scanConversationExportJob(row)
}

func (r *conversationCaptureRepository) ListExportJobs(ctx context.Context, page, pageSize int) ([]service.ConversationExportJob, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_export_jobs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, conversationExportJobSelectSQL()+`
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []service.ConversationExportJob{}
	for rows.Next() {
		item, err := scanConversationExportJob(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *conversationCaptureRepository) GetExportJobByID(ctx context.Context, id int64) (*service.ConversationExportJob, error) {
	return scanConversationExportJob(r.db.QueryRowContext(ctx, conversationExportJobSelectSQL()+`WHERE id = $1`, id))
}

func (r *conversationCaptureRepository) MarkExportJobRunning(ctx context.Context, id int64, startedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET status = 'running', started_at = $2, updated_at = $2
WHERE id = $1 AND status = 'pending'`, id, startedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, "conversation export job is not pending")
}

func (r *conversationCaptureRepository) CompleteExportJob(ctx context.Context, id int64, sessionCount, turnCount, fileSize int64, s3Key string, completedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET status = 'completed',
    session_count = $2,
    turn_count = $3,
    file_size = $4,
    s3_key = NULLIF($5,''),
    completed_at = $6,
    updated_at = $6,
    error_message = NULL
WHERE id = $1 AND status = 'running'`, id, sessionCount, turnCount, fileSize, s3Key, completedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, "conversation export job is not running")
}

func (r *conversationCaptureRepository) FailExportJob(ctx context.Context, id int64, errorMessage string, failedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET status = 'failed', error_message = NULLIF($2,''), completed_at = $3, updated_at = $3
WHERE id = $1 AND status IN ('pending','running')`, id, errorMessage, failedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, "conversation export job cannot fail from current status")
}

func (r *conversationCaptureRepository) SetExportJobDownloadURLExpiresAt(ctx context.Context, id int64, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET download_url_expires_at = $2, updated_at = NOW()
WHERE id = $1`, id, expiresAt)
	return err
}

func (r *conversationCaptureRepository) MarkExportJobDeleted(ctx context.Context, id int64, deletedAt time.Time) (*service.ConversationExportJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanConversationExportJob(tx.QueryRowContext(ctx, conversationExportJobSelectSQL()+`WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if job.Status == service.ConversationExportJobStatusRunning {
		return nil, fmt.Errorf("conversation export job is running")
	}
	result, err := tx.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET status = 'deleted', updated_at = $2
WHERE id = $1 AND status <> 'running'`, id, deletedAt)
	if err != nil {
		return nil, err
	}
	if err := requireRowsAffected(result, "conversation export job is running"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *conversationCaptureRepository) MarkExpiredExportJobs(ctx context.Context, now time.Time, limit int) ([]service.ConversationExportJob, error) {
	rows, err := r.db.QueryContext(ctx, conversationExportJobSelectSQL()+`
WHERE id IN (
    SELECT id
    FROM conversation_export_jobs
    WHERE expires_at < $1
      AND status NOT IN ('expired','deleted','running')
    ORDER BY expires_at ASC
    LIMIT $2
)
ORDER BY expires_at ASC`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []service.ConversationExportJob
	for rows.Next() {
		job, err := scanConversationExportJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return jobs, nil
	}
	return jobs, nil
}

func (r *conversationCaptureRepository) MarkExportJobsExpired(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE conversation_export_jobs
SET status = 'expired', updated_at = $2
WHERE id = ANY($1)
  AND status NOT IN ('expired','deleted','running')`, pq.Array(ids), now)
	return err
}

func buildConversationExportTurnsSQL(req service.ConversationExportMessagesJSONLRequest) (string, []any) {
	where, args := buildConversationSessionWhere(req.ConversationSessionFilters, "s")
	clauses := []string{"t.exportable = TRUE", "t.parse_status = 'success'", "t.truncated = FALSE", "t.client_disconnect = FALSE"}
	if !req.IncludeHeuristic {
		clauses = append(clauses, "s.session_source <> 'heuristic'")
	}
	if where != "" {
		clauses = append(clauses, strings.TrimPrefix(where, "WHERE "))
	}
	sqlText := conversationTurnSelectSQL() + `
JOIN conversation_sessions s ON s.session_id = t.session_id
WHERE ` + strings.Join(clauses, " AND ") + `
ORDER BY t.created_at DESC, t.id DESC
LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, req.Limit)
	return sqlText, args
}

func (r *conversationCaptureRepository) CleanupExpired(ctx context.Context, now time.Time, limit int) (int64, int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	sessionRows, err := tx.QueryContext(ctx, `
	SELECT session_id
	FROM conversation_sessions
	WHERE retention_until < $1
	ORDER BY retention_until ASC
	LIMIT $2`, now, limit)
	if err != nil {
		return 0, 0, err
	}
	sessionIDs := []string{}
	for sessionRows.Next() {
		var sessionID string
		if err := sessionRows.Scan(&sessionID); err != nil {
			sessionRows.Close()
			return 0, 0, err
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	if err := sessionRows.Close(); err != nil {
		return 0, 0, err
	}
	if err := sessionRows.Err(); err != nil {
		return 0, 0, err
	}
	if len(sessionIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}
	turnResult, err := tx.ExecContext(ctx, `
	DELETE FROM conversation_turns
	WHERE session_id = ANY($1)`, pq.Array(sessionIDs))
	if err != nil {
		return 0, 0, err
	}
	sessionResult, err := tx.ExecContext(ctx, `
	DELETE FROM conversation_sessions
	WHERE session_id = ANY($1)`, pq.Array(sessionIDs))
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	turns, _ := turnResult.RowsAffected()
	sessions, _ := sessionResult.RowsAffected()
	return sessions, turns, nil
}

func buildConversationSessionWhere(filters service.ConversationSessionFilters, alias string) (string, []any) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	clauses := []string{}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filters.UserID > 0 {
		add(prefix+"user_id = $%d", filters.UserID)
	}
	if filters.APIKeyID > 0 {
		add(prefix+"api_key_id = $%d", filters.APIKeyID)
	}
	if strings.TrimSpace(filters.Model) != "" {
		add(prefix+"model ILIKE $%d", "%"+strings.TrimSpace(filters.Model)+"%")
	}
	if strings.TrimSpace(filters.QualityStatus) != "" {
		add(prefix+"quality_status = $%d", strings.TrimSpace(filters.QualityStatus))
	}
	if filters.Exportable != nil {
		add(prefix+"exportable = $%d", *filters.Exportable)
	}
	if filters.StartedAtFrom != nil {
		add(prefix+"started_at >= $%d", *filters.StartedAtFrom)
	}
	if filters.StartedAtTo != nil {
		add(prefix+"started_at <= $%d", *filters.StartedAtTo)
	}
	if strings.TrimSpace(filters.RequestID) != "" {
		args = append(args, "%"+strings.TrimSpace(filters.RequestID)+"%")
		n := len(args)
		clauses = append(clauses, fmt.Sprintf(`EXISTS (
SELECT 1 FROM conversation_turns rt
WHERE rt.session_id = %ssession_id
  AND (rt.request_id ILIKE $%d OR COALESCE(rt.client_request_id,'') ILIKE $%d)
)`, prefix, n, n))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanConversationSession(row rowScanner) (service.ConversationSession, error) {
	var item service.ConversationSession
	var accountID sql.NullInt64
	var qualityErrors []byte
	err := row.Scan(
		&item.ID, &item.SessionID, &item.UserID, &item.APIKeyID, &accountID, &item.Provider, &item.Model, &item.RequestPath, &item.Status,
		&item.TurnCount, &item.SourceRequestCount, &item.InputTokens, &item.OutputTokens, &item.TotalTokens, &item.ActualCost,
		&item.QualityStatus, &qualityErrors, &item.Exportable, &item.DuplicateTurnCount, &item.CaptureStatus, &item.SessionSource, &item.RetentionUntil,
		&item.StartedAt, &item.EndedAt, &item.CreatedAt, &item.UpdatedAt, &item.UserEmail,
	)
	if accountID.Valid {
		item.AccountID = &accountID.Int64
	}
	_ = json.Unmarshal(qualityErrors, &item.QualityErrors)
	if item.QualityErrors == nil {
		item.QualityErrors = []service.QualityError{}
	}
	return item, err
}

func conversationTurnSelectSQL() string {
	return `
SELECT t.id, t.session_id, t.request_id, t.upstream_request_id, t.client_request_id, t.turn_index,
       t.provider, t.model, t.request_path, t.request_messages, t.response_messages, t.tools,
       t.usage, t.meta, t.input_tokens, t.output_tokens, t.total_tokens, t.actual_cost,
       t.stream, t.client_disconnect, t.truncated, t.quality_status, t.quality_errors, t.exportable,
       t.parse_status, t.parse_error, t.dedupe_hash, t.raw_archive_key, t.payload_preview, t.retention_until, t.created_at
FROM conversation_turns t `
}

func conversationTurnSummarySelectSQL() string {
	return `
SELECT t.id, t.session_id, t.request_id, t.upstream_request_id, t.client_request_id, t.turn_index,
       t.provider, t.model, t.request_path, t.input_tokens, t.output_tokens, t.total_tokens, t.actual_cost,
       t.stream, t.client_disconnect, t.truncated, t.quality_status, t.quality_errors, t.exportable,
       t.parse_status, t.parse_error, t.dedupe_hash,
       CASE WHEN t.dedupe_hash = '' THEN 0 ELSE GREATEST(COUNT(*) OVER (PARTITION BY t.dedupe_hash) - 1, 0) END AS duplicate_count,
       t.payload_preview, t.retention_until, t.created_at
FROM conversation_turns t `
}

func scanConversationTurns(rows *sql.Rows) ([]service.ConversationTurn, error) {
	items := []service.ConversationTurn{}
	for rows.Next() {
		item, err := scanConversationTurn(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanConversationTurn(row rowScanner) (service.ConversationTurn, error) {
	var item service.ConversationTurn
	var upstreamRequestID, clientRequestID, parseError, rawArchiveKey, preview sql.NullString
	var reqMessages, respMessages, tools, usage, meta, qualityErrors []byte
	err := row.Scan(
		&item.ID, &item.SessionID, &item.RequestID, &upstreamRequestID, &clientRequestID, &item.TurnIndex,
		&item.Provider, &item.Model, &item.RequestPath, &reqMessages, &respMessages, &tools,
		&usage, &meta, &item.InputTokens, &item.OutputTokens, &item.TotalTokens, &item.ActualCost,
		&item.Stream, &item.ClientDisconnect, &item.Truncated, &item.QualityStatus, &qualityErrors, &item.Exportable,
		&item.ParseStatus, &parseError, &item.DedupeHash, &rawArchiveKey, &preview, &item.RetentionUntil, &item.CreatedAt,
	)
	if err != nil {
		return item, err
	}
	if upstreamRequestID.Valid {
		item.UpstreamRequestID = &upstreamRequestID.String
	}
	if clientRequestID.Valid {
		item.ClientRequestID = &clientRequestID.String
	}
	if parseError.Valid {
		item.ParseError = &parseError.String
	}
	if rawArchiveKey.Valid {
		item.RawArchiveKey = &rawArchiveKey.String
	}
	if preview.Valid {
		item.PayloadPreview = &preview.String
	}
	_ = json.Unmarshal(qualityErrors, &item.QualityErrors)
	_ = json.Unmarshal(reqMessages, &item.RequestMessages)
	_ = json.Unmarshal(respMessages, &item.ResponseMessages)
	_ = json.Unmarshal(tools, &item.Tools)
	_ = json.Unmarshal(usage, &item.Usage)
	_ = json.Unmarshal(meta, &item.Meta)
	if item.Usage == nil {
		item.Usage = map[string]any{}
	}
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	if item.QualityErrors == nil {
		item.QualityErrors = []service.QualityError{}
	}
	return item, nil
}

func scanConversationTurnSummaries(rows *sql.Rows) ([]service.ConversationTurnSummary, error) {
	items := []service.ConversationTurnSummary{}
	for rows.Next() {
		item, err := scanConversationTurnSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanConversationTurnSummary(row rowScanner) (service.ConversationTurnSummary, error) {
	var item service.ConversationTurnSummary
	var upstreamRequestID, clientRequestID, parseError, preview sql.NullString
	var qualityErrors []byte
	err := row.Scan(
		&item.ID, &item.SessionID, &item.RequestID, &upstreamRequestID, &clientRequestID, &item.TurnIndex,
		&item.Provider, &item.Model, &item.RequestPath, &item.InputTokens, &item.OutputTokens, &item.TotalTokens, &item.ActualCost,
		&item.Stream, &item.ClientDisconnect, &item.Truncated, &item.QualityStatus, &qualityErrors, &item.Exportable,
		&item.ParseStatus, &parseError, &item.DedupeHash, &item.DuplicateCount, &preview, &item.RetentionUntil, &item.CreatedAt,
	)
	if err != nil {
		return item, err
	}
	if upstreamRequestID.Valid {
		item.UpstreamRequestID = &upstreamRequestID.String
	}
	if clientRequestID.Valid {
		item.ClientRequestID = &clientRequestID.String
	}
	if parseError.Valid {
		item.ParseError = &parseError.String
	}
	if preview.Valid {
		item.PayloadPreview = &preview.String
	}
	_ = json.Unmarshal(qualityErrors, &item.QualityErrors)
	if item.QualityErrors == nil {
		item.QualityErrors = []service.QualityError{}
	}
	return item, nil
}

func conversationExportJobSelectSQL() string {
	return `
SELECT id, status, filters, format, encoding, session_count, turn_count, file_size,
       s3_key, download_url_expires_at, expires_at, error_message, created_by,
       created_at, updated_at, started_at, completed_at
FROM conversation_export_jobs `
}

func scanConversationExportJob(row rowScanner) (*service.ConversationExportJob, error) {
	var item service.ConversationExportJob
	var filters []byte
	var s3Key, errorMessage sql.NullString
	var downloadURLExpiresAt, startedAt, completedAt sql.NullTime
	err := row.Scan(
		&item.ID, &item.Status, &filters, &item.Format, &item.Encoding, &item.SessionCount, &item.TurnCount, &item.FileSize,
		&s3Key, &downloadURLExpiresAt, &item.ExpiresAt, &errorMessage, &item.CreatedBy,
		&item.CreatedAt, &item.UpdatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(filters) > 0 {
		_ = json.Unmarshal(filters, &item.Filters)
	}
	if s3Key.Valid {
		item.S3Key = &s3Key.String
	}
	if downloadURLExpiresAt.Valid {
		item.DownloadURLExpiresAt = &downloadURLExpiresAt.Time
	}
	if errorMessage.Valid {
		item.ErrorMessage = &errorMessage.String
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Time
	}
	return &item, nil
}

func stringPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func requireRowsAffected(result sql.Result, message string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%s", message)
	}
	return nil
}
