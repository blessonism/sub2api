import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get, post } }))

import announcementsAPI from '@/api/admin/announcements'

describe('admin announcement email broadcast api', () => {
  beforeEach(() => { get.mockReset(); post.mockReset() })

  it('uses the broadcast management endpoints', async () => {
    get.mockResolvedValue({ data: { broadcast: null, eligible_count: 3, can_send: true } })
    post.mockResolvedValue({ data: { id: 9 } })

    await announcementsAPI.getEmailBroadcast(7)
    await announcementsAPI.createEmailBroadcast(7)
    await announcementsAPI.listEmailDeliveries(7, 2, 50, { status: 'failed', search: 'user' })
    await announcementsAPI.retryFailedEmailDeliveries(7)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/announcements/7/email-broadcast')
    expect(post).toHaveBeenNthCalledWith(1, '/admin/announcements/7/email-broadcast')
    expect(get).toHaveBeenNthCalledWith(2, '/admin/announcements/7/email-broadcast/deliveries', {
      params: { page: 2, page_size: 50, status: 'failed', search: 'user' }
    })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/announcements/7/email-broadcast/retry-failed')
  })
})
