package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestBuildConversationExportPayloadZstdDedupe(t *testing.T) {
	turns := []ConversationTurn{
		conversationExportTurn("req-1", "same-hash"),
		conversationExportTurn("req-2", "same-hash"),
	}

	payload, sessions, turnsCount, err := buildConversationExportPayload(turns, ConversationExportEncodingZstd, true)

	require.NoError(t, err)
	require.Equal(t, int64(1), sessions)
	require.Equal(t, int64(1), turnsCount)
	zr, err := zstd.NewReader(bytes.NewReader(payload))
	require.NoError(t, err)
	defer zr.Close()
	decompressed, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.Len(t, nonEmptyJSONLLines(t, decompressed), 1)
}

func TestBuildConversationExportPayloadRechecksQualityGate(t *testing.T) {
	rejected := conversationExportTurn("req-rejected", "hash-rejected")
	rejected.ClientDisconnect = true
	rejected.QualityStatus = ConversationQualityStatusClean
	rejected.Exportable = true
	clean := conversationExportTurn("req-clean", "hash-clean")

	payload, sessions, turnsCount, err := buildConversationExportPayloadWithOptions(
		[]ConversationTurn{rejected, clean},
		ConversationExportEncodingPlain,
		ConversationExportJobFilters{IncludeDuplicates: true, Limit: 10},
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), sessions)
	require.Equal(t, int64(1), turnsCount)
	require.Contains(t, string(payload), "req-clean")
	require.NotContains(t, string(payload), "req-rejected")
}

func TestConversationExportJobUploadFailureMarksFailed(t *testing.T) {
	repo := newConversationExportJobTestRepo()
	store := &conversationExportJobTestStore{uploadErr: errors.New("upload failed")}
	svc := &ConversationCaptureService{
		repo:         repo,
		settingRepo:  &conversationExportJobSettingRepo{values: map[string]string{SettingKeyBackupS3Config: `{"bucket":"bucket","access_key_id":"ak","secret_access_key":"sk"}`}},
		storeFactory: func(context.Context, *BackupS3Config) (BackupObjectStore, error) { return store, nil },
	}
	job := repo.seedJob(ConversationExportJob{
		Status:    ConversationExportJobStatusPending,
		Filters:   ConversationExportJobFilters{Dedupe: true, Limit: 10},
		Format:    ConversationExportFormatMessagesJSONL,
		Encoding:  ConversationExportEncodingZstd,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedBy: 1,
	})
	repo.turns = []ConversationTurn{conversationExportTurn("req-1", "hash-1")}

	svc.runExportJob(context.Background(), job.ID)

	got, err := repo.GetExportJobByID(context.Background(), job.ID)
	require.NoError(t, err)
	require.Equal(t, ConversationExportJobStatusFailed, got.Status)
	require.NotNil(t, got.ErrorMessage)
	require.NotContains(t, *got.ErrorMessage, "hello")
}

func TestConversationExportDownloadTicketExpires(t *testing.T) {
	repo := newConversationExportJobTestRepo()
	store := &conversationExportJobTestStore{presignURL: "https://download.example/test"}
	svc := &ConversationCaptureService{
		repo:         repo,
		settingRepo:  &conversationExportJobSettingRepo{values: map[string]string{SettingKeyBackupS3Config: `{"bucket":"bucket","access_key_id":"ak","secret_access_key":"sk"}`}},
		storeFactory: func(context.Context, *BackupS3Config) (BackupObjectStore, error) { return store, nil },
	}
	key := "conversation-exports/2026/06/27/test.jsonl.zst"
	job := repo.seedJob(ConversationExportJob{
		Status:    ConversationExportJobStatusCompleted,
		Format:    ConversationExportFormatMessagesJSONL,
		Encoding:  ConversationExportEncodingZstd,
		S3Key:     &key,
		ExpiresAt: time.Now().Add(3 * time.Minute),
		CreatedBy: 1,
	})

	ticket, err := svc.CreateExportDownloadTicket(context.Background(), job.ID)

	require.NoError(t, err)
	require.Equal(t, "https://download.example/test", ticket.DownloadURL)
	require.True(t, ticket.ExpiresAt.Before(time.Now().Add(4*time.Minute)))
	got, err := repo.GetExportJobByID(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, got.DownloadURLExpiresAt)
}

func TestConversationExportJobRequestRejectsUnsupportedEncoding(t *testing.T) {
	_, err := normalizeConversationExportJobRequest(ConversationCreateExportJobRequest{
		Format:   ConversationExportFormatMessagesJSONL,
		Encoding: "zip",
	})

	require.Error(t, err)
}

func TestConversationCaptureResponsesPreviousResponseIDChaining(t *testing.T) {
	repo := newConversationExportJobTestRepo()
	svc := &ConversationCaptureService{repo: repo}
	now := time.Now()
	repo.responseToSession["resp_previous"] = "resp_session"

	sessionID, source := svc.resolveResponsesSessionID(context.Background(), ConversationCaptureMeta{}, conversationParseResult{
		PreviousResponseID: "resp_previous",
		ResponseID:         "resp_current",
	}, now)

	require.Equal(t, "resp_session", sessionID)
	require.Equal(t, ConversationSessionSourceResponses, source)
}

