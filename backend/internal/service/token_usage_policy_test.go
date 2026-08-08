package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTokenUsagePolicyInputDefaultsAndValidation(t *testing.T) {
	enabled := true
	policy, tiers, err := normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "  high usage  ",
		Enabled:       &enabled,
		TargetGroupID: 7,
		Tiers: []TokenUsageAutoPolicyTier{
			{MinTokens: 1000, RateMultiplier: 0.9},
			{MinTokens: 0, RateMultiplier: 1.0},
		},
	}, 0)
	require.NoError(t, err)
	require.Equal(t, "high usage", policy.Name)
	require.Equal(t, 30, policy.WindowDays)
	require.Equal(t, TokenUsagePolicyActionRateOnly, policy.ActionMode)
	require.Equal(t, TokenUsagePolicyConflictManualPriority, policy.ConflictMode)
	require.Equal(t, TokenUsagePolicyFrequencyDaily, policy.ScheduleFrequency)
	require.Equal(t, int64(1000), tiers[0].MinTokens)
	require.Equal(t, int64(0), tiers[1].MinTokens)

	_, _, err = normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "bad",
		WindowDays:    14,
		TargetGroupID: 7,
		Tiers:         []TokenUsageAutoPolicyTier{{MinTokens: 0, RateMultiplier: 1}},
	}, 0)
	require.Error(t, err)

	_, _, err = normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "duplicate",
		TargetGroupID: 7,
		Tiers: []TokenUsageAutoPolicyTier{
			{MinTokens: 100, RateMultiplier: 1},
			{MinTokens: 100, RateMultiplier: 0.8},
		},
	}, 0)
	require.Error(t, err)
}

func TestSelectTokenUsageTierSupportsConfiguredConditions(t *testing.T) {
	tiers := []TokenUsageAutoPolicyTier{
		{ConditionMode: TokenUsagePolicyConditionToken, MinTokens: 100, RateMultiplier: 0.9},
		{ConditionMode: TokenUsagePolicyConditionActualCost, MinActualCost: 2, RateMultiplier: 0.8},
		{ConditionMode: TokenUsagePolicyConditionBoth, MinTokens: 1000, MinActualCost: 10, RateMultiplier: 0.7},
	}

	require.Equal(t, 0.9, selectTokenUsageTier(tiers, 100, 0).RateMultiplier)
	require.Equal(t, 0.8, selectTokenUsageTier(tiers, 0, 2).RateMultiplier)
	require.Equal(t, 0.7, selectTokenUsageTier(tiers, 1000, 10).RateMultiplier)
	require.Nil(t, selectTokenUsageTier(tiers, 99, 1.99))
}

func TestNormalizeTokenUsagePolicyInputPreservesConditionOrderAndValidatesCost(t *testing.T) {
	_, tiers, err := normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "conditions",
		TargetGroupID: 7,
		Tiers: []TokenUsageAutoPolicyTier{
			{ConditionMode: TokenUsagePolicyConditionActualCost, MinActualCost: 1.25, RateMultiplier: 0.9},
			{ConditionMode: TokenUsagePolicyConditionBoth, MinTokens: 100, MinActualCost: 2.5, RateMultiplier: 0.8},
		},
	}, 0)
	require.NoError(t, err)
	require.Equal(t, TokenUsagePolicyConditionActualCost, tiers[0].ConditionMode)
	require.Equal(t, 1, tiers[0].SortOrder)
	require.Equal(t, TokenUsagePolicyConditionBoth, tiers[1].ConditionMode)
	require.Equal(t, 2, tiers[1].SortOrder)

	_, _, err = normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "invalid cost",
		TargetGroupID: 7,
		Tiers:         []TokenUsageAutoPolicyTier{{ConditionMode: TokenUsagePolicyConditionActualCost, MinActualCost: -1, RateMultiplier: 1}},
	}, 0)
	require.Error(t, err)

	_, _, err = normalizeTokenUsagePolicyInput(TokenUsageAutoPolicyInput{
		Name:          "invalid mode",
		TargetGroupID: 7,
		Tiers:         []TokenUsageAutoPolicyTier{{ConditionMode: "unknown", MinTokens: 1, RateMultiplier: 1}},
	}, 0)
	require.Error(t, err)
}

