import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import GptIntelligencePanel from '@/components/user/monitor/GptIntelligencePanel.vue'
import type { GptIntelligenceSnapshot } from '@/api/gptIntelligence'

const { updateGptIntelligenceTemplates } = vi.hoisted(() => ({
  updateGptIntelligenceTemplates: vi.fn(),
}))

vi.mock('@/api/admin/gptIntelligence', () => ({
  updateGptIntelligenceTemplates,
}))

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
  'channelStatus.modelIq.sortLabel': 'Sort by',
  'channelStatus.modelIq.sortByIq': 'IQ high to low',
  'channelStatus.modelIq.sortBySeries': 'Group by series',
  'channelStatus.modelIq.intelligenceCheck.button': 'Intelligence check',
  'channelStatus.modelIq.intelligenceCheck.title': 'GPT intelligence check',
  'channelStatus.modelIq.intelligenceCheck.subtitle': 'Pick a prompt template',
  'channelStatus.modelIq.intelligenceCheck.adminDraft': 'Admin draft',
  'channelStatus.modelIq.intelligenceCheck.customTemplate': 'Custom prompt',
  'channelStatus.modelIq.intelligenceCheck.addTemplate': 'Add test prompt',
  'channelStatus.modelIq.intelligenceCheck.deleteTemplate': 'Delete prompt',
  'channelStatus.modelIq.intelligenceCheck.newTemplateTitle': 'New test prompt',
  'channelStatus.modelIq.intelligenceCheck.emptyTemplates': 'No test prompts',
  'channelStatus.modelIq.intelligenceCheck.emptyDescription': 'Admins can add a prompt and save it to the global prompt list.',
  'channelStatus.modelIq.intelligenceCheck.titleLabel': 'Prompt title',
  'channelStatus.modelIq.intelligenceCheck.titlePlaceholder': 'Enter a prompt title',
  'channelStatus.modelIq.intelligenceCheck.descriptionLabel': 'Description',
  'channelStatus.modelIq.intelligenceCheck.descriptionPlaceholder': 'Enter a template description',
  'channelStatus.modelIq.intelligenceCheck.reset': 'Reset default',
  'channelStatus.modelIq.intelligenceCheck.saveDraft': 'Save draft',
  'channelStatus.modelIq.intelligenceCheck.saved': 'Draft saved',
  'channelStatus.modelIq.intelligenceCheck.saveFailed': 'Failed to save draft',
  'channelStatus.modelIq.intelligenceCheck.copyPrompt': 'Copy prompt',
  'channelStatus.modelIq.intelligenceCheck.promptLabel': 'Prompt template',
  'channelStatus.modelIq.intelligenceCheck.promptPlaceholder': 'Enter an intelligence-check prompt',
  'channelStatus.modelIq.intelligenceCheck.expectedLabel': 'Expected signal',
  'channelStatus.modelIq.intelligenceCheck.expectedPlaceholder': 'Enter the expected signal',
  'channelStatus.modelIq.intelligenceCheck.thresholdLabel': 'Failure threshold',
  'channelStatus.modelIq.intelligenceCheck.thresholdPlaceholder': 'Enter the failure threshold',
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

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title'],
    template: `
      <div v-if="show" class="base-dialog">
        <h3>{{ title }}</h3>
        <slot />
        <slot name="footer" />
      </div>
    `,
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
  beforeEach(() => {
    updateGptIntelligenceTemplates.mockReset()
  })

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
    expect(wrapper.text()).toContain('Intelligence check')
    expect(wrapper.text()).not.toContain('GPT intelligence check')
    expect(wrapper.get('a[href="https://github.com/datacurve-ai/deep-swe"]').attributes('target')).toBe('_blank')

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

  it('adapts overview card columns to available intelligence series', async () => {
    const snapshotWithFourSeries: GptIntelligenceSnapshot = {
      ...snapshot,
      comparisons: [
        ...snapshot.comparisons,
        {
          ...snapshot.comparisons[0],
          key: 'gpt_54_high',
          label: 'GPT-5.4 high',
        },
        {
          ...snapshot.comparisons[0],
          key: 'gpt_53_high',
          label: 'GPT-5.3 high',
        },
      ],
    }
    const snapshotWithFiveSeries: GptIntelligenceSnapshot = {
      ...snapshotWithFourSeries,
      comparisons: [
        ...snapshotWithFourSeries.comparisons,
        {
          ...snapshot.comparisons[0],
          key: 'gpt_52_high',
          label: 'GPT-5.2 high',
        },
      ],
    }

    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: snapshotWithFourSeries,
        loading: false,
        error: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const overviewGrid = () => wrapper.get('[data-test="gpt-intelligence-overview-cards"]')
    const overviewCards = () => overviewGrid().findAll('.rounded-xl.border.p-4')
    expect(overviewCards()).toHaveLength(4)
    expect(overviewGrid().classes()).toContain('xl:grid-cols-4')

    await wrapper.setProps({ snapshot: snapshotWithFiveSeries })

    expect(overviewCards()).toHaveLength(5)
    expect(overviewGrid().classes()).toContain('xl:grid-cols-5')
  })

  it('sorts overview cards by IQ descending by default', () => {
    const snapshotWithMoreSeries: GptIntelligenceSnapshot = {
      ...snapshot,
      comparisons: [
        ...snapshot.comparisons,
        {
          ...snapshot.comparisons[0],
          key: 'gpt_54_high',
          label: 'GPT-5.4 high',
          model: 'gpt-5.4',
          latest: { ...snapshot.comparisons[0].latest!, model: 'gpt-5.4', score: 130 },
        },
        {
          ...snapshot.comparisons[0],
          key: 'gpt_53_high',
          label: 'GPT-5.3 high',
          model: 'gpt-5.3',
          latest: { ...snapshot.comparisons[0].latest!, model: 'gpt-5.3', score: 110 },
        },
      ],
    }
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: snapshotWithMoreSeries,
        loading: false,
        error: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const cardTitles = () => wrapper
      .get('[data-test="gpt-intelligence-overview-cards"]')
      .findAll('.rounded-xl.border.p-4')
      .map((card) => card.get('div').text())

    expect(cardTitles()).toEqual(['GPT-5.4-high', 'GPT-5.5-xhigh', 'GPT-5.3-high', 'GPT-5.5-high'])
  })

  it('groups overview cards by model series when selected', async () => {
    const snapshotWithMoreSeries: GptIntelligenceSnapshot = {
      ...snapshot,
      comparisons: [
        ...snapshot.comparisons,
        {
          ...snapshot.comparisons[0],
          key: 'gpt_54_high',
          label: 'GPT-5.4 high',
          model: 'gpt-5.4',
          latest: { ...snapshot.comparisons[0].latest!, model: 'gpt-5.4', score: 130 },
        },
        {
          ...snapshot.comparisons[0],
          key: 'gpt_53_high',
          label: 'GPT-5.3 high',
          model: 'gpt-5.3',
          latest: { ...snapshot.comparisons[0].latest!, model: 'gpt-5.3', score: 110 },
        },
      ],
    }
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: snapshotWithMoreSeries,
        loading: false,
        error: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('[data-test="sort-by-series"]').trigger('click')

    const cardTitles = () => wrapper
      .get('[data-test="gpt-intelligence-overview-cards"]')
      .findAll('.rounded-xl.border.p-4')
      .map((card) => card.get('div').text())

    expect(cardTitles()).toEqual(['GPT-5.5-high', 'GPT-5.5-xhigh', 'GPT-5.3-high', 'GPT-5.4-high'])
  })

  it('opens intelligence check templates as read-only for regular users', async () => {
    const snapshotWithGlobalTemplates: GptIntelligenceSnapshot = {
      ...snapshot,
      intelligence_check_templates: [
        {
          id: 'logic',
          title: '管理员逻辑题',
          description: '所有用户可见',
          prompt: '全局 Prompt',
          expected: '全局期望',
          threshold: '全局阈值',
        },
      ],
    }
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: snapshotWithGlobalTemplates,
        loading: false,
        error: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Intelligence check')
    expect(wrapper.text()).not.toContain('GPT intelligence check')

    await wrapper.get('button[aria-label="Intelligence check"]').trigger('click')

    expect(wrapper.text()).toContain('GPT intelligence check')
    expect(wrapper.text()).toContain('管理员逻辑题')
    expect(wrapper.text()).toContain('所有用户可见')
    expect(wrapper.find('textarea').element.value).toBe('全局 Prompt')
    expect(wrapper.text()).not.toContain('指令遵循')
    expect(wrapper.text()).not.toContain('上下文抗干扰')
    expect(wrapper.text()).toContain('Copy prompt')
    expect(wrapper.text()).not.toContain('Save draft')
    expect(wrapper.find('#model-iq-template-title').exists()).toBe(false)
    expect(wrapper.find('#model-iq-template-description').exists()).toBe(false)
    expect(wrapper.find('.base-dialog').exists()).toBe(true)
  })

  it('lets admins edit intelligence check template fields', async () => {
    updateGptIntelligenceTemplates.mockResolvedValueOnce({
      templates: [
        {
          id: 'logic',
          title: '保存后的逻辑题',
          description: '所有用户可见',
          prompt: '保存后的 Prompt',
          expected: '保存后的期望',
          threshold: '保存后的阈值',
        },
      ],
    })
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot,
        loading: false,
        error: null,
        canEditIntelligenceTemplates: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('button[aria-label="Intelligence check"]').trigger('click')

    expect(wrapper.text()).toContain('Admin draft')
    expect(wrapper.text()).toContain('Save draft')
    expect(wrapper.find('#model-iq-template-title').exists()).toBe(true)
    expect(wrapper.find('#model-iq-template-description').exists()).toBe(true)
    expect(wrapper.find('textarea[placeholder="Enter the expected signal"]').exists()).toBe(true)
    expect(wrapper.find('textarea[placeholder="Enter the failure threshold"]').exists()).toBe(true)

    await wrapper.find('#model-iq-template-title').setValue('保存后的逻辑题')
    const saveButton = wrapper.findAll('button').find((button) => button.text().includes('Save draft'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(updateGptIntelligenceTemplates).toHaveBeenCalledOnce()
    expect(updateGptIntelligenceTemplates.mock.calls[0][0][0]).toMatchObject({
      id: 'logic',
      title: '保存后的逻辑题',
    })
    expect(wrapper.text()).toContain('Draft saved')
  })

  it('lets admins delete default intelligence check templates', async () => {
    updateGptIntelligenceTemplates.mockResolvedValueOnce({
      templates: [],
    })
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot,
        loading: false,
        error: null,
        canEditIntelligenceTemplates: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('button[aria-label="Intelligence check"]').trigger('click')

    expect(wrapper.text()).toContain('Delete prompt')
    const deleteButton = wrapper.get('[data-test="delete-intelligence-template"]')
    expect(deleteButton.text()).toContain('Delete prompt')
    await deleteButton.trigger('click')

    const saveButton = wrapper.findAll('button').find((button) => button.text().includes('Save draft'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(updateGptIntelligenceTemplates).toHaveBeenCalledOnce()
    expect(updateGptIntelligenceTemplates.mock.calls[0][0].some((template) => template.id === 'logic')).toBe(false)
    expect(wrapper.text()).toContain('No test prompts')
  })

  it('lets admins add a prompt after all prompts were deleted', async () => {
    const emptySnapshot: GptIntelligenceSnapshot = {
      ...snapshot,
      intelligence_check_templates: [],
    }
    const wrapper = mount(GptIntelligencePanel, {
      props: {
        snapshot: emptySnapshot,
        loading: false,
        error: null,
        canEditIntelligenceTemplates: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('button[aria-label="Intelligence check"]').trigger('click')
    expect(wrapper.text()).toContain('No test prompts')

    const addButton = wrapper.findAll('button').find((button) => button.text().includes('Add test prompt'))
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')

    expect(wrapper.text()).toContain('New test prompt')
    expect(wrapper.find('#model-iq-template-title').exists()).toBe(true)
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
