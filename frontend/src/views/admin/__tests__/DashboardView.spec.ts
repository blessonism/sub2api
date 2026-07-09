import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  today_active_users: 0,
  yesterday_active_users: 0,
  total_user_balance: 0,
  subscription_remaining_value: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('renders operational metric cards', async () => {
    getSnapshotV2.mockResolvedValue({
      stats: {
        ...createDashboardStats(),
        today_active_users: 12,
        yesterday_active_users: 9,
        total_user_balance: 1234.5,
        subscription_remaining_value: 678.9
      },
      trend: [],
      models: []
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('admin.dashboard.todayActiveUsers')
    expect(text).toContain('admin.dashboard.yesterdayActiveUsers')
    expect(text).toContain('admin.dashboard.totalUserBalance')
    expect(text).toContain('admin.dashboard.subscriptionRemainingValue')
    expect(text).toContain('12')
    expect(text).toContain('9')
    expect(text).toContain('$1,234.50')
    expect(text).toContain('$678.90')
  })

  it('quickly switches dashboard ranking range between daily and weekly natural days', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })
    await flushPromises()
    getSnapshotV2.mockClear()
    getUserUsageTrend.mockClear()
    getUserSpendingRanking.mockClear()

    const now = new Date()
    const sixDaysAgo = new Date()
    sixDaysAgo.setDate(sixDaysAgo.getDate() - 6)
    const today = formatLocalDate(now)
    const weekStart = formatLocalDate(sixDaysAgo)

    await wrapper.get('[data-testid="admin-dashboard-ranking-period-day"]').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: today,
      end_date: today,
      granularity: 'hour'
    }))
    expect(getUserSpendingRanking).toHaveBeenLastCalledWith({
      start_date: today,
      end_date: today,
      limit: 12
    })

    await wrapper.get('[data-testid="admin-dashboard-ranking-period-week"]').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: weekStart,
      end_date: today,
      granularity: 'day'
    }))
    expect(getUserUsageTrend).toHaveBeenLastCalledWith({
      start_date: weekStart,
      end_date: today,
      granularity: 'day',
      limit: 12
    })
    expect(getUserSpendingRanking).toHaveBeenLastCalledWith({
      start_date: weekStart,
      end_date: today,
      limit: 12
    })
  })
})
