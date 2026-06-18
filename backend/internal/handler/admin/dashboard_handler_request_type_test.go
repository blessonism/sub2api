package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dashboardUsageRepoCapture struct {
	service.UsageLogRepository
	trendRequestType *int16
	trendStream      *bool
	modelRequestType *int16
	modelStream      *bool
	rankingLimit     int
	ranking          []usagestats.UserSpendingRankingItem
	rankingTotal     float64
	tokenFilters     usagestats.AdminTokenLeaderboardFilters
	tokenDetailsUser int64
}

type balanceGrantCall struct {
	grants []service.BalanceGrantInput
	notes  string
}

type dashboardBalanceGrantCapture struct {
	calls []balanceGrantCall
}

func (s *dashboardBalanceGrantCapture) GrantUserBalances(ctx context.Context, grants []service.BalanceGrantInput, notes string) ([]service.BalanceGrantResult, error) {
	s.calls = append(s.calls, balanceGrantCall{
		grants: append([]service.BalanceGrantInput(nil), grants...),
		notes:  notes,
	})
	results := make([]service.BalanceGrantResult, 0, len(grants))
	for _, grant := range grants {
		results = append(results, service.BalanceGrantResult{
			User: &service.User{
				ID:       grant.UserID,
				Email:    "granted@example.com",
				Username: "granted",
				Balance:  42.5,
			},
			BalanceDelta: grant.Amount,
		})
	}
	return results, nil
}

func (s *dashboardUsageRepoCapture) GetUsageTrendWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	granularity string,
	userID, apiKeyID, accountID, groupID int64,
	model string,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.TrendDataPoint, error) {
	s.trendRequestType = requestType
	s.trendStream = stream
	return []usagestats.TrendDataPoint{}, nil
}

func (s *dashboardUsageRepoCapture) GetModelStatsWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	userID, apiKeyID, accountID, groupID int64,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.ModelStat, error) {
	s.modelRequestType = requestType
	s.modelStream = stream
	return []usagestats.ModelStat{}, nil
}

func (s *dashboardUsageRepoCapture) GetUserSpendingRanking(
	ctx context.Context,
	startTime, endTime time.Time,
	limit int,
) (*usagestats.UserSpendingRankingResponse, error) {
	s.rankingLimit = limit
	return &usagestats.UserSpendingRankingResponse{
		Ranking:         s.ranking,
		TotalActualCost: s.rankingTotal,
		TotalRequests:   44,
		TotalTokens:     1234,
	}, nil
}

func (s *dashboardUsageRepoCapture) GetAdminTokenLeaderboard(
	ctx context.Context,
	startTime, endTime time.Time,
	filters usagestats.AdminTokenLeaderboardFilters,
) (*usagestats.AdminTokenLeaderboardResponse, error) {
	s.tokenFilters = filters
	return &usagestats.AdminTokenLeaderboardResponse{
		Ranking: []usagestats.AdminTokenLeaderboardUser{
			{
				Rank:         1,
				UserID:       7,
				Email:        "alice@example.com",
				Username:     "alice",
				Status:       "active",
				RegisteredAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Requests:     3,
				Tokens:       1200,
				ActualCost:   1.25,
			},
		},
		TotalRequests:   3,
		TotalTokens:     1200,
		TotalActualCost: 1.25,
	}, nil
}

func (s *dashboardUsageRepoCapture) GetAdminTokenLeaderboardUserDetails(
	ctx context.Context,
	startTime, endTime time.Time,
	userID int64,
	filters usagestats.AdminTokenLeaderboardFilters,
) (*usagestats.AdminTokenLeaderboardUserDetails, error) {
	s.tokenDetailsUser = userID
	s.tokenFilters = filters
	return &usagestats.AdminTokenLeaderboardUserDetails{
		APIKeys: []usagestats.AdminTokenLeaderboardAPIKeyUsage{
			{APIKeyID: 3, APIKeyName: "prod", Requests: 2, Tokens: 800, ActualCost: 0.8},
		},
	}, nil
}

func newDashboardRequestTypeTestRouter(repo *dashboardUsageRepoCapture, balanceSvc ...adminBalanceUpdater) *gin.Engine {
	gin.SetMode(gin.TestMode)
	dashboardSvc := service.NewDashboardService(repo, nil, nil, nil)
	handler := NewDashboardHandler(dashboardSvc, nil, nil)
	if len(balanceSvc) > 0 {
		handler.adminService = balanceSvc[0]
	}
	router := gin.New()
	router.GET("/admin/dashboard/trend", handler.GetUsageTrend)
	router.GET("/admin/dashboard/models", handler.GetModelStats)
	router.GET("/admin/dashboard/users-ranking", handler.GetUserSpendingRanking)
	router.GET("/admin/dashboard/token-leaderboard", handler.GetAdminTokenLeaderboard)
	router.POST("/admin/dashboard/token-leaderboard/grant-balance", handler.GrantAdminTokenLeaderboardBalance)
	router.GET("/admin/dashboard/token-leaderboard/users/:user_id/details", handler.GetAdminTokenLeaderboardUserDetails)
	return router
}

