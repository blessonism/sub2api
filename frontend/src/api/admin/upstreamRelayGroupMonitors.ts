import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type UpstreamRelayAuthMode = 'manual_session' | 'password_login'
export type UpstreamRelayConnectorStatus = 'active' | 'needs_reauth' | 'invalid' | 'paused'
export type UpstreamRelayProbeProtocol = 'chat_completions' | 'responses'
export type UpstreamRelayRunStatus = 'success' | 'failed'
export type UpstreamRelaySnapshotChangeType = 'added' | 'removed' | 'rate_changed'
export type UpstreamRelayRecommendationSortField = 'rate_asc' | 'success_rate_desc' | 'latency_asc'

export interface UpstreamRelayConnector {
  id: number
  name: string
  base_url: string
  auth_mode: UpstreamRelayAuthMode
  status: UpstreamRelayConnectorStatus
  credential_version: number
  upstream_account_balance?: number | null
  upstream_account_balance_checked_at?: string | null
  bearer_token_masked?: string
  refresh_token_masked?: string
  login_email_masked?: string
  cookie_masked?: string
  user_agent_masked?: string
  has_bearer_token: boolean
  has_refresh_token: boolean
  has_login_email: boolean
  has_cookie: boolean
  has_user_agent: boolean
  last_verified_at?: string | null
  last_synced_at?: string | null
  last_error?: string
  created_by?: number
  created_at: string
  updated_at: string
}

export interface UpstreamRelayConnectorInput {
  name: string
  base_url: string
  auth_mode?: UpstreamRelayAuthMode
  bearer_token?: string
  login_email?: string
  login_password?: string
  cookie?: string
  user_agent?: string
}

export interface UpstreamRelayGroupRateSnapshot {
  id: number
  connector_id: number
  upstream_group_id: string
  name: string
  platform: string
  status: string
  default_rate_multiplier: number
  override_rate_multiplier?: number | null
  final_rate_multiplier: number
  today_actual_cost?: number | null
  today_total_tokens?: number | null
  today_usage_checked_at?: string | null
  source: string
  last_seen_at: string
}

export interface UpstreamRelayGroupRateSnapshotChange {
  id: number
  connector_id: number
  connector_name?: string
  upstream_group_id: string
  group_name: string
  platform: string
  change_type: UpstreamRelaySnapshotChangeType
  old_final_rate_multiplier?: number | null
  new_final_rate_multiplier?: number | null
  old_status: string
  new_status: string
  source: string
  changed_at: string
}

export interface UpstreamRelayConnectorMetricsRefreshResult {
  connector: UpstreamRelayConnector
  snapshots: UpstreamRelayGroupRateSnapshot[]
  balance_available: boolean
  balance_error?: string
  usage_available: boolean
  usage_error?: string
  refreshed_at: string
}

export interface UpstreamRelayProbeResult {
  id: number
  candidate_id: number
  success: boolean
  latency_ms?: number | null
  http_status?: number | null
  error_class?: string
  error_message?: string
  probed_at: string
}

export interface UpstreamRelayCandidateHealth {
  probe_count: number
  success_count: number
  success_rate: number
  avg_latency_ms?: number | null
  p95_latency_ms?: number | null
  consecutive_successes: number
  consecutive_failures: number
  last_error_class?: string
  last_success_at?: string | null
  window_minutes: number
  sample_size: number
}

export interface UpstreamRelayUsageDeltaSample {
  id?: number
  candidate_id: number
  probe_result_id?: number | null
  model: string
  status: 'reliable' | 'insufficient' | 'unavailable'
  before_cost?: number | null
  before_actual_cost?: number | null
  after_cost?: number | null
  after_actual_cost?: number | null
  cost_delta?: number | null
  actual_cost_delta?: number | null
  derived_rate_multiplier?: number | null
  unreliable_reason?: string
  sampled_at: string
}

