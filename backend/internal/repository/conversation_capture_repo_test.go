package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestConversationCaptureRepositorySetSessionExportableUpdatesEligibleTurns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE conversation_sessions SET exportable = $2, updated_at = NOW() WHERE id = $1`)).
		WithArgs(int64(9), true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("quality_status = 'clean'").
		WithArgs("s1", true).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	require.NoError(t, repo.SetSessionExportable(context.Background(), 9, true))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositorySetSessionExportableFalseClearsAllTurns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE conversation_sessions SET exportable = $2, updated_at = NOW() WHERE id = $1`)).
		WithArgs(int64(9), false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE conversation_turns SET exportable = FALSE WHERE session_id = $1`)).
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	require.NoError(t, repo.SetSessionExportable(context.Background(), 9, false))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositorySetTurnExportableRecalculatesSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE conversation_turns SET exportable = $2 WHERE id = $1`)).
		WithArgs(int64(7), true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectConversationSessionSummaryRecalculation(mock, "s1")
	mock.ExpectCommit()

	require.NoError(t, repo.SetTurnExportable(context.Background(), 7, true))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositorySetTurnQualityRecalculatesSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	exportable := true
	qualityErrors := []service.QualityError{{Code: "manual_review", Message: "verified", Source: "admin"}}
	errorsRaw, err := json.Marshal(qualityErrors)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec("UPDATE conversation_turns").
		WithArgs(int64(7), service.ConversationQualityStatusClean, string(errorsRaw), exportable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectConversationSessionSummaryRecalculation(mock, "s1")
	mock.ExpectCommit()

	require.NoError(t, repo.SetTurnQuality(context.Background(), 7, service.ConversationQualityStatusClean, qualityErrors, &exportable))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryBulkSetQualityRecalculatesAffectedTurnSessionsOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	exportable := true
	qualityErrors := []service.QualityError{{Code: "manual_clean", Message: "approved"}}
	errorsRaw, err := json.Marshal(qualityErrors)
	require.NoError(t, err)

	mock.ExpectBegin()
	for _, turnID := range []int64{7, 8} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`)).
			WithArgs(turnID).
			WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
		mock.ExpectExec("UPDATE conversation_turns").
			WithArgs(turnID, service.ConversationQualityStatusClean, string(errorsRaw), exportable).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s2"))
	mock.ExpectExec("UPDATE conversation_turns").
		WithArgs(int64(9), service.ConversationQualityStatusClean, string(errorsRaw), exportable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectConversationSessionSummaryRecalculation(mock, "s1")
	expectConversationSessionSummaryRecalculation(mock, "s2")
	mock.ExpectCommit()

	err = repo.BulkSetQuality(context.Background(), service.ConversationBulkQualityUpdateRequest{
		TurnIDs:       []int64{7, 8, 9},
		QualityStatus: service.ConversationQualityStatusClean,
		QualityErrors: qualityErrors,
		Exportable:    &exportable,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryBulkSetQualityKeepsSessionLevelUpdateLast(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	exportable := true
	qualityErrors := []service.QualityError{{Code: "manual_clean", Message: "approved"}}
	errorsRaw, err := json.Marshal(qualityErrors)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_turns WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec("UPDATE conversation_turns").
		WithArgs(int64(7), service.ConversationQualityStatusClean, string(errorsRaw), exportable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectConversationSessionSummaryRecalculation(mock, "s1")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT session_id FROM conversation_sessions WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectExec("UPDATE conversation_sessions").
		WithArgs(int64(5), service.ConversationQualityStatusClean, string(errorsRaw), exportable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE conversation_turns").
		WithArgs("s1", exportable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.BulkSetQuality(context.Background(), service.ConversationBulkQualityUpdateRequest{
		SessionIDs:    []int64{5},
		TurnIDs:       []int64{7},
		QualityStatus: service.ConversationQualityStatusClean,
		QualityErrors: qualityErrors,
		Exportable:    &exportable,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryListExportableTurnsUsesTurnExportableHardGate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)

	rows := sqlmock.NewRows([]string{
		"id", "session_id", "request_id", "upstream_request_id", "client_request_id", "turn_index",
		"provider", "model", "request_path", "request_messages", "response_messages", "tools",
		"usage", "meta", "input_tokens", "output_tokens", "total_tokens", "actual_cost",
		"stream", "client_disconnect", "truncated", "quality_status", "quality_errors", "exportable",
		"parse_status", "parse_error", "dedupe_hash", "raw_archive_key", "payload_preview", "retention_until", "created_at",
	})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT t.id, t.session_id, t.request_id")).
		WithArgs(10).
		WillReturnRows(rows)

	_, err = repo.ListExportableTurns(context.Background(), service.ConversationExportMessagesJSONLRequest{Limit: 10})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryListExportableTurnsSQLHardGate(t *testing.T) {
	sqlText, _ := buildConversationExportTurnsSQL(service.ConversationExportMessagesJSONLRequest{Limit: 10})

	require.Contains(t, sqlText, "t.exportable = TRUE")
	require.NotContains(t, sqlText, "OR s.exportable")
}

func TestConversationCaptureRepositoryListExportableTurnsCleanFilterStillUsesTurnHardGate(t *testing.T) {
	sqlText, args := buildConversationExportTurnsSQL(service.ConversationExportMessagesJSONLRequest{
		ConversationSessionFilters: service.ConversationSessionFilters{
			QualityStatus: service.ConversationQualityStatusClean,
		},
		Limit: 10,
	})

	require.Contains(t, sqlText, "t.exportable = TRUE")
	require.Contains(t, sqlText, "t.parse_status = 'success'")
	require.Contains(t, sqlText, "t.truncated = FALSE")
	require.Contains(t, sqlText, "t.client_disconnect = FALSE")
	require.Contains(t, sqlText, "s.quality_status = $1")
	require.NotContains(t, sqlText, "OR s.exportable")
	require.Equal(t, []any{service.ConversationQualityStatusClean, 10}, args)
}

func TestConversationCaptureRepositoryListTurnsBySessionIDUsesSummaryColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`)).
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT t.id, t.session_id, t.request_id")).
		WithArgs("s1", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "request_id", "upstream_request_id", "client_request_id", "turn_index",
			"provider", "model", "request_path", "input_tokens", "output_tokens", "total_tokens", "actual_cost",
			"stream", "client_disconnect", "truncated", "quality_status", "quality_errors", "exportable",
			"parse_status", "parse_error", "dedupe_hash", "duplicate_count", "payload_preview", "retention_until", "created_at",
		}).AddRow(
			int64(1), "s1", "req-1", nil, nil, 1,
			"openai", "gpt-5", "/v1/chat/completions", int64(1), int64(2), int64(3), float64(0.01),
			false, false, false, "unchecked", []byte(`[]`), true,
			"success", nil, "hash", int64(0), "preview", now, now,
		))

	items, total, err := repo.ListTurnsBySessionID(context.Background(), "s1", 1, 20)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "preview", *items[0].PayloadPreview)
	require.NoError(t, mock.ExpectationsWereMet())

	summarySQL := conversationTurnSummarySelectSQL()
	require.NotContains(t, summarySQL, "request_messages")
	require.NotContains(t, summarySQL, "response_messages")
	require.NotContains(t, summarySQL, "tools")
	require.NotContains(t, summarySQL, "usage,")
	require.NotContains(t, summarySQL, "meta,")
}

