import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelIqMonitorView from '../ModelIqMonitorView.vue'

const { api, accountsList, showError, showSuccess } = vi.hoisted(() => ({
  api: { getConfig: vi.fn(), updateConfig: vi.fn(), listResults: vi.fn(), runProbe: vi.fn(), getRunState: vi.fn(), overrideResult: vi.fn() },
  accountsList: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { modelIqProbe: api, accounts: { list: accountsList } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${JSON.stringify(params)}` : key }),
}))
const config = { enabled: true, visibility: 'admin_only', user_display_mode: 'real', show_exclusion: false,
  question: 'Question', expected_answer: '21', interval_minutes: 360, target_mode: 'account_model', models: [], account_ids: [] }
const idle = { id: '', status: 'idle', started_at: null, finished_at: null, summary: { total: 0, pass: 0, wrong_answer: 0, error: 0 } }
const active = { ...idle, id: 'run-1', status: 'running', summary: { ...idle.summary, total: 2 } }
const completed = { ...active, status: 'completed', summary: { total: 2, pass: 1, wrong_answer: 1, error: 0 } }
const mountView = () => shallowMount(ModelIqMonitorView, { global: { stubs: {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
  Select: { props: ['modelValue', 'options'], emits: ['update:modelValue'], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>' },
  Toggle: { props: ['modelValue'], emits: ['update:modelValue'], template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />' },
} } })
let wrapper: ReturnType<typeof mountView> | undefined
beforeEach(() => {
  vi.useFakeTimers(); vi.clearAllMocks(); localStorage.clear()
  api.getConfig.mockResolvedValue({ ...config })
  api.updateConfig.mockImplementation(async (value) => value)
  api.listResults.mockResolvedValue({ items: [], total: 0 })
  api.getRunState.mockResolvedValue(idle)
  api.runProbe.mockResolvedValue(active)
  accountsList.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers() })

describe('ModelIqMonitorView', () => {
  it('loads and saves independent visibility, display mode and exclusion settings', async () => {
    wrapper = mountView(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await wrapper.get('[aria-label="admin.modelIqMonitor.visibilityLabel"]').setValue('all_users')
    await wrapper.get('[aria-label="admin.modelIqMonitor.displayModeLabel"]').setValue('always_green')
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ visibility: 'all_users', user_display_mode: 'always_green', show_exclusion: true }))
  })

  it('polls background progress and refreshes results when a run completes', async () => {
    wrapper = mountView(); await flushPromises()
    api.getRunState.mockResolvedValueOnce(active).mockResolvedValue(completed)
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(api.runProbe).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.modelIqMonitor.runProgress')
    expect(button.attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(2000); await flushPromises()
    expect(wrapper.text()).not.toContain('admin.modelIqMonitor.runProgress')
    expect(api.listResults).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('admin.modelIqMonitor.runSummary'))
  })

  it('restores an existing run after reopening and stops polling on unmount', async () => {
    api.getRunState.mockResolvedValue(active)
    wrapper = mountView(); await flushPromises()
    expect(wrapper.text()).toContain('admin.modelIqMonitor.runProgress')
    wrapper.unmount(); wrapper = undefined
    await vi.advanceTimersByTimeAsync(10000)
    expect(api.getRunState).toHaveBeenCalledTimes(1)
  })

  it('reports a failed background run instead of showing success', async () => {
    wrapper = mountView(); await flushPromises()
    api.getRunState.mockResolvedValueOnce(active).mockResolvedValue({ ...completed, status: 'failed', error_message: 'storage unavailable' })
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    await vi.advanceTimersByTimeAsync(2000); await flushPromises()
    expect(showError).toHaveBeenCalledWith('storage unavailable')
    expect(wrapper.text()).toContain('storage unavailable')
    expect(showSuccess).not.toHaveBeenCalledWith(expect.stringContaining('runSummary'))
  })

  it.each([
    { status: 409, message: 'probe already running' },
    { code: 'ERR_NETWORK', message: 'response lost' },
  ])('recovers an active background run after a rejected startup response: %s', async (error) => {
    wrapper = mountView(); await flushPromises()
    api.getRunState.mockResolvedValueOnce(active).mockResolvedValue(completed)
    api.runProbe.mockRejectedValue(error)
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('admin.modelIqMonitor.runProgress')
    expect(button.attributes('disabled')).toBeDefined()
    expect(showError).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(2000); await flushPromises()
    expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('runSummary'))
  })

  it('reports a new completed run if its startup response was lost', async () => {
    wrapper = mountView(); await flushPromises()
    api.runProbe.mockRejectedValue(new Error('response lost'))
    api.getRunState.mockResolvedValue(completed)
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('runSummary'))
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps the original startup error if no new task exists', async () => {
    api.getRunState.mockResolvedValue(completed)
    wrapper = mountView(); await flushPromises()
    api.runProbe.mockRejectedValue(new Error('probe disabled'))
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(showError).toHaveBeenCalledWith('probe disabled')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(button.attributes('disabled')).toBeUndefined()
  })

  it('continues recovery polling if the first status request fails', async () => {
    wrapper = mountView(); await flushPromises()
    api.runProbe.mockRejectedValue(new Error('response lost'))
    api.getRunState.mockRejectedValueOnce(new Error('temporary status failure')).mockResolvedValue(active)
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(button.attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(2000); await flushPromises()
    expect(wrapper.text()).toContain('admin.modelIqMonitor.runProgress')
  })

  it('does not mistake old completion for success before the initial status is known', async () => {
    api.getRunState.mockImplementationOnce(() => new Promise(() => {})).mockResolvedValue(completed)
    wrapper = mountView(); await flushPromises()
    api.runProbe.mockRejectedValue(new Error('probe disabled'))
    const button = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.runNow'))!
    await button.trigger('click'); await flushPromises()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('probe disabled')
  })
})
