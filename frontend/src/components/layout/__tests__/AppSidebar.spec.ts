import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { h } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppSidebar from '../AppSidebar.vue'

const sidebarTestState = vi.hoisted(() => ({
  route: {
    path: '/dashboard',
  },
  router: {
    push: vi.fn(),
  },
  appStore: {
    sidebarCollapsed: false,
    mobileOpen: false,
    backendModeEnabled: false,
    siteName: 'Sub2API',
    siteLogo: '',
    siteVersion: 'test-version',
    publicSettingsLoaded: true,
    cachedPublicSettings: {
      custom_menu_items: [],
      purchase_subscription_enabled: false,
      purchase_subscription_url: '',
    },
    toggleSidebar: vi.fn(),
    setMobileOpen: vi.fn(),
  },
  authStore: {
    isAdmin: false,
    isSimpleMode: false,
  },
  onboardingStore: {
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  },
  adminSettingsStore: {
    opsMonitoringEnabled: false,
    paymentEnabled: false,
    customMenuItems: [],
    fetch: vi.fn(),
  },
  batchImageAccess: {
    canUseBatchImage: { value: false },
    refreshBatchImageAccess: vi.fn(),
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => sidebarTestState.route,
  useRouter: () => sidebarTestState.router,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'nav.activities': 'Activity Center',
    'nav.affiliate': 'Invite Rebates',
    'nav.campaignRewards': 'Campaign Rewards',
    'nav.myAccount': 'My Account',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAdminSettingsStore: () => sidebarTestState.adminSettingsStore,
  useAppStore: () => sidebarTestState.appStore,
  useAuthStore: () => sidebarTestState.authStore,
  useOnboardingStore: () => sidebarTestState.onboardingStore,
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: {
    availableChannels: {},
    channelMonitor: {},
    payment: {},
    affiliate: {},
    riskControl: {},
  },
  makeSidebarFlag: () => () => true,
}))

vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => sidebarTestState.batchImageAccess,
}))

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

const RouterLinkStub = {
  name: 'RouterLink',
  props: {
    to: {
      type: [String, Object],
      required: true,
    },
  },
  setup(props: { to: string | Record<string, unknown> }, { attrs, slots }: { attrs: Record<string, unknown>, slots: { default?: () => unknown } }) {
    return () =>
      h(
        'a',
        {
          ...attrs,
          href: typeof props.to === 'string' ? props.to : '#',
          'data-router-link-to': typeof props.to === 'string' ? props.to : JSON.stringify(props.to),
        },
        slots.default?.()
      )
  },
}

function mountSidebar(routePath = '/dashboard') {
  sidebarTestState.route.path = routePath

  return mount(AppSidebar, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        VersionBadge: true,
      },
    },
  })
}

function sidebarLinks(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAll('a.sidebar-link')
}

function sidebarLinkIndex(wrapper: ReturnType<typeof mount>, label: string): number {
  return sidebarLinks(wrapper).findIndex((link) => link.text().includes(label))
}

function sidebarLinkByLabel(wrapper: ReturnType<typeof mount>, label: string) {
  const link = sidebarLinks(wrapper).find((candidate) => candidate.text().includes(label))
  expect(link, `${label} sidebar link`).toBeDefined()
  return link!
}

beforeEach(() => {
  sidebarTestState.route.path = '/dashboard'
  sidebarTestState.router.push.mockReset()
  sidebarTestState.appStore.sidebarCollapsed = false
  sidebarTestState.appStore.mobileOpen = false
  sidebarTestState.appStore.backendModeEnabled = false
  sidebarTestState.appStore.cachedPublicSettings = {
    custom_menu_items: [],
    purchase_subscription_enabled: false,
    purchase_subscription_url: '',
  }
  sidebarTestState.authStore.isAdmin = false
  sidebarTestState.authStore.isSimpleMode = false
  sidebarTestState.onboardingStore.isCurrentStep.mockReturnValue(false)
  sidebarTestState.onboardingStore.nextStep.mockReset()
  sidebarTestState.adminSettingsStore.fetch.mockReset()
  sidebarTestState.batchImageAccess.canUseBatchImage.value = false
  sidebarTestState.batchImageAccess.refreshBatchImageAccess.mockReset()
  localStorage.clear()
  document.documentElement.classList.remove('dark')
})

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar external payment entry', () => {
  it('uses the public settings purchase URL as a direct sidebar link', () => {
    expect(componentSource).not.toContain('EXTERNAL_PAYMENT_URL')
    expect(componentSource).not.toContain('https://pay.ldxp.cn/shop/Y5TXJ2DO')
    expect(componentSource).toContain('settings?.purchase_subscription_enabled')
    expect(componentSource).toContain('settings.purchase_subscription_url?.trim()')
    expect(componentSource).toContain("label: t('nav.externalPayment')")
    expect(componentSource).toContain('externalUrl: externalPaymentUrl.value')
    expect(componentSource).toContain(':href="item.externalUrl"')
    expect(componentSource).toContain('target="_blank"')
    expect(componentSource).toContain('rel="noopener noreferrer"')
  })
})

describe('AppSidebar admin token leaderboard entry', () => {
  it('keeps the admin Token leaderboard as an admin-only navigation item', () => {
    expect(componentSource).toContain("path: '/admin/token-leaderboard'")
    expect(componentSource).toContain("label: t('nav.tokenLeaderboard')")
  })
})

describe('AppSidebar affiliate leaderboard entry', () => {
  it('adds the global invite leaderboard under affiliate management', () => {
    expect(componentSource).toContain("path: '/admin/affiliates/leaderboard'")
    expect(componentSource).toContain("label: t('nav.affiliateLeaderboard')")
  })
})

