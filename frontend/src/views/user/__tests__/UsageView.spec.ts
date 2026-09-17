import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageView from '../UsageView.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'

const {
  query,
  getStatsByDateRange,
  getStats,
  getDashboardModels,
  getDashboardSnapshotV2,
  listMyErrorRequests,
  list,
  getAvailable,
  adminGetUserView,
  adminGetUserViewStats,
  adminSearchApiKeys,
  adminSearchUsers,
  adminCreateCalibration,
  adminListCalibrations,
  adminRevokeCalibration,
  adminUsersGetById,
  showError,
  showWarning,
  showSuccess,
  showInfo,
  authState,
} = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  getStats: vi.fn(),
  getDashboardModels: vi.fn(),
  getDashboardSnapshotV2: vi.fn(),
  listMyErrorRequests: vi.fn(),
  list: vi.fn(),
  getAvailable: vi.fn(),
  adminGetUserView: vi.fn(),
  adminGetUserViewStats: vi.fn(),
  adminSearchApiKeys: vi.fn(),
  adminSearchUsers: vi.fn(),
  adminCreateCalibration: vi.fn(),
  adminListCalibrations: vi.fn(),
  adminRevokeCalibration: vi.fn(),
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
  'usage.adminCalibrationFailed': 'Failed to submit calibration',
  'usage.adminCalibrationRevoke': 'Revoke',
  'usage.adminCalibrationRevoking': 'Revoking...',
  'usage.adminCalibrationRevoked': 'Revoked',
  'usage.adminCalibrationRevokeConfirm': 'Revoke this calibration?',
  'usage.adminCalibrationRevokeSuccess': 'Calibration revoked',
  'usage.adminCalibrationRevokeFailed': 'Failed to revoke calibration',
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
  'admin.dashboard.timeRange': 'Time range',
  'admin.dashboard.granularity': 'Granularity',
  'admin.dashboard.day': 'Day',
  'admin.dashboard.hour': 'Hour',
  'admin.users.columnSettings': 'Columns',
  'admin.usage.group': 'Group',
  'admin.usage.billingType': 'Billing type',
  'admin.usage.billingMode': 'Billing mode',
  'admin.usage.allTypes': 'All types',
  'admin.usage.allBillingTypes': 'All billing types',
  'admin.usage.billingTypeBalance': 'Balance',
  'admin.usage.billingTypeSubscription': 'Subscription',
  'admin.usage.allBillingModes': 'All billing modes',
  'admin.usage.allGroups': 'All groups',
  'admin.usage.allModels': 'All models',
  'usage.errors.allKeys': 'All API Keys',
  'usage.tabs.usage': 'Usage records',
  'usage.tabs.errors': 'Error records',
  'usage.ws': 'WS',
  'usage.stream': 'Stream',
  'usage.sync': 'Sync',
  'usage.compactionFilter': 'Request Kind',
  'usage.allCompactionTypes': 'All Requests',
  'usage.compactionOnly': 'Compaction Only',
  'usage.exporting': 'Exporting',
  'usage.exportCsv': 'Export CSV',
  'usage.failedToLoad': 'Failed to load',
  'usage.noDataToExport': 'No data',
  'usage.preparingExport': 'Preparing export',
  'usage.exportSuccess': 'Export success',
  'usage.exportFailed': 'Export failed',
  'common.refresh': 'Refresh',
  'common.reset': 'Reset',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
    getStats,
    getDashboardModels,
    getDashboardSnapshotV2,
    listMyErrorRequests,
  },
  keysAPI: {
    list,
  },
  userGroupsAPI: {
    getAvailable,
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
    revokeCalibration: adminRevokeCalibration,
  },
}))

vi.mock('@/api/admin/users', () => ({
  usersAPI: {
    getById: adminUsersGetById,
  },
}))

