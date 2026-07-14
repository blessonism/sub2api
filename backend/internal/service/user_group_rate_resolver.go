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
	r.cache.Delete(userGroupRateSourceCacheKey(userID, groupID))
	r.cache.Delete(userGroupVisibleRateCacheKey(userID, groupID))
	r.cache.Delete(userGroupVisibleRateSourceCacheKey(userID, groupID))
}

func invalidateUserGroupRateCache(userID, groupID int64) {
	if userID <= 0 || groupID <= 0 {
		return
	}
	key := userGroupRateCacheKey(userID, groupID)
	sourceKey := userGroupRateSourceCacheKey(userID, groupID)
	visibleKey := userGroupVisibleRateCacheKey(userID, groupID)
	visibleSourceKey := userGroupVisibleRateSourceCacheKey(userID, groupID)
	userGroupRateResolverCaches.Range(func(cache, _ any) bool {
		if c, ok := cache.(*gocache.Cache); ok && c != nil {
			c.Delete(key)
			c.Delete(sourceKey)
			c.Delete(visibleKey)
			c.Delete(visibleSourceKey)
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
	prefixActualSource := fmt.Sprintf("actual-source:%d:", userID)
	prefixVisible := fmt.Sprintf("visible:%d:", userID)
	prefixVisibleSource := fmt.Sprintf("visible-source:%d:", userID)
	userGroupRateResolverCaches.Range(func(cache, _ any) bool {
		if c, ok := cache.(*gocache.Cache); ok && c != nil {
			for key := range c.Items() {
				if strings.HasPrefix(key, prefixActual) || strings.HasPrefix(key, prefixActualSource) || strings.HasPrefix(key, prefixVisible) || strings.HasPrefix(key, prefixVisibleSource) {
					c.Delete(key)
				}
			}
		}
		return true
	})
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	multiplier, _ := r.ResolveWithSource(ctx, userID, groupID, groupDefaultMultiplier)
	return multiplier
}

type resolvedUserGroupRate struct {
	multiplier float64
	overridden bool
}

func (r *userGroupRateResolver) ResolveWithSource(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) (float64, bool) {
	if r == nil || userID <= 0 || groupID <= 0 {
		return groupDefaultMultiplier, false
	}

	key := userGroupRateCacheKey(userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				overridden := multiplier != groupDefaultMultiplier
				if source, sourceOK := r.cache.Get(userGroupRateSourceCacheKey(userID, groupID)); sourceOK {
					overridden, _ = source.(bool)
				}
				return multiplier, overridden
			}
		}
	}
	if r.repo == nil {
		return groupDefaultMultiplier, false
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					overridden := multiplier != groupDefaultMultiplier
					if source, sourceOK := r.cache.Get(userGroupRateSourceCacheKey(userID, groupID)); sourceOK {
						overridden, _ = source.(bool)
					}
					return resolvedUserGroupRate{multiplier: multiplier, overridden: overridden}, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := groupDefaultMultiplier
		overridden := userRate != nil
		if userRate != nil {
			capped, capErr := capTokenUsageAutoRateMultiplier(ctx, r.repo, userID, groupID, *userRate, groupDefaultMultiplier)
			if capErr != nil {
				return nil, capErr
			}
			multiplier = capped
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
			r.cache.Set(userGroupRateSourceCacheKey(userID, groupID), overridden, r.cacheTTL)
		}
		return resolvedUserGroupRate{multiplier: multiplier, overridden: overridden}, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed, fallback to group default: user=%d group=%d err=%v", userID, groupID, err)
		return groupDefaultMultiplier, false
	}

	resolved, ok := value.(resolvedUserGroupRate)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return groupDefaultMultiplier, false
	}
	return resolved.multiplier, resolved.overridden
}

func (r *userGroupRateResolver) ResolveVisible(ctx context.Context, userID, groupID int64, groupVisibleMultiplier *float64, effectiveRateMultiplier float64) float64 {
	multiplier, _ := r.ResolveVisibleWithSource(ctx, userID, groupID, groupVisibleMultiplier, effectiveRateMultiplier)
	return multiplier
}

func (r *userGroupRateResolver) ResolveVisibleWithSource(ctx context.Context, userID, groupID int64, groupVisibleMultiplier *float64, effectiveRateMultiplier float64) (float64, bool) {
	defaultVisible := effectiveRateMultiplier
	if groupVisibleMultiplier != nil {
		defaultVisible = *groupVisibleMultiplier
	}
	if r == nil || userID <= 0 || groupID <= 0 {
		return defaultVisible, false
	}

	key := userGroupVisibleRateCacheKey(userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				overridden := multiplier != defaultVisible
				if source, sourceOK := r.cache.Get(userGroupVisibleRateSourceCacheKey(userID, groupID)); sourceOK {
					overridden, _ = source.(bool)
				}
				return multiplier, overridden
			}
		}
	}
	if r.repo == nil {
		return defaultVisible, false
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					overridden := multiplier != defaultVisible
					if source, sourceOK := r.cache.Get(userGroupVisibleRateSourceCacheKey(userID, groupID)); sourceOK {
						overridden, _ = source.(bool)
					}
					return resolvedUserGroupRate{multiplier: multiplier, overridden: overridden}, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userVisibleRate, repoErr := r.repo.GetVisibleByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := defaultVisible
		overridden := userVisibleRate != nil
		if userVisibleRate != nil {
			multiplier = *userVisibleRate
		} else {
			userRate, rateErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
			if rateErr != nil {
				return nil, rateErr
			}
			if userRate != nil {
				overridden = true
				capped, capErr := capTokenUsageAutoRateMultiplier(ctx, r.repo, userID, groupID, *userRate, defaultVisible)
				if capErr != nil {
					return nil, capErr
				}
				multiplier = capped
			}
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
			r.cache.Set(userGroupVisibleRateSourceCacheKey(userID, groupID), overridden, r.cacheTTL)
		}
		return resolvedUserGroupRate{multiplier: multiplier, overridden: overridden}, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group visible rate failed, fallback to visible default: user=%d group=%d err=%v", userID, groupID, err)
		return defaultVisible, false
	}

	resolved, ok := value.(resolvedUserGroupRate)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return defaultVisible, false
	}
	return resolved.multiplier, resolved.overridden
}

func userGroupRateCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("actual:%d:%d", userID, groupID)
}

func userGroupRateSourceCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("actual-source:%d:%d", userID, groupID)
}

func userGroupVisibleRateCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("visible:%d:%d", userID, groupID)
}

func userGroupVisibleRateSourceCacheKey(userID, groupID int64) string {
	return fmt.Sprintf("visible-source:%d:%d", userID, groupID)
}
