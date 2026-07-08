import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CampaignRewardsView from '../CampaignRewardsView.vue'

const messages: Record<string, string> = {
  'campaignRewards.periodElapsed': '{percent}% elapsed',
  'campaignRewards.ruleSummaryThresholdTitle': 'Valid Invite Threshold',
  'campaignRewards.ruleSummaryThresholdDesc': 'threshold {threshold}',
  'campaignRewards.ruleSummaryPoolTitle': 'Bonus Pool Allocation',
  'campaignRewards.ruleSummaryPoolDesc': 'pool {rate} split {rank} {contribution}',
  'campaignRewards.ruleSummaryPoolDescInviteesOnly': 'pool {rate} split {rank} {contribution}',
  'campaignRewards.ruleSummaryPoolDescAllUsers': 'pool {rate} split {rank} {contribution}',
  'campaignRewards.ruleSummarySettlementTitle': 'Audit & Settlement',
  'campaignRewards.ruleSummarySettlementDesc': 'settlement estimate',
  'campaignRewards.pendingRechargeInvites': 'Pending Recharge Invites',
  'campaignRewards.myCode': 'My Invite Code',
  'campaignRewards.inviteLink': 'Invite Link',
  'campaignRewards.copyCode': 'Copy Code',
  'campaignRewards.copyLink': 'Copy Link',
  'campaignRewards.codeCopied': 'Invite code copied',
  'campaignRewards.linkCopied': 'Invite link copied',
}

const {
  copyToClipboard,
  getActiveCampaign,
  getCampaignLeaderboard,
  getMyCampaignData,
  listCampaignInvites,
  showError,
} = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getActiveCampaign: vi.fn(),
  getCampaignLeaderboard: vi.fn(),
  getMyCampaignData: vi.fn(),
  listCampaignInvites: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/campaigns', () => ({
  default: {
    getActiveCampaign,
    getMyCampaignData,
    listCampaignInvites,
    getCampaignLeaderboard,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en' },
      t: (key: string, params?: Record<string, unknown>) => {
        let text = messages[key] ?? key
        for (const [name, value] of Object.entries(params ?? {})) {
          text = text.replace(`{${name}}`, String(value))
        }
        return text
      },
    }),
  }
})

const campaign = {
  id: 7,
  name: '七月邀请活动',
  description: '邀请好友拿奖励',
  cover_url: '',
  rules_text: '后端配置的完整规则',
  status: 'active',
  start_at: '2026-07-04T00:00:00.000Z',
  end_at: '2026-07-06T03:12:08.000Z',
  created_at: '2026-07-01T00:00:00.000Z',
  updated_at: '2026-07-01T00:00:00.000Z',
}

function mountView() {
  return mount(CampaignRewardsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        EmptyState: {
          props: ['title', 'description'],
          template: '<div data-testid="empty">{{ title }} {{ description }}<slot /></div>',
        },
        Icon: { template: '<span data-testid="icon" />' },
        LoadingSpinner: { template: '<div data-testid="loading" />' },
      },
    },
  })
}

