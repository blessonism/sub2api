import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import CandidateHealthDialog from '../CandidateHealthDialog.vue'

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

    expect(wrapper.text()).toContain('计算于 formatted:2026-06-28T12:00:00Z')
    expect(wrapper.text()).toContain('健康数据已过期')
    expect(wrapper.text()).toContain('3 / 1440min')
  })
})
