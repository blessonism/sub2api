import { describe, expect, it, vi } from 'vitest'
import enMessages from '@/i18n/locales/en'
import zhMessages from '@/i18n/locales/zh'

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
  it('provides activity center labels and route meta keys in both locales', () => {
    expect(enMessages.nav.activities).toBe('Activity Center')
    expect(zhMessages.nav.activities).toBe('活动中心')
    expect(enMessages.activities.title).toBe('Activity Center')
    expect(zhMessages.activities.title).toBe('活动中心')
    expect(enMessages.admin.activities.title).toBe('Activity Center')
    expect(zhMessages.admin.activities.title).toBe('活动中心')
    expect(enMessages.admin.activities.description).toBeTruthy()
    expect(zhMessages.admin.activities.description).toBeTruthy()
  })

  it('registers the admin invite leaderboard with localized navigation', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'AdminAffiliateLeaderboard')

    expect(route?.path).toBe('/admin/affiliates/leaderboard')
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.titleKey).toBe('nav.affiliateLeaderboard')
    expect(route?.meta.descriptionKey).toBe('admin.affiliates.leaderboard.description')
    expect(enMessages.nav.affiliateLeaderboard).toBe('Invite Leaderboard')
    expect(zhMessages.nav.affiliateLeaderboard).toBe('邀请排行榜')
    expect(enMessages.admin.affiliates.leaderboard.paymentRedeemAmount).toBeTruthy()
    expect(zhMessages.admin.affiliates.leaderboard.paymentRedeemAmount).toBeTruthy()
  })

  it('keeps affiliate rebate separate and maps both activity center paths to the same user page', async () => {
    const { default: router } = await import('@/router')
    const affiliateRoute = router.getRoutes().find((record) => record.name === 'Affiliate')
    const activitiesRoute = router.getRoutes().find((record) => record.name === 'Activities')
    const adminActivitiesRoute = router.getRoutes().find((record) => record.name === 'AdminActivities')
    const legacyCampaignRewards = router.resolve('/campaign-rewards')
    const legacyAdminCampaignRewards = router.resolve('/admin/campaign-rewards')

    expect(affiliateRoute?.path).toBe('/affiliate')
    expect(affiliateRoute?.meta.requiresAuth).toBe(true)
    expect(affiliateRoute?.meta.requiresAdmin).toBe(false)
    expect(affiliateRoute?.meta.titleKey).toBe('affiliate.title')
    expect(String(affiliateRoute?.components?.default ?? affiliateRoute?.component)).toContain('AffiliateView.vue')

    expect(activitiesRoute?.path).toBe('/activities')
    expect(activitiesRoute?.meta.requiresAuth).toBe(true)
    expect(activitiesRoute?.meta.requiresAdmin).toBe(false)
    expect(activitiesRoute?.meta.titleKey).toBe('activities.title')
    expect(activitiesRoute?.meta.descriptionKey).toBe('activities.description')
    expect(String(activitiesRoute?.components?.default ?? activitiesRoute?.component)).toContain('CampaignRewardsView.vue')

    expect(legacyCampaignRewards.name).toBe('Activities')
    expect(legacyCampaignRewards.fullPath).toBe('/campaign-rewards')
    expect(legacyCampaignRewards.meta.titleKey).toBe('activities.title')

    expect(adminActivitiesRoute?.path).toBe('/admin/activities')
    expect(adminActivitiesRoute?.meta.requiresAuth).toBe(true)
    expect(adminActivitiesRoute?.meta.requiresAdmin).toBe(true)
    expect(adminActivitiesRoute?.meta.titleKey).toBe('admin.activities.title')
    expect(adminActivitiesRoute?.meta.descriptionKey).toBe('admin.activities.description')
    expect(String(adminActivitiesRoute?.components?.default ?? adminActivitiesRoute?.component)).toContain('CampaignRewardsView.vue')

    expect(legacyAdminCampaignRewards.name).toBe('AdminActivities')
    expect(legacyAdminCampaignRewards.fullPath).toBe('/admin/campaign-rewards')
    expect(legacyAdminCampaignRewards.meta.titleKey).toBe('admin.activities.title')
  })
})