func TestTokenUsagePolicyPreviewDoesNotWrite(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 1, UserName: "u1", UserEmail: "u1@example.com", TokenUsage: 1500}}
	repo.states = map[int64]TokenUsageAutoPolicyState{1: {UserID: 1}}
	svc := NewTokenUsageAutoPolicyService(repo)

	preview, err := svc.PreviewPolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.Len(t, preview.Changes, 1)
	require.Equal(t, TokenUsagePolicyChangeCreate, preview.Changes[0].ChangeType)
	require.False(t, repo.applyCalled)
	require.False(t, repo.runStarted)
}

func TestTokenUsagePolicyBuildChangesDowngradeClearAndManualSkip(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	policy := baseTokenUsagePolicy()
	svc := NewTokenUsageAutoPolicyService(repo)

	t.Run("downgrade", func(t *testing.T) {
		rate := 0.8
		last := 0.7
		previousTierID := int64(2)
		repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 1, TokenUsage: 500}}
		repo.states = map[int64]TokenUsageAutoPolicyState{
			1: {
				UserID:      1,
				CurrentRate: &last,
				Assignment: &TokenUsageAutoAssignment{
					UserID:             1,
					TierID:             &previousTierID,
					LastRateMultiplier: &last,
				},
			},
		}
		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)
		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeDowngrade, changes[0].ChangeType)
		require.InDelta(t, rate, *changes[0].NewRateMultiplier, 0.00001)
		require.Equal(t, 1, stats.DowngradeCount)
	})

	t.Run("clear", func(t *testing.T) {
		oldRate := 0.8
		repo.usageRows = nil
		repo.states = map[int64]TokenUsageAutoPolicyState{
			2: {
				UserID:          2,
				CurrentRate:     &oldRate,
				HasAllowedGroup: true,
				Assignment:      &TokenUsageAutoAssignment{UserID: 2, GroupGrantedByPolicy: true},
			},
		}
		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)
		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeClear, changes[0].ChangeType)
		require.Equal(t, 1, stats.ClearCount)
	})

	t.Run("manual priority skips changed rate", func(t *testing.T) {
		current := 0.6
		lastAuto := 0.8
		repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 3, TokenUsage: 1500}}
		repo.states = map[int64]TokenUsageAutoPolicyState{
			3: {
				UserID:      3,
				CurrentRate: &current,
				Assignment: &TokenUsageAutoAssignment{
					UserID:             3,
					LastRateMultiplier: &lastAuto,
				},
			},
		}
		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)
		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeSkipManual, changes[0].ChangeType)
		require.True(t, changes[0].ManualTakeover)
		require.Equal(t, 1, stats.SkipCount)
	})

	t.Run("manual priority skips existing manual rate before first assignment", func(t *testing.T) {
		current := 0.6
		repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 4, TokenUsage: 1500}}
		repo.states = map[int64]TokenUsageAutoPolicyState{
			4: {
				UserID:      4,
				CurrentRate: &current,
			},
		}
		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)
		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeSkipManual, changes[0].ChangeType)
		require.Equal(t, "existing rate_multiplier is manual", changes[0].Reason)
		require.True(t, changes[0].ManualTakeover)
		require.Equal(t, 1, stats.SkipCount)
	})

	t.Run("manual priority clears policy group ownership when granted group was removed", func(t *testing.T) {
		lastAuto := 0.7
		repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 5, TokenUsage: 1500}}
		repo.states = map[int64]TokenUsageAutoPolicyState{
			5: {
				UserID:          5,
				CurrentRate:     &lastAuto,
				HasAllowedGroup: false,
				Assignment: &TokenUsageAutoAssignment{
					UserID:                 5,
					LastRateMultiplier:     &lastAuto,
					GroupGrantedByPolicy:   true,
					PreviousRateMultiplier: nil,
				},
			},
		}

		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)

		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeSkipManual, changes[0].ChangeType)
		require.Equal(t, TokenUsagePolicyManualGroupRemovedReason, changes[0].Reason)
		require.True(t, changes[0].GroupGranted)
		require.True(t, changes[0].ManualTakeover)
		require.Equal(t, 1, stats.SkipCount)
	})

	t.Run("manual takeover still clears policy group ownership when granted group was removed", func(t *testing.T) {
		lastAuto := 0.7
		repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 6, TokenUsage: 1500}}
		repo.states = map[int64]TokenUsageAutoPolicyState{
			6: {
				UserID:          6,
				CurrentRate:     &lastAuto,
				HasAllowedGroup: false,
				Assignment: &TokenUsageAutoAssignment{
					UserID:               6,
					LastRateMultiplier:   &lastAuto,
					GroupGrantedByPolicy: true,
					ManualTakeover:       true,
					ManualTakeoverReason: "rate_multiplier was changed manually",
				},
			},
		}

		changes, stats, err := svc.buildPolicyChanges(context.Background(), policy)

		require.NoError(t, err)
		require.Len(t, changes, 1)
		require.Equal(t, TokenUsagePolicyChangeSkipManual, changes[0].ChangeType)
		require.Equal(t, TokenUsagePolicyManualGroupRemovedReason, changes[0].Reason)
		require.True(t, changes[0].GroupGranted)
		require.True(t, changes[0].ManualTakeover)
		require.Equal(t, 1, stats.SkipCount)
	})
}

