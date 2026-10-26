import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AccountMultiSelector from '../AccountMultiSelector.vue'

const options = [
  { id: 11, name: '收-6', platform: 'openai', status: 'active' },
  { id: 24, name: '黑化-不降智', platform: 'openai', status: 'active' },
  { id: 7, name: '旧号', platform: 'openai', status: 'inactive' },
]

const mountSelector = (modelValue: number[] = []) =>
  mount(AccountMultiSelector, {
    props: {
      modelValue,
      options,
      placeholder: '选择账号',
      searchPlaceholder: '搜索',
      emptyText: '没有匹配',
      inactiveLabel: '已停用',
      selectedLabel: `已选 ${modelValue.length} 个`,
    },
  })

describe('AccountMultiSelector', () => {
  it('opens the dropdown and selects accounts by click, emitting updated ids', async () => {
    const wrapper = mountSelector()
    await wrapper.find('[role="combobox"]').trigger('click')
    const optionButtons = wrapper.findAll('[data-testid="account-option"]')
    expect(optionButtons).toHaveLength(3)

    await optionButtons[0].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([11])

    await wrapper.setProps({ modelValue: [11] })
    await wrapper.findAll('[data-testid="account-option"]')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([11, 24])
  })

  it('toggles an already selected account off and removes chips', async () => {
    const wrapper = mountSelector([11, 24])

    // 已选标签里的 × 移除第一个账号
    await wrapper.find('[role="combobox"] button').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([24])

    await wrapper.setProps({ modelValue: [24] })
    await wrapper.find('[role="combobox"]').trigger('click')
    await wrapper.findAll('[data-testid="account-option"]')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([])
  })

  it('filters options by name or id through the search box', async () => {
    const wrapper = mountSelector()
    await wrapper.find('[role="combobox"]').trigger('click')
    await wrapper.find('input').setValue('黑化')
    expect(wrapper.findAll('[data-testid="account-option"]')).toHaveLength(1)

    await wrapper.find('input').setValue('7')
    const visible = wrapper.findAll('[data-testid="account-option"]')
    expect(visible).toHaveLength(1)
    expect(visible[0].text()).toContain('旧号')
    expect(visible[0].text()).toContain('已停用')
  })

  it('shows the empty state when nothing matches', async () => {
    const wrapper = mountSelector()
    await wrapper.find('[role="combobox"]').trigger('click')
    await wrapper.find('input').setValue('不存在')
    expect(wrapper.text()).toContain('没有匹配')
  })
})
