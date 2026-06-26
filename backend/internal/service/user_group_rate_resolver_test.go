package service

import (
	"context"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type userGroupRateResolverRepoStub struct {
	UserGroupRateRepository

	rate         *float64
	visibleRate  *float64
	err          error
	visibleErr   error
	calls        int
	visibleCalls int
}

func (s *userGroupRateResolverRepoStub) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.rate, nil
}

func (s *userGroupRateResolverRepoStub) GetVisibleByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	s.visibleCalls++
	if s.visibleErr != nil {
		return nil, s.visibleErr
	}
	return s.visibleRate, nil
}

func TestNewUserGroupRateResolver_Defaults(t *testing.T) {
	resolver := newUserGroupRateResolver(nil, nil, 0, nil, "")

	require.NotNil(t, resolver)
	require.NotNil(t, resolver.cache)
	require.Equal(t, defaultUserGroupRateCacheTTL, resolver.cacheTTL)
	require.NotNil(t, resolver.sf)
	require.Equal(t, "service.gateway", resolver.logComponent)
}

func TestUserGroupRateResolverResolve_FallbackForNilResolverAndInvalidIDs(t *testing.T) {
	var nilResolver *userGroupRateResolver
	require.Equal(t, 1.4, nilResolver.Resolve(context.Background(), 101, 202, 1.4))

	resolver := newUserGroupRateResolver(nil, nil, time.Second, nil, "service.test")
	require.Equal(t, 1.4, resolver.Resolve(context.Background(), 0, 202, 1.4))
	require.Equal(t, 1.4, resolver.Resolve(context.Background(), 101, 0, 1.4))
}

func TestUserGroupRateResolverResolve_InvalidCacheEntryLoadsRepoAndCaches(t *testing.T) {
	resetGatewayHotpathStatsForTest()

	rate := 1.7
	repo := &userGroupRateResolverRepoStub{rate: &rate}
	cache := gocache.New(time.Minute, time.Minute)
	key := userGroupRateCacheKey(101, 202)
	cache.Set(key, "bad-cache", time.Minute)
	resolver := newUserGroupRateResolver(repo, cache, time.Minute, nil, "service.test")

	got := resolver.Resolve(context.Background(), 101, 202, 1.2)
	require.Equal(t, rate, got)
	require.Equal(t, 1, repo.calls)

	cached, ok := cache.Get(key)
	require.True(t, ok)
	require.Equal(t, rate, cached)

	hit, miss, load, _, fallback := GatewayUserGroupRateCacheStats()
	require.Equal(t, int64(0), hit)
	require.Equal(t, int64(1), miss)
	require.Equal(t, int64(1), load)
	require.Equal(t, int64(0), fallback)
}

func TestInvalidateUserGroupRateCacheClearsRegisteredResolverCaches(t *testing.T) {
	cacheA := gocache.New(time.Minute, time.Minute)
	cacheB := gocache.New(time.Minute, time.Minute)
	resolverA := newUserGroupRateResolver(nil, cacheA, time.Minute, nil, "service.test")
	resolverB := newUserGroupRateResolver(nil, cacheB, time.Minute, nil, "service.test")
	resolverA.cache.Set(userGroupRateCacheKey(101, 202), 1.7, time.Minute)
	resolverA.cache.Set(userGroupVisibleRateCacheKey(101, 202), 1.3, time.Minute)
	resolverB.cache.Set(userGroupRateCacheKey(101, 202), 1.8, time.Minute)
	resolverB.cache.Set(userGroupVisibleRateCacheKey(101, 202), 1.4, time.Minute)

	invalidateUserGroupRateCache(101, 202)

	_, ok := resolverA.cache.Get(userGroupRateCacheKey(101, 202))
	require.False(t, ok)
	_, ok = resolverA.cache.Get(userGroupVisibleRateCacheKey(101, 202))
	require.False(t, ok)
	_, ok = resolverB.cache.Get(userGroupRateCacheKey(101, 202))
	require.False(t, ok)
	_, ok = resolverB.cache.Get(userGroupVisibleRateCacheKey(101, 202))
	require.False(t, ok)
}

