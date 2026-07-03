import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import ConversationHistoryView from '../ConversationHistoryView.vue'
import type {
  ConversationCaptureConfig,
  ConversationExportRequest,
  ConversationSession,
  ConversationSessionFilters,
} from '@/api/admin/conversations'

const {
  getConfig,
  updateConfig,
  listSessions,
  listSessionTurns,
  getTurn,
  listExportJobs,
  exportMessagesJSONL,
} = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  listSessions: vi.fn(),
  listSessionTurns: vi.fn(),
  getTurn: vi.fn(),
  listExportJobs: vi.fn(),
  exportMessagesJSONL: vi.fn(),
}))

const { routerPush } = vi.hoisted(() => ({
  routerPush: vi.fn(),
}))

vi.mock('@/api/admin/conversations', () => ({
  default: {
    getConfig,
    updateConfig,
    listSessions,
    getSession: vi.fn(),
    listSessionTurns,
    getTurn,
    setSessionExportable: vi.fn(),
    setTurnExportable: vi.fn(),
    setSessionQuality: vi.fn(),
    setTurnQuality: vi.fn(),
    bulkSetQuality: vi.fn(),
    mergeSessions: vi.fn(),
    splitSession: vi.fn(),
    moveTurn: vi.fn(),
    exportMessagesJSONL,
    createExportJob: vi.fn(),
    listExportJobs,
    getExportJob: vi.fn(),
    createExportDownloadTicket: vi.fn(),
    deleteExportJob: vi.fn(),
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        return key.replace(/\{(\w+)\}/g, (_, token) => String(params[token] ?? `{${token}}`))
      },
    }),
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: routerPush,
  }),
}))

const baseConfig = (overrides: Partial<ConversationCaptureConfig> = {}): ConversationCaptureConfig => ({
  enabled: false,
  sample_percent: 100,
  capture_chat_completions: true,
  capture_responses: false,
  raw_archive_enabled: false,
  max_turn_payload_bytes: 1048576,
  payload_preview_chars: 8000,
  session_window_minutes: 30,
  retention_days: 30,
  export_enabled: true,
  subject_filter_mode: 'blacklist',
  excluded_user_ids: [],
  excluded_api_key_ids: [],
  included_user_ids: [],
  included_api_key_ids: [],
  ...overrides,
})

const baseSession = (overrides: Partial<ConversationSession> = {}): ConversationSession => ({
  id: 1,
  session_id: 'session_1234567890',
  user_id: 7,
  api_key_id: 9,
  account_id: null,
  provider: 'openai',
  model: 'gpt-5',
  request_path: '/v1/chat/completions',
  status: 'completed',
  turn_count: 1,
  source_request_count: 1,
  input_tokens: 12,
  output_tokens: 34,
  total_tokens: 46,
  actual_cost: 0.001,
  quality_status: 'unchecked',
  quality_errors: [],
  exportable: true,
  duplicate_turn_count: 0,
  capture_status: 'captured',
  session_source: 'explicit',
  retention_until: '2026-07-01T00:00:00Z',
  started_at: '2026-06-27T00:00:00Z',
  ended_at: '2026-06-27T00:00:10Z',
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:10Z',
  ...overrides,
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const EmptyStateStub = defineComponent({
  props: {
    title: {
      type: String,
      default: '',
    },
    description: {
      type: String,
      default: '',
    },
  },
  template: '<div data-test="empty-state">{{ title }} {{ description }}</div>',
})
const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('button', {
      type: 'button',
      'data-test': props.modelValue ? 'toggle-on' : 'toggle-off',
      disabled: props.disabled,
      onClick: () => emit('update:modelValue', !props.modelValue),
    }, String(props.modelValue))
  },
})

async function setDataBrowserFilters(wrapper: ReturnType<typeof mountView>, values: {
  userId?: string
  apiKeyId?: string
  model?: string
  requestId?: string
  qualityStatus?: string
  exportable?: string
}) {
  if (values.userId !== undefined) await wrapper.find('[data-test="conversation-filter-user-id"]').setValue(values.userId)
  if (values.apiKeyId !== undefined) await wrapper.find('[data-test="conversation-filter-api-key-id"]').setValue(values.apiKeyId)
  if (values.model !== undefined) await wrapper.find('[data-test="conversation-filter-model"]').setValue(values.model)
  if (values.requestId !== undefined) await wrapper.find('[data-test="conversation-filter-request-id"]').setValue(values.requestId)
  if (values.qualityStatus !== undefined) await wrapper.find('[data-test="conversation-filter-quality-status"]').setValue(values.qualityStatus)
  if (values.exportable !== undefined) await wrapper.find('[data-test="conversation-filter-exportable"]').setValue(values.exportable)
}

function mountView() {
  return mount(ConversationHistoryView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        EmptyState: EmptyStateStub,
        Icon: true,
        LoadingSpinner: { template: '<div data-test="loading"></div>' },
        Pagination: true,
        Toggle: ToggleStub,
        ConfirmDialog: true,
      },
    },
  })
}

