package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type usageLeaderboardRepoStub struct {
	UsageLogRepository

	rows          *usagestats.UserTokenLeaderboardRows
	startTime     time.Time
	endTime       time.Time
	limit         int
	currentUserID int64
}

func (s *usageLeaderboardRepoStub) GetUserTokenLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) (*usagestats.UserTokenLeaderboardRows, error) {
	s.startTime = startTime
	s.endTime = endTime
	s.limit = limit
	s.currentUserID = currentUserID
	return s.rows, nil
}

type usageLeaderboardUserRepoStub struct {
	UserRepository

	user *User
}

func (s *usageLeaderboardUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	clone := *s.user
	return &clone, nil
}

type usageStatsRepoStub struct {
	UsageLogRepository

	stats          *usagestats.UsageStats
	dashboardStats *usagestats.UserDashboardStats
}

func (s *usageStatsRepoStub) GetUserStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	return s.stats, nil
}

func (s *usageStatsRepoStub) GetUserDashboardStats(ctx context.Context, userID int64) (*usagestats.UserDashboardStats, error) {
	return s.dashboardStats, nil
}

type usageCalibrationRepoStub struct {
	AdminUsageCalibrationRepository

	tokenDelta   int64
	balanceSpent map[string]float64
}

func (s *usageCalibrationRepoStub) SumTokenAllocations(ctx context.Context, userID int64, startDate, endDateExclusive string) (int64, error) {
	return s.tokenDelta, nil
}

func (s *usageCalibrationRepoStub) SumAllTokenAllocations(ctx context.Context, startDate, endDateExclusive string) (int64, error) {
	return s.tokenDelta, nil
}

func (s *usageCalibrationRepoStub) SumTokenAllocationsByDate(ctx context.Context, userID int64, startDate, endDateExclusive string) (map[string]int64, error) {
	return nil, nil
}

func (s *usageCalibrationRepoStub) SumBalanceSpent(ctx context.Context, userID int64, startTime, endTime time.Time) (float64, error) {
	if s.balanceSpent == nil {
		return 0, nil
	}
	if startTime.IsZero() && endTime.IsZero() {
		return s.balanceSpent["total"], nil
	}
	if !startTime.IsZero() && !endTime.IsZero() && endTime.Sub(startTime) == 24*time.Hour {
		return s.balanceSpent["today"], nil
	}
	return s.balanceSpent["range"], nil
}

func (s *usageCalibrationRepoStub) SumBalanceSpentByUsers(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]float64, error) {
	return nil, nil
}

func TestUsageServiceGetUserTokenLeaderboardMasksEmails(t *testing.T) {
	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)
	repo := &usageLeaderboardRepoStub{
		rows: &usagestats.UserTokenLeaderboardRows{
			Ranking: []usagestats.UserTokenLeaderboardRow{
				{Rank: 1, UserID: 1, Email: "alpha@example.com", Requests: 10, Tokens: 1000, DiscountRateMultiplier: ptrLeaderboardRate(0.6)},
				{Rank: 2, UserID: 2, Email: "beta@example.com", Requests: 9, Tokens: 900, DiscountRateMultiplier: nil},
			},
			MyRank: &usagestats.UserTokenLeaderboardRow{
				Rank:                   11,
				UserID:                 currentUserID,
				Email:                  "current@example.com",
				Requests:               3,
				Tokens:                 120,
				DiscountRateMultiplier: ptrLeaderboardRate(0.7),
			},
		},
	}
	svc := NewUsageService(repo, nil, nil, nil)

	got, err := svc.GetUserTokenLeaderboard(context.Background(), currentUserID, start, end, "day")

	require.NoError(t, err)
	require.Equal(t, 10, repo.limit)
	require.Equal(t, 10, got.Limit)
	require.Equal(t, "day", got.Period)
	require.Equal(t, currentUserID, repo.currentUserID)
	require.Equal(t, start, repo.startTime)
	require.Equal(t, end, repo.endTime)
	require.Equal(t, "alp***ha@example.com", got.Ranking[0].MaskedEmail)
	require.Equal(t, ptrLeaderboardRate(0.6), got.Ranking[0].DiscountRateMultiplier)
	require.False(t, got.Ranking[0].IsCurrentUser)
	require.Nil(t, got.Ranking[1].DiscountRateMultiplier)
	require.Equal(t, int64(11), got.MyRank.Rank)
	require.Equal(t, "cur***nt@example.com", got.MyRank.MaskedEmail)
	require.Equal(t, ptrLeaderboardRate(0.7), got.MyRank.DiscountRateMultiplier)
	require.True(t, got.MyRank.IsCurrentUser)

	payload, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "alpha@example.com")
	require.NotContains(t, string(payload), "current@example.com")
}