func TestTokenUsagePolicyRunWritesHistoryAndChanges(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 1, TokenUsage: 1500}}
	repo.states = map[int64]TokenUsageAutoPolicyState{1: {UserID: 1}}
	svc := NewTokenUsageAutoPolicyService(repo)

	run, err := svc.RunPolicy(context.Background(), repo.policy.ID, TokenUsagePolicyRunTypeManual)

	require.NoError(t, err)
	require.Equal(t, TokenUsagePolicyRunStatusSuccess, run.Status)
	require.True(t, repo.applyCalled)
	require.True(t, repo.runStarted)
	require.Len(t, repo.appliedChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeCreate, repo.appliedChanges[0].ChangeType)
	require.Len(t, repo.savedRunChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeCreate, repo.savedRunChanges[0].ChangeType)
	require.False(t, repo.disablePolicy)
}

func TestTokenUsagePolicyClearGeneratesClearChangesAndWritesHistory(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	currentRate := 0.7
	previousRate := 1.0
	tierID := int64(2)
	repo.assignmentStates = map[int64]TokenUsageAutoPolicyState{
		1: {
			UserID:          1,
			UserName:        "u1",
			UserEmail:       "u1@example.com",
			CurrentRate:     &currentRate,
			HasAllowedGroup: true,
			Assignment: &TokenUsageAutoAssignment{
				UserID:                 1,
				TargetGroupID:          repo.policy.TargetGroupID,
				TierID:                 &tierID,
				LastTokenUsage:         1500,
				LastRateMultiplier:     &currentRate,
				GroupGrantedByPolicy:   true,
				PreviousRateMultiplier: &previousRate,
			},
		},
	}
	svc := NewTokenUsageAutoPolicyService(repo)

	run, err := svc.ClearPolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.Equal(t, TokenUsagePolicyRunStatusSuccess, run.Status)
	require.True(t, repo.runStarted)
	require.True(t, repo.applyCalled)
	require.Len(t, repo.appliedChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeClear, repo.appliedChanges[0].ChangeType)
	require.False(t, repo.appliedChanges[0].ManualTakeover)
	require.Equal(t, 1, repo.lastRun.ClearCount)
	require.Len(t, repo.savedRunChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeClear, repo.savedRunChanges[0].ChangeType)
	require.True(t, repo.disablePolicy)
	require.Nil(t, repo.nextRunAt)
}

