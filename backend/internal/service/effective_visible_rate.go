package service

import (
	"context"
	"fmt"
)

// userGroupRateMaps 保存用户专属真实倍率与显式可见倍率。
type userGroupRateMaps struct {
	actual  map[int64]float64
	visible map[int64]float64
}

func loadUserGroupRateMaps(ctx context.Context, repo UserGroupRateRepository, userID int64) (userGroupRateMaps, error) {
	rates := userGroupRateMaps{
		actual:  map[int64]float64{},
		visible: map[int64]float64{},
	}
	if repo == nil || userID <= 0 {
		return rates, nil
	}

	actualRates, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		return rates, fmt.Errorf("get user group rates: %w", err)
	}
	visibleRates, err := repo.GetVisibleByUserID(ctx, userID)
	if err != nil {
		return rates, fmt.Errorf("get user visible group rates: %w", err)
	}
	rates.actual = actualRates
	rates.visible = visibleRates
	return rates, nil
}

func effectiveVisibleRateForGroup(group *Group, rates userGroupRateMaps) float64 {
	if group == nil {
		return 1
	}
	if rate, ok := rates.visible[group.ID]; ok {
		return rate
	}
	if rate, ok := rates.actual[group.ID]; ok {
		return rate
	}
	return group.VisibleEffectiveRateMultiplier()
}
