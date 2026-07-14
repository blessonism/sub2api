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
  getCampaignConfig,
  getFinalRewardResults,
  getLeaderboard,
  getPoolSummary,
  listCampaigns,
  payoutCampaign,
  pauseCampaign,
  publishCampaign,
  recalculateRewards,
  resumeCampaign,
  unfreezeCampaign,
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
  getCampaignConfig: vi.fn(),
  getFinalRewardResults: vi.fn(),
  getLeaderboard: vi.fn(),
  getPoolSummary: vi.fn(),
  listCampaigns: vi.fn(),
  payoutCampaign: vi.fn(),
  pauseCampaign: vi.fn(),
  publishCampaign: vi.fn(),
  recalculateRewards: vi.fn(),
  resumeCampaign: vi.fn(),
  unfreezeCampaign: vi.fn(),
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
      getCampaignConfig,
      getFinalRewardResults,
      getLeaderboard,
      getPoolSummary,
      listCampaigns,
      payoutCampaign,
      pauseCampaign,
      publishCampaign,
      recalculateRewards,
      resumeCampaign,
      unfreezeCampaign,
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
    getCampaignConfig.mockReset()
    getFinalRewardResults.mockReset()
    getLeaderboard.mockReset()
    getPoolSummary.mockReset()
    listCampaigns.mockReset()
    payoutCampaign.mockReset()
    pauseCampaign.mockReset()
    publishCampaign.mockReset()
    recalculateRewards.mockReset()
    resumeCampaign.mockReset()
    unfreezeCampaign.mockReset()
    updateCampaign.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listCampaigns.mockResolvedValue({ items: [campaign], total: 1, page: 1, page_size: 50 })
    getCampaign.mockResolvedValue(campaign)
    getCampaignConfig.mockResolvedValue({ historical_invite_ratio: '0.3' })
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
    createCampaign.mockResolvedValue({
      campaign: { ...campaign, id: 10, name: '邀请好友赢奖金' },
      config_version: { id: 100 },
    })
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

  it('keeps finalize disabled before the campaign ends', async () => {
    const wrapper = mountView()
    await flushPromises()

    const finalizeButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.finalize'))
    expect(finalizeButton).toBeTruthy()
    expect(finalizeButton!.attributes('disabled')).toBeDefined()
    expect(recalculateRewards).not.toHaveBeenCalled()
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
    expect(dialog.findAll('[data-testid="rank-weight-row"]')).toHaveLength(10)

    const numberInputs = dialog.findAll('input[type="number"]')
    await numberInputs[4].setValue('0.9')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.campaignRewards.createPoolRatioInvalid')

    await numberInputs[4].setValue('0.2')
    await dialog.findAll('[data-testid="rank-weight-input"]')[9].setValue('1')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.campaignRewards.createWeightsSumInvalid')
    expect(createCampaign).not.toHaveBeenCalled()
  })

  it('shows an invalid weight hint instead of a NaN total when a rank weight is empty', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    await dialog.findAll('[data-testid="rank-weight-input"]')[0].setValue('')
    await flushPromises()

    expect(dialog.get('[data-testid="rank-weight-total"]').text()).toContain('admin.campaignRewards.createWeightsInvalid')
    expect(dialog.get('[data-testid="rank-weight-total"]').text()).not.toContain('NaN')
    expect(wrapper.text()).toContain('admin.campaignRewards.createWeightsInvalid')
    expect(createCampaign).not.toHaveBeenCalled()
  })

  it('keeps the rank weight total in warning state when invalid weights still sum to 100', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    const weightInputs = dialog.findAll('[data-testid="rank-weight-input"]')
    await weightInputs[0].setValue('30.5')
    await weightInputs[1].setValue('19.5')
    await flushPromises()

    const total = dialog.get('[data-testid="rank-weight-total"]')
    expect(total.text()).toContain('admin.campaignRewards.createWeightsInvalid')
    expect(total.classes()).toContain('text-amber-600')
    expect(total.classes()).not.toContain('text-emerald-600')
    expect(createCampaign).not.toHaveBeenCalled()
  })

  it('submits rank reward count and weights from the editable table after deleting a rank', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    await dialog.findAll('[data-testid="remove-rank-weight-row"]')[9].trigger('click')
    await dialog.findAll('[data-testid="rank-weight-input"]')[0].setValue('32')
    await flushPromises()

    expect(dialog.findAll('[data-testid="rank-weight-row"]')).toHaveLength(9)
    expect(dialog.text()).toContain('admin.campaignRewards.rankWeightRankValue')

    await dialog.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    expect(createCampaign).toHaveBeenCalledWith(expect.objectContaining({
      historical_invite_ratio: 0,
      rank_reward_count: 9,
      rank_weights: [32, 20, 15, 10, 8, 6, 4, 3, 2],
    }))
  })

  it('adds a rank weight row and validates it as part of the total', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="base-dialog"]')
    await dialog.get('[data-testid="add-rank-weight-row"]').trigger('click')
    await flushPromises()

    expect(dialog.findAll('[data-testid="rank-weight-row"]')).toHaveLength(11)
    expect(wrapper.text()).toContain('admin.campaignRewards.createWeightsSumInvalid')

    await dialog.findAll('[data-testid="rank-weight-input"]')[0].setValue('29')
    await flushPromises()

    await dialog.findAll('button').find(button => button.text().includes('admin.campaignRewards.createCampaign'))?.trigger('click')
    await flushPromises()

    expect(createCampaign).toHaveBeenCalledWith(expect.objectContaining({
      rank_reward_count: 11,
      rank_weights: [29, 20, 15, 10, 8, 6, 4, 3, 2, 2, 1],
    }))
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

  it('requires confirmation before pausing an active campaign', async () => {
    const activeCampaign = { ...campaign, status: 'active' }
    listCampaigns.mockResolvedValue({ items: [activeCampaign], total: 1, page: 1, page_size: 50 })
    getCampaign.mockResolvedValue(activeCampaign)
    pauseCampaign.mockResolvedValue({ ...activeCampaign, status: 'paused' })

    const wrapper = mountView()
    await flushPromises()

    const pauseButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.pause'))
    expect(pauseButton).toBeTruthy()
    expect(pauseButton!.attributes('disabled')).toBeUndefined()

    await pauseButton!.trigger('click')
    await flushPromises()

    expect(pauseCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmPauseTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(pauseCampaign).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.paused')
  })

  it('shows unfreeze only for frozen campaigns and confirms before calling the API', async () => {
    const frozenCampaign = { ...campaign, status: 'frozen' }
    listCampaigns.mockResolvedValue({ items: [frozenCampaign], total: 1, page: 1, page_size: 50 })
    getCampaign.mockResolvedValue(frozenCampaign)
    unfreezeCampaign.mockResolvedValue({ ...frozenCampaign, status: 'active' })

    const wrapper = mountView()
    await flushPromises()

    const unfreezeButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.unfreeze'))
    expect(unfreezeButton).toBeTruthy()
    expect(wrapper.findAll('button').some(button => button.text().includes('admin.campaignRewards.resume'))).toBe(false)

    await unfreezeButton!.trigger('click')
    await flushPromises()

    expect(unfreezeCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmUnfreezeTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(unfreezeCampaign).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.unfrozen')
  })

  it('shows resume only for paused campaigns and confirms before calling the API', async () => {
    const pausedCampaign = { ...campaign, status: 'paused' }
    listCampaigns.mockResolvedValue({ items: [pausedCampaign], total: 1, page: 1, page_size: 50 })
    getCampaign.mockResolvedValue(pausedCampaign)
    resumeCampaign.mockResolvedValue({ ...pausedCampaign, status: 'active' })

    const wrapper = mountView()
    await flushPromises()

    const resumeButton = wrapper.findAll('button').find(button => button.text().includes('admin.campaignRewards.resume'))
    expect(resumeButton).toBeTruthy()
    expect(wrapper.findAll('button').some(button => button.text().includes('admin.campaignRewards.pause'))).toBe(false)

    await resumeButton!.trigger('click')
    await flushPromises()

    expect(resumeCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.campaignRewards.confirmResumeTitle')

    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await flushPromises()

    expect(resumeCampaign).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('admin.campaignRewards.resumed')
  })

  it('updates campaign details and lifecycle times from the edit dialog', async () => {
    const updated = {
      ...campaign,
      name: '生产邀请活动',
      historical_invite_ratio: 0.3,
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
    await dialog.find('input[type="number"]').setValue('40')
    const timeInputs = dialog.findAll('input[type="datetime-local"]')
    await timeInputs[2].setValue('2026-07-09T12:30')
    await timeInputs[3].setValue('2026-07-10T09:00')
    await timeInputs[4].setValue('')

    await dialog.findAll('button').find(button => button.text().includes('admin.campaignRewards.saveCampaign'))?.trigger('click')
    await flushPromises()

    expect(updateCampaign).toHaveBeenCalledWith(9, expect.objectContaining({
      name: '生产邀请活动',
      historical_invite_ratio: 0.4,
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
