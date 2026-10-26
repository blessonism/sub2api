import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelIqMonitorView from '../ModelIqMonitorView.vue'

const { api, accountsList, showError, showSuccess } = vi.hoisted(() => ({
  api: { getConfig: vi.fn(), listAvailableModels: vi.fn(), updateConfig: vi.fn(), listResults: vi.fn(), runProbe: vi.fn(), getRunState: vi.fn(), overrideResult: vi.fn(), fetchPassAnswerSample: vi.fn() },
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
  question: 'Question', expected_answer: '21', interval_minutes: 360, target_mode: 'account_model', models: [], account_ids: [], disabled_account_ids: [] }
const idle = { id: '', status: 'idle', started_at: null, finished_at: null, summary: { total: 0, pass: 0, wrong_answer: 0, error: 0 } }
const active = { ...idle, id: 'run-1', status: 'running', summary: { ...idle.summary, total: 2 } }
const completed = { ...active, status: 'completed', summary: { total: 2, pass: 1, wrong_answer: 1, error: 0 } }
const mountView = () => shallowMount(ModelIqMonitorView, { global: { stubs: {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  // 渲染行级 cell 插槽，让改判/复制按钮可被测试触达。
  DataTable: { props: ['columns', 'data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actual_answer" :row="row" /><slot name="cell-actions" :row="row" /></div></div>' },
  Select: { props: ['modelValue', 'options'], emits: ['update:modelValue'], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>' },
  Toggle: { props: ['modelValue'], emits: ['update:modelValue'], template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />' },
} } })
let wrapper: ReturnType<typeof mountView> | undefined
beforeEach(() => {
  vi.useFakeTimers(); vi.clearAllMocks(); localStorage.clear()
  api.getConfig.mockResolvedValue({ ...config })
  api.updateConfig.mockImplementation(async (value) => value)
  api.listAvailableModels.mockResolvedValue(['gpt-5.1', 'gpt-5.2'])
  api.listResults.mockResolvedValue({ items: [], total: 0 })
  api.getRunState.mockResolvedValue(idle)
  api.runProbe.mockResolvedValue(active)
  accountsList.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers() })

describe('ModelIqMonitorView', () => {
  it('toggles an added account without removing it from the roster', async () => {
    accountsList.mockResolvedValue({ items: [
      { id: 1, name: 'Alpha', platform: 'openai', status: 'active' },
      { id: 2, name: 'Beta', platform: 'openai', status: 'active' },
    ], total: 2, page: 1, page_size: 100, pages: 1 })
    api.getConfig.mockResolvedValue({ ...config, account_ids: [1, 2], disabled_account_ids: [2] })
    wrapper = mountView(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click'); await flushPromises()
    const toggleFor = (name: string) =>
      wrapper.findAll('input[type="checkbox"]').find((n) => (n.attributes('aria-label') || '').includes(name))!
    // Beta 处于停用态但仍在名单里，Alpha 正常启用。
    expect(toggleFor('Alpha').element.checked).toBe(true)
    expect(toggleFor('Beta').element.checked).toBe(false)
    // 重新开启 Beta：名单不变，停用集合清空。
    await toggleFor('Beta').setValue(true); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ account_ids: [1, 2], disabled_account_ids: [] }))
  })

  it('drops the disable flag when an account is removed from the selector', async () => {
    accountsList.mockResolvedValue({ items: [
      { id: 1, name: 'Alpha', platform: 'openai', status: 'active' },
      { id: 2, name: 'Beta', platform: 'openai', status: 'active' },
    ], total: 2, page: 1, page_size: 100, pages: 1 })
    api.getConfig.mockResolvedValue({ ...config, account_ids: [1, 2], disabled_account_ids: [2] })
    wrapper = mountView(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click'); await flushPromises()
    // AccountMultiSelector 为 shallow stub：直接改写 v-model 模拟移除账号 2。
    await wrapper.findComponent({ name: 'AccountMultiSelector' }).vm.$emit('update:modelValue', [1])
    await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ account_ids: [1], disabled_account_ids: [] }))
  })

  it('rewrites the reply on override and can fill a random passing reply', async () => {
    api.listResults.mockResolvedValue({ items: [{
      id: 7, account_id: 1, account_name: 'OpenAI-Main', model: 'gpt-5.2', protocol: 'responses',
      status: 'wrong_answer', override_status: null, effective_status: 'wrong_answer',
      error_category: '', error_message: '', expected_answer: '21', actual_answer: '答案是 20 个。',
      latency_ms: 100, input_tokens: null, output_tokens: null, cache_read_tokens: null, created_at: new Date().toISOString(),
    }], total: 1, page: 1, page_size: 20, pages: 1 })
    api.fetchPassAnswerSample.mockResolvedValue('所以最少取出 21 个糖果。')
    wrapper = mountView(); await flushPromises()
    // 打开改判弹窗，回答原文预填当前回复。
    const overrideBtn = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.overrideAction'))!
    await overrideBtn.trigger('click'); await flushPromises()
    const textarea = wrapper.find('#iq-override-answer')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('答案是 20 个。')
    // 一键替换：拉取随机通过回复填入。
    const replaceBtn = wrapper.findAll('button').find((b) => b.text().includes('admin.modelIqMonitor.replaceWithPassReply'))!
    await replaceBtn.trigger('click'); await flushPromises()
    expect(api.fetchPassAnswerSample).toHaveBeenCalledWith(7)
    expect((textarea.element as HTMLTextAreaElement).value).toBe('所以最少取出 21 个糖果。')
    // 保存：改判为 pass 且同时改写回复。
    await wrapper.find('input[value="pass"]').setValue(true)
    const confirmBtn = wrapper.findAll('button').find((b) => b.text().trim() === 'common.confirm')!
    await confirmBtn.trigger('click'); await flushPromises()
    expect(api.overrideResult).toHaveBeenCalledWith(7, 'pass', '所以最少取出 21 个糖果。')
  })

  it('pulls available models and toggles them into the saved config', async () => {
    wrapper = mountView(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click'); await flushPromises()
    expect(api.listAvailableModels).toHaveBeenCalledTimes(1)
    const chip = wrapper.findAll('button').find((b) => b.text() === 'gpt-5.1')!
    expect(chip.attributes('aria-pressed')).toBe('false')
    await chip.trigger('click')
    const selected = wrapper.findAll('button').find((b) => b.text() === 'gpt-5.1')!
    expect(selected.attributes('aria-pressed')).toBe('true')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ models: ['gpt-5.1'] }))
  })

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
