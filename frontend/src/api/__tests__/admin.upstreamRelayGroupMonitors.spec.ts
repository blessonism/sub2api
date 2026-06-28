import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put
  }
}))

import {
  generateRecommendations,
  getRecommendationPolicy,
  previewRecommendations,
  refreshConnectorMetrics,
  updateRecommendationPolicy,
  type UpstreamRelayRecommendationPolicy
} from '@/api/admin/upstreamRelayGroupMonitors'

describe('admin upstream relay group monitors api', () => {
  const policy: UpstreamRelayRecommendationPolicy = {
    snapshot_freshness_minutes: 1440,
    usage_delta_freshness_minutes: 1440,
    probe_freshness_minutes: 30,
    min_success_rate: 0.5,
    min_sample_size: 3,
    exclude_consecutive_failures: true,
    priority_start: 10,
    priority_step: 10,
    sort_fields: ['rate_asc', 'success_rate_desc', 'latency_asc']
  }

  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
  })

  it('loads the global recommendation policy', async () => {
    get.mockResolvedValue({ data: policy })

    await expect(getRecommendationPolicy()).resolves.toEqual(policy)
    expect(get).toHaveBeenCalledWith('/admin/upstream-relay-group-monitors/recommendation-policy')
  })

  it('saves the global recommendation policy', async () => {
    put.mockResolvedValue({ data: policy })

    await expect(updateRecommendationPolicy(policy)).resolves.toEqual(policy)
    expect(put).toHaveBeenCalledWith('/admin/upstream-relay-group-monitors/recommendation-policy', policy)
  })

  it('previews recommendations without creating a run', async () => {
    const response = {
      policy,
      total_candidates: 2,
      suggestion_count: 1,
      excluded_count: 1,
      suggestions: [],
      exclusions: []
    }
    post.mockResolvedValue({ data: response })

    await expect(previewRecommendations(policy)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/upstream-relay-group-monitors/recommendations/preview', policy)
  })

  it('generates formal recommendations through the persisted-run endpoint', async () => {
    const run = {
      id: 7,
      status: 'success',
      total_candidates: 2,
      suggestion_count: 1,
      applied: false,
      created_at: '2026-06-28T12:00:00Z'
    }
    post.mockResolvedValue({ data: run })

    await expect(generateRecommendations()).resolves.toEqual(run)
    expect(post).toHaveBeenCalledWith('/admin/upstream-relay-group-monitors/recommendations')
  })

  it('refreshes connector metrics without full connector sync', async () => {
    const response = {
      connector: { id: 7, name: 'relay' },
      snapshots: [],
      balance_available: true,
      usage_available: true,
      refreshed_at: '2026-06-28T12:00:00Z'
    }
    post.mockResolvedValue({ data: response })

    await expect(refreshConnectorMetrics(7)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/upstream-relay-group-monitors/connectors/7/metrics/refresh')
  })
})