func TestParseOpenAIResponsesTurnNonStream(t *testing.T) {
	req := []byte(`{"input":[{"role":"user","content":"hello"}],"previous_response_id":"resp_prev"}`)
	resp := []byte(`{"id":"resp_current","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]},{"type":"function_call","name":"lookup","arguments":"{}"}]}`)

	got := parseOpenAIResponsesTurn(req, resp, false)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Equal(t, "resp_prev", got.PreviousResponseID)
	require.Equal(t, "resp_current", got.ResponseID)
	require.Len(t, got.RequestMessages, 1)
	require.Len(t, got.ResponseMessages, 1)
	require.Len(t, got.Tools, 1)
	require.Contains(t, string(got.ResponseMessages[0]), "hi")
}

func TestParseOpenAIResponsesTurnStream(t *testing.T) {
	req := []byte(`{"input":"hello"}`)
	resp := []byte(strings.Join([]string{
		`data: {"type":"response.output_text.delta","response":{"id":"resp_stream"},"delta":"he"}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"llo"}`,
		``,
		`data: {"type":"response.output_item.done","item":{"type":"function_call","name":"lookup","arguments":"{}"}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_stream","output":[]}}`,
		``,
	}, "\n"))

	got := parseOpenAIResponsesTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Equal(t, "resp_stream", got.ResponseID)
	require.Len(t, got.ResponseMessages, 1)
	require.Len(t, got.Tools, 1)
	require.Contains(t, string(got.ResponseMessages[0]), "hello")
}

func TestParseOpenAIResponsesTurnStreamSeparatesToolArgumentDeltas(t *testing.T) {
	req := []byte(`{"input":"hello"}`)
	resp := []byte(strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","name":"lookup"}}`,
		``,
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"query\":\"secret\"}"}`,
		``,
		`data: {"type":"response.output_text.delta","response":{"id":"resp_stream"},"delta":"hi"}`,
		``,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","name":"lookup","arguments":"{\"query\":\"secret\"}"}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_stream","output":[]}}`,
		``,
	}, "\n"))

	got := parseOpenAIResponsesTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Len(t, got.ResponseMessages, 1)
	require.Len(t, got.Tools, 1)
	require.Contains(t, string(got.ResponseMessages[0]), "hi")
	require.NotContains(t, string(got.ResponseMessages[0]), "secret")
	require.Contains(t, string(got.Tools[0]), "secret")
}

func TestParseOpenAIResponsesTurnStreamDoesNotDuplicateCompletedOutput(t *testing.T) {
	req := []byte(`{"input":"hello"}`)
	resp := []byte(strings.Join([]string{
		`data: {"type":"response.output_text.delta","response":{"id":"resp_stream"},"delta":"hi"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_stream","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}]}}`,
		``,
	}, "\n"))

	got := parseOpenAIResponsesTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Len(t, got.ResponseMessages, 1)
	require.Contains(t, string(got.ResponseMessages[0]), "hi")
}

type conversationExportJobTestRepo struct {
	mu                sync.Mutex
	jobs              map[int64]ConversationExportJob
	nextID            int64
	turns             []ConversationTurn
	responseToSession map[string]string
}

func newConversationExportJobTestRepo() *conversationExportJobTestRepo {
	return &conversationExportJobTestRepo{
		jobs:              map[int64]ConversationExportJob{},
		nextID:            1,
		responseToSession: map[string]string{},
	}
}

func (r *conversationExportJobTestRepo) seedJob(job ConversationExportJob) ConversationExportJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job.ID == 0 {
		job.ID = r.nextID
		r.nextID++
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}
	r.jobs[job.ID] = job
	return job
}

func (r *conversationExportJobTestRepo) UpsertTurn(_ context.Context, record ConversationTurnRecord) error {
	if responseID, _ := record.Meta["response_id"].(string); responseID != "" {
		r.responseToSession[responseID] = record.SessionID
	}
	return nil
}

func (r *conversationExportJobTestRepo) ListSessions(context.Context, ConversationSessionFilters, int, int) ([]ConversationSession, int64, error) {
	return nil, 0, nil
}

func (r *conversationExportJobTestRepo) GetSessionByID(context.Context, int64) (*ConversationSession, error) {
	return nil, nil
}

func (r *conversationExportJobTestRepo) ListTurnsBySessionID(context.Context, string, int, int) ([]ConversationTurnSummary, int64, error) {
	return nil, 0, nil
}

func (r *conversationExportJobTestRepo) GetTurnByID(context.Context, int64) (*ConversationTurn, error) {
	return nil, nil
}

func (r *conversationExportJobTestRepo) SetSessionExportable(context.Context, int64, bool) error {
	return nil
}

func (r *conversationExportJobTestRepo) SetTurnExportable(context.Context, int64, bool) error {
	return nil
}

func (r *conversationExportJobTestRepo) SetSessionQuality(context.Context, int64, string, []QualityError, *bool) error {
	return nil
}

