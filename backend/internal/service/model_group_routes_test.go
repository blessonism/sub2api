package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type modelRouteGroupRepo struct {
	GroupRepository
	groups map[int64]*Group
}

func (r *modelRouteGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	return r.groups[id], nil
}

type modelRouteAccountRepo struct {
	AccountRepository
	accounts map[int64][]Account
}

func (r *modelRouteAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, id int64, _ string) ([]Account, error) {
	return r.accounts[id], nil
}

func modelRouteFixture() (*ModelGroupRouteResolver, *APIKey, *modelRouteAccountRepo) {
	source := &Group{ID: 51, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 0.2, Hydrated: true,
		ModelGroupRoutes: []ModelGroupRoute{{Model: "gpt-6-luna", TargetGroupID: 40, Mode: domain.ModelGroupRouteTargetOnly, Enabled: true}}}
	target := &Group{ID: 40, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 0.3, Hydrated: true}
	override := 99
	key := &APIKey{ID: 1, GroupID: &source.ID, Group: source, UserID: 2, User: &User{ID: 2, Status: StatusActive, UserGroupRPMOverride: &override}, Quota: 10}
	accounts := &modelRouteAccountRepo{accounts: map[int64][]Account{}}
	gateway := &OpenAIGatewayService{accountRepo: accounts}
	return &ModelGroupRouteResolver{groups: &modelRouteGroupRepo{groups: map[int64]*Group{40: target, 51: source}}, openai: gateway}, key, accounts
}

func TestModelGroupRoutesTargetKeepsKeyIdentityAndIsolatesSnapshots(t *testing.T) {
	r, key, _ := modelRouteFixture()
	target, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna"})
	require.NoError(t, err)
	require.Equal(t, int64(40), *target.GroupID)
	require.Equal(t, int64(51), *target.SourceGroupID)
	require.Equal(t, key.ID, target.ID)
	require.Equal(t, key.Quota, target.Quota)
	require.Equal(t, 0.3, target.Group.RateMultiplier)
	require.Nil(t, target.User.UserGroupRPMOverride)
	require.NotNil(t, key.User.UserGroupRPMOverride)
	require.Equal(t, int64(51), *key.GroupID)
	require.Nil(t, key.SourceGroupID)
	// 目标也配置路由时不得递归。
	target.Group.ModelGroupRoutes = append(target.Group.ModelGroupRoutes, ModelGroupRoute{Model: "gpt-6-luna", TargetGroupID: 51, Enabled: true})
	again, err := r.Resolve(context.Background(), target, ModelGroupRouteRequest{Model: "gpt-6-luna"})
	require.NoError(t, err)
	require.Same(t, target, again)
	other, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-sol"})
	require.NoError(t, err)
	require.Same(t, key, other)
}

func TestModelGroupRoutesPreferLocalUsesEligibilityWithoutAcquisition(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*Account)
		local     bool
	}{
		{"eligible even when concurrency is busy", func(a *Account) { a.Concurrency = 1 }, true},
		{"disabled", func(a *Account) { a.Schedulable = false }, false},
		{"rate limited", func(a *Account) { until := time.Now().Add(time.Hour); a.RateLimitResetAt = &until }, false},
		{"model rate limited", func(a *Account) {
			a.Extra = map[string]any{modelRateLimitsKey: map[string]any{"gpt-6-luna": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}}
		}, false},
		{"wrong model", func(a *Account) {
			a.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-6-sol": "gpt-6-sol"}}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, key, repo := modelRouteFixture()
			key.Group.ModelGroupRoutes[0].Mode = domain.ModelGroupRoutePreferLocal
			account := Account{ID: 248, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-luna": "gpt-6-luna"}}}
			tc.configure(&account)
			repo.accounts[51] = []Account{account}
			selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna", RequiredTransport: OpenAIUpstreamTransportAny})
			require.NoError(t, err)
			if tc.local {
				require.Same(t, key, selected)
			} else {
				require.Equal(t, int64(40), *selected.GroupID)
			}
		})
	}
}

func TestModelGroupRoutesPreferLocalUsesGlobalQuotaAutoPause(t *testing.T) {
	r, key, accounts := modelRouteFixture()
	key.Group.ModelGroupRoutes[0].Mode = domain.ModelGroupRoutePreferLocal
	accounts.accounts[51] = []Account{{ID: 248, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{"codex_5h_used_percent": 96.0}}}
	settings := newRuntimeSettingRepoStub()
	settings.values[SettingKeyOpsAdvancedSettings] = `{"openai_account_quota_auto_pause":{"default_threshold_5h":0.95}}`
	r.openai.settingService = NewSettingService(settings, &config.Config{})
	r.openai.settingService.WarmOpenAIQuotaAutoPauseSettings(context.Background())
	selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna"})
	require.NoError(t, err)
	require.Equal(t, int64(40), *selected.GroupID, "达到真实调度器全局暂停阈值的本组账号不能阻止转发")
}