const appStoreState = vi.hoisted(() => ({
  cachedPublicSettings: { allow_user_view_error_requests: true } as Record<string, unknown>,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError, showWarning, showSuccess, showInfo,
    get cachedPublicSettings() {
      return appStoreState.cachedPublicSettings
    },
  }),
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

vi.mock('@/components/charts/ModelDistributionChart.vue', () => ({
  default: { name: 'ModelDistributionChart', template: '<div />' },
}))
vi.mock('@/components/charts/GroupDistributionChart.vue', () => ({
  default: { name: 'GroupDistributionChart', template: '<div />' },
}))
vi.mock('@/components/charts/EndpointDistributionChart.vue', () => ({
  default: { name: 'EndpointDistributionChart', template: '<div />' },
}))
vi.mock('@/components/charts/TokenUsageTrend.vue', () => ({
  default: { name: 'TokenUsageTrend', template: '<div />' },
}))

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
    getStats.mockReset()
    getDashboardModels.mockReset()
    getDashboardSnapshotV2.mockReset()
    listMyErrorRequests.mockReset()
    list.mockReset()
    getAvailable.mockReset()
    adminGetUserView.mockReset()
    adminGetUserViewStats.mockReset()
    adminSearchApiKeys.mockReset()
    adminSearchUsers.mockReset()
    adminCreateCalibration.mockReset()
    adminListCalibrations.mockReset()
    adminRevokeCalibration.mockReset()
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
    adminRevokeCalibration.mockResolvedValue({})
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
      endpoints: [],
    })
    getDashboardModels.mockResolvedValue({ models: [], start_date: '', end_date: '' })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '',
      start_date: '',
      end_date: '',
      granularity: 'day',
      trend: [],
      groups: [],
    })
    listMyErrorRequests.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    getAvailable.mockResolvedValue([])

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

  it('surfaces backend error message when calibration submit fails', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({ total_requests: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0 })
    list.mockResolvedValue({ items: [] })
    adminCreateCalibration.mockRejectedValue({
      message: '该范围没有原始用量，无法按比例分摊',
      reason: 'ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE',
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

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 7
    setupState.selectedAdminUser = { id: 7, email: 'user@example.com', deleted: false }
    setupState.selectedAdminUserDetail = { id: 7, balance: 20 }
    setupState.calibrationForm.tokenEnabled = false
    setupState.calibrationForm.balanceEnabled = false
    setupState.calibrationForm.consumptionEnabled = true
    setupState.calibrationForm.consumptionMode = 'delta'
    setupState.calibrationForm.consumptionValue = '-70'
    setupState.calibrationForm.tokenStartDate = '2026-07-24'
    setupState.calibrationForm.tokenEndDate = '2026-07-24'
    setupState.calibrationConsumptionCurrentTotal = 70

    await setupState.submitCalibration()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('该范围没有原始用量，无法按比例分摊')
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
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
      endpoints: [],
    })
    getDashboardModels.mockResolvedValue({ models: [], start_date: '', end_date: '' })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '',
      start_date: '',
      end_date: '',
      granularity: 'day',
      trend: [],
      groups: [],
    })
    listMyErrorRequests.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    getAvailable.mockResolvedValue([])

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

  it('revokes an active calibration and refreshes usage data', async () => {
    authState.isAdmin = true
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStatsByDateRange.mockResolvedValue({ total_requests: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0 })
    list.mockResolvedValue({ items: [] })
    adminListCalibrations.mockResolvedValue({
      items: [{ id: 9, target_user_id: 7, token_delta: 100, created_at: '2026-07-24T08:00:00Z' }],
      total: 1,
      pages: 1,
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

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.adminSelectedUserValue = 7
    setupState.selectedAdminUser = { id: 7, email: 'user@example.com', deleted: false }
    setupState.calibrationHistory = [{ id: 9, target_user_id: 7, token_delta: 100, created_at: '2026-07-24T08:00:00Z' }]
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    await setupState.revokeCalibration(setupState.calibrationHistory[0])

    expect(adminRevokeCalibration).toHaveBeenCalledWith(9)
    expect(showSuccess).toHaveBeenCalledWith('Calibration revoked')
    expect(adminListCalibrations).toHaveBeenCalled()
    confirmSpy.mockRestore()
    wrapper.unmount()
  })
})

