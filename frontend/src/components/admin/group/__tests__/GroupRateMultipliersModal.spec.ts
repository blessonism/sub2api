import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import GroupRateMultipliersModal from '../GroupRateMultipliersModal.vue'
import type { AdminGroup } from '@/types'

const apiMocks = vi.hoisted(() => ({
  getGroupRateMultipliers: vi.fn(),
  batchSetGroupRateMultipliers: vi.fn(),
  listUsers: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.groups.selectedCount') return `selected:${params?.count}`
        return key
      }
    })
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupRateMultipliers: apiMocks.getGroupRateMultipliers,
      batchSetGroupRateMultipliers: apiMocks.batchSetGroupRateMultipliers
    },
    users: {
      list: apiMocks.listUsers
    }
  }
}))

const group: AdminGroup = {
  id: 10,
  name: 'Claude',
  platform: 'anthropic',
  priority: 1,
  weight: 1,
  rate_multiplier: 1,
  image_rate_multiplier: 1,
  status: 'active',
  created_at: '',
  updated_at: ''
} as AdminGroup

const mountModal = async () => {
  const wrapper = mount(GroupRateMultipliersModal, {
    props: {
      show: false,
      group
    },
    global: {
      plugins: [createPinia()],
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /></div>'
        },
        Pagination: {
          template: '<div />'
        },
        Icon: {
          template: '<span />'
        },
        PlatformIcon: {
          template: '<span />'
        }
      }
    }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('GroupRateMultipliersModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMocks.getGroupRateMultipliers.mockResolvedValue([
      { user_id: 1, user_name: 'alice', user_email: 'alice@example.com', user_notes: '', user_status: 'active', rate_multiplier: 1, rpm_override: null },
      { user_id: 2, user_name: 'bob', user_email: 'bob@example.com', user_notes: '', user_status: 'active', rate_multiplier: 2, rpm_override: null },
      { user_id: 3, user_name: 'carol', user_email: 'carol@example.com', user_notes: '', user_status: 'active', rate_multiplier: 3, rpm_override: null }
    ])
    apiMocks.batchSetGroupRateMultipliers.mockResolvedValue({ message: 'ok' })
    apiMocks.listUsers.mockResolvedValue({ items: [] })
  })

  it('把统一倍率只应用到选中条目并保存完整分组倍率列表', async () => {
    const wrapper = await mountModal()

    const rowCheckboxes = wrapper.findAll('tbody input[type="checkbox"]')
    await rowCheckboxes[0].setValue(true)
    await rowCheckboxes[1].setValue(true)

    const numberInputs = wrapper.findAll('input[type="number"]')
    await numberInputs[1].setValue('1.25')

    const applyButton = wrapper.findAll('button').find(button => button.text() === 'admin.groups.applyMultiplier')
    expect(applyButton).toBeTruthy()
    await applyButton!.trigger('click')

    const saveButton = wrapper.findAll('button').find(button => button.text() === 'common.save')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(10, [
      { user_id: 1, rate_multiplier: 1.25 },
      { user_id: 2, rate_multiplier: 1.25 },
      { user_id: 3, rate_multiplier: 3 }
    ])
  })

  it('拒绝超过两位小数的批量倍率输入', async () => {
    const wrapper = await mountModal()

    const rowCheckboxes = wrapper.findAll('tbody input[type="checkbox"]')
    await rowCheckboxes[0].setValue(true)

    const numberInputs = wrapper.findAll('input[type="number"]')
    await numberInputs[1].setValue('1.234')

    const applyButton = wrapper.findAll('button').find(button => button.text() === 'admin.groups.applyMultiplier')
    expect(applyButton?.attributes('disabled')).toBeDefined()
  })

  it('未选中旧三四位小数存在时仍可保存选中条目的统一两位小数倍率', async () => {
    apiMocks.getGroupRateMultipliers.mockResolvedValueOnce([
      { user_id: 1, user_name: 'alice', user_email: 'alice@example.com', user_notes: '', user_status: 'active', rate_multiplier: 1, rpm_override: null },
      { user_id: 2, user_name: 'bob', user_email: 'bob@example.com', user_notes: '', user_status: 'active', rate_multiplier: 2, rpm_override: null },
      { user_id: 3, user_name: 'carol', user_email: 'carol@example.com', user_notes: '', user_status: 'active', rate_multiplier: 1.234, rpm_override: null },
      { user_id: 4, user_name: 'dave', user_email: 'dave@example.com', user_notes: '', user_status: 'active', rate_multiplier: 2.3456, rpm_override: null }
    ])
    const wrapper = await mountModal()

    const rowCheckboxes = wrapper.findAll('tbody input[type="checkbox"]')
    await rowCheckboxes[0].setValue(true)
    await rowCheckboxes[1].setValue(true)

    const numberInputs = wrapper.findAll('input[type="number"]')
    await numberInputs[1].setValue('1.25')

    const applyButton = wrapper.findAll('button').find(button => button.text() === 'admin.groups.applyMultiplier')
    expect(applyButton).toBeTruthy()
    await applyButton!.trigger('click')

    const saveButton = wrapper.findAll('button').find(button => button.text() === 'common.save')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(10, [
      { user_id: 1, rate_multiplier: 1.25 },
      { user_id: 2, rate_multiplier: 1.25 },
      { user_id: 3, rate_multiplier: 1.234 },
      { user_id: 4, rate_multiplier: 2.3456 }
    ])
  })
})
