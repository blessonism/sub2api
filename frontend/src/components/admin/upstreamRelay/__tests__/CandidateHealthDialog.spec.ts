import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import CandidateHealthDialog from '../CandidateHealthDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => `${key}${params ? ` ${Object.values(params).join(' ')}` : ''}`,
  }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value?: string | null) => (value ? `formatted:${value}` : ''),
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div v-if="show"><slot /></div>',
}

describe('CandidateHealthDialog', () => {
  it('显示持久化健康快照的计算时间和过期状态', () => {
    const wrapper = mount(CandidateHealthDialog, {
      props: {
        show: true,
        candidate: {
          id: 9,
          connector_id: 7,
          account_id: 42,
          account_name: 'relay-account',
          upstream_group_id: 'team-a',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: true,
          notes: '',
          health: {
            probe_count: 3,
            success_count: 2,
            success_rate: 0.666667,
            p95_latency_ms: 120,
            consecutive_successes: 0,
            consecutive_failures: 1,
            window_minutes: 1440,
            sample_size: 3,
            calculated_at: '2026-06-28T12:00:00Z',
            stale: true,
          },
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
        },
      },
    })

    expect(wrapper.text()).toContain('healthDialog.calculatedAt formatted:2026-06-28T12:00:00Z')
    expect(wrapper.text()).toContain('healthDialog.stale')
    expect(wrapper.text()).toContain('healthDialog.stats.sampleValue 3 1440')
  })

  it('把探测错误和不可靠原因转换为处理建议，原文只放技术详情', () => {
    const wrapper = mount(CandidateHealthDialog, {
      props: {
        show: true,
        candidate: {
          id: 9,
          connector_id: 7,
          account_id: 42,
          account_name: 'relay-account',
          upstream_group_id: 'team-a',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: true,
          notes: '',
          latest_probe: {
            id: 1,
            candidate_id: 9,
            success: false,
            error_class: 'timeout',
            error_message: 'upstream request deadline exceeded',
            probed_at: '2026-06-28T12:00:00Z',
          },
          latest_usage_delta: {
            id: 1,
            candidate_id: 9,
            model: 'gpt-4o-mini',
            status: 'insufficient',
            unreliable_reason: 'usage delta is not positive',
            sampled_at: '2026-06-28T12:00:00Z',
          },
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
      },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })

    expect(wrapper.text()).toContain('healthDialog.errors.timeout.reason')
    expect(wrapper.text()).toContain('healthDialog.errors.timeout.advice')
    expect(wrapper.text()).toContain('healthDialog.deltaIssues.nonPositiveDelta.reason')
    expect(wrapper.findAll('details')).toHaveLength(2)
    expect(wrapper.findAll('details')[0]!.text()).toContain('upstream request deadline exceeded')
    expect(wrapper.findAll('details')[1]!.text()).toContain('usage delta is not positive')
  })
})