func TestTokenUsagePolicyClearPreservesManualTakeoverRate(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	currentRate := 0.6
	lastAutoRate := 0.7
	repo.assignmentStates = map[int64]TokenUsageAutoPolicyState{
		1: {
			UserID:      1,
			CurrentRate: &currentRate,
			Assignment: &TokenUsageAutoAssignment{
				UserID:               1,
				TargetGroupID:        repo.policy.TargetGroupID,
				LastTokenUsage:       1500,
				LastRateMultiplier:   &lastAutoRate,
				GroupGrantedByPolicy: true,
				ManualTakeover:       true,
			},
		},
	}
	svc := NewTokenUsageAutoPolicyService(repo)

	_, err := svc.ClearPolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.Len(t, repo.appliedChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeClear, repo.appliedChanges[0].ChangeType)
	require.True(t, repo.appliedChanges[0].ManualTakeover)
	require.Equal(t, TokenUsagePolicyManualTakeoverPreservedReason, repo.appliedChanges[0].Reason)
}

func TestTokenUsagePolicyClearRestoresAutoRateAfterManualGroupRemoval(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	currentRate := 0.7
	previousRate := 1.0
	repo.assignmentStates = map[int64]TokenUsageAutoPolicyState{
		1: {
			UserID:          1,
			CurrentRate:     &currentRate,
			HasAllowedGroup: false,
			Assignment: &TokenUsageAutoAssignment{
				UserID:                 1,
				TargetGroupID:          repo.policy.TargetGroupID,
				LastTokenUsage:         1500,
				LastRateMultiplier:     &currentRate,
				GroupGrantedByPolicy:   false,
				PreviousRateMultiplier: &previousRate,
				ManualTakeover:         true,
				ManualTakeoverReason:   TokenUsagePolicyManualGroupRemovedReason,
			},
		},
	}
	svc := NewTokenUsageAutoPolicyService(repo)

	_, err := svc.ClearPolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.Len(t, repo.appliedChanges, 1)
	require.Equal(t, TokenUsagePolicyChangeClear, repo.appliedChanges[0].ChangeType)
	require.False(t, repo.appliedChanges[0].ManualTakeover)
	require.Equal(t, "policy cleared by admin", repo.appliedChanges[0].Reason)
}

func TestTokenUsagePolicyRunApplyFailureDoesNotSaveChanges(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 1, TokenUsage: 1500}}
	repo.states = map[int64]TokenUsageAutoPolicyState{1: {UserID: 1}}
	repo.applyErr = errors.New("apply failed")
	svc := NewTokenUsageAutoPolicyService(repo)

	run, err := svc.RunPolicy(context.Background(), repo.policy.ID, TokenUsagePolicyRunTypeManual)

	require.Error(t, err)
	require.Nil(t, run)
	require.True(t, repo.applyCalled)
	require.Equal(t, TokenUsagePolicyRunStatusFailed, repo.lastRun.Status)
	require.Empty(t, repo.savedRunChanges)
}

func TestTokenUsagePolicyRunInvalidatesAuthCacheForGrantGroupChanges(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	repo.usageRows = []TokenUsageAutoPolicyUsageRow{{UserID: 1, TokenUsage: 1500}}
	repo.states = map[int64]TokenUsageAutoPolicyState{1: {UserID: 1}}
	invalidator := &tokenUsagePolicyAuthCacheInvalidator{}
	rateResolverCache := newUserGroupRateResolver(nil, nil, time.Minute, nil, "service.test").cache
	rateResolverCache.Set(userGroupRateCacheKey(1, repo.policy.TargetGroupID), 9.9, time.Minute)
	svc := NewTokenUsageAutoPolicyService(repo)
	svc.authCacheInvalidator = invalidator

	_, err := svc.RunPolicy(context.Background(), repo.policy.ID, TokenUsagePolicyRunTypeManual)

	require.NoError(t, err)
	require.Equal(t, []int64{1}, invalidator.userIDs)
	_, ok := rateResolverCache.Get(userGroupRateCacheKey(1, repo.policy.TargetGroupID))
	require.False(t, ok)
}

