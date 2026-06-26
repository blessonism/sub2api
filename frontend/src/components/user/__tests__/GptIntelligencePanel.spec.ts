import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import GptIntelligencePanel from '@/components/user/monitor/GptIntelligencePanel.vue'
import type { GptIntelligenceSnapshot } from '@/api/gptIntelligence'

const messages: Record<string, string> = {
  'channelStatus.modelIq.title': 'GPT IQ Test',
  'channelStatus.modelIq.description': 'Experimental metric',
  'channelStatus.modelIq.source': 'View source',
  'channelStatus.modelIq.updatedAt': 'Updated {time} · {timezone}',
  'channelStatus.modelIq.updatedUnknown': 'Update time unavailable',
  'channelStatus.modelIq.loadErrorTitle': 'Unable to load GPT intelligence check',
  'channelStatus.modelIq.loadErrorDescription': 'The external source is unavailable.',
  'channelStatus.modelIq.staleNotice': 'Showing stale snapshot',
  'channelStatus.modelIq.score': 'IQ Score',
  'channelStatus.modelIq.passed': 'Passed Tasks',
  'channelStatus.modelIq.baseline': 'Baseline',
  'channelStatus.modelIq.baselineValue': 'DeepSWE · {tasks} tasks',
  'channelStatus.modelIq.baselineEmpty': 'DeepSWE · tasks unknown',
  'channelStatus.modelIq.sampledAt': 'Sample batch: {date}',
  'channelStatus.modelIq.testBadge': 'DeepSWE · {tasks} tasks',
  'channelStatus.modelIq.testBadgeFallback': 'DeepSWE probe',
  'channelStatus.modelIq.model': 'Test Model',
  'channelStatus.modelIq.duration': 'Duration',
  'channelStatus.modelIq.tokens': 'Tokens',
  'channelStatus.modelIq.cost': 'Cost',
  'channelStatus.modelIq.recentTrend': 'IQ Index Trend',
  'channelStatus.modelIq.trendSubtitle': 'Latest 12 available samples',
  'channelStatus.modelIq.overviewTrend': 'IQ Index Overview',
  'channelStatus.modelIq.overviewSubtitle': 'Recent samples aligned by model and reasoning effort',
  'channelStatus.modelIq.reasoningTrends': 'Reasoning Effort Trends',
  'channelStatus.modelIq.reasoningTrendsSubtitle': 'Each model/reasoning effort uses the same chart lens as the current xhigh series',
  'channelStatus.modelIq.currentSeries': 'Current primary probe',
  'channelStatus.modelIq.emptyTrend': 'No recent trend data',
  'channelStatus.modelIq.reasoning': 'Reasoning: {effort}',
  'channelStatus.modelIq.reasoningEmpty': 'Reasoning: -',
  'channelStatus.modelIq.passRate': 'Passed {passed}/{tasks}',
  'channelStatus.modelIq.passEmpty': 'Pass count unavailable',
  'channelStatus.modelIq.outputTokens': 'Output {tokens}',
  'channelStatus.modelIq.outputTokensEmpty': 'Output tokens unavailable',
  'channelStatus.modelIq.seconds': '{seconds}s',
  'channelStatus.modelIq.quotaSummary': '{window} window · rate {rate}',
  'channelStatus.modelIq.quotaEmpty': 'Quota radar unavailable',
  'channelStatus.modelIq.status.green': 'GREEN',
  'channelStatus.modelIq.status.yellow': 'YELLOW',
  'channelStatus.modelIq.status.red': 'RED',
  'channelStatus.modelIq.status.unknown': 'UNKNOWN',
  'monitorCommon.latencyEmpty': '-',
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

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div class="line-chart">{{ JSON.stringify(data) }}</div>',
  },
}))

