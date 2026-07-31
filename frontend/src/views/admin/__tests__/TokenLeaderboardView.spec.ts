import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import TokenLeaderboardView from '../TokenLeaderboardView.vue'

const {
  getAdminTokenLeaderboard,
  getAdminTokenLeaderboardUserDetails,
  grantAdminTokenLeaderboardBalance,
  getSettings,
  updateTokenLeaderboardSettings,
  getAllIncludingInactive,
  getUserBalanceHistory,
  getUserById,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getAdminTokenLeaderboard: vi.fn(),
  getAdminTokenLeaderboardUserDetails: vi.fn(),
  grantAdminTokenLeaderboardBalance: vi.fn(),
  getSettings: vi.fn(),
  updateTokenLeaderboardSettings: vi.fn(),
  getAllIncludingInactive: vi.fn(),
  getUserBalanceHistory: vi.fn(),
  getUserById: vi.fn(),
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
    settings: {
      getSettings,
      updateTokenLeaderboardSettings
    },
    groups: {
      getAllIncludingInactive
    },
    users: {
      getById: getUserById,
      getUserBalanceHistory
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
        UserBalanceHistoryModal: {
          props: {
            show: Boolean,
            user: Object,
            hideActions: Boolean
          },
          emits: ['close'],
          template: `
            <div v-if="show" data-testid="balance-history-modal">
              <span>{{ user?.email }}</span>
              <span>{{ hideActions ? 'hide-actions' : 'show-actions' }}</span>
              <button type="button" data-testid="close-history" @click="$emit('close')">close</button>
            </div>
          `
        },
        Select: defineComponent({
          props: {
            modelValue: {
              type: [String, Number, Boolean, null],
              default: ''
            },
            options: {
              type: Array,
              default: () => []
            }
          },
          emits: ['update:modelValue', 'change'],
          setup(props, { emit }) {
            const onChange = (event: Event) => {
              const target = event.target as HTMLSelectElement
              const option = (props.options as Array<Record<string, unknown>>).find((item) => String(item.value ?? '') === target.value) ?? null
              const value = option ? option.value : target.value
              emit('update:modelValue', value)
              emit('change', value, option)
            }

            return () => h(
              'select',
              {
                value: props.modelValue ?? '',
                onChange
              },
              (props.options as Array<Record<string, unknown>>).map((option) => h(
                'option',
                {
                  key: `${String(option.value ?? '')}:${String(option.label ?? '')}`,
                  value: option.value as string | number | boolean | null
                },
                String(option.label ?? '')
              ))
            )
          }
        })
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
    getSettings.mockReset()
    updateTokenLeaderboardSettings.mockReset()
    getUserBalanceHistory.mockReset()
    getUserById.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getAllIncludingInactive.mockResolvedValue([
      { id: 3, name: 'VIP', rate_multiplier: 1.25, status: 'active', subscription_type: 'standard', is_exclusive: false },
      { id: 4, name: 'Hidden', rate_multiplier: 0.9, status: 'inactive', subscription_type: 'standard', is_exclusive: false },
      { id: 5, name: 'Exclusive', rate_multiplier: 0.8, status: 'active', subscription_type: 'standard', is_exclusive: true },
    ])
    getSettings.mockResolvedValue({
      token_leaderboard_common_group_id: 0,
      token_leaderboard_tier_tooltip: ''
    })
    updateTokenLeaderboardSettings.mockResolvedValue({
      token_leaderboard_common_group_id: 4,
      token_leaderboard_tier_tooltip: '按最近用量匹配阶梯'
    })
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
      calibration_balance_delta: -2.5,
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
    getUserById.mockResolvedValue({
      id: 7,
      email: 'alice@example.com',
      username: 'alice',
      role: 'user',
      status: 'active',
      balance: 42.5,
      concurrency: 2,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-06-18T08:30:00Z',
      deleted_at: null,
      notes: ''
    })
    getUserBalanceHistory.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 15, total_recharged: 0 })
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

  it('switches the admin token leaderboard to the current week range', async () => {
    const wrapper = mountView()
    await flushPromises()

    const weekButton = wrapper.findAll('button').find((button) => button.text() === 'admin.tokenLeaderboard.weekShortcut')
    expect(weekButton).toBeTruthy()
    await weekButton!.trigger('click')
    await flushPromises()

    expect(getAdminTokenLeaderboard).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-06-15',
      end_date: '2026-06-21',
      limit: 10
    }))
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

    const detailButton = wrapper.findAll('tbody button').find((button) => button.attributes('title') !== 'admin.users.balanceHistoryTip')
    expect(detailButton).toBeTruthy()
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
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.calibrationBalance')
    expect(wrapper.text()).toContain('-2.5000')
    expect(wrapper.text()).toContain('admin.tokenLeaderboard.requests')
    expect(wrapper.text()).toContain('2')
  })

  it('opens recharge and concurrency history when clicking the user email', async () => {
    const wrapper = mountView()
    await flushPromises()

    const userButton = wrapper.find('button[title="admin.users.balanceHistoryTip"]')
    expect(userButton.exists()).toBe(true)
    await userButton.trigger('click')
    await flushPromises()

    expect(getUserById).toHaveBeenCalledWith(7, true)
    expect(wrapper.get('[data-testid="balance-history-modal"]').text()).toContain('alice@example.com')
    expect(wrapper.get('[data-testid="balance-history-modal"]').text()).toContain('hide-actions')
  })

  it('reloads per-user details after refreshing the leaderboard', async () => {
    const wrapper = mountView()
    await flushPromises()

    const detailButton = wrapper.findAll('tbody button').find((button) => button.attributes('title') !== 'admin.users.balanceHistoryTip')
    expect(detailButton).toBeTruthy()
    await detailButton.trigger('click')
    await flushPromises()

    await detailButton.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === 'admin.tokenLeaderboard.refresh')!.trigger('click')
    await flushPromises()

    const refreshedDetailButton = wrapper.findAll('tbody button').find((button) => button.attributes('title') !== 'admin.users.balanceHistoryTip')
    expect(refreshedDetailButton).toBeTruthy()
    await refreshedDetailButton.trigger('click')
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

  it('saves the common fallback group and reloads leaderboard settings', async () => {
    const wrapper = mountView()
    await flushPromises()

    const select = wrapper.get('[data-testid="token-leaderboard-common-group-select"]')
    expect(select).toBeTruthy()
    await select.setValue('3')
    await flushPromises()

    const saveButton = wrapper.get('[data-testid="token-leaderboard-common-group-save"]')
    await saveButton.trigger('click')
    await flushPromises()

    expect(updateTokenLeaderboardSettings).toHaveBeenCalledWith({
      token_leaderboard_common_group_id: 3,
      token_leaderboard_tier_tooltip: ''
    })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('saves the tier multiplier tooltip copy', async () => {
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.get('[data-testid="token-leaderboard-tier-tooltip-edit"]')
    await editButton.trigger('click')
    await flushPromises()

    const tooltipInput = wrapper.get('[data-testid="token-leaderboard-tier-tooltip-input"]')
    await tooltipInput.setValue(' 按最近用量匹配阶梯倍率 ')
    await flushPromises()

    const confirmButton = wrapper.get('[data-testid="token-leaderboard-tier-tooltip-confirm"]')
    await confirmButton.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="token-leaderboard-tier-tooltip-status"]').text()).toContain('admin.tokenLeaderboard.tierTooltipConfigured')

    const saveButton = wrapper.get('[data-testid="token-leaderboard-common-group-save"]')
    await saveButton.trigger('click')
    await flushPromises()

    expect(updateTokenLeaderboardSettings).toHaveBeenCalledWith({
      token_leaderboard_common_group_id: 0,
      token_leaderboard_tier_tooltip: '按最近用量匹配阶梯倍率'
    })
    expect(showSuccess).toHaveBeenCalled()
  })
})
