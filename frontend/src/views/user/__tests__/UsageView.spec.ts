import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageView from '../UsageView.vue'

const {
  query,
  getStatsByDateRange,
  list,
  adminGetUserView,
  adminGetUserViewStats,
  adminSearchApiKeys,
  adminSearchUsers,
  adminCreateCalibration,
  adminListCalibrations,
  adminUsersGetById,
  showError,
  showWarning,
  showSuccess,
  showInfo,
  authState,
} = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  list: vi.fn(),
  adminGetUserView: vi.fn(),
  adminGetUserViewStats: vi.fn(),
  adminSearchApiKeys: vi.fn(),
  adminSearchUsers: vi.fn(),
  adminCreateCalibration: vi.fn(),
  adminListCalibrations: vi.fn(),
  adminUsersGetById: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  authState: { isAdmin: false },
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.billed': 'Billed',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.adminCalibration': 'Calibrate',
  'usage.adminUserFilter': 'User',
  'usage.adminSearchUserPlaceholder': 'Search by email or user ID',
  'usage.adminSearchingUsers': 'Searching users...',
  'usage.adminUserSearchResults': 'Search results',
  'usage.adminClearSelectedUser': 'Exit view',
  'usage.adminDeletedUser': 'deleted',
  'usage.adminNoUsersFound': 'No matching users found',
  'usage.adminSelectUserFirst': 'Select a target user first',
  'usage.adminDeletedUserCannotCalibrate': 'Deleted users cannot be calibrated',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.latency': 'Latency',
  'usage.latencyFirstToken': 'First token',
  'usage.latencyDuration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
  'usage.imageUnit': ' images',
  'usage.imageCount': 'Image count',
  'usage.imageBillingSize': 'Billing size',
  'usage.imageInputSize': 'Input size',
  'usage.imageOutputSize': 'Output size',
  'usage.imageSizeSource': 'Size source',
  'usage.imageSizeBreakdown': 'Size breakdown',
  'usage.imageSizeSourceOutput': 'Upstream output',
  'usage.imageSizeSourceInput': 'Request input',
  'usage.imageSizeSourceDefault': 'Default billing tier',
  'usage.imageSizeSourceLegacy': 'Legacy record',
  'usage.imageSizeSourceMissing': 'Not recorded',
  'usage.imageSizeNotRecorded': 'not recorded',
  'usage.imageSizeLegacyUnstandardized': 'legacy unstandardized',
  'usage.imageSizeUnknown': 'unknown',
  'usage.imageUnitPrice': 'Per-image price',
  'usage.imageTotalPrice': 'Image total price',
  'admin.usage.billingModeToken': 'Token',
  'admin.usage.billingModePerRequest': 'Per request',
  'admin.usage.billingModeImage': 'Image',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    getUserView: adminGetUserView,
    getUserViewStats: adminGetUserViewStats,
    searchApiKeys: adminSearchApiKeys,
    searchUsers: adminSearchUsers,
    createCalibration: adminCreateCalibration,
    listCalibrations: adminListCalibrations,
  },
}))