func (r *conversationExportJobTestRepo) SetTurnQuality(context.Context, int64, string, []QualityError, *bool) error {
	return nil
}

func (r *conversationExportJobTestRepo) BulkSetQuality(context.Context, ConversationBulkQualityUpdateRequest) error {
	return nil
}

func (r *conversationExportJobTestRepo) MergeSessions(context.Context, int64, []int64) error {
	return nil
}

func (r *conversationExportJobTestRepo) SplitSessionFromTurn(context.Context, int64, int64, string, time.Time) (*ConversationSession, error) {
	return nil, nil
}

func (r *conversationExportJobTestRepo) MoveTurn(context.Context, int64, int64) error {
	return nil
}

func (r *conversationExportJobTestRepo) ListExportableTurns(context.Context, ConversationExportMessagesJSONLRequest) ([]ConversationTurn, error) {
	return r.turns, nil
}

func (r *conversationExportJobTestRepo) FindSessionIDByResponseID(_ context.Context, responseID string) (string, error) {
	sessionID, ok := r.responseToSession[responseID]
	if !ok {
		return "", errors.New("not found")
	}
	return sessionID, nil
}

func (r *conversationExportJobTestRepo) CreateExportJob(_ context.Context, job ConversationExportJob) (*ConversationExportJob, error) {
	seeded := r.seedJob(job)
	return &seeded, nil
}

func (r *conversationExportJobTestRepo) ListExportJobs(context.Context, int, int) ([]ConversationExportJob, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ConversationExportJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		out = append(out, job)
	}
	return out, int64(len(out)), nil
}

func (r *conversationExportJobTestRepo) GetExportJobByID(_ context.Context, id int64) (*ConversationExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &job, nil
}

func (r *conversationExportJobTestRepo) MarkExportJobRunning(_ context.Context, id int64, startedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	if job.Status != ConversationExportJobStatusPending && job.Status != ConversationExportJobStatusRunning {
		return errors.New("not pending")
	}
	job.Status = ConversationExportJobStatusRunning
	job.StartedAt = &startedAt
	job.UpdatedAt = startedAt
	r.jobs[id] = job
	return nil
}

func (r *conversationExportJobTestRepo) CompleteExportJob(_ context.Context, id int64, sessionCount, turnCount, fileSize int64, s3Key string, completedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	job.Status = ConversationExportJobStatusCompleted
	job.SessionCount = sessionCount
	job.TurnCount = turnCount
	job.FileSize = fileSize
	job.S3Key = &s3Key
	job.CompletedAt = &completedAt
	job.UpdatedAt = completedAt
	r.jobs[id] = job
	return nil
}

func (r *conversationExportJobTestRepo) FailExportJob(_ context.Context, id int64, errorMessage string, failedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	job.Status = ConversationExportJobStatusFailed
	job.ErrorMessage = &errorMessage
	job.CompletedAt = &failedAt
	job.UpdatedAt = failedAt
	r.jobs[id] = job
	return nil
}

func (r *conversationExportJobTestRepo) SetExportJobDownloadURLExpiresAt(_ context.Context, id int64, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	job.DownloadURLExpiresAt = &expiresAt
	job.UpdatedAt = time.Now()
	r.jobs[id] = job
	return nil
}

func (r *conversationExportJobTestRepo) MarkExportJobDeleted(_ context.Context, id int64, deletedAt time.Time) (*ConversationExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	job.Status = ConversationExportJobStatusDeleted
	job.UpdatedAt = deletedAt
	r.jobs[id] = job
	return &job, nil
}

func (r *conversationExportJobTestRepo) MarkExpiredExportJobs(context.Context, time.Time, int) ([]ConversationExportJob, error) {
	return nil, nil
}

func (r *conversationExportJobTestRepo) MarkExportJobsExpired(context.Context, []int64, time.Time) error {
	return nil
}

func (r *conversationExportJobTestRepo) CleanupExpired(context.Context, time.Time, int) (int64, int64, error) {
	return 0, 0, nil
}

type conversationExportJobSettingRepo struct {
	values map[string]string
}

func (r *conversationExportJobSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *conversationExportJobSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (r *conversationExportJobSettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *conversationExportJobSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (r *conversationExportJobSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *conversationExportJobSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *conversationExportJobSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

type conversationExportJobTestStore struct {
	uploadErr  error
	presignURL string
}

func (s *conversationExportJobTestStore) Upload(context.Context, string, io.Reader, string) (int64, error) {
	if s.uploadErr != nil {
		return 0, s.uploadErr
	}
	return 123, nil
}

func (s *conversationExportJobTestStore) UploadFile(ctx context.Context, key string, filePath string, contentType string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	return s.Upload(ctx, key, file, contentType)
}

func (s *conversationExportJobTestStore) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *conversationExportJobTestStore) Delete(context.Context, string) error {
	return nil
}

func (s *conversationExportJobTestStore) PresignURL(context.Context, string, time.Duration) (string, error) {
	if s.presignURL != "" {
		return s.presignURL, nil
	}
	return "https://download.example/default", nil
}

func (s *conversationExportJobTestStore) HeadBucket(context.Context) error {
	return nil
}
