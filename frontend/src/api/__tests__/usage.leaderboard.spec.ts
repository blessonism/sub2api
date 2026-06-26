import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { getDashboardLeaderboard, type UserTokenLeaderboardResponse } from '@/api/usage'

describe('usage leaderboard api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('requests the user dashboard leaderboard endpoint', async () => {
    const response: UserTokenLeaderboardResponse = {
      ranking: [
        {
          rank: 1,
          masked_email: 'a***a@example.com',
          requests: 10,
          tokens: 1000,
          discount_rate_multiplier: 0.8,
          is_current_user: false,
        },
      ],
      my_rank: {
        rank: 11,
        masked_email: 'm***e@example.com',
        requests: 2,
        tokens: 80,
        discount_rate_multiplier: 0.7,
        is_current_user: true,
      },
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10,
      period: 'day',
    }
    get.mockResolvedValue({ data: response })

    const result = await getDashboardLeaderboard()

    expect(get).toHaveBeenCalledWith('/usage/dashboard/leaderboard', { params: undefined })
    expect(result).toEqual(response)
  })

  it('passes the requested leaderboard period', async () => {
    const response: UserTokenLeaderboardResponse = {
      ranking: [],
      my_rank: {
        rank: 0,
        masked_email: 'm***e@example.com',
        requests: 0,
        tokens: 0,
        discount_rate_multiplier: 1,
        is_current_user: true,
      },
      start_date: '2026-06-14',
      end_date: '2026-06-20',
      limit: 10,
      period: 'week',
    }
    get.mockResolvedValue({ data: response })

    const result = await getDashboardLeaderboard({ period: 'week' })

    expect(get).toHaveBeenCalledWith('/usage/dashboard/leaderboard', { params: { period: 'week' } })
    expect(result).toEqual(response)
  })

  it('passes the requested last 7 days leaderboard period', async () => {
    const response: UserTokenLeaderboardResponse = {
      ranking: [],
      my_rank: {
        rank: 0,
        masked_email: 'm***e@example.com',
        requests: 0,
        tokens: 0,
        is_current_user: true,
      },
      start_date: '2026-06-14',
      end_date: '2026-06-20',
      limit: 10,
      period: 'last7d',
    }
    get.mockResolvedValue({ data: response })

    const result = await getDashboardLeaderboard({ period: 'last7d' })

    expect(get).toHaveBeenCalledWith('/usage/dashboard/leaderboard', { params: { period: 'last7d' } })
    expect(result).toEqual(response)
  })
})
