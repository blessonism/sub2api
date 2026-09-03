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
	trendRequestType      *int16
	trendStream           *bool
	trendNativeCompaction *bool
	modelRequestType      *int16
	modelStream           *bool
	modelNativeCompaction *bool
	groupNativeCompaction *bool
	trendMismatch         *bool
	modelMismatch         *bool
	groupMismatch         *bool
	rankingLimit          int
	ranking               []usagestats.UserSpendingRankingItem
	rankingTotal          float64
	tokenFilters          usagestats.AdminTokenLeaderboardFilters
	tokenDetailsUser      int64
}

type balanceGrantCall struct {
	grants []service.BalanceGrantInput
	notes  string
}

type dashboardBalanceGrantCapture struct {
	calls          []balanceGrantCall
	summary        *service.AdminBalanceSummary
	updatedUserIDs []int64
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

func (s *dashboardBalanceGrantCapture) GetBalanceSummary(context.Context) (*service.AdminBalanceSummary, error) {
	if s.summary != nil {
		return s.summary, nil
	}
	return &service.AdminBalanceSummary{
		TotalUsers:    2,
		IncludedUsers: 2,
		TotalBalance:  12.5,
	}, nil
}

func (s *dashboardBalanceGrantCapture) UpdateBalanceSummaryExclusions(_ context.Context, userIDs []int64) (*service.AdminBalanceSummary, error) {
	s.updatedUserIDs = make([]int64, len(userIDs))
	copy(s.updatedUserIDs, userIDs)
	return &service.AdminBalanceSummary{
		TotalUsers:        2,
		IncludedUsers:     1,
		ExcludedUserCount: len(userIDs),
		TotalBalance:      10,
		ExcludedUserIDs:   append([]int64(nil), userIDs...),
	}, nil
}

func (s *dashboardUsageRepoCapture) GetUsageTrendWithUsageFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	granularity string,
	filters usagestats.UsageLogFilters,
) ([]usagestats.TrendDataPoint, error) {
	s.trendRequestType = filters.RequestType
	s.trendStream = filters.Stream
	s.trendNativeCompaction = filters.NativeCompactionV2
	s.trendMismatch = filters.UpstreamModelMismatch
	return []usagestats.TrendDataPoint{}, nil
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

func (s *dashboardUsageRepoCapture) GetModelStatsWithUsageFiltersBySource(
	ctx context.Context,
	startTime, endTime time.Time,
	filters usagestats.UsageLogFilters,
	source string,
) ([]usagestats.ModelStat, error) {
	s.modelRequestType = filters.RequestType
	s.modelStream = filters.Stream
	s.modelNativeCompaction = filters.NativeCompactionV2
	s.modelMismatch = filters.UpstreamModelMismatch
	return []usagestats.ModelStat{}, nil
}

func (s *dashboardUsageRepoCapture) GetGroupStatsWithUsageFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	filters usagestats.UsageLogFilters,
) ([]usagestats.GroupStat, error) {
	s.groupNativeCompaction = filters.NativeCompactionV2
	s.groupMismatch = filters.UpstreamModelMismatch
	return []usagestats.GroupStat{}, nil
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
				LastUsedAt:   time.Date(2025, 1, 2, 8, 30, 0, 0, time.UTC),
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
		CalibrationBalanceDelta: -2.5,
		APIKeys: []usagestats.AdminTokenLeaderboardAPIKeyUsage{
			{APIKeyID: 3, APIKeyName: "prod", Requests: 2, Tokens: 800, ActualCost: 0.8},
		},
	}, nil
}

type dashboardAdminBalanceService interface {
	adminBalanceUpdater
	adminBalanceSummaryService
}

func newDashboardRequestTypeTestRouter(repo *dashboardUsageRepoCapture, balanceSvc ...dashboardAdminBalanceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	dashboardSvc := service.NewDashboardService(repo, nil, nil, nil)
	handler := NewDashboardHandler(dashboardSvc, nil, nil)
	if len(balanceSvc) > 0 {
		handler.adminService = balanceSvc[0]
		handler.balanceSummary = balanceSvc[0]
	}
	router := gin.New()
	router.GET("/admin/dashboard/trend", handler.GetUsageTrend)
	router.GET("/admin/dashboard/models", handler.GetModelStats)
	router.GET("/admin/dashboard/groups", handler.GetGroupStats)
	router.GET("/admin/dashboard/users-ranking", handler.GetUserSpendingRanking)
	router.GET("/admin/dashboard/token-leaderboard", handler.GetAdminTokenLeaderboard)
	router.POST("/admin/dashboard/token-leaderboard/grant-balance", handler.GrantAdminTokenLeaderboardBalance)
	router.GET("/admin/dashboard/token-leaderboard/users/:user_id/details", handler.GetAdminTokenLeaderboardUserDetails)
	router.GET("/admin/dashboard/balance-summary", handler.GetBalanceSummary)
	router.PUT("/admin/dashboard/balance-summary/exclusions", handler.UpdateBalanceSummaryExclusions)
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

func TestDashboardNativeCompactionFilterPropagatesAlongsideTransport(t *testing.T) {
	resetDashboardReadCachesForTest()
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	for _, path := range []string{
		"/admin/dashboard/trend?request_type=stream&native_compaction_v2=true",
		"/admin/dashboard/models?request_type=stream&native_compaction_v2=true",
		"/admin/dashboard/groups?request_type=stream&native_compaction_v2=true",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, path)
	}

	require.NotNil(t, repo.trendNativeCompaction)
	require.True(t, *repo.trendNativeCompaction)
	require.NotNil(t, repo.modelNativeCompaction)
	require.True(t, *repo.modelNativeCompaction)
	require.NotNil(t, repo.groupNativeCompaction)
	require.True(t, *repo.groupNativeCompaction)
	require.NotNil(t, repo.trendRequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.trendRequestType)
}

