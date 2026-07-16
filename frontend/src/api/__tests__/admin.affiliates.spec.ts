import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { affiliatesAPI } from '@/api/admin/affiliates'

describe('admin affiliates api', () => {
  beforeEach(() => get.mockReset())

  it('loads the global invite leaderboard with pagination and search', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0, page: 2, page_size: 50 } })

    await affiliatesAPI.listLeaderboard({ page: 2, page_size: 50, search: 'alice' })

    expect(get).toHaveBeenCalledWith('/admin/affiliates/leaderboard', {
      params: { page: 2, page_size: 50, search: 'alice' },
    })
  })
})
