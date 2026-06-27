package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConversationExportMessagesJSONLAllowsDedupeFalse(t *testing.T) {
	repo := &conversationCaptureExportRepo{
		turns: []ConversationTurn{
			conversationExportTurn("req-1", "same-hash"),
			conversationExportTurn("req-2", "same-hash"),
		},
	}
	svc := NewConversationCaptureService(repo, nil, nil, nil)
	dedupe := false

	data, err := svc.ExportMessagesJSONL(context.Background(), ConversationExportMessagesJSONLRequest{
		Dedupe: &dedupe,
		Limit:  10,
	})

	require.NoError(t, err)
	lines := nonEmptyJSONLLines(t, data)
	require.Len(t, lines, 2)
}

func TestConversationExportMessagesJSONLDefaultsDedupeTrue(t *testing.T) {
	repo := &conversationCaptureExportRepo{
		turns: []ConversationTurn{
			conversationExportTurn("req-1", "same-hash"),
			conversationExportTurn("req-2", "same-hash"),
		},
	}
	svc := NewConversationCaptureService(repo, nil, nil, nil)

	data, err := svc.ExportMessagesJSONL(context.Background(), ConversationExportMessagesJSONLRequest{Limit: 10})

	require.NoError(t, err)
	lines := nonEmptyJSONLLines(t, data)
	require.Len(t, lines, 1)
}

func TestConversationExportMessagesJSONLIncludeDuplicates(t *testing.T) {
	repo := &conversationCaptureExportRepo{
		turns: []ConversationTurn{
			conversationExportTurn("req-1", "same-hash"),
			conversationExportTurn("req-2", "same-hash"),
		},
	}
	svc := NewConversationCaptureService(repo, nil, nil, nil)

	data, err := svc.ExportMessagesJSONL(context.Background(), ConversationExportMessagesJSONLRequest{
		IncludeDuplicates: true,
		Limit:             10,
	})

	require.NoError(t, err)
	lines := nonEmptyJSONLLines(t, data)
	require.Len(t, lines, 2)
}

func TestConversationExportMessagesJSONLRedactsSensitiveText(t *testing.T) {
	repo := &conversationCaptureExportRepo{
		turns: []ConversationTurn{
			{
				SessionID:        "s1",
				RequestID:        "req-1",
				RequestMessages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":"email me at suki@example.com with Bearer sk-test-secret-token-1234567890"}`)},
				ResponseMessages: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"visit https://example.com/cb?token=abc123456789012345&ok=1 or call +1 415 555 1212"}`)},
				Meta:             map[string]any{"user_id": float64(1), "api_key_id": float64(2)},
				Model:            "gpt-5",
				DedupeHash:       "hash-1",
			},
		},
	}
	svc := NewConversationCaptureService(repo, nil, nil, nil)

	data, err := svc.ExportMessagesJSONL(context.Background(), ConversationExportMessagesJSONLRequest{
		RedactionEnabled: true,
		Limit:            10,
	})

	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "[REDACTED_EMAIL]")
	require.Contains(t, text, "[REDACTED_TOKEN]")
	require.Contains(t, text, "%5BREDACTED%5D")
	require.Contains(t, text, "[REDACTED_PHONE]")
	require.NotContains(t, text, "suki@example.com")
	require.NotContains(t, text, "sk-test-secret-token")
}

func conversationExportTurn(requestID, hash string) ConversationTurn {
	return ConversationTurn{
		SessionID:        "s1",
		RequestID:        requestID,
		RequestMessages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)},
		ResponseMessages: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"hi"}`)},
		Meta:             map[string]any{"user_id": float64(1), "api_key_id": float64(2)},
		Model:            "gpt-5",
		DedupeHash:       hash,
	}
}

func nonEmptyJSONLLines(t *testing.T, data []byte) []string {
	t.Helper()
	var lines []string
	for _, line := range splitJSONLLines(string(data)) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitJSONLLines(s string) []string {
	out := []string{}
	start := 0
	for i, r := range s {
		if r == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

type conversationCaptureExportRepo struct {
	turns []ConversationTurn
}

func (r *conversationCaptureExportRepo) UpsertTurn(context.Context, ConversationTurnRecord) error {
	return nil
}
func (r *conversationCaptureExportRepo) ListSessions(context.Context, ConversationSessionFilters, int, int) ([]ConversationSession, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureExportRepo) GetSessionByID(context.Context, int64) (*ConversationSession, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) ListTurnsBySessionID(context.Context, string, int, int) ([]ConversationTurnSummary, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureExportRepo) GetTurnByID(context.Context, int64) (*ConversationTurn, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) SetSessionExportable(context.Context, int64, bool) error {
	return nil
}
func (r *conversationCaptureExportRepo) SetTurnExportable(context.Context, int64, bool) error {
	return nil
}
func (r *conversationCaptureExportRepo) SetSessionQuality(context.Context, int64, string, []QualityError, *bool) error {
	return nil
}
func (r *conversationCaptureExportRepo) SetTurnQuality(context.Context, int64, string, []QualityError, *bool) error {
	return nil
}
func (r *conversationCaptureExportRepo) BulkSetQuality(context.Context, ConversationBulkQualityUpdateRequest) error {
	return nil
}
func (r *conversationCaptureExportRepo) MergeSessions(context.Context, int64, []int64) error {
	return nil
}
func (r *conversationCaptureExportRepo) SplitSessionFromTurn(context.Context, int64, int64, string, time.Time) (*ConversationSession, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) MoveTurn(context.Context, int64, int64) error {
	return nil
}
func (r *conversationCaptureExportRepo) ListExportableTurns(context.Context, ConversationExportMessagesJSONLRequest) ([]ConversationTurn, error) {
	return r.turns, nil
}
func (r *conversationCaptureExportRepo) FindSessionIDByResponseID(context.Context, string) (string, error) {
	return "", nil
}
func (r *conversationCaptureExportRepo) CreateExportJob(context.Context, ConversationExportJob) (*ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) ListExportJobs(context.Context, int, int) ([]ConversationExportJob, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureExportRepo) GetExportJobByID(context.Context, int64) (*ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) MarkExportJobRunning(context.Context, int64, time.Time) error {
	return nil
}
func (r *conversationCaptureExportRepo) CompleteExportJob(context.Context, int64, int64, int64, int64, string, time.Time) error {
	return nil
}
func (r *conversationCaptureExportRepo) FailExportJob(context.Context, int64, string, time.Time) error {
	return nil
}
func (r *conversationCaptureExportRepo) SetExportJobDownloadURLExpiresAt(context.Context, int64, time.Time) error {
	return nil
}
func (r *conversationCaptureExportRepo) MarkExportJobDeleted(context.Context, int64, time.Time) (*ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) MarkExpiredExportJobs(context.Context, time.Time, int) ([]ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureExportRepo) MarkExportJobsExpired(context.Context, []int64, time.Time) error {
	return nil
}
func (r *conversationCaptureExportRepo) CleanupExpired(context.Context, time.Time, int) (int64, int64, error) {
	return 0, 0, nil
}
