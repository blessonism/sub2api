import { describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({
    initialized: true,
    fetchStatus: vi.fn(),
    requiresAcknowledgement: false,
    requireAcknowledgement: vi.fn(),
  }),
}))

vi.mock('@/api/setup', () => ({
  getSetupStatus: vi.fn(),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('router invite routes', () => {
  it('keeps affiliate rebate and campaign rewards as separate user routes', async () => {
    const { default: router } = await import('@/router')
    const affiliateRoute = router.getRoutes().find((record) => record.name === 'Affiliate')
    const campaignRewardsRoute = router.getRoutes().find((record) => record.name === 'CampaignRewards')

    expect(affiliateRoute?.path).toBe('/affiliate')
    expect(affiliateRoute?.meta.requiresAuth).toBe(true)
    expect(affiliateRoute?.meta.requiresAdmin).toBe(false)
    expect(affiliateRoute?.meta.titleKey).toBe('affiliate.title')
    expect(String(affiliateRoute?.components?.default ?? affiliateRoute?.component)).toContain('AffiliateView.vue')

    expect(campaignRewardsRoute?.path).toBe('/campaign-rewards')
    expect(campaignRewardsRoute?.meta.requiresAuth).toBe(true)
    expect(campaignRewardsRoute?.meta.requiresAdmin).toBe(false)
    expect(campaignRewardsRoute?.meta.titleKey).toBe('campaignRewards.title')
    expect(String(campaignRewardsRoute?.components?.default ?? campaignRewardsRoute?.component)).toContain('CampaignRewardsView.vue')
  })
})
