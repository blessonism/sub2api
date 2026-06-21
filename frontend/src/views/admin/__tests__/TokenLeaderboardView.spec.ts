import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import TokenLeaderboardView from '../TokenLeaderboardView.vue'

const {
  getAdminTokenLeaderboard,
  getAdminTokenLeaderboardUserDetails,
  grantAdminTokenLeaderboardBalance,
  getAllIncludingInactive,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getAdminTokenLeaderboard: vi.fn(),
  getAdminTokenLeaderboardUserDetails: vi.fn(),
  grantAdminTokenLeaderboardBalance: vi.fn(),
  getAllIncludingInactive: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getAdminTokenLeaderboard,
      getAdminTokenLeaderboardUserDetails,
      grantAdminTokenLeaderboardBalance
    },
    groups: {
      getAllIncludingInactive
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
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
  mount(TokenLeaderboardView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DateRangePicker: { template: '<div data-testid="date-range"></div>' },
        EmptyState: { template: '<div data-testid="empty-state"></div>' },
        Icon: { template: '<span />' },
        LoadingSpinner: { template: '<div data-testid="loading"></div>' },
        BaseDialog: { template: '<div v-if="show" data-testid="dialog"><slot /><slot name="footer" /></div>', props: ['show', 'title'] },
        Select: {
          props: ['modelValue', 'options'],
          template: '<select :value="modelValue"></select>'
        }
      }
    }
  })

describe('TokenLeaderboardView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-18T10:00:00+08:00'))
    getAllIncludingInactive.mockReset()
    getAdminTokenLeaderboard.mockReset()
    getAdminTokenLeaderboardUserDetails.mockReset()
    grantAdminTokenLeaderboardBalance.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getAllIncludingInactive.mockResolvedValue([{ id: 3, name: 'VIP' }])
    getAdminTokenLeaderboard.mockResolvedValue({
      ranking: [
        {
          rank: 1,
          user_id: 7,
          email: 'alice@example.com',
          username: 'alice',
          status: 'active',
          registered_at: '2026-01-01T00:00:00Z',
          last_used_at: '2026-06-18T08:30:00Z',
          requests: 5,
          tokens: 1500,
          cost: 2.2,
          actual_cost: 1.8,
          account_cost: 1.1
        }
      ],
      total_requests: 5,
      total_tokens: 1500,
      total_cost: 2.2,
      total_actual_cost: 1.8,
      total_account_cost: 1.1,
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10
    })
    getAdminTokenLeaderboardUserDetails.mockResolvedValue({
      user_id: 7,
      calibration_tokens: -120,
      api_keys: [{ api_key_id: 9, api_key_name: 'prod', requests: 2, tokens: 900, cost: 1, actual_cost: 0.8, account_cost: 0.5 }],
      groups: [],
      models: [],
      start_date: '2026-06-18',
      end_date: '2026-06-18'
    })
    grantAdminTokenLeaderboardBalance.mockResolvedValue({
      granted_count: 1,
      amount: 3.5,
      users: [
        {
          user_id: 7,
          email: 'alice@example.com',
          username: 'alice',
          balance: 42.5,
          granted_amount: 3.5
        }
      ]
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads today admin leaderboard and renders full email', async () => {
    const wrapper = mountView()

    await flushPromises()

    expect(getAdminTokenLeaderboard).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10
    }))
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.lastUsedAt')
    expect(wrapper.text()).not.toContain('admin.tokenLeaderboard.cost')
    expect(wrapper.text()).not.toContain('admin.tokenLeaderboard.accountCost')
  })

  it('hides user ids by default and toggles them on demand', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.tokenLeaderboard.showUserIds')
    expect(wrapper.text()).not.toContain('admin.tokenLeaderboard.userId 7')

    const toggleButton = wrapper.findAll('button').find((button) => button.text() === 'admin.tokenLeaderboard.showUserIds')
    expect(toggleButton).toBeTruthy()
    await toggleButton!.trigger('click')

    expect(wrapper.text()).toContain('admin.tokenLeaderboard.hideUserIds')
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.userId 7')
  })

  it('loads per-user details when expanding a row', async () => {
    const wrapper = mountView()
    await flushPromises()

    const detailButton = wrapper.find('tbody button')
    await detailButton.trigger('click')
    await flushPromises()

    expect(getAdminTokenLeaderboardUserDetails).toHaveBeenCalledWith(7, expect.objectContaining({
      start_date: '2026-06-18',
      end_date: '2026-06-18',
      limit: 10
    }))
    expect(wrapper.text()).toContain('prod (#9)')
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.calibrationTokens')
    expect(wrapper.text()).toContain('-120')
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.requests')
    expect(wrapper.text()).toContain('2')
  })

  it('reloads per-user details after refreshing the leaderboard', async () => {
    const wrapper = mountView()
    await flushPromises()

    const detailButton = wrapper.find('tbody button')
    await detailButton.trigger('click')
    await flushPromises()

    await detailButton.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === 'admin.tokenLeaderboard.refresh')!.trigger('click')
    await flushPromises()

    await wrapper.find('tbody button').trigger('click')
    await flushPromises()

    expect(getAdminTokenLeaderboardUserDetails).toHaveBeenCalledTimes(2)
  })

  it('allows selecting Top10 users and granting balance', async () => {
    const wrapper = mountView()
    await flushPromises()

    const checkbox = wrapper.find('tbody input[type="checkbox"]')
    await checkbox.setChecked(true)

    const amountInput = wrapper.find('input[type="number"]')
    await amountInput.setValue('3.5')
    const notesInput = wrapper.find('input[type="text"]')
    await notesInput.setValue('campaign bonus')

    const grantButton = wrapper.findAll('button').find((button) => button.text() === 'admin.tokenLeaderboard.grantBalance')
    expect(grantButton).toBeTruthy()
    await grantButton!.trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find((button) => button.text() === 'common.confirm')
    expect(confirmButton).toBeTruthy()
    await confirmButton!.trigger('click')
    await flushPromises()

    expect(grantAdminTokenLeaderboardBalance).toHaveBeenCalledWith(
      expect.objectContaining({
        start_date: '2026-06-18',
        end_date: '2026-06-18',
        limit: 10
      }),
      {
        user_ids: [7],
        amount: 3.5,
        notes: 'campaign bonus'
      }
    )
    expect(showSuccess).toHaveBeenCalled()
  })
})
