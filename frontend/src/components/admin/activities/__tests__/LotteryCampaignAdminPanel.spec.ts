import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LotteryCampaignAdminPanel from '../LotteryCampaignAdminPanel.vue'

const messages: Record<string, string> = {
  'admin.lotteryCampaigns.rawTokenEquivalent': 'Equals {tokens} tokens',
  'admin.lotteryCampaigns.dailyOnceRulePreview': 'Users get 1 entry after daily usage reaches {threshold}.',
  'admin.lotteryCampaigns.steppedRulePreview': 'Users get 1 entry at {threshold}, then +1 entry per extra {step}, up to {max}.',
  'admin.lotteryCampaigns.oneTimeOnceRulePreview': 'Users get 1 entry after campaign usage reaches {threshold}.',
  'admin.lotteryCampaigns.oneTimeSteppedRulePreview': 'Users get 1 entry at {threshold} during the campaign, then +1 entry per extra {step}, up to {max}.',
}

const {
  listLotteryCampaigns,
  createLotteryCampaign,
  updateLotteryCampaign,
  publishLotteryCampaign,
  cancelLotteryCampaign,
  featureLotteryCampaign,
  deleteLotteryCampaign,
  syncLotteryEntries,
  drawLotteryCampaign,
  listLotteryWinners,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listLotteryCampaigns: vi.fn(),
  createLotteryCampaign: vi.fn(),
  updateLotteryCampaign: vi.fn(),
  publishLotteryCampaign: vi.fn(),
  cancelLotteryCampaign: vi.fn(),
  featureLotteryCampaign: vi.fn(),
  deleteLotteryCampaign: vi.fn(),
  syncLotteryEntries: vi.fn(),
  drawLotteryCampaign: vi.fn(),
  listLotteryWinners: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    lotteryCampaigns: {
      listLotteryCampaigns,
      createLotteryCampaign,
      updateLotteryCampaign,
      publishLotteryCampaign,
      cancelLotteryCampaign,
      featureLotteryCampaign,
      deleteLotteryCampaign,
      syncLotteryEntries,
      drawLotteryCampaign,
      listLotteryWinners,
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

function mountPanel() {
  return mount(LotteryCampaignAdminPanel, {
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /></div>',
        },
        EmptyState: true,
        Icon: true,
        LoadingSpinner: true,
      },
    },
  })
}

