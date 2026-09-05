package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

var (
	ErrUsageLogNotFound = infraerrors.NotFound("USAGE_LOG_NOT_FOUND", "usage log not found")
)

const userTokenLeaderboardLimit = 10

// CreateUsageLogRequest 创建使用日志请求
type CreateUsageLogRequest struct {
	UserID                int64   `json:"user_id"`
	APIKeyID              int64   `json:"api_key_id"`
	AccountID             int64   `json:"account_id"`
	RequestID             string  `json:"request_id"`
	Model                 string  `json:"model"`
	InputTokens           int     `json:"input_tokens"`
	OutputTokens          int     `json:"output_tokens"`
	CacheCreationTokens   int     `json:"cache_creation_tokens"`
	CacheReadTokens       int     `json:"cache_read_tokens"`
	CacheCreation5mTokens int     `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int     `json:"cache_creation_1h_tokens"`
	InputCost             float64 `json:"input_cost"`
	OutputCost            float64 `json:"output_cost"`
	CacheCreationCost     float64 `json:"cache_creation_cost"`
	CacheReadCost         float64 `json:"cache_read_cost"`
	TotalCost             float64 `json:"total_cost"`
	ActualCost            float64 `json:"actual_cost"`
	RateMultiplier        float64 `json:"rate_multiplier"`
	Stream                bool    `json:"stream"`
	DurationMs            *int    `json:"duration_ms"`
}

// UsageStats 使用统计
type UsageStats struct {
	TotalRequests            int64   `json:"total_requests"`
	TotalInputTokens         int64   `json:"total_input_tokens"`
	TotalOutputTokens        int64   `json:"total_output_tokens"`
	TotalCacheTokens         int64   `json:"total_cache_tokens"`
	TotalCacheCreationTokens int64   `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64   `json:"total_cache_read_tokens"`
	CalibrationTokens        int64   `json:"calibration_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	TotalCost                float64 `json:"total_cost"`
	TotalActualCost          float64 `json:"total_actual_cost"`
	AverageDurationMs        float64 `json:"average_duration_ms"`
}

// UsageService 使用统计服务
type UsageService struct {
	usageRepo            UsageLogRepository
	userRepo             UserRepository
	entClient            *dbent.Client
	authCacheInvalidator APIKeyAuthCacheInvalidator
	calibrationRepo      AdminUsageCalibrationRepository
}

// NewUsageService 创建使用统计服务实例
func NewUsageService(usageRepo UsageLogRepository, userRepo UserRepository, entClient *dbent.Client, authCacheInvalidator APIKeyAuthCacheInvalidator) *UsageService {
	return &UsageService{
		usageRepo:            usageRepo,
		userRepo:             userRepo,
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
	}
}

func (s *UsageService) SetAdminUsageCalibrationRepository(repo AdminUsageCalibrationRepository) {
	s.calibrationRepo = repo
}

// Create 创建使用日志
func (s *UsageService) Create(ctx context.Context, req CreateUsageLogRequest) (*UsageLog, error) {
	// 使用数据库事务保证「使用日志插入」与「扣费」的原子性，避免重复扣费或漏扣风险。
	tx, err := s.entClient.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := ctx
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txCtx = dbent.NewTxContext(ctx, tx)
	}

	// 验证用户存在
	_, err = s.userRepo.GetByID(txCtx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 创建使用日志
	usageLog := &UsageLog{
		UserID:                req.UserID,
		APIKeyID:              req.APIKeyID,
		AccountID:             req.AccountID,
		RequestID:             req.RequestID,
		Model:                 req.Model,
		InputTokens:           req.InputTokens,
		OutputTokens:          req.OutputTokens,
		CacheCreationTokens:   req.CacheCreationTokens,
		CacheReadTokens:       req.CacheReadTokens,
		CacheCreation5mTokens: req.CacheCreation5mTokens,
		CacheCreation1hTokens: req.CacheCreation1hTokens,
		InputCost:             req.InputCost,
		OutputCost:            req.OutputCost,
		CacheCreationCost:     req.CacheCreationCost,
		CacheReadCost:         req.CacheReadCost,
		TotalCost:             req.TotalCost,
		ActualCost:            req.ActualCost,
		RateMultiplier:        req.RateMultiplier,
		Stream:                req.Stream,
		DurationMs:            req.DurationMs,
	}

	inserted, err := s.usageRepo.Create(txCtx, usageLog)
	if err != nil {
		return nil, fmt.Errorf("create usage log: %w", err)
	}

	// 扣除用户余额
	balanceUpdated := false
	if inserted && req.ActualCost > 0 {
		if err := s.userRepo.UpdateBalance(txCtx, req.UserID, -req.ActualCost); err != nil {
			return nil, fmt.Errorf("update user balance: %w", err)
		}
		balanceUpdated = true
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit transaction: %w", err)
		}
	}

	s.invalidateUsageCaches(ctx, req.UserID, balanceUpdated)

	return usageLog, nil
}

func (s *UsageService) invalidateUsageCaches(ctx context.Context, userID int64, balanceUpdated bool) {
	if !balanceUpdated || s.authCacheInvalidator == nil {
		return
	}
	s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
}

// GetByID 根据ID获取使用日志
func (s *UsageService) GetByID(ctx context.Context, id int64) (*UsageLog, error) {
	log, err := s.usageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get usage log: %w", err)
	}
	return log, nil
}

// ListByUser 获取用户的使用日志列表
func (s *UsageService) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	logs, pagination, err := s.usageRepo.ListByUser(ctx, userID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list usage logs: %w", err)
	}
	return logs, pagination, nil
}

// ListByAPIKey 获取API Key的使用日志列表
func (s *UsageService) ListByAPIKey(ctx context.Context, apiKeyID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	logs, pagination, err := s.usageRepo.ListByAPIKey(ctx, apiKeyID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list usage logs: %w", err)
	}
	return logs, pagination, nil
}

// ListByAccount 获取账号的使用日志列表
func (s *UsageService) ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	logs, pagination, err := s.usageRepo.ListByAccount(ctx, accountID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list usage logs: %w", err)
	}
	return logs, pagination, nil
}

// GetStatsByUser 获取用户的使用统计
func (s *UsageService) GetStatsByUser(ctx context.Context, userID int64, startTime, endTime time.Time) (*UsageStats, error) {
	stats, err := s.usageRepo.GetUserStatsAggregated(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get user stats: %w", err)
	}

	out := &UsageStats{
		TotalRequests:            stats.TotalRequests,
		TotalInputTokens:         stats.TotalInputTokens,
		TotalOutputTokens:        stats.TotalOutputTokens,
		TotalCacheTokens:         stats.TotalCacheTokens,
		TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens:     stats.TotalCacheReadTokens,
		CalibrationTokens:        stats.CalibrationTokens,
		TotalTokens:              stats.TotalTokens,
		TotalCost:                stats.TotalCost,
		TotalActualCost:          stats.TotalActualCost,
		AverageDurationMs:        stats.AverageDurationMs,
	}
	if err := s.applyCalibrationToUsageStats(ctx, userID, startTime, endTime, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStatsByAPIKey 获取API Key的使用统计
func (s *UsageService) GetStatsByAPIKey(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) (*UsageStats, error) {
	stats, err := s.usageRepo.GetAPIKeyStatsAggregated(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get api key stats: %w", err)
	}

	return &UsageStats{
		TotalRequests:            stats.TotalRequests,
		TotalInputTokens:         stats.TotalInputTokens,
		TotalOutputTokens:        stats.TotalOutputTokens,
		TotalCacheTokens:         stats.TotalCacheTokens,
		TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens:     stats.TotalCacheReadTokens,
		TotalTokens:              stats.TotalTokens,
		TotalCost:                stats.TotalCost,
		TotalActualCost:          stats.TotalActualCost,
		AverageDurationMs:        stats.AverageDurationMs,
	}, nil
}

// GetStatsByAccount 获取账号的使用统计
func (s *UsageService) GetStatsByAccount(ctx context.Context, accountID int64, startTime, endTime time.Time) (*UsageStats, error) {
	stats, err := s.usageRepo.GetAccountStatsAggregated(ctx, accountID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get account stats: %w", err)
	}

	return &UsageStats{
		TotalRequests:            stats.TotalRequests,
		TotalInputTokens:         stats.TotalInputTokens,
		TotalOutputTokens:        stats.TotalOutputTokens,
		TotalCacheTokens:         stats.TotalCacheTokens,
		TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens:     stats.TotalCacheReadTokens,
		TotalTokens:              stats.TotalTokens,
		TotalCost:                stats.TotalCost,
		TotalActualCost:          stats.TotalActualCost,
		AverageDurationMs:        stats.AverageDurationMs,
	}, nil
}

// GetStatsByModel 获取模型的使用统计
func (s *UsageService) GetStatsByModel(ctx context.Context, modelName string, startTime, endTime time.Time) (*UsageStats, error) {
	stats, err := s.usageRepo.GetModelStatsAggregated(ctx, modelName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get model stats: %w", err)
	}

	return &UsageStats{
		TotalRequests:            stats.TotalRequests,
		TotalInputTokens:         stats.TotalInputTokens,
		TotalOutputTokens:        stats.TotalOutputTokens,
		TotalCacheTokens:         stats.TotalCacheTokens,
		TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens:     stats.TotalCacheReadTokens,
		TotalTokens:              stats.TotalTokens,
		TotalCost:                stats.TotalCost,
		TotalActualCost:          stats.TotalActualCost,
		AverageDurationMs:        stats.AverageDurationMs,
	}, nil
}

// GetDailyStats 获取每日使用统计（最近N天）
func (s *UsageService) GetDailyStats(ctx context.Context, userID int64, days int) ([]map[string]any, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	stats, err := s.usageRepo.GetDailyStatsAggregated(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get daily stats: %w", err)
	}

	return stats, nil
}

// Delete 删除使用日志（管理员功能，谨慎使用）
func (s *UsageService) Delete(ctx context.Context, id int64) error {
	if err := s.usageRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete usage log: %w", err)
	}
	return nil
}

// GetUserDashboardStats returns per-user dashboard summary stats.
func (s *UsageService) GetUserDashboardStats(ctx context.Context, userID int64) (*usagestats.UserDashboardStats, error) {
	stats, err := s.usageRepo.GetUserDashboardStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user dashboard stats: %w", err)
	}
	if err := s.applyCalibrationToUserDashboardStats(ctx, userID, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetAPIKeyDashboardStats returns dashboard summary stats filtered by API Key.
func (s *UsageService) GetAPIKeyDashboardStats(ctx context.Context, apiKeyID int64) (*usagestats.UserDashboardStats, error) {
	stats, err := s.usageRepo.GetAPIKeyDashboardStats(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("get api key dashboard stats: %w", err)
	}
	return stats, nil
}

// GetUserUsageTrendByUserID returns per-user usage trend.
func (s *UsageService) GetUserUsageTrendByUserID(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string) ([]usagestats.TrendDataPoint, error) {
	trend, err := s.usageRepo.GetUserUsageTrendByUserID(ctx, userID, startTime, endTime, granularity)
	if err != nil {
		return nil, fmt.Errorf("get user usage trend: %w", err)
	}
	if err := s.applyTokenCalibrationToTrend(ctx, userID, startTime, endTime, granularity, &trend); err != nil {
		return nil, err
	}
	return trend, nil
}

// GetUsageTrendWithFilters returns trend data using the shared usage filter shape.
func (s *UsageService) GetUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters) ([]usagestats.TrendDataPoint, error) {
	type usageTrendWithFiltersRepo interface {
		GetUsageTrendWithUsageFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters) ([]usagestats.TrendDataPoint, error)
	}
	if filterRepo, ok := s.usageRepo.(usageTrendWithFiltersRepo); ok {
		trend, err := filterRepo.GetUsageTrendWithUsageFilters(ctx, startTime, endTime, granularity, filters)
		if err != nil {
			return nil, fmt.Errorf("get usage trend with filters: %w", err)
		}
		return trend, nil
	}
	trend, err := s.usageRepo.GetUsageTrendWithFilters(ctx, startTime, endTime, granularity, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.Model, filters.RequestType, filters.Stream, filters.BillingType)
	if err != nil {
		return nil, fmt.Errorf("get usage trend with filters: %w", err)
	}
	return trend, nil
}

// GetUserModelStats returns per-user model usage stats.
func (s *UsageService) GetUserModelStats(ctx context.Context, userID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error) {
	stats, err := s.usageRepo.GetUserModelStats(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get user model stats: %w", err)
	}
	return stats, nil
}

// GetUserTokenLeaderboard 返回普通用户可见的 Token 排行榜，并在服务层完成邮箱脱敏。
func (s *UsageService) GetUserTokenLeaderboard(ctx context.Context, userID int64, startTime, endTime time.Time, period string) (*usagestats.UserTokenLeaderboardResponse, error) {
	rows, err := s.usageRepo.GetUserTokenLeaderboard(ctx, startTime, endTime, userTokenLeaderboardLimit, userID)
	if err != nil {
		return nil, fmt.Errorf("get user token leaderboard: %w", err)
	}
	if rows == nil {
		rows = &usagestats.UserTokenLeaderboardRows{}
	}

	ranking := make([]usagestats.UserTokenLeaderboardItem, 0, len(rows.Ranking))
	for _, row := range rows.Ranking {
		ranking = append(ranking, userTokenLeaderboardPublicItem(row, userID))
	}

	myRank := usagestats.UserTokenLeaderboardItem{
		Rank:          0,
		MaskedEmail:   "***",
		Requests:      0,
		Tokens:        0,
		IsCurrentUser: true,
	}
	if rows.MyRank != nil {
		myRank = userTokenLeaderboardPublicItem(*rows.MyRank, userID)
		myRank.IsCurrentUser = true
	} else if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get current user: %w", err)
		}
		myRank.MaskedEmail = maskUserTokenLeaderboardEmail(user.Email)
	}

	return &usagestats.UserTokenLeaderboardResponse{
		Ranking:   ranking,
		MyRank:    myRank,
		StartDate: startTime.Format("2006-01-02"),
		EndDate:   endTime.AddDate(0, 0, -1).Format("2006-01-02"),
		Limit:     userTokenLeaderboardLimit,
		Period:    period,
	}, nil
}

func userTokenLeaderboardPublicItem(row usagestats.UserTokenLeaderboardRow, currentUserID int64) usagestats.UserTokenLeaderboardItem {
	return usagestats.UserTokenLeaderboardItem{
		Rank:                   row.Rank,
		MaskedEmail:            maskUserTokenLeaderboardEmail(row.Email),
		Requests:               row.Requests,
		Tokens:                 row.Tokens,
		DiscountRateMultiplier: normalizeLeaderboardRateMultiplier(row.DiscountRateMultiplier),
		IsCurrentUser:          row.UserID == currentUserID,
	}
}

// maskUserTokenLeaderboardEmail 仅用于用户侧排行榜：保留用户名前 3 位和 @ 前最后 2 位。
func maskUserTokenLeaderboardEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}

	atIdx := -1
	for i, c := range email {
		if c == '@' {
			atIdx = i
			break
		}
	}

	if atIdx == -1 || atIdx < 1 {
		emailRunes := []rune(email)
		if len(emailRunes) == 0 {
			return "***"
		}
		return string(emailRunes[:1]) + "***"
	}

	localPart := email[:atIdx]
	domain := email[atIdx:]
	localRunes := []rune(localPart)
	if len(localRunes) <= 2 {
		return string(localRunes[:1]) + "***" + domain
	}
	return string(localRunes[:3]) + "***" + string(localRunes[len(localRunes)-2:]) + domain
}

func normalizeLeaderboardRateMultiplier(value *float64) *float64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return value
}

// GetModelStatsWithFiltersBySource returns model stats using the shared usage filter shape.
func (s *UsageService) GetModelStatsWithFiltersBySource(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, modelSource string) ([]usagestats.ModelStat, error) {
	normalizedSource := usagestats.NormalizeModelSource(modelSource)
	type modelStatsWithUsageFiltersRepo interface {
		GetModelStatsWithUsageFiltersBySource(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, source string) ([]usagestats.ModelStat, error)
	}
	if filterRepo, ok := s.usageRepo.(modelStatsWithUsageFiltersRepo); ok {
		stats, err := filterRepo.GetModelStatsWithUsageFiltersBySource(ctx, startTime, endTime, filters, normalizedSource)
		if err != nil {
			return nil, fmt.Errorf("get model stats with filters by source: %w", err)
		}
		return stats, nil
	}
	type modelStatsBySourceRepo interface {
		GetModelStatsWithFiltersBySource(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, source string) ([]usagestats.ModelStat, error)
	}
	if sourceRepo, ok := s.usageRepo.(modelStatsBySourceRepo); ok {
		stats, err := sourceRepo.GetModelStatsWithFiltersBySource(ctx, startTime, endTime, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.RequestType, filters.Stream, filters.BillingType, normalizedSource)
		if err != nil {
			return nil, fmt.Errorf("get model stats with filters by source: %w", err)
		}
		return stats, nil
	}
	stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, startTime, endTime, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.RequestType, filters.Stream, filters.BillingType)
	if err != nil {
		return nil, fmt.Errorf("get model stats with filters: %w", err)
	}
	return stats, nil
}

// GetGroupStatsWithFilters returns group stats using the shared usage filter shape.
func (s *UsageService) GetGroupStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters) ([]usagestats.GroupStat, error) {
	type groupStatsWithUsageFiltersRepo interface {
		GetGroupStatsWithUsageFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters) ([]usagestats.GroupStat, error)
	}
	if filterRepo, ok := s.usageRepo.(groupStatsWithUsageFiltersRepo); ok {
		stats, err := filterRepo.GetGroupStatsWithUsageFilters(ctx, startTime, endTime, filters)
		if err != nil {
			return nil, fmt.Errorf("get group stats with filters: %w", err)
		}
		return stats, nil
	}
	stats, err := s.usageRepo.GetGroupStatsWithFilters(ctx, startTime, endTime, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.RequestType, filters.Stream, filters.BillingType)
	if err != nil {
		return nil, fmt.Errorf("get group stats with filters: %w", err)
	}
	return stats, nil
}

// GetAPIKeyModelStats returns per-model usage stats for a specific API Key.
func (s *UsageService) GetAPIKeyModelStats(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error) {
	stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, startTime, endTime, 0, apiKeyID, 0, 0, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get api key model stats: %w", err)
	}
	return stats, nil
}

// GetAPIKeyDailyUsage returns daily usage stats for a user's API key.
func (s *UsageService) GetAPIKeyDailyUsage(ctx context.Context, userID, apiKeyID int64, startTime, endTime time.Time) ([]usagestats.APIKeyDailyUsagePoint, error) {
	trend, err := s.usageRepo.GetUsageTrendWithFilters(ctx, startTime, endTime, "day", userID, apiKeyID, 0, 0, "", nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get api key daily usage: %w", err)
	}

	points := make([]usagestats.APIKeyDailyUsagePoint, 0, len(trend))
	for _, row := range trend {
		points = append(points, usagestats.APIKeyDailyUsagePoint{
			Date:             row.Date,
			Requests:         row.Requests,
			InputTokens:      row.InputTokens,
			OutputTokens:     row.OutputTokens,
			CacheReadTokens:  row.CacheReadTokens,
			CacheWriteTokens: row.CacheCreationTokens,
			TotalTokens:      row.TotalTokens,
			Cost:             row.Cost,
			ActualCost:       row.ActualCost,
		})
	}
	return points, nil
}

// GetBatchAPIKeyUsageStats returns today/total actual_cost for given api keys.
func (s *UsageService) GetBatchAPIKeyUsageStats(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error) {
	stats, err := s.usageRepo.GetBatchAPIKeyUsageStats(ctx, apiKeyIDs, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get batch api key usage stats: %w", err)
	}
	return stats, nil
}

// ListWithFilters lists usage logs with admin filters.
func (s *UsageService) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]UsageLog, *pagination.PaginationResult, error) {
	logs, result, err := s.usageRepo.ListWithFilters(ctx, params, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("list usage logs with filters: %w", err)
	}
	return logs, result, nil
}

func (s *UsageService) GetSharedIPUsersSummary(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.SharedIPUsersSummary, error) {
	summary, err := s.usageRepo.GetSharedIPUsersSummary(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("get shared IP users summary: %w", err)
	}
	return summary, nil
}

// GetGlobalStats returns global usage stats for a time range.
func (s *UsageService) GetGlobalStats(ctx context.Context, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	stats, err := s.usageRepo.GetGlobalStats(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get global usage stats: %w", err)
	}
	return stats, nil
}

// GetStatsWithFilters returns usage stats with optional filters.
func (s *UsageService) GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error) {
	stats, err := s.usageRepo.GetStatsWithFilters(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("get usage stats with filters: %w", err)
	}
	if s.shouldApplyTokenCalibrationToFilters(filters) {
		if err := s.applyCalibrationToFilteredStats(ctx, filters, stats); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

func (s *UsageService) applyCalibrationToUsageStats(ctx context.Context, userID int64, startTime, endTime time.Time, stats *UsageStats) error {
	if stats == nil || s.calibrationRepo == nil || userID <= 0 {
		return nil
	}
	startDate, endDate, ok := tokenAllocationDateRange(startTime, endTime)
	if ok {
		delta, err := s.calibrationRepo.SumTokenAllocations(ctx, userID, startDate, endDate)
		if err != nil {
			return fmt.Errorf("sum token calibrations: %w", err)
		}
		addTokenDeltaToUsageStats(stats, delta)
	}
	return nil
}

func (s *UsageService) applyCalibrationToUserDashboardStats(ctx context.Context, userID int64, stats *usagestats.UserDashboardStats) error {
	if stats == nil || s.calibrationRepo == nil || userID <= 0 {
		return nil
	}
	totalDelta, err := s.calibrationRepo.SumTokenAllocations(ctx, userID, "", "")
	if err != nil {
		return fmt.Errorf("sum total token calibrations: %w", err)
	}
	stats.TotalCalibrationTokens += totalDelta
	stats.TotalTokens += totalDelta

	today := timezone.Today()
	todayDelta, err := s.calibrationRepo.SumTokenAllocations(ctx, userID, today.Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("sum today token calibrations: %w", err)
	}
	stats.TodayCalibrationTokens += todayDelta
	stats.TodayTokens += todayDelta
	return nil
}

func (s *UsageService) applyTokenCalibrationToTrend(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string, trend *[]usagestats.TrendDataPoint) error {
	if trend == nil || s.calibrationRepo == nil || userID <= 0 || granularity != "day" {
		return nil
	}
	startDate, endDate, ok := tokenAllocationDateRange(startTime, endTime)
	if !ok {
		return nil
	}
	allocations, err := s.calibrationRepo.SumTokenAllocationsByDate(ctx, userID, startDate, endDate)
	if err != nil {
		return fmt.Errorf("sum token calibrations by date: %w", err)
	}
	if len(allocations) == 0 {
		return nil
	}
	byDate := make(map[string]int, len(*trend))
	for i := range *trend {
		byDate[(*trend)[i].Date] = i
	}
	for date, delta := range allocations {
		if idx, ok := byDate[date]; ok {
			(*trend)[idx].CalibrationTokens += delta
			(*trend)[idx].TotalTokens += delta
			continue
		}
		*trend = append(*trend, usagestats.TrendDataPoint{
			Date:              date,
			CalibrationTokens: delta,
			TotalTokens:       delta,
		})
	}
	return nil
}

func (s *UsageService) shouldApplyTokenCalibrationToFilters(filters usagestats.UsageLogFilters) bool {
	return filters.APIKeyID == 0 &&
		filters.AccountID == 0 &&
		filters.GroupID == 0 &&
		filters.Model == "" &&
		filters.RequestType == nil &&
		filters.Stream == nil &&
		filters.BillingType == nil &&
		filters.BillingMode == ""
}

func (s *UsageService) applyCalibrationToFilteredStats(ctx context.Context, filters usagestats.UsageLogFilters, stats *usagestats.UsageStats) error {
	if stats == nil || s.calibrationRepo == nil {
		return nil
	}
	start := time.Time{}
	end := time.Time{}
	if filters.StartTime != nil {
		start = *filters.StartTime
	}
	if filters.EndTime != nil {
		end = *filters.EndTime
	}
	startDate, endDate, ok := tokenAllocationDateRange(start, end)
	if ok {
		var (
			delta int64
			err   error
		)
		if filters.UserID > 0 {
			delta, err = s.calibrationRepo.SumTokenAllocations(ctx, filters.UserID, startDate, endDate)
		} else {
			delta, err = s.calibrationRepo.SumAllTokenAllocations(ctx, startDate, endDate)
		}
		if err != nil {
			return fmt.Errorf("sum token calibrations for filtered stats: %w", err)
		}
		stats.CalibrationTokens += delta
		stats.TotalTokens += delta
	}
	return nil
}

func addTokenDeltaToUsageStats(stats *UsageStats, delta int64) {
	if stats == nil || delta == 0 {
		return
	}
	stats.CalibrationTokens += delta
	stats.TotalTokens += delta
}
