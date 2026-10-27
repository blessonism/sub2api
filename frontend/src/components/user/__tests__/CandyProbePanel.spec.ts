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
  'channelStatus.candyProbe.markedForExclusion': 'Marked for exclusion',
  'channelStatus.candyProbe.excludedCountChip': '{n} excluded',
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
  it('opens history for the model whose timeline button was clicked', async () => {
    const wrapper = mount(CandyProbePanel, { props: { items }, global: { stubs: { CandyProbeHistoryDrawer: { props: ['model'], template: '<div class="history-target">{{ model }}</div>' } } } })
    const buttons = wrapper.findAll('button').filter((button) => button.text().includes('channelStatus.candyProbe.viewHistory'))
    expect(buttons).toHaveLength(3)
    await buttons[1].trigger('click')
    expect(wrapper.find('.history-target').text()).toBe('gpt-5.1')
    wrapper.unmount()
  })

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

  it('excludes failed probes from the ring rate and shows an em dash without valid probes', () => {
    const mixed: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.4',
        latest: { status: 'pass', checked_at: new Date().toISOString() },
        recent: [
          { status: 'pass', checked_at: new Date().toISOString() },
          { status: 'error', checked_at: new Date().toISOString() },
          { status: 'wrong_answer', checked_at: new Date().toISOString() },
        ],
      },
    ]
    const wrapper = mount(CandyProbePanel, { props: { items: mixed } })
    // 探测失败不进分母：有效样本 = pass + wrong_answer = 2，通过 1 → 50%。
    expect(wrapper.text()).toContain('50%')

    const allFailed: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.5',
        latest: { status: 'error', checked_at: new Date().toISOString() },
        recent: [
          { status: 'error', checked_at: new Date().toISOString() },
          { status: 'error', checked_at: new Date().toISOString() },
        ],
      },
    ]
    const failed = mount(CandyProbePanel, { props: { items: allFailed } })
    // 全失败时通过率无意义，不显示 0%。
    expect(failed.text()).not.toContain('%')
    expect(failed.text()).toContain('—')
    failed.unmount()
    wrapper.unmount()
  })

  it('colors failed probes amber, distinct from the red of suspected degradation', () => {
    const mixed: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.4',
        latest: { status: 'pass', checked_at: new Date().toISOString() },
        recent: [
          { status: 'pass', checked_at: new Date().toISOString(), answer_excerpt: '第 1 次：21 个。' },
          { status: 'error', checked_at: new Date().toISOString(), answer_excerpt: 'upstream 429: rate limited' },
          { status: 'wrong_answer', checked_at: new Date().toISOString(), answer_excerpt: '答案是 20 个。' },
        ],
      },
    ]
    const wrapper = mount(CandyProbePanel, { props: { items: mixed } })
    const classes = wrapper.findAll('.candy-bar').map((bar) => bar.classes())
    expect(classes.filter((c) => c.includes('bg-amber-500'))).toHaveLength(1)
    expect(classes.filter((c) => c.includes('bg-red-500'))).toHaveLength(1)
    // 每个点悬浮可看该次回复摘要（error 点为错误信息）。
    const titles = wrapper.findAll('.candy-bar').map((bar) => bar.attributes('title') ?? '')
    expect(titles.some((t) => t.includes('第 1 次：21 个。'))).toBe(true)
    expect(titles.some((t) => t.includes('upstream 429: rate limited'))).toBe(true)
    expect(titles.some((t) => t.includes('答案是 20 个。'))).toBe(true)
    wrapper.unmount()
  })

  it('skips failed probes when counting the consecutive pass streak', () => {
    const withError: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.6',
        latest: { status: 'pass', checked_at: new Date().toISOString() },
        recent: [
          { status: 'pass', checked_at: new Date(Date.now() - 7_200_000).toISOString() },
          { status: 'error', checked_at: new Date(Date.now() - 3_600_000).toISOString() },
          { status: 'pass', checked_at: new Date().toISOString() },
        ],
      },
    ]
    const wrapper = mount(CandyProbePanel, { props: { items: withError } })
    expect(wrapper.text()).toContain('2 passed in a row')
    wrapper.unmount()
  })

  it('keeps the responsive multi-column grid even with a single model', () => {
    const wrapper = mount(CandyProbePanel, { props: { items: [items[0]] } })
    const grid = wrapper.find('.grid.gap-4')
    expect(grid.classes()).toContain('md:grid-cols-2')
    expect(grid.classes()).toContain('xl:grid-cols-4')
  })

  it('renders the latest reply as sanitized markdown with full reply on hover', () => {
    const markdownItems: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.3',
        latest: {
          status: 'pass',
          answer_excerpt: '**21** 个：星形 12 + 圆形 9。',
          answer: '**21** 个：星形 12 + 圆形 9。',
          checked_at: new Date().toISOString(),
        },
        recent: [{ status: 'pass', checked_at: new Date().toISOString() }],
      },
    ]
    const wrapper = mount(CandyProbePanel, { props: { items: markdownItems } })
    const reply = wrapper.find('.candy-reply')
    expect(reply.html()).toContain('<strong>21</strong>')
    // 悬浮展示完整回复原文。
    expect(reply.attributes('title')).toContain('星形 12 + 圆形 9')
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

  it('visually separates excluded wrong answers from unhandled ones on the strip and legend', () => {
    const excludedItems: ModelIqProbeSummaryItem[] = [
      {
        model: 'gpt-5.7',
        latest: { status: 'wrong_answer', checked_at: new Date().toISOString(), marked_for_exclusion: true },
        recent: [
          { status: 'pass', checked_at: new Date().toISOString() },
          { status: 'wrong_answer', checked_at: new Date().toISOString(), marked_for_exclusion: true },
        ],
      },
    ]
    const wrapper = mount(CandyProbePanel, { props: { items: excludedItems } })
    const classes = wrapper.findAll('.candy-bar').map((bar) => bar.classes())
    // 已剔除降智点用浅红，未处理正红不再出现。
    expect(classes.filter((c) => c.includes('bg-red-300'))).toHaveLength(1)
    expect(classes.filter((c) => c.includes('bg-red-500'))).toHaveLength(0)
    const text = wrapper.text()
    // 图例出现「疑似降智 · 已标记剔除」条目；悬浮标题保留标记行。
    expect(text).toContain('Likely degraded · Marked for exclusion')
    expect(wrapper.findAll('.candy-bar').at(-1)?.attributes('title')).toContain('Marked for exclusion')
    // 常显计数徽标：无需 hover 即可看到剔除数量。
    expect(text).toContain('1 excluded')
    wrapper.unmount()
  })

  it('renders exclusion markers on the latest verdict and history tooltip', () => {
    const projected = {
      model: 'gpt-5.4',
      latest: { status: 'pass' as const, checked_at: new Date().toISOString(), marked_for_exclusion: true },
      recent: [{ status: 'pass' as const, checked_at: new Date().toISOString(), marked_for_exclusion: true, answer_excerpt: '所以最少 21 个。' }],
    }
    const wrapper = mount(CandyProbePanel, { props: { items: [projected] } })
    expect(wrapper.text()).toContain('Marked for exclusion')
    expect(wrapper.text()).toContain('100%')
    const lastBar = wrapper.findAll('.candy-bar').at(-1)
    expect(lastBar?.attributes('title')).toContain('Marked for exclusion')
    // 剔除点位的悬浮标题同时带该次回复摘要。
    expect(lastBar?.attributes('title')).toContain('所以最少 21 个。')
    wrapper.unmount()
  })
})
