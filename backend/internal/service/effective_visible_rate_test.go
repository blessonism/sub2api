//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type effectiveVisibleRateRepoStub struct {
	userGroupRateRepoStubForGroupRate

	actualRates  map[int64]float64
	visibleRates map[int64]float64
}

func (r *effectiveVisibleRateRepoStub) GetByUserID(_ context.Context, _ int64) (map[int64]float64, error) {
	return r.actualRates, nil
}

func (r *effectiveVisibleRateRepoStub) GetVisibleByUserID(_ context.Context, _ int64) (map[int64]float64, error) {
	return r.visibleRates, nil
}

func TestEffectiveVisibleRateForGroup_Precedence(t *testing.T) {
	groupVisible := 1.4
	group := &Group{
		ID:                    10,
		RateMultiplier:        2,
		VisibleRateMultiplier: &groupVisible,
	}

	rates, err := loadUserGroupRateMaps(context.Background(), &effectiveVisibleRateRepoStub{
		actualRates:  map[int64]float64{10: 0.7},
		visibleRates: map[int64]float64{10: 0.6},
	}, 100)
	require.NoError(t, err)
	require.Equal(t, 0.6, effectiveVisibleRateForGroup(group, rates))

	rates, err = loadUserGroupRateMaps(context.Background(), &effectiveVisibleRateRepoStub{
		actualRates: map[int64]float64{10: 0.7},
	}, 100)
	require.NoError(t, err)
	require.Equal(t, 0.7, effectiveVisibleRateForGroup(group, rates))

	rates, err = loadUserGroupRateMaps(context.Background(), &effectiveVisibleRateRepoStub{}, 100)
	require.NoError(t, err)
	require.Equal(t, groupVisible, effectiveVisibleRateForGroup(group, rates))

	group.VisibleRateMultiplier = nil
	require.Equal(t, 2.0, effectiveVisibleRateForGroup(group, rates))
}

func TestEffectiveVisibleRateForGroup_TimeRateUserPriority(t *testing.T) {
	groupVisible := 1.5
	group := &Group{
		ID:                    10,
		RateMultiplier:        2,
		VisibleRateMultiplier: &groupVisible,
		TimeRatePriority:      TimeRatePriorityUserFirst,
		TimeRatePeriods: []GroupTimeRatePeriod{{
			StartTime: "00:00", EndTime: "24:00", RateMultiplier: 0.5, VisibleRateMultiplier: 0.3, Enabled: true,
		}},
	}

	require.Equal(t, 0.3, effectiveVisibleRateForGroup(group, userGroupRateMaps{}))
	require.Equal(t, 0.7, effectiveVisibleRateForGroup(group, userGroupRateMaps{actual: map[int64]float64{10: 0.7}}))
	require.Equal(t, 0.6, effectiveVisibleRateForGroup(group, userGroupRateMaps{
		actual: map[int64]float64{10: 0.7}, visible: map[int64]float64{10: 0.6},
	}))
}