describe('LotteryCampaignAdminPanel', () => {
  beforeEach(() => {
    listLotteryCampaigns.mockReset()
    createLotteryCampaign.mockReset()
    updateLotteryCampaign.mockReset()
    publishLotteryCampaign.mockReset()
    cancelLotteryCampaign.mockReset()
    featureLotteryCampaign.mockReset()
    deleteLotteryCampaign.mockReset()
    syncLotteryEntries.mockReset()
    drawLotteryCampaign.mockReset()
    listLotteryWinners.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    listLotteryCampaigns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    listLotteryWinners.mockResolvedValue({ items: [] })
  })

  it('converts million-unit token inputs to raw token payload values', async () => {
    createLotteryCampaign.mockResolvedValue({ id: 7 })

    const wrapper = mountPanel()
    await flushPromises()

    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      submitSave: () => Promise<void>
      form: {
        threshold_token_millions: number
        draw_schedule_type: 'single' | 'daily'
        entry_mode: 'daily_once' | 'stepped'
        entry_step_token_millions: number
        max_entries_per_user: number
      }
    }

    vm.openCreate()
    await nextTick()
    expect(wrapper.text()).toContain('admin.lotteryCampaigns.thresholdTokenMillions')
    expect(wrapper.text()).toContain('Equals 100,000 tokens')
    expect(wrapper.text()).toContain('Users get 1 entry after campaign usage reaches 0.10M.')
    expect(wrapper.text()).not.toContain('admin.lotteryCampaigns.entryStepTokenMillions')

    vm.form.threshold_token_millions = 1.25
    vm.form.entry_mode = 'stepped'
    vm.form.entry_step_token_millions = 0.5
    vm.form.max_entries_per_user = 3
    await nextTick()
    expect(wrapper.text()).toContain('admin.lotteryCampaigns.entryStepTokenMillions')
    expect(wrapper.text()).toContain('Equals 500,000 tokens')
    expect(wrapper.text()).toContain('Users get 1 entry at 1.25M during the campaign, then +1 entry per extra 0.50M, up to 3.')

    await vm.submitSave()

    expect(createLotteryCampaign).toHaveBeenCalledWith(expect.objectContaining({
      threshold_tokens: 1_250_000,
      entry_step_tokens: 500_000,
      max_entries_per_user: 3,
    }))

    createLotteryCampaign.mockClear()
    vm.form.threshold_token_millions = 0.005
    vm.form.entry_mode = 'daily_once'
    vm.form.entry_step_token_millions = 0.5
    vm.form.max_entries_per_user = 3

    await vm.submitSave()

    expect(createLotteryCampaign).toHaveBeenCalledWith(expect.objectContaining({
      threshold_tokens: 5_000,
      entry_step_tokens: 0,
      max_entries_per_user: 1,
    }))
  })

  it('backfills existing raw token values as million-unit form values', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const vm = wrapper.vm as unknown as {
      openEdit: (campaign: Record<string, unknown>) => void
      form: {
        threshold_token_millions: number
        entry_step_token_millions: number
      }
    }

    vm.openEdit({
      id: 9,
      name: 'Token Lottery',
      description: '',
      rules_text: '',
      status: 'draft',
      participation_mode: 'auto',
      draw_schedule_type: 'single',
      prize_mode: 'single',
      entry_mode: 'stepped',
      threshold_tokens: 1_250_000,
      entry_step_tokens: 500_000,
      max_entries_per_user: 3,
      start_at: '2026-07-08T00:00:00.000Z',
      end_at: '2026-07-09T00:00:00.000Z',
      draw_at: '2026-07-08T20:00:00.000Z',
      daily_draw_time: '',
      prize_tiers: [{ tier_name: 'Prize', winner_count: 1, reward_amount_cents: 100, sort_order: 1 }],
      created_at: '2026-07-08T00:00:00.000Z',
      updated_at: '2026-07-08T00:00:00.000Z',
      is_featured: false,
    })

    expect(vm.form.threshold_token_millions).toBe(1.25)
    expect(vm.form.entry_step_token_millions).toBe(0.5)
  })

  it('allows an ended published campaign to become persistent and shows its winners', async () => {
    listLotteryCampaigns.mockResolvedValue({
      items: [{
        id: 7,
        name: 'Ended Lottery',
        description: '',
        rules_text: '',
        status: 'published',
        participation_mode: 'auto',
        draw_schedule_type: 'single',
        prize_mode: 'single',
        entry_mode: 'daily_once',
        threshold_tokens: 100_000,
        entry_step_tokens: 0,
        max_entries_per_user: 1,
        start_at: '2026-07-01T00:00:00.000Z',
        end_at: '2026-07-02T00:00:00.000Z',
        draw_at: '2026-07-02T00:00:00.000Z',
        daily_draw_time: '',
        prize_tiers: [{ id: 1, campaign_id: 7, tier_name: 'Gold', winner_count: 1, reward_amount_cents: 1000, sort_order: 1, created_at: '2026-07-01T00:00:00.000Z' }],
        created_at: '2026-07-01T00:00:00.000Z',
        updated_at: '2026-07-02T00:00:00.000Z',
        is_featured: false,
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listLotteryWinners.mockResolvedValue({
      items: [{
        id: 11,
        batch_id: 9,
        campaign_id: 7,
        user_id: 42,
        prize_tier_id: 1,
        prize_name: 'Gold',
        entry_date: '2026-07-02',
        reward_amount_cents: 1000,
        status: 'success',
        idempotency_key: 'hidden-in-admin-only-response',
        error_message: '',
        created_at: '2026-07-02T00:00:00.000Z',
        processed_at: '2026-07-02T00:01:00.000Z',
      }],
    })

    const wrapper = mountPanel()
    await flushPromises()

    const persistentButton = wrapper.findAll('button').find(button => button.text().includes('admin.lotteryCampaigns.setFeatured'))
    expect(persistentButton?.attributes('disabled')).toBeUndefined()
    expect(listLotteryWinners).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('#42')
    expect(wrapper.text()).toContain('Gold')
  })
})