func TestTokenUsagePolicyClearInvalidatesAuthCacheForPolicyGrantedGroups(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.policy = baseTokenUsagePolicy()
	repo.policy.ActionMode = TokenUsagePolicyActionRateOnly
	currentRate := 0.7
	repo.assignmentStates = map[int64]TokenUsageAutoPolicyState{
		1: {
			UserID:      1,
			CurrentRate: &currentRate,
			Assignment: &TokenUsageAutoAssignment{
				UserID:               1,
				TargetGroupID:        repo.policy.TargetGroupID,
				LastRateMultiplier:   &currentRate,
				GroupGrantedByPolicy: true,
			},
		},
	}
	invalidator := &tokenUsagePolicyAuthCacheInvalidator{}
	rateResolverCache := newUserGroupRateResolver(nil, nil, time.Minute, nil, "service.test").cache
	rateResolverCache.Set(userGroupRateCacheKey(1, repo.policy.TargetGroupID), 9.9, time.Minute)
	svc := NewTokenUsageAutoPolicyService(repo)
	svc.authCacheInvalidator = invalidator

	_, err := svc.ClearPolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.Equal(t, []int64{1}, invalidator.userIDs)
	_, ok := rateResolverCache.Get(userGroupRateCacheKey(1, repo.policy.TargetGroupID))
	require.False(t, ok)
}

func TestTokenUsagePolicyUpdateInvalidatesUserGroupRateCacheOnEnabledChange(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	userID := int64(101)
	groupID := repo.policy.TargetGroupID
	userRate := 0.8
	rateRepo := &userGroupRateResolverRepoStub{rate: &userRate, autoRate: true}
	resolver := newUserGroupRateResolver(rateRepo, nil, time.Minute, nil, "service.test")
	svc := NewTokenUsageAutoPolicyService(repo)

	got := resolver.Resolve(context.Background(), userID, groupID, 0.6)
	require.Equal(t, 0.6, got)
	require.Equal(t, 1, rateRepo.calls)

	rateRepo.autoRate = false
	input := tokenUsagePolicyInputForUpdate(repo.policy, false, groupID)
	_, err := svc.UpdatePolicy(context.Background(), repo.policy.ID, input)

	require.NoError(t, err)
	got = resolver.Resolve(context.Background(), userID, groupID, 0.6)
	require.Equal(t, userRate, got)
	require.Equal(t, 2, rateRepo.calls)
}

func TestTokenUsagePolicyUpdateInvalidatesOldAndNewTargetGroupCaches(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	oldGroupID := repo.policy.TargetGroupID
	newGroupID := oldGroupID + 10
	otherGroupID := oldGroupID + 20
	userID := int64(101)
	resolver := newUserGroupRateResolver(nil, nil, time.Minute, nil, "service.test")
	resolver.cache.Set(userGroupRateCacheKey(userID, oldGroupID), 1.7, time.Minute)
	resolver.cache.Set(userGroupVisibleRateCacheKey(userID, oldGroupID), 1.3, time.Minute)
	resolver.cache.Set(userGroupRateCacheKey(userID, newGroupID), 1.8, time.Minute)
	resolver.cache.Set(userGroupVisibleRateCacheKey(userID, newGroupID), 1.4, time.Minute)
	resolver.cache.Set(userGroupRateCacheKey(userID, otherGroupID), 1.9, time.Minute)
	svc := NewTokenUsageAutoPolicyService(repo)
	input := tokenUsagePolicyInputForUpdate(repo.policy, repo.policy.Enabled, newGroupID)

	_, err := svc.UpdatePolicy(context.Background(), repo.policy.ID, input)

	require.NoError(t, err)
	_, ok := resolver.cache.Get(userGroupRateCacheKey(userID, oldGroupID))
	require.False(t, ok)
	_, ok = resolver.cache.Get(userGroupVisibleRateCacheKey(userID, oldGroupID))
	require.False(t, ok)
	_, ok = resolver.cache.Get(userGroupRateCacheKey(userID, newGroupID))
	require.False(t, ok)
	_, ok = resolver.cache.Get(userGroupVisibleRateCacheKey(userID, newGroupID))
	require.False(t, ok)
	cached, ok := resolver.cache.Get(userGroupRateCacheKey(userID, otherGroupID))
	require.True(t, ok)
	require.Equal(t, 1.9, cached)
}

