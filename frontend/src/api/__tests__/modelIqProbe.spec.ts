import { describe, expect, it } from 'vitest'
import { parseModelIqProbeSummary } from '@/api/modelIqProbe'

describe('parseModelIqProbeSummary', () => {
  it('parses items with latest verdict, excerpt and recent timeline', () => {
    const summary = parseModelIqProbeSummary({
      items: [
        {
          model: 'gpt-5.2',
          latest: {
            status: 'pass',
            answer_excerpt: '所以最少取出 21 个糖果。',
            checked_at: '2026-10-26T03:00:00Z',
          },
          recent: [
            { status: 'wrong_answer', checked_at: '2026-10-26T01:00:00Z' },
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
      checked_at: '2026-10-26T03:00:00Z',
    })
    expect(summary.items[0].recent).toHaveLength(2)
    expect(summary.items[0].recent[0].status).toBe('wrong_answer')
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
})
