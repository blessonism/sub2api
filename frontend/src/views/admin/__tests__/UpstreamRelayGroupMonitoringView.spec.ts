import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamRelayGroupMonitoringView from '../UpstreamRelayGroupMonitoringView.vue'

const {
  listConnectors,
  listCandidates,
  listRecommendationRuns,
  getMonitoringPolicy,
  updateMonitoringPolicy,
  getRecommendationPolicy,
  listSnapshots,
  refreshConnectorMetrics,
  syncAllConnectors,
  probeAllCandidates,
  createCandidate,
  updateCandidate,
  listAccounts,
  listGroups,
} = vi.hoisted(() => ({
  listConnectors: vi.fn(),
  listCandidates: vi.fn(),
  listRecommendationRuns: vi.fn(),
  getMonitoringPolicy: vi.fn(),
  updateMonitoringPolicy: vi.fn(),
  getRecommendationPolicy: vi.fn(),
  listSnapshots: vi.fn(),
  refreshConnectorMetrics: vi.fn(),
  syncAllConnectors: vi.fn(),
  probeAllCandidates: vi.fn(),
  createCandidate: vi.fn(),
  updateCandidate: vi.fn(),
  listAccounts: vi.fn(),
  listGroups: vi.fn(),
}))

vi.mock('@/api/admin/upstreamRelayGroupMonitors', () => ({
  default: {
    listConnectors,
    listCandidates,
    listRecommendationRuns,
    getMonitoringPolicy,
    updateMonitoringPolicy,
    getRecommendationPolicy,
    listSnapshots,
    refreshConnectorMetrics,
    syncAllConnectors,
    probeAllCandidates,
    createCandidate,
    updateCandidate,
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    list: listAccounts,
  },
}))

vi.mock('@/api/admin/groups', () => ({
  default: {
    getAllIncludingInactive: listGroups,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        return `${key.replace(/\{(\w+)\}/g, (_, token) => String(params[token] ?? `{${token}}`))} ${Object.values(params).join(' ')}`
      },
    }),
  }
})

function mountView() {
  return mount(UpstreamRelayGroupMonitoringView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        LoadingSpinner: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        AutoRefreshButton: true,
        HealthRateBar: true,
        RateSourceTag: true,
        CandidateHealthDialog: true,
      },
    },
  })
}

