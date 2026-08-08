package admin

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// DashboardHandler handles admin dashboard statistics
type DashboardHandler struct {
	dashboardService   *service.DashboardService
	aggregationService *service.DashboardAggregationService
	adminService       adminBalanceUpdater
	balanceSummary     adminBalanceSummaryService
	startTime          time.Time // Server start time for uptime calculation
}

type adminBalanceUpdater interface {
	GrantUserBalances(ctx context.Context, grants []service.BalanceGrantInput, notes string) ([]service.BalanceGrantResult, error)
}

type adminBalanceSummaryService interface {
	GetBalanceSummary(ctx context.Context) (*service.AdminBalanceSummary, error)
	UpdateBalanceSummaryExclusions(ctx context.Context, userIDs []int64) (*service.AdminBalanceSummary, error)
}

// NewDashboardHandler creates a new admin dashboard handler
func NewDashboardHandler(dashboardService *service.DashboardService, aggregationService *service.DashboardAggregationService, adminService service.AdminService) *DashboardHandler {
	h := &DashboardHandler{
		dashboardService:   dashboardService,
		aggregationService: aggregationService,
		adminService:       adminService,
		balanceSummary:     adminService,
		startTime:          time.Now(),
	}
	return h
}

// parseTimeRange parses start_date, end_date query parameters
// Uses user's timezone if provided, otherwise falls back to server timezone
func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime time.Time

	if startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		} else {
			startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
		}
	} else {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	}

	if endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.Add(24 * time.Hour) // Include the end date
		} else {
			endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		}
	} else {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	}

	return startTime, endTime
}

func parseOptionalBoolDashboardFilter(c *gin.Context, name string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// GetStats handles getting dashboard statistics
// GET /api/v1/admin/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashboardService.GetDashboardStats(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "Failed to get dashboard statistics")
		return
	}

	// Calculate uptime in seconds
	uptime := int64(time.Since(h.startTime).Seconds())

	response.Success(c, gin.H{
		// 用户统计
		"total_users":            stats.TotalUsers,
		"today_new_users":        stats.TodayNewUsers,
		"active_users":           stats.ActiveUsers,
		"today_active_users":     stats.TodayActiveUsers,
		"yesterday_active_users": stats.YesterdayActiveUsers,
		"total_user_balance":     stats.TotalUserBalance,

		// API Key 统计
		"total_api_keys":  stats.TotalAPIKeys,
		"active_api_keys": stats.ActiveAPIKeys,

		// 账户统计
		"total_accounts":     stats.TotalAccounts,
		"normal_accounts":    stats.NormalAccounts,
		"error_accounts":     stats.ErrorAccounts,
		"ratelimit_accounts": stats.RateLimitAccounts,
		"overload_accounts":  stats.OverloadAccounts,

		// 累计 Token 使用统计
		"total_requests":              stats.TotalRequests,
		"total_input_tokens":          stats.TotalInputTokens,
		"total_output_tokens":         stats.TotalOutputTokens,
		"total_cache_creation_tokens": stats.TotalCacheCreationTokens,
		"total_cache_read_tokens":     stats.TotalCacheReadTokens,
		"total_calibration_tokens":    stats.TotalCalibrationTokens,
		"total_tokens":                stats.TotalTokens,
		"total_cost":                  stats.TotalCost,       // 标准计费
		"total_actual_cost":           stats.TotalActualCost, // 实际扣除

		// 今日 Token 使用统计
		"today_requests":              stats.TodayRequests,
		"today_input_tokens":          stats.TodayInputTokens,
		"today_output_tokens":         stats.TodayOutputTokens,
		"today_cache_creation_tokens": stats.TodayCacheCreationTokens,
		"today_cache_read_tokens":     stats.TodayCacheReadTokens,
		"today_calibration_tokens":    stats.TodayCalibrationTokens,
		"today_tokens":                stats.TodayTokens,
		"today_cost":                  stats.TodayCost,       // 今日标准计费
		"today_actual_cost":           stats.TodayActualCost, // 今日实际扣除

		// 系统运行统计
		"average_duration_ms": stats.AverageDurationMs,
		"uptime":              uptime,

		// 性能指标
		"rpm": stats.Rpm,
		"tpm": stats.Tpm,

		// 运营资产统计
		"subscription_remaining_value": stats.SubscriptionRemainingValue,

		// 预聚合新鲜度
		"hourly_active_users": stats.HourlyActiveUsers,
		"stats_updated_at":    stats.StatsUpdatedAt,
		"stats_stale":         stats.StatsStale,
	})
}

type DashboardAggregationBackfillRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type BalanceSummaryExclusionsRequest struct {
	UserIDs *[]int64 `json:"user_ids" binding:"required"`
}

// GetBalanceSummary handles admin balance liability summary.
// GET /api/v1/admin/dashboard/balance-summary
func (h *DashboardHandler) GetBalanceSummary(c *gin.Context) {
	if h.balanceSummary == nil {
		response.InternalError(c, "Admin service not available")
		return
	}
	summary, err := h.balanceSummary.GetBalanceSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// UpdateBalanceSummaryExclusions saves the internal-user exclusion list and returns refreshed summary.
// PUT /api/v1/admin/dashboard/balance-summary/exclusions
func (h *DashboardHandler) UpdateBalanceSummaryExclusions(c *gin.Context) {
	if h.balanceSummary == nil {
		response.InternalError(c, "Admin service not available")
		return
	}

	var req BalanceSummaryExclusionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.UserIDs == nil {
		response.BadRequest(c, "user_ids is required")
		return
	}
	summary, err := h.balanceSummary.UpdateBalanceSummaryExclusions(c.Request.Context(), *req.UserIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// BackfillAggregation handles triggering aggregation backfill
// POST /api/v1/admin/dashboard/aggregation/backfill
func (h *DashboardHandler) BackfillAggregation(c *gin.Context) {
	if h.aggregationService == nil {
		response.InternalError(c, "Aggregation service not available")
		return
	}

	var req DashboardAggregationBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		response.BadRequest(c, "Invalid start time")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		response.BadRequest(c, "Invalid end time")
		return
	}

	if err := h.aggregationService.TriggerBackfill(start, end); err != nil {
		if errors.Is(err, service.ErrDashboardBackfillDisabled) {
			response.Forbidden(c, "Backfill is disabled")
			return
		}
		if errors.Is(err, service.ErrDashboardBackfillTooLarge) {
			response.BadRequest(c, "Backfill range too large")
			return
		}
		response.InternalError(c, "Failed to trigger backfill")
		return
	}

	response.Success(c, gin.H{
		"status": "accepted",
	})
}

// GetRealtimeMetrics handles getting real-time system metrics
// GET /api/v1/admin/dashboard/realtime
func (h *DashboardHandler) GetRealtimeMetrics(c *gin.Context) {
	// Return mock data for now
	response.Success(c, gin.H{
		"active_requests":       0,
		"requests_per_minute":   0,
		"average_response_time": 0,
		"error_rate":            0.0,
	})
}

// GetUsageTrend handles getting usage trend data
// GET /api/v1/admin/dashboard/trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), user_id, api_key_id, model, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")

	// Parse optional filter params
	var userID, apiKeyID, accountID, groupID int64
	var model string
	var requestType *int16
	var stream *bool
	var billingType *int8
	var upstreamModelMismatch *bool

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if modelStr := c.Query("model"); modelStr != "" {
		model = modelStr
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}
	upstreamModelMismatch, err := parseOptionalBoolDashboardFilter(c, "upstream_model_mismatch")
	if err != nil {
		response.BadRequest(c, "Invalid upstream_model_mismatch value, use true or false")
		return
	}

	trend, hit, err := h.getUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, userID, apiKeyID, accountID, groupID, model, requestType, stream, billingType, upstreamModelMismatch)
	if err != nil {
		response.Error(c, 500, "Failed to get usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// GetModelStats handles getting model usage statistics
// GET /api/v1/admin/dashboard/models
// Query params: start_date, end_date (YYYY-MM-DD), user_id, api_key_id, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetModelStats(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	// Parse optional filter params
	var userID, apiKeyID, accountID, groupID int64
	modelSource := usagestats.ModelSourceRequested
	var requestType *int16
	var stream *bool
	var billingType *int8
	var upstreamModelMismatch *bool

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if rawModelSource := strings.TrimSpace(c.Query("model_source")); rawModelSource != "" {
		if !usagestats.IsValidModelSource(rawModelSource) {
			response.BadRequest(c, "Invalid model_source, use requested/upstream/mapping")
			return
		}
		modelSource = rawModelSource
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}
	upstreamModelMismatch, err := parseOptionalBoolDashboardFilter(c, "upstream_model_mismatch")
	if err != nil {
		response.BadRequest(c, "Invalid upstream_model_mismatch value, use true or false")
		return
	}

	stats, hit, err := h.getModelStatsCached(c.Request.Context(), startTime, endTime, userID, apiKeyID, accountID, groupID, modelSource, requestType, stream, billingType, upstreamModelMismatch)
	if err != nil {
		response.Error(c, 500, "Failed to get model statistics")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"models":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// GetGroupStats handles getting group usage statistics
// GET /api/v1/admin/dashboard/groups
// Query params: start_date, end_date (YYYY-MM-DD), user_id, api_key_id, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetGroupStats(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	var userID, apiKeyID, accountID, groupID int64
	var requestType *int16
	var stream *bool
	var billingType *int8
	var upstreamModelMismatch *bool

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}
	upstreamModelMismatch, err := parseOptionalBoolDashboardFilter(c, "upstream_model_mismatch")
	if err != nil {
		response.BadRequest(c, "Invalid upstream_model_mismatch value, use true or false")
		return
	}

	stats, hit, err := h.getGroupStatsCached(c.Request.Context(), startTime, endTime, userID, apiKeyID, accountID, groupID, requestType, stream, billingType, upstreamModelMismatch)
	if err != nil {
		response.Error(c, 500, "Failed to get group statistics")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"groups":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// GetAPIKeyUsageTrend handles getting API key usage trend data
// GET /api/v1/admin/dashboard/api-keys-trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), limit (default 5)
func (h *DashboardHandler) GetAPIKeyUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}

	trend, hit, err := h.getAPIKeyUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get API key usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// GetUserUsageTrend handles getting user usage trend data
// GET /api/v1/admin/dashboard/users-trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), limit (default 12)
func (h *DashboardHandler) GetUserUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")
	limitStr := c.DefaultQuery("limit", "12")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 12
	}

	trend, hit, err := h.getUserUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get user usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// BatchUsersUsageRequest represents the request body for batch user usage stats
type BatchUsersUsageRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

var dashboardUsersRankingCache = newSnapshotCache(5 * time.Minute)
var dashboardBatchUsersUsageCache = newSnapshotCache(30 * time.Second)
var dashboardBatchAPIKeysUsageCache = newSnapshotCache(30 * time.Second)

func clearAdminDashboardUsageSnapshotCaches() {
	dashboardUsersRankingCache.Clear()
	dashboardBatchUsersUsageCache.Clear()
	dashboardBatchAPIKeysUsageCache.Clear()
	dashboardTrendCache.Clear()
	dashboardAPIKeysTrendCache.Clear()
	dashboardUsersTrendCache.Clear()
	dashboardModelStatsCache.Clear()
	dashboardGroupStatsCache.Clear()
}

func parseRankingLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 12
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func parseAdminTokenLeaderboardLimit(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return 10, true
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	switch limit {
	case 10, 20, 50, 100:
		return limit, true
	default:
		return 0, false
	}
}

func parseAdminTokenLeaderboardFilters(c *gin.Context) (usagestats.AdminTokenLeaderboardFilters, string) {
	limit, ok := parseAdminTokenLeaderboardLimit(c.DefaultQuery("limit", "10"))
	if !ok {
		return usagestats.AdminTokenLeaderboardFilters{}, "Invalid limit, use 10/20/50/100"
	}

	modelSource := strings.TrimSpace(c.DefaultQuery("model_source", usagestats.ModelSourceRequested))
	if !usagestats.IsValidModelSource(modelSource) {
		return usagestats.AdminTokenLeaderboardFilters{}, "Invalid model_source, use requested/upstream/mapping"
	}

	status := strings.TrimSpace(c.Query("user_status"))
	if status != "" && status != "active" && status != "disabled" {
		return usagestats.AdminTokenLeaderboardFilters{}, "Invalid user_status, use active/disabled"
	}

	var groupID int64
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			return usagestats.AdminTokenLeaderboardFilters{}, "Invalid group_id"
		}
		groupID = id
	}

	return usagestats.AdminTokenLeaderboardFilters{
		Email:      strings.TrimSpace(c.Query("email")),
		GroupID:    groupID,
		Model:      strings.TrimSpace(c.Query("model")),
		ModelType:  modelSource,
		UserStatus: status,
		Limit:      limit,
	}, ""
}

type AdminTokenLeaderboardGrantBalanceRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
	Amount  float64 `json:"amount" binding:"required,gt=0"`
	Notes   string  `json:"notes"`
}

type AdminTokenLeaderboardGrantBalanceResult struct {
	UserID        int64   `json:"user_id"`
	Email         string  `json:"email"`
	Username      string  `json:"username"`
	Balance       float64 `json:"balance"`
	GrantedAmount float64 `json:"granted_amount"`
}

const defaultAdminTokenLeaderboardGrantNotes = "Token leaderboard Top10 grant"

func validateAdminTokenLeaderboardGrantRequest(req AdminTokenLeaderboardGrantBalanceRequest) ([]int64, string) {
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 {
		return nil, "amount must be a finite number greater than 0"
	}
	if len(req.UserIDs) == 0 {
		return nil, "user_ids is required"
	}
	if len(req.UserIDs) > 10 {
		return nil, "user_ids cannot contain more than 10 users"
	}

	seen := make(map[int64]struct{}, len(req.UserIDs))
	cleaned := make([]int64, 0, len(req.UserIDs))
	for _, id := range req.UserIDs {
		if id <= 0 {
			return nil, "user_ids must only contain positive user IDs"
		}
		if _, ok := seen[id]; ok {
			return nil, "user_ids cannot contain duplicate user IDs"
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	return cleaned, ""
}

func selectAdminTokenLeaderboardTop10Rows(userIDs []int64, ranking []usagestats.AdminTokenLeaderboardUser) ([]usagestats.AdminTokenLeaderboardUser, string) {
	top10ByID := make(map[int64]usagestats.AdminTokenLeaderboardUser, len(ranking))
	for _, row := range ranking {
		if row.Rank <= 0 || row.Rank > 10 {
			continue
		}
		top10ByID[row.UserID] = row
	}

	rows := make([]usagestats.AdminTokenLeaderboardUser, 0, len(userIDs))
	missing := make([]int64, 0)
	for _, userID := range userIDs {
		row, ok := top10ByID[userID]
		if !ok {
			missing = append(missing, userID)
			continue
		}
		rows = append(rows, row)
	}
	if len(missing) > 0 {
		return nil, "selected users must be in the current Top10: " + formatInt64List(missing)
	}
	return rows, ""
}

func formatInt64List(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// GetUserSpendingRanking handles getting user spending ranking data.
// GET /api/v1/admin/dashboard/users-ranking
func (h *DashboardHandler) GetUserSpendingRanking(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	limit := parseRankingLimit(c.DefaultQuery("limit", "12"))

	keyRaw, _ := json.Marshal(struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Limit int    `json:"limit"`
	}{
		Start: startTime.UTC().Format(time.RFC3339),
		End:   endTime.UTC().Format(time.RFC3339),
		Limit: limit,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardUsersRankingCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	ranking, err := h.dashboardService.GetUserSpendingRanking(c.Request.Context(), startTime, endTime, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get user spending ranking")
		return
	}

	payload := gin.H{
		"ranking":           ranking.Ranking,
		"total_actual_cost": ranking.TotalActualCost,
		"total_requests":    ranking.TotalRequests,
		"total_tokens":      ranking.TotalTokens,
		"start_date":        startTime.Format("2006-01-02"),
		"end_date":          endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	}
	dashboardUsersRankingCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// GetAdminTokenLeaderboard handles the admin-only Token leaderboard.
// GET /api/v1/admin/dashboard/token-leaderboard
func (h *DashboardHandler) GetAdminTokenLeaderboard(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	filters, msg := parseAdminTokenLeaderboardFilters(c)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	leaderboard, err := h.dashboardService.GetAdminTokenLeaderboard(c.Request.Context(), startTime, endTime, filters)
	if err != nil {
		response.Error(c, 500, "Failed to get admin token leaderboard")
		return
	}

	response.Success(c, gin.H{
		"ranking":            leaderboard.Ranking,
		"total_requests":     leaderboard.TotalRequests,
		"total_tokens":       leaderboard.TotalTokens,
		"total_cost":         leaderboard.TotalCost,
		"total_actual_cost":  leaderboard.TotalActualCost,
		"total_account_cost": leaderboard.TotalAccountCost,
		"start_date":         startTime.Format("2006-01-02"),
		"end_date":           endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"limit":              filters.Limit,
	})
}

// GrantAdminTokenLeaderboardBalance grants balance to selected users from the current Top10 leaderboard.
// POST /api/v1/admin/dashboard/token-leaderboard/grant-balance
func (h *DashboardHandler) GrantAdminTokenLeaderboardBalance(c *gin.Context) {
	if h.adminService == nil {
		response.InternalError(c, "Admin service not available")
		return
	}

	startTime, endTime := parseTimeRange(c)
	filters, msg := parseAdminTokenLeaderboardFilters(c)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	var req AdminTokenLeaderboardGrantBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	userIDs, msg := validateAdminTokenLeaderboardGrantRequest(req)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	// 赠额只允许针对同一筛选条件下的当前前十名，不能使用页面 TopN 扩展到第 11 名以后。
	filters.Limit = 10
	leaderboard, err := h.dashboardService.GetAdminTokenLeaderboard(c.Request.Context(), startTime, endTime, filters)
	if err != nil {
		response.Error(c, 500, "Failed to validate admin token leaderboard Top10")
		return
	}
	selectedRows, msg := selectAdminTokenLeaderboardTop10Rows(userIDs, leaderboard.Ranking)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = defaultAdminTokenLeaderboardGrantNotes
	}

	grants := make([]service.BalanceGrantInput, 0, len(selectedRows))
	for _, row := range selectedRows {
		grants = append(grants, service.BalanceGrantInput{UserID: row.UserID, Amount: req.Amount})
	}

	grantResults, err := h.adminService.GrantUserBalances(c.Request.Context(), grants, notes)
	if err != nil {
		response.Error(c, 500, "Failed to grant balance to selected users")
		return
	}

	rowsByID := make(map[int64]usagestats.AdminTokenLeaderboardUser, len(selectedRows))
	for _, row := range selectedRows {
		rowsByID[row.UserID] = row
	}
	results := make([]AdminTokenLeaderboardGrantBalanceResult, 0, len(grantResults))
	for _, grantResult := range grantResults {
		if grantResult.User == nil {
			continue
		}
		result := AdminTokenLeaderboardGrantBalanceResult{
			UserID:        grantResult.User.ID,
			Email:         grantResult.User.Email,
			Username:      grantResult.User.Username,
			Balance:       grantResult.User.Balance,
			GrantedAmount: req.Amount,
		}
		if result.Email == "" || result.Username == "" {
			if row, ok := rowsByID[grantResult.User.ID]; ok {
				if result.Email == "" {
					result.Email = row.Email
				}
				if result.Username == "" {
					result.Username = row.Username
				}
			}
		}
		results = append(results, result)
	}

	response.Success(c, gin.H{
		"granted_count": len(results),
		"amount":        req.Amount,
		"users":         results,
	})
}

// GetAdminTokenLeaderboardUserDetails handles admin-only per-user leaderboard details.
// GET /api/v1/admin/dashboard/token-leaderboard/users/:user_id/details
func (h *DashboardHandler) GetAdminTokenLeaderboardUserDetails(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	filters, msg := parseAdminTokenLeaderboardFilters(c)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}

	details, err := h.dashboardService.GetAdminTokenLeaderboardUserDetails(c.Request.Context(), startTime, endTime, userID, filters)
	if err != nil {
		response.Error(c, 500, "Failed to get admin token leaderboard user details")
		return
	}

	response.Success(c, gin.H{
		"api_keys":                  details.APIKeys,
		"calibration_balance_delta": details.CalibrationBalanceDelta,
		"calibration_tokens":        details.CalibrationTokens,
		"groups":                    details.Groups,
		"models":                    details.Models,
		"start_date":                startTime.Format("2006-01-02"),
		"end_date":                  endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"user_id":                   userID,
	})
}

// GetBatchUsersUsage handles getting usage stats for multiple users
// POST /api/v1/admin/dashboard/users-usage
func (h *DashboardHandler) GetBatchUsersUsage(c *gin.Context) {
	var req BatchUsersUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	userIDs := normalizeInt64IDList(req.UserIDs)
	if len(userIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	// cacheKey 必须包含当日日期，否则跨午夜后 30s 内会复用昨天的 "today_*" 结果。
	keyRaw, _ := json.Marshal(struct {
		V       int     `json:"v"`
		Day     string  `json:"day"`
		UserIDs []int64 `json:"user_ids"`
	}{
		V:       2, // bump 当响应结构变化（如加入 by_platform 时）
		Day:     timezone.Today().Format("2006-01-02"),
		UserIDs: userIDs,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardBatchUsersUsageCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.dashboardService.GetBatchUserUsageStats(c.Request.Context(), userIDs, time.Time{}, time.Time{})
	if err != nil {
		response.Error(c, 500, "Failed to get user usage stats")
		return
	}

	payload := gin.H{"stats": stats}
	dashboardBatchUsersUsageCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// BatchAPIKeysUsageRequest represents the request body for batch api key usage stats
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// GetBatchAPIKeysUsage handles getting usage stats for multiple API keys
// POST /api/v1/admin/dashboard/api-keys-usage
func (h *DashboardHandler) GetBatchAPIKeysUsage(c *gin.Context) {
	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	apiKeyIDs := normalizeInt64IDList(req.APIKeyIDs)
	if len(apiKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	keyRaw, _ := json.Marshal(struct {
		APIKeyIDs []int64 `json:"api_key_ids"`
	}{
		APIKeyIDs: apiKeyIDs,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardBatchAPIKeysUsageCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.dashboardService.GetBatchAPIKeyUsageStats(c.Request.Context(), apiKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.Error(c, 500, "Failed to get API key usage stats")
		return
	}

	payload := gin.H{"stats": stats}
	dashboardBatchAPIKeysUsageCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// GetUserBreakdown handles getting per-user usage breakdown within a dimension.
// GET /api/v1/admin/dashboard/user-breakdown
// Query params: start_date, end_date, group_id, model, endpoint, endpoint_type, limit
func (h *DashboardHandler) GetUserBreakdown(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	dim := usagestats.UserBreakdownDimension{}
	if v := c.Query("group_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.GroupID = id
		}
	}
	dim.Model = c.Query("model")
	rawModelSource := strings.TrimSpace(c.DefaultQuery("model_source", usagestats.ModelSourceRequested))
	if !usagestats.IsValidModelSource(rawModelSource) {
		response.BadRequest(c, "Invalid model_source, use requested/upstream/mapping")
		return
	}
	dim.ModelType = rawModelSource
	dim.Endpoint = c.Query("endpoint")
	dim.EndpointType = c.DefaultQuery("endpoint_type", "inbound")

	// Additional filter conditions
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.UserID = id
		}
	}
	if v := c.Query("api_key_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.APIKeyID = id
		}
	}
	if v := c.Query("account_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.AccountID = id
		}
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		dim.RequestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		streamVal, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		dim.Stream = &streamVal
	}
	if v := c.Query("billing_type"); v != "" {
		if bt, err := strconv.ParseInt(v, 10, 8); err == nil {
			btVal := int8(bt)
			dim.BillingType = &btVal
		}
	}

	// sort_by 由 repo 层 allowlist 校验;非法值静默回退默认排序(actual_cost)。
	dim.SortBy = strings.TrimSpace(c.Query("sort_by"))

	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	stats, err := h.dashboardService.GetUserBreakdownStats(
		c.Request.Context(), startTime, endTime, dim, limit,
	)
	if err != nil {
		response.Error(c, 500, "Failed to get user breakdown stats")
		return
	}

	response.Success(c, gin.H{
		"users":      stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}
