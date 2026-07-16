import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import UpstreamRelayGroupMonitoringView from '../UpstreamRelayGroupMonitoringView.vue'
import { useAuthStore } from '@/stores/auth'

const {
  listConnectors,
  listCandidates,
  listRecommendationRuns,
  getRecommendationRun,
  applyRecommendationRun,
  closeRecommendationRun,
  restoreRecommendationRun,
  getMonitoringPolicy,
  updateMonitoringPolicy,
  getRecommendationPolicy,
  updateRecommendationPolicy,
  previewRecommendations,
  generateRecommendations,
  listSnapshots,
  listUsageHistory,
  refreshConnectorMetrics,
  refreshMonitoringData,
  syncConnector,
  syncAllConnectors,
  probeAllCandidates,
  createConnector,
  updateConnector,
  createCandidate,
  updateCandidate,
  listConnectorAPIKeys,
  listAccounts,
  listGroups,
  getRunnerStatus,
} = vi.hoisted(() => ({
  listConnectors: vi.fn(),
  listCandidates: vi.fn(),
  listRecommendationRuns: vi.fn(),
  getRecommendationRun: vi.fn(),
  applyRecommendationRun: vi.fn(),
  closeRecommendationRun: vi.fn(),
  restoreRecommendationRun: vi.fn(),
  getMonitoringPolicy: vi.fn(),
  updateMonitoringPolicy: vi.fn(),
  getRecommendationPolicy: vi.fn(),
  updateRecommendationPolicy: vi.fn(),
  previewRecommendations: vi.fn(),
  generateRecommendations: vi.fn(),
  listSnapshots: vi.fn(),
  listUsageHistory: vi.fn(),
  refreshConnectorMetrics: vi.fn(),
  refreshMonitoringData: vi.fn(),
  syncConnector: vi.fn(),
  syncAllConnectors: vi.fn(),
  probeAllCandidates: vi.fn(),
  createConnector: vi.fn(),
  updateConnector: vi.fn(),
  createCandidate: vi.fn(),
  updateCandidate: vi.fn(),
  listConnectorAPIKeys: vi.fn(),
  listAccounts: vi.fn(),
  listGroups: vi.fn(),
  getRunnerStatus: vi.fn(),
}))

const {
  login,
  login2FA,
  logout,
  getCurrentUser,
  register,
  refreshToken,
  reportActivity,
} = vi.hoisted(() => ({
  login: vi.fn(),
  login2FA: vi.fn(),
  logout: vi.fn(),
  getCurrentUser: vi.fn(),
  register: vi.fn(),
  refreshToken: vi.fn(),
  reportActivity: vi.fn(),
}))

