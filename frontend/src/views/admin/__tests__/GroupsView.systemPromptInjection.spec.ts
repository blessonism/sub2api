import { defineComponent } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminGroup, GroupPlatform } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'

const { list, create, update, authState } = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), authState: { isSimpleMode: false }
}))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list, create, update,
      getAll: vi.fn(async () => []),
      getModelAllowlistCandidates: vi.fn(async () => []),
      getUsageSummary: vi.fn(async () => []),
      getCapacitySummary: vi.fn(async () => []),
      getLiveCapability: vi.fn(async () => ({ supported: false }))
    },
    accounts: { list: vi.fn(), getById: vi.fn() }
  }
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() })
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)

const sourceGroup = {
  time_rate_priority: "schedule_first",
  time_rate_periods: [],
  id: 42,
  name: "OpenAI",
  description: null,
  platform: "openai",
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: "active",
  subscription_type: "standard",
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: true,
  force_openai_fast: false,
  free_openai_fast: false,
  model_pricing: [],
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: "",
  peak_end: "",
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  allow_live: false,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: "2026-09-05T00:00:00Z",
  updated_at: "2026-09-05T00:00:00Z",
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: undefined,
  codex_models_manifest_config: {
    enabled: false,
    account_ids: [],
    fallback_to_scheduler: false,
  },
  sort_order: 10,
} satisfies AdminGroup;


