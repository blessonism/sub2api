package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	createdBy             int64
	created               *service.UpstreamRelayConnector
	snapshots             []service.UpstreamRelayGroupRateSnapshot
	snapshotChangeFilters service.UpstreamRelaySnapshotChangeListFilters
	snapshotChangeParams  pagination.PaginationParams
	usageHistoryFilters   service.UpstreamRelayUsageHistoryListFilters
	usageHistoryParams    pagination.PaginationParams
	usageHistoryRows      []service.UpstreamRelayGroupUsageHistory
	recommendationFilters service.UpstreamRelayRecommendationRunListFilters
	recommendationParams  pagination.PaginationParams
	syncStatus            string
	syncError             string
	usageByGroup          map[string]service.UpstreamRelayGroupTodayUsage
	usageCheckedAt        *time.Time
}

func (r *upstreamRelayHandlerRepo) ListConnectors(context.Context, pagination.PaginationParams, service.UpstreamRelayConnectorListFilters) ([]service.UpstreamRelayConnector, *pagination.PaginationResult, error) {
	if r.created != nil {
		return []service.UpstreamRelayConnector{*r.created}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1}, nil
	}
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

func (r *upstreamRelayHandlerRepo) UpdateConnectorTokens(context.Context, int64, int64, string, string) (*service.UpstreamRelayConnector, error) {
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

func (r *upstreamRelayHandlerRepo) ListSnapshotChanges(_ context.Context, params pagination.PaginationParams, filters service.UpstreamRelaySnapshotChangeListFilters) ([]service.UpstreamRelayGroupRateSnapshotChange, *pagination.PaginationResult, error) {
	r.snapshotChangeParams = params
	r.snapshotChangeFilters = filters
	oldRate := 1.25
	newRate := 1.5
	return []service.UpstreamRelayGroupRateSnapshotChange{
		{
			ID:                     9,
			ConnectorID:            filters.ConnectorID,
			ConnectorName:          "relay-a",
			UpstreamGroupID:        "gpt-pro",
			GroupName:              "GPT Pro",
			Platform:               "openai",
			ChangeType:             service.UpstreamRelaySnapshotChangeRateChanged,
			OldFinalRateMultiplier: &oldRate,
			NewFinalRateMultiplier: &newRate,
			OldStatus:              "active",
			NewStatus:              "active",
			Source:                 service.UpstreamRelayRateSourceOverride,
			ChangedAt:              time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		},
	}, &pagination.PaginationResult{Total: 1, Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *upstreamRelayHandlerRepo) UpsertUsageHistory(context.Context, []service.UpstreamRelayGroupUsageHistoryUpsert) error {
	return nil
}

func (r *upstreamRelayHandlerRepo) ListUsageHistory(_ context.Context, params pagination.PaginationParams, filters service.UpstreamRelayUsageHistoryListFilters) ([]service.UpstreamRelayGroupUsageHistory, *pagination.PaginationResult, error) {
	r.usageHistoryParams = params
	r.usageHistoryFilters = filters
	items := append([]service.UpstreamRelayGroupUsageHistory{}, r.usageHistoryRows...)
	if len(items) == 0 {
		items = []service.UpstreamRelayGroupUsageHistory{
			{
				ID:              10,
				UsageDate:       filters.StartDate,
				ConnectorID:     filters.ConnectorID,
				ConnectorName:   "relay-a",
				UpstreamGroupID: filters.UpstreamGroupID,
				GroupName:       "GPT Pro",
				Platform:        "openai",
				ActualCost:      1.25,
				TotalTokens:     1200,
				CheckedAt:       time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
			},
		}
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *upstreamRelayHandlerRepo) MarkConnectorSync(_ context.Context, _ int64, status string, errMessage string) error {
	r.syncStatus = status
	r.syncError = errMessage
	if r.created != nil {
		r.created.Status = status
		r.created.LastError = errMessage
	}
	return nil
}

func (r *upstreamRelayHandlerRepo) UpdateConnectorAccountBalance(context.Context, int64, *float64, *time.Time) error {
	return nil
}

func (r *upstreamRelayHandlerRepo) UpdateSnapshotTodayUsage(_ context.Context, _ int64, usageByGroup map[string]service.UpstreamRelayGroupTodayUsage, checkedAt *time.Time) error {
	r.usageByGroup = usageByGroup
	r.usageCheckedAt = checkedAt
	for i := range r.snapshots {
		if checkedAt == nil || usageByGroup == nil {
			r.snapshots[i].TodayActualCost = nil
			r.snapshots[i].TodayTotalTokens = nil
			r.snapshots[i].TodayUsageCheckedAt = nil
			continue
		}
		usage := usageByGroup[r.snapshots[i].UpstreamGroupID]
		actualCost := usage.ActualCost
		totalTokens := usage.TotalTokens
		r.snapshots[i].TodayActualCost = &actualCost
		r.snapshots[i].TodayTotalTokens = &totalTokens
		r.snapshots[i].TodayUsageCheckedAt = checkedAt
	}
	return nil
}

func (r *upstreamRelayHandlerRepo) ListCandidateUsageBindings(context.Context, int64) ([]service.UpstreamRelayCandidateUsageBinding, error) {
	return nil, nil
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

func (r *upstreamRelayHandlerRepo) GetMonitoringPolicy(context.Context) (*service.UpstreamRelayMonitoringPolicy, error) {
	return nil, nil
}

func (r *upstreamRelayHandlerRepo) UpsertMonitoringPolicy(_ context.Context, policy service.UpstreamRelayMonitoringPolicy, operatorID int64) (*service.UpstreamRelayMonitoringPolicy, error) {
	policy.UpdatedBy = operatorID
	return &policy, nil
}

func (r *upstreamRelayHandlerRepo) GetRecommendationPolicy(context.Context) (*service.UpstreamRelayRecommendationPolicy, error) {
	return nil, nil
}

func (r *upstreamRelayHandlerRepo) UpsertRecommendationPolicy(_ context.Context, policy service.UpstreamRelayRecommendationPolicy, operatorID int64) (*service.UpstreamRelayRecommendationPolicy, error) {
	policy.UpdatedBy = operatorID
	return &policy, nil
}

func (r *upstreamRelayHandlerRepo) CreateRecommendationRun(_ context.Context, run service.UpstreamRelayRecommendationRun, suggestions []service.UpstreamRelayRecommendationSuggestion) (*service.UpstreamRelayRecommendationRun, error) {
	run.ID = 1
	run.Suggestions = suggestions
	return &run, nil
}

func (r *upstreamRelayHandlerRepo) GetRecommendationRun(context.Context, int64) (*service.UpstreamRelayRecommendationRun, error) {
	return nil, service.ErrUpstreamRelayRunNotFound
}

func (r *upstreamRelayHandlerRepo) ListRecommendationRuns(_ context.Context, params pagination.PaginationParams, filters service.UpstreamRelayRecommendationRunListFilters) ([]service.UpstreamRelayRecommendationRun, *pagination.PaginationResult, error) {
	r.recommendationParams = params
	r.recommendationFilters = filters
	return []service.UpstreamRelayRecommendationRun{
		{
			ID:              55,
			Status:          service.UpstreamRelayRunStatusSuccess,
			TotalCandidates: 3,
			SuggestionCount: 2,
			CreatedAt:       time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		},
	}, &pagination.PaginationResult{Total: 21, Page: params.Page, PageSize: params.PageSize, Pages: 2}, nil
}

func (r *upstreamRelayHandlerRepo) ApplyRecommendationRun(context.Context, int64, int64) (*service.UpstreamRelayRecommendationRun, error) {
	return nil, service.ErrUpstreamRelayRunNotFound
}

func (r *upstreamRelayHandlerRepo) CloseRecommendationRun(_ context.Context, runID int64, operatorID int64) (*service.UpstreamRelayRecommendationRun, error) {
	closedAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	return &service.UpstreamRelayRecommendationRun{
		ID:              runID,
		Status:          service.UpstreamRelayRunStatusSuccess,
		TotalCandidates: 3,
		SuggestionCount: 2,
		Closed:          true,
		ClosedBy:        &operatorID,
		ClosedAt:        &closedAt,
	}, nil
}

func (r *upstreamRelayHandlerRepo) RestoreRecommendationRun(_ context.Context, runID int64, operatorID int64) (*service.UpstreamRelayRecommendationRun, error) {
	_ = operatorID
	return &service.UpstreamRelayRecommendationRun{
		ID:              runID,
		Status:          service.UpstreamRelayRunStatusSuccess,
		TotalCandidates: 3,
		SuggestionCount: 2,
		Closed:          false,
	}, nil
}

func (r *upstreamRelayHandlerRepo) DeleteRecommendationRun(context.Context, int64) error {
	return service.ErrUpstreamRelayRunNotFound
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

func TestUpstreamRelayHandlerCreateConnectorSucceedsWhenImmediateSyncFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary upstream outage", http.StatusBadGateway)
	}))
	defer upstream.Close()

	body := bytes.NewBufferString(`{
		"name":"relay",
		"base_url":"` + upstream.URL + `",
		"auth_mode":"manual_session",
		"bearer_token":"sk-handler-secret"
	}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/connectors", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	handler.CreateConnector(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, service.UpstreamRelayConnectorStatusNeedsReauth, repo.syncStatus)
	require.Contains(t, repo.syncError, "upstream HTTP 502")

	var envelope struct {
		Data service.UpstreamRelayConnector `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(42), envelope.Data.ID)
	require.Equal(t, service.UpstreamRelayConnectorStatusNeedsReauth, envelope.Data.Status)
	require.Contains(t, envelope.Data.LastError, "upstream HTTP 502")
	require.NotContains(t, w.Body.String(), "sk-handler-secret")
}

func TestUpstreamRelayHandlerListSnapshotChangesReturnsPaginatedShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/upstream-relay-group-monitors/snapshot-changes?page=2&page_size=5&connector_id=42&change_type=rate_changed&search=gpt", nil)

	handler.ListSnapshotChanges(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 2, repo.snapshotChangeParams.Page)
	require.Equal(t, 5, repo.snapshotChangeParams.PageSize)
	require.Equal(t, int64(42), repo.snapshotChangeFilters.ConnectorID)
	require.Equal(t, service.UpstreamRelaySnapshotChangeRateChanged, repo.snapshotChangeFilters.ChangeType)
	require.Equal(t, "gpt", repo.snapshotChangeFilters.Search)
	var envelope struct {
		Data struct {
			Items []service.UpstreamRelayGroupRateSnapshotChange `json:"items"`
			Total int64                                          `json:"total"`
			Page  int                                            `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Equal(t, 2, envelope.Data.Page)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "gpt-pro", envelope.Data.Items[0].UpstreamGroupID)
	require.Equal(t, service.UpstreamRelaySnapshotChangeRateChanged, envelope.Data.Items[0].ChangeType)
}

func TestUpstreamRelayHandlerRefreshMonitoringDataReturnsAggregateResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer handler-token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Group 1","platform":"openai","status":"active","rate_multiplier":1.5}]`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"g1":0.75}`))
		case "/api/v1/user/profile":
			_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := &upstreamRelayHandlerRepo{
		created: &service.UpstreamRelayConnector{
			ID:                   42,
			Name:                 "relay-a",
			BaseURL:              upstream.URL,
			BearerTokenEncrypted: "handler-token",
			Status:               service.UpstreamRelayConnectorStatusActive,
		},
	}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/refresh", nil)

	handler.RefreshMonitoringData(c)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data service.UpstreamRelayMonitoringRefreshResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 1, envelope.Data.Total)
	require.Equal(t, 1, envelope.Data.Partial)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "partial", envelope.Data.Items[0].Status)
	require.Equal(t, "success", envelope.Data.Items[0].SnapshotStatus)
	require.NotNil(t, envelope.Data.Items[0].Metrics)
}

func TestUpstreamRelayHandlerListUsageHistoryReturnsPaginatedShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/upstream-relay-group-monitors/usage-history?page=3&page_size=10&start_date=2026-06-28&end_date=2026-06-29&connector_id=42&upstream_group_id=gpt-pro&search=relay", nil)

	handler.ListUsageHistory(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 3, repo.usageHistoryParams.Page)
	require.Equal(t, 10, repo.usageHistoryParams.PageSize)
	require.Equal(t, "2026-06-28", repo.usageHistoryFilters.StartDate)
	require.Equal(t, "2026-06-29", repo.usageHistoryFilters.EndDate)
	require.Equal(t, int64(42), repo.usageHistoryFilters.ConnectorID)
	require.Equal(t, "gpt-pro", repo.usageHistoryFilters.UpstreamGroupID)
	require.Equal(t, "relay", repo.usageHistoryFilters.Search)
	var envelope struct {
		Data struct {
			Items []service.UpstreamRelayGroupUsageHistory `json:"items"`
			Total int64                                    `json:"total"`
			Page  int                                      `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Equal(t, 3, envelope.Data.Page)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "gpt-pro", envelope.Data.Items[0].UpstreamGroupID)
	require.Equal(t, 1.25, envelope.Data.Items[0].ActualCost)
}

