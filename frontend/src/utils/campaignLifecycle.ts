import type { Campaign } from '@/api/campaigns'

export type CampaignLifecyclePhase =
  | 'draft'
  | 'warmup'
  | 'active'
  | 'auditing'
  | 'publicizing'
  | 'pending_payout'
  | 'paid'
  | 'cancelled'
  | 'terminated'
  | 'unknown'

export interface CampaignLifecycleState {
  phase: CampaignLifecyclePhase
  labelKey: string
  descriptionKey: string
  targetAt: string | null
  remainingMs: number | null
  countdownText: string | null
  isActionable: boolean
  isUrgent: boolean
  isTerminal: boolean
  hasMissingTarget: boolean
  hasExpiredTarget: boolean
}

const TARGET_BY_STATUS: Partial<Record<CampaignLifecyclePhase, keyof Campaign>> = {
  warmup: 'start_at',
  active: 'end_at',
  auditing: 'audit_end_at',
  publicizing: 'publicity_end_at',
  pending_payout: 'payout_due_at',
}

const PHASE_DESCRIPTION_KEYS: Record<CampaignLifecyclePhase, string> = {
  draft: 'campaignRewards.lifecycle.draftDesc',
  warmup: 'campaignRewards.lifecycle.warmupDesc',
  active: 'campaignRewards.lifecycle.activeDesc',
  auditing: 'campaignRewards.lifecycle.auditingDesc',
  publicizing: 'campaignRewards.lifecycle.publicizingDesc',
  pending_payout: 'campaignRewards.lifecycle.pendingPayoutDesc',
  paid: 'campaignRewards.lifecycle.paidDesc',
  cancelled: 'campaignRewards.lifecycle.cancelledDesc',
  terminated: 'campaignRewards.lifecycle.terminatedDesc',
  unknown: 'campaignRewards.lifecycle.unknownDesc',
}

const PHASE_LABEL_KEYS: Record<CampaignLifecyclePhase, string> = {
  draft: 'campaignRewards.lifecycle.draft',
  warmup: 'campaignRewards.lifecycle.warmup',
  active: 'campaignRewards.lifecycle.active',
  auditing: 'campaignRewards.lifecycle.auditing',
  publicizing: 'campaignRewards.lifecycle.publicizing',
  pending_payout: 'campaignRewards.lifecycle.pendingPayout',
  paid: 'campaignRewards.lifecycle.paid',
  cancelled: 'campaignRewards.lifecycle.cancelled',
  terminated: 'campaignRewards.lifecycle.terminated',
  unknown: 'campaignRewards.lifecycle.unknown',
}

const TERMINAL_PHASES = new Set<CampaignLifecyclePhase>(['paid', 'cancelled', 'terminated'])
const ACTIONABLE_PHASES = new Set<CampaignLifecyclePhase>(['warmup', 'active'])

function normalizePhase(status?: string | null): CampaignLifecyclePhase {
  if (
    status === 'draft' ||
    status === 'warmup' ||
    status === 'active' ||
    status === 'auditing' ||
    status === 'publicizing' ||
    status === 'pending_payout' ||
    status === 'paid' ||
    status === 'cancelled' ||
    status === 'terminated'
  ) {
    return status
  }
  return 'unknown'
}

function parseTime(raw?: string | null): number | null {
  if (!raw) return null
  const value = new Date(raw).getTime()
  return Number.isFinite(value) ? value : null
}

export function formatCampaignRemainingTime(remainingMs: number | null, locale: 'en' | 'zh' = 'en'): string | null {
  if (remainingMs === null || remainingMs <= 0) return null

  const totalSeconds = Math.floor(remainingMs / 1000)
  const days = Math.floor(totalSeconds / 86_400)
  const hours = Math.floor((totalSeconds % 86_400) / 3_600)
  const minutes = Math.floor((totalSeconds % 3_600) / 60)
  const seconds = totalSeconds % 60

  if (days > 0) {
    const dayUnit = locale === 'zh' ? ' 天' : 'd'
    return `${days}${dayUnit} ${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }

  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

export function getCampaignLifecycleState(
  campaign: Campaign,
  now: Date = new Date(),
  locale: 'en' | 'zh' = 'en',
): CampaignLifecycleState {
  const phase = normalizePhase(campaign.status)
  const targetField = TARGET_BY_STATUS[phase]
  const targetAt = targetField ? (campaign[targetField] as string | null | undefined) ?? null : null
  const targetMs = parseTime(targetAt)
  const rawRemainingMs = targetMs === null ? null : targetMs - now.getTime()
  const remainingMs = rawRemainingMs === null ? null : Math.max(0, rawRemainingMs)
  const hasExpiredTarget = rawRemainingMs !== null && rawRemainingMs <= 0

  return {
    phase,
    labelKey: PHASE_LABEL_KEYS[phase],
    descriptionKey: PHASE_DESCRIPTION_KEYS[phase],
    targetAt,
    remainingMs,
    countdownText: formatCampaignRemainingTime(remainingMs, locale),
    isActionable: ACTIONABLE_PHASES.has(phase) && !hasExpiredTarget,
    isUrgent: remainingMs !== null && remainingMs > 0 && remainingMs <= 24 * 60 * 60 * 1000,
    isTerminal: TERMINAL_PHASES.has(phase),
    hasMissingTarget: Boolean(targetField && targetMs === null),
    hasExpiredTarget,
  }
}

export function getCampaignTimeWarnings(campaign: Campaign): string[] {
  const checks: Array<[string | null | undefined, string | null | undefined]> = [
    [campaign.warmup_start_at, campaign.start_at],
    [campaign.start_at, campaign.end_at],
    [campaign.end_at, campaign.audit_start_at],
    [campaign.audit_start_at, campaign.audit_end_at],
    [campaign.audit_end_at, campaign.publicity_start_at],
    [campaign.publicity_start_at, campaign.publicity_end_at],
    [campaign.publicity_end_at, campaign.payout_due_at],
  ]

  return checks.reduce<string[]>((warnings, [from, to]) => {
    const fromTime = parseTime(from)
    const toTime = parseTime(to)
    if (fromTime !== null && toTime !== null && fromTime > toTime) {
      warnings.push('campaignRewards.lifecycle.timeOrderWarning')
    }
    return warnings
  }, [])
}
