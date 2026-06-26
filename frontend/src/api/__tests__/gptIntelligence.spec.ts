import { afterEach, describe, expect, it, vi } from 'vitest'
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
  vi.useRealTimers()
  vi.unstubAllGlobals()
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

  it('falls back to UTC when the source timezone is invalid', () => {
    const snapshot = parseGptIntelligenceSnapshot({
      ...payload,
      timezone: 'Mars/OlympusMons',
    })

    expect(snapshot.timezone).toBe('UTC')
  })
})

describe('fetchGptIntelligenceSnapshot', () => {
  it('passes a cancellable signal to fetch and parses the response', async () => {
    const fetchMock = vi.fn(async (_url: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.signal).toBeInstanceOf(AbortSignal)
      return new Response(JSON.stringify(payload), { status: 200 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const snapshot = await fetchGptIntelligenceSnapshot({ timeoutMs: 1000 })

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(snapshot.latest?.score).toBe(125)
  })

  it('aborts the fetch when the caller signal aborts', async () => {
    const ctrl = new AbortController()
    let passedSignal: AbortSignal | null = null
    const fetchMock = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      passedSignal = init?.signal ?? null
      return new Promise<Response>(() => {})
    })
    vi.stubGlobal('fetch', fetchMock)

    void fetchGptIntelligenceSnapshot({ signal: ctrl.signal, timeoutMs: 1000 }).catch(() => undefined)
    await vi.waitFor(() => expect(passedSignal).toBeInstanceOf(AbortSignal))

    ctrl.abort()

    expect(passedSignal?.aborted).toBe(true)
  })

  it('aborts the fetch when the timeout elapses', async () => {
    vi.useFakeTimers()
    let passedSignal: AbortSignal | null = null
    const fetchMock = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      passedSignal = init?.signal ?? null
      return new Promise<Response>(() => {})
    })
    vi.stubGlobal('fetch', fetchMock)

    void fetchGptIntelligenceSnapshot({ timeoutMs: 25 }).catch(() => undefined)
    await vi.waitFor(() => expect(passedSignal).toBeInstanceOf(AbortSignal))

    vi.advanceTimersByTime(25)

    expect(passedSignal?.aborted).toBe(true)
  })
})
