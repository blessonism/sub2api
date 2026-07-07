import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CampaignRewardsView from '../CampaignRewardsView.vue'

const {
  addPoolAdjustment,
  copyCampaign,
  createCampaign,
  createConfigVersion,
  deleteCampaign,
  freezeLeaderboard,
  getCampaign,
  getFinalRewardResults,
  getLeaderboard,
  getPoolSummary,
  listCampaigns,
  payoutCampaign,
  publishCampaign,
  recalculateRewards,
  updateCampaign,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  addPoolAdjustment: vi.fn(),
  copyCampaign: vi.fn(),
  createCampaign: vi.fn(),
  createConfigVersion: vi.fn(),
  deleteCampaign: vi.fn(),
  freezeLeaderboard: vi.fn(),
  getCampaign: vi.fn(),
  getFinalRewardResults: vi.fn(),
  getLeaderboard: vi.fn(),
  getPoolSummary: vi.fn(),
  listCampaigns: vi.fn(),
  payoutCampaign: vi.fn(),
  publishCampaign: vi.fn(),
  recalculateRewards: vi.fn(),
  updateCampaign: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    campaigns: {
      addPoolAdjustment,
      copyCampaign,
      createCampaign,
      createConfigVersion,
      deleteCampaign,
      freezeLeaderboard,
      getCampaign,
      getFinalRewardResults,
      getLeaderboard,
      getPoolSummary,
      listCampaigns,
      payoutCampaign,
      publishCampaign,
      recalculateRewards,
      updateCampaign,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let text = key
        for (const [name, value] of Object.entries(params ?? {})) {
          text = text.replace(`{${name}}`, String(value))
        }
        return text
      },
      locale: { value: 'en' },
    }),
  }
})

const campaign = {
  id: 9,
  name: '管理员活动',
  description: '运营说明',
  cover_url: '',
  rules_text: '',
  status: 'draft',
  warmup_start_at: '2026-07-05T00:00:00.000Z',
  start_at: '2026-07-04T00:00:00.000Z',
  end_at: '2026-07-06T00:00:00.000Z',
  audit_start_at: '2026-07-07T00:00:00.000Z',
  audit_end_at: null,
  publicity_start_at: '2026-07-08T00:00:00.000Z',
  publicity_end_at: null,
  payout_due_at: null,
  created_at: '2026-07-01T00:00:00.000Z',
  updated_at: '2026-07-01T00:00:00.000Z',
}

