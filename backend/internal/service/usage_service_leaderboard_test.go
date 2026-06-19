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

func TestUsageServiceGetUserTokenLeaderboardMasksEmails(t *testing.T) {
	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)
	repo := &usageLeaderboardRepoStub{
		rows: &usagestats.UserTokenLeaderboardRows{
			Ranking: []usagestats.UserTokenLeaderboardRow{
				{Rank: 1, UserID: 1, Email: "alpha@example.com", Requests: 10, Tokens: 1000},
				{Rank: 2, UserID: 2, Email: "beta@example.com", Requests: 9, Tokens: 900},
			},
			MyRank: &usagestats.UserTokenLeaderboardRow{
				Rank:     11,
				UserID:   currentUserID,
				Email:    "current@example.com",
				Requests: 3,
				Tokens:   120,
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
	require.Equal(t, "a***a@example.com", got.Ranking[0].MaskedEmail)
	require.False(t, got.Ranking[0].IsCurrentUser)
	require.Equal(t, int64(11), got.MyRank.Rank)
	require.Equal(t, "c***t@example.com", got.MyRank.MaskedEmail)
	require.True(t, got.MyRank.IsCurrentUser)

	payload, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "alpha@example.com")
	require.NotContains(t, string(payload), "current@example.com")
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
	require.Equal(t, "n***e@example.com", got.MyRank.MaskedEmail)
	require.True(t, got.MyRank.IsCurrentUser)
}
