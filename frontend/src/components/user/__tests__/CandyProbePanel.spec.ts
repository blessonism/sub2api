import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CandyProbePanel from '@/components/user/monitor/CandyProbePanel.vue'
import type { ModelIqProbeSummaryItem } from '@/api/modelIqProbe'

const messages: Record<string, string> = {
  'channelStatus.candyProbe.title': 'Candy Probe · Degradation Check',
  'channelStatus.candyProbe.description': 'Candy puzzle probe description',
  'channelStatus.candyProbe.summary': '{ok}/{total} healthy',
  'channelStatus.candyProbe.verdictPass': 'Healthy',
  'channelStatus.candyProbe.verdictWrong': 'Likely degraded',
  'channelStatus.candyProbe.verdictError': 'Probe failed',
  'channelStatus.candyProbe.verdictUnknown': 'Not probed',
  'channelStatus.candyProbe.rateLabel': 'healthy rate (last {n})',
  'channelStatus.candyProbe.streak': '{n} passed in a row',
  'channelStatus.candyProbe.answerFallback': 'No answer text',
  'channelStatus.candyProbe.noTimeline': 'No probes yet',
  'channelStatus.candyProbe.loadErrorTitle': 'Unable to load candy probe results',
  'channelStatus.candyProbe.noData': 'No probe data yet',
  'channelStatus.card.probeCount': 'Last {n} probes',
  'monitorCommon.latencyEmpty': '-',
  'monitorCommon.relativeSecondsAgo': '{n}s ago',
  'monitorCommon.relativeMinutesAgo': '{n}m ago',
  'monitorCommon.relativeHoursAgo': '{n}h ago',
  'monitorCommon.relativeDaysAgo': '{n}d ago',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        let message = messages[key] ?? key
        for (const [paramKey, value] of Object.entries(params ?? {})) {
          message = message.replace(`{${paramKey}}`, String(value))
        }
        return message
      },
    }),
  }
})

const items: ModelIqProbeSummaryItem[] = [
  {
    model: 'gpt-5.2',
    latest: { status: 'pass', answer_excerpt: '所以最少取出 21 个糖果。', checked_at: new Date().toISOString() },
    recent: [
      { status: 'wrong_answer', checked_at: new Date(Date.now() - 7_200_000).toISOString() },
      { status: 'pass', checked_at: new Date().toISOString() },
    ],
  },
  {
    model: 'gpt-5.1',
    latest: { status: 'wrong_answer', answer_excerpt: '答案是 20 个。', checked_at: new Date().toISOString() },
    recent: [{ status: 'wrong_answer', checked_at: new Date().toISOString() }],
  },
  {
    model: 'gpt-5',
    latest: { status: 'error', checked_at: new Date().toISOString() },
    recent: [],
  },
]

describe('CandyProbePanel', () => {
  it('renders one card per model with verdict badge and answer excerpt', () => {
    const wrapper = mount(CandyProbePanel, { props: { items } })
    const text = wrapper.text()
    expect(text).toContain('gpt-5.2')
    expect(text).toContain('所以最少取出 21 个糖果。')
    expect(text).toContain('答案是 20 个。')
    expect(text).toContain('Healthy')
    expect(text).toContain('Likely degraded')
    expect(text).toContain('Probe failed')
  })

  it('shows the healthy summary badge counting latest verdicts', () => {
    const wrapper = mount(CandyProbePanel, { props: { items } })
    expect(wrapper.text()).toContain('1/3 healthy')
  })

  it('renders a fixed 60-slot strip per model with rate and streak stats', () => {
    const wrapper = mount(CandyProbePanel, { props: { items } })
    // 每卡固定 60 槽（不足左补幽灵条），与渠道监控卡片时间线同密度，3 张卡共 180 条。
    const bars = wrapper.findAll('.candy-bar')
    expect(bars.length).toBe(180)
    const text = wrapper.text()
    // gpt-5.2: 2 次里 1 次正常 → 50%，连续 1 次。
    expect(text).toContain('50%')
    expect(text).toContain('1 passed in a row')
  })

  it('uses a single-column grid when there is only one model', () => {
    const wrapper = mount(CandyProbePanel, { props: { items: [items[0]] } })
    const grid = wrapper.find('.grid.gap-4')
    expect(grid.classes()).not.toContain('sm:grid-cols-2')
  })

  it('shows the empty state when there is no data', () => {
    const wrapper = mount(CandyProbePanel, { props: { items: [] } })
    expect(wrapper.text()).toContain('No probe data yet')
  })

  it('shows the error state when loading failed', () => {
    const wrapper = mount(CandyProbePanel, { props: { items: [], error: 'boom' } })
    expect(wrapper.text()).toContain('Unable to load candy probe results')
  })

  it('never renders account information', () => {
    const wrapper = mount(CandyProbePanel, { props: { items } })
    expect(wrapper.text()).not.toContain('@')
    expect(wrapper.html()).not.toContain('account')
  })
})