function mountView() {
  return mount(CampaignRewardsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show" data-testid="base-dialog"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: {
          props: ['show', 'title', 'message', 'confirmText', 'danger'],
          emits: ['confirm', 'cancel'],
          template: `
            <div v-if="show" data-testid="confirm-dialog">
              <h3>{{ title }}</h3>
              <p>{{ message }}</p>
              <slot />
              <button type="button" data-testid="confirm" @click="$emit('confirm')">{{ confirmText }}</button>
              <button type="button" data-testid="cancel" @click="$emit('cancel')">cancel</button>
            </div>
          `,
        },
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

describe('admin CampaignRewardsView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-04T00:00:00.000Z'))
    addPoolAdjustment.mockReset()
    copyCampaign.mockReset()
    createCampaign.mockReset()
    createConfigVersion.mockReset()
    deleteCampaign.mockReset()
    freezeLeaderboard.mockReset()
    getCampaign.mockReset()
    getFinalRewardResults.mockReset()
    getLeaderboard.mockReset()
    getPoolSummary.mockReset()
    listCampaigns.mockReset()
    payoutCampaign.mockReset()
    publishCampaign.mockReset()
    recalculateRewards.mockReset()
    updateCampaign.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listCampaigns.mockResolvedValue({ items: [campaign], total: 1, page: 1, page_size: 50 })
    getCampaign.mockResolvedValue(campaign)
    getPoolSummary.mockResolvedValue({
      campaign_id: 9,
      confirmed_pool_cents: 10000,
      pending_pool_cents: 500,
      estimated_total_pool_cents: 10500,
      adjustment_total_cents: 0,
      deducted_pool_cents: 0,
      final_pool_cents: 10000,
    })
    getLeaderboard.mockResolvedValue({ items: [] })
    getFinalRewardResults.mockRejectedValue({ response: { data: { code: 'CAMPAIGN_NO_FINAL_SETTLEMENT' } } })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders campaign countdown, user preview, timeline warning, and timeline fallback', async () => {
    getCampaign.mockResolvedValueOnce({ ...campaign, status: 'auditing' })

    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('管理员活动')
    expect(text).toContain('admin.campaignRewards.userPreview')
    expect(text).toContain('admin.campaignRewards.timelineTitle')
    expect(text).toContain('admin.campaignRewards.timelineAuditStart')
    expect(text).toContain('admin.campaignRewards.timelinePublicityStart')
    expect(text).toContain('campaignRewards.lifecycle.timeOrderWarning')
    expect(text).toContain('admin.campaignRewards.timelineMissing')
  })

  it('renders the campaign list as a full-width responsive selector instead of a sticky sidebar', async () => {
    const wrapper = mountView()
    await flushPromises()

    const listCard = wrapper.get('[data-testid="campaign-list-card"]')
    const listItems = wrapper.get('[data-testid="campaign-list-items"]')

    expect(listCard.classes()).not.toContain('xl:sticky')
    expect(listItems.classes()).toEqual(expect.arrayContaining(['grid', 'md:grid-cols-2', '2xl:grid-cols-3']))
  })

  it('keeps payout disabled until a local final calculation exists', async () => {
    const wrapper = mountView()
    await flushPromises()

    const payoutButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.payout'))
    expect(payoutButton).toBeTruthy()
    expect(payoutButton!.attributes('disabled')).toBeDefined()
    expect(payoutButton!.attributes('title')).toBe('admin.campaignRewards.payoutNeedsFinalLocal')
    expect(wrapper.text()).toContain('admin.campaignRewards.payoutNeedsFinalLocal')

    await payoutButton!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
    expect(payoutCampaign).not.toHaveBeenCalled()
  })

  it('loads persisted final calculation and allows payout after page reload', async () => {
    getFinalRewardResults.mockResolvedValue({
      campaign_id: 9,
      calculation_status: 'final',
      calculation_batch_no: 'final-1',
      final_pool_cents: 10000,
      rank_pool_cents: 8000,
      contribution_pool_cents: 2000,
      total_gross_reward_cents: 1000,
      total_final_payout_cents: 1000,
      total_withheld_cents: 0,
      total_rounding_residual_cents: 0,
      results: [{
        id: 1,
        campaign_id: 9,
        config_version_id: 1,
        calculation_batch_no: 'final-1',
        user_id: 42,
        rank: 1,
        rank_reward_amount_cents: 800,
        contribution_weight: '1',
        contribution_reward_amount_cents: 200,
        gross_reward_amount_cents: 1000,
        min_payout_amount_snapshot_cents: 100,
        final_payout_amount_cents: 1000,
        withheld_amount_cents: 0,
        withheld_reason: '',
        rounding_residual_cents: 0,
        calculation_status: 'final',
        calculated_at: '2026-07-04T00:00:00.000Z',
      }],
    })

    const wrapper = mountView()
    await flushPromises()

    const payoutButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.payout'))
    expect(payoutButton).toBeTruthy()
    expect(payoutButton!.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('admin.campaignRewards.rewardResults')

    await payoutButton!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmPayoutTitle')
  })

  it('blocks create campaign when pool ratios and weights do not match backend rules', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    const inputs = dialog.findAll('input')
    await inputs[6].setValue('0.9')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.campaignRewards.createPoolRatioInvalid')

    await inputs[6].setValue('0.8')
    await inputs[9].setValue('30,20,15,10,8,6,4,3,2,1')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.campaignRewards.createWeightsSumInvalid')
    expect(createCampaign).not.toHaveBeenCalled()
  })

  it('requires confirmation before publishing a campaign', async () => {
    publishCampaign.mockResolvedValue({ ...campaign, status: 'warmup' })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.publish'))?.trigger('click')
    await flushPromises()

    expect(publishCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmPublishTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(publishCampaign).toHaveBeenCalledWith(9)
  })

  it('updates campaign details and lifecycle times from the edit dialog', async () => {
    const updated = {
      ...campaign,
      name: '生产邀请活动',
      end_at: new Date('2026-07-09T12:30').toISOString(),
      audit_start_at: new Date('2026-07-10T09:00').toISOString(),
      audit_end_at: null,
    }
    updateCampaign.mockResolvedValue(updated)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.editCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    await dialog.find('input[type="text"]').setValue('生产邀请活动')
    const timeInputs = dialog.findAll('input[type="datetime-local"]')
    await timeInputs[2].setValue('2026-07-09T12:30')
    await timeInputs[3].setValue('2026-07-10T09:00')
    await timeInputs[4].setValue('')

    await dialog.findAll('button').find(button => button.text().includes('admin.campaignRewards.saveCampaign'))?.trigger('click')
    await flushPromises()

    expect(updateCampaign).toHaveBeenCalledWith(9, expect.objectContaining({
      name: '生产邀请活动',
      end_at: new Date('2026-07-09T12:30').toISOString(),
      audit_start_at: new Date('2026-07-10T09:00').toISOString(),
      audit_end_at: null,
    }))
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.updated')
  })

  it('requires confirmation before copying a campaign as draft', async () => {
    const copied = { ...campaign, id: 19, name: '管理员活动 副本', status: 'draft' }
    copyCampaign.mockResolvedValue({ campaign: copied, config_version: { id: 88 } })
    getCampaign.mockImplementation(async (id: number) => (id === 19 ? copied : campaign))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.copyAsDraft'))?.trigger('click')
    await flushPromises()

    expect(copyCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmCopyTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(copyCampaign).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.copiedAsDraft')
    expect(getCampaign).toHaveBeenCalledWith(19)
  })

  it('requires confirmation before deleting or archiving a campaign', async () => {
    deleteCampaign.mockResolvedValue({
      action: 'deleted',
      campaign: null,
      impact: {
        participants: 0,
        invite_records: 0,
        pool_entries: 0,
        pool_adjustments: 0,
        leaderboard_snapshots: 0,
        reward_results: 0,
        payout_batches: 0,
        payout_items: 0,
      },
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.deleteOrArchive'))?.trigger('click')
    await flushPromises()

    expect(deleteCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmDeleteTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(deleteCampaign).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.deleted')
  })
})