func TestMaskUserTokenLeaderboardEmail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		email string
		want  string
	}{
		{name: "long local part", email: "current@example.com", want: "cur***nt@example.com"},
		{name: "five char local part", email: "alpha@example.com", want: "alp***ha@example.com"},
		{name: "three char local part", email: "abc@example.com", want: "abc***bc@example.com"},
		{name: "two char local part", email: "ab@example.com", want: "a***@example.com"},
		{name: "one char local part", email: "a@example.com", want: "a***@example.com"},
		{name: "missing at", email: "opaque-id", want: "o***"},
		{name: "unicode local part", email: "用户测试@example.com", want: "用户测***测试@example.com"},
		{name: "empty", email: "", want: "***"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, maskUserTokenLeaderboardEmail(tt.email))
		})
	}
}

func TestUsageServiceGetStatsByUserExcludesBalanceCalibrationFromSpend(t *testing.T) {
	start := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	repo := &usageStatsRepoStub{
		stats: &usagestats.UsageStats{
			TotalTokens:     100,
			TotalActualCost: 1.25,
		},
	}
	calibrationRepo := &usageCalibrationRepoStub{
		balanceSpent: map[string]float64{"range": 2.5},
	}
	svc := NewUsageService(repo, nil, nil, nil)
	svc.SetAdminUsageCalibrationRepository(calibrationRepo)

	got, err := svc.GetStatsByUser(context.Background(), 7, start, end)

	require.NoError(t, err)
	require.Equal(t, int64(100), got.TotalTokens)
	require.InDelta(t, 1.25, got.TotalActualCost, 1e-9)
}

func TestUsageServiceGetUserDashboardStatsExcludesBalanceCalibrationFromSpend(t *testing.T) {
	repo := &usageStatsRepoStub{
		dashboardStats: &usagestats.UserDashboardStats{
			TotalActualCost: 10,
			TodayActualCost: 1,
		},
	}
	calibrationRepo := &usageCalibrationRepoStub{
		balanceSpent: map[string]float64{
			"total": 3.25,
			"today": 0.75,
		},
	}
	svc := NewUsageService(repo, nil, nil, nil)
	svc.SetAdminUsageCalibrationRepository(calibrationRepo)

	got, err := svc.GetUserDashboardStats(context.Background(), 7)

	require.NoError(t, err)
	require.InDelta(t, 10, got.TotalActualCost, 1e-9)
	require.InDelta(t, 1, got.TodayActualCost, 1e-9)
}

func TestUsageServiceGetUserTokenLeaderboardReturnsZeroRankWhenCurrentUserHasNoUsage(t *testing.T) {
	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	repo := &usageLeaderboardRepoStub{rows: &usagestats.UserTokenLeaderboardRows{}}
	userRepo := &usageLeaderboardUserRepoStub{user: &User{ID: 7, Email: "no-usage@example.com"}}
	svc := NewUsageService(repo, userRepo, nil, nil)

	got, err := svc.GetUserTokenLeaderboard(context.Background(), 7, start, end, "week")

	require.NoError(t, err)
	require.Empty(t, got.Ranking)
	require.Equal(t, 10, got.Limit)
	require.Equal(t, "week", got.Period)
	require.Equal(t, int64(0), got.MyRank.Rank)
	require.Equal(t, int64(0), got.MyRank.Requests)
	require.Equal(t, int64(0), got.MyRank.Tokens)
	require.Nil(t, got.MyRank.DiscountRateMultiplier)
	require.Equal(t, "no-***ge@example.com", got.MyRank.MaskedEmail)
	require.True(t, got.MyRank.IsCurrentUser)
}

func ptrLeaderboardRate(v float64) *float64 {
	return &v
}
