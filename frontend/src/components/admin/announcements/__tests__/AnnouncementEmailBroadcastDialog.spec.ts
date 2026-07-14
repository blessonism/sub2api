import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getEmailBroadcast: vi.fn(),
  createEmailBroadcast: vi.fn(),
  listEmailDeliveries: vi.fn(),
  retryFailedEmailDeliveries: vi.fn()
}))

vi.mock('@/api/admin', () => ({ adminAPI: { announcements: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string, params?: { count?: number }) => params?.count ? `${key}:${params.count}` : key })
}))
vi.mock('@/composables/usePersistedPageSize', () => ({ getPersistedPageSize: () => 20 }))

import AnnouncementEmailBroadcastDialog from '../AnnouncementEmailBroadcastDialog.vue'

const page = { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
const runningBroadcast = {
  id: 1,
  announcement_id: 7,
  status: 'running' as const,
  total_count: 3,
  pending_count: 3,
  sent_count: 0,
  failed_count: 0,
  created_at: '2026-07-13T00:00:00Z'
}

function mountDialog() {
  return mount(AnnouncementEmailBroadcastDialog, {
    props: { show: true, announcementId: 7 },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot/><slot name="footer"/></div>' },
        DataTable: { template: '<div />' },
        Pagination: true,
        Select: true,
        Icon: true
      }
    }
  })
}

describe('AnnouncementEmailBroadcastDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.values(api).forEach(mock => mock.mockReset())
    api.listEmailDeliveries.mockResolvedValue(page)
  })

  afterEach(() => vi.useRealTimers())

  it('confirms a preview and starts a single broadcast', async () => {
    api.getEmailBroadcast.mockResolvedValue({ broadcast: null, eligible_count: 3, can_send: true })
    api.createEmailBroadcast.mockResolvedValue(runningBroadcast)
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.announcements.emailBroadcast.confirmMessage:3')
    await wrapper.get('[data-testid="send-broadcast"]').trigger('click')
    await flushPromises()

    expect(api.createEmailBroadcast).toHaveBeenCalledWith(7)
    expect(api.listEmailDeliveries).toHaveBeenCalled()
  })

  it('polls running progress every two seconds', async () => {
    api.getEmailBroadcast.mockResolvedValue({ broadcast: runningBroadcast, eligible_count: 0, can_send: false })
    const wrapper = mountDialog()
    await flushPromises()
    expect(api.getEmailBroadcast).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()
    expect(api.getEmailBroadcast).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('discards a stale response after switching announcements', async () => {
    let resolveFirst!: (value: unknown) => void
    api.getEmailBroadcast.mockImplementation((id: number) => id === 7
      ? new Promise(resolve => { resolveFirst = resolve })
      : Promise.resolve({ broadcast: null, eligible_count: 8, can_send: true }))
    api.createEmailBroadcast.mockResolvedValue({ ...runningBroadcast, announcement_id: 8 })
    const wrapper = mountDialog()
    await wrapper.setProps({ announcementId: 8 })
    await flushPromises()
    expect(wrapper.text()).toContain('admin.announcements.emailBroadcast.confirmMessage:8')

    resolveFirst({ broadcast: null, eligible_count: 7, can_send: true })
    await flushPromises()
    expect(wrapper.text()).toContain('admin.announcements.emailBroadcast.confirmMessage:8')
    await wrapper.get('[data-testid="send-broadcast"]').trigger('click')
    await flushPromises()
    expect(api.createEmailBroadcast).toHaveBeenCalledWith(8)
  })
})