vi.mock('@/api/admin/users', () => ({
  usersAPI: {
    getById: adminUsersGetById,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot /></div>',
}
const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.request_id">
        <slot name="cell-billing_mode" :row="row" />
        <slot name="cell-tokens" :row="row" />
        <slot name="cell-cost" :row="row" />
        <slot name="cell-latency" :row="row" />
      </div>
    </div>
  `,
}

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    authState.isAdmin = false
    query.mockReset()
    getStatsByDateRange.mockReset()
    list.mockReset()
    adminGetUserView.mockReset()
    adminGetUserViewStats.mockReset()
    adminSearchApiKeys.mockReset()
    adminSearchUsers.mockReset()
    adminCreateCalibration.mockReset()
    adminListCalibrations.mockReset()
    adminUsersGetById.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

    adminGetUserView.mockResolvedValue({ items: [], total: 0, pages: 0 })
    adminGetUserViewStats.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    adminSearchApiKeys.mockResolvedValue([])
    adminSearchUsers.mockResolvedValue([])
    adminListCalibrations.mockResolvedValue({ items: [], total: 0, pages: 0 })
    adminUsersGetById.mockResolvedValue({
      id: 7,
      email: 'user@example.com',
      role: 'user',
      balance: 0,
      concurrency: 0,
      status: 'active',
      allowed_groups: null,
      balance_notify_enabled: false,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '2026-06-01T00:00:00Z',
      updated_at: '2026-06-01T00:00:00Z',
      notes: '',
    })

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }
  })

  it('shows fast service tier and unit prices in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 0,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
  })

  it('renders the combined latency cell with compact durations and health styles', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-latency',
          actual_cost: 0.1,
          total_cost: 0.1,
          rate_multiplier: 1,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 1,
          output_tokens: 1,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          first_token_ms: 65_000,
          duration_ms: 125_000,
          created_at: '2026-07-10T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 2,
      total_cost: 0.1,
      avg_duration_ms: 125_000,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('First token')
    expect(wrapper.text()).toContain('1m 5s')
    expect(wrapper.text()).toContain('2m 5s')
    const latencyCell = wrapper.get('[data-testid="usage-latency-cell"]')
    expect(latencyCell.find('.from-red-500').exists()).toBe(true)
    expect(latencyCell.find('.to-amber-400').exists()).toBe(true)
    expect(latencyCell.find('.text-red-600').text()).toBe('1m 5s')
    expect(latencyCell.find('.text-amber-600').text()).toBe('2m 5s')
  })

  it('does not render a user-side csv export button', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('usage.exportCsv')
    expect((wrapper.vm as any).$?.setupState.exportToCSV).toBeUndefined()
  })

  it('does not display a 2K fallback for historical image rows with missing size', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-legacy-missing-image',
          actual_cost: 0.2,
          total_cost: 0.2,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 1,
          image_size: null,
          image_input_size: null,
          image_output_size: null,
          image_size_source: null,
          image_size_breakdown: null,
          billing_mode: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
          model: 'gpt-image-2',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 0,
      total_cost: 0.2,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Image')
    expect(text).toContain('not recorded')
    expect(text).not.toContain('(2K)')
  })

  it('shows image billing metadata in the user cost tooltip', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-output-image',
      actual_cost: 0.8,
      total_cost: 0.8,
      rate_multiplier: 1,
      service_tier: null,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      billing_mode: null,
      image_count: 2,
      image_size: '4K',
      image_input_size: '1024x1024',
      image_output_size: '3840x2160',
      image_size_source: 'output',
      image_size_breakdown: { '4K': 2 },
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Image count')
    expect(text).toContain('Billing size')
    expect(text).toContain('4K')
    expect(text).toContain('Size source')
    expect(text).toContain('Upstream output')
    expect(text).toContain('Input size')
    expect(text).toContain('1024x1024')
    expect(text).toContain('Output size')
    expect(text).toContain('3840x2160')
    expect(text).toContain('4K x 2')
  })

  it('uses browser timezone when loading admin token calibration preview', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    list.mockResolvedValue({ items: [] })
    adminGetUserViewStats.mockResolvedValue({
      total_requests: 1,
      total_tokens: 1234,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    const dateTimeFormatSpy = vi.spyOn(Intl, 'DateTimeFormat').mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Asia/Tokyo' }),
    }) as Intl.DateTimeFormat)

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          BaseDialog: true,
          UserErrorRequestsTable: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 7
    setupState.selectedAdminUser = { id: 7, email: 'user@example.com', deleted: false }
    setupState.calibrationDialogVisible = true
    setupState.calibrationForm.tokenEnabled = true
    setupState.calibrationForm.tokenStartDate = '2026-06-01'
    setupState.calibrationForm.tokenEndDate = '2026-06-07'

    await setupState.loadCalibrationTokenCurrentTotal()

    expect(adminGetUserViewStats).toHaveBeenLastCalledWith({
      user_id: 7,
      start_date: '2026-06-01',
      end_date: '2026-06-07',
      timezone: 'Asia/Tokyo',
    })
    dateTimeFormatSpy.mockRestore()
  })

  it('submits range consumption target with timezone', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({ total_requests: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0 })
    list.mockResolvedValue({ items: [] })
    adminCreateCalibration.mockResolvedValue({})
    const dateTimeFormatSpy = vi.spyOn(Intl, 'DateTimeFormat').mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Asia/Tokyo' }),
    }) as Intl.DateTimeFormat)

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          BaseDialog: true,
          UserErrorRequestsTable: true,
          Icon: true,
          Teleport: true,
        },
      },
    })
    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 7
    setupState.selectedAdminUser = { id: 7, email: 'user@example.com', deleted: false }
    setupState.selectedAdminUserDetail = { id: 7, balance: 20 }
    setupState.calibrationForm.tokenEnabled = false
    setupState.calibrationForm.balanceEnabled = false
    setupState.calibrationForm.consumptionEnabled = true
    setupState.calibrationForm.consumptionMode = 'target'
    setupState.calibrationForm.consumptionValue = '14'
    setupState.calibrationForm.tokenStartDate = '2026-06-01'
    setupState.calibrationForm.tokenEndDate = '2026-06-02'
    setupState.calibrationConsumptionCurrentTotal = 10

    await setupState.submitCalibration()

    expect(adminCreateCalibration).toHaveBeenCalledWith({
      target_user_id: 7,
      consumption: {
        mode: 'target',
        value: 14,
        start_date: '2026-06-01',
        end_date: '2026-06-02',
        timezone: 'Asia/Tokyo',
      },
    }, expect.any(String))
    dateTimeFormatSpy.mockRestore()
    wrapper.unmount()
  })

  it('searches admin users only after clicking search and selects from real results', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    list.mockResolvedValue({ items: [] })
    adminSearchUsers.mockResolvedValue([
      { id: 42, email: 'target@example.com', deleted: false },
    ])
    adminUsersGetById.mockResolvedValue({
      id: 42,
      email: 'target@example.com',
      role: 'user',
      balance: 1,
      concurrency: 0,
      status: 'active',
      allowed_groups: null,
      balance_notify_enabled: false,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '2026-06-01T00:00:00Z',
      updated_at: '2026-06-01T00:00:00Z',
      notes: '',
    })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          BaseDialog: true,
          UserErrorRequestsTable: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const input = wrapper.get('[data-testid="admin-user-search-input"]')
    await input.setValue('42')
    await nextTick()

    expect(adminSearchUsers).not.toHaveBeenCalled()
    expect(adminUsersGetById).not.toHaveBeenCalled()
    expect(adminGetUserView).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="admin-user-search-form"]').trigger('submit')
    await flushPromises()

    expect(adminSearchUsers).toHaveBeenCalledWith('42')
    expect(adminUsersGetById).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('target@example.com')

    await wrapper.get('[data-testid="admin-user-result-42"]').trigger('click')
    await flushPromises()

    expect(adminUsersGetById).toHaveBeenCalledWith(42, true)
    expect(adminSearchApiKeys).toHaveBeenCalledWith(42)
    expect(adminGetUserView).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 42 }),
      expect.any(Object),
    )
    expect(wrapper.text()).toContain('target@example.com')
    expect(wrapper.text()).toContain('#42')
  })

  it('keeps admin calibration dialog open while typing into form fields', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          UserErrorRequestsTable: true,
          Icon: true,
          Teleport: true,
        },
      },
      attachTo: document.body,
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 7
    setupState.selectedAdminUser = { id: 7, email: 'user@example.com', deleted: false }
    setupState.calibrationDialogVisible = true
    await nextTick()

    const numberInputs = Array.from(document.body.querySelectorAll<HTMLInputElement>('input[type="number"]'))
    expect(numberInputs.length).toBeGreaterThanOrEqual(2)

    numberInputs[0].value = '1000'
    numberInputs[0].dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(setupState.calibrationDialogVisible).toBe(true)

    setupState.calibrationForm.balanceEnabled = true
    await nextTick()
    numberInputs[1].value = '1.5'
    numberInputs[1].dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(setupState.calibrationDialogVisible).toBe(true)

    wrapper.unmount()
  })

  it('blocks admin calibration for deleted users', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          BaseDialog: true,
          UserErrorRequestsTable: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 9
    setupState.selectedAdminUser = { id: 9, email: 'deleted@example.com', deleted: true }
    await nextTick()

    const canCalibrate = setupState.canCalibrateSelectedAdminUser?.value ?? setupState.canCalibrateSelectedAdminUser
    expect(canCalibrate).toBe(false)

    await setupState.openCalibrationDialog()

    expect(showWarning).toHaveBeenCalledWith('Deleted users cannot be calibrated')
    expect(setupState.calibrationDialogVisible).toBe(false)
    expect(adminUsersGetById).not.toHaveBeenCalled()
    expect(adminListCalibrations).not.toHaveBeenCalled()
  })
})
