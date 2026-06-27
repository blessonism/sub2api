package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type upstreamRelayHandlerEncryptor struct{}

func (upstreamRelayHandlerEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (upstreamRelayHandlerEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

type upstreamRelayHandlerRepo struct {
	createdBy  int64
	created    *service.UpstreamRelayConnector
	snapshots  []service.UpstreamRelayGroupRateSnapshot
	syncStatus string
}

func (r *upstreamRelayHandlerRepo) ListConnectors(context.Context, pagination.PaginationParams, service.UpstreamRelayConnectorListFilters) ([]service.UpstreamRelayConnector, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Total: 0, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamRelayHandlerRepo) GetConnector(context.Context, int64) (*service.UpstreamRelayConnector, error) {
	if r.created != nil {
		copy := *r.created
		return &copy, nil
	}
	return nil, service.ErrUpstreamRelayConnectorNotFound
}

func (r *upstreamRelayHandlerRepo) CreateConnector(_ context.Context, connector *service.UpstreamRelayConnector) (*service.UpstreamRelayConnector, error) {
	r.createdBy = connector.CreatedBy
	copy := *connector
	copy.ID = 42
	r.created = &copy
	return &copy, nil
}

func (r *upstreamRelayHandlerRepo) UpdateConnector(context.Context, *service.UpstreamRelayConnector, bool) (*service.UpstreamRelayConnector, error) {
	return nil, service.ErrUpstreamRelayConnectorNotFound
}

func (r *upstreamRelayHandlerRepo) SoftDeleteConnector(context.Context, int64) error {
	return service.ErrUpstreamRelayConnectorNotFound
}

func (r *upstreamRelayHandlerRepo) UpsertSnapshots(_ context.Context, _ int64, snapshots []service.UpstreamRelayGroupRateSnapshot) error {
	r.snapshots = []service.UpstreamRelayGroupRateSnapshot{}
	r.snapshots = append(r.snapshots, snapshots...)
	return nil
}

func (r *upstreamRelayHandlerRepo) ListSnapshots(context.Context, int64) ([]service.UpstreamRelayGroupRateSnapshot, error) {
	return r.snapshots, nil
}

func (r *upstreamRelayHandlerRepo) MarkConnectorSync(_ context.Context, _ int64, status string, _ string) error {
	r.syncStatus = status
	return nil
}

func (r *upstreamRelayHandlerRepo) ListCandidates(context.Context, pagination.PaginationParams, service.UpstreamRelayCandidateListFilters) ([]service.UpstreamRelayCandidate, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Total: 0, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamRelayHandlerRepo) GetCandidate(context.Context, int64) (*service.UpstreamRelayCandidate, error) {
	return nil, service.ErrUpstreamRelayCandidateNotFound
}

func (r *upstreamRelayHandlerRepo) CreateCandidate(context.Context, *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	return nil, service.ErrUpstreamRelayCandidateNotFound
}

func (r *upstreamRelayHandlerRepo) UpdateCandidate(context.Context, *service.UpstreamRelayCandidate) (*service.UpstreamRelayCandidate, error) {
	return nil, service.ErrUpstreamRelayCandidateNotFound
}

func (r *upstreamRelayHandlerRepo) SoftDeleteCandidate(context.Context, int64) error {
	return service.ErrUpstreamRelayCandidateNotFound
}

func (r *upstreamRelayHandlerRepo) InsertProbeResult(context.Context, service.UpstreamRelayProbeResult) (*service.UpstreamRelayProbeResult, error) {
	return nil, service.ErrUpstreamRelayCandidateNotFound
}

func (r *upstreamRelayHandlerRepo) InsertUsageDeltaSample(context.Context, service.UpstreamRelayUsageDeltaSample) (*service.UpstreamRelayUsageDeltaSample, error) {
	return nil, nil
}

func (r *upstreamRelayHandlerRepo) ListRecommendationInputs(context.Context) ([]service.UpstreamRelayCandidate, error) {
	return nil, nil
}

func (r *upstreamRelayHandlerRepo) CreateRecommendationRun(_ context.Context, run service.UpstreamRelayRecommendationRun, suggestions []service.UpstreamRelayRecommendationSuggestion) (*service.UpstreamRelayRecommendationRun, error) {
	run.ID = 1
	run.Suggestions = suggestions
	return &run, nil
}

func (r *upstreamRelayHandlerRepo) GetRecommendationRun(context.Context, int64) (*service.UpstreamRelayRecommendationRun, error) {
	return nil, service.ErrUpstreamRelayRunNotFound
}

func (r *upstreamRelayHandlerRepo) ListRecommendationRuns(context.Context, pagination.PaginationParams) ([]service.UpstreamRelayRecommendationRun, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Total: 0, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamRelayHandlerRepo) ApplyRecommendationRun(context.Context, int64, int64) (*service.UpstreamRelayRecommendationRun, error) {
	return nil, service.ErrUpstreamRelayRunNotFound
}

func TestUpstreamRelayHandlerCreateConnectorUsesOperatorAndRedactsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-handler-secret", r.Header.Get("Authorization"))
		require.Equal(t, "cf_clearance=secret", r.Header.Get("Cookie"))
		require.Equal(t, "browser", r.Header.Get("User-Agent"))
		switch r.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Group 1","platform":"openai","status":"active","rate_multiplier":1.5}]`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"g1":0.75}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	body := bytes.NewBufferString(`{
		"name":"relay",
		"base_url":"` + upstream.URL + `",
		"auth_mode":"manual_session",
		"bearer_token":"Bearer sk-handler-secret",
		"cookie":"cf_clearance=secret",
		"user_agent":"browser"
	}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/connectors", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	handler.CreateConnector(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(77), repo.createdBy)
	require.Equal(t, service.UpstreamRelayConnectorStatusActive, repo.syncStatus)
	require.Len(t, repo.snapshots, 1)
	require.Equal(t, 0.75, repo.snapshots[0].FinalRateMultiplier)
	require.NotNil(t, repo.snapshots[0].OverrideRateMultiplier)

	var envelope struct {
		Data service.UpstreamRelayConnector `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(42), envelope.Data.ID)
	require.True(t, envelope.Data.HasBearerToken)
	require.NotContains(t, w.Body.String(), "sk-handler-secret")
	require.NotContains(t, w.Body.String(), "cf_clearance=secret")
	require.Empty(t, envelope.Data.BearerTokenPlain)
	require.Empty(t, envelope.Data.CookiePlain)
}

func TestUpstreamRelayHandlerCreatePasswordLoginConnectorRedactsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			body, _ := io.ReadAll(r.Body)
			require.Contains(t, string(body), "admin@example.com")
			require.Contains(t, string(body), "upstream-password")
			_, _ = w.Write([]byte(`{"data":{"access_token":"login-access-token","refresh_token":"login-refresh-token"}}`))
		case "/api/v1/groups/available":
			require.Equal(t, "Bearer login-access-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Group 1","platform":"openai","status":"active","rate_multiplier":1.5}]`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	body := bytes.NewBufferString(`{
		"name":"relay",
		"base_url":"` + upstream.URL + `",
		"auth_mode":"password_login",
		"login_email":"admin@example.com",
		"login_password":"upstream-password"
	}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/connectors", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	handler.CreateConnector(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(77), repo.createdBy)
	require.Equal(t, service.UpstreamRelayConnectorStatusActive, repo.syncStatus)

	var envelope struct {
		Data service.UpstreamRelayConnector `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, service.UpstreamRelayAuthModePasswordLogin, envelope.Data.AuthMode)
	require.True(t, envelope.Data.HasBearerToken)
	require.True(t, envelope.Data.HasRefreshToken)
	require.True(t, envelope.Data.HasLoginEmail)
	require.NotContains(t, w.Body.String(), "upstream-password")
	require.NotContains(t, w.Body.String(), "login-access-token")
	require.NotContains(t, w.Body.String(), "login-refresh-token")
	require.NotContains(t, w.Body.String(), "admin@example.com")
}