const SelectStub = defineComponent({
  props: ['modelValue', 'options', 'disabled'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
})
const mountView = () => mount(GroupsView, {
  global: {
    stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
      DataTable: {
        props: ['data'],
        template: '<div><slot v-for="row in data" name="cell-actions" :row="row" /></div>'
      },
      BaseDialog: {
        props: ['show'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>'
      },
      Select: SelectStub,
      ReasoningEffortPolicyFields: defineComponent({
        setup(_, { expose }) {
          expose({ validate: () => true, resetValidation: () => undefined })
          return () => null
        }
      }),
      CodexManifestAccountsField: true,
      GroupRateMultipliersModal: true,
      GroupTimeRateStrategyModal: true,
      GroupRPMOverridesModal: true,
      GroupCapacityBadge: true,
      Pagination: true, ConfirmDialog: true, Icon: true,
      PlatformIcon: true, EmptyState: true, PricingEntryCard: true, VueDraggable: true
    }
  }
})

async function openCreate(wrapper: ReturnType<typeof mountView>) {
  await flushPromises()
  await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
  await wrapper.get('[data-tour="group-form-name"]').setValue('Custom group')
}
async function openEdit(wrapper: ReturnType<typeof mountView>) {
  await flushPromises()
  const edit = wrapper.findAll('button').find(button => button.text() === 'common.edit')!
  await edit.trigger('click')
  await flushPromises()
}

describe('GroupsView system prompt injection', () => {
  beforeEach(() => {
    localStorage.clear()
    authState.isSimpleMode = false
    vi.clearAllMocks()
    list.mockResolvedValue({ items: [sourceGroup], total: 1, page: 1, page_size: 20, pages: 1 })
    create.mockResolvedValue(sourceGroup)
    update.mockResolvedValue(sourceGroup)
  })

  it.each(['prepend', 'append', 'replace'] as const)('creates with %s and rejects empty enabled content', async mode => {
    const wrapper = mountView()
    await openCreate(wrapper)
    expect(wrapper.get('[data-testid="system-prompt-toggle"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    await wrapper.get('#create-group-form').trigger('submit')
    expect(create).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="system-prompt-error"]').exists()).toBe(true)
    await wrapper.get('[data-testid="system-prompt-content"]').setValue('  custom instructions\n')
    await wrapper.get('[data-testid="system-prompt-mode"]').setValue(mode)
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({
      system_prompt_injection: { enabled: true, mode, content: '  custom instructions\n' }
    }))
  })

  it('loads a saved config into an independent draft and saves disabled content', async () => {
    const savedGroup: AdminGroup = {
      ...sourceGroup,
      system_prompt_injection: { enabled: true, mode: 'replace', content: 'saved instructions' }
    }
    list.mockResolvedValue({ items: [savedGroup], total: 1 })
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.get<HTMLTextAreaElement>('[data-testid="system-prompt-content"]').element.value).toBe('saved instructions')
    expect(wrapper.get<HTMLSelectElement>('[data-testid="system-prompt-mode"]').element.value).toBe('replace')
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    expect(savedGroup.system_prompt_injection?.enabled).toBe(true)
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(42, expect.objectContaining({
      system_prompt_injection: { enabled: false, mode: 'replace', content: 'saved instructions' }
    }))
    await openEdit(wrapper)
    expect(wrapper.get('[data-testid="system-prompt-toggle"]').attributes('aria-checked')).toBe('true')
  })

  it.each(['openai', 'anthropic'] as const)('submits edited %s config and validates whitespace', async platform => {
    list.mockResolvedValue({ items: [{ ...sourceGroup, platform }], total: 1 })
    const wrapper = mountView()
    await openEdit(wrapper)
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    await wrapper.get('[data-testid="system-prompt-content"]').setValue(' \n ')
    await wrapper.get('#edit-group-form').trigger('submit')
    expect(update).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="system-prompt-content"]').setValue('edited instructions')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(42, expect.objectContaining({
      system_prompt_injection: { enabled: true, mode: 'append', content: 'edited instructions' }
    }))
  })

  it.each(['composite', 'gemini', 'antigravity', 'grok'] satisfies GroupPlatform[])('disables injection on %s and preserves the draft on platform switches', async platform => {
    const wrapper = mountView()
    await openCreate(wrapper)
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    await wrapper.get('[data-testid="system-prompt-content"]').setValue('retained')
    await wrapper.get('[data-tour="group-form-platform"]').setValue(platform)
    expect(wrapper.find('[data-testid="system-prompt-toggle"]').exists()).toBe(false)
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    expect(wrapper.get('[data-testid="system-prompt-toggle"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    expect(wrapper.get<HTMLTextAreaElement>('[data-testid="system-prompt-content"]').element.value).toBe('retained')
    await wrapper.get('[data-tour="group-form-platform"]').setValue(platform)
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({
      system_prompt_injection: { enabled: false, mode: 'append', content: 'retained' }
    }))
  })

  it('saves defaults for a legacy group without configuration', async () => {
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.get('[data-testid="system-prompt-toggle"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(42, expect.objectContaining({
      system_prompt_injection: { enabled: false, mode: 'append', content: '' }
    }))
  })

  it('disables a stale enabled config for an unsupported group while keeping the content', async () => {
    list.mockResolvedValue({ items: [{
      ...sourceGroup,
      platform: 'composite',
      system_prompt_injection: { enabled: true, mode: 'prepend', content: 'retained' }
    }], total: 1 })
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.find('[data-testid="system-prompt-toggle"]').exists()).toBe(false)
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(42, expect.objectContaining({
      system_prompt_injection: { enabled: false, mode: 'prepend', content: 'retained' }
    }))
  })

  it.each(['create', 'edit'] as const)('includes injection in simple mode %s payloads', async action => {
    authState.isSimpleMode = true
    const wrapper = mountView()
    if (action === 'create') await openCreate(wrapper)
    else await openEdit(wrapper)
    await wrapper.get('[data-testid="system-prompt-toggle"]').trigger('click')
    await wrapper.get('[data-testid="system-prompt-content"]').setValue('simple mode instructions')
    await wrapper.get(`#${action}-group-form`).trigger('submit')
    await flushPromises()
    const payload = action === 'create' ? create.mock.calls[0][0] : update.mock.calls[0][1]
    expect(payload.system_prompt_injection).toEqual({ enabled: true, mode: 'append', content: 'simple mode instructions' })
  })
})