func TestDashboardTrendRequestTypePriority(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/trend?request_type=ws_v2&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.trendRequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.trendRequestType)
	require.Nil(t, repo.trendStream)
}

func TestDashboardTrendInvalidRequestType(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/trend?request_type=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardTrendInvalidStream(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/trend?stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardModelStatsRequestTypePriority(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/models?request_type=sync&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.modelRequestType)
	require.Equal(t, int16(service.RequestTypeSync), *repo.modelRequestType)
	require.Nil(t, repo.modelStream)
}

func TestDashboardModelStatsInvalidRequestType(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/models?request_type=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardModelStatsInvalidStream(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/models?stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardModelStatsInvalidModelSource(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/models?model_source=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardModelStatsValidModelSource(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/models?model_source=upstream", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDashboardUsersRankingLimitAndCache(t *testing.T) {
	dashboardUsersRankingCache = newSnapshotCache(5 * time.Minute)
	repo := &dashboardUsageRepoCapture{
		ranking: []usagestats.UserSpendingRankingItem{
			{UserID: 7, Email: "rank@example.com", ActualCost: 10.5, Requests: 3, Tokens: 300},
		},
		rankingTotal: 88.8,
	}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/users-ranking?limit=100&start_date=2025-01-01&end_date=2025-01-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 50, repo.rankingLimit)
	require.Contains(t, rec.Body.String(), "\"total_actual_cost\":88.8")
	require.Contains(t, rec.Body.String(), "\"total_requests\":44")
	require.Contains(t, rec.Body.String(), "\"total_tokens\":1234")
	require.Equal(t, "miss", rec.Header().Get("X-Snapshot-Cache"))

	req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard/users-ranking?limit=100&start_date=2025-01-01&end_date=2025-01-02", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusOK, rec2.Code)
	require.Equal(t, "hit", rec2.Header().Get("X-Snapshot-Cache"))
}

func TestAdminTokenLeaderboardFiltersForwarded(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/token-leaderboard?limit=20&email=alice&group_id=9&model=claude&model_source=upstream&user_status=active", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, usagestats.AdminTokenLeaderboardFilters{
		Email:      "alice",
		GroupID:    9,
		Model:      "claude",
		ModelType:  usagestats.ModelSourceUpstream,
		UserStatus: "active",
		Limit:      20,
	}, repo.tokenFilters)
	require.Contains(t, rec.Body.String(), "\"email\":\"alice@example.com\"")
}

func TestAdminTokenLeaderboardRejectsUnsupportedLimit(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/token-leaderboard?limit=12", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminTokenLeaderboardDetailsParsesUserID(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/token-leaderboard/users/42/details?limit=50", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.tokenDetailsUser)
	require.Equal(t, 50, repo.tokenFilters.Limit)
	require.Contains(t, rec.Body.String(), "\"api_key_name\":\"prod\"")
}

func TestAdminTokenLeaderboardGrantBalanceUsesTop10AndAdminBalance(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	body := bytes.NewBufferString(`{"user_ids":[7],"amount":3.5,"notes":"campaign bonus"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/token-leaderboard/grant-balance?limit=50&email=alice", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 10, repo.tokenFilters.Limit)
	require.Equal(t, "alice", repo.tokenFilters.Email)
	require.Equal(t, []balanceGrantCall{
		{grants: []service.BalanceGrantInput{{UserID: 7, Amount: 3.5}}, notes: "campaign bonus"},
	}, balanceSvc.calls)
	require.Contains(t, rec.Body.String(), "\"granted_count\":1")
	require.Contains(t, rec.Body.String(), "\"balance\":42.5")
}

func TestAdminTokenLeaderboardGrantBalanceRejectsNonTop10User(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	body := bytes.NewBufferString(`{"user_ids":[8],"amount":1}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/token-leaderboard/grant-balance", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, balanceSvc.calls)
	require.Contains(t, rec.Body.String(), "current Top10")
}

func TestAdminTokenLeaderboardGrantBalanceRejectsDuplicateUsers(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	body := bytes.NewBufferString(`{"user_ids":[7,7],"amount":1}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/token-leaderboard/grant-balance", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, balanceSvc.calls)
	require.Contains(t, rec.Body.String(), "duplicate")
}
