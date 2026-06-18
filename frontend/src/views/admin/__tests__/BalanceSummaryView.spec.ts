import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BalanceSummaryView from '../BalanceSummaryView.vue'

const {
  getAdminBalanceSummary,
  updateAdminBalanceSummaryExclusions,
  listUsers,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getAdminBalanceSummary: vi.fn(),
  updateAdminBalanceSummaryExclusions: vi.fn(),
  listUsers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getAdminBalanceSummary,
      updateAdminBalanceSummaryExclusions
    },
    users: {
      list: listUsers
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
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key} ${JSON.stringify(params)}`
      }
    })
  }
})

const baseSummary = {
  total_users: 4,
  included_users: 3,
  excluded_user_count: 1,
  invalid_exclusions: 0,
  total_balance: 125.5,
  by_role: [
    { key: 'admin', user_count: 1, balance: 25, excluded_count: 0 },
    { key: 'user', user_count: 2, balance: 100.5, excluded_count: 1 }
  ],
  by_status: [
    { key: 'active', user_count: 2, balance: 75.25, excluded_count: 1 },
    { key: 'disabled', user_count: 1, balance: 50.25, excluded_count: 0 }
  ],
  excluded_user_ids: [7],
  excluded_users: [
    {
      user_id: 7,
      email: 'internal@example.com',
      username: 'internal',
      role: 'user',
      status: 'active',
      balance: 99,
      valid: true
    }
  ],
  generated_at: '2026-06-19T00:00:00Z'
}

const aliceUser = {
  id: 9,
  username: 'alice',
  email: 'alice@example.com',
  role: 'admin',
  balance: 42,
  concurrency: 0,
  status: 'disabled',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  notes: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
}

const mountView = () =>
  mount(BalanceSummaryView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        EmptyState: {
          props: ['title', 'description', 'actionText'],
          template: '<div data-testid="empty-state"><button v-if="actionText" @click="$emit(\'action\')">{{ actionText }}</button>{{ title }}{{ description }}</div>'
        },
        Icon: { template: '<span />' },
        LoadingSpinner: { template: '<div data-testid="loading"></div>' }
      }
    }
  })

describe('BalanceSummaryView', () => {
  beforeEach(() => {
    getAdminBalanceSummary.mockReset()
    updateAdminBalanceSummaryExclusions.mockReset()
    listUsers.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    vi.restoreAllMocks()

    getAdminBalanceSummary.mockResolvedValue({ ...baseSummary })
    updateAdminBalanceSummaryExclusions.mockResolvedValue({
      ...baseSummary,
      excluded_user_ids: [9],
      excluded_users: [
        {
          user_id: 9,
          email: 'alice@example.com',
          username: 'alice',
          role: 'admin',
          status: 'disabled',
          balance: 42,
          valid: true
        }
      ]
    })
    listUsers.mockResolvedValue({
      items: [aliceUser],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1
    })
  })

  it('loads and renders balance summary totals', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getAdminBalanceSummary).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('$125.50')
    expect(wrapper.text()).toContain('internal@example.com')
    expect(wrapper.text()).toContain('admin.balanceSummary.scopeNotice')
  })

  it('refreshes summary from the one-click refresh button', async () => {
    const wrapper = mountView()
    await flushPromises()

    const refreshButton = wrapper.findAll('button').find((button) => button.text() === 'admin.balanceSummary.refresh')
    expect(refreshButton).toBeTruthy()
    await refreshButton!.trigger('click')
    await flushPromises()

    expect(getAdminBalanceSummary).toHaveBeenCalledTimes(2)
  })

  it('does not refresh when unsaved exclusion changes are not confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = mountView()
    await flushPromises()

    const searchInput = wrapper.find('input[type="search"]')
    await searchInput.setValue('alice')
    const searchButton = wrapper.findAll('button').find((button) => button.text() === 'common.search')
    await searchButton!.trigger('click')
    await flushPromises()
    const addButton = wrapper.findAll('button').find((button) => button.text().includes('alice@example.com'))
    await addButton!.trigger('click')
    await flushPromises()

    const refreshButton = wrapper.findAll('button').find((button) => button.text() === 'admin.balanceSummary.refresh')
    await refreshButton!.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('admin.balanceSummary.discardUnsavedChangesConfirm')
    expect(getAdminBalanceSummary).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('alice@example.com')
  })

  it('searches, adds, removes and saves exclusions', async () => {
    const wrapper = mountView()
    await flushPromises()

    const searchInput = wrapper.find('input[type="search"]')
    await searchInput.setValue('alice')
    const searchButton = wrapper.findAll('button').find((button) => button.text() === 'common.search')
    expect(searchButton).toBeTruthy()
    await searchButton!.trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(1, 10, { search: 'alice' })
    expect(wrapper.text()).toContain('alice@example.com')

    const addButton = wrapper.findAll('button').find((button) => button.text().includes('alice@example.com'))
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')
    await flushPromises()

    const removeButtons = wrapper.findAll('button[aria-label]')
    expect(removeButtons).toHaveLength(2)
    await removeButtons[0].trigger('click')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((button) => button.text() === 'admin.balanceSummary.saveExclusions')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(updateAdminBalanceSummaryExclusions).toHaveBeenCalledWith([9])
    expect(showSuccess).toHaveBeenCalledWith('admin.balanceSummary.saveSuccess')
  })

  it('removes invalid exclusions and saves only valid user IDs', async () => {
    getAdminBalanceSummary.mockResolvedValueOnce({
      ...baseSummary,
      invalid_exclusions: 1,
      excluded_user_ids: [7, 99],
      excluded_users: [
        ...baseSummary.excluded_users,
        {
          user_id: 99,
          valid: false,
          reason: 'not_found_or_deleted'
        }
      ]
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.balanceSummary.invalidExclusionsHint')

    const removeButtons = wrapper.findAll('button[aria-label]')
    expect(removeButtons).toHaveLength(2)
    await removeButtons[1].trigger('click')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((button) => button.text() === 'admin.balanceSummary.saveExclusions')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(updateAdminBalanceSummaryExclusions).toHaveBeenCalledWith([7])
  })

  it('keeps newer search results when an older request resolves later', async () => {
    let resolveOldSearch: (value: unknown) => void = () => {}
    let resolveNewSearch: (value: unknown) => void = () => {}
    listUsers
      .mockImplementationOnce(() => new Promise((resolve) => { resolveOldSearch = resolve }))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveNewSearch = resolve }))

    const wrapper = mountView()
    await flushPromises()

    const searchInput = wrapper.find('input[type="search"]')
    const searchButton = wrapper.findAll('button').find((button) => button.text() === 'common.search')

    await searchInput.setValue('old')
    await searchButton!.trigger('click')
    await searchInput.setValue('new')
    await searchInput.trigger('keyup.enter')
    expect(listUsers).toHaveBeenCalledTimes(2)

    resolveNewSearch({
      items: [{ ...aliceUser, id: 10, email: 'new@example.com', username: 'new-user' }],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1
    })
    await flushPromises()

    expect(wrapper.text()).toContain('new@example.com')

    resolveOldSearch({
      items: [{ ...aliceUser, id: 11, email: 'old@example.com', username: 'old-user' }],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1
    })
    await flushPromises()

    expect(wrapper.text()).toContain('new@example.com')
    expect(wrapper.text()).not.toContain('old@example.com')
  })
})
