import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import CandyProbeHistoryDrawer from '@/components/user/monitor/CandyProbeHistoryDrawer.vue'

const fetchHistory = vi.hoisted(() => vi.fn())
vi.mock('@/api/modelIqProbe', () => ({ fetchModelIqProbeHistory: fetchHistory }))

const messages: Record<string, string> = {
  historyTitle: '{model} history', historyPeriod: 'Time range', historyResult: 'Result', history24h: '24h', history7d: '7d', history30d: '30d', historyAll: 'All', verdictPass: 'Healthy', verdictWrong: 'Wrong', verdictError: 'Failed', markedForExclusion: 'Marked for exclusion', historyRate: '{rate}% / {n}', historyNoValid: 'No valid results', historyLoading: 'Loading', historyError: 'Error', historyRetry: 'Retry', historyEmpty: 'Empty', historyNoMatches: 'No matches', historyNoResponse: 'No response', historyExpand: 'View answer', historyCollapse: 'Collapse', historyMore: 'Load more', historyRetention: '30 days', answerFallback: 'No answer',
}
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: ref('en'), t: (key: string, params?: Record<string, string | number>) => {
  let text = messages[key.split('.').pop() ?? ''] ?? key
  for (const [name, value] of Object.entries(params ?? {})) text = text.replace(`{${name}}`, String(value))
  return text
} }) }))

const response = (items: Array<{ id: number; status: string; checked_at: string; answer?: string; marked_for_exclusion?: boolean }>, total = items.length) => ({ items, total, counts: { pass: 1, wrong_answer: 1, error: 1 } })

describe('CandyProbeHistoryDrawer', () => {
  beforeEach(() => fetchHistory.mockReset())
  it('shows the exclusion marker only on marked records', async () => {
    fetchHistory.mockResolvedValueOnce(response([
      { id: 1, status: 'wrong_answer', checked_at: '2026-10-27T06:00:00Z', answer: '20', marked_for_exclusion: true },
      { id: 2, status: 'wrong_answer', checked_at: '2026-10-27T05:00:00Z', answer: '23' },
    ]))
    const wrapper = mount(CandyProbeHistoryDrawer, { props: { model: 'gpt-5.4' }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const rows = wrapper.findAll('article')
    expect(rows[0].text()).toContain('Marked for exclusion')
    expect(rows[1].text()).not.toContain('Marked for exclusion')
    wrapper.unmount()
  })
  it('loads the selected model, filters history, and expands answers', async () => {
    fetchHistory.mockResolvedValueOnce(response([{ id: 1, status: 'wrong_answer', checked_at: '2026-10-27T06:00:00Z', answer: 'answer text' }]))
    fetchHistory.mockResolvedValueOnce(response([]))
    const wrapper = mount(CandyProbeHistoryDrawer, { props: { model: 'gpt-5.4' }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(fetchHistory).toHaveBeenCalledWith({ model: 'gpt-5.4', hours: 168, status: '', page: 1 }, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('50% / 2')
    expect(wrapper.text()).toContain('answer text')
    await wrapper.findAll('button').find((button) => button.text() === 'View answer')!.trigger('click')
    expect(wrapper.text()).toContain('Collapse')
    await wrapper.findAll('button').find((button) => button.text() === '24h')!.trigger('click')
    await flushPromises()
    expect(fetchHistory).toHaveBeenLastCalledWith({ model: 'gpt-5.4', hours: 24, status: '', page: 1 }, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('Empty')
    wrapper.unmount()
  })
  it('appends the next page and keeps a failure out of the valid rate', async () => {
    fetchHistory.mockResolvedValueOnce(response([{ id: 1, status: 'error', checked_at: '2026-10-27T06:00:00Z' }], 2))
    fetchHistory.mockResolvedValueOnce(response([{ id: 2, status: 'pass', checked_at: '2026-10-27T05:00:00Z', answer: '21' }], 2))
    const wrapper = mount(CandyProbeHistoryDrawer, { props: { model: 'gpt-5.4' }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('No response')
    await wrapper.findAll('button').find((button) => button.text() === 'Load more')!.trigger('click')
    await flushPromises()
    expect(fetchHistory).toHaveBeenLastCalledWith({ model: 'gpt-5.4', hours: 168, status: '', page: 2 }, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('21')
    expect(wrapper.text()).not.toContain('Load more')
    wrapper.unmount()
  })
})
