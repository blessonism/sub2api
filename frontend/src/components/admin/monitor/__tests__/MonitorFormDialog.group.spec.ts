import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorFormDialog from '../MonitorFormDialog.vue'

const mocks = vi.hoisted(() => ({ getAll: vi.fn(), create: vi.fn(), update: vi.fn(), list: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin/groups', () => ({ getAll: mocks.getAll }))
vi.mock('@/api/admin', () => ({ adminAPI: { channelMonitor: mocks, channelMonitorTemplate: { list: mocks.list } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, te: () => true }) }))
enableAutoUnmount(afterEach)
beforeEach(() => {
 vi.clearAllMocks()
 mocks.getAll.mockResolvedValue([{ id: 7, name: 'Group Seven' }, { id: 8, name: 'Group Eight' }])
 mocks.list.mockResolvedValue({ items: [] })
 mocks.create.mockResolvedValue({})
 mocks.update.mockResolvedValue({})
})
const SelectStub = defineComponent({
 props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'],
 template: `<select :value="modelValue" :disabled="disabled" @change="$emit('update:modelValue', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>`,
})
function dialog(monitor: ChannelMonitor | null = null) {
 return mount(MonitorFormDialog, { props: { show: true, monitor }, global: { stubs: {
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  Select: SelectStub, MonitorKeyPickerDialog: true, MonitorAdvancedRequestConfig: true, ModelTagInput: true, Toggle: true,
 } } })
}
const existing = { id: 1, name: 'Monitor', provider: 'openai', api_mode: 'responses', check_mode: 'probe', endpoint: 'https://api.example.com', primary_model: 'gpt-test', group_id: 7, group_name: 'display label', extra_models: [], interval_seconds: 60, jitter_seconds: 0, enabled: true, extra_headers: {}, body_override_mode: 'off' } as ChannelMonitor

describe('MonitorFormDialog linked group', () => {
 it('loads and changes a saved group, then explicitly unlinks with zero', async () => {
  const wrapper = dialog(existing)
  await flushPromises()
  const select = wrapper.get<HTMLSelectElement>('#monitor-metric-group')
  expect(select.element.value).toBe('7')
  await select.setValue('8')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.update).toHaveBeenLastCalledWith(1, expect.objectContaining({ group_id: 8, group_name: 'display label' }))
  await select.setValue('')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.update).toHaveBeenLastCalledWith(1, expect.objectContaining({ group_id: 0 }))
 })
 it('automatically uses the selected site API key group on create', async () => {
  const wrapper = dialog()
  await flushPromises()
  await wrapper.get('input[type="text"]').setValue('New Monitor')
  await wrapper.get('[data-testid="monitor-primary-model"]').setValue('gpt-test')
  wrapper.getComponent({ name: 'MonitorKeyPickerDialog' }).vm.$emit('pick', { key: 'sk-test', group_id: 8 })
  await flushPromises()
  expect(wrapper.get<HTMLSelectElement>('#monitor-metric-group').element.value).toBe('8')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ group_id: 8, api_key: 'sk-test' }))
 })
 it('keeps unlinked legacy monitors unlinked and preserves missing selected groups', async () => {
  const wrapper = dialog({ ...existing, group_id: null })
  await flushPromises()
  expect(wrapper.get<HTMLSelectElement>('#monitor-metric-group').element.value).toBe('')
  await wrapper.setProps({ monitor: { ...existing, group_id: 99 } })
  await flushPromises()
  expect(wrapper.get<HTMLSelectElement>('#monitor-metric-group').element.value).toBe('99')
 })
 it('keeps the current association when the groups query fails', async () => {
  mocks.getAll.mockRejectedValue(new Error('unavailable'))
  const wrapper = dialog(existing)
  await flushPromises()
  expect(wrapper.get('#monitor-metric-group').attributes('disabled')).toBeDefined()
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.update).toHaveBeenCalledWith(1, expect.objectContaining({ group_id: 7 }))
 })
})
