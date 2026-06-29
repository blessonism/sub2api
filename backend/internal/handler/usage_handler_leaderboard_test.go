package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type leaderboardUsageRepoStub struct {
	service.UsageLogRepository

	rows          *usagestats.UserTokenLeaderboardRows
	called        bool
	startTime     time.Time
	endTime       time.Time
	limit         int
	currentUserID int64
}

func (s *leaderboardUsageRepoStub) GetUserTokenLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) (*usagestats.UserTokenLeaderboardRows, error) {
	s.called = true
	s.startTime = startTime
	s.endTime = endTime
	s.limit = limit
	s.currentUserID = currentUserID
	return s.rows, nil
}

type leaderboardSettingRepoStub struct {
	values map[string]string
}

func (s *leaderboardSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, errors.New("setting not found")
	}
	return &service.Setting{Key: key, Value: value}, nil
}

func (s *leaderboardSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", errors.New("setting not found")
	}
	return value, nil
}

func (s *leaderboardSettingRepoStub) Set(ctx context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *leaderboardSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *leaderboardSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *leaderboardSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *leaderboardSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func newLeaderboardTestRouter(usageRepo *leaderboardUsageRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(usageRepo, nil, nil, nil)
	settingSvc := service.NewSettingService(&leaderboardSettingRepoStub{values: map[string]string{
		service.SettingKeyTokenLeaderboardUserVisible: "true",
		service.SettingKeyTokenLeaderboardTierTooltip: "按最近 Token 用量匹配阶梯倍率",
	}}, &config.Config{})
	handler := NewUsageHandler(usageSvc, nil, nil, settingSvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/usage/dashboard/leaderboard", handler.DashboardLeaderboard)
	return router
}

func TestDashboardLeaderboardReturnsMaskedEmailsOnly(t *testing.T) {
	currentUserID := int64(9)
	usageRepo := &leaderboardUsageRepoStub{
		rows: &usagestats.UserTokenLeaderboardRows{
			Ranking: []usagestats.UserTokenLeaderboardRow{
				{Rank: 1, UserID: 1, Email: "alpha@example.com", Requests: 10, Tokens: 1000, DiscountRateMultiplier: ptrLeaderboardRateForHandler(0.8)},
			},
			MyRank: &usagestats.UserTokenLeaderboardRow{
				Rank:                   12,
				UserID:                 currentUserID,
				Email:                  "current@example.com",
				Requests:               2,
				Tokens:                 80,
				DiscountRateMultiplier: ptrLeaderboardRateForHandler(0.7),
			},
		},
	}
	router := newLeaderboardTestRouter(usageRepo, currentUserID)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/leaderboard?timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.called)
	require.Equal(t, 10, usageRepo.limit)
	require.Equal(t, currentUserID, usageRepo.currentUserID)
	require.NotContains(t, rec.Body.String(), "alpha@example.com")
	require.NotContains(t, rec.Body.String(), "current@example.com")

	var got struct {
		Code int                                     `json:"code"`
		Data usagestats.UserTokenLeaderboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, 10, got.Data.Limit)
	require.Equal(t, "day", got.Data.Period)
	require.Equal(t, "alp***ha@example.com", got.Data.Ranking[0].MaskedEmail)
	require.Equal(t, ptrLeaderboardRateForHandler(0.8), got.Data.Ranking[0].DiscountRateMultiplier)
	require.Equal(t, "cur***nt@example.com", got.Data.MyRank.MaskedEmail)
	require.Equal(t, ptrLeaderboardRateForHandler(0.7), got.Data.MyRank.DiscountRateMultiplier)
	require.Equal(t, int64(12), got.Data.MyRank.Rank)
	require.True(t, got.Data.MyRank.IsCurrentUser)
	require.Equal(t, "按最近 Token 用量匹配阶梯倍率", got.Data.TierTooltip)
}

func ptrLeaderboardRateForHandler(v float64) *float64 {
	return &v
}

func TestDashboardLeaderboardSupportsWeekPeriod(t *testing.T) {
	currentUserID := int64(9)
	usageRepo := &leaderboardUsageRepoStub{rows: &usagestats.UserTokenLeaderboardRows{}}
	router := newLeaderboardTestRouter(usageRepo, currentUserID)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/leaderboard?timezone=UTC&period=week", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.called)
	require.Equal(t, 7, int(usageRepo.endTime.Sub(usageRepo.startTime).Hours()/24))
	require.Equal(t, time.Monday, usageRepo.startTime.Weekday())
	require.Equal(t, time.Monday, usageRepo.endTime.Weekday())
	require.Equal(t, 0, usageRepo.startTime.Hour())
	require.Equal(t, 0, usageRepo.endTime.Hour())

	var got struct {
		Code int                                     `json:"code"`
		Data usagestats.UserTokenLeaderboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "week", got.Data.Period)
}

func TestDashboardLeaderboardSupportsLast7DaysPeriod(t *testing.T) {
	currentUserID := int64(9)
	usageRepo := &leaderboardUsageRepoStub{rows: &usagestats.UserTokenLeaderboardRows{}}
	router := newLeaderboardTestRouter(usageRepo, currentUserID)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/leaderboard?timezone=UTC&period=last7d", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.called)
	require.Equal(t, 7, int(usageRepo.endTime.Sub(usageRepo.startTime).Hours()/24))
	require.Equal(t, 0, usageRepo.startTime.Hour())
	require.Equal(t, 0, usageRepo.endTime.Hour())

	var got struct {
		Code int                                     `json:"code"`
		Data usagestats.UserTokenLeaderboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "last7d", got.Data.Period)
}

func TestDashboardLeaderboardRejectsInvalidPeriod(t *testing.T) {
	usageRepo := &leaderboardUsageRepoStub{rows: &usagestats.UserTokenLeaderboardRows{}}
	router := newLeaderboardTestRouter(usageRepo, 9)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/leaderboard?period=month", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, usageRepo.called)
	require.Contains(t, rec.Body.String(), "day/week/last7d")
}