const snapshot: GptIntelligenceSnapshot = {
  monitored_at: '2026-06-25T00:55:43.703187+08:00',
  timezone: 'Asia/Shanghai',
  latest: {
    date: '2026-06-24-pm',
    score: 125,
    status: 'green',
    passed: 10,
    tasks: 12,
    invalid: 0,
    total_tokens: 39090118,
    output_tokens: 363304,
    wall_seconds: 1503,
    wall_time_human: '25分钟',
    model: 'gpt-5.5',
    reasoning_effort: 'xhigh',
    cost_usd: 40.383558,
  },
  recent_days: [
    {
      date: '2026-06-23',
      score: 100,
      status: 'green',
      passed: 8,
      tasks: 12,
      invalid: 0,
      total_tokens: 1,
      output_tokens: 1,
      wall_seconds: 100,
      wall_time_human: '2分钟',
      model: '',
      reasoning_effort: '',
      cost_usd: null,
    },
    {
      date: '2026-06-24-am',
      score: 112.5,
      status: 'green',
      passed: 9,
      tasks: 12,
      invalid: 0,
      total_tokens: 2,
      output_tokens: 1,
      wall_seconds: 120,
      wall_time_human: '2分钟',
      model: '',
      reasoning_effort: '',
      cost_usd: null,
    },
  ],
  comparisons: [
    {
      key: 'gpt_55_high',
      label: 'GPT-5.5 high',
      model: 'gpt-5.5',
      reasoning_effort: 'high',
      latest: {
        date: '2026-06-24-pm',
        score: 87.5,
        status: 'yellow',
        passed: 7,
        tasks: 12,
        invalid: 0,
        total_tokens: null,
        output_tokens: null,
        wall_seconds: null,
        wall_time_human: '',
        model: 'gpt-5.5',
        reasoning_effort: 'high',
        cost_usd: null,
      },
      recent_days: [
        {
          date: '2026-06-23',
          score: 87.5,
          status: 'yellow',
          passed: 7,
          tasks: 12,
          invalid: 0,
          total_tokens: null,
          output_tokens: null,
          wall_seconds: null,
          wall_time_human: '',
          model: 'gpt-5.5',
          reasoning_effort: 'high',
          cost_usd: null,
        },
        {
          date: '2026-06-24-am',
          score: 100,
          status: 'green',
          passed: 8,
          tasks: 12,
          invalid: 0,
          total_tokens: null,
          output_tokens: null,
          wall_seconds: null,
          wall_time_human: '',
          model: 'gpt-5.5',
          reasoning_effort: 'high',
          cost_usd: null,
        },
      ],
    },
  ],
  quota_radar: {
    basis_window_label: '5h',
    cost_usd: 119.52,
    rate: 2.8457,
    adjusted_delta: 42,
    updated_at: '2026-06-24T04:58:05Z',
  },
}

describe('GptIntelligencePanel', () => {
  it('renders the latest intelligence snapshot', () => {
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot,
        loading: false,
        error: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('GPT IQ Test')
    expect(wrapper.text()).toContain('DeepSWE · 12 tasks')
    expect(wrapper.text()).toContain('125.0')
    expect(wrapper.text()).toContain('Passed 10/12')
    expect(wrapper.text()).toContain('Sample batch: 2026-06-24-pm')
    expect(wrapper.text()).toContain('gpt-5.5')
    expect(wrapper.text()).toContain('Reasoning: xhigh')
    expect(wrapper.text()).toContain('Tokens')
    expect(wrapper.text()).toContain('Output')
    expect(wrapper.text()).toContain('$40.38')
    expect(wrapper.text()).toContain('5h window · rate 2.85')
    expect(wrapper.text()).toContain('GPT-5.5-xhigh')
    expect(wrapper.text()).toContain('GPT-5.5-high')
    expect(wrapper.text()).toContain('IQ Index Overview')
    expect(wrapper.text()).toContain('Reasoning Effort Trends')

    const charts = wrapper.findAll('.line-chart')
    expect(charts).toHaveLength(3)

    const overviewChartData = JSON.parse(charts[0].text())
    expect(overviewChartData.datasets.map((dataset: { label: string }) => dataset.label)).toEqual([
      '5.5-xhigh',
      '5.5-high',
    ])
    expect(overviewChartData.datasets[0].data).toEqual([100, 112.5, 125])
    expect(overviewChartData.datasets[1].data).toEqual([87.5, 100, 87.5])

    const highChartData = JSON.parse(charts[2].text())
    expect(highChartData.datasets[0].data).toEqual([87.5, 100, 87.5])
  })

  it('shows an inline error when there is no cached snapshot', () => {
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: null,
        loading: false,
        error: 'network',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Unable to load GPT intelligence check')
    expect(wrapper.text()).toContain('The external source is unavailable.')
  })
})
