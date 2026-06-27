package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConversationCaptureWriterCapturesAndTruncates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	writer := newConversationCaptureWriter(ctx.Writer, 5)

	n, err := writer.Write([]byte("hello world"))
	snapshot, truncated, disconnect := writer.Snapshot()

	require.NoError(t, err)
	require.Equal(t, len("hello world"), n)
	require.Equal(t, "hello world", rec.Body.String())
	require.Equal(t, []byte("hello"), snapshot)
	require.True(t, truncated)
	require.False(t, disconnect)
}

func TestConversationCaptureWriterSnapshotReturnsCopy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	writer := newConversationCaptureWriter(ctx.Writer, 20)
	_, _ = writer.WriteString("hello")

	snapshot, _, _ := writer.Snapshot()
	snapshot[0] = 'X'
	again, _, _ := writer.Snapshot()

	require.Equal(t, []byte("hello"), again)
}

func TestConversationCaptureWriterDoesNotOverrideFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	writer := newConversationCaptureWriter(ctx.Writer, 20)

	writer.Header().Set("X-Test", "ok")
	writer.WriteHeader(http.StatusCreated)

	require.Equal(t, http.StatusCreated, writer.Status())
	require.Equal(t, "ok", rec.Header().Get("X-Test"))
}

func TestConversationCaptureStateSubmitsOnlyAfterMetaAndResponseReady(t *testing.T) {
	repo := &conversationCaptureStateTestRepo{}
	pool := service.NewConversationCaptureWorkerPool(nil)
	defer pool.Stop()
	svc := service.NewConversationCaptureService(repo, nil, pool, nil)
	state := &conversationCaptureState{
		svc: svc,
		decision: service.ConversationCaptureDecision{
			Capture:              true,
			MaxTurnPayloadBytes:  1024,
			PayloadPreviewChars:  200,
			SessionWindowMinutes: 30,
			RetentionDays:        30,
		},
	}

	state.setResponse([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`), false, false)
	require.Eventually(t, func() bool { return repo.count() == 0 }, 50*time.Millisecond, 5*time.Millisecond)

	state.setMeta(service.ConversationCaptureMeta{
		RequestID:   "req-1",
		UserID:      1,
		APIKeyID:    2,
		AccountID:   3,
		Model:       "gpt-5",
		RequestPath: "/v1/chat/completions",
		RequestBody: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	})

	require.Eventually(t, func() bool { return repo.count() == 1 }, time.Second, 10*time.Millisecond)
}

type conversationCaptureStateTestRepo struct {
	ch chan service.ConversationTurnRecord
}

func (r *conversationCaptureStateTestRepo) ensure() {
	if r.ch == nil {
		r.ch = make(chan service.ConversationTurnRecord, 4)
	}
}

func (r *conversationCaptureStateTestRepo) count() int {
	r.ensure()
	return len(r.ch)
}

func (r *conversationCaptureStateTestRepo) UpsertTurn(_ context.Context, record service.ConversationTurnRecord) error {
	r.ensure()
	r.ch <- record
	return nil
}
func (r *conversationCaptureStateTestRepo) ListSessions(context.Context, service.ConversationSessionFilters, int, int) ([]service.ConversationSession, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureStateTestRepo) GetSessionByID(context.Context, int64) (*service.ConversationSession, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) ListTurnsBySessionID(context.Context, string, int, int) ([]service.ConversationTurnSummary, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureStateTestRepo) GetTurn(context.Context, int64) (*service.ConversationTurn, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) GetTurnByID(context.Context, int64) (*service.ConversationTurn, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) SetSessionExportable(context.Context, int64, bool) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) SetTurnExportable(context.Context, int64, bool) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) SetSessionQuality(context.Context, int64, string, []service.QualityError, *bool) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) SetTurnQuality(context.Context, int64, string, []service.QualityError, *bool) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) BulkSetQuality(context.Context, service.ConversationBulkQualityUpdateRequest) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) MergeSessions(context.Context, int64, []int64) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) SplitSessionFromTurn(context.Context, int64, int64, string, time.Time) (*service.ConversationSession, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) MoveTurn(context.Context, int64, int64) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) ListExportableTurns(context.Context, service.ConversationExportMessagesJSONLRequest) ([]service.ConversationTurn, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) FindSessionIDByResponseID(context.Context, string) (string, error) {
	return "", nil
}
func (r *conversationCaptureStateTestRepo) CreateExportJob(context.Context, service.ConversationExportJob) (*service.ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) ListExportJobs(context.Context, int, int) ([]service.ConversationExportJob, int64, error) {
	return nil, 0, nil
}
func (r *conversationCaptureStateTestRepo) GetExportJobByID(context.Context, int64) (*service.ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) MarkExportJobRunning(context.Context, int64, time.Time) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) CompleteExportJob(context.Context, int64, int64, int64, int64, string, time.Time) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) FailExportJob(context.Context, int64, string, time.Time) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) SetExportJobDownloadURLExpiresAt(context.Context, int64, time.Time) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) MarkExportJobDeleted(context.Context, int64, time.Time) (*service.ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) MarkExpiredExportJobs(context.Context, time.Time, int) ([]service.ConversationExportJob, error) {
	return nil, nil
}
func (r *conversationCaptureStateTestRepo) MarkExportJobsExpired(context.Context, []int64, time.Time) error {
	return nil
}
func (r *conversationCaptureStateTestRepo) CleanupExpired(context.Context, time.Time, int) (int64, int64, error) {
	return 0, 0, nil
}