func TestInvalidateUserGroupRateCacheByGroupIDClearsRegisteredResolverCaches(t *testing.T) {
	cache := gocache.New(time.Minute, time.Minute)
	resolver := newUserGroupRateResolver(nil, cache, time.Minute, nil, "service.test")
	resolver.cache.Set(userGroupRateCacheKey(101, 202), 1.7, time.Minute)
	resolver.cache.Set(userGroupVisibleRateCacheKey(101, 202), 1.3, time.Minute)
	resolver.cache.Set(userGroupRateCacheKey(101, 303), 1.9, time.Minute)

	invalidateUserGroupRateCacheByGroupID(202)

	_, ok := resolver.cache.Get(userGroupRateCacheKey(101, 202))
	require.False(t, ok)
	_, ok = resolver.cache.Get(userGroupVisibleRateCacheKey(101, 202))
	require.False(t, ok)
	cached, ok := resolver.cache.Get(userGroupRateCacheKey(101, 303))
	require.True(t, ok)
	require.Equal(t, 1.9, cached)
}

func TestInvalidateUserGroupRateCacheByUserIDClearsRegisteredResolverCaches(t *testing.T) {
	cache := gocache.New(time.Minute, time.Minute)
	resolver := newUserGroupRateResolver(nil, cache, time.Minute, nil, "service.test")
	resolver.cache.Set(userGroupRateCacheKey(101, 202), 1.7, time.Minute)
	resolver.cache.Set(userGroupVisibleRateCacheKey(101, 202), 1.3, time.Minute)
	resolver.cache.Set(userGroupRateCacheKey(303, 202), 1.9, time.Minute)
	resolver.cache.Set(userGroupVisibleRateCacheKey(303, 202), 1.4, time.Minute)

	invalidateUserGroupRateCacheByUserID(101)

	_, ok := resolver.cache.Get(userGroupRateCacheKey(101, 202))
	require.False(t, ok)
	_, ok = resolver.cache.Get(userGroupVisibleRateCacheKey(101, 202))
	require.False(t, ok)
	cached, ok := resolver.cache.Get(userGroupRateCacheKey(303, 202))
	require.True(t, ok)
	require.Equal(t, 1.9, cached)
	cached, ok = resolver.cache.Get(userGroupVisibleRateCacheKey(303, 202))
	require.True(t, ok)
	require.Equal(t, 1.4, cached)
}

func TestUserGroupRateResolverResolveVisible_PrecedenceAndFallbacks(t *testing.T) {
	var nilResolver *userGroupRateResolver
	groupVisible := 1.25
	require.Equal(t, groupVisible, nilResolver.ResolveVisible(context.Background(), 101, 202, &groupVisible, 1.8))
	require.Equal(t, 1.8, nilResolver.ResolveVisible(context.Background(), 101, 202, nil, 1.8))

	resolverWithoutRepo := newUserGroupRateResolver(nil, nil, time.Second, nil, "service.test")
	require.Equal(t, groupVisible, resolverWithoutRepo.ResolveVisible(context.Background(), 101, 202, &groupVisible, 1.8))
	require.Equal(t, 1.8, resolverWithoutRepo.ResolveVisible(context.Background(), 101, 202, nil, 1.8))

	userVisible := 0.95
	repo := &userGroupRateResolverRepoStub{visibleRate: &userVisible}
	cache := gocache.New(time.Minute, time.Minute)
	resolver := newUserGroupRateResolver(repo, cache, time.Minute, nil, "service.test")

	got := resolver.ResolveVisible(context.Background(), 101, 202, &groupVisible, 1.8)
	require.Equal(t, userVisible, got)
	require.Equal(t, 1, repo.visibleCalls)

	cached, ok := cache.Get(userGroupVisibleRateCacheKey(101, 202))
	require.True(t, ok)
	require.Equal(t, userVisible, cached)
}

func TestGatewayServiceGetUserGroupRateMultiplier_FallbacksAndUsesExistingResolver(t *testing.T) {
	var nilSvc *GatewayService
	require.Equal(t, 1.3, nilSvc.getUserGroupRateMultiplier(context.Background(), 101, 202, 1.3))

	rate := 1.9
	repo := &userGroupRateResolverRepoStub{rate: &rate}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "service.gateway")
	svc := &GatewayService{userGroupRateResolver: resolver}

	got := svc.getUserGroupRateMultiplier(context.Background(), 101, 202, 1.2)
	require.Equal(t, rate, got)
	require.Equal(t, 1, repo.calls)
}
