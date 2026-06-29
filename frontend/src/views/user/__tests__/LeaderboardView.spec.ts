import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UserTokenLeaderboardResponse } from '@/api/usage'
import LeaderboardView from '../LeaderboardView.vue'

const { getDashboardLeaderboard } = vi.hoisted(() => ({
  getDashboardLeaderboard: vi.fn(),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardLeaderboard,
  },
}))

const messages: Record<string, string> = {
  'common.refresh': 'Refresh',
  'leaderboard.title': 'Leaderboard',
  'leaderboard.todayRange': 'Today: {date}',
  'leaderboard.weekRange': 'This week: {start} - {end}',
  'leaderboard.last7dRange': 'Last 7 days: {start} - {end}',
  'leaderboard.periodDay': 'Daily',
  'leaderboard.periodWeek': 'Weekly',
  'leaderboard.periodLast7d': 'Last 7 days',
  'leaderboard.myRank': 'My Rank',
  'leaderboard.discountRate': 'Tier Multiplier',
  'leaderboard.tokens': 'Tokens',
  'leaderboard.requests': 'Requests',
  'leaderboard.topUsers': 'Top {limit}',
  'leaderboard.topTokens': 'Top Tokens',
  'leaderboard.leaderboard': 'Leaderboard',
  'leaderboard.today': 'Today',
  'leaderboard.rank': 'Rank',
  'leaderboard.user': 'User',
  'leaderboard.unranked': 'Unranked',
  'leaderboard.failedToLoad': 'Failed to load leaderboard',
  'leaderboard.retry': 'Retry',
  'leaderboard.noData': 'No usage today',
  'leaderboard.noDataDescription': 'No token usage has been recorded today.',
  'leaderboard.noDataWeek': 'No usage this week',
  'leaderboard.noDataWeekDescription': 'No token usage has been recorded this week.',
  'leaderboard.noDataLast7d': 'No usage in the last 7 days',
  'leaderboard.noDataLast7dDescription': 'No token usage has been recorded in the last 7 natural days.',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let text = messages[key] ?? key
        for (const [name, value] of Object.entries(params ?? {})) {
          text = text.replace(`{${name}}`, String(value))
        }
        return text
      },
    }),
  }
})

function mountView() {
  return mount(LeaderboardView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: { template: '<div data-testid="loading" />' },
        HelpTooltip: {
          props: ['content'],
          template: '<span data-testid="help-tooltip">{{ content }}</span>',
        },
        EmptyState: {
          props: ['title', 'description'],
          template: '<div data-testid="empty">{{ title }} {{ description }}</div>',
        },
      },
    },
  })
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

