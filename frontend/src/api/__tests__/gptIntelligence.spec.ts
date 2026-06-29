import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import { fetchGptIntelligenceSnapshot, parseGptIntelligenceSnapshot } from '@/api/gptIntelligence'

const payload = {
  monitored_at: '2026-06-25T00:55:43.703187+08:00',
  timezone: 'Asia/Shanghai',
  status: 'none',
  model_iq: {
    latest: {
      date: '2026-06-24-pm',
      score: 125,
      status: 'green',
      passed: 10,
      tasks: 12,
      total_tokens: 39090118,
      output_tokens: 363304,
      wall_seconds: 1503,
      wall_time_human: '25分钟',
      model: 'gpt-5.5',
      reasoning_effort: 'xhigh',
      cost_usd: 40.383558,
    },
    recent_days: [
      { date: '2026-06-24-pm', score: 125, status: 'green', passed: 10, tasks: 12 },
    ],
    comparisons: {
      gpt_55_high: {
        label: 'GPT-5.5 high',
        model: 'gpt-5.5',
        reasoning_effort: 'high',
        latest: { date: '2026-06-24-pm', score: 87.5, status: 'yellow', passed: 7, tasks: 12 },
        recent_days: [
          { date: '2026-06-23', score: 100, status: 'green', passed: 8, tasks: 12 },
          { date: '2026-06-24-pm', score: 87.5, status: 'yellow', passed: 7, tasks: 12 },
        ],
      },
    },
    quota_radar: {
      basis_window_label: '5h',
      cost_usd: 119.521482,
      rate: 2.8457,
      adjusted_delta: 42,
      updated_at: '2026-06-24T04:58:05Z',
    },
  },
}

afterEach(() => {
  apiClient.defaults.adapter = undefined
  vi.restoreAllMocks()
})

describe('parseGptIntelligenceSnapshot', () => {
  it('extracts model_iq fields from the CodexRadar payload', () => {
    const snapshot = parseGptIntelligenceSnapshot(payload)

    expect(snapshot.latest?.score).toBe(125)
    expect(snapshot.latest?.status).toBe('green')
    expect(snapshot.timezone).toBe('Asia/Shanghai')
    expect(snapshot.recent_days).toHaveLength(1)
    expect(snapshot.comparisons[0]).toMatchObject({
      key: 'gpt_55_high',
      label: 'GPT-5.5 high',
    })
    expect(snapshot.comparisons[0]?.recent_days).toHaveLength(2)
    expect(snapshot.comparisons[0]?.recent_days[0]?.score).toBe(100)
    expect(snapshot.quota_radar?.basis_window_label).toBe('5h')
  })

  it('rejects payloads without model_iq data', () => {
    expect(() => parseGptIntelligenceSnapshot({ status: 'none' })).toThrow('missing model_iq payload')
  })

  it('accepts backend-normalized snapshots without the legacy model_iq wrapper', () => {
    const snapshot = parseGptIntelligenceSnapshot({
      monitored_at: '2026-06-29T09:00:00Z',
      timezone: 'Asia/Shanghai',
      latest: {
        date: '2026-06-29-pm',
        score: 75,
        status: 'yellow',
        passed: 6,
        tasks: 12,
        model: 'GPT-5.5',
        reasoning_effort: 'xhigh',
      },
      recent_days: [],
      comparisons: [
        {
          key: 'gpt_55_high',
          label: 'GPT-5.5 high',
          model: 'GPT-5.5',
          reasoning_effort: 'high',
          latest: { date: '2026-06-29-pm', score: 87.5, status: 'yellow', passed: 7, tasks: 12 },
          recent_days: [],
        },
      ],
      quota_radar: null,
    })

    expect(snapshot.latest?.score).toBe(75)
    expect(snapshot.comparisons[0]?.key).toBe('gpt_55_high')
    expect(snapshot.quota_radar).toBeNull()
  })

  it('falls back to UTC when the source timezone is invalid', () => {
    const snapshot = parseGptIntelligenceSnapshot({
      ...payload,
      timezone: 'Mars/OlympusMons',
    })

    expect(snapshot.timezone).toBe('UTC')
  })
})

describe('fetchGptIntelligenceSnapshot', () => {
  it('loads the snapshot through the backend channel monitor endpoint', async () => {
    const ctrl = new AbortController()
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: payload.model_iq, message: 'success' },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    apiClient.defaults.adapter = adapter

    const snapshot = await fetchGptIntelligenceSnapshot({ signal: ctrl.signal, timeoutMs: 1000 })

    expect(adapter).toHaveBeenCalledOnce()
    const config = adapter.mock.calls[0][0]
    expect(config.url).toBe('/channel-monitors/gpt-intelligence')
    expect(config.signal).toBe(ctrl.signal)
    expect(config.timeout).toBe(1000)
    expect(snapshot.latest?.score).toBe(125)
  })
})
