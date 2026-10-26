import { defineComponent, h, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ModelGroupRouteFields from '../ModelGroupRouteFields.vue'
import type { ModelGroupRoute } from '@/types'

vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getAll: vi.fn(async () => [
  { id: 40, name: 'Luna', platform: 'openai', status: 'active', subscription_type: 'standard' },
  { id: 51, name: 'Source', platform: 'openai', status: 'active', subscription_type: 'standard' },
  { id: 21, name: 'Subscription', platform: 'openai', status: 'active', subscription_type: 'subscription' },
  { id: 14, name: 'Claude', platform: 'anthropic', status: 'active', subscription_type: 'standard' }
]) } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function render(initial: ModelGroupRoute[] = []) {
  const routes = ref(initial)
  const wrapper = mount(defineComponent({ setup: () => () => h(ModelGroupRouteFields, {
    modelValue: routes.value, sourceGroupId: 51, idPrefix: 'test-route',
    'onUpdate:modelValue': (value: ModelGroupRoute[]) => { routes.value = value }
  }) }), { global: { stubs: { Select: defineComponent({
    props: ['modelValue', 'options'], emits: ['update:modelValue'],
    template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
  }) } } })
  return { wrapper, routes }
}

describe('model group routes', () => {
  it('adds, edits, switches mode, toggles and removes a rule', async () => {
    const { wrapper, routes } = render()
    await flushPromises()
    await wrapper.get('[data-testid="route-add"]').trigger('click')
    await wrapper.get('[data-testid="route-model"]').setValue('gpt-6-luna')
    await wrapper.get('[data-testid="route-target"]').setValue('40')
    await wrapper.get('[data-testid="route-mode"]').setValue('prefer_local')
    expect(routes.value).toEqual([{ model: 'gpt-6-luna', target_group_id: 40, mode: 'prefer_local', enabled: true }])
    await wrapper.get('[data-testid="route-enabled"]').setValue(false)
    expect(routes.value[0]?.enabled).toBe(false)
    await wrapper.get('[data-testid="route-remove"]').trigger('click')
    expect(routes.value).toEqual([])
    wrapper.unmount()
  })

  it('restores saved rules and filters target options', async () => {
    const saved: ModelGroupRoute = { model: 'gpt-5.6-luna', target_group_id: 40, mode: 'target_only', enabled: false }
    const { wrapper } = render([saved])
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('[data-testid="route-model"]').element.value).toBe(saved.model)
    expect(wrapper.get('[data-testid="route-target"]').text()).toContain('Luna')
    expect(wrapper.get('[data-testid="route-target"]').text()).not.toContain('Source')
    expect(wrapper.get('[data-testid="route-target"]').text()).not.toContain('Subscription')
    expect(wrapper.get('[data-testid="route-target"]').text()).not.toContain('Claude')
    wrapper.unmount()
  })

  it('rejects duplicate models and incomplete rules', async () => {
    const rule: ModelGroupRoute = { model: 'gpt-6-luna', target_group_id: 40, mode: 'target_only', enabled: true }
    const { wrapper } = render([rule, { ...rule }])
    await flushPromises()
    expect(wrapper.findComponent(ModelGroupRouteFields).vm.validate()).toBe(false)
    await flushPromises()
    expect(wrapper.find('[data-testid="route-error"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
