import { describe, expect, it, vi } from 'vitest'
import { parseModelIqProbeSummary } from '@/api/modelIqProbe'
import { getRunState, runProbe, updateConfig } from '@/api/admin/modelIqProbe'

const { get, post, put } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put } }))

describe('parseModelIqProbeSummary', () => {
  it('parses items with latest verdict, excerpt and recent timeline', () => {
    const summary = parseModelIqProbeSummary({
      items: [
        {
          model: 'gpt-5.2',
          latest: {
            status: 'pass',
            answer_excerpt: '所以最少取出 21 个糖果。',
            answer: '所以最少取出 21 个糖果：星形 12 个、圆形 9 个。',
            checked_at: '2026-10-26T03:00:00Z',
          },
          recent: [
            { status: 'wrong_answer', checked_at: '2026-10-26T01:00:00Z', answer_excerpt: '答案是 20 个。' },
            { status: 'pass', checked_at: '2026-10-26T03:00:00Z' },
          ],
        },
      ],
    })

    expect(summary.items).toHaveLength(1)
    expect(summary.items[0].model).toBe('gpt-5.2')
    expect(summary.items[0].latest).toMatchObject({
      status: 'pass',
      answer_excerpt: '所以最少取出 21 个糖果。',
      answer: '所以最少取出 21 个糖果：星形 12 个、圆形 9 个。',
      checked_at: '2026-10-26T03:00:00Z',
    })
    expect(summary.items[0].recent).toHaveLength(2)
    expect(summary.items[0].recent[0].status).toBe('wrong_answer')
    expect(summary.items[0].recent[0].answer_excerpt).toBe('答案是 20 个。')
  })

  it('normalizes unknown statuses and drops invalid entries', () => {
    const summary = parseModelIqProbeSummary({
      items: [
        { model: 'gpt-5.1', latest: { status: 'weird', checked_at: '2026-10-26T03:00:00Z' }, recent: [{ status: 42, checked_at: 'x' }] },
        { latest: { status: 'pass', checked_at: 'x' } },
        'not-an-object',
      ],
    })

    expect(summary.items).toHaveLength(1)
    expect(summary.items[0].model).toBe('gpt-5.1')
    expect(summary.items[0].latest?.status).toBe('unknown')
    expect(summary.items[0].recent[0].status).toBe('unknown')
  })

  it('rejects payloads without an items array', () => {
    expect(() => parseModelIqProbeSummary({})).toThrow('invalid model iq probe payload')
    expect(() => parseModelIqProbeSummary({ items: 'nope' })).toThrow('invalid model iq probe payload')
  })

  it('keeps empty item lists (probe disabled or no data)', () => {
    const summary = parseModelIqProbeSummary({ items: [] })
    expect(summary.items).toEqual([])
  })

  it('only accepts boolean exclusion markers without exposing extra fields', () => {
    const summary = parseModelIqProbeSummary({ items: [{ model: 'gpt-5.4',
      latest: { status: 'pass', checked_at: 'now', marked_for_exclusion: true, account_id: 42 },
      recent: [{ status: 'pass', checked_at: 'now', marked_for_exclusion: 'true' }],
    }] })
    expect(summary.items[0].latest?.marked_for_exclusion).toBe(true)
    expect(summary.items[0].recent[0].marked_for_exclusion).toBe(false)
    expect(summary.items[0].latest).not.toHaveProperty('account_id')
  })
})

describe('admin model IQ probe API', () => {
  it('starts a background run and fetches shared progress with cancellation', async () => {
    const state = { id: 'run-1', status: 'running' }
    post.mockResolvedValue({ data: state })
    get.mockResolvedValue({ data: state })
    const signal = new AbortController().signal
    expect(await runProbe()).toEqual(state)
    expect(post).toHaveBeenCalledWith('/admin/model-iq-probe/run')
    expect(await getRunState({ signal })).toEqual(state)
    expect(get).toHaveBeenCalledWith('/admin/model-iq-probe/run', { signal })
  })

  it('persists all display options with the probe configuration', async () => {
    const config = { enabled: true, visibility: 'all_users' as const, user_display_mode: 'always_green' as const,
      show_exclusion: true, question: 'Question', expected_answer: '21', interval_minutes: 360,
      target_mode: 'account_model' as const, models: [], account_ids: [], }
    put.mockResolvedValue({ data: config })
    expect(await updateConfig(config)).toEqual(config)
    expect(put).toHaveBeenCalledWith('/admin/model-iq-probe/config', config)
  })
})