const simpleStub = { template: '<div><slot /></div>' }
const chartStub = { template: '<div />' }

const usageLog = {
  id: 1,
  request_id: 'req-user-export',
  actual_cost: 0.092883,
  total_cost: 0.092883,
  rate_multiplier: 1,
  service_tier: 'priority',
  input_cost: 0.020285,
  output_cost: 0.00303,
  cache_creation_cost: 0.000001,
  cache_read_cost: 0.069568,
  input_tokens: 4057,
  output_tokens: 101,
  cache_creation_tokens: 4,
  cache_read_tokens: 278272,
  cache_creation_5m_tokens: 0,
  cache_creation_1h_tokens: 0,
  image_count: 0,
  image_size: null,
  first_token_ms: 12,
  duration_ms: 345,
  created_at: '2026-03-08T00:00:00Z',
  model: 'gpt-5.4',
  reasoning_effort: null,
  ip_address: '203.0.113.10',
  api_key: { name: 'demo-key' },
  billing_mode: 'token',
  request_type: 'sync',
  stream: false,
  native_compaction_v2: false,
}

function mountUsageView() {
  return mount(UsageView, {
    global: {
      stubs: {
        AppLayout: simpleStub,
        TablePageLayout: TablePageLayoutStub,
        Pagination: true,
        Select: true,
        DateRangePicker: true,
        Icon: true,
        UsageStatsCards: chartStub,
        UsageTable: chartStub,
        UserErrorRequestsTable: chartStub,
        ModelDistributionChart: chartStub,
        GroupDistributionChart: chartStub,
        EndpointDistributionChart: chartStub,
        TokenUsageTrend: chartStub,
      },
    },
  })
}