export interface UpstreamRelayCandidate {
  id: number
  connector_id: number
  connector_name?: string
  connector_status?: UpstreamRelayConnectorStatus
  today_actual_cost?: number | null
  today_total_tokens?: number | null
  today_usage_checked_at?: string | null
  account_id: number
  account_name?: string
  account_platform?: string
  upstream_group_id: string
  upstream_group_name?: string
  probe_model: string
  probe_protocol: UpstreamRelayProbeProtocol
  target_group_id: number
  target_group_name?: string
  current_priority?: number | null
  enabled: boolean
  notes: string
  latest_probe?: UpstreamRelayProbeResult | null
  latest_snapshot?: UpstreamRelayGroupRateSnapshot | null
  health?: UpstreamRelayCandidateHealth | null
  latest_usage_delta?: UpstreamRelayUsageDeltaSample | null
  created_by?: number
  created_at: string
  updated_at: string
}

export interface UpstreamRelayCandidateInput {
  connector_id: number
  account_id: number
  upstream_group_id: string
  probe_model: string
  probe_protocol?: UpstreamRelayProbeProtocol
  target_group_id: number
  enabled?: boolean
  notes?: string
}

export interface UpstreamRelayRecommendationSuggestion {
  id?: number
  run_id?: number
  candidate_id: number
  connector_id: number
  connector_name?: string
  account_id: number
  account_name?: string
  upstream_group_id: string
  upstream_group_name?: string
  target_group_id: number
  target_group_name?: string
  old_priority?: number | null
  new_priority: number
  final_rate_multiplier: number
  health_status: string
  reason_code: string
  confidence: string
  health_summary: string
  rate_source: string
  reason: string
  applied: boolean
  applied_by?: number | null
  applied_at?: string | null
  created_at?: string
}

export interface UpstreamRelayRecommendationRun {
  id: number
  status: UpstreamRelayRunStatus
  total_candidates: number
  suggestion_count: number
  applied: boolean
  applied_by?: number | null
  applied_at?: string | null
  error_message?: string
  created_by?: number
  created_at: string
  suggestions?: UpstreamRelayRecommendationSuggestion[]
}

export interface UpstreamRelayRecommendationPolicy {
  snapshot_freshness_minutes: number
  usage_delta_freshness_minutes: number
  probe_freshness_minutes: number
  min_success_rate: number
  min_sample_size: number
  exclude_consecutive_failures: boolean
  priority_start: number
  priority_step: number
  sort_fields: UpstreamRelayRecommendationSortField[]
  updated_by?: number
  created_at?: string
  updated_at?: string
}

export interface UpstreamRelayRecommendationExclusion {
  candidate_id: number
  connector_id: number
  connector_name?: string
  account_id: number
  account_name?: string
  upstream_group_id: string
  upstream_group_name?: string
  target_group_id: number
  target_group_name?: string
  old_priority?: number | null
  expected_priority?: number | null
  final_rate_multiplier?: number | null
  rate_source?: string
  confidence?: string
  health_status: string
  health_summary: string
  reason_code: string
  reason: string
}

export interface UpstreamRelayRecommendationPreview {
  policy: UpstreamRelayRecommendationPolicy
  total_candidates: number
  suggestion_count: number
  excluded_count: number
  suggestions: UpstreamRelayRecommendationSuggestion[]
  exclusions: UpstreamRelayRecommendationExclusion[]
}

const base = '/admin/upstream-relay-group-monitors'

export async function listConnectors(params?: {
  page?: number
  page_size?: number
  status?: string
  search?: string
}): Promise<PaginatedResponse<UpstreamRelayConnector>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamRelayConnector>>(`${base}/connectors`, { params })
  return data
}

export async function createConnector(payload: UpstreamRelayConnectorInput): Promise<UpstreamRelayConnector> {
  const { data } = await apiClient.post<UpstreamRelayConnector>(`${base}/connectors`, payload)
  return data
}

export async function updateConnector(id: number, payload: UpstreamRelayConnectorInput): Promise<UpstreamRelayConnector> {
  const { data } = await apiClient.put<UpstreamRelayConnector>(`${base}/connectors/${id}`, payload)
  return data
}

export async function deleteConnector(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`${base}/connectors/${id}`)
  return data
}

export async function syncConnector(id: number): Promise<UpstreamRelayGroupRateSnapshot[]> {
  const { data } = await apiClient.post<UpstreamRelayGroupRateSnapshot[]>(`${base}/connectors/${id}/sync`)
  return data
}

export async function refreshConnectorMetrics(id: number): Promise<UpstreamRelayConnectorMetricsRefreshResult> {
  const { data } = await apiClient.post<UpstreamRelayConnectorMetricsRefreshResult>(`${base}/connectors/${id}/metrics/refresh`)
  return data
}