describe('user CampaignRewardsView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-04T00:00:00.000Z'))
    copyToClipboard.mockReset()
    getActiveCampaign.mockReset()
    getCampaignLeaderboard.mockReset()
    getMyCampaignData.mockReset()
    listCampaignInvites.mockReset()
    showError.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders lifecycle countdown, primary CTA, rule summaries, and invite status notes', async () => {
    vi.setSystemTime(new Date('2026-07-05T00:00:00.000Z'))
    getActiveCampaign.mockResolvedValue({
      campaign,
      config: {
        recharge_threshold_cents: 2000,
        pool_injection_rate: '0.125',
        rank_pool_ratio: '0.7',
        contribution_pool_ratio: '0.3',
      },
      pool: { estimated_total_pool_cents: 123456 },
      leaderboard: [],
      data_delay_notice: '',
      estimate_notice: '',
    })
    getMyCampaignData.mockResolvedValue({
      campaign_id: 7,
      user_id: 42,
      invite_code: 'INV42',
      invite_link: '/invite/INV42',
      valid_invite_count: 3,
      pending_invite_count: 1,
      invalid_invite_count: 0,
      current_rank: null,
      estimated_rank_reward_cents: 1000,
      estimated_contribution_reward_cents: 500,
      estimated_total_reward_cents: 1500,
      invitee_recharge_amount_cents: 8888,
      distance_to_previous: 0,
      distance_to_top10: 2,
      invite_records: [],
    })
    listCampaignInvites.mockResolvedValue({
      items: [
        {
          id: 1,
          campaign_id: 7,
          config_version_id: 1,
          inviter_user_id: 42,
          invitee_user_id: 100,
          invitee_masked_email: 'fri***nd@example.com',
          invite_source: 'link',
          threshold_snapshot_cents: 2000,
          registered_at: '2026-07-04T01:00:00.000Z',
          qualified_at: null,
          effective_recharge_amount_cents: 0,
          status: 'recharge_unqualified',
          risk_level: 'none',
          invalid_reason: '',
          audit_status: '',
          audit_note: '',
        },
      ],
      total: 1,
      page: 1,
      page_size: 100,
    })
    getCampaignLeaderboard.mockResolvedValue({
      items: [
        {
          rank: 1,
          user_id: 42,
          username: 'me',
          valid_invite_count: 3,
          pending_invite_count: 1,
          invitee_recharge_amount_cents: 8888,
          reached_count_at: '2026-07-04T01:00:00.000Z',
          joined_at: '2026-07-04T01:00:00.000Z',
          estimated_reward_cents: 1500,
          final_reward_cents: 0,
        },
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(wrapper.find('[data-testid="activity-switcher"]').exists()).toBe(false)
    expect(text).toContain('七月邀请活动')
    expect(text).toContain('campaignRewards.lifecycle.active')
    expect(text).toContain('campaignRewards.lifecycle.endsAt')
    expect(text).toContain('campaignRewards.campaignPeriod')
    expect(text).toContain('7/4 - 7/6')
    expect(text).toContain('47% elapsed')
    expect(text).toContain('1d 03:12:08')
    expect(text).not.toContain('→')
    const periodProgress = wrapper.get('[role="progressbar"]')
    expect(periodProgress.attributes('aria-valuenow')).toBe('47')
    expect(periodProgress.attributes('data-progress-value')).toBe('47')
    expect(text).toContain('campaignRewards.copyInviteLinkPrimary')
    expect(text).toContain('My Invite Code')
    expect(text).toContain('Invite Link')
    expect(text).toContain('Copy Code')
    expect(text).toContain('Copy Link')
    expect(text).toContain('campaignRewards.distanceToTop10')
    expect(wrapper.get('[data-testid="campaign-rules-layout"]').classes()).toContain('xl:items-stretch')
    expect(wrapper.get('[data-testid="campaign-rules-card"]').classes()).toContain('h-full')
    expect(wrapper.get('[data-testid="campaign-rules-card"]').classes()).not.toContain('self-start')
    expect(wrapper.findAll('[data-testid="campaign-rule-summary-item"]')).toHaveLength(3)
    expect(text).toContain('Valid Invite Threshold')
    expect(text).toContain('threshold ¥20.00')
    expect(text).toContain('Bonus Pool Allocation')
    expect(text).toContain('pool 12.5% split 70% 30%')
    expect(text).toContain('Audit & Settlement')
    expect(text).toContain('settlement estimate')
    expect(wrapper.find('[data-testid="campaign-full-rules"]').exists()).toBe(false)
    expect(text).not.toContain('后端配置的完整规则')
    expect(text).toContain('Pending Recharge Invites')
    expect(text).not.toContain('campaignRewards.rechargeAmount')
    expect(text).toContain('campaignRewards.inviteStatusNotes.recharge_unqualified')
    expect(text).toContain('campaignRewards.rechargeProgress')
    expect(text).toContain('campaignRewards.timeline')
    expect(text).toContain('campaignRewards.notQualifiedYet')

    await wrapper.findAll('button').find(button => button.text().includes('Copy Code'))?.trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('INV42', 'Invite code copied')

    await wrapper.findAll('button').find(button => button.text().includes('Copy Link'))?.trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('http://localhost:3000/invite/INV42', 'Invite link copied')
  })

  it('disables the hero CTA after active participation stage', async () => {
    getActiveCampaign.mockResolvedValue({
      campaign: { ...campaign, status: 'auditing', audit_end_at: null },
      config: null,
      pool: null,
      leaderboard: [],
      data_delay_notice: '',
      estimate_notice: '',
    })
    getMyCampaignData.mockResolvedValue({
      campaign_id: 7,
      user_id: 42,
      invite_code: 'INV42',
      invite_link: '/invite/INV42',
      valid_invite_count: 0,
      pending_invite_count: 0,
      invalid_invite_count: 0,
      current_rank: null,
      estimated_rank_reward_cents: 0,
      estimated_contribution_reward_cents: 0,
      estimated_total_reward_cents: 0,
      invitee_recharge_amount_cents: 0,
      distance_to_previous: 0,
      distance_to_top10: 0,
      invite_records: [],
    })
    listCampaignInvites.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100 })
    getCampaignLeaderboard.mockResolvedValue({ items: [] })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('campaignRewards.lifecycle.targetMissing')
    expect(wrapper.findAll('button').some(button => button.text().includes('campaignRewards.copyInviteLinkPrimary') && button.attributes('disabled') !== undefined)).toBe(true)
  })

  it('treats active campaigns past end_at as ended and not actionable', async () => {
    getActiveCampaign.mockResolvedValue({
      campaign: { ...campaign, end_at: '2026-07-03T23:59:59.000Z' },
      config: {
        recharge_threshold_cents: 2000,
        pool_injection_rate: '0.1',
        rank_pool_ratio: '0.8',
        contribution_pool_ratio: '0.2',
      },
      pool: null,
      leaderboard: [],
      data_delay_notice: '',
      estimate_notice: '',
    })
    getMyCampaignData.mockResolvedValue({
      campaign_id: 7,
      user_id: 42,
      invite_code: 'INV42',
      invite_link: '/invite/INV42',
      valid_invite_count: 0,
      pending_invite_count: 0,
      invalid_invite_count: 0,
      current_rank: null,
      estimated_rank_reward_cents: 0,
      estimated_contribution_reward_cents: 0,
      estimated_total_reward_cents: 0,
      invitee_recharge_amount_cents: 0,
      distance_to_previous: 0,
      distance_to_top10: 0,
      invite_records: [],
    })
    listCampaignInvites.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100 })
    getCampaignLeaderboard.mockResolvedValue({ items: [] })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('campaignRewards.lifecycle.expired')
    const inviteButtons = wrapper.findAll('button').filter(button => button.text().includes('campaignRewards.copyInviteLinkPrimary'))
    expect(inviteButtons.length).toBeGreaterThanOrEqual(2)
    expect(inviteButtons.every(button => button.attributes('disabled') !== undefined)).toBe(true)
    expect(copyToClipboard).not.toHaveBeenCalled()
  })
})
