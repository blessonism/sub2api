//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestConversationHistoryRemovalFreshInstall(t *testing.T) {
	for _, table := range []string{"conversation_sessions", "conversation_turns", "conversation_export_jobs"} {
		var name sql.NullString
		require.NoError(t, integrationDB.QueryRow("SELECT to_regclass($1)", table).Scan(&name))
		require.False(t, name.Valid, table)
	}
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT count(*) FROM settings WHERE key = 'conversation_capture_config'").Scan(&count))
	require.Zero(t, count)
}

func TestConversationHistoryRemovalUpgradeAndRepeat(t *testing.T) {
	tx := testTx(t)
	seedLegacyConversationHistory(t, tx)
	content, err := migrations.FS.ReadFile("254_remove_conversation_history.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(context.Background(), string(content))
		require.NoError(t, err)
	}
	for _, name := range []string{
		"conversation_sessions", "conversation_turns", "conversation_export_jobs",
		"conversation_sessions_id_seq", "conversation_turns_id_seq", "conversation_export_jobs_id_seq",
	} {
		var relation sql.NullString
		require.NoError(t, tx.QueryRow("SELECT to_regclass($1)", name).Scan(&relation))
		require.False(t, relation.Valid, name)
	}
	var count int
	require.NoError(t, tx.QueryRow("SELECT count(*) FROM settings WHERE key = 'conversation_capture_config'").Scan(&count))
	require.Zero(t, count)
	var value string
	require.NoError(t, tx.QueryRow("SELECT value FROM settings WHERE key = 'backup_s3_config'").Scan(&value))
	require.Equal(t, "ordinary-backup-config", value)
	require.NoError(t, tx.QueryRow("SELECT count(*) FROM usage_logs").Scan(&count))
}

func TestConversationHistoryRemovalDependencyFailureRollsBack(t *testing.T) {
	tx := testTx(t)
	seedLegacyConversationHistory(t, tx)
	_, err := tx.Exec("CREATE VIEW local_conversation_dependency AS SELECT id FROM conversation_sessions")
	require.NoError(t, err)
	_, err = tx.Exec("SAVEPOINT before_removal")
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("254_remove_conversation_history.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(content))
	require.ErrorContains(t, err, "depend")
	_, err = tx.Exec("ROLLBACK TO SAVEPOINT before_removal")
	require.NoError(t, err)
	for _, table := range []string{"conversation_sessions", "conversation_turns", "conversation_export_jobs"} {
		var count int
		require.NoError(t, tx.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
		require.Equal(t, 1, count, table)
	}
	var value string
	require.NoError(t, tx.QueryRow("SELECT value FROM settings WHERE key = 'conversation_capture_config'").Scan(&value))
	require.Equal(t, `{"enabled":true}`, value)
}

func seedLegacyConversationHistory(t *testing.T, tx *sql.Tx) {
	t.Helper()
	for _, name := range []string{
		"161_conversation_capture.sql", "162_conversation_export_jobs.sql", "163_conversation_quality_governance.sql",
	} {
		content, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = tx.ExecContext(context.Background(), string(content))
		require.NoError(t, err)
	}
	_, err := tx.Exec(`
		INSERT INTO conversation_sessions
			(session_id, user_id, api_key_id, model, request_path, retention_until, started_at, ended_at)
		VALUES ('local-test', 1, 1, 'gpt-5', '/v1/responses', NOW(), NOW(), NOW());
		INSERT INTO conversation_turns
			(session_id, request_id, turn_index, model, request_path, dedupe_hash, retention_until)
		VALUES ('local-test', 'request-test', 1, 'gpt-5', '/v1/responses', 'test-hash', NOW());
		INSERT INTO conversation_export_jobs (expires_at, created_by) VALUES (NOW(), 1);
		INSERT INTO settings (key, value)
		VALUES ('conversation_capture_config', '{"enabled":true}'), ('backup_s3_config', 'ordinary-backup-config')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
	`)
	require.NoError(t, err)
}