describe('UpstreamRelayGroupMonitoringView', () => {
  beforeEach(() => {
    listConnectors.mockReset()
    listCandidates.mockReset()
    listRecommendationRuns.mockReset()
    getMonitoringPolicy.mockReset()
    updateMonitoringPolicy.mockReset()
    getRecommendationPolicy.mockReset()
    listSnapshots.mockReset()
    refreshConnectorMetrics.mockReset()
    syncAllConnectors.mockReset()
    probeAllCandidates.mockReset()
    createCandidate.mockReset()
    updateCandidate.mockReset()
    listAccounts.mockReset()
    listGroups.mockReset()

    listConnectors.mockResolvedValue({
      items: [{
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    listCandidates.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 1 })
    listRecommendationRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    listSnapshots.mockResolvedValue([])
    getRecommendationPolicy.mockResolvedValue({
      snapshot_freshness_minutes: 1440,
      usage_delta_freshness_minutes: 1440,
      probe_freshness_minutes: 30,
      min_success_rate: 0.5,
      min_sample_size: 3,
      exclude_consecutive_failures: true,
      priority_start: 10,
      priority_step: 10,
      sort_fields: ['rate_asc', 'success_rate_desc', 'latency_asc'],
    })
    getMonitoringPolicy.mockResolvedValue({
      auto_sync_enabled: true,
      sync_interval_minutes: 60,
      auto_probe_enabled: true,
      probe_interval_minutes: 15,
      failure_retry_interval_minutes: 5,
      sync_concurrency: 2,
      probe_concurrency: 5,
      snapshot_stale_after_minutes: 180,
      usage_delta_stale_after_minutes: 180,
      probe_stale_after_minutes: 45,
      updated_at: '2026-06-28T12:00:00Z',
    })
    updateMonitoringPolicy.mockResolvedValue({
      auto_sync_enabled: true,
      sync_interval_minutes: 60,
      auto_probe_enabled: true,
      probe_interval_minutes: 15,
      failure_retry_interval_minutes: 5,
      sync_concurrency: 2,
      probe_concurrency: 5,
      snapshot_stale_after_minutes: 180,
      usage_delta_stale_after_minutes: 180,
      probe_stale_after_minutes: 45,
      updated_at: '2026-06-28T12:10:00Z',
    })
    listAccounts.mockResolvedValue({
      items: [{ id: 42, name: 'claude-relay', platform: 'claude' }],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1,
    })
    listGroups.mockResolvedValue([{ id: 9, name: 'vip', description: '', user_count: 0, created_at: '', updated_at: '' }])
  })

  it('顶部概览合并状态卡、今日用量和最近同步，并在 Priority tab 显示数字徽章', async () => {
    listConnectors.mockResolvedValue({
      items: [
        {
          id: 7,
          name: 'relay-a',
          base_url: 'https://relay.example.com',
          auth_mode: 'manual_session',
          status: 'active',
          credential_version: 1,
          has_bearer_token: true,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          last_synced_at: '2026-06-28T12:00:00Z',
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
        {
          id: 8,
          name: 'relay-b',
          base_url: 'https://relay-b.example.com',
          auth_mode: 'manual_session',
          status: 'needs_reauth',
          credential_version: 1,
          has_bearer_token: false,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    listCandidates.mockResolvedValue({
      items: [
        {
          id: 101,
          connector_id: 7,
          account_id: 42,
          upstream_group_id: 'team-a',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: true,
          notes: '',
          latest_probe: { id: 1, candidate_id: 101, success: true, probed_at: '2026-06-28T12:00:00Z' },
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
        {
          id: 102,
          connector_id: 8,
          account_id: 42,
          upstream_group_id: 'team-b',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: false,
          notes: '',
          latest_probe: { id: 2, candidate_id: 102, success: false, probed_at: '2026-06-28T12:00:00Z' },
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    listRecommendationRuns.mockResolvedValue({
      items: [{
        id: 77,
        status: 'success',
        total_candidates: 2,
        suggestion_count: 3,
        applied: false,
        created_at: '2026-06-28T12:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listSnapshots.mockImplementation((connectorId: number) => Promise.resolve(
      connectorId === 7
        ? [
            {
              id: 501,
              connector_id: 7,
              upstream_group_id: 'team-a',
              name: 'Team A',
              platform: 'claude',
              status: 'active',
              default_rate_multiplier: 1,
              final_rate_multiplier: 1,
              today_actual_cost: 1.25,
              today_total_tokens: 1_250_000,
              source: 'login_available_groups',
              last_seen_at: '2026-06-28T12:00:00Z',
            },
            {
              id: 502,
              connector_id: 7,
              upstream_group_id: 'team-extra',
              name: 'Team Extra',
              platform: 'claude',
              status: 'active',
              default_rate_multiplier: 1,
              final_rate_multiplier: 1,
              today_actual_cost: 0.5,
              today_total_tokens: 500_000,
              source: 'login_available_groups',
              last_seen_at: '2026-06-28T12:00:00Z',
            },
          ]
        : [
            {
              id: 601,
              connector_id: 8,
              upstream_group_id: 'team-b',
              name: 'Team B',
              platform: 'claude',
              status: 'active',
              default_rate_multiplier: 1,
              final_rate_multiplier: 1,
              today_actual_cost: 2,
              today_total_tokens: 1_500_000,
              source: 'login_available_groups',
              last_seen_at: '2026-06-28T12:00:00Z',
            },
          ]
    ))

    const wrapper = mountView()
    await flushPromises()

    const overview = wrapper.find('section')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.connectorStatus')
    expect(overview.text()).toContain('1 / 2')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.connectorStatusHint')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.candidateStatus')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.candidateStatusHint')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.todayUsage')
    expect(overview.find('[data-testid="overview-card-value-today-usage"]').text()).toBe('$3.75')
    expect(overview.text()).toContain('3.25M')
    expect(overview.text()).toContain('admin.upstreamRelayGroupMonitoring.overview.latestSync')
    expect(overview.text()).not.toContain('admin.upstreamRelayGroupMonitoring.overview.pendingSuggestions')
    expect(listSnapshots).toHaveBeenCalledWith(7)
    expect(listSnapshots).toHaveBeenCalledWith(8)

    const priorityTab = wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!
    expect(priorityTab.text()).toContain('3')
  })

  it('轻量刷新首次没有倍率快照时提示先完整同步', async () => {
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        upstream_account_balance: 12.34,
        upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'partial',
      balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: {
        status: 'skipped',
        total_groups: 1,
        updated_groups: 0,
        missing_groups: [{ upstream_group_id: 'team-alpha', reason: 'no_snapshot', message: 'full sync required' }],
      },
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    expect(wrapper.find('[data-testid="page-error"]').text()).toContain('admin.upstreamRelayGroupMonitoring.errors.metricsRefreshNeedsFullSync')
  })

  it('轻量刷新接口失败时优先展示真实失败原因', async () => {
    refreshConnectorMetrics.mockRejectedValueOnce(new Error('network failed'))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    expect(summary.exists()).toBe(true)
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').exists()).toBe(true)
    expect(summary.text()).toContain('relay-a')
    expect(wrapper.find('[data-testid="page-error"]').text()).toContain('network failed')
    expect(wrapper.text()).toContain('network failed')
    expect(wrapper.text()).not.toContain('admin.upstreamRelayGroupMonitoring.errors.metricsRefreshNeedsFullSync')
  })

  it('手动刷新用量余额后展示摘要和连接器详情', async () => {
    listConnectors.mockResolvedValue({
      items: [
        {
          id: 7,
          name: 'relay-a',
          base_url: 'https://relay.example.com',
          auth_mode: 'manual_session',
          status: 'active',
          credential_version: 1,
          has_bearer_token: true,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
        {
          id: 8,
          name: 'relay-b',
          base_url: 'https://relay-b.example.com',
          auth_mode: 'manual_session',
          status: 'active',
          credential_version: 1,
          has_bearer_token: true,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    refreshConnectorMetrics
      .mockResolvedValueOnce({
        connector: {
          id: 7,
          name: 'relay-a',
          base_url: 'https://relay.example.com',
          auth_mode: 'manual_session',
          status: 'active',
          credential_version: 1,
          upstream_account_balance: 12.34,
          upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
          has_bearer_token: true,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:05:00Z',
        },
        snapshots: [{ id: 1, connector_id: 7, upstream_group_id: 'team-a', name: 'Team A', platform: 'claude', status: 'active', default_rate_multiplier: 1, final_rate_multiplier: 1, source: 'login_available_groups', last_seen_at: '2026-06-28T12:00:00Z' }],
        status: 'success',
        balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
        usage_detail: { status: 'success', total_groups: 1, updated_groups: 1, missing_groups: [] },
        balance_available: true,
        usage_available: true,
        refreshed_at: '2026-06-28T12:05:00Z',
      })
      .mockResolvedValueOnce({
        connector: {
          id: 8,
          name: 'relay-b',
          base_url: 'https://relay-b.example.com',
          auth_mode: 'manual_session',
          status: 'active',
          credential_version: 1,
          upstream_account_balance: 1.23,
          upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
          has_bearer_token: true,
          has_refresh_token: false,
          has_login_email: false,
          has_cookie: false,
          has_user_agent: false,
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:05:00Z',
        },
        snapshots: [{ id: 2, connector_id: 8, upstream_group_id: 'team-b', name: 'Team B', platform: 'claude', status: 'active', default_rate_multiplier: 1, final_rate_multiplier: 1, source: 'login_available_groups', last_seen_at: '2026-06-28T12:00:00Z' }],
        status: 'partial',
        balance_detail: { status: 'success', value: 1.23, checked_at: '2026-06-28T12:05:00Z' },
        usage_detail: {
          status: 'failed',
          total_groups: 1,
          updated_groups: 0,
          missing_groups: [{ upstream_group_id: 'team-b', name: 'Team B', reason: 'usage_refresh_failed', message: 'connector has no local account bindings' }],
          error: 'connector has no local account bindings',
        },
        balance_available: true,
        usage_available: false,
        usage_error: 'connector has no local account bindings',
        refreshed_at: '2026-06-28T12:05:00Z',
      })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(summary.exists()).toBe(true)
    expect(summary.text()).toContain('metricsRefresh.summaryTitle')
    expect(summary.text()).toContain('1 1 0')
    expect(summary.text()).toContain('2 2 1')
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').exists()).toBe(true)
    expect(summary.text()).toContain('relay-a')
    expect(summary.text()).toContain('relay-b')
    expect(summary.text()).toContain('Team B')
    expect(summary.text()).toContain('connector has no local account bindings')
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('metricsRefresh.inlineUsagePartial')
  })

  it('余额成功但 usage 无本地绑定时即使没有快照也不显示顶部错误', async () => {
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        upstream_account_balance: 12.34,
        upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'partial',
      balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: {
        status: 'skipped',
        total_groups: 0,
        updated_groups: 0,
        missing_groups: [],
        error: 'connector has no local account bindings',
      },
      balance_available: true,
      usage_available: false,
      usage_error: 'connector has no local account bindings',
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(summary.exists()).toBe(true)
    expect(summary.text()).toContain('connector has no local account bindings')
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.upstreamRelayGroupMonitoring.errors.metricsRefreshNeedsFullSync')
  })

  it('轻量刷新返回 null missing_groups 时不崩溃且保留 usage 错误详情', async () => {
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        upstream_account_balance: 12.34,
        upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'partial',
      balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: {
        status: 'skipped',
        total_groups: 0,
        updated_groups: 0,
        missing_groups: null,
        error: 'connector has no local account bindings',
      },
      balance_available: true,
      usage_available: false,
      usage_error: 'connector has no local account bindings',
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(summary.exists()).toBe(true)
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('connector has no local account bindings')
    expect(wrapper.text()).not.toContain('admin.upstreamRelayGroupMonitoring.errors.metricsRefreshNeedsFullSync')
  })

  it('轻量刷新返回 undefined missing_groups 且仅本地绑定缺失时不显示顶部错误', async () => {
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        upstream_account_balance: null,
        upstream_account_balance_checked_at: null,
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'failed',
      balance_detail: { status: 'failed', error: 'connector has no local account bindings' },
      usage_detail: {
        status: 'failed',
        total_groups: 0,
        updated_groups: 0,
        error: 'connector has no local account bindings',
      },
      balance_available: false,
      balance_error: 'connector has no local account bindings',
      usage_available: false,
      usage_error: 'connector has no local account bindings',
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(summary.exists()).toBe(true)
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('connector has no local account bindings')
  })

  it('连接器展开区以无表头只读数据展示分组概览', async () => {
    listCandidates.mockResolvedValue({
      items: [{
        id: 101,
        connector_id: 7,
        account_id: 42,
        account_name: 'claude-relay',
        account_platform: 'claude',
        upstream_group_id: 'team-alpha',
        upstream_group_name: 'Team Alpha',
        probe_model: 'gpt-4o-mini',
        probe_protocol: 'chat_completions',
        current_priority: 20,
        enabled: true,
        notes: '',
        today_actual_cost: 1.25,
        today_total_tokens: 1234,
        today_usage_checked_at: '2026-06-28T12:05:00Z',
        latest_probe: { id: 1, candidate_id: 101, success: true, latency_ms: 123, probed_at: '2026-06-28T12:00:00Z' },
        latest_snapshot: {
          id: 501,
          connector_id: 7,
          upstream_group_id: 'team-alpha',
          name: 'Team Alpha',
          platform: 'claude',
          status: 'active',
          default_rate_multiplier: 1.25,
          final_rate_multiplier: 1.25,
          source: 'login_available_groups',
          last_seen_at: '2026-06-28T12:00:00Z',
        },
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label')?.includes('connectors.expandGroups'))!
      .trigger('click')
    await flushPromises()

    const expansion = wrapper.find('[data-testid="connector-group-expansion"]')
    expect(expansion.exists()).toBe(true)
    expect(expansion.findAll('[data-testid="connector-group-item"]')).toHaveLength(1)
    expect(expansion.text()).toContain('Team Alpha')
    expect(expansion.text()).toContain('team-alpha')
    expect(expansion.text()).toContain('1.25')
    expect(expansion.text()).toContain('admin.upstreamRelayGroupMonitoring.health.success')
    expect(expansion.text()).toContain('Priority 20')
    expect(expansion.text()).toContain('$1.25')
    expect(expansion.text()).toContain('0.00M')
    expect(expansion.text()).not.toContain('同步于')
    expect(expansion.text()).not.toContain('2026-06-28T12:05:00Z')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.connectors.colName')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.edit')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.probe')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.delete')
  })

  it('保存候选失败时展示接口返回的详细原因', async () => {
    createCandidate.mockRejectedValue({
      status: 400,
      code: 'UPSTREAM_RELAY_INVALID_CANDIDATE',
      message: 'upstream_group_id 不能为空',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('candidates.newCandidate'))!.trigger('click')
    await flushPromises()
    await wrapper.find('#candidate-form').trigger('submit')
    await flushPromises()

    expect(createCandidate).toHaveBeenCalledWith(expect.objectContaining({ connector_id: 7 }))
    expect(createCandidate.mock.calls[0]?.[0]).not.toHaveProperty('target_group_id')
    expect(wrapper.text()).toContain('upstream_group_id 不能为空')
    expect(wrapper.text()).not.toContain('admin.upstreamRelayGroupMonitoring.errors.saveCandidateFailed')
  })

  it('自动监控页展示派生阈值并保存配置', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('monitoring.snapshotStaleDerived')
    expect(wrapper.text()).toContain('60 180')
    expect(wrapper.text()).toContain('15 45')

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.save'))!.trigger('click')
    await flushPromises()

    expect(updateMonitoringPolicy).toHaveBeenCalledWith({
      auto_sync_enabled: true,
      sync_interval_minutes: 60,
      auto_probe_enabled: true,
      probe_interval_minutes: 15,
      failure_retry_interval_minutes: 5,
      sync_concurrency: 2,
      probe_concurrency: 5,
    })
    expect(wrapper.text()).toContain('monitoring.savedAt')
  })

  it('自动监控页调用批量同步和批量探测接口并展示失败摘要', async () => {
    listCandidates.mockResolvedValue({
      items: [{
        id: 101,
        connector_id: 7,
        account_id: 42,
        account_name: 'claude-relay',
        upstream_group_id: 'team-alpha',
        probe_model: 'gpt-4o-mini',
        probe_protocol: 'chat_completions',
        enabled: true,
        notes: '',
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    syncAllConnectors.mockResolvedValue({
      total: 2,
      success: 1,
      failed: 1,
      items: [{ id: 7, connector_id: 7, connector_name: 'relay-a', success: false, error_reason: 'timeout' }],
    })
    probeAllCandidates.mockResolvedValue({
      total: 1,
      success: 0,
      failed: 1,
      items: [{ id: 101, candidate_id: 101, account_id: 42, account_name: 'claude-relay', success: false, error_reason: 'rate limited' }],
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.syncAll'))!.trigger('click')
    await flushPromises()

    expect(syncAllConnectors).toHaveBeenCalled()
    expect(wrapper.text()).toContain('monitoring.syncResultTitle')
    expect(wrapper.text()).toContain('relay-a: timeout')
    expect(wrapper.text()).toContain('admin.upstreamRelayGroupMonitoring.errors.syncAllPartialFailed')

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.probeAll'))!.trigger('click')
    await flushPromises()

    expect(probeAllCandidates).toHaveBeenCalled()
    expect(wrapper.text()).toContain('monitoring.probeResultTitle')
    expect(wrapper.text()).toContain('#42 claude-relay: rate limited')
    expect(wrapper.text()).toContain('admin.upstreamRelayGroupMonitoring.errors.probeAllPartialFailed')
  })
})