export async function listSnapshots(id: number): Promise<UpstreamRelayGroupRateSnapshot[]> {
  const { data } = await apiClient.get<UpstreamRelayGroupRateSnapshot[]>(`${base}/connectors/${id}/snapshots`)
  return data
}

export async function listSnapshotChanges(params?: {
  page?: number
  page_size?: number
  connector_id?: number
  change_type?: UpstreamRelaySnapshotChangeType | ''
  search?: string
}): Promise<PaginatedResponse<UpstreamRelayGroupRateSnapshotChange>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamRelayGroupRateSnapshotChange>>(`${base}/snapshot-changes`, { params })
  return data
}

export async function listCandidates(params?: {
  page?: number
  page_size?: number
  connector_id?: number
  enabled?: boolean
}): Promise<PaginatedResponse<UpstreamRelayCandidate>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamRelayCandidate>>(`${base}/candidates`, { params })
  return data
}

export async function createCandidate(payload: UpstreamRelayCandidateInput): Promise<UpstreamRelayCandidate> {
  const { data } = await apiClient.post<UpstreamRelayCandidate>(`${base}/candidates`, payload)
  return data
}

export async function updateCandidate(id: number, payload: UpstreamRelayCandidateInput): Promise<UpstreamRelayCandidate> {
  const { data } = await apiClient.put<UpstreamRelayCandidate>(`${base}/candidates/${id}`, payload)
  return data
}

export async function deleteCandidate(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`${base}/candidates/${id}`)
  return data
}

export async function probeCandidate(id: number): Promise<UpstreamRelayProbeResult> {
  const { data } = await apiClient.post<UpstreamRelayProbeResult>(`${base}/candidates/${id}/probe`)
  return data
}

export async function getRecommendationPolicy(): Promise<UpstreamRelayRecommendationPolicy> {
  const { data } = await apiClient.get<UpstreamRelayRecommendationPolicy>(`${base}/recommendation-policy`)
  return data
}

export async function updateRecommendationPolicy(payload: UpstreamRelayRecommendationPolicy): Promise<UpstreamRelayRecommendationPolicy> {
  const { data } = await apiClient.put<UpstreamRelayRecommendationPolicy>(`${base}/recommendation-policy`, payload)
  return data
}

export async function previewRecommendations(payload?: UpstreamRelayRecommendationPolicy): Promise<UpstreamRelayRecommendationPreview> {
  const { data } = await apiClient.post<UpstreamRelayRecommendationPreview>(`${base}/recommendations/preview`, payload)
  return data
}

export async function generateRecommendations(): Promise<UpstreamRelayRecommendationRun> {
  const { data } = await apiClient.post<UpstreamRelayRecommendationRun>(`${base}/recommendations`)
  return data
}

export async function listRecommendationRuns(params?: {
  page?: number
  page_size?: number
}): Promise<PaginatedResponse<UpstreamRelayRecommendationRun>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamRelayRecommendationRun>>(`${base}/recommendations`, { params })
  return data
}

export async function getRecommendationRun(id: number): Promise<UpstreamRelayRecommendationRun> {
  const { data } = await apiClient.get<UpstreamRelayRecommendationRun>(`${base}/recommendations/${id}`)
  return data
}

export async function applyRecommendationRun(id: number): Promise<UpstreamRelayRecommendationRun> {
  const { data } = await apiClient.post<UpstreamRelayRecommendationRun>(`${base}/recommendations/${id}/apply`)
  return data
}

export async function deleteRecommendationRun(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`${base}/recommendations/${id}`)
  return data
}

export const upstreamRelayGroupMonitorsAPI = {
  listConnectors,
  createConnector,
  updateConnector,
  deleteConnector,
  syncConnector,
  refreshConnectorMetrics,
  listSnapshots,
  listSnapshotChanges,
  listCandidates,
  createCandidate,
  updateCandidate,
  deleteCandidate,
  probeCandidate,
  getRecommendationPolicy,
  updateRecommendationPolicy,
  previewRecommendations,
  generateRecommendations,
  listRecommendationRuns,
  getRecommendationRun,
  applyRecommendationRun,
  deleteRecommendationRun
}

export default upstreamRelayGroupMonitorsAPI