func modelRouteTestGroups(r *ModelGroupRouteResolver) map[int64]*Group {
	repo, ok := r.groups.(*modelRouteGroupRepo)
	if !ok {
		panic("unexpected model route group repository in test")
	}
	return repo.groups
}

func TestModelGroupRoutesAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*ModelGroupRouteResolver, *APIKey)
	}{
		{"source whitelist", func(_ *ModelGroupRouteResolver, key *APIKey) {
			key.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-sol"}}
		}},
		{"target whitelist", func(r *ModelGroupRouteResolver, _ *APIKey) {
			modelRouteTestGroups(r)[40].ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-sol"}}
		}},
		{"target disabled", func(r *ModelGroupRouteResolver, _ *APIKey) {
			modelRouteTestGroups(r)[40].Status = StatusDisabled
		}},
		{"target deleted", func(r *ModelGroupRouteResolver, _ *APIKey) { delete(modelRouteTestGroups(r), 40) }},
		{"exclusive target", func(r *ModelGroupRouteResolver, _ *APIKey) {
			modelRouteTestGroups(r)[40].IsExclusive = true
		}},
		{"restricted public target", func(_ *ModelGroupRouteResolver, key *APIKey) {
			key.User.RestrictPublicGroups = true
			key.User.AllowedGroups = []int64{51}
		}},
		{"subscription target", func(r *ModelGroupRouteResolver, _ *APIKey) {
			modelRouteTestGroups(r)[40].SubscriptionType = SubscriptionTypeSubscription
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, key, _ := modelRouteFixture()
			tc.modify(r, key)
			_, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna"})
			require.Error(t, err)
		})
	}
}

func TestModelGroupRoutesValidationAndAuthCache(t *testing.T) {
	r, key, _ := modelRouteFixture()
	admin := &adminServiceImpl{groupRepo: r.groups}
	valid := key.Group.ModelGroupRoutes
	normalized, err := admin.normalizeModelGroupRoutes(context.Background(), 51, PlatformOpenAI, SubscriptionTypeStandard, valid)
	require.NoError(t, err)
	require.Equal(t, valid, normalized)
	for _, routes := range [][]ModelGroupRoute{
		{{Model: "gpt-*", TargetGroupID: 40}}, {{Model: "gpt-6-luna", TargetGroupID: 51}},
		{valid[0], valid[0]}, {{Model: "gpt-6-luna", TargetGroupID: 40, Mode: "unknown"}},
	} {
		_, err = admin.normalizeModelGroupRoutes(context.Background(), 51, PlatformOpenAI, SubscriptionTypeStandard, routes)
		require.Error(t, err)
	}
	snapshot := &APIKeyAuthSnapshot{Version: apiKeyAuthSnapshotVersion, APIKeyID: 1, GroupID: key.GroupID, Group: &APIKeyAuthGroupSnapshot{ID: 51, ModelGroupRoutes: valid}}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var decoded APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(raw, &decoded))
	svc := &APIKeyService{}
	cached := svc.snapshotToAPIKey("route-key", &decoded)
	require.Equal(t, valid, cached.Group.ModelGroupRoutes)
	old, ok, err := svc.applyAuthCacheEntry("route-key", &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: 25}})
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, old)
	ctx := ModelGroupRouteContext(context.Background(), key)
	require.Same(t, key.Group, ctx.Value(ctxkey.Group))
}

func TestModelGroupRoutesBillingUsesEffectiveGroupAndOriginalKeyOnce(t *testing.T) {
	r, key, _ := modelRouteFixture()
	key.Group.ModelGroupRoutes[0].Model = "gpt-5.1"
	key.RateLimit5h = 100
	selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-5.1"})
	require.NoError(t, err)
	usage := &openAIRecordUsageLogRepoStub{inserted: true}
	ledger := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usage, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	tokens := OpenAIUsage{InputTokens: 100, OutputTokens: 20}
	err = svc.RecordUsage(ModelGroupRouteContext(context.Background(), selected), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "route-one-charge", Model: "gpt-5.1", Usage: tokens},
		APIKey: selected, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, User: selected.User, Account: &Account{ID: 230, Type: AccountTypeAPIKey, Platform: PlatformOpenAI},
	})
	require.NoError(t, err)
	require.Equal(t, 1, ledger.calls)
	require.Equal(t, 1, usage.calls)
	require.Equal(t, key.ID, ledger.lastCmd.APIKeyID)
	require.Equal(t, int64(40), *usage.lastLog.GroupID)
	require.Equal(t, int64(51), *usage.lastLog.SourceGroupID)
	require.Equal(t, 0.3, usage.lastLog.RateMultiplier)
	expected := expectedOpenAICost(t, svc, "gpt-5.1", tokens, 0.3)
	require.InDelta(t, expected.ActualCost, ledger.lastCmd.BalanceCost, 1e-8)
	require.Equal(t, ledger.lastCmd.BalanceCost, ledger.lastCmd.APIKeyQuotaCost)
	require.Equal(t, ledger.lastCmd.BalanceCost, ledger.lastCmd.APIKeyRateLimitCost)
	require.Equal(t, int64(51), *key.GroupID)
}

