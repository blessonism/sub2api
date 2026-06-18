package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	limit         int
	currentUserID int64
}

func (s *leaderboardUsageRepoStub) GetUserTokenLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) (*usagestats.UserTokenLeaderboardRows, error) {
	s.called = true
	s.limit = limit
	s.currentUserID = currentUserID
	return s.rows, nil
}

func newLeaderboardTestRouter(usageRepo *leaderboardUsageRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(usageRepo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
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
				{Rank: 1, UserID: 1, Email: "alpha@example.com", Requests: 10, Tokens: 1000},
			},
			MyRank: &usagestats.UserTokenLeaderboardRow{
				Rank:     12,
				UserID:   currentUserID,
				Email:    "current@example.com",
				Requests: 2,
				Tokens:   80,
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
	require.Equal(t, "a***a@example.com", got.Data.Ranking[0].MaskedEmail)
	require.Equal(t, "c***t@example.com", got.Data.MyRank.MaskedEmail)
	require.Equal(t, int64(12), got.Data.MyRank.Rank)
	require.True(t, got.Data.MyRank.IsCurrentUser)
}
