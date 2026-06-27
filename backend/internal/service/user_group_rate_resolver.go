package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type userGroupRateResolver struct {
	repo         UserGroupRateRepository
	cache        *gocache.Cache
	cacheTTL     time.Duration
	sf           *singleflight.Group
	logComponent string
}

var userGroupRateResolverCaches sync.Map

func newUserGroupRateResolver(repo UserGroupRateRepository, cache *gocache.Cache, cacheTTL time.Duration, sf *singleflight.Group, logComponent string) *userGroupRateResolver {
	if cacheTTL <= 0 {
		cacheTTL = defaultUserGroupRateCacheTTL
	}
	if cache == nil {
		cache = gocache.New(cacheTTL, time.Minute)
	}
	if logComponent == "" {
		logComponent = "service.gateway"
	}
	if sf == nil {
		sf = &singleflight.Group{}
	}
	userGroupRateResolverCaches.Store(cache, struct{}{})

	return &userGroupRateResolver{
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		sf:           sf,
		logComponent: logComponent,
	}
}

func (r *userGroupRateResolver) Invalidate(userID, groupID int64) {
	if r == nil || r.cache == nil || userID <= 0 || groupID <= 0 {
		return
	}
	r.cache.Delete(userGroupRateCacheKey(userID, groupID))
	r.cache.Delete(userGroupVisibleRateCacheKey(userID, groupID))
}

func invalidateUserGroupRateCache(userID, groupID int64) {
	if userID <= 0 || groupID <= 0 {
		return
	}
	key := userGroupRateCacheKey(userID, groupID)
	visibleKey := userGroupVisibleRateCacheKey(userID, groupID)
	userGroupRateResolverCaches.Range(func(cache, _ any) bool {
		if c, ok := cache.(*gocache.Cache); ok && c != nil {
			c.Delete(key)
			c.Delete(visibleKey)
		}
		return true
	})
}

func invalidateUserGroupRateCacheByGroupID(groupID int64) {
	if groupID <= 0 {
		return
	}
	suffix := fmt.Sprintf(":%d", groupID)
	userGroupRateResolverCaches.Range(func(cache, _ any) bool {
		if c, ok := cache.(*gocache.Cache); ok && c != nil {
			for key := range c.Items() {
				if strings.HasSuffix(key, suffix) {
					c.Delete(key)
				}
			}
		}
		return true
	})
}

func invalidateUserGroupRateCacheByUserID(userID int64) {
	if userID <= 0 {
		return
	}
	prefixActual := fmt.Sprintf("actual:%d:", userID)
	prefixVisible := fmt.Sprintf("visible:%d:", userID)
	userGroupRateResolverCaches.Range(func(cache, _ any) bool {
		if c, ok := cache.(*gocache.Cache); ok && c != nil {
			for key := range c.Items() {
				if strings.HasPrefix(key, prefixActual) || strings.HasPrefix(key, prefixVisible) {
					c.Delete(key)
				}
			}
		}
		return true
	})
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	if r == nil || userID <= 0 || groupID <= 0 {
		return groupDefaultMultiplier
	}

	key := userGroupRateCacheKey(userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				return multiplier
			}
		}
	}
	if r.repo == nil {
		return groupDefaultMultiplier
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					return multiplier, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := groupDefaultMultiplier
		if userRate != nil {
			multiplier = *userRate
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
		}
		return multiplier, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed, fallback to group default: user=%d group=%d err=%v", userID, groupID, err)
		return groupDefaultMultiplier
	}

	multiplier, ok := value.(float64)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return groupDefaultMultiplier
	}
	return multiplier
}

func (r *userGroupRateResolver) ResolveVisible(ctx context.Context, userID, groupID int64, groupVisibleMultiplier *float64, effectiveRateMultiplier float64) float64 {
	defaultVisible := effectiveRateMultiplier
	if groupVisibleMultiplier != nil {
		defaultVisible = *groupVisibleMultiplier
	}
	if r == nil || userID <= 0 || groupID <= 0 {
		return defaultVisible
	}

	key := userGroupVisibleRateCacheKey(userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				return multiplier
			}
		}
	}
	if r.repo == nil {
		return defaultVisible
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					return multiplier, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userVisibleRate, repoErr := r.repo.GetVisibleByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := defaultVisible
		if userVisibleRate != nil {
			multiplier = *userVisibleRate
		} else {
			userRate, rateErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
			if rateErr != nil {
				return nil, rateErr
			}
			if userRate != nil {
				multiplier = *userRate
			}
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
		}
		return multiplier, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group visible rate failed, fallback to visible default: user=%d group=%d err=%v", userID, groupID, err)
		return defaultVisible
	}

	multiplier, ok := value.(float64)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return defaultVisible
	}
	return multiplier
}

func userGroupRateCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("actual:%d:%d", userID, groupID)
}

func userGroupVisibleRateCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("visible:%d:%d", userID, groupID)
}