func TestTokenUsagePolicyUpdateRejectsTargetGroupChangeWithAssignments(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.assignmentCount = 1
	svc := NewTokenUsageAutoPolicyService(repo)
	input := TokenUsageAutoPolicyInput{
		Name:              "policy",
		Enabled:           tokenUsagePolicyBoolPtr(true),
		WindowDays:        30,
		TargetGroupID:     repo.policy.TargetGroupID + 1,
		ActionMode:        TokenUsagePolicyActionGrantGroupAndRate,
		ConflictMode:      TokenUsagePolicyConflictManualPriority,
		ScheduleFrequency: TokenUsagePolicyFrequencyDaily,
		Tiers:             []TokenUsageAutoPolicyTier{{MinTokens: 0, RateMultiplier: 1}},
	}

	_, err := svc.UpdatePolicy(context.Background(), repo.policy.ID, input)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "POLICY_TARGET_GROUP_LOCKED", infraerrors.Reason(err))
	require.False(t, repo.updateCalled)
}

func TestTokenUsagePolicyDeleteAndRunsValidatePolicyExists(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.getErr = infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	svc := NewTokenUsageAutoPolicyService(repo)

	err := svc.DeletePolicy(context.Background(), repo.policy.ID)
	require.Error(t, err)
	require.Equal(t, "INVALID_POLICY_ID", infraerrors.Reason(err))

	_, _, err = svc.ListRuns(context.Background(), repo.policy.ID, 1, 20)
	require.Error(t, err)
	require.Equal(t, "INVALID_POLICY_ID", infraerrors.Reason(err))
}

func TestTokenUsagePolicyDeleteRejectsBlockingAssignments(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.assignmentCount = 1
	svc := NewTokenUsageAutoPolicyService(repo)

	err := svc.DeletePolicy(context.Background(), repo.policy.ID)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "POLICY_HAS_ASSIGNMENTS", infraerrors.Reason(err))
	require.False(t, repo.deleteCalled)
}

func TestTokenUsagePolicyDeleteAllowsManualOnlyAssignments(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.assignmentCount = 0
	svc := NewTokenUsageAutoPolicyService(repo)

	err := svc.DeletePolicy(context.Background(), repo.policy.ID)

	require.NoError(t, err)
	require.True(t, repo.deleteCalled)
}

func TestTokenUsagePolicyRunAlreadyRunningReturnsConflict(t *testing.T) {
	repo := newTokenUsagePolicyFakeRepo()
	repo.tryLockAllowed = false
	svc := NewTokenUsageAutoPolicyService(repo)

	_, err := svc.RunPolicy(context.Background(), repo.policy.ID, TokenUsagePolicyRunTypeManual)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "POLICY_ALREADY_RUNNING", infraerrors.Reason(err))
	require.False(t, repo.runStarted)
}

func baseTokenUsagePolicy() TokenUsageAutoPolicy {
	return TokenUsageAutoPolicy{
		ID:                9,
		Name:              "policy",
		Enabled:           true,
		WindowDays:        30,
		TargetGroupID:     8,
		ActionMode:        TokenUsagePolicyActionGrantGroupAndRate,
		ConflictMode:      TokenUsagePolicyConflictManualPriority,
		ScheduleFrequency: TokenUsagePolicyFrequencyDaily,
		Tiers: []TokenUsageAutoPolicyTier{
			{ID: 1, PolicyID: 9, MinTokens: 100, RateMultiplier: 0.8},
			{ID: 2, PolicyID: 9, MinTokens: 1000, RateMultiplier: 0.7},
		},
	}
}

func tokenUsagePolicyBoolPtr(v bool) *bool {
	return &v
}

