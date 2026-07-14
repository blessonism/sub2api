import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import GroupTimeRateStrategyModal from '../GroupTimeRateStrategyModal.vue'
import type { AdminGroup } from '@/types'

const update = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

vi.mock('@/api/admin', () => ({ adminAPI: { groups: { update } } }))

const group = {
  id: 10,
  name: 'K12',
  platform: 'openai',
  rate_multiplier: 0.04,
  visible_rate_multiplier: 0.4,
  time_rate_priority: 'schedule_first',
  time_rate_periods: []
} as AdminGroup

async function mountModal() {
  const wrapper = mount(GroupTimeRateStrategyModal, {
    props: { show: true, group },
    global: {
      plugins: [createPinia()],
      stubs: {
        BaseDialog: { template: '<div><slot /></div>' },
        Icon: { template: '<span />' },
        PlatformIcon: { template: '<span />' }
      }
    }
  })
  await flushPromises()
  return wrapper
}

describe('GroupTimeRateStrategyModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    update.mockReset().mockResolvedValue({})
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('新增时间段时真实和可见倍率默认一致并可保存优先模式', async () => {
    const wrapper = await mountModal()
    await wrapper.get('button.btn-secondary').trigger('click')
    const rates = wrapper.findAll('input[type="number"]')
    expect((rates[0].element as HTMLInputElement).value).toBe('0.04')
    expect((rates[1].element as HTMLInputElement).value).toBe('0.04')

    const proportional = wrapper.findAll('button').find(button => button.text() === 'admin.groups.timeRate.proportional')!
    await proportional.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(10, {
      time_rate_priority: 'proportional',
      time_rate_periods: [{ start_time: '00:00', end_time: '01:00', rate_multiplier: 0.04, visible_rate_multiplier: 0.04, enabled: true }]
    })
  })

  it('阻止保存重叠的启用时间段', async () => {
    const wrapper = await mountModal()
    const add = wrapper.get('button.btn-secondary')
    await add.trigger('click')
    await add.trigger('click')
    const textInputs = wrapper.findAll('input[inputmode="numeric"]')
    await textInputs[2].setValue('00:30')
    await textInputs[3].setValue('02:00')
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    expect(wrapper.text()).toContain('admin.groups.timeRate.overlap')
    expect(update).not.toHaveBeenCalled()
  })

  it('撤销后恢复服务端快照', async () => {
    const wrapper = await mountModal()
    await wrapper.get('button.btn-secondary').trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'admin.groups.revertChanges')!.trigger('click')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).toContain('admin.groups.timeRate.empty')
  })

  it('支持停用和删除时间段', async () => {
    const wrapper = await mountModal()
    await wrapper.get('button.btn-secondary').trigger('click')
    await wrapper.get('input[type="checkbox"]').setValue(false)
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update.mock.calls[0][1].time_rate_periods[0].enabled).toBe(false)

    const reopened = await mountModal()
    await reopened.get('button.btn-secondary').trigger('click')
    await reopened.get('button[title="common.delete"]').trigger('click')
    expect(reopened.text()).toContain('admin.groups.timeRate.empty')
  })

  it('关闭脏数据前要求确认', async () => {
    const wrapper = await mountModal()
    await wrapper.get('button.btn-secondary').trigger('click')
    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await wrapper.findAll('button').find(button => button.text() === 'common.close')!.trigger('click')
    expect(wrapper.emitted('close')).toBeUndefined()

    vi.mocked(window.confirm).mockReturnValueOnce(true)
    await wrapper.findAll('button').find(button => button.text() === 'common.close')!.trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
