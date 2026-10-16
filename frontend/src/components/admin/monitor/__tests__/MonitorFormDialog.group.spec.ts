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


describe('MonitorFormDialog display overrides', () => {
 it('edits percentages, preserves zero, clears individual values and restores automatic values', async () => {
  const wrapper = dialog({ ...existing, display_overrides: { visible_rate_multiplier: 0, cache_rate_24h: 0.625, output_tps_5m: 25, primary_latency_ms: 850, availability_7d: 90, availability_15d: 95, availability_30d: 99 } })
  await flushPromises()
  const input = (key: string) => wrapper.get<HTMLInputElement>(`[data-testid="monitor-override-${key}"]`)
  expect(input('visible_rate_multiplier').element.value).toBe('0')
  expect(input('cache_rate_24h').element.value).toBe('62.5')
  await input('cache_rate_24h').setValue('50')
  await input('primary_latency_ms').setValue('0')
  await input('availability_15d').setValue('0')
  await input('output_tps_5m').setValue('')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  const saved = mocks.update.mock.calls.at(-1)?.[1].display_overrides
  expect(saved).toEqual({ visible_rate_multiplier: 0, cache_rate_24h: 0.5, primary_latency_ms: 0, availability_7d: 90, availability_15d: 0, availability_30d: 99 })
  await wrapper.get('[data-testid="monitor-overrides-reset"]').trigger('click')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(mocks.update).toHaveBeenLastCalledWith(1, expect.objectContaining({ display_overrides: {} }))
 })
 it('starts new monitors in automatic mode and resets when switching monitors', async () => {
  const wrapper = dialog({ ...existing, display_overrides: { visible_rate_multiplier: 0.185 } })
  await flushPromises()
  await wrapper.setProps({ monitor: null })
  await flushPromises()
  expect(wrapper.get<HTMLInputElement>('[data-testid="monitor-override-visible_rate_multiplier"]').element.value).toBe('')
 })
})
