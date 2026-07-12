import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LotteryCampaignActivity from '../LotteryCampaignActivity.vue'

const messages: Record<string, string> = {
  'lotteryCampaign.prizePool': 'Prize pool',
  'lotteryCampaign.winnerCount': '{count} winners',
  'lotteryCampaign.entryLadder': 'Lottery entries',
  'lotteryCampaign.weightLadder': 'Lottery entries',
  'lotteryCampaign.ladderNext': 'Use {amount} more to unlock the next entry',
  'lotteryCampaign.ladderMaxed': 'Max entries reached',
  'lotteryCampaign.ladderStart': 'Reach the threshold to earn your first entry',
  'lotteryCampaign.rulesTitle': 'Rules',
  'lotteryCampaign.recentWinners': 'Recent winners',
  'lotteryCampaign.noRecentWinners': 'No winners yet, be the first',
  'lotteryCampaign.participants': 'joining',
  'lotteryCampaign.nextDraw': 'Next draw',
  'lotteryCampaign.drawTime': 'Draw time',
  'lotteryCampaign.pendingDraw': 'Pending draw',
  'lotteryCampaign.thresholdProgressPercent': '{percent}% complete',
  'lotteryCampaign.tokensToThreshold': '{amount} remaining to qualify',
  'lotteryCampaign.entryStatuses.enrolled': 'In pool',
}

const { getActiveLotteryCampaign, getMyLotteryCampaignData, enrollLotteryCampaign, getRecentLotteryWinners, getLotteryParticipants, showError, showSuccess } = vi.hoisted(() => ({
  getActiveLotteryCampaign: vi.fn(),
  getMyLotteryCampaignData: vi.fn(),
  enrollLotteryCampaign: vi.fn(),
  getRecentLotteryWinners: vi.fn(),
  getLotteryParticipants: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/lotteryCampaigns', () => ({
  default: { getActiveLotteryCampaign, getMyLotteryCampaignData, enrollLotteryCampaign, getRecentLotteryWinners, getLotteryParticipants },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
  useAuthStore: () => ({ user: { email: 'current@example.com' } }),
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

function mountActivity() {
  return mount(LotteryCampaignActivity, {
    global: { stubs: { EmptyState: true, Icon: true, LoadingSpinner: true } },
  })
}

const CAMPAIGN = {
  id: 9,
  name: 'Token Lottery',
  description: '',
  rules_text: 'Draw runs nightly.',
  status: 'published',
  participation_mode: 'auto',
  draw_schedule_type: 'daily',
  prize_mode: 'multi',
  entry_mode: 'stepped',
  threshold_tokens: 1_000_000,
  entry_step_tokens: 500_000,
  max_entries_per_user: 5,
  start_at: '2026-07-04T00:00:00.000Z',
  end_at: '2026-07-30T00:00:00.000Z',
  draw_at: null,
  daily_draw_time: '20:00',
  created_at: '2026-07-04T00:00:00.000Z',
  updated_at: '2026-07-04T00:00:00.000Z',
  is_featured: true,
  prize_tiers: [
    { id: 1, campaign_id: 9, tier_name: 'Gold', winner_count: 1, reward_amount_cents: 10_000, sort_order: 0, created_at: '' },
    { id: 2, campaign_id: 9, tier_name: 'Silver', winner_count: 3, reward_amount_cents: 5_000, sort_order: 1, created_at: '' },
  ],
}

describe('LotteryCampaignActivity', () => {
  beforeEach(() => {
    getActiveLotteryCampaign.mockReset()
    getMyLotteryCampaignData.mockReset()
    enrollLotteryCampaign.mockReset()
    getRecentLotteryWinners.mockReset()
    getRecentLotteryWinners.mockResolvedValue({ items: [] })
    getLotteryParticipants.mockReset()
    getLotteryParticipants.mockResolvedValue({ items: [] })
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('renders the prize wall from campaign prize tiers', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      winners: [],
    })

    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.text()).toContain('Prize pool')
    expect(wrapper.text()).toContain('Gold')
    expect(wrapper.text()).toContain('Silver')
    expect(wrapper.text()).toContain('1 winners')
    expect(wrapper.text()).toContain('3 winners')
  })

  it('shows the stepped ladder ratio and next-entry hint', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      winners: [],
    })

    const wrapper = mountActivity()
    await flushPromises()

    // entryCount=3 → next entry at threshold + 3*step = 2.5M; tokens=2M → need 0.5M
    expect(wrapper.text()).toContain('Lottery entries')
    expect(wrapper.text()).toContain('3 / 5')
    expect(wrapper.text()).toContain('Use 0.50M more to unlock the next entry')
  })

  it('renders exactly one threshold progress bar after dedup', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      winners: [],
    })

    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.findAll('[data-testid="lottery-threshold-progress"]')).toHaveLength(1)
  })

  it('renders the recent winners feed with masked identities and amounts', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      winners: [],
    })
    getRecentLotteryWinners.mockResolvedValue({
      items: [
        { masked_email: 'a***@qq.com', prize_name: 'Gold', reward_amount_cents: 10_000, created_at: '2026-07-07T20:00:00.000Z' },
      ],
    })

    const wrapper = mountActivity()
    await flushPromises()

    expect(getRecentLotteryWinners).toHaveBeenCalledWith(9)
    expect(wrapper.text()).toContain('Recent winners')
    expect(wrapper.text()).toContain('a***@qq.com')
  })

  it('shows the empty state when there are no recent winners', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      winners: [],
    })
    getRecentLotteryWinners.mockResolvedValue({ items: [] })

    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.text()).toContain('No winners yet, be the first')
  })

  it('shows the participant count when the draw window has entrants', async () => {
    getActiveLotteryCampaign.mockResolvedValue({ campaign: CAMPAIGN })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2026-07-31T20:00:00.000Z',
      participant_count: 42,
      winners: [],
    })

    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.text()).toContain('joining')
    expect(wrapper.text()).toContain('42')
  })

  it('keeps showing the scheduled draw time after the countdown ends', async () => {
    getActiveLotteryCampaign.mockResolvedValue({
      campaign: {
        ...CAMPAIGN,
        draw_schedule_type: 'single',
        draw_at: '2000-01-01T00:00:00.000Z',
      },
    })
    getMyLotteryCampaignData.mockResolvedValue({
      campaign: CAMPAIGN,
      today_tokens: 2_000_000,
      threshold_tokens: 1_000_000,
      entry_count: 3,
      entry_status: 'enrolled',
      next_draw_at: '2000-01-01T00:00:00.000Z',
      winners: [],
    })

    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.text()).toContain('Jan 1, 2000')
    expect(wrapper.text()).not.toContain('Pending draw')
  })
})