describe('user UsageView', () => {
  beforeEach(() => {
    authState.isAdmin = false
    query.mockReset()
    getStats.mockReset()
    getDashboardModels.mockReset()
    getDashboardSnapshotV2.mockReset()
    listMyErrorRequests.mockReset()
    list.mockReset()
    getAvailable.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

    query.mockResolvedValue({ items: [usageLog], total: 1, pages: 1 })
    getStats.mockResolvedValue({
      total_requests: 1,
      total_input_tokens: 10,
      total_output_tokens: 20,
      total_cache_tokens: 0,
      total_tokens: 30,
      total_cost: 0.1,
      total_actual_cost: 0.08,
      average_duration_ms: 12,
      endpoints: [],
      upstream_endpoints: [],
      endpoint_paths: [],
    })
    getDashboardModels.mockResolvedValue({
      models: [{ model: 'gpt-5.4', requests: 1, input_tokens: 10, output_tokens: 20, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 30, cost: 0.1, actual_cost: 0.08 }],
      start_date: '2026-03-08',
      end_date: '2026-03-08',
    })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '2026-03-08T00:00:00Z',
      start_date: '2026-03-08',
      end_date: '2026-03-08',
      granularity: 'hour',
      trend: [],
      groups: [],
    })
    listMyErrorRequests.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    list.mockResolvedValue({ items: [{ id: 1, name: 'demo-key' }], total: 1, page: 1, page_size: 100, pages: 1 })
    getAvailable.mockResolvedValue([{ id: 1, name: 'default' }])
  })

  it('loads logs, stats, model stats, and snapshot on first render', async () => {
    mountUsageView()
    await flushPromises()

    expect(query).toHaveBeenCalled()
    expect(getStats).toHaveBeenCalled()
    expect(getDashboardModels).toHaveBeenCalled()
    expect(getDashboardSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
    }))
    expect(list).toHaveBeenCalledTimes(1)
    expect(list).toHaveBeenCalledWith(1, 100)
    expect(getAvailable).toHaveBeenCalled()
  })

  it('includes API keys after the first page in both record filters and queries by the selected key', async () => {
    const firstPageKeys = Array.from({ length: 100 }, (_, index) => ({
      id: index + 1,
      name: `key-${index + 1}`,
    }))
    const laterKey = { id: 101, name: 'key-from-second-page' }
    list
      .mockResolvedValueOnce({ items: firstPageKeys, total: 101, page: 1, page_size: 100, pages: 2 })
      .mockResolvedValueOnce({ items: [laterKey], total: 101, page: 2, page_size: 100, pages: 2 })

    const wrapper = mountUsageView()
    await flushPromises()

    expect(list.mock.calls).toEqual([[1, 100], [2, 100]])
    const usageKeySelect = wrapper.findAllComponents(Select).find((select) =>
      select.props('options').some((option: SelectOption) => option.label === 'All API Keys')
    )!
    expect(usageKeySelect.props('options')).toHaveLength(102)
    expect(usageKeySelect.props('options')).toContainEqual({ value: laterKey.id, label: laterKey.name })

    query.mockClear()
    usageKeySelect.vm.$emit('update:modelValue', laterKey.id)
    usageKeySelect.vm.$emit('change', laterKey.id)
    await flushPromises()

    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({ api_key_id: laterKey.id, page: 1 }),
      expect.anything()
    )

    await wrapper.findAll('button').find((button) => button.text() === 'Error records')!.trigger('click')
    await flushPromises()

    const errorKeySelect = wrapper.findAllComponents(Select).find((select) =>
      select.props('options').some((option: SelectOption) => option.label === 'All API Keys')
    )!
    expect(errorKeySelect.props('options')).toHaveLength(102)
    expect(errorKeySelect.props('options')).toContainEqual({ value: laterKey.id, label: laterKey.name })

    listMyErrorRequests.mockClear()
    errorKeySelect.vm.$emit('update:modelValue', laterKey.id)
    errorKeySelect.vm.$emit('change', laterKey.id)
    await flushPromises()

    expect(listMyErrorRequests).toHaveBeenCalledWith(
      expect.objectContaining({ api_key_id: laterKey.id, page: 1 })
    )
    expect(list).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not request another API key page when the user has no keys', async () => {
    list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })

    const wrapper = mountUsageView()
    await flushPromises()

    expect(list.mock.calls).toEqual([[1, 100]])
    const keySelect = wrapper.findAllComponents(Select).find((select) =>
      select.props('options').some((option: SelectOption) => option.label === 'All API Keys')
    )!
    expect(keySelect.props('options')).toEqual([{ value: null, label: 'All API Keys' }])
    expect(getAvailable).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('stops loading API keys when a later page is empty despite an outdated page count', async () => {
    const firstPageKeys = Array.from({ length: 100 }, (_, index) => ({
      id: index + 1,
      name: `key-${index + 1}`,
    }))
    list
      .mockResolvedValueOnce({ items: firstPageKeys, total: 201, page: 1, page_size: 100, pages: 3 })
      .mockResolvedValueOnce({ items: [], total: 201, page: 2, page_size: 100, pages: 3 })

    const wrapper = mountUsageView()
    await flushPromises()

    expect(list.mock.calls).toEqual([[1, 100], [2, 100]])
    const keySelect = wrapper.findAllComponents(Select).find((select) =>
      select.props('options').some((option: SelectOption) => option.label === 'All API Keys')
    )!
    expect(keySelect.props('options')).toHaveLength(101)
    expect(keySelect.props('options')).toContainEqual({ value: 100, label: 'key-100' })
    wrapper.unmount()
  })

  it('propagates and resets the native compaction filter across page requests', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    expect((wrapper.vm as any).compactionOptions).toEqual([
      { value: null, label: 'All Requests' },
      { value: true, label: 'Compaction Only' },
    ])

    query.mockClear()
    getStats.mockClear()
    getDashboardModels.mockClear()
    getDashboardSnapshotV2.mockClear()

    ;(wrapper.vm as any).filters.native_compaction_v2 = true
    ;(wrapper.vm as any).applyFilters()
    await flushPromises()

    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({ native_compaction_v2: true }),
      expect.anything()
    )
    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: true }))
    expect(getDashboardModels).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: true }))
    expect(getDashboardSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: true }))

    query.mockClear()
    getStats.mockClear()
    getDashboardModels.mockClear()
    getDashboardSnapshotV2.mockClear()

    ;(wrapper.vm as any).resetFilters()
    await flushPromises()

    expect((wrapper.vm as any).filters.native_compaction_v2).toBeNull()
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({ native_compaction_v2: null }),
      expect.anything()
    )
    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: null }))
    expect(getDashboardModels).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: null }))
    expect(getDashboardSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({ native_compaction_v2: null }))
  })

  it('exports csv with current filters and without admin-only fields', async () => {
    const wrapper = mountUsageView()
    await flushPromises()
    ;(wrapper.vm as any).filters.native_compaction_v2 = true

    let exportedBlob: Blob | null = null
    let csvContent = ''
    const OriginalBlob = globalThis.Blob
    vi.stubGlobal('Blob', vi.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
      csvContent = parts.map((part) => String(part)).join('')
      return new OriginalBlob(parts, options)
    }))
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    await (wrapper.vm as any).exportToCSV()

    expect(exportedBlob).not.toBeNull()
    expect(query).toHaveBeenCalledWith(expect.objectContaining({
      page_size: 100,
      sort_by: 'created_at',
      sort_order: 'desc',
      native_compaction_v2: true,
    }))
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
    expect(csvContent.startsWith('\uFEFF')).toBe(true)
    expect(csvContent.slice(1)).toBe([
      'Time,API Key Name,Model,Reasoning Effort,Inbound Endpoint,IP Address,Type,Billing Mode,Input Tokens,Output Tokens,Cache Read Tokens,Cache Creation Tokens,Rate Multiplier,Billed Cost,Original Cost,First Token (ms),Duration (ms)',
      '2026-03-08T00:00:00Z,demo-key,gpt-5.4,"\'-",,203.0.113.10,Sync,Token,4057,101,278272,4,1,0.09288300,0.09288300,12,345',
    ].join('\n'))
    expect(csvContent).toContain('IP Address')
    expect(csvContent).toContain('203.0.113.10')
    expect(csvContent).toContain('Billed Cost')
    expect(csvContent).toContain('Original Cost')
    expect(csvContent).not.toContain('Upstream Endpoint')
    expect(csvContent).not.toContain('account_cost')
    expect(csvContent).not.toContain('account_rate_multiplier')

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    vi.unstubAllGlobals()
    clickSpy.mockRestore()
  })

  it('keeps the initial filters, sort, and filename while exporting multiple pages', async () => {
    const pageResponse = { items: [usageLog], total: 101, pages: 2 }
    query.mockResolvedValue(pageResponse)
    const wrapper = mountUsageView()
    await flushPromises()

    const datePicker = wrapper.findComponent(DateRangePicker)
    datePicker.vm.$emit('change', { startDate: '2026-03-01', endDate: '2026-03-08', preset: null })
    await flushPromises()

    let resolveFirstPage!: (value: typeof pageResponse) => void
    const firstPage = new Promise<typeof pageResponse>((resolve) => { resolveFirstPage = resolve })
    query.mockClear()
    query.mockImplementation((params, options) =>
      !options && params.page === 1 ? firstPage : Promise.resolve(pageResponse)
    )
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn(() => 'blob:usage-export')
    window.URL.revokeObjectURL = vi.fn()
    let filename = ''
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      filename = this.download
    })

    try {
      await wrapper.findAll('button').find((button) => button.text() === 'Export CSV')!.trigger('click')
      const initialParams = { ...query.mock.calls[0][0] }
      expect(initialParams).toMatchObject({
        page: 1, page_size: 100, start_date: '2026-03-01', end_date: '2026-03-08',
        sort_by: 'created_at', sort_order: 'desc',
      })

      const keySelect = wrapper.findAllComponents(Select).find((select) =>
        select.props('options').some((option: SelectOption) => option.label === 'All API Keys')
      )!
      keySelect.vm.$emit('update:modelValue', 1)
      keySelect.vm.$emit('change', 1)
      datePicker.vm.$emit('change', { startDate: '2026-04-01', endDate: '2026-04-08', preset: null })
      wrapper.findComponent(UsageTable).vm.$emit('sort', 'actual_cost', 'asc')
      await flushPromises()
      expect(query).toHaveBeenCalledWith(expect.objectContaining({
        api_key_id: 1, start_date: '2026-04-01', end_date: '2026-04-08',
        sort_by: 'actual_cost', sort_order: 'asc',
      }), expect.anything())

      resolveFirstPage(pageResponse)
      await flushPromises()

      const exportCalls = query.mock.calls.filter((call) => call.length === 1)
      expect.soft(exportCalls).toEqual([[initialParams], [{ ...initialParams, page: 2 }]])
      expect.soft(filename).toBe('usage_2026-03-01_to_2026-03-08.csv')
      expect(showSuccess).toHaveBeenCalledWith('Export success')
      expect(showError).not.toHaveBeenCalled()
    } finally {
      window.URL.createObjectURL = originalCreateObjectURL
      window.URL.revokeObjectURL = originalRevokeObjectURL
      clickSpy.mockRestore()
      wrapper.unmount()
    }
  })

  it('exports historical image rows with image billing mode derived from image_count', async () => {
    query.mockResolvedValue({
      items: [
        {
          ...usageLog,
          request_id: 'req-user-export-legacy-image',
          actual_cost: 0.2,
          total_cost: 0.2,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          image_count: 1,
          model: 'gpt-image-2',
          billing_mode: null,
          ip_address: null,
        },
      ],
      total: 1,
      pages: 1,
    })

    const wrapper = mountUsageView()
    await flushPromises()

    let csvContent = ''
    const OriginalBlob = globalThis.Blob
    vi.stubGlobal('Blob', vi.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
      csvContent = parts.map((part) => String(part)).join('')
      return new OriginalBlob(parts, options)
    }))
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn(() => 'blob:usage-export') as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    await (wrapper.vm as any).exportToCSV()

    expect(csvContent).toContain('Billing Mode')
    expect(csvContent).toContain('Image')
    expect(csvContent).not.toContain(',Token,0,0,0,0,')

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    vi.unstubAllGlobals()
    clickSpy.mockRestore()
  })
})

describe('UsageView subscription feature flag', () => {
  afterEach(() => {
    appStoreState.cachedPublicSettings = { allow_user_view_error_requests: true }
  })

  function billingTypeSelect(wrapper: ReturnType<typeof mountUsageView>) {
    return wrapper.findAllComponents(Select).find((select) =>
      select.props('options').some((option: SelectOption) => option.label === 'Subscription')
    )
  }

  it('offers the balance / subscription billing-type filter by default', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    expect(billingTypeSelect(wrapper)).toBeDefined()
    expect(wrapper.text()).toContain('Billing type')
    wrapper.unmount()
  })

  it('hides the billing-type filter entirely when subscriptions are disabled', async () => {
    appStoreState.cachedPublicSettings = { allow_user_view_error_requests: true, subscription_enabled: false }

    const wrapper = mountUsageView()
    await flushPromises()

    expect(billingTypeSelect(wrapper)).toBeUndefined()
    expect(wrapper.text()).not.toContain('Billing type')
    wrapper.unmount()
  })
})
