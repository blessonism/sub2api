import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getUser: vi.fn(),
  updateUser: vi.fn(),
  listAccounts: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: { list: apiMocks.listGroups },
    users: { getById: apiMocks.getUser, update: apiMocks.updateUser },
    accounts: { list: apiMocks.listAccounts },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: apiMocks.showError, showSuccess: apiMocks.showSuccess }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

vi.mock('@/components/common/PlatformIcon.vue', () => ({
  default: { template: '<span />' },
}))

import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const user = {
  id: 7,
  email: 'user@example.com',
  allowed_groups: [],
  group_rates: {},
  visible_group_rates: {},
} as any

const group = {
  id: 9,
  name: 'Public Claude',
  platform: 'anthropic',
  subscription_type: 'standard',
  status: 'active',
  is_exclusive: false,
  rate_multiplier: 1,
} as any

async function mountAndOpen() {
  const wrapper = mount(UserAllowedGroupsModal, { props: { show: false, user } })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 1000, pages: 1 })
  apiMocks.getUser.mockResolvedValue({ ...user, group_account_bindings: {} })
  apiMocks.listAccounts.mockResolvedValue({
    items: [
      { id: 101, name: 'Account A', status: 'active' },
      { id: 102, name: 'Account B', status: 'inactive' },
    ],
    total: 2,
    page: 1,
    page_size: 200,
    pages: 1,
  })
  apiMocks.updateUser.mockResolvedValue(user)
})

describe('UserAllowedGroupsModal account bindings', () => {
  it('loads an existing binding and its group accounts', async () => {
    apiMocks.getUser.mockResolvedValueOnce({
      ...user,
      group_account_bindings: { 9: { account_ids: [102], fallback_to_group: false } },
    })
    const wrapper = await mountAndOpen()

    expect(apiMocks.listAccounts).toHaveBeenCalledWith(1, 200, { group: '9', lite: 'true' })
    expect(wrapper.text()).toContain('Account B')
    const selected = wrapper.findAll('input[type="checkbox"]').find((input) => (input.element as HTMLInputElement).checked && input.element.getAttribute('type') === 'checkbox')
    expect(selected).toBeTruthy()
  })

  it('saves selected accounts and fallback policy', async () => {
    const wrapper = await mountAndOpen()
    const limitToggle = wrapper.findAll('label').find((label) => label.text().includes('admin.users.limitAccounts'))!
    await limitToggle.find('input').setValue(true)
    await flushPromises()

    const accountLabel = wrapper.findAll('label').find((label) => label.text().includes('Account B'))!
    await accountLabel.find('input').setValue(true)
    const fallbackLabel = wrapper.findAll('label').find((label) => label.text().includes('admin.users.accountBindingFallback'))!
    await fallbackLabel.find('input').setValue(true)

    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')!
    await save.trigger('click')
    await flushPromises()

    expect(apiMocks.updateUser).toHaveBeenCalledWith(7, expect.objectContaining({
      group_account_bindings: { 9: { account_ids: [102], fallback_to_group: true } },
    }))
  })

  it('rejects an enabled binding without accounts', async () => {
    const wrapper = await mountAndOpen()
    const limitToggle = wrapper.findAll('label').find((label) => label.text().includes('admin.users.limitAccounts'))!
    await limitToggle.find('input').setValue(true)
    await flushPromises()

    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')!
    await save.trigger('click')
    expect(apiMocks.showError).toHaveBeenCalled()
    expect(apiMocks.updateUser).not.toHaveBeenCalled()
  })
})
