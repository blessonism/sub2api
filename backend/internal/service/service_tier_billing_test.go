package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBillingServiceTier(t *testing.T) {
	tests := []struct {
		name       string
		requested  string
		observed   string
		billing    string
		downgraded bool
	}{
		{name: "openai priority served as default", requested: "priority", observed: "default", billing: "default", downgraded: true},
		{name: "anthropic fast served as standard", requested: "fast", observed: "standard", billing: "standard", downgraded: true},
		{name: "priority honoured", requested: "priority", observed: "priority", billing: "priority"},
		{name: "no declaration keeps request", requested: "priority", observed: "", billing: "priority"},
		{name: "no request no declaration", requested: "", observed: "", billing: ""},
		{name: "response never raises the tier", requested: "", observed: "priority", billing: ""},
		{name: "flex never raised to default", requested: "flex", observed: "default", billing: "flex"},
		{name: "default echoed for untiered request", requested: "", observed: "default", billing: ""},
		{name: "unknown response tier ignored", requested: "priority", observed: "turbo", billing: "priority"},
		{name: "case and whitespace normalised", requested: " Priority ", observed: "DEFAULT", billing: "default", downgraded: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBillingServiceTier(tt.requested, tt.observed)
			require.Equal(t, tt.billing, got.Billing)
			require.Equal(t, tt.downgraded, got.Downgraded)
		})
	}
}

func TestApplyServiceTierBillingResolutionOnlyRewritesDowngrades(t *testing.T) {
	t.Run("codex exception only covers OpenAI default", func(t *testing.T) {
		require.True(t, codexOAuthResponseTierIsNonAuthoritative("default"))
		require.False(t, codexOAuthResponseTierIsNonAuthoritative("standard"))
		require.False(t, codexOAuthResponseTierIsNonAuthoritative("flex"))
	})

	t.Run("openai downgrade rewrites tier", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result, false)
		require.True(t, resolution.Downgraded)
		require.NotNil(t, result.ServiceTier)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("openai honoured tier keeps pointer", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "priority"}
		require.False(t, ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result, false).Downgraded)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("openai untiered request stays nil", func(t *testing.T) {
		result := &OpenAIForwardResult{UpstreamResponseServiceTier: "priority"}
		require.False(t, ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result, false).Downgraded)
		require.Nil(t, result.ServiceTier)
	})

	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run("codex "+accountType+" keeps outbound priority despite default echo", func(t *testing.T) {
			requested := "priority"
			result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
				false,
			)
			require.False(t, resolution.Downgraded)
			require.Equal(t, "priority", resolution.Requested)
			require.Equal(t, "default", resolution.Observed)
			require.Equal(t, "priority", resolution.Billing)
			require.Same(t, &requested, result.ServiceTier)
		})

		t.Run("codex "+accountType+" still accepts an explicit flex downgrade", func(t *testing.T) {
			requested := "priority"
			result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "flex"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
				false,
			)
			require.True(t, resolution.Downgraded)
			require.Equal(t, "flex", resolution.Billing)
			require.Equal(t, "flex", *result.ServiceTier)
		})

		t.Run("codex "+accountType+" response never promotes an untiered request", func(t *testing.T) {
			result := &OpenAIForwardResult{UpstreamResponseServiceTier: "priority"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
				false,
			)
			require.False(t, resolution.Downgraded)
			require.Empty(t, resolution.Billing)
			require.Nil(t, result.ServiceTier)
		})
	}

	t.Run("non-openai oauth still uses the generic response contract", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(
			&Account{Platform: PlatformGrok, Type: AccountTypeOAuth},
			result,
			false,
		)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "default", resolution.Billing)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("anthropic standard speed rewrites fast", func(t *testing.T) {
		requested := "fast"
		result := &ForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "standard"}
		require.True(t, ApplyForwardServiceTierBillingResolution(result).Downgraded)
		require.Equal(t, "standard", *result.ServiceTier)
	})

	t.Run("nil results are ignored", func(t *testing.T) {
		require.False(t, ApplyOpenAIServiceTierBillingResolution(nil, nil, false).Downgraded)
		require.False(t, ApplyForwardServiceTierBillingResolution(nil).Downgraded)
	})
}

func TestTrustRequestedServiceTierBilling(t *testing.T) {
	relayAccount := func() *Account {
		return &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{"openai_trust_requested_service_tier": true},
		}
	}

	t.Run("relay apikey keeps fast despite a default echo", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relayAccount(), result, false)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Requested)
		require.Equal(t, "default", resolution.Observed)
		require.Equal(t, "priority", resolution.Billing)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("relay apikey also ignores a standard echo", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "standard"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relayAccount(), result, false)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Billing)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("relay apikey never promotes an untiered request", func(t *testing.T) {
		result := &OpenAIForwardResult{UpstreamResponseServiceTier: "priority"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relayAccount(), result, false)
		require.False(t, resolution.Downgraded)
		require.Empty(t, resolution.Billing)
		require.Nil(t, result.ServiceTier)
	})

	t.Run("switch off keeps the generic apikey downgrade", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		resolution := ApplyOpenAIServiceTierBillingResolution(account, result, false)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "default", resolution.Billing)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("switch off but codex oauth still honoured", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra:    map[string]any{"openai_trust_requested_service_tier": false},
		}
		resolution := ApplyOpenAIServiceTierBillingResolution(account, result, false)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Billing)
	})

	t.Run("shadow account override opts out even when credential extra misses the flag", func(t *testing.T) {
		// 调度选中的影子账号开启了开关,而 resolveCredentialAccount 解析出的凭据
		// 账号 Extra 未携带:override 参数把管理员意图带进档位判定。
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		credentialAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		resolution := ApplyOpenAIServiceTierBillingResolution(credentialAccount, result, true)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Requested)
		require.Equal(t, "default", resolution.Observed)
		require.Equal(t, "priority", resolution.Billing)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("relay apikey also ignores a flex echo", func(t *testing.T) {
		// 开关的语义是"响应档位声明一律不可信",flex 虽更便宜但不豁免于该信任。
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "flex"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relayAccount(), result, false)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Billing)
		require.Equal(t, "flex", resolution.Observed)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("nil account and non-bool extras fall back safely", func(t *testing.T) {
		require.False(t, (*Account)(nil).TrustRequestedServiceTier())
		require.False(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}).TrustRequestedServiceTier())
		require.False(t, (&Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{"openai_trust_requested_service_tier": "yes"},
		}).TrustRequestedServiceTier())
		require.False(t, (&Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{"openai_trust_requested_service_tier": true},
		}).TrustRequestedServiceTier())
	})
}