describe('ConversationHistoryView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getConfig.mockReset()
    updateConfig.mockReset()
    listSessions.mockReset()
    listSessionTurns.mockReset()
    getTurn.mockReset()
    listExportJobs.mockReset()
    exportMessagesJSONL.mockReset()
    routerPush.mockReset()

    getConfig.mockResolvedValue(baseConfig())
    updateConfig.mockImplementation(async (payload: ConversationCaptureConfig) => payload)
    listSessions.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    listSessionTurns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50, pages: 0 })
    getTurn.mockResolvedValue({
      id: 11,
      session_id: 'session_1234567890',
      request_id: 'req-1',
      turn_index: 1,
      provider: 'openai',
      model: 'gpt-5',
      request_path: '/v1/chat/completions',
      request_messages: [{ role: 'user', content: 'hello' }],
      response_messages: [{ role: 'assistant', content: 'hi' }],
      tools: [],
      usage: {},
      meta: {},
      input_tokens: 1,
      output_tokens: 1,
      total_tokens: 2,
      actual_cost: 0.001,
      stream: false,
      client_disconnect: false,
      truncated: false,
      quality_status: 'unchecked',
      quality_errors: [],
      exportable: true,
      parse_status: 'success',
      dedupe_hash: 'hash-1',
      retention_until: '2026-07-01T00:00:00Z',
      created_at: '2026-06-27T00:00:00Z',
    })
    listExportJobs.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    exportMessagesJSONL.mockResolvedValue(new Blob(['ok'], { type: 'application/x-ndjson' }))
    URL.createObjectURL = vi.fn(() => 'blob:conversation-export')
    URL.revokeObjectURL = vi.fn()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
  })

  it('auto-saves the capture toggle and shows the enabled empty state', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="toggle-off"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({ enabled: true }))
    expect(wrapper.text()).toContain('admin.conversations.captureOn')
    expect(wrapper.text()).toContain('admin.conversations.emptyEnabledTitle')
  })

  it('reloads the persisted config when a toggle save fails', async () => {
    getConfig
      .mockResolvedValueOnce(baseConfig({ enabled: false }))
      .mockResolvedValueOnce(baseConfig({ enabled: false }))
    updateConfig.mockRejectedValueOnce(new Error('save failed'))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="toggle-off"]').trigger('click').catch(() => undefined)
    await flushPromises()

    expect(getConfig).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.conversations.captureOff')
    expect(wrapper.text()).toContain('admin.conversations.emptyDisabledTitle')
  })

  it('saves whitelist mode with pasted and deduped included subject IDs', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="conversation-mode-whitelist"]').trigger('click')
    await wrapper.find('[data-test="conversation-id-input-included_user_ids"]').trigger('paste', {
      clipboardData: { getData: () => '7, 7 9\n10' },
    })
    await nextTick()
    await wrapper.find('[data-test="conversation-id-input-included_api_key_ids"]').trigger('paste', {
      clipboardData: { getData: () => '11 11' },
    })
    await nextTick()
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      subject_filter_mode: 'whitelist',
      included_user_ids: [7, 9, 10],
      included_api_key_ids: [11],
      excluded_user_ids: [],
      excluded_api_key_ids: [],
    }))
  })

  it('removes a loaded whitelist subject ID chip before saving', async () => {
    getConfig.mockResolvedValueOnce(baseConfig({
      subject_filter_mode: 'whitelist',
      included_user_ids: [7, 9],
      included_api_key_ids: [11],
    }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="conversation-id-chip-included_user_ids-9"] button').trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      subject_filter_mode: 'whitelist',
      included_user_ids: [7],
      included_api_key_ids: [11],
    }))
  })

  it('warns when whitelist mode has no included subject IDs', async () => {
    getConfig.mockResolvedValueOnce(baseConfig({ subject_filter_mode: 'whitelist' }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="conversation-empty-whitelist-warning"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.conversations.emptyWhitelistWarning')
  })

  it('passes data browser filters to session loading', async () => {
    const wrapper = mountView()
    await flushPromises()

    await setDataBrowserFilters(wrapper, {
      userId: '7',
      apiKeyId: '9',
      model: 'gpt-5',
      requestId: 'req-1',
      qualityStatus: 'clean',
      exportable: 'true',
    })
    await wrapper.findAll('button').find((button) => button.text() === 'common.search')!.trigger('click')
    await flushPromises()

    expect(listSessions).toHaveBeenLastCalledWith(expect.objectContaining<ConversationSessionFilters>({
      user_id: 7,
      api_key_id: 9,
      model: 'gpt-5',
      request_id: 'req-1',
      quality_status: 'clean',
      exportable: true,
      page: 1,
      page_size: 20,
    }))
  })

  it('exports JSONL with the current filters and export options', async () => {
    listSessions.mockResolvedValue({ items: [baseSession()], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await flushPromises()

    await setDataBrowserFilters(wrapper, {
      userId: '7',
      model: 'gpt-5',
    })
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true)
    const exportButton = wrapper.findAll('button').find((button) => button.text().includes('admin.conversations.exportCurrentFilters'))
    await exportButton!.trigger('click')
    await flushPromises()

    expect(exportMessagesJSONL).toHaveBeenCalledWith(expect.objectContaining<ConversationExportRequest>({
      user_id: 7,
      model: 'gpt-5',
      include_duplicates: true,
      dedupe: false,
      redaction_enabled: true,
      limit: 200,
    }))
  })

  it('navigates from the session list to the conversation detail page', async () => {
    listSessions.mockResolvedValue({ items: [baseSession()], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('tbody tr').trigger('click')
    await flushPromises()

    expect(routerPush).toHaveBeenCalledWith('/admin/conversations/1')
    expect(listSessionTurns).not.toHaveBeenCalled()
    expect(getTurn).not.toHaveBeenCalled()
  })

  it('shows quality error reasons for sessions', async () => {
    listSessions.mockResolvedValue({
      items: [baseSession({
        quality_status: 'needs_review',
        quality_errors: [{ code: 'heuristic_session', message: 'session was inferred heuristically', source: 'auto' }],
      })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('session was inferred heuristically')
  })
})
