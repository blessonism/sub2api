import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put,
    delete: del,
  },
}))

import { enrollLotteryCampaign, getActiveLotteryCampaign, getMyLotteryCampaignData, getRecentLotteryWinners } from '@/api/lotteryCampaigns'
import {
  createLotteryCampaign,
  cancelLotteryCampaign,
  deleteLotteryCampaign,
  drawLotteryCampaign,
  featureLotteryCampaign,
  listLotteryCampaigns,
  listLotteryWinners,
  publishLotteryCampaign,
  syncLotteryEntries,
  updateLotteryCampaign,
} from '@/api/admin/lotteryCampaigns'

describe('lottery campaigns api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('calls user lottery endpoints', async () => {
    get.mockResolvedValueOnce({ data: { campaign: null } })
    await expect(getActiveLotteryCampaign()).resolves.toEqual({ campaign: null })
    expect(get).toHaveBeenCalledWith('/lottery-campaigns/active')

    get.mockResolvedValueOnce({ data: { campaign: { id: 7 }, today_tokens: 100, threshold_tokens: 100, entry_count: 1, entry_status: 'enrolled', winners: [] } })
    await getMyLotteryCampaignData(7)
    expect(get).toHaveBeenCalledWith('/lottery-campaigns/7/me')

    post.mockResolvedValueOnce({ data: { id: 1 } })
    await enrollLotteryCampaign(7)
    expect(post).toHaveBeenCalledWith('/lottery-campaigns/7/enroll')

    get.mockResolvedValueOnce({ data: { items: [] } })
    await getRecentLotteryWinners(7)
    expect(get).toHaveBeenCalledWith('/lottery-campaigns/7/winners', { params: undefined })

    get.mockResolvedValueOnce({ data: { items: [] } })
    await getRecentLotteryWinners(7, 5)
    expect(get).toHaveBeenCalledWith('/lottery-campaigns/7/winners', { params: { limit: 5 } })
  })

  it('calls admin lottery endpoints', async () => {
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 } })
    await listLotteryCampaigns()
    expect(get).toHaveBeenCalledWith('/admin/lottery-campaigns', { params: { page: 1, page_size: 20 } })

    const payload = {
      name: 'Token Lottery',
      participation_mode: 'auto' as const,
      draw_schedule_type: 'single' as const,
      prize_mode: 'single' as const,
      entry_mode: 'daily_once' as const,
      threshold_tokens: 100,
      entry_step_tokens: 0,
      max_entries_per_user: 1,
      start_at: '2026-07-08T00:00:00Z',
      end_at: '2026-07-09T00:00:00Z',
      draw_at: '2026-07-08T20:00:00Z',
      prize_tiers: [{ tier_name: 'Prize', winner_count: 1, reward_amount_cents: 100, sort_order: 1 }],
    }
    post.mockResolvedValueOnce({ data: { id: 7 } })
    await createLotteryCampaign(payload)
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns', payload)

    put.mockResolvedValueOnce({ data: { id: 7 } })
    await updateLotteryCampaign(7, payload)
    expect(put).toHaveBeenCalledWith('/admin/lottery-campaigns/7', payload)

    post.mockResolvedValueOnce({ data: { id: 7, status: 'published' } })
    await publishLotteryCampaign(7)
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns/7/publish')

    post.mockResolvedValueOnce({ data: { id: 7, status: 'cancelled' } })
    await cancelLotteryCampaign(7)
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns/7/cancel')

    post.mockResolvedValueOnce({ data: { id: 7, is_featured: true } })
    await featureLotteryCampaign(7)
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns/7/feature')

    del.mockResolvedValueOnce({ data: { deleted: true } })
    await deleteLotteryCampaign(7)
    expect(del).toHaveBeenCalledWith('/admin/lottery-campaigns/7')

    post.mockResolvedValueOnce({ data: { synced: 3 } })
    await syncLotteryEntries(7, '2026-07-08')
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns/7/sync-entries', undefined, { params: { date: '2026-07-08' } })

    post.mockResolvedValueOnce({ data: { id: 9 } })
    await drawLotteryCampaign(7, '2026-07-08')
    expect(post).toHaveBeenCalledWith('/admin/lottery-campaigns/7/draw', undefined, { params: { date: '2026-07-08' } })

    get.mockResolvedValueOnce({ data: { items: [] } })
    await listLotteryWinners(7)
    expect(get).toHaveBeenCalledWith('/admin/lottery-campaigns/7/winners')

    get.mockResolvedValueOnce({ data: { items: [] } })
    await listLotteryWinners(7, 9)
    expect(get).toHaveBeenCalledWith('/admin/lottery-campaigns/7/draw-batches/9/winners')
  })
})