func tokenUsagePolicyInputForUpdate(policy TokenUsageAutoPolicy, enabled bool, targetGroupID int64) TokenUsageAutoPolicyInput {
	return TokenUsageAutoPolicyInput{
		Name:              policy.Name,
		Enabled:           tokenUsagePolicyBoolPtr(enabled),
		WindowDays:        policy.WindowDays,
		TargetGroupID:     targetGroupID,
		ActionMode:        policy.ActionMode,
		ConflictMode:      policy.ConflictMode,
		ScheduleFrequency: policy.ScheduleFrequency,
		Filters:           policy.Filters,
		Tiers:             policy.Tiers,
	}
}

type tokenUsagePolicyAuthCacheInvalidator struct {
	userIDs []int64
}

func (i *tokenUsagePolicyAuthCacheInvalidator) InvalidateAuthCacheByKey(context.Context, string) {}

func (i *tokenUsagePolicyAuthCacheInvalidator) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func (i *tokenUsagePolicyAuthCacheInvalidator) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	i.userIDs = append(i.userIDs, userID)
}

type tokenUsagePolicyFakeRepo struct {
	policy           TokenUsageAutoPolicy
	getErr           error
	applyErr         error
	usageRows        []TokenUsageAutoPolicyUsageRow
	states           map[int64]TokenUsageAutoPolicyState
	assignmentStates map[int64]TokenUsageAutoPolicyState
	assignmentCount  int64
	updateCalled     bool
	deleteCalled     bool
	tryLockAllowed   bool
	applyCalled      bool
	runStarted       bool
	appliedChanges   []TokenUsageAutoPolicyChange
	savedRunChanges  []TokenUsageAutoPolicyChange
	nextRunAt        *time.Time
	disablePolicy    bool
	lastRun          TokenUsageAutoPolicyRun
}

func newTokenUsagePolicyFakeRepo() *tokenUsagePolicyFakeRepo {
	return &tokenUsagePolicyFakeRepo{
		policy:         baseTokenUsagePolicy(),
		states:         map[int64]TokenUsageAutoPolicyState{},
		tryLockAllowed: true,
	}
}

func (r *tokenUsagePolicyFakeRepo) ListPolicies(context.Context, pagination.PaginationParams, TokenUsageAutoPolicyListFilters) ([]TokenUsageAutoPolicy, *pagination.PaginationResult, error) {
	return []TokenUsageAutoPolicy{r.policy}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *tokenUsagePolicyFakeRepo) GetPolicyByID(context.Context, int64) (*TokenUsageAutoPolicy, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	p := r.policy
	return &p, nil
}

func (r *tokenUsagePolicyFakeRepo) CreatePolicy(context.Context, *TokenUsageAutoPolicy, []TokenUsageAutoPolicyTier) (*TokenUsageAutoPolicy, error) {
	return &r.policy, nil
}

func (r *tokenUsagePolicyFakeRepo) UpdatePolicy(_ context.Context, policy *TokenUsageAutoPolicy, tiers []TokenUsageAutoPolicyTier) (*TokenUsageAutoPolicy, error) {
	r.updateCalled = true
	r.policy = *policy
	r.policy.Tiers = append([]TokenUsageAutoPolicyTier(nil), tiers...)
	return &r.policy, nil
}

func (r *tokenUsagePolicyFakeRepo) DeletePolicy(context.Context, int64) error {
	r.deleteCalled = true
	return nil
}

func (r *tokenUsagePolicyFakeRepo) CountAssignments(context.Context, int64) (int64, error) {
	return r.assignmentCount, nil
}

func (r *tokenUsagePolicyFakeRepo) ListDuePolicies(context.Context, time.Time, int) ([]TokenUsageAutoPolicy, error) {
	return []TokenUsageAutoPolicy{r.policy}, nil
}

func (r *tokenUsagePolicyFakeRepo) TryLockPolicy(context.Context, int64) (bool, error) {
	return r.tryLockAllowed, nil
}

func (r *tokenUsagePolicyFakeRepo) AggregatePolicyUsage(context.Context, TokenUsageAutoPolicy, time.Time) ([]TokenUsageAutoPolicyUsageRow, error) {
	return r.usageRows, nil
}

