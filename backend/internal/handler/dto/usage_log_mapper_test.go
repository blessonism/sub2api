package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogFromServiceUsesVisibleRateSnapshotForUserDTO(t *testing.T) {
	visibleRate := 0.8
	groupVisibleRate := 0.9
	log := &service.UsageLog{
		ID:                    1,
		RateMultiplier:        1.6,
		VisibleRateMultiplier: &visibleRate,
		Group: &service.Group{
			ID:                    10,
			Name:                  "vip",
			RateMultiplier:        1.6,
			VisibleRateMultiplier: &groupVisibleRate,
		},
		APIKey: &service.APIKey{
			ID: 2,
			Group: &service.Group{
				ID:                    10,
				Name:                  "vip",
				RateMultiplier:        1.6,
				VisibleRateMultiplier: &groupVisibleRate,
			},
		},
		Subscription: &service.UserSubscription{
			ID: 3,
			Group: &service.Group{
				ID:                    10,
				Name:                  "vip",
				RateMultiplier:        1.6,
				VisibleRateMultiplier: &groupVisibleRate,
			},
		},
	}

	userDTO := UsageLogFromService(log)
	require.NotNil(t, userDTO)
	require.Equal(t, visibleRate, userDTO.RateMultiplier)
	require.NotNil(t, userDTO.Group)
	require.Equal(t, visibleRate, userDTO.Group.RateMultiplier)
	require.Nil(t, userDTO.Group.VisibleRateMultiplier)
	require.NotNil(t, userDTO.APIKey)
	require.NotNil(t, userDTO.APIKey.Group)
	require.Equal(t, visibleRate, userDTO.APIKey.Group.RateMultiplier)
	require.Nil(t, userDTO.APIKey.Group.VisibleRateMultiplier)
	require.NotNil(t, userDTO.Subscription)
	require.NotNil(t, userDTO.Subscription.Group)
	require.Equal(t, visibleRate, userDTO.Subscription.Group.RateMultiplier)
	require.Nil(t, userDTO.Subscription.Group.VisibleRateMultiplier)

	adminDTO := UsageLogFromServiceAdmin(log)
	require.NotNil(t, adminDTO)
	require.Equal(t, 1.6, adminDTO.RateMultiplier)
	require.Equal(t, &visibleRate, adminDTO.VisibleRateMultiplier)
}