func TestDashboardNativeCompactionFilterRejectsInvalidBoolean(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	for _, path := range []string{
		"/admin/dashboard/trend?native_compaction_v2=invalid",
		"/admin/dashboard/models?native_compaction_v2=invalid",
		"/admin/dashboard/groups?native_compaction_v2=invalid",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
	}
}

func TestDashboardModelAuditFilterPropagatesToTrendModelAndGroupQueries(t *testing.T) {
	resetDashboardReadCachesForTest()
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	for _, path := range []string{
		"/admin/dashboard/trend?upstream_model_mismatch=true",
		"/admin/dashboard/models?upstream_model_mismatch=true",
		"/admin/dashboard/groups?upstream_model_mismatch=true",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, path)
	}

	require.NotNil(t, repo.trendMismatch)
	require.True(t, *repo.trendMismatch)
	require.NotNil(t, repo.modelMismatch)
	require.True(t, *repo.modelMismatch)
	require.NotNil(t, repo.groupMismatch)
	require.True(t, *repo.groupMismatch)
}

func TestDashboardModelAuditFilterRejectsInvalidBoolean(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	router := newDashboardRequestTypeTestRouter(repo)

	for _, path := range []string{
		"/admin/dashboard/trend?upstream_model_mismatch=invalid",
		"/admin/dashboard/models?upstream_model_mismatch=invalid",
		"/admin/dashboard/groups?upstream_model_mismatch=invalid",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
	}
}

func TestDashboardUsersRankingLimitAndCache(t *testing.T) {
	dashboardUsersRankingCache = newSnapshotCache(5 * time.Minute)
	lastUsed := time.Date(2025, 1, 2, 18, 30, 0, 0, time.UTC)
	repo := &dashboardUsageRepoCapture{
		ranking: []usagestats.UserSpendingRankingItem{
			{UserID: 7, Email: "rank@example.com", ActualCost: 10.5, Requests: 3, Tokens: 300, LastUsedAt: lastUsed},
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
	require.Contains(t, rec.Body.String(), "\"last_used_at\":\"2025-01-02T18:30:00Z\"")
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
	require.Contains(t, rec.Body.String(), "\"calibration_balance_delta\":-2.5")
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

func TestAdminBalanceSummaryReturnsSummary(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{
		summary: &service.AdminBalanceSummary{
			TotalUsers:        3,
			IncludedUsers:     2,
			ExcludedUserCount: 1,
			TotalBalance:      12.5,
			ByRole: []service.AdminBalanceSummaryBucket{
				{Key: service.RoleUser, UserCount: 2, Balance: 12.5},
			},
		},
	}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/balance-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "\"total_users\":3")
	require.Contains(t, rec.Body.String(), "\"excluded_user_count\":1")
	require.Contains(t, rec.Body.String(), "\"total_balance\":12.5")
}

func TestAdminBalanceSummaryUpdateExclusionsSavesIDs(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	body := bytes.NewBufferString(`{"user_ids":[5,7]}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/dashboard/balance-summary/exclusions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{5, 7}, balanceSvc.updatedUserIDs)
	require.Contains(t, rec.Body.String(), "\"excluded_user_ids\":[5,7]")
}

func TestAdminBalanceSummaryUpdateExclusionsRequiresUserIDsField(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	for _, body := range []string{`{}`, `{"user_ids":null}`} {
		req := httptest.NewRequest(http.MethodPut, "/admin/dashboard/balance-summary/exclusions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Nil(t, balanceSvc.updatedUserIDs)
	}
}

func TestAdminBalanceSummaryUpdateExclusionsAllowsExplicitEmptyList(t *testing.T) {
	repo := &dashboardUsageRepoCapture{}
	balanceSvc := &dashboardBalanceGrantCapture{}
	router := newDashboardRequestTypeTestRouter(repo, balanceSvc)

	req := httptest.NewRequest(http.MethodPut, "/admin/dashboard/balance-summary/exclusions", bytes.NewBufferString(`{"user_ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, balanceSvc.updatedUserIDs)
	require.Empty(t, balanceSvc.updatedUserIDs)
}