func (r *tokenUsagePolicyFakeRepo) ListPolicyStates(context.Context, int64, int64, []int64) (map[int64]TokenUsageAutoPolicyState, error) {
	out := make(map[int64]TokenUsageAutoPolicyState, len(r.states))
	for k, v := range r.states {
		out[k] = v
	}
	return out, nil
}

func (r *tokenUsagePolicyFakeRepo) ListPolicyAssignmentStates(context.Context, int64) (map[int64]TokenUsageAutoPolicyState, error) {
	out := make(map[int64]TokenUsageAutoPolicyState, len(r.assignmentStates))
	for k, v := range r.assignmentStates {
		out[k] = v
	}
	return out, nil
}

func (r *tokenUsagePolicyFakeRepo) ApplyPolicyChanges(_ context.Context, _ TokenUsageAutoPolicy, changes []TokenUsageAutoPolicyChange, _ *time.Time) error {
	r.applyCalled = true
	r.appliedChanges = append([]TokenUsageAutoPolicyChange(nil), changes...)
	if r.applyErr != nil {
		return r.applyErr
	}
	return nil
}

func (r *tokenUsagePolicyFakeRepo) ApplyPolicyChangesAndFinishRun(_ context.Context, _ int64, _ TokenUsageAutoPolicy, changes []TokenUsageAutoPolicyChange, stats TokenUsageAutoPolicyRunStats, nextRunAt *time.Time, disablePolicy bool) error {
	r.applyCalled = true
	r.appliedChanges = append([]TokenUsageAutoPolicyChange(nil), changes...)
	r.nextRunAt = nextRunAt
	r.disablePolicy = disablePolicy
	if r.applyErr != nil {
		return r.applyErr
	}
	r.lastRun.Status = TokenUsagePolicyRunStatusSuccess
	r.lastRun.TotalUsers = stats.TotalUsers
	r.lastRun.CreateCount = stats.CreateCount
	r.lastRun.UpdateCount = stats.UpdateCount
	r.lastRun.DowngradeCount = stats.DowngradeCount
	r.lastRun.ClearCount = stats.ClearCount
	r.lastRun.SkipCount = stats.SkipCount
	r.savedRunChanges = append([]TokenUsageAutoPolicyChange(nil), changes...)
	return nil
}

func (r *tokenUsagePolicyFakeRepo) BeginPolicyRun(_ context.Context, _ int64, runType string) (*TokenUsageAutoPolicyRun, error) {
	r.runStarted = true
	r.lastRun = TokenUsageAutoPolicyRun{ID: 1, PolicyID: r.policy.ID, RunType: runType, Status: TokenUsagePolicyRunStatusRunning, CreatedAt: time.Now()}
	return &r.lastRun, nil
}

func (r *tokenUsagePolicyFakeRepo) FinishPolicyRun(_ context.Context, _ int64, status string, stats TokenUsageAutoPolicyRunStats, changes []TokenUsageAutoPolicyChange, _ string) error {
	r.lastRun.Status = status
	r.lastRun.TotalUsers = stats.TotalUsers
	r.lastRun.CreateCount = stats.CreateCount
	r.lastRun.UpdateCount = stats.UpdateCount
	r.lastRun.DowngradeCount = stats.DowngradeCount
	r.lastRun.ClearCount = stats.ClearCount
	r.lastRun.SkipCount = stats.SkipCount
	r.savedRunChanges = append([]TokenUsageAutoPolicyChange(nil), changes...)
	return nil
}

func (r *tokenUsagePolicyFakeRepo) ListPolicyRuns(context.Context, int64, pagination.PaginationParams) ([]TokenUsageAutoPolicyRun, *pagination.PaginationResult, error) {
	return []TokenUsageAutoPolicyRun{r.lastRun}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 1, Pages: 1}, nil
}

func (r *tokenUsagePolicyFakeRepo) ListPolicyRunChanges(context.Context, int64, int64, pagination.PaginationParams) ([]TokenUsageAutoPolicyChange, *pagination.PaginationResult, error) {
	return r.savedRunChanges, &pagination.PaginationResult{Total: int64(len(r.savedRunChanges)), Page: 1, PageSize: 20, Pages: 1}, nil
}
