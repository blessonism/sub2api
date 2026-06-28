import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamRelayGroupMonitoringView from '../UpstreamRelayGroupMonitoringView.vue'

const {
  listConnectors,
  listCandidates,
  listRecommendationRuns,
  getRecommendationPolicy,
  refreshConnectorMetrics,
  listAccounts,
  listGroups,
} = vi.hoisted(() => ({
  listConnectors: vi.fn(),
  listCandidates: vi.fn(),
  listRecommendationRuns: vi.fn(),
  getRecommendationPolicy: vi.fn(),
  refreshConnectorMetrics: vi.fn(),
  listAccounts: vi.fn(),
  listGroups: vi.fn(),
}))

vi.mock('@/api/admin/upstreamRelayGroupMonitors', () => ({
  default: {
    listConnectors,
    listCandidates,
    listRecommendationRuns,
    getRecommendationPolicy,
    refreshConnectorMetrics,
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
        return key.replace(/\{(\w+)\}/g, (_, token) => String(params[token] ?? `{${token}}`))
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
        BaseDialog: true,
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
    getRecommendationPolicy.mockReset()
    refreshConnectorMetrics.mockReset()
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
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 200, pages: 1 })
    listGroups.mockResolvedValue([])
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
    expect(wrapper.text()).toContain('admin.upstreamRelayGroupMonitoring.errors.metricsRefreshNeedsFullSync')
  })
})