func TestUpstreamRelayHandlerListRecommendationRunsParsesSuggestionFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/upstream-relay-group-monitors/recommendations?page=2&page_size=20&has_suggestions=true", nil)

	handler.ListRecommendationRuns(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 2, repo.recommendationParams.Page)
	require.Equal(t, 20, repo.recommendationParams.PageSize)
	require.NotNil(t, repo.recommendationFilters.HasSuggestions)
	require.True(t, *repo.recommendationFilters.HasSuggestions)
	var envelope struct {
		Data struct {
			Items []service.UpstreamRelayRecommendationRun `json:"items"`
			Total int64                                    `json:"total"`
			Page  int                                      `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(21), envelope.Data.Total)
	require.Equal(t, 2, envelope.Data.Page)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(55), envelope.Data.Items[0].ID)
}

func TestUpstreamRelayHandlerCloseRecommendationRunReturnsClosedDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/recommendations/55/close", nil)
	c.Params = gin.Params{{Key: "id", Value: "55"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	handler.CloseRecommendationRun(c)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data service.UpstreamRelayRecommendationRun `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(55), envelope.Data.ID)
	require.True(t, envelope.Data.Closed)
	require.NotNil(t, envelope.Data.ClosedBy)
	require.Equal(t, int64(77), *envelope.Data.ClosedBy)
}

func TestUpstreamRelayHandlerRestoreRecommendationRunReturnsRestoredDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &upstreamRelayHandlerRepo{}
	svc := service.NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayHandlerEncryptor{})
	handler := NewUpstreamRelayGroupMonitoringHandler(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/upstream-relay-group-monitors/recommendations/55/restore", nil)
	c.Params = gin.Params{{Key: "id", Value: "55"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	handler.RestoreRecommendationRun(c)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data service.UpstreamRelayRecommendationRun `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, int64(55), envelope.Data.ID)
	require.False(t, envelope.Data.Closed)
	require.False(t, envelope.Data.Applied)
}
