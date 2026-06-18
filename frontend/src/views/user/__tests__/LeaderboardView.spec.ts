import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
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
  'leaderboard.myRank': 'My Rank',
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
        EmptyState: {
          props: ['title', 'description'],
          template: '<div data-testid="empty">{{ title }} {{ description }}</div>',
        },
      },
    },
  })
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
          masked_email: 'a***a@example.com',
          requests: 10,
          tokens: 1_200_000,
          is_current_user: false,
        },
        {
          rank: 2,
          masked_email: 'b***b@example.com',
          requests: 8,
          tokens: 50_000,
          is_current_user: false,
        },
      ],
      my_rank: {
        rank: 11,
        masked_email: 'm***e@example.com',
        requests: 2,
        tokens: 180_000,
        is_current_user: true,
      },
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10,
    })

    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('a***a@example.com')
    expect(text).toContain('m***e@example.com')
    expect(text).toContain('Top 10')
    expect(text).toContain('#11')
    expect(text).toContain('0.18M')
    expect(text).toContain('1.25M')
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('1.20M')
    expect(rows[1].text()).toContain('0.05M')
    expect(text).not.toContain('alpha@example.com')
    expect(text).not.toContain('mine@example.com')
  })

  it('renders empty state when no user has token usage today', async () => {
    getDashboardLeaderboard.mockResolvedValue({
      ranking: [],
      my_rank: {
        rank: 0,
        masked_email: 'm***e@example.com',
        requests: 0,
        tokens: 0,
        is_current_user: true,
      },
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="empty"]').text()).toContain('No usage today')
    expect(wrapper.text()).toContain('Unranked')
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