func TestConversationCaptureRepositoryListSessionsQualityStatusFilterUsesSessionStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM conversation_sessions s WHERE s.quality_status = $1`)).
		WithArgs(service.ConversationQualityStatusClean).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id, s.session_id")).
		WithArgs(service.ConversationQualityStatusClean, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "user_id", "api_key_id", "account_id", "provider", "model", "request_path", "status",
			"turn_count", "source_request_count", "input_tokens", "output_tokens", "total_tokens", "actual_cost",
			"quality_status", "quality_errors", "exportable", "duplicate_turn_count", "capture_status", "session_source", "retention_until",
			"started_at", "ended_at", "created_at", "updated_at", "user_email",
		}).AddRow(
			int64(1), "s1", int64(1), int64(2), nil, service.ConversationProviderOpenAI, "gpt-5", "/v1/chat/completions", "active",
			1, 1, int64(1), int64(2), int64(3), float64(0.01),
			service.ConversationQualityStatusClean, []byte(`[]`), true, int64(0), "captured", service.ConversationSessionSourceExplicit, now,
			now, now, now, now, "user@example.com",
		))

	items, total, err := repo.ListSessions(context.Background(), service.ConversationSessionFilters{
		QualityStatus: service.ConversationQualityStatusClean,
	}, 1, 20)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, service.ConversationQualityStatusClean, items[0].QualityStatus)
	require.True(t, items[0].Exportable)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryUpsertTurnCreatesSessionBeforeLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()
	record := conversationCaptureRepoTestTurnRecord(now)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO conversation_sessions").
		WithArgs(record.SessionID, record.UserID, record.APIKeyID, record.AccountID, record.Provider, record.Model, record.RequestPath, record.SessionSource, record.RetentionUntil, record.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT turn_count FROM conversation_sessions WHERE session_id = $1 FOR UPDATE`)).
		WithArgs(record.SessionID).
		WillReturnRows(sqlmock.NewRows([]string{"turn_count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO conversation_turns").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`)).
		WithArgs(record.SessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE conversation_sessions").
		WithArgs(record.SessionID, record.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.UpsertTurn(context.Background(), record))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryUpsertTurnRecalculatesSessionQuality(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()
	record := conversationCaptureRepoTestTurnRecord(now)
	record.QualityStatus = service.ConversationQualityStatusClean
	record.Exportable = true

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO conversation_sessions").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT turn_count FROM conversation_sessions WHERE session_id = $1 FOR UPDATE`)).
		WithArgs(record.SessionID).
		WillReturnRows(sqlmock.NewRows([]string{"turn_count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO conversation_turns").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`)).
		WithArgs(record.SessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE conversation_sessions").
		WithArgs(record.SessionID, record.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.UpsertTurn(context.Background(), record))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryUpsertTurnNoInsertDoesNotUpdateSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()
	record := conversationCaptureRepoTestTurnRecord(now)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO conversation_sessions").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT turn_count FROM conversation_sessions WHERE session_id = $1 FOR UPDATE`)).
		WithArgs(record.SessionID).
		WillReturnRows(sqlmock.NewRows([]string{"turn_count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO conversation_turns").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = repo.UpsertTurn(context.Background(), record)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryCleanupExpiredDeletesWholeSessions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT session_id").
		WithArgs(now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1").AddRow("s2"))
	mock.ExpectExec("DELETE FROM conversation_turns").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM conversation_sessions").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	sessions, turns, err := repo.CleanupExpired(context.Background(), now, 100)

	require.NoError(t, err)
	require.Equal(t, int64(2), sessions)
	require.Equal(t, int64(3), turns)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryCompleteExportJobRequiresRunning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()

	mock.ExpectExec("UPDATE conversation_export_jobs").
		WithArgs(int64(7), int64(1), int64(2), int64(3), "conversation-exports/test.jsonl.zst", now).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.CompleteExportJob(context.Background(), 7, 1, 2, 3, "conversation-exports/test.jsonl.zst", now)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryFailExportJobRequiresPendingOrRunning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()

	mock.ExpectExec("UPDATE conversation_export_jobs").
		WithArgs(int64(7), "upload failed", now).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.FailExportJob(context.Background(), 7, "upload failed", now)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryMarkExpiredExportJobsDoesNotUpdateBeforeObjectDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()
	expiresAt := now.Add(-time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "status", "filters", "format", "encoding", "session_count", "turn_count", "file_size",
		"s3_key", "download_url_expires_at", "expires_at", "error_message", "created_by",
		"created_at", "updated_at", "started_at", "completed_at",
	}).AddRow(
		int64(7), service.ConversationExportJobStatusCompleted, []byte(`{"dedupe":true}`),
		service.ConversationExportFormatMessagesJSONL, service.ConversationExportEncodingZstd,
		int64(1), int64(2), int64(3), "conversation-exports/test.jsonl.zst", nil, expiresAt, nil, int64(1),
		expiresAt, expiresAt, expiresAt, expiresAt,
	)
	mock.ExpectQuery("SELECT id, status, filters").WithArgs(now, 100).WillReturnRows(rows)

	jobs, err := repo.MarkExpiredExportJobs(context.Background(), now, 100)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, service.ConversationExportJobStatusCompleted, jobs[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationCaptureRepositoryMarkExportJobsExpiredUpdatesSelectedIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewConversationCaptureRepository(db)
	now := time.Now()

	mock.ExpectExec("UPDATE conversation_export_jobs").
		WithArgs(sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 2))

	require.NoError(t, repo.MarkExportJobsExpired(context.Background(), []int64{7, 8}, now))
	require.NoError(t, mock.ExpectationsWereMet())
}

func conversationCaptureRepoTestTurnRecord(now time.Time) service.ConversationTurnRecord {
	return service.ConversationTurnRecord{
		SessionID:        "s1",
		SessionSource:    service.ConversationSessionSourceExplicit,
		UserID:           1,
		APIKeyID:         2,
		AccountID:        3,
		Provider:         service.ConversationProviderOpenAI,
		Model:            "gpt-5",
		RequestPath:      "/v1/chat/completions",
		RequestID:        "req-1",
		RequestMessages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		ResponseMessages: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"ok"}`)},
		Tools:            []json.RawMessage{},
		Usage:            map[string]any{},
		Meta:             map[string]any{},
		InputTokens:      1,
		OutputTokens:     2,
		TotalTokens:      3,
		ActualCost:       0.01,
		QualityStatus:    "unchecked",
		ParseStatus:      service.ConversationParseStatusSuccess,
		DedupeHash:       "hash",
		PayloadPreview:   "preview",
		RetentionUntil:   now.AddDate(0, 0, 30),
		CreatedAt:        now,
	}
}

func expectConversationSessionSummaryRecalculation(mock sqlmock.Sqlmock, sessionID string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM conversation_turns WHERE session_id = $1`)).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE conversation_sessions").
		WithArgs(sessionID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