describe('AppSidebar admin balance redemption entry', () => {
  it('keeps balance and redeem records behind one admin navigation item', () => {
    expect(componentSource).toContain("path: '/admin/balance-redemption'")
    expect(componentSource).toContain("label: t('nav.balanceRedemption')")
    expect(componentSource).not.toContain("path: '/admin/redeem-records'")
  })
})

describe('AppSidebar leaderboard entry', () => {
  it('adds leaderboard to the shared user and admin personal navigation declaration', () => {
    expect(componentSource).toContain("{ path: '/leaderboard', label: t('nav.leaderboard'), icon: LeaderboardIcon }")
    expect(componentSource).toContain('const userNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(true)))')
    expect(componentSource).toContain('const personalNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(false)))')
  })
})

describe('AppSidebar mounted activity center navigation', () => {
  it('renders the regular user activity center entry after the affiliate rebate entry', () => {
    const wrapper = mountSidebar()

    const affiliateIndex = sidebarLinkIndex(wrapper, 'Invite Rebates')
    const activitiesIndex = sidebarLinkIndex(wrapper, 'Activity Center')
    const affiliateLink = sidebarLinkByLabel(wrapper, 'Invite Rebates')
    const activitiesLink = sidebarLinkByLabel(wrapper, 'Activity Center')

    expect(affiliateIndex).toBeGreaterThanOrEqual(0)
    expect(activitiesIndex).toBeGreaterThanOrEqual(0)
    expect(affiliateIndex).toBeLessThan(activitiesIndex)
    expect(affiliateLink.attributes('data-router-link-to')).toBe('/affiliate')
    expect(activitiesLink.attributes('data-router-link-to')).toBe('/activities')
  })

  it('marks activity center active when the legacy campaign rewards route is current', () => {
    const wrapper = mountSidebar('/campaign-rewards')

    const activitiesLink = sidebarLinkByLabel(wrapper, 'Activity Center')

    expect(activitiesLink.classes()).toContain('sidebar-link-active')
    expect(activitiesLink.attributes('data-router-link-to')).toBe('/activities')
  })

  it('renders the admin personal activity center entry and keeps it active for the legacy user route', () => {
    sidebarTestState.authStore.isAdmin = true
    sidebarTestState.authStore.isSimpleMode = false

    const wrapper = mountSidebar('/campaign-rewards')
    const personalSection = wrapper.findAll('.sidebar-section').find((section) => {
      const title = section.find('.sidebar-section-title')
      return title.exists() && title.text().includes('My Account')
    })

    expect(personalSection, 'admin personal section').toBeDefined()

    const activitiesLink = personalSection!.findAll('a.sidebar-link').find((candidate) => candidate.text().includes('Activity Center'))

    expect(activitiesLink, 'admin personal activity center link').toBeDefined()
    expect(activitiesLink!.attributes('data-router-link-to')).toBe('/activities')
    expect(activitiesLink!.classes()).toContain('sidebar-link-active')
  })

  it('renders the admin activity center entry and marks it active on the main admin route', () => {
    sidebarTestState.authStore.isAdmin = true
    sidebarTestState.authStore.isSimpleMode = false

    const wrapper = mountSidebar('/admin/activities')
    const activitiesLink = sidebarLinks(wrapper).find((candidate) => {
      return candidate.attributes('data-router-link-to') === '/admin/activities'
    })

    expect(activitiesLink, 'admin activity center link').toBeDefined()
    expect(activitiesLink!.text()).toContain('Activity Center')
    expect(activitiesLink!.text()).not.toContain('Campaign Rewards')
    expect(activitiesLink!.classes()).toContain('sidebar-link-active')
  })

  it('marks the admin activity center entry active when the legacy admin campaign rewards route is current', () => {
    sidebarTestState.authStore.isAdmin = true
    sidebarTestState.authStore.isSimpleMode = false

    const wrapper = mountSidebar('/admin/campaign-rewards')
    const activitiesLink = sidebarLinks(wrapper).find((candidate) => {
      return candidate.text().includes('Activity Center') && candidate.classes().includes('sidebar-link-active')
    })

    expect(activitiesLink, 'active admin activity center link').toBeDefined()
    expect(activitiesLink.attributes('data-router-link-to')).toBe('/admin/activities')
    expect(activitiesLink.classes()).toContain('sidebar-link-active')
  })
})

describe('AppSidebar invite navigation entries', () => {
  it('keeps the persistent affiliate rebate entry separate from the activity center', () => {
    const affiliateEntry = "{ path: '/affiliate', label: t('nav.affiliate'), icon: AffiliateIcon, hideInSimpleMode: true, featureFlag: flagAffiliate }"
    const activitiesEntry = "{ path: '/activities', label: t('nav.activities'), icon: BadgeIcon, hideInSimpleMode: true }"

    expect(componentSource).toContain(affiliateEntry)
    expect(componentSource).toContain(activitiesEntry)
    expect(componentSource.indexOf(affiliateEntry)).toBeLessThan(componentSource.indexOf(activitiesEntry))
  })

  it('keeps both activity center entries highlighted for their legacy aliases', () => {
    expect(componentSource).toContain("if (path === '/activities' && route.path === '/campaign-rewards')")
    expect(componentSource).toContain("if (path === '/admin/activities' && route.path === '/admin/campaign-rewards')")
    expect(componentSource).toContain("return route.path === path || route.path.startsWith(path + '/')")
    expect(componentSource).toContain("{ path: '/admin/activities', label: t('nav.activities'), icon: BadgeIcon, hideInSimpleMode: true }")
  })
})
