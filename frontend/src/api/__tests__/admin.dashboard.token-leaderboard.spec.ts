import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post
  }
}))

import {
  grantAdminTokenLeaderboardBalance,
  getAdminTokenLeaderboard,
  getAdminTokenLeaderboardUserDetails,
  type AdminTokenLeaderboardParams
} from '@/api/admin/dashboard'

describe('admin dashboard token leaderboard api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('calls the admin token leaderboard endpoint with filters', async () => {
    const params: AdminTokenLeaderboardParams = {
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      email: 'alice',
      group_id: 3,
      model: 'claude',
      user_status: 'active',
      limit: 20
    }
    const response = {
      ranking: [],
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      total_account_cost: 0,
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 20
    }
    get.mockResolvedValue({ data: response })

    await expect(getAdminTokenLeaderboard(params)).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/dashboard/token-leaderboard', { params })
  })

  it('calls the per-user detail endpoint', async () => {
    const response = {
      user_id: 7,
      api_keys: [],
      groups: [],
      models: [],
      start_date: '2026-06-18',
      end_date: '2026-06-18'
    }
    get.mockResolvedValue({ data: response })

    await expect(getAdminTokenLeaderboardUserDetails(7, { limit: 50 })).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/dashboard/token-leaderboard/users/7/details', {
      params: { limit: 50 }
    })
  })

  it('calls the Top10 grant balance endpoint', async () => {
    const params: AdminTokenLeaderboardParams = {
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10
    }
    const request = {
      user_ids: [7, 8],
      amount: 3.5,
      notes: 'campaign bonus'
    }
    const response = {
      granted_count: 2,
      amount: 3.5,
      users: []
    }
    post.mockResolvedValue({ data: response })

    await expect(grantAdminTokenLeaderboardBalance(params, request)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/dashboard/token-leaderboard/grant-balance', request, {
      params
    })
  })
})
