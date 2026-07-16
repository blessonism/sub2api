import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AdminAffiliateLeaderboardView from '../AdminAffiliateLeaderboardView.vue'

const { listLeaderboard, showError } = vi.hoisted(() => ({
  listLeaderboard: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI: { listLeaderboard },
  default: { listLeaderboard },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountView() {
  return mount(AdminAffiliateLeaderboardView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: {
          props: ['data', 'loading'],
          template: `
            <div data-testid="rows">
              <div v-for="row in data" :key="row.user_id">
                <slot name="cell-rank" :row="row" />
                <slot name="cell-inviter" :row="row" />
                <slot name="cell-aff_code" :row="row" />
                <slot name="cell-invite_count" :row="row" />
                <slot name="cell-all_credit_amount" :row="row" />
                <slot name="cell-payment_redeem_amount" :row="row" />
              </div>
              <slot v-if="!loading && data.length === 0" name="empty" />
            </div>
          `,
        },
        Pagination: {
          props: ['page', 'total', 'pageSize'],
          emits: ['update:page'],
          template: '<button data-testid="next-page" @click="$emit(\'update:page\', 2)">next</button>',
        },
        Icon: { template: '<span />' },
      },
    },
  })
}

describe('AdminAffiliateLeaderboardView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listLeaderboard.mockReset()
    showError.mockReset()
    listLeaderboard.mockResolvedValue({
      items: [{
        rank: 7,
        user_id: 42,
        email: 'alice@example.com',
        username: 'Alice',
        aff_code: 'ALICE42',
        invite_count: 3,
        all_credit_amount: 120.5,
        payment_redeem_amount: 88,
      }],
      total: 21,
      page: 1,
      page_size: 20,
    })
  })

  afterEach(() => vi.useRealTimers())

  it('renders global rank and both cumulative amounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listLeaderboard).toHaveBeenCalledWith({ page: 1, page_size: 20, search: '' })
    expect(wrapper.text()).toContain('#7')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('ALICE42')
    expect(wrapper.text()).toContain('$120.50')
    expect(wrapper.text()).toContain('$88.00')
  })

  it('resets pagination when searching and loads the requested page', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()
    expect(listLeaderboard).toHaveBeenLastCalledWith({ page: 2, page_size: 20, search: '' })

    await wrapper.get('[data-testid="leaderboard-search"]').setValue(' alice ')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(listLeaderboard).toHaveBeenLastCalledWith({ page: 1, page_size: 20, search: 'alice' })
  })
})