describe('LeaderboardView', () => {
  beforeEach(() => {
    getDashboardLeaderboard.mockReset()
  })

  it('renders loading state while fetching leaderboard data', () => {
    getDashboardLeaderboard.mockReturnValue(new Promise(() => undefined))

    const wrapper = mountView()

    expect(wrapper.find('[data-testid="loading"]').exists()).toBe(true)
  })

  it('renders masked leaderboard rows and current user rank', async () => {
    getDashboardLeaderboard.mockResolvedValue({
      ranking: [
        {
          rank: 1,
          masked_email: 'alp***ha@example.com',
          requests: 10,
          tokens: 1_200_000,
          discount_rate_multiplier: 0.6,
          is_current_user: false,
        },
        {
          rank: 2,
          masked_email: 'bet***ta@example.com',
          requests: 8,
          tokens: 50_000,
          discount_rate_multiplier: 0.8,
          is_current_user: false,
        },
      ],
      my_rank: {
        rank: 11,
        masked_email: 'min***ne@example.com',
        requests: 2,
        tokens: 180_000,
        discount_rate_multiplier: 0.7,
        is_current_user: true,
      },
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10,
      period: 'day',
      tier_tooltip: '管理员配置的阶梯倍率规则',
    })

    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('alp***ha@example.com')
    expect(text).toContain('min***ne@example.com')
    expect(text).toContain('Top 10')
    expect(text).toContain('Today: 2026-06-18')
    expect(getDashboardLeaderboard).toHaveBeenCalledWith({ period: 'day' })
    expect(text).toContain('#11')
    expect(text).toContain('0.70x')
    expect(text).toContain('0.18M')
    expect(text).toContain('1.25M')
    expect(wrapper.get('[data-testid="help-tooltip"]').text()).toContain('管理员配置的阶梯倍率规则')
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('0.60x')
    expect(rows[0].text()).toContain('1.20M')
    expect(rows[1].text()).toContain('0.80x')
    expect(rows[1].text()).toContain('0.05M')
    expect(text).not.toContain('alpha@example.com')
    expect(text).not.toContain('mine@example.com')
  })

  it('renders empty state when no user has token usage today', async () => {
    getDashboardLeaderboard.mockResolvedValue({
      ranking: [],
      my_rank: {
        rank: 0,
        masked_email: 'min***ne@example.com',
        requests: 0,
        tokens: 0,
        discount_rate_multiplier: null,
        is_current_user: true,
      },
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10,
      period: 'day',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="empty"]').text()).toContain('No usage today')
    expect(wrapper.text()).toContain('Unranked')
    expect(wrapper.text()).toContain('-')
  })

  it('switches to weekly leaderboard and updates empty copy', async () => {
    getDashboardLeaderboard
      .mockResolvedValueOnce({
        ranking: [
          {
            rank: 1,
            masked_email: 'alp***ha@example.com',
            requests: 10,
            tokens: 1_200_000,
            discount_rate_multiplier: 0.8,
            is_current_user: false,
          },
        ],
        my_rank: {
          rank: 3,
          masked_email: 'min***ne@example.com',
          requests: 2,
          tokens: 180_000,
          discount_rate_multiplier: 0.7,
          is_current_user: true,
        },
        start_date: '2026-06-20',
        end_date: '2026-06-20',
        limit: 10,
        period: 'day',
      })
      .mockResolvedValueOnce({
        ranking: [],
        my_rank: {
          rank: 0,
          masked_email: 'min***ne@example.com',
          requests: 0,
          tokens: 0,
          discount_rate_multiplier: null,
          is_current_user: true,
        },
        start_date: '2026-06-15',
        end_date: '2026-06-21',
        limit: 10,
        period: 'week',
      })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === 'Weekly')!.trigger('click')
    await flushPromises()

    expect(getDashboardLeaderboard).toHaveBeenLastCalledWith({ period: 'week' })
    expect(wrapper.text()).toContain('This week: 2026-06-15 - 2026-06-21')
    expect(wrapper.get('[data-testid="empty"]').text()).toContain('No usage this week')
  })

  it('switches to last 7 days leaderboard independently from weekly leaderboard', async () => {
    getDashboardLeaderboard
      .mockResolvedValueOnce({
        ranking: [],
        my_rank: {
          rank: 0,
          masked_email: 'min***ne@example.com',
          requests: 0,
          tokens: 0,
          discount_rate_multiplier: null,
          is_current_user: true,
        },
        start_date: '2026-06-20',
        end_date: '2026-06-20',
        limit: 10,
        period: 'day',
      })
      .mockResolvedValueOnce({
        ranking: [],
        my_rank: {
          rank: 0,
          masked_email: 'min***ne@example.com',
          requests: 0,
          tokens: 0,
          discount_rate_multiplier: null,
          is_current_user: true,
        },
        start_date: '2026-06-14',
        end_date: '2026-06-20',
        limit: 10,
        period: 'last7d',
      })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === 'Last 7 days')!.trigger('click')
    await flushPromises()

    expect(getDashboardLeaderboard).toHaveBeenLastCalledWith({ period: 'last7d' })
    expect(wrapper.text()).toContain('Last 7 days: 2026-06-14 - 2026-06-20')
    expect(wrapper.get('[data-testid="empty"]').text()).toContain('No usage in the last 7 days')
  })

  it('keeps the latest weekly leaderboard when the previous daily request resolves late', async () => {
    const dailyRequest = createDeferred<UserTokenLeaderboardResponse>()
    const weeklyRequest = createDeferred<UserTokenLeaderboardResponse>()
    getDashboardLeaderboard
      .mockReturnValueOnce(dailyRequest.promise)
      .mockReturnValueOnce(weeklyRequest.promise)

    const wrapper = mountView()

    await wrapper.findAll('button').find((button) => button.text() === 'Weekly')!.trigger('click')
    expect(getDashboardLeaderboard).toHaveBeenNthCalledWith(1, { period: 'day' })
    expect(getDashboardLeaderboard).toHaveBeenNthCalledWith(2, { period: 'week' })

    weeklyRequest.resolve({
      ranking: [
        {
          rank: 1,
          masked_email: 'wee***ly@example.com',
          requests: 70,
          tokens: 7_000_000,
          discount_rate_multiplier: 0.5,
          is_current_user: true,
        },
      ],
      my_rank: {
        rank: 1,
        masked_email: 'wee***ly@example.com',
        requests: 70,
        tokens: 7_000_000,
        discount_rate_multiplier: 0.5,
        is_current_user: true,
      },
      start_date: '2026-06-15',
      end_date: '2026-06-21',
      limit: 10,
      period: 'week',
    })
    await flushPromises()

    expect(wrapper.text()).toContain('This week: 2026-06-15 - 2026-06-21')
    expect(wrapper.text()).toContain('wee***ly@example.com')

    dailyRequest.resolve({
      ranking: [
        {
          rank: 1,
          masked_email: 'dai***ly@example.com',
          requests: 10,
          tokens: 1_000_000,
          discount_rate_multiplier: 0.8,
          is_current_user: true,
        },
      ],
      my_rank: {
        rank: 1,
        masked_email: 'dai***ly@example.com',
        requests: 10,
        tokens: 1_000_000,
        discount_rate_multiplier: 0.8,
        is_current_user: true,
      },
      start_date: '2026-06-20',
      end_date: '2026-06-20',
      limit: 10,
      period: 'day',
    })
    await flushPromises()

    expect(wrapper.text()).toContain('This week: 2026-06-15 - 2026-06-21')
    expect(wrapper.text()).toContain('wee***ly@example.com')
    expect(wrapper.text()).not.toContain('dai***ly@example.com')
  })

  it('renders error state when leaderboard request fails', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    getDashboardLeaderboard.mockRejectedValue(new Error('boom'))

    try {
      const wrapper = mountView()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load leaderboard')
      expect(wrapper.text()).toContain('Retry')
    } finally {
      errorSpy.mockRestore()
    }
  })
})