type modelRouteRPMCache struct {
	UserRPMCache
	groups []int64
}

func (r *modelRouteRPMCache) IncrementUserGroupRPM(_ context.Context, _ int64, groupID int64) (int, error) {
	r.groups = append(r.groups, groupID)
	return len(r.groups), nil
}

func TestModelGroupRoutesCountsOnlyEffectiveGroupRPM(t *testing.T) {
	r, key, _ := modelRouteFixture()
	selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna"})
	require.NoError(t, err)
	selected.Group.RPMLimit = 1
	counter := &modelRouteRPMCache{}
	billing := &BillingCacheService{userRPMCache: counter}
	require.Empty(t, counter.groups, "选组本身不能增加计数")
	require.NoError(t, billing.checkRPM(context.Background(), selected.User, selected.Group))
	require.Equal(t, []int64{40}, counter.groups)
	require.ErrorIs(t, billing.checkRPM(context.Background(), selected.User, selected.Group), ErrGroupRPMExceeded)
	require.NotNil(t, key.User.UserGroupRPMOverride, "共享认证用户快照保持不变")
}

type modelRouteBindingRepo struct {
	UserGroupAccountBindingRepository
	binding UserGroupAccountBinding
}

func (r *modelRouteBindingRepo) GetByUserAndGroup(context.Context, int64, int64) (*UserGroupAccountBinding, error) {
	return &r.binding, nil
}

func TestModelGroupRoutesPreferLocalHonorsUserBindingAndCapabilities(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		r, key, accounts := modelRouteFixture()
		key.Group.ModelGroupRoutes[0].Mode = domain.ModelGroupRoutePreferLocal
		accounts.accounts[51] = []Account{{ID: 248, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}}
		r.openai.userGroupAccountBindingResolver = NewUserGroupAccountBindingResolver(&modelRouteBindingRepo{binding: UserGroupAccountBinding{AccountIDs: []int64{999}, FallbackToGroup: fallback}})
		selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna", RequiredTransport: OpenAIUpstreamTransportAny})
		require.NoError(t, err)
		if fallback {
			require.Equal(t, int64(51), *selected.GroupID)
		} else {
			require.Equal(t, int64(40), *selected.GroupID)
		}
	}
	r, key, accounts := modelRouteFixture()
	key.Group.ModelGroupRoutes[0].Mode = domain.ModelGroupRoutePreferLocal
	accounts.accounts[51] = []Account{{ID: 248, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{"openai_responses_supported": false}}}
	selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-6-luna", RequiredTransport: OpenAIUpstreamTransportAny, RequiredCapability: OpenAIEndpointCapabilityResponses})
	require.NoError(t, err)
	require.Equal(t, int64(40), *selected.GroupID)
}

type modelRouteRateRepo struct {
	userGroupRateResolverRepoStub
	groupIDs []int64
}

func (r *modelRouteRateRepo) GetByUserAndGroup(_ context.Context, _ int64, id int64) (*float64, error) {
	r.groupIDs = append(r.groupIDs, id)
	return r.rate, nil
}

func TestModelGroupRoutesUsesTargetUserRate(t *testing.T) {
	r, key, _ := modelRouteFixture()
	key.Group.ModelGroupRoutes[0].Model = "gpt-5.1"
	selected, err := r.Resolve(context.Background(), key, ModelGroupRouteRequest{Model: "gpt-5.1"})
	require.NoError(t, err)
	rate := 0.12
	rates := &modelRouteRateRepo{userGroupRateResolverRepoStub: userGroupRateResolverRepoStub{rate: &rate}}
	usage := &openAIRecordUsageLogRepoStub{inserted: true}
	ledger := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usage, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, rates)
	err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{APIKey: selected, User: selected.User, Account: &Account{ID: 230, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, Result: &OpenAIForwardResult{RequestID: "route-personal-rate", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 20}}})
	require.NoError(t, err)
	require.NotEmpty(t, rates.groupIDs)
	for _, id := range rates.groupIDs {
		require.Equal(t, int64(40), id)
	}
	require.Equal(t, rate, usage.lastLog.RateMultiplier)
}