vi.mock('@/api/admin/upstreamRelayGroupMonitors', () => ({
  default: {
    listConnectors,
    listCandidates,
    listRecommendationRuns,
    getRecommendationRun,
    applyRecommendationRun,
    closeRecommendationRun,
    restoreRecommendationRun,
    getMonitoringPolicy,
    updateMonitoringPolicy,
    getRecommendationPolicy,
    updateRecommendationPolicy,
    previewRecommendations,
    generateRecommendations,
    listSnapshots,
    listUsageHistory,
    refreshConnectorMetrics,
    refreshMonitoringData,
    syncConnector,
    syncAllConnectors,
    probeAllCandidates,
    createConnector,
    updateConnector,
    createCandidate,
    updateCandidate,
    listConnectorAPIKeys,
    getRunnerStatus,
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

vi.mock('@/api', () => ({
  authAPI: {
    login,
    login2FA,
    logout,
    getCurrentUser,
    register,
    refreshToken,
    reportActivity,
  },
  isTotp2FARequired: (response: { requires_2fa?: boolean }) => response?.requires_2fa === true,
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

const mountedWrappers: VueWrapper[] = []

function mountView() {
  const wrapper = mount(UpstreamRelayGroupMonitoringView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        LoadingSpinner: true,
        BaseDialog: {
          props: ['show', 'closeOnEscape', 'closeOnClickOutside', 'showCloseButton', 'animated'],
          emits: ['close'],
          template: '<div v-if="show" :data-close-on-escape="String(closeOnEscape)" :data-close-on-click-outside="String(closeOnClickOutside)" :data-show-close-button="String(showCloseButton)" :data-animated="String(animated)"><button type="button" data-testid="implicit-close" @click="$emit(\'close\')">implicit close</button><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: {
          props: ['show', 'title', 'message', 'danger'],
          emits: ['confirm', 'cancel'],
          template: '<div v-if="show" data-testid="confirm-dialog" :data-danger="String(danger)"><div data-testid="confirm-title">{{ title }}</div><div data-testid="confirm-message">{{ message }}</div><button type="button" data-testid="confirm-action" @click="$emit(\'confirm\')">confirm</button><button type="button" data-testid="confirm-cancel" @click="$emit(\'cancel\')">cancel</button></div>',
        },
        AutoRefreshButton: true,
        HealthRateBar: true,
        RateSourceTag: true,
        CandidateHealthDialog: true,
      },
    },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function mountViewWithRealDialog() {
  const wrapper = mount(UpstreamRelayGroupMonitoringView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        LoadingSpinner: true,
        ConfirmDialog: true,
        AutoRefreshButton: true,
        HealthRateBar: true,
        RateSourceTag: true,
        CandidateHealthDialog: true,
      },
    },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function usageHistoryCalls() {
  return listUsageHistory.mock.calls.map((call) => call[0] || {})
}

function usageHistoryTabCalls() {
  return usageHistoryCalls().filter((params) => params.page_size === 50)
}

function monitoringPolicy(overrides: Record<string, unknown> = {}) {
  return {
    auto_sync_enabled: true,
    sync_interval_minutes: 60,
    auto_probe_enabled: true,
    probe_interval_minutes: 15,
    auto_recommendation_enabled: false,
    recommendation_interval_minutes: 60,
    auto_apply_recommendations_enabled: false,
    max_auto_apply_suggestions: 20,
    max_auto_apply_priority_delta: 100,
    min_auto_apply_confidence: 'medium',
    allow_auto_apply_degraded_health: false,
    failure_retry_interval_minutes: 5,
    sync_concurrency: 2,
    probe_concurrency: 5,
    snapshot_stale_after_minutes: 180,
    usage_delta_stale_after_minutes: 180,
    probe_stale_after_minutes: 45,
    updated_at: '2026-06-28T12:00:00Z',
    ...overrides,
  }
}

function runnerStatus(overrides: Record<string, unknown> = {}) {
  return {
    observed_at: '2026-06-28T12:00:00Z',
    sync: { enabled: true, in_flight: false, last_finished_at: null, last_succeeded: null, last_error: '', next_run_at: null, interval_minutes: 60, failure_retry_minutes: 5 },
    probe: { enabled: true, in_flight: false, last_finished_at: null, last_succeeded: null, last_error: '', next_run_at: null, interval_minutes: 15, failure_retry_minutes: 5 },
    recommendation: { enabled: false, in_flight: false, last_finished_at: null, last_succeeded: null, last_error: '', next_run_at: null, interval_minutes: 60, failure_retry_minutes: 5 },
    finalize: { enabled: true, in_flight: false, last_finished_at: null, last_succeeded: null, last_error: '', next_run_at: null, interval_minutes: 0, failure_retry_minutes: 5 },
    ...overrides,
  }
}

type TestConnectorMetricsRefreshResult = {
  connector: Record<string, unknown> & { id: number; name: string }
  snapshots: Array<Record<string, unknown> & { connector_id: number }>
  status: 'success' | 'partial' | 'failed'
  balance_detail: Record<string, unknown>
  usage_detail: Record<string, unknown>
  balance_available: boolean
  usage_available: boolean
  balance_error?: string
  usage_error?: string
  refreshed_at: string
}

function monitoringRefreshResultFromMetrics(items: TestConnectorMetricsRefreshResult[]) {
  return {
    status: items.every((item) => item.status === 'success') ? 'success' : items.every((item) => item.status === 'failed') ? 'failed' : 'partial',
    total: items.length,
    success: items.filter((item) => item.status === 'success').length,
    partial: items.filter((item) => item.status === 'partial').length,
    failed: items.filter((item) => item.status === 'failed').length,
    refreshed_at: '2026-06-28T12:05:00Z',
    items: items.map((item) => ({
      connector_id: item.connector.id,
      connector_name: item.connector.name,
      connector: item.connector,
      status: item.status,
      snapshot_status: item.snapshots.length > 0 ? 'success' : 'skipped',
      snapshot_count: item.snapshots.length,
      snapshots: item.snapshots,
      metrics: item,
      error_reason: item.usage_error || item.balance_error,
    })),
  }
}

function recommendationRun(overrides: Record<string, unknown> = {}) {
  return {
    id: 77,
    status: 'success',
    total_candidates: 1,
    suggestion_count: 1,
    applied: false,
    closed: false,
    created_by: 42,
    created_at: '2026-06-28T12:00:00Z',
    suggestions: [{
      id: 1,
      run_id: 77,
      action_type: 'priority_update',
      candidate_id: 101,
      connector_id: 7,
      connector_name: 'relay-a',
      account_id: 42,
      account_name: 'claude-relay',
      upstream_group_id: 'team-alpha',
      upstream_group_name: 'Team Alpha',
      old_priority: 50,
      new_priority: 10,
      final_rate_multiplier: 0.75,
      health_status: 'success',
      reason_code: 'rate_health_priority',
      confidence: 'high',
      health_summary: 'probe ok',
      rate_source: 'login_user_group_rates',
      reason: '倍率更优，建议提高优先级',
      applied: false,
      created_at: '2026-06-28T12:00:00Z',
    }],
    ...overrides,
  }
}

describe('UpstreamRelayGroupMonitoringView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    listConnectors.mockReset()
    listCandidates.mockReset()
    listRecommendationRuns.mockReset()
    getRecommendationRun.mockReset()
    applyRecommendationRun.mockReset()
    closeRecommendationRun.mockReset()
    restoreRecommendationRun.mockReset()
    getMonitoringPolicy.mockReset()
    updateMonitoringPolicy.mockReset()
    getRecommendationPolicy.mockReset()
    updateRecommendationPolicy.mockReset()
    previewRecommendations.mockReset()
    generateRecommendations.mockReset()
    listSnapshots.mockReset()
    listUsageHistory.mockReset()
    refreshConnectorMetrics.mockReset()
    refreshMonitoringData.mockReset()
    getRunnerStatus.mockReset()
    getRunnerStatus.mockResolvedValue(runnerStatus())
    syncConnector.mockReset()
    syncAllConnectors.mockReset()
    probeAllCandidates.mockReset()
    createConnector.mockReset()
    updateConnector.mockReset()
    createCandidate.mockReset()
    updateCandidate.mockReset()
    listConnectorAPIKeys.mockReset()
    listConnectorAPIKeys.mockResolvedValue([])
    listAccounts.mockReset()
    listGroups.mockReset()
    login.mockReset()
    login2FA.mockReset()
    logout.mockReset()
    getCurrentUser.mockReset()
    register.mockReset()
    refreshToken.mockReset()
    reportActivity.mockReset()

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
    getRecommendationRun.mockRejectedValue(new Error('not found'))
    applyRecommendationRun.mockRejectedValue(new Error('apply failed'))
    listSnapshots.mockResolvedValue([])
    refreshMonitoringData.mockResolvedValue({
      status: 'success',
      total: 1,
      success: 1,
      partial: 0,
      failed: 0,
      refreshed_at: '2026-06-28T12:05:00Z',
      items: [{
        connector_id: 7,
        connector_name: 'relay-a',
        status: 'success',
        snapshot_status: 'success',
        snapshot_count: 0,
        snapshots: [],
        metrics: {
          connector: {
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
            updated_at: '2026-06-28T12:05:00Z',
          },
          snapshots: [],
          status: 'success',
          balance_detail: { status: 'success', checked_at: '2026-06-28T12:05:00Z' },
          usage_detail: { status: 'success', total_groups: 0, updated_groups: 0, missing_groups: [] },
          balance_available: true,
          usage_available: true,
          refreshed_at: '2026-06-28T12:05:00Z',
        },
      }],
    })
    syncConnector.mockResolvedValue([])
    listUsageHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
    })
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
    updateRecommendationPolicy.mockResolvedValue({
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
    previewRecommendations.mockResolvedValue({
      policy: {
        snapshot_freshness_minutes: 1440,
        usage_delta_freshness_minutes: 1440,
        probe_freshness_minutes: 30,
        min_success_rate: 0.5,
        min_sample_size: 3,
        exclude_consecutive_failures: true,
        priority_start: 10,
        priority_step: 10,
        sort_fields: ['rate_asc', 'success_rate_desc', 'latency_asc'],
      },
      total_candidates: 0,
      suggestion_count: 0,
      excluded_count: 0,
      suggestions: [],
      exclusions: [],
    })
    generateRecommendations.mockResolvedValue(recommendationRun())
    getMonitoringPolicy.mockResolvedValue(monitoringPolicy())
    updateMonitoringPolicy.mockResolvedValue(monitoringPolicy({ updated_at: '2026-06-28T12:10:00Z' }))
    listAccounts.mockResolvedValue({
      items: [{ id: 42, name: 'claude-relay', platform: 'claude' }],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1,
    })
    listGroups.mockResolvedValue([{ id: 9, name: 'vip', description: '', user_count: 0, created_at: '', updated_at: '' }])
    createConnector.mockResolvedValue({
      id: 9,
      name: 'relay-password',
      base_url: 'https://relay-password.example.com',
      auth_mode: 'password_login',
      status: 'active',
      credential_version: 1,
      has_bearer_token: false,
      has_refresh_token: false,
      has_login_email: true,
      has_cookie: false,
      has_user_agent: false,
      created_at: '2026-06-29T12:00:00Z',
      updated_at: '2026-06-29T12:00:00Z',
    })
    updateConnector.mockResolvedValue({
      id: 7,
      name: 'relay-a',
      base_url: 'https://relay.example.com',
      auth_mode: 'manual_session',
      status: 'active',
      credential_version: 2,
      has_bearer_token: true,
      has_refresh_token: false,
      has_login_email: false,
      has_cookie: false,
      has_user_agent: false,
      created_at: '2026-06-28T12:00:00Z',
      updated_at: '2026-06-29T12:00:00Z',
    })
    logout.mockResolvedValue(undefined)
    reportActivity.mockResolvedValue(undefined)
  })

  afterEach(async () => {
    for (const wrapper of mountedWrappers.splice(0)) {
      wrapper.unmount()
    }
    vi.useRealTimers()
    await useAuthStore().logout().catch(() => undefined)
    document.body.innerHTML = ''
  })

  it('新建连接器选择账号密码登录时保持弹窗表单打开', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    const connectorDialog = wrapper.find('[data-close-on-escape="false"]')
    expect(connectorDialog.exists()).toBe(true)

    await connectorDialog.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectorForm.authPassword'))!.trigger('click')
    await flushPromises()

    const activeDialog = wrapper.find('[data-close-on-escape="false"]')
    expect(activeDialog.exists()).toBe(true)
    expect(activeDialog.attributes('data-close-on-click-outside')).toBe('false')
    expect(activeDialog.attributes('data-show-close-button')).toBe('false')
    expect(activeDialog.attributes('data-animated')).toBe('false')

    await activeDialog.get('[data-testid="implicit-close"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-close-on-escape="false"]').exists()).toBe(true)
    expect(wrapper.find('input[type="email"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"][autocomplete="current-password"]').exists()).toBe(true)

    const emailInput = wrapper.find('input[type="email"]')
    const passwordInput = wrapper.find('input[type="password"][autocomplete="current-password"]')
    await emailInput.trigger('pointerdown')
    await emailInput.trigger('mousedown')
    await emailInput.setValue('relay@example.com')
    await emailInput.trigger('change')
    await passwordInput.trigger('pointerdown')
    await passwordInput.trigger('mousedown')
    await passwordInput.setValue('secret-value')
    await passwordInput.trigger('change')

    expect(wrapper.find('[data-close-on-escape="false"]').exists()).toBe(true)
  })

  it('真实弹窗中新建连接器点击账号密码不会触发关闭', async () => {
    const wrapper = mountViewWithRealDialog()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()

    const authPasswordButton = Array.from(document.body.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('admin.upstreamRelayGroupMonitoring.connectorForm.authPassword')
    ) as HTMLButtonElement
    expect(authPasswordButton).toBeTruthy()

    authPasswordButton.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    await flushPromises()
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(document.body.querySelector('input[autocomplete="username"]')).toBeNull()

    authPasswordButton.dispatchEvent(new Event('pointerup', { bubbles: true }))
    authPasswordButton.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    authPasswordButton.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }))
    authPasswordButton.click()
    await flushPromises()

    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(document.body.querySelector('input[type="email"]')).not.toBeNull()
    const emailInput = document.body.querySelector('input[type="email"]') as HTMLInputElement
    const passwordInput = document.body.querySelector('input[type="password"][autocomplete="current-password"]') as HTMLInputElement
    expect(emailInput).not.toBeNull()
    expect(passwordInput).not.toBeNull()

    emailInput.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    emailInput.dispatchEvent(new Event('pointerup', { bubbles: true }))
    emailInput.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    emailInput.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }))
    emailInput.value = 'relay@example.com'
    emailInput.dispatchEvent(new Event('input', { bubbles: true }))
    emailInput.dispatchEvent(new Event('change', { bubbles: true }))
    passwordInput.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    passwordInput.dispatchEvent(new Event('pointerup', { bubbles: true }))
    passwordInput.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    passwordInput.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }))
    passwordInput.value = 'secret-value'
    passwordInput.dispatchEvent(new Event('input', { bubbles: true }))
    passwordInput.dispatchEvent(new Event('change', { bubbles: true }))
    await flushPromises()

    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(createConnector).not.toHaveBeenCalled()
  })

  it('真实弹窗中账号密码触摸事件链不会触发关闭', async () => {
    const wrapper = mountViewWithRealDialog()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    const authPasswordButton = Array.from(document.body.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('admin.upstreamRelayGroupMonitoring.connectorForm.authPassword')
    ) as HTMLButtonElement
    expect(authPasswordButton).toBeTruthy()

    authPasswordButton.dispatchEvent(new Event('touchstart', { bubbles: true }))
    authPasswordButton.dispatchEvent(new Event('touchend', { bubbles: true }))
    await flushPromises()
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(document.body.querySelector('input[type="email"]')).toBeNull()

    authPasswordButton.click()
    await flushPromises()

    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(document.body.querySelector('input[type="email"]')).not.toBeNull()
    expect(document.body.querySelector('input[type="password"][autocomplete="current-password"]')).not.toBeNull()
    expect(createConnector).not.toHaveBeenCalled()
  })

  it('真实弹窗账号密码事件不会触发全局前台活跃上报', async () => {
    login.mockResolvedValue({
      access_token: 'test-token-123',
      refresh_token: 'refresh-token-456',
      expires_in: 3600,
      token_type: 'Bearer',
      user: {
        id: 1,
        username: 'admin',
        email: 'admin@example.com',
        role: 'admin',
        balance: 0,
        concurrency: 1,
        status: 'active',
        allowed_groups: null,
        created_at: '2026-06-29T12:00:00Z',
        updated_at: '2026-06-29T12:00:00Z',
      },
    })
    const authStore = useAuthStore()
    await authStore.login({ email: 'admin@example.com', password: 'secret-value' })
    reportActivity.mockClear()

    const wrapper = mountViewWithRealDialog()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    const authPasswordButton = Array.from(document.body.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('admin.upstreamRelayGroupMonitoring.connectorForm.authPassword')
    ) as HTMLButtonElement
    expect(authPasswordButton).toBeTruthy()

    authPasswordButton.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    authPasswordButton.dispatchEvent(new Event('touchstart', { bubbles: true }))
    authPasswordButton.click()
    await flushPromises()

    expect(reportActivity).not.toHaveBeenCalled()
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(document.body.querySelector('input[type="email"]')).not.toBeNull()
    expect(createConnector).not.toHaveBeenCalled()
  })

  it('新建手动会话连接器会提交 refresh token', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs.find((input) => input.attributes('type') === 'text')!.setValue('relay-manual')
    await inputs.find((input) => input.attributes('type') === 'url')!.setValue('https://relay.example.com')
    const passwordInputs = wrapper.findAll('input[type="password"]')
    await passwordInputs[0].setValue('manual-access-token')
    await passwordInputs[1].setValue('manual-refresh-token')

    await wrapper.find('form#connector-form').trigger('submit')
    await flushPromises()

    expect(createConnector).toHaveBeenCalledWith(expect.objectContaining({
      name: 'relay-manual',
      base_url: 'https://relay.example.com',
      auth_mode: 'manual_session',
      bearer_token: 'manual-access-token',
      refresh_token: 'manual-refresh-token',
    }))
  })

  it('保存连接器失败时在弹窗内展示接口详细原因', async () => {
    createConnector.mockRejectedValueOnce({
      status: 400,
      code: 'UPSTREAM_RELAY_PASSWORD_LOGIN_NEEDS_MANUAL_SESSION',
      message: 'upstream login requires browser verification or 2FA; use manual_session instead',
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.newConnector'))!.trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs.find((input) => input.attributes('type') === 'text')!.setValue('relay-manual')
    await inputs.find((input) => input.attributes('type') === 'url')!.setValue('https://relay.example.com')
    await wrapper.find('input[type="password"]').setValue('manual-access-token')

    await wrapper.find('form#connector-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    const formError = wrapper.get('[data-testid="connector-form-error"]')
    expect(formError.text()).toContain('admin.upstreamRelayGroupMonitoring.connectorForm.saveFailedTitle')
    expect(formError.text()).toContain('admin.upstreamRelayGroupMonitoring.connectorForm.passwordLoginNeedsManualSession.reason')
    expect(formError.text()).toContain('admin.upstreamRelayGroupMonitoring.connectorForm.passwordLoginNeedsManualSession.action')
    expect(formError.text()).toContain('upstream login requires browser verification or 2FA; use manual_session instead')
    expect(wrapper.find('[data-close-on-escape="false"]').exists()).toBe(true)
  })

  it('编辑连接器点击账号密码也保持弹窗打开', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.edit'))!.trigger('click')
    await flushPromises()

    const connectorDialog = wrapper.find('[data-close-on-escape="false"]')
    expect(connectorDialog.exists()).toBe(true)

    await connectorDialog.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectorForm.authPassword'))!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-close-on-escape="false"]').exists()).toBe(true)
    expect(wrapper.find('input[type="email"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"][autocomplete="current-password"]').exists()).toBe(true)
  })

  it('编辑手动会话连接器可显式清空已保存的 refresh token', async () => {
    listConnectors.mockResolvedValueOnce({
      items: [{
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        has_bearer_token: true,
        has_refresh_token: true,
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
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.edit'))!.trigger('click')
    await flushPromises()

    const clearRefreshCheckbox = wrapper.find('form#connector-form input[type="checkbox"]')
    expect(clearRefreshCheckbox.exists()).toBe(true)
    await clearRefreshCheckbox.setValue(true)
    await wrapper.find('form#connector-form').trigger('submit')
    await flushPromises()

    expect(updateConnector).toHaveBeenCalledWith(7, expect.objectContaining({
      name: 'relay-a',
      base_url: 'https://relay.example.com',
      auth_mode: 'manual_session',
      refresh_token: '',
    }))
  })

  it('初始加载会拉取连接器和候选映射的后续分页，避免概览静默截断', async () => {
    listConnectors
      .mockResolvedValueOnce({
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
        total: 2,
        page: 1,
        page_size: 100,
        pages: 2,
      })
      .mockResolvedValueOnce({
        items: [{
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
        }],
        total: 2,
        page: 2,
        page_size: 100,
        pages: 2,
      })
    listCandidates
      .mockResolvedValueOnce({
        items: [{
          id: 101,
          connector_id: 7,
          account_id: 42,
          upstream_group_id: 'team-a',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: true,
          notes: '',
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        }],
        total: 2,
        page: 1,
        page_size: 100,
        pages: 2,
      })
      .mockResolvedValueOnce({
        items: [{
          id: 102,
          connector_id: 8,
          account_id: 42,
          upstream_group_id: 'team-b',
          probe_model: 'gpt-4o-mini',
          probe_protocol: 'chat_completions',
          enabled: false,
          notes: '',
          created_at: '2026-06-28T12:00:00Z',
          updated_at: '2026-06-28T12:00:00Z',
        }],
        total: 2,
        page: 2,
        page_size: 100,
        pages: 2,
      })

    const wrapper = mountView()
    await flushPromises()

    expect(listConnectors).toHaveBeenCalledWith({ page: 1, page_size: 100 })
    expect(listConnectors).toHaveBeenCalledWith({ page: 2, page_size: 100 })
    expect(listCandidates).toHaveBeenCalledWith({ page: 1, page_size: 100 })
    expect(listCandidates).toHaveBeenCalledWith({ page: 2, page_size: 100 })
    const overview = wrapper.findAll('section')[0]
    expect(overview.text()).toContain('1 / 2')
    expect(overview.text()).toContain('1 / 2')
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
    listUsageHistory.mockResolvedValue({
      items: [
        {
          id: 88,
          usage_date: '2026-06-29',
          connector_id: 7,
          connector_name: 'relay-a',
          upstream_group_id: 'team-a',
          group_name: 'Team A',
          platform: 'claude',
          actual_cost: 1.25,
          total_tokens: 1_250_000,
          checked_at: '2026-06-29T12:00:00Z',
        },
        {
          id: 89,
          usage_date: '2026-06-29',
          connector_id: 7,
          connector_name: 'relay-a',
          upstream_group_id: 'team-extra',
          group_name: 'Team Extra',
          platform: 'claude',
          actual_cost: 0.5,
          total_tokens: 500_000,
          checked_at: '2026-06-29T12:00:00Z',
        },
        {
          id: 90,
          usage_date: '2026-06-29',
          connector_id: 8,
          connector_name: 'relay-b',
          upstream_group_id: 'team-b',
          group_name: 'Team B',
          platform: 'claude',
          actual_cost: 2,
          total_tokens: 1_500_000,
          checked_at: '2026-06-29T12:00:00Z',
        },
      ],
      total: 3,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 3.75, total_tokens: 3_250_000, connector_count: 2, group_count: 3, latest_checked_at: '2026-06-29T12:00:00Z', pending_finalize: 0 },
    })

    const wrapper = mountView()
    await flushPromises()

    const overview = wrapper.findAll('section')[0]
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
    expect(wrapper.text()).toContain('actionGroups.data')
    expect(wrapper.text()).toContain('actionGroups.recommendation')
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

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
    await flushPromises()

    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="metrics-refresh-guidance"]').text()).toContain('metricsRefresh.guidance.syncConnector')
    expect(wrapper.find('[data-testid="metrics-refresh-guidance"]').text()).toContain('metricsRefresh.actions.syncConnector')
  })

  it('快照弹窗内可手动拉取当前连接器倍率快照', async () => {
    listSnapshots.mockResolvedValueOnce([])
    syncConnector.mockResolvedValueOnce([
      {
        id: 501,
        connector_id: 7,
        upstream_group_id: 'team-alpha',
        name: 'Team Alpha',
        platform: 'claude',
        status: 'active',
        default_rate_multiplier: 1,
        final_rate_multiplier: 0.75,
        source: 'login_user_group_rates',
        last_seen_at: '2026-06-30T12:00:00Z',
      },
    ])

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.connectors.snapshots'))!.trigger('click')
    await flushPromises()

    const fetchButton = wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.snapshotDialog.fetchLatest'))!
    await fetchButton.trigger('click')
    await flushPromises()

    expect(syncConnector).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.text()).toContain('team-alpha')
    expect(wrapper.find('[data-testid="page-success"]').text()).toContain('admin.upstreamRelayGroupMonitoring.snapshotDialog.fetchSuccess')
  })

  it('拉取倍率快照按钮仅显示在倍率快照 tab 操作区', async () => {
    syncAllConnectors.mockResolvedValueOnce({
      total: 1,
      success: 1,
      failed: 0,
      items: [{ id: 7, connector_id: 7, connector_name: 'relay-a', success: true, count: 1 }],
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((button) =>
      button.text().includes('admin.upstreamRelayGroupMonitoring.fetchSnapshots')
    )).toBe(false)

    await wrapper.findAll('button').find((button) =>
      button.text().includes('admin.upstreamRelayGroupMonitoring.tabs.snapshotChanges')
    )!.trigger('click')
    await flushPromises()

    const fetchButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.upstreamRelayGroupMonitoring.fetchSnapshots')
    )!
    expect(fetchButton.exists()).toBe(true)

    await fetchButton.trigger('click')
    await flushPromises()

    expect(syncAllConnectors).toHaveBeenCalled()
    const resultPanel = wrapper.find('[data-testid="operation-result-panel"]')
    expect(resultPanel.text()).toContain('operationResult.titles.sync')
    expect(resultPanel.text()).toContain('operationResult.results.bulk')
  })

  it('历史用量页按日期加载并展示每日分组汇总', async () => {
    const freshCheckedAt = new Date().toISOString()
    listUsageHistory.mockResolvedValue({
      items: [
        {
          id: 88,
          usage_date: '2026-06-28',
          connector_id: 7,
          connector_name: 'relay-a',
          upstream_group_id: 'team-a',
          group_name: 'Team A',
          platform: 'openai',
          actual_cost: 2.5,
          total_tokens: 1500000,
          checked_at: freshCheckedAt,
        },
        {
          id: 89,
          usage_date: '2026-06-28',
          connector_id: 7,
          connector_name: 'relay-a',
          upstream_group_id: 'team-b',
          group_name: 'Team B',
          platform: 'openai',
          actual_cost: 1.25,
          total_tokens: 0,
          checked_at: '2000-01-01T00:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 3.75, total_tokens: 1500000, connector_count: 1, group_count: 2, latest_checked_at: freshCheckedAt, pending_finalize: 2 },
    })
    const wrapper = mountView()
    await flushPromises()

    const usageHistoryTab = wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!
    await usageHistoryTab.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls()).toContainEqual(expect.objectContaining({
      page: 1,
      page_size: 50,
      start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
    }))
    expect(usageHistoryTabCalls().at(-1)?.include_zero_usage).toBeUndefined()
    expect(wrapper.text()).toContain('relay-a')
    expect(wrapper.text()).toContain('Team A')
    expect(wrapper.text()).toContain('Team B')
    expect(wrapper.text()).toContain('usageHistory.startDate')
    expect(wrapper.text()).toContain('usageHistory.endDate')
    expect(wrapper.text()).toContain('usageHistory.connector')
    expect(wrapper.text()).toContain('usageHistory.groupId')
    expect(wrapper.text()).toContain('usageHistory.keyword')
    expect(wrapper.text()).toContain('usageHistory.includeZeroUsage')
    expect(wrapper.text()).toContain('usageHistory.onlyAnomalies')
    expect(wrapper.text()).toContain('usageHistory.summaryCost')
    expect(wrapper.text()).toContain('usageHistory.summaryTokens')
    expect(wrapper.text()).toContain('usageHistory.summaryConnectors')
    expect(wrapper.text()).toContain('usageHistory.summaryGroups')
    expect(wrapper.text()).toContain('usageHistory.summaryCostPerMillion')
    expect(wrapper.text()).toContain('usageHistory.summaryLatestCheckedAt')
    expect(wrapper.text()).toContain('usageHistory.summaryPendingFinalize')
    expect(wrapper.text()).toContain('usageHistory.appliedFilterSummary')
    expect(wrapper.text()).toContain('usageHistory.pendingFinalizeHint')
    expect(wrapper.text()).toContain('usageHistory.currentPageScopeHint')
    expect(wrapper.text()).toContain('usageHistory.shortcutToday')
    expect(wrapper.text()).toContain('usageHistory.shortcutLast7d')
    expect(wrapper.text()).toContain('usageHistory.resetFilters')
    expect(wrapper.text()).toContain('usageHistory.groupSubtotalCost')
    expect(wrapper.text()).toContain('usageHistory.groupSubtotalTokens')
    expect(wrapper.text()).toContain('usageHistory.groupSubtotalGroups')
    expect(wrapper.text()).toContain('usageHistory.groupLatestCheckedAt')
    expect(wrapper.text()).toContain('usageHistory.flagCostWithoutTokens')
    expect(wrapper.text()).toContain('usageHistory.badgePendingFinalize')
    expect(wrapper.text()).toContain('usageHistory.manualFinalizeButton')
    expect(wrapper.text()).toContain('$3.75')
    expect(wrapper.text()).toContain('$2.50')
    expect(wrapper.text()).toContain('1.50M')

    const anomaliesToggle = wrapper.findAll('label').find((label) => label.text().includes('usageHistory.onlyAnomalies'))!.find('input')
    await anomaliesToggle.setValue(true)
    await flushPromises()

    expect(wrapper.text()).not.toContain('Team A')
    expect(wrapper.text()).toContain('Team B')
  })

  it('历史用量默认隐藏无消费分组，开启开关后包含无消费分组', async () => {
    listUsageHistory.mockImplementation((params = {}) => Promise.resolve({
      items: params.include_zero_usage
        ? [{
            id: 90,
            usage_date: '2026-06-28',
            connector_id: 7,
            connector_name: 'relay-a',
            upstream_group_id: 'team-zero',
            group_name: 'Team Zero',
            platform: 'openai',
            actual_cost: 0,
            total_tokens: 0,
            checked_at: '2026-06-28T12:00:00Z',
          }]
        : [],
      total: params.include_zero_usage ? 1 : 0,
      page: 1,
      page_size: params.page_size || 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: params.include_zero_usage ? 1 : 0, group_count: params.include_zero_usage ? 1 : 0, latest_checked_at: null, pending_finalize: 0 },
    }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls().at(-1)?.include_zero_usage).toBeUndefined()
    expect(wrapper.text()).not.toContain('Team Zero')

    const includeZeroToggle = wrapper.findAll('label').find((label) => label.text().includes('usageHistory.includeZeroUsage'))!.find('input')
    await includeZeroToggle.setValue(true)
    await flushPromises()
    expect(wrapper.text()).toContain('usageHistory.filtersPending')

    await wrapper.findAll('button').find((button) => button.text().includes('usageHistory.applyFilters'))!.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls().at(-1)).toEqual(expect.objectContaining({ include_zero_usage: true }))
    expect(wrapper.text()).toContain('Team Zero')
  })

  it('历史用量在后端页数字段异常时仍可按总数进入下一页', async () => {
    const firstPageItem = {
      id: 88,
      usage_date: '2026-06-28',
      connector_id: 7,
      connector_name: 'relay-a',
      upstream_group_id: 'team-a',
      group_name: 'Team A',
      platform: 'openai',
      actual_cost: 2.5,
      total_tokens: 1500000,
      checked_at: '2026-06-28T12:00:00Z',
    }
    const secondPageItem = {
      ...firstPageItem,
      id: 139,
      connector_name: 'relay-b',
      upstream_group_id: 'team-b',
      group_name: 'Team B',
    }
    listUsageHistory.mockImplementation((params = {}) => {
      if (params.page_size === 200) {
        return Promise.resolve({
          items: [],
          total: 0,
          page: 1,
          page_size: 200,
          pages: 1,
          summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
        })
      }
      if (params.page === 2) {
        return Promise.resolve({
          items: [secondPageItem],
          total: 75,
          page: 2,
          page_size: 50,
          pages: 1,
          summary: { total_cost: 3.75, total_tokens: 1500000, connector_count: 2, group_count: 75, latest_checked_at: firstPageItem.checked_at, pending_finalize: 0 },
        })
      }
      return Promise.resolve({
        items: [firstPageItem],
        total: 75,
        page: 1,
        page_size: 50,
        pages: 1,
        summary: { total_cost: 3.75, total_tokens: 1500000, connector_count: 2, group_count: 75, latest_checked_at: firstPageItem.checked_at, pending_finalize: 0 },
      })
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()

    const nextButton = wrapper.findAll('button').find((button) => button.text().includes('usageHistory.next'))!
    expect(nextButton.attributes('disabled')).toBeUndefined()

    await nextButton.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls()).toContainEqual(expect.objectContaining({ page: 2, page_size: 50 }))
    expect(wrapper.text()).toContain('relay-b')
  })

  it('历史用量快捷日期会写入日期范围并立即查询', async () => {
    listUsageHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()
    expect(usageHistoryTabCalls()).toHaveLength(1)

    await wrapper.findAll('button').find((button) => button.text().includes('usageHistory.shortcutLast7d'))!.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls()).toHaveLength(2)
    const latestParams = usageHistoryTabCalls().at(-1)!
    expect(latestParams).toEqual(expect.objectContaining({
      page: 1,
      page_size: 50,
      start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
    }))
    expect(latestParams.start_date).not.toBe(latestParams.end_date)

    await wrapper.findAll('button').find((button) => button.text().includes('usageHistory.resetFilters'))!.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls()).toHaveLength(3)
    const resetParams = usageHistoryTabCalls().at(-1)!
    expect(resetParams.start_date).toBe(resetParams.end_date)
    expect(resetParams.connector_id).toBeUndefined()
    expect(resetParams.search).toBeUndefined()
  })

  it('历史用量日期使用 Asia/Shanghai 业务日而不是浏览器本地日期', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-28T16:30:00.000Z'))
    const realDateTimeFormat = Intl.DateTimeFormat
    const dateTimeFormatSpy = vi.spyOn(Intl, 'DateTimeFormat')
    dateTimeFormatSpy.mockImplementation(((...args: ConstructorParameters<typeof Intl.DateTimeFormat>) => new realDateTimeFormat(...args)) as typeof Intl.DateTimeFormat)
    listUsageHistory.mockResolvedValue({
      items: [
        {
          id: 88,
          usage_date: '2026-06-29',
          connector_id: 7,
          connector_name: 'relay-a',
          upstream_group_id: 'team-a',
          group_name: 'Team A',
          platform: 'openai',
          actual_cost: 2.5,
          total_tokens: 1500000,
          checked_at: '2026-06-29T10:00:00+08:00',
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 2.5, total_tokens: 1500000, connector_count: 1, group_count: 1, latest_checked_at: '2026-06-29T10:00:00+08:00', pending_finalize: 0 },
    })

    try {
      const wrapper = mountView()
      await flushPromises()

      await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
      await flushPromises()

      const initialParams = usageHistoryTabCalls().at(-1)!
      expect(initialParams.start_date).toBe('2026-06-29')
      expect(initialParams.end_date).toBe('2026-06-29')
      expect(wrapper.text()).toContain('usageHistory.badgeLive')
      expect(dateTimeFormatSpy.mock.calls.some(([, options]) => options?.timeZone === 'Asia/Shanghai')).toBe(true)

      await wrapper.findAll('button').find((button) => button.text().includes('usageHistory.shortcutYesterday'))!.trigger('click')
      await flushPromises()

      const yesterdayParams = usageHistoryTabCalls().at(-1)!
      expect(yesterdayParams.start_date).toBe('2026-06-28')
      expect(yesterdayParams.end_date).toBe('2026-06-28')
    } finally {
      dateTimeFormatSpy.mockRestore()
    }
  })

  it('历史用量日期范围非法时提示并阻止查询', async () => {
    listUsageHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()
    expect(usageHistoryTabCalls()).toHaveLength(1)

    const dateInputs = wrapper.findAll('input[type="date"]')
    await dateInputs[0].setValue('2026-06-30')
    await dateInputs[1].setValue('2026-06-01')
    await flushPromises()

    expect(wrapper.text()).toContain('usageHistory.invalidDateRange')
    const applyButton = wrapper.findAll('button').find((button) => button.text().includes('usageHistory.applyFilters'))!
    expect(applyButton.attributes('disabled')).toBeDefined()
    await applyButton.trigger('click')
    await flushPromises()
    expect(usageHistoryTabCalls()).toHaveLength(1)
  })

  it('历史用量筛选变更后只标记待应用，点击查询后才重新加载', async () => {
    listUsageHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()
    expect(usageHistoryTabCalls()).toHaveLength(1)

    const controls = wrapper.findAll('input, select')
    await controls.find((input) => input.attributes('type') === 'date')!.setValue('2026-06-27')
    await flushPromises()

    expect(usageHistoryTabCalls()).toHaveLength(1)
    expect(wrapper.text()).toContain('usageHistory.filtersPending')
    expect(wrapper.text()).toContain('usageHistory.applyFilters')
    expect(wrapper.text()).toContain('usageHistory.resultUsesAppliedFilters')

    await wrapper.findAll('button').find((button) => button.text().includes('usageHistory.applyFilters'))!.trigger('click')
    await flushPromises()

    expect(usageHistoryTabCalls()).toHaveLength(2)
    expect(wrapper.text()).not.toContain('usageHistory.filtersPending')
  })

  it('历史用量加载时展示骨架状态而不是整页 spinner', async () => {
    let resolveUsageHistory: (value: { items: unknown[]; total: number; page: number; page_size: number; pages: number }) => void
    listUsageHistory
      .mockResolvedValueOnce({ items: [], total: 0, page: 1, page_size: 50, pages: 1 })
      .mockReturnValueOnce(new Promise((resolve) => {
        resolveUsageHistory = resolve
      }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()

    const skeleton = wrapper.find('[data-testid="usage-history-skeleton"]')
    expect(skeleton.exists()).toBe(true)
    expect(skeleton.attributes('aria-label')).toContain('usageHistory.loading')
    expect(skeleton.findAll('.skeleton').length).toBeGreaterThan(0)

    resolveUsageHistory!({ items: [], total: 0, page: 1, page_size: 50, pages: 1 })
    await flushPromises()
    expect(wrapper.find('[data-testid="usage-history-skeleton"]').exists()).toBe(false)
  })

  it('历史用量页手动刷新用量余额后重载当前历史列表', async () => {
    listUsageHistory
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 50,
        pages: 1,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 88,
            usage_date: '2026-06-28',
            connector_id: 7,
            connector_name: 'relay-a',
            upstream_group_id: 'team-a',
            group_name: 'Team A',
            platform: 'openai',
            actual_cost: 2.5,
            total_tokens: 1500000,
            checked_at: '2026-06-28T15:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 50,
        pages: 1,
      })
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 50,
        pages: 1,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 89,
            usage_date: '2026-06-28',
            connector_id: 7,
            connector_name: 'relay-a',
            upstream_group_id: 'team-a',
            group_name: 'Team A',
            platform: 'openai',
            actual_cost: 3.75,
            total_tokens: 2500000,
            checked_at: '2026-06-28T15:05:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 50,
        pages: 1,
      })
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
      snapshots: [{ id: 1, connector_id: 7, upstream_group_id: 'team-a', name: 'Team A', platform: 'claude', status: 'active', default_rate_multiplier: 1, final_rate_multiplier: 1, source: 'login_available_groups', last_seen_at: '2026-06-28T12:00:00Z' }],
      status: 'success',
      balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: { status: 'success', total_groups: 1, updated_groups: 1, missing_groups: [] },
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('$2.50')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    expect(refreshMonitoringData).toHaveBeenCalled()
    expect(refreshConnectorMetrics).not.toHaveBeenCalled()
    expect(usageHistoryTabCalls()).toHaveLength(2)
    expect(wrapper.text()).toContain('$3.75')
    expect(wrapper.text()).toContain('2.50M')
  })

  it('历史用量筛选待应用时刷新用量余额不会用旧筛选悄悄重载列表', async () => {
    listUsageHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: { total_cost: 0, total_tokens: 0, connector_count: 0, group_count: 0, latest_checked_at: null, pending_finalize: 0 },
    })
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
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
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'success',
      balance_detail: { status: 'success', checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: { status: 'success', total_groups: 0, updated_groups: 0, missing_groups: [] },
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.usageHistory'))!.trigger('click')
    await flushPromises()
    expect(usageHistoryTabCalls()).toHaveLength(1)

    const controls = wrapper.findAll('input, select')
    await controls.find((input) => input.attributes('type') === 'date')!.setValue('2026-06-27')
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    expect(refreshMonitoringData).toHaveBeenCalled()
    expect(refreshConnectorMetrics).not.toHaveBeenCalled()
    expect(usageHistoryTabCalls()).toHaveLength(1)
    expect(wrapper.text()).toContain('usageHistory.resultUsesAppliedFilters')
  })

  it('连接器行可单独轻量刷新当前连接器并合并刷新摘要', async () => {
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
      snapshots: [{ id: 1, connector_id: 7, upstream_group_id: 'team-a', name: 'Team A', platform: 'claude', status: 'active', default_rate_multiplier: 1, final_rate_multiplier: 1, source: 'login_available_groups', last_seen_at: '2026-06-28T12:00:00Z' }],
      status: 'success',
      balance_detail: { status: 'success', value: 12.34, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: { status: 'success', total_groups: 1, updated_groups: 1, missing_groups: [] },
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:05:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
    await flushPromises()

    expect(refreshConnectorMetrics).toHaveBeenCalledTimes(1)
    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(summary.exists()).toBe(true)
    await wrapper.findAll('button').find((button) => button.text().includes('metricsRefresh.showDetails'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').text()).toContain('relay-a')
    expect(wrapper.text()).toContain('$12.34')
  })

  it('轻量刷新接口失败时展示处理建议并把原始原因收进技术详情', async () => {
    refreshConnectorMetrics.mockRejectedValueOnce(new Error('network failed'))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    expect(summary.exists()).toBe(true)
    expect(wrapper.find('[data-testid="metrics-refresh-details"]').exists()).toBe(true)
    expect(summary.text()).toContain('relay-a')
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.upstreamRelayGroupMonitoring.errors.unknownUpstreamError')
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
    refreshMonitoringData.mockResolvedValueOnce(monitoringRefreshResultFromMetrics([
      {
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
      },
      {
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
      },
    ]))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    expect(refreshMonitoringData).toHaveBeenCalled()
    expect(refreshConnectorMetrics).not.toHaveBeenCalled()
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

  it('Usage 缺少上游 API Key 时展示账号定位、分组差异和直接修复入口', async () => {
    const rawError = 'candidate 7 for bound account 122 has no upstream api key binding'
    listConnectorAPIKeys.mockResolvedValue([{ id: 855, name: 'repair-key', masked_key: 'sk-***' }])
    listAccounts.mockResolvedValue({
      items: [{ id: 122, name: 'smart-account', platform: 'openai' }],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1,
    })
    listCandidates.mockResolvedValue({
      items: [{
        id: 7,
        connector_id: 7,
        connector_name: '聪明',
        account_id: 122,
        account_name: 'smart-account',
        account_platform: 'openai',
        upstream_group_id: 'fast',
        upstream_group_name: '快速稳定分组1（倍率上限0.1）',
        upstream_api_key_id: null,
        upstream_api_key_name: '',
        upstream_api_key_masked: '',
        probe_model: 'gpt-4o-mini',
        probe_protocol: 'chat_completions',
        enabled: true,
        notes: '',
      }],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    refreshMonitoringData.mockResolvedValue(monitoringRefreshResultFromMetrics([{
      connector: {
        id: 7,
        name: '聪明',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'active',
        credential_version: 1,
        upstream_account_balance: 38.6393,
        upstream_account_balance_checked_at: '2026-06-28T12:05:00Z',
        has_bearer_token: true,
        has_refresh_token: false,
        has_login_email: false,
        has_cookie: false,
        has_user_agent: false,
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [
        { id: 1, connector_id: 7, upstream_group_id: 'fast', name: '快速稳定分组1（倍率上限0.1）' },
        { id: 2, connector_id: 7, upstream_group_id: 'pro', name: 'pro分组' },
      ],
      status: 'partial',
      balance_detail: { status: 'success', value: 38.6393, checked_at: '2026-06-28T12:05:00Z' },
      usage_detail: {
        status: 'partial',
        total_groups: 2,
        updated_groups: 1,
        missing_groups: [
          { upstream_group_id: 'fast', name: '快速稳定分组1（倍率上限0.1）', reason: 'missing_upstream_api_key_binding', message: rawError },
        ],
        error: rawError,
        issue: {
          code: 'missing_upstream_api_key_binding',
          message: rawError,
          candidate_id: 7,
          account_id: 122,
          upstream_group_id: 'fast',
        },
        issues: [{
          code: 'missing_upstream_api_key_binding',
          message: rawError,
          candidate_id: 7,
          account_id: 122,
          upstream_group_id: 'fast',
        }],
      },
      balance_available: true,
      usage_available: false,
      usage_error: rawError,
      refreshed_at: '2026-06-28T12:05:00Z',
    }]))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('candidates.identity 7 122')
    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    const summary = wrapper.find('[data-testid="metrics-refresh-summary"]')
    const guidance = wrapper.find('[data-testid="metrics-refresh-guidance"]')
    expect(guidance.text()).toContain('metricsRefresh.issues.missingApiKeyBinding #122')
    expect(summary.text()).toContain('metricsRefresh.missingReasons.missing_upstream_api_key_binding')
    expect(summary.text()).toContain('metricsRefresh.usagePartial 1 2 1')
    expect(guidance.text()).toContain('metricsRefresh.guidance.bindApiKey')
    expect(guidance.text()).toContain('metricsRefresh.actions.bindApiKey')
    expect(guidance.find('details').text()).toContain(rawError)
    expect(summary.text().split(rawError)).toHaveLength(2)
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)

    await guidance.find('button').trigger('click')
    await flushPromises()
    expect(listConnectorAPIKeys).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('candidateForm.updateCandidate')

    updateCandidate.mockResolvedValue({
      id: 7,
      connector_id: 7,
      account_id: 122,
      upstream_group_id: 'fast',
      upstream_api_key_id: 855,
      probe_model: 'gpt-4o-mini',
      probe_protocol: 'chat_completions',
      enabled: true,
      notes: '',
    })
    refreshConnectorMetrics.mockResolvedValue({
      connector: {
        id: 7,
        name: '聪明',
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
        updated_at: '2026-06-28T12:05:00Z',
      },
      snapshots: [],
      status: 'success',
      balance_detail: { status: 'success', value: 38.6393 },
      usage_detail: { status: 'success', total_groups: 2, updated_groups: 2, missing_groups: [] },
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:06:00Z',
    })
    const candidateForm = wrapper.get('#candidate-form')
    const candidateSelects = candidateForm.findAll('select')
    await candidateSelects[0]!.setValue('7')
    await candidateSelects[1]!.setValue('122')
    await candidateSelects[2]!.setValue('fast')
    await candidateSelects[3]!.setValue('855')
    await candidateForm.find('input[type="text"]').setValue('gpt-4o-mini')
    await candidateForm.trigger('submit')
    await flushPromises()

    expect(updateCandidate).toHaveBeenCalledWith(7, expect.objectContaining({ upstream_api_key_id: 855 }))
    expect(refreshConnectorMetrics).toHaveBeenCalledWith(7)
    expect(wrapper.find('[data-testid="monitoring-operation-result"]').text()).toContain('operationResult.results.monitoring')
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
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
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
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
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
    await wrapper.findAll('button').find((button) => button.text().includes('connectors.refreshMetrics'))!.trigger('click')
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
    expect(expansion.text()).toContain('admin.upstreamRelayGroupMonitoring.candidates.colPriority')
    expect(expansion.text()).toContain('20')
    expect(expansion.text()).toContain('$1.25')
    expect(expansion.text()).toContain('0.00M')
    expect(expansion.text()).not.toContain('同步于')
    expect(expansion.text()).not.toContain('2026-06-28T12:05:00Z')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.connectors.colName')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.edit')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.probe')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.candidates.delete')
  })

  it('连接器展开区展示未绑定候选的上游倍率快照', async () => {
    listSnapshots.mockResolvedValue([{
      id: 501,
      connector_id: 7,
      upstream_group_id: 'team-alpha',
      name: 'Team Alpha',
      platform: 'claude',
      status: 'active',
      default_rate_multiplier: 1,
      final_rate_multiplier: 1.75,
      today_actual_cost: 2.5,
      today_total_tokens: 2_500_000,
      source: 'login_available_groups',
      last_seen_at: '2026-06-28T12:00:00Z',
    }])

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
    expect(expansion.text()).toContain('1.75')
    expect(expansion.text()).toContain('$2.50')
    expect(expansion.text()).toContain('2.50M')
    expect(expansion.text()).toContain('admin.upstreamRelayGroupMonitoring.connectors.notBoundCandidate')
    expect(expansion.text()).toContain('admin.upstreamRelayGroupMonitoring.connectors.createCandidate')
    expect(expansion.text()).not.toContain('admin.upstreamRelayGroupMonitoring.connectors.noGroups')
  })

  it('自动刷新有连接器但没有 active connector 时仍调用统一监控刷新', async () => {
    listConnectors.mockResolvedValueOnce({
      items: [{
        id: 7,
        name: 'relay-a',
        base_url: 'https://relay.example.com',
        auth_mode: 'manual_session',
        status: 'needs_reauth',
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
    vi.useFakeTimers()
    const wrapper = mountView()
    await flushPromises()
    refreshMonitoringData.mockClear()
    refreshConnectorMetrics.mockClear()

    const autoRefresh = wrapper.findComponent({ name: 'AutoRefreshButton' })
    autoRefresh.vm.$emit('update:interval', 15)
    autoRefresh.vm.$emit('update:enabled', true)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()

    expect(refreshMonitoringData).toHaveBeenCalledTimes(1)
    expect(refreshConnectorMetrics).not.toHaveBeenCalled()
  })

  it('统一刷新快照同步成功且返回空 snapshots 时清空该连接器旧概览快照', async () => {
    listSnapshots.mockResolvedValue([{
      id: 501,
      connector_id: 7,
      upstream_group_id: 'team-alpha',
      name: 'Team Alpha',
      platform: 'claude',
      status: 'active',
      default_rate_multiplier: 1,
      final_rate_multiplier: 1.75,
      source: 'login_available_groups',
      last_seen_at: '2026-06-28T12:00:00Z',
    }])
    refreshMonitoringData.mockResolvedValueOnce({
      status: 'success',
      total: 1,
      success: 1,
      partial: 0,
      failed: 0,
      refreshed_at: '2026-06-28T12:05:00Z',
      items: [{
        connector_id: 7,
        connector_name: 'relay-a',
        status: 'success',
        snapshot_status: 'success',
        snapshot_count: 0,
        snapshots: [],
        metrics: {
          connector: {
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
            updated_at: '2026-06-28T12:05:00Z',
          },
          snapshots: [],
          status: 'success',
          balance_detail: { status: 'success', checked_at: '2026-06-28T12:05:00Z' },
          usage_detail: { status: 'success', total_groups: 0, updated_groups: 0, missing_groups: [] },
          balance_available: true,
          usage_available: true,
          refreshed_at: '2026-06-28T12:05:00Z',
        },
      }],
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.attributes('aria-label')?.includes('connectors.expandGroups'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="connector-group-expansion"]').text()).toContain('Team Alpha')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    expect(refreshMonitoringData).toHaveBeenCalled()
    expect(wrapper.find('[data-testid="connector-group-expansion"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.upstreamRelayGroupMonitoring.connectors.noGroups')
  })

  it('统一刷新快照同步失败且未返回 snapshots 时保留旧概览快照', async () => {
    listSnapshots.mockResolvedValue([{
      id: 501,
      connector_id: 7,
      upstream_group_id: 'team-alpha',
      name: 'Team Alpha',
      platform: 'claude',
      status: 'active',
      default_rate_multiplier: 1,
      final_rate_multiplier: 1.75,
      source: 'login_available_groups',
      last_seen_at: '2026-06-28T12:00:00Z',
    }])
    refreshMonitoringData.mockResolvedValueOnce({
      status: 'partial',
      total: 1,
      success: 0,
      partial: 1,
      failed: 0,
      refreshed_at: '2026-06-28T12:05:00Z',
      items: [{
        connector_id: 7,
        connector_name: 'relay-a',
        status: 'partial',
        snapshot_status: 'failed',
        snapshot_count: 0,
        snapshot_error: 'sync failed',
        metrics: {
          connector: {
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
            updated_at: '2026-06-28T12:05:00Z',
          },
          snapshots: [],
          status: 'success',
          balance_detail: { status: 'success', checked_at: '2026-06-28T12:05:00Z' },
          usage_detail: { status: 'success', total_groups: 0, updated_groups: 0, missing_groups: [] },
          balance_available: true,
          usage_available: true,
          refreshed_at: '2026-06-28T12:05:00Z',
        },
      }],
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.attributes('aria-label')?.includes('connectors.expandGroups'))!.trigger('click')
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.upstreamRelayGroupMonitoring.refreshMonitoring'))!.trigger('click')
    await flushPromises()

    const expansion = wrapper.find('[data-testid="connector-group-expansion"]')
    expect(refreshMonitoringData).toHaveBeenCalled()
    expect(expansion.text()).toContain('Team Alpha')
    expect(expansion.findAll('[data-testid="connector-group-item"]')).toHaveLength(1)
  })

  it('候选五个必填字段缺失时就地提示且不发出保存请求', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('candidates.newCandidate'))!.trigger('click')
    await flushPromises()
    await wrapper.find('#candidate-form').trigger('submit')
    await flushPromises()

    expect(createCandidate).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('candidateForm.requiredAccount')
    expect(wrapper.text()).toContain('candidateForm.requiredUpstreamGroup')
    expect(wrapper.text()).toContain('candidateForm.requiredUpstreamApiKey')
    expect(wrapper.text()).toContain('candidateForm.requiredProbeModel')
  })

  it('历史候选缺少 API Key 时展示配置不完整并支持一键筛选', async () => {
    listCandidates.mockResolvedValue({
      items: [
        {
          id: 101, connector_id: 7, account_id: 42, account_name: 'missing-key', upstream_group_id: 'g1',
          upstream_api_key_id: null, probe_model: 'gpt-4o-mini', probe_protocol: 'chat_completions', enabled: true, notes: '',
        },
        {
          id: 102, connector_id: 7, account_id: 43, account_name: 'configured', upstream_group_id: 'g2',
          upstream_api_key_id: 855, probe_model: 'gpt-4o-mini', probe_protocol: 'chat_completions', enabled: true, notes: '',
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('candidates.configurationIncomplete')
    expect(wrapper.text()).toContain('missing-key')
    expect(wrapper.text()).toContain('configured')

    await wrapper.get('[data-testid="candidate-incomplete-filter"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('missing-key')
    expect(wrapper.text()).not.toContain('configured')
    expect(wrapper.text()).toContain('candidates.showAll')
  })

  it('候选映射编辑可从已同步上游分组下拉选择并提交 Group ID', async () => {
    listConnectorAPIKeys.mockResolvedValue([{ id: 855, name: 'team-key', masked_key: 'sk-***' }])
    listSnapshots.mockResolvedValue([{
      id: 501,
      connector_id: 7,
      upstream_group_id: 'team-alpha',
      name: 'Team Alpha',
      platform: 'claude',
      status: 'active',
      default_rate_multiplier: 1,
      final_rate_multiplier: 1.25,
      source: 'login_available_groups',
      last_seen_at: '2026-06-28T12:00:00Z',
    }])
    createCandidate.mockResolvedValue({
      id: 101,
      connector_id: 7,
      account_id: 42,
      upstream_group_id: 'team-alpha',
      probe_model: 'gpt-4o-mini',
      probe_protocol: 'chat_completions',
      enabled: true,
      notes: '',
      created_at: '2026-06-28T12:00:00Z',
      updated_at: '2026-06-28T12:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('candidates.newCandidate'))!.trigger('click')
    await flushPromises()

    const protocolSelect = wrapper.findAll('#candidate-form select').find((select) => select.text().includes('anthropic'))
    expect(protocolSelect?.text()).toContain('anthropic')

    const groupSelect = wrapper.get('[data-testid="candidate-upstream-group-select"]')
    expect(groupSelect.text()).toContain('Team Alpha')
    expect(groupSelect.text()).toContain('team-alpha')
    expect(wrapper.find('[data-testid="candidate-upstream-group-manual-input"]').exists()).toBe(true)

    await groupSelect.setValue('team-alpha')
    const form = wrapper.get('#candidate-form')
    const formSelects = form.findAll('select')
    await formSelects[1]!.setValue('42')
    await formSelects[3]!.setValue('855')
    await form.find('input[type="text"]').setValue('gpt-4o-mini')
    await flushPromises()

    expect(wrapper.find('[data-testid="candidate-upstream-group-manual-input"]').exists()).toBe(false)
    await form.trigger('submit')
    await flushPromises()

    expect(createCandidate).toHaveBeenCalledWith(expect.objectContaining({
      connector_id: 7,
      upstream_group_id: 'team-alpha',
    }))
  })

  it('候选选择上游 API Key 后自动回显 Key 当前分组', async () => {
    listConnectorAPIKeys.mockResolvedValue([{ id: 855, name: 'team-key', masked_key: 'sk-***', group_id: 'team-alpha' }])
    listSnapshots.mockResolvedValue([{
      id: 501,
      connector_id: 7,
      upstream_group_id: 'team-alpha',
      name: 'Team Alpha',
      platform: 'claude',
      status: 'active',
      default_rate_multiplier: 1,
      final_rate_multiplier: 1.25,
      source: 'login_available_groups',
      last_seen_at: '2026-06-28T12:00:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('candidates.newCandidate'))!.trigger('click')
    await flushPromises()

    const form = wrapper.get('#candidate-form')
    const formSelects = form.findAll('select')
    await formSelects[3]!.setValue('855')
    await flushPromises()

    expect(wrapper.get('[data-testid="candidate-upstream-group-select"]').element).toHaveProperty('value', 'team-alpha')
    expect(wrapper.find('[data-testid="candidate-upstream-group-manual-input"]').exists()).toBe(false)
  })

  it('自动监控页展示派生阈值并保存配置', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('monitoring.description')
    expect(wrapper.text()).not.toContain('当前阶段仅保存配置和支持手动触发')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.status.running')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.status.detailBoth')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.statusChips.sync')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.statusChips.probe')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.statusChips.recommendation')
    expect(wrapper.find('[data-testid="auto-monitoring-status"]').text()).toContain('monitoring.statusChips.autoApply')
    expect(wrapper.text()).toContain('monitoring.snapshotStaleDerived')
    expect(wrapper.text()).toContain('monitoring.recommendationAutomationTitle')
    expect(wrapper.text()).toContain('monitoring.autoApplyDisabledHint')
    expect(wrapper.text()).toContain('60 180')
    expect(wrapper.text()).toContain('15 45')

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.save'))!.trigger('click')
    await flushPromises()

    expect(updateMonitoringPolicy).toHaveBeenCalledWith({
      auto_sync_enabled: true,
      sync_interval_minutes: 60,
      auto_probe_enabled: true,
      probe_interval_minutes: 15,
      auto_recommendation_enabled: false,
      recommendation_interval_minutes: 60,
      auto_apply_recommendations_enabled: false,
      max_auto_apply_suggestions: 20,
      max_auto_apply_priority_delta: 100,
      min_auto_apply_confidence: 'medium',
      allow_auto_apply_degraded_health: false,
      failure_retry_interval_minutes: 5,
      sync_concurrency: 2,
      probe_concurrency: 5,
    })
    expect(wrapper.text()).toContain('monitoring.savedAt')
  })

  it('顶部同步新鲜度使用已保存策略，设置页派生文案使用草稿策略', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-28T12:00:00Z'))
    getMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      sync_interval_minutes: 60,
      probe_interval_minutes: 15,
    }))
    listConnectors.mockResolvedValueOnce({
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
        last_synced_at: '2026-06-28T10:00:00Z',
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

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('input[type="number"]')[0].setValue(10)
    await flushPromises()

    expect(wrapper.get('[data-testid="freshness-badge-synced"]').text()).toContain('freshness.fresh')
    expect(wrapper.text()).toContain('10 30')
  })

  it('推荐策略保存使用监控草稿派生阈值', async () => {
    getMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      sync_interval_minutes: 60,
      probe_interval_minutes: 15,
    }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()
    const numberInputs = wrapper.findAll('input[type="number"]')
    await numberInputs[0].setValue(120)
    await numberInputs[1].setValue(20)

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.policy'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('policy.save'))!.trigger('click')
    await flushPromises()

    expect(updateRecommendationPolicy).toHaveBeenCalledWith(expect.objectContaining({
      snapshot_freshness_minutes: 360,
      usage_delta_freshness_minutes: 360,
      probe_freshness_minutes: 60,
    }))
  })

  it('推荐策略保存包含管理员可选 Pause 策略', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.policy'))!.trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="policy-pause-rate-gap-enabled"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-testid="policy-pause-rate-gap-threshold"]').setValue(0.08)
    await wrapper.get('[data-testid="policy-pause-consecutive-failures-enabled"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-testid="policy-pause-consecutive-failures-threshold"]').setValue(5)
    await wrapper.get('[data-testid="policy-pause-success-rate-enabled"]').setValue(true)

    await wrapper.findAll('button').find((button) => button.text().includes('policy.save'))!.trigger('click')
    await flushPromises()

    expect(updateRecommendationPolicy).toHaveBeenCalledWith(expect.objectContaining({
      pause_rate_gap_enabled: true,
      pause_rate_gap_threshold: 0.08,
      pause_consecutive_failures_enabled: true,
      pause_consecutive_failures_threshold: 5,
      pause_success_rate_enabled: true,
    }))
  })

  it('自动监控页在自动项关闭时展示未启用状态', async () => {
    getMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      auto_sync_enabled: false,
      auto_probe_enabled: false,
    }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    const status = wrapper.find('[data-testid="auto-monitoring-status"]')
    expect(status.text()).toContain('monitoring.status.disabled')
    expect(status.text()).toContain('monitoring.status.detailDisabled')
    expect(status.text()).toContain('monitoring.statusChips.sync')
    expect(status.text()).toContain('monitoring.statusChips.probe')
  })

  it('自动监控状态基于已保存策略，未保存表单变更不会显示运行中', async () => {
    getMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      auto_sync_enabled: false,
      auto_probe_enabled: false,
    }))
    updateMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      auto_sync_enabled: true,
      auto_probe_enabled: false,
      updated_at: '2026-06-28T12:10:00Z',
    }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    const status = wrapper.find('[data-testid="auto-monitoring-status"]')
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    expect(status.text()).toContain('monitoring.status.disabled')
    expect(status.text()).toContain('monitoring.status.detailDisabled')

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.save'))!.trigger('click')
    await flushPromises()

    expect(status.text()).toContain('monitoring.status.running')
    expect(status.text()).toContain('monitoring.status.detailSyncOnly')
  })

  it('顶部调度开关基于已保存策略提交，且不夹带策略页草稿', async () => {
    updateMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      auto_sync_enabled: false,
      sync_interval_minutes: 60,
      updated_at: '2026-06-28T12:10:00Z',
    }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()
    const syncIntervalInput = wrapper.findAll('input[type="number"]')[0]
    await syncIntervalInput.setValue(999)

    await wrapper.get('[data-testid="runner-toggle-sync"]').trigger('click')
    await flushPromises()

    expect(updateMonitoringPolicy).toHaveBeenCalledWith(expect.objectContaining({
      auto_sync_enabled: false,
      sync_interval_minutes: 60,
      auto_probe_enabled: true,
      probe_interval_minutes: 15,
    }))
    expect((syncIntervalInput.element as HTMLInputElement).value).toBe('999')
  })

  it('顶部调度开关保存失败时回滚到已保存状态', async () => {
    updateMonitoringPolicy.mockRejectedValueOnce(new Error('save failed'))

    const wrapper = mountView()
    await flushPromises()

    const toggle = wrapper.get('[data-testid="runner-toggle-sync"]')
    expect(toggle.attributes('aria-checked')).toBe('true')

    await toggle.trigger('click')
    await flushPromises()

    expect(toggle.attributes('aria-checked')).toBe('true')
    expect(wrapper.find('[data-testid="page-error"]').text()).toContain('save failed')
    expect(wrapper.find('[data-testid="page-error"]').text()).toContain('operationResult.sources.candidates')

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.connectors'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
  })

  it('顶部调度失败卡展示 last_error 详情', async () => {
    getRunnerStatus.mockResolvedValueOnce(runnerStatus({
      sync: { enabled: true, in_flight: false, last_finished_at: '2026-06-28T12:00:00Z', last_succeeded: false, last_error: 'sync exploded', next_run_at: null, interval_minutes: 60, failure_retry_minutes: 5 },
    }))

    const wrapper = mountView()
    await flushPromises()

    const card = wrapper.get('[data-testid="runner-status-card-sync"]')
    expect(card.text()).toContain('runnerStatus.states.failed')
    expect(card.text()).toContain('sync exploded')
  })

  it('定格失败卡展示日期、连接器并把原始原因放在技术详情', async () => {
    getRunnerStatus.mockResolvedValueOnce(runnerStatus({
      finalize: {
        enabled: true,
        in_flight: false,
        last_finished_at: '2026-06-29T00:05:00Z',
        last_succeeded: false,
        last_error: 'finalize usage failed for 2026-06-28: relay-a (#7): upstream timeout',
        last_failures: [{ connector_id: 7, connector_name: 'relay-a', date: '2026-06-28', reason: 'upstream timeout' }],
        next_run_at: '2026-06-29T00:10:00Z',
        interval_minutes: 0,
        failure_retry_minutes: 5,
      },
    }))

    const wrapper = mountView()
    await flushPromises()

    const card = wrapper.get('[data-testid="runner-status-card-finalize"]')
    expect(card.text()).toContain('runnerStatus.hints.finalizeFailures')
    expect(card.text()).toContain('2026-06-28')
    expect(card.text()).toContain('relay-a')
    expect(card.find('details').text()).toContain('upstream timeout')
    expect(card.find('[role="switch"]').exists()).toBe(false)
  })

  it('自动推荐和自动应用开启时展示安全门提示', async () => {
    getMonitoringPolicy.mockResolvedValueOnce(monitoringPolicy({
      auto_sync_enabled: false,
      auto_probe_enabled: false,
      auto_recommendation_enabled: true,
      recommendation_interval_minutes: 30,
      auto_apply_recommendations_enabled: true,
      min_auto_apply_confidence: 'high',
    }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    const status = wrapper.find('[data-testid="auto-monitoring-status"]')
    expect(status.text()).toContain('monitoring.status.running')
    expect(status.text()).toContain('monitoring.status.detailAutoApply')
    expect(status.text()).toContain('monitoring.statusChips.recommendation')
    expect(status.text()).toContain('monitoring.statusChips.autoApply')
    expect(wrapper.text()).toContain('monitoring.autoApplyEnabledHint')
  })

  it('推荐历史失败 run 优先显示失败状态且不计入待应用建议', async () => {
    listRecommendationRuns.mockResolvedValue({
      items: [{
        id: 88,
        status: 'failed',
        total_candidates: 2,
        suggestion_count: 1,
        applied: false,
        error_message: 'policy failed',
        created_by: 0,
        created_at: '2026-06-28T12:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    const priorityTab = wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!
    expect(priorityTab.text()).not.toContain('1')
    await priorityTab.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('recommendations.failed')
    expect(wrapper.findAll('button').some((button) => button.text().includes('recommendations.viewAndApply'))).toBe(false)
  })

  it('推荐历史运行中 run 优先显示运行中而不是待确认', async () => {
    listRecommendationRuns.mockResolvedValue({
      items: [recommendationRun({
        id: 89,
        status: 'running',
        suggestion_count: 2,
        applied: false,
      })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()

    const statusCell = wrapper.find('[data-testid="recommendation-run-status-89"]')
    expect(statusCell.text()).toContain('recommendations.running')
    expect(statusCell.text()).not.toContain('recommendations.pending')
    expect(wrapper.findAll('button').some((button) => button.text().includes('recommendations.viewAndApply'))).toBe(false)
  })

  it('推荐历史展示系统和管理员生成及应用来源', async () => {
    listRecommendationRuns.mockResolvedValue({
      items: [
        {
          id: 77,
          status: 'success',
          total_candidates: 2,
          suggestion_count: 1,
          applied: true,
          applied_by: 0,
          applied_at: '2026-06-28T12:05:00Z',
          created_by: 0,
          created_at: '2026-06-28T12:00:00Z',
        },
        {
          id: 78,
          status: 'success',
          total_candidates: 1,
          suggestion_count: 1,
          applied: true,
          applied_by: 42,
          applied_at: '2026-06-28T12:15:00Z',
          created_by: 42,
          created_at: '2026-06-28T12:10:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('recommendations.colSource')
    expect(wrapper.text()).toContain('recommendations.sourceSystem')
    expect(wrapper.text()).toContain('recommendations.appliedBySystem')
    expect(wrapper.text()).toContain('recommendations.sourceManual')
    expect(wrapper.text()).toContain('recommendations.appliedByManual')
  })

  it('推荐历史支持翻页查看更多旧记录', async () => {
    listRecommendationRuns.mockResolvedValueOnce({
      items: [recommendationRun({ id: 77 })],
      total: 21,
      page: 1,
      page_size: 20,
      pages: 2,
    }).mockResolvedValueOnce({
      items: [recommendationRun({ id: 55, created_at: '2026-06-27T12:00:00Z' })],
      total: 21,
      page: 2,
      page_size: 20,
      pages: 2,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.next'))!.trigger('click')
    await flushPromises()

    expect(listRecommendationRuns).toHaveBeenLastCalledWith({ page: 2, page_size: 20, has_suggestions: undefined })
    expect(wrapper.text()).toContain('#55')
  })

  it('推荐历史可一键过滤无建议记录', async () => {
    listRecommendationRuns.mockResolvedValueOnce({
      items: [
        recommendationRun({ id: 77, suggestion_count: 1 }),
        recommendationRun({ id: 78, suggestion_count: 0 }),
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    }).mockResolvedValueOnce({
      items: [recommendationRun({ id: 77, suggestion_count: 1 })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    const suggestionFilter = wrapper.findAll('label').find((label) => label.text().includes('recommendations.onlyWithSuggestions'))!
    await suggestionFilter.get('input[type="checkbox"]').setValue(true)
    await flushPromises()

    expect(listRecommendationRuns).toHaveBeenLastCalledWith({ page: 1, page_size: 20, has_suggestions: true })
    expect(wrapper.text()).toContain('#77')
    expect(wrapper.text()).not.toContain('#78')
  })

  it('关闭的建议不会被当作最新待处理建议补全详情', async () => {
    const closedRun = recommendationRun({
      id: 88,
      closed: true,
      created_at: '2026-06-29T12:00:00Z',
      suggestions: undefined,
    })
    const pendingRun = recommendationRun({
      id: 77,
      created_at: '2026-06-28T12:00:00Z',
      suggestions: undefined,
    })
    listRecommendationRuns.mockResolvedValue({
      items: [closedRun, pendingRun],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(recommendationRun({ id: 77 }))

    mountView()
    await flushPromises()

    expect(getRecommendationRun).toHaveBeenCalledTimes(1)
    expect(getRecommendationRun).toHaveBeenCalledWith(77)
  })

  it('推荐详情展示账号暂停和恢复建议', async () => {
    const gateRun = recommendationRun({
      suggestion_count: 2,
      suggestions: [
        {
          ...recommendationRun().suggestions[0],
          id: 11,
          action_type: 'account_pause',
          account_id: 42,
          account_name: 'claude-relay',
          old_priority: 50,
          new_priority: null,
          old_schedulable: true,
          new_schedulable: false,
          health_status: 'failed',
          reason_code: 'account_gate_latest_probe_failed',
          confidence: 'medium',
          health_summary: 'latest probe failed',
          reason: '最近探测失败，建议暂停账号承接',
        },
        {
          ...recommendationRun().suggestions[0],
          id: 12,
          action_type: 'account_resume',
          account_id: 43,
          account_name: 'claude-recovered',
          old_priority: 60,
          new_priority: null,
          old_schedulable: false,
          new_schedulable: true,
          health_status: 'success',
          reason_code: 'account_gate_recovered',
          confidence: 'high',
          health_summary: 'probe ok',
          reason: '建议恢复账号承接',
        },
      ],
    })
    listRecommendationRuns.mockResolvedValue({
      items: [gateRun],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(gateRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewAndApply'))!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('suggestionActions.accountPause')
    expect(wrapper.text()).toContain('suggestionActions.accountResume')
    expect(wrapper.text()).toContain('schedulableStatus.enabled')
    expect(wrapper.text()).toContain('schedulableStatus.paused')
    expect(wrapper.text()).toContain('→')
    expect(wrapper.text()).toContain('applyDialog.riskPause')
    expect(wrapper.text()).toContain('applyDialog.riskResume')
  })

  it('已应用的 Priority 建议仍可打开并查看明细', async () => {
    const appliedRun = recommendationRun({
      applied: true,
      applied_by: 42,
      applied_at: '2026-06-28T12:05:00Z',
      suggestions: recommendationRun().suggestions.map((item) => ({
        ...item,
        applied: true,
        applied_by: 42,
        applied_at: '2026-06-28T12:05:00Z',
      })),
    })
    listRecommendationRuns.mockResolvedValue({
      items: [appliedRun],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(appliedRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewDetails'))!.trigger('click')
    await flushPromises()

    expect(getRecommendationRun).toHaveBeenCalledWith(77)
    expect(wrapper.text()).toContain('applyDialog.appliedNotice')
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.text()).toContain('probe ok')
    expect(wrapper.html()).not.toContain('rate-source-tag-stub')
    expect(wrapper.findAll('button').some((button) => button.text().includes('applyDialog.confirmApply'))).toBe(false)
  })

  it('应用 Priority 建议成功后展示明确反馈并保留审计详情', async () => {
    const pendingRun = recommendationRun()
    const appliedRun = recommendationRun({
      applied: true,
      applied_by: 42,
      applied_at: '2026-06-28T12:05:00Z',
      suggestions: recommendationRun().suggestions.map((item) => ({
        ...item,
        applied: true,
        applied_by: 42,
        applied_at: '2026-06-28T12:05:00Z',
      })),
    })
    listRecommendationRuns.mockResolvedValue({
      items: [pendingRun],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(pendingRun)
    applyRecommendationRun.mockResolvedValue(appliedRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewAndApply'))!.trigger('click')
    await flushPromises()
    const applyButton = wrapper.findAll('button').find((button) => button.text().includes('applyDialog.confirmApply'))!
    expect(wrapper.find('[data-testid="apply-confirmation-control"]').exists()).toBe(false)
    expect(applyButton.attributes('disabled')).toBeUndefined()
    await applyButton.trigger('click')
    await flushPromises()

    expect(applyRecommendationRun).toHaveBeenCalledWith(77)
    expect(wrapper.find('[data-testid="page-success"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="apply-operation-result"]').text()).toContain('operationResult.results.applySuccess')
    expect(wrapper.text()).toContain('applyDialog.appliedNotice')
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.findAll('button').some((button) => button.text().includes('applyDialog.confirmApply'))).toBe(false)
  })

  it('应用 Priority 建议失败时展示后端详细原因并保留详情', async () => {
    const pendingRun = recommendationRun()
    listRecommendationRuns.mockResolvedValue({
      items: [pendingRun],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(pendingRun)
    applyRecommendationRun.mockRejectedValue(new Error('账号 #42 的 priority 已变化：生成建议时为 50，当前为 80，建议值为 10；请重新生成建议后再应用'))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewAndApply'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('applyDialog.confirmApply'))!.trigger('click')
    await flushPromises()

    expect(applyRecommendationRun).toHaveBeenCalledWith(77)
    expect(wrapper.find('[data-testid="page-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="apply-operation-result"]').text()).toContain('operationResult.results.applyFailed')
    expect(wrapper.find('[data-testid="apply-operation-result"]').text()).toContain('账号 #42 的 priority 已变化')
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.html()).not.toContain('rate-source-tag-stub')
    expect(wrapper.text()).toContain('applyDialog.confirmApply')
  })

  it('应用弹窗中可以关闭待应用建议并保留历史详情', async () => {
    const pendingRun = recommendationRun()
    const closedRun = recommendationRun({
      closed: true,
      closed_by: 42,
      closed_at: '2026-06-30T12:00:00Z',
    })
    listRecommendationRuns.mockResolvedValue({
      items: [pendingRun],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(pendingRun)
    closeRecommendationRun.mockResolvedValue(closedRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewAndApply'))!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="apply-confirmation-control"]').exists()).toBe(false)
    await wrapper.findAll('button').find((button) => button.text().includes('applyDialog.closeSuggestion'))!.trigger('click')
    await flushPromises()

    expect(closeRecommendationRun).toHaveBeenCalledWith(77)
    expect(applyRecommendationRun).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="recommendation-run-status-77"]').text()).toContain('recommendations.closed')
    expect(wrapper.text()).toContain('applyDialog.closedNotice')
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.findAll('button').some((button) => button.text().includes('applyDialog.confirmApply'))).toBe(false)
  })

  it('关闭建议后保留当前列表排序位置', async () => {
    const firstRun = recommendationRun({ id: 88, created_at: '2026-06-29T12:00:00Z' })
    const pendingRun = recommendationRun({ id: 77, created_at: '2026-06-28T12:00:00Z' })
    const closedRun = recommendationRun({
      id: 77,
      created_at: '2026-06-28T12:00:00Z',
      closed: true,
      closed_by: 42,
      closed_at: '2026-06-30T12:00:00Z',
    })
    listRecommendationRuns.mockResolvedValue({
      items: [firstRun, pendingRun],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    closeRecommendationRun.mockResolvedValue(closedRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').filter((button) => button.text().includes('recommendations.closeSuggestion')).at(1)!.trigger('click')
    await flushPromises()

    expect(closeRecommendationRun).toHaveBeenCalledWith(77)
    const runCells = wrapper.findAll('tbody tr td.font-mono').map((cell) => cell.text())
    expect(runCells).toEqual(['#88', '#77'])
    expect(wrapper.find('[data-testid="recommendation-run-status-77"]').text()).toContain('recommendations.closed')
  })

  it('恢复关闭建议后保留详情并恢复可应用状态', async () => {
    const newerRun = recommendationRun({ id: 88, created_at: '2026-06-29T12:00:00Z' })
    const closedRun = recommendationRun({
      id: 77,
      closed: true,
      closed_by: 42,
      closed_at: '2026-06-30T12:00:00Z',
      created_at: '2026-06-28T12:00:00Z',
    })
    const restoredRun = recommendationRun({
      id: 77,
      closed: false,
      closed_by: null,
      closed_at: null,
      created_at: '2026-06-28T12:00:00Z',
    })
    listRecommendationRuns.mockResolvedValue({
      items: [newerRun, closedRun],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getRecommendationRun.mockResolvedValue(closedRun)
    restoreRecommendationRun.mockResolvedValue(restoredRun)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.recommendations'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('recommendations.viewDetails'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('applyDialog.restoreSuggestion')

    await wrapper.findAll('button').find((button) => button.text().includes('applyDialog.restoreSuggestion'))!.trigger('click')
    await flushPromises()

    expect(restoreRecommendationRun).toHaveBeenCalledWith(77)
    expect(wrapper.text()).toContain('Team Alpha')
    expect(wrapper.text()).toContain('applyDialog.confirmApply')
    expect(wrapper.find('[data-testid="recommendation-run-status-77"]').text()).toContain('recommendations.pending')
    const runCells = wrapper.findAll('tbody tr td.font-mono').map((cell) => cell.text())
    expect(runCells).toEqual(['#88', '#77'])
  })

  it('自动监控页调用批量同步和批量探测接口并展示成功失败明细', async () => {
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
      total: 2,
      success: 1,
      failed: 1,
      items: [
        {
          id: 101,
          candidate_id: 101,
          account_id: 42,
          account_name: 'claude-relay',
          success: false,
          error_class: 'rate_limited',
          error_reason: 'rate limited',
          latency_ms: 1300,
          http_status: 429,
          probed_at: '2026-06-28T12:10:00Z',
        },
        {
          id: 102,
          candidate_id: 102,
          account_id: 43,
          account_name: 'openai-relay',
          success: true,
          latency_ms: 620,
          http_status: 200,
          probed_at: '2026-06-28T12:10:01Z',
        },
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('tabs.monitoring'))!.trigger('click')
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.fetchSnapshotsAll'))!.trigger('click')
    await flushPromises()

    expect(syncAllConnectors).toHaveBeenCalled()
    expect(wrapper.text()).toContain('operationResult.titles.sync')
    expect(wrapper.text()).toContain('relay-a')
    expect(wrapper.text()).toContain('timeout')
    expect(wrapper.text()).toContain('operationResult.impacts.syncPartial')

    await wrapper.findAll('button').find((button) => button.text().includes('monitoring.probeAll'))!.trigger('click')
    await flushPromises()

    expect(probeAllCandidates).toHaveBeenCalled()
    expect(wrapper.text()).toContain('operationResult.titles.probe')
    expect(wrapper.text()).toContain('#42 claude-relay')
    expect(wrapper.text()).toContain('rate limited')
    expect(wrapper.text()).toContain('1300ms')
    expect(wrapper.text()).toContain('429')
    expect(wrapper.text()).toContain('#43 openai-relay')
    expect(wrapper.text()).toContain('620ms')
    expect(wrapper.text()).toContain('200')
  })

  it('候选页一键探测混合结果展示成功失败和隐藏数量', async () => {
    listCandidates.mockResolvedValue({
      items: Array.from({ length: 8 }, (_, index) => ({
        id: 101 + index,
        connector_id: 7,
        account_id: 42 + index,
        account_name: `relay-${index + 1}`,
        upstream_group_id: `team-${index + 1}`,
        probe_model: 'gpt-4o-mini',
        probe_protocol: 'chat_completions',
        enabled: true,
        notes: '',
        created_at: '2026-06-28T12:00:00Z',
        updated_at: '2026-06-28T12:00:00Z',
      })),
      total: 8,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    probeAllCandidates.mockResolvedValue({
      total: 8,
      success: 6,
      failed: 2,
      items: [
        ...Array.from({ length: 6 }, (_, index) => ({
          id: 101 + index,
          candidate_id: 101 + index,
          account_id: 42 + index,
          account_name: `relay-${index + 1}`,
          success: true,
          latency_ms: 500 + index,
          http_status: 200,
          probed_at: '2026-06-28T12:10:00Z',
        })),
        {
          id: 201,
          candidate_id: 201,
          account_id: 51,
          account_name: 'failed-relay-a',
          success: false,
          error_class: 'timeout',
          error_reason: 'upstream timeout',
          latency_ms: 2000,
          probed_at: '2026-06-28T12:10:02Z',
        },
        {
          id: 202,
          candidate_id: 202,
          account_id: 52,
          account_name: 'failed-relay-b',
          success: false,
          error_class: 'auth_failed',
          error_reason: 'invalid token',
          probed_at: '2026-06-28T12:10:03Z',
        },
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('candidates.probeAll'))!.trigger('click')
    await flushPromises()

    const feedback = wrapper.find('[data-testid="candidate-bulk-probe-feedback"]')
    expect(feedback.exists()).toBe(true)
    expect(feedback.text()).toContain('relay-1')
    expect(feedback.text()).toContain('500ms')
    expect(feedback.text()).toContain('failed-relay-a')
    expect(feedback.text()).toContain('upstream timeout')
    expect(feedback.text()).toContain('failed-relay-b')
    expect(feedback.text()).toContain('invalid token')
    expect(feedback.text()).toContain('admin.upstreamRelayGroupMonitoring.operationResult.itemListWithMore')
    expect(feedback.text()).toContain('1')
  })
})
