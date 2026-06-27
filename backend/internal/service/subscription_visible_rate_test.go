//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type subscriptionVisibleRateRepoStub struct {
	userSubRepoNoop

	subs []UserSubscription
}

func (r *subscriptionVisibleRateRepoStub) ListByUserID(_ context.Context, _ int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, len(r.subs))
	copy(out, r.subs)
	return out, nil
}

func (r *subscriptionVisibleRateRepoStub) ListActiveByUserID(_ context.Context, _ int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, len(r.subs))
	copy(out, r.subs)
	return out, nil
}

type userVisibleGroupRateRepoStub struct {
	userGroupRateRepoStubForGroupRate

	actualRates  map[int64]float64
	visibleRates map[int64]float64
}

func (r *userVisibleGroupRateRepoStub) GetByUserID(_ context.Context, _ int64) (map[int64]float64, error) {
	return r.actualRates, nil
}

func (r *userVisibleGroupRateRepoStub) GetVisibleByUserID(_ context.Context, _ int64) (map[int64]float64, error) {
	return r.visibleRates, nil
}

func TestSubscriptionServiceListUserSubscriptionsUsesVisibleRates(t *testing.T) {
	groupVisible := 0.8
	groupVisibleForActualFallback := 1.4
	subRepo := &subscriptionVisibleRateRepoStub{
		subs: []UserSubscription{
			{
				ID:        1,
				UserID:    10,
				GroupID:   20,
				Status:    SubscriptionStatusActive,
				StartsAt:  time.Now().Add(-time.Hour),
				ExpiresAt: time.Now().Add(time.Hour),
				Group: &Group{
					ID:                    20,
					RateMultiplier:        2,
					VisibleRateMultiplier: &groupVisible,
				},
			},
			{
				ID:        2,
				UserID:    10,
				GroupID:   30,
				Status:    SubscriptionStatusActive,
				StartsAt:  time.Now().Add(-time.Hour),
				ExpiresAt: time.Now().Add(time.Hour),
				Group: &Group{
					ID:             30,
					RateMultiplier: 3,
				},
			},
			{
				ID:        3,
				UserID:    10,
				GroupID:   40,
				Status:    SubscriptionStatusActive,
				StartsAt:  time.Now().Add(-time.Hour),
				ExpiresAt: time.Now().Add(time.Hour),
				Group: &Group{
					ID:                    40,
					RateMultiplier:        4,
					VisibleRateMultiplier: &groupVisibleForActualFallback,
				},
			},
		},
	}
	svc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	svc.SetUserGroupRateRepository(&userVisibleGroupRateRepoStub{
		actualRates:  map[int64]float64{40: 0.7},
		visibleRates: map[int64]float64{30: 0.6},
	})

	subs, err := svc.ListUserSubscriptions(context.Background(), 10)

	require.NoError(t, err)
	require.Len(t, subs, 3)
	require.Equal(t, 0.8, subs[0].Group.RateMultiplier)
	require.Nil(t, subs[0].Group.VisibleRateMultiplier)
	require.Equal(t, 0.6, subs[1].Group.RateMultiplier)
	require.Nil(t, subs[1].Group.VisibleRateMultiplier)
	require.Equal(t, 0.7, subs[2].Group.RateMultiplier)
	require.Nil(t, subs[2].Group.VisibleRateMultiplier)
}
