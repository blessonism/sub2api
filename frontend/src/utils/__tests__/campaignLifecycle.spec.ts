import { describe, expect, it } from 'vitest'
import type { Campaign } from '@/api/campaigns'
import {
  formatCampaignRemainingTime,
  getCampaignLifecycleState,
  getCampaignTimeWarnings,
} from '../campaignLifecycle'

const baseCampaign: Campaign = {
  id: 1,
  name: '邀请活动',
  description: '',
  cover_url: '',
  rules_text: '',
  status: 'active',
  start_at: '2026-07-01T00:00:00.000Z',
  end_at: '2026-07-10T00:00:00.000Z',
  created_at: '2026-07-01T00:00:00.000Z',
  updated_at: '2026-07-01T00:00:00.000Z',
}

describe('campaignLifecycle', () => {
  it('derives active countdown from end_at without negative display', () => {
    const state = getCampaignLifecycleState(baseCampaign, new Date('2026-07-09T23:59:30.000Z'))

    expect(state.phase).toBe('active')
    expect(state.labelKey).toBe('campaignRewards.lifecycle.active')
    expect(state.targetAt).toBe(baseCampaign.end_at)
    expect(state.countdownText).toBe('00:00:30')
    expect(state.isActionable).toBe(true)
    expect(state.isUrgent).toBe(true)
  })

  it('falls back to phase explanation when lifecycle target is missing', () => {
    const state = getCampaignLifecycleState(
      { ...baseCampaign, status: 'auditing', audit_end_at: null },
      new Date('2026-07-11T00:00:00.000Z'),
    )

    expect(state.phase).toBe('auditing')
    expect(state.targetAt).toBeNull()
    expect(state.countdownText).toBeNull()
    expect(state.hasMissingTarget).toBe(true)
    expect(state.isActionable).toBe(false)
  })

  it('marks active campaigns with expired end_at as non-actionable', () => {
    const state = getCampaignLifecycleState(baseCampaign, new Date('2026-07-10T00:00:01.000Z'))

    expect(state.phase).toBe('active')
    expect(state.remainingMs).toBe(0)
    expect(state.countdownText).toBeNull()
    expect(state.hasExpiredTarget).toBe(true)
    expect(state.isActionable).toBe(false)
  })

  it('formats multi-day and same-day remaining time consistently', () => {
    expect(formatCampaignRemainingTime(2 * 86_400_000 + 3 * 3_600_000 + 12 * 60_000 + 8_000)).toBe('2d 03:12:08')
    expect(formatCampaignRemainingTime(2 * 86_400_000 + 3 * 3_600_000 + 12 * 60_000 + 8_000, 'zh')).toBe('2 天 03:12:08')
    expect(formatCampaignRemainingTime(3 * 3_600_000 + 12 * 60_000 + 8_000)).toBe('03:12:08')
    expect(formatCampaignRemainingTime(0)).toBeNull()
  })

  it('reports admin timeline order warnings for reversed dates', () => {
    const warnings = getCampaignTimeWarnings({
      ...baseCampaign,
      warmup_start_at: '2026-07-02T00:00:00.000Z',
      start_at: '2026-07-01T00:00:00.000Z',
    })

    expect(warnings).toEqual(['campaignRewards.lifecycle.timeOrderWarning'])
  })
})
