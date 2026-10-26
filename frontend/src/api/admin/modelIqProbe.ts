/**
 * Admin Model IQ Probe API endpoints
 * 智商监控（糖果问题探针）管理端：配置、结果查询、手动触发、人工改判
 */

import { apiClient } from '../client'

export type ModelIqProbeStatus = 'pass' | 'wrong_answer' | 'error'
export type ModelIqProbeTargetMode = 'account_model' | 'model_only'

export interface ModelIqProbeConfig {
  enabled: boolean
  question: string
  expected_answer: string
  interval_minutes: number
  target_mode: ModelIqProbeTargetMode
  models: string[]
  account_ids: number[]
}

export interface ModelIqProbeResult {
  id: number
  account_id: number | null
  account_name: string
  model: string
  protocol: string
  status: ModelIqProbeStatus
  override_status: ModelIqProbeStatus | null
  effective_status: ModelIqProbeStatus
  error_category: string
  error_message: string
  expected_answer: string
  actual_answer: string
  latency_ms: number
  input_tokens: number | null
  output_tokens: number | null
  cache_read_tokens: number | null
  created_at: string
}

export interface ModelIqProbeResultListParams {
  page?: number
  page_size?: number
  model?: string
  account_id?: number
  status?: ModelIqProbeStatus | ''
}

export interface ModelIqProbeResultListResponse {
  items: ModelIqProbeResult[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ModelIqProbeCycleSummary {
  total: number
  pass: number
  wrong_answer: number
  error: number
}

export async function getConfig(): Promise<ModelIqProbeConfig> {
  const { data } = await apiClient.get<ModelIqProbeConfig>('/admin/model-iq-probe/config')
  return data
}

export async function updateConfig(config: ModelIqProbeConfig): Promise<ModelIqProbeConfig> {
  const { data } = await apiClient.put<ModelIqProbeConfig>('/admin/model-iq-probe/config', config)
  return data
}

export async function listResults(
  params: ModelIqProbeResultListParams = {},
  options?: { signal?: AbortSignal }
): Promise<ModelIqProbeResultListResponse> {
  const { data } = await apiClient.get<ModelIqProbeResultListResponse>('/admin/model-iq-probe/results', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function runProbe(): Promise<ModelIqProbeCycleSummary> {
  const { data } = await apiClient.post<ModelIqProbeCycleSummary>('/admin/model-iq-probe/run')
  return data
}

/** status 传 null 表示清除人工改判。 */
export async function overrideResult(id: number, status: ModelIqProbeStatus | null): Promise<void> {
  await apiClient.post(`/admin/model-iq-probe/results/${id}/override`, { status })
}

export default {
  getConfig,
  updateConfig,
  listResults,
  runProbe,
  overrideResult,
}
