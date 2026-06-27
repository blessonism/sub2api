import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { mount } from '@vue/test-utils'

import BalanceRedemptionView from '../BalanceRedemptionView.vue'

const routeState = reactive({
  query: {} as Record<string, unknown>
})

const replace = vi.fn((location: { query?: Record<string, unknown> }) => {
  routeState.query = location.query || {}
})

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ replace })
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

const mountView = () =>
  mount(BalanceRedemptionView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        KeepAlive: { template: '<slot />' },
        BalanceSummaryView: { template: '<div data-testid="balance-summary"></div>' },
        RedeemRecordsView: { template: '<div data-testid="redeem-records"></div>' }
      }
    }
  })

describe('BalanceRedemptionView', () => {
  beforeEach(() => {
    routeState.query = {}
    replace.mockClear()
  })

  it('默认展示余额汇总标签', () => {
    const wrapper = mountView()

    expect(wrapper.find('[data-testid="balance-summary"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="redeem-records"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.balanceRedemption.tabs.balanceSummary')
  })

  it('根据 query 展示兑换记录标签', () => {
    routeState.query = { tab: 'redeem-records' }
    const wrapper = mountView()

    expect(wrapper.find('[data-testid="balance-summary"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="redeem-records"]').exists()).toBe(true)
  })

  it('点击标签时更新统一路由 query', async () => {
    const wrapper = mountView()
    const buttons = wrapper.findAll('button.tab')

    await buttons[1].trigger('click')
    expect(replace).toHaveBeenCalledWith({
      path: '/admin/balance-redemption',
      query: { tab: 'redeem-records' }
    })

    await buttons[0].trigger('click')
    expect(replace).toHaveBeenLastCalledWith({
      path: '/admin/balance-redemption',
      query: {}
    })
  })
})
