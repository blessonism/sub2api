import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type UpstreamCostCalibrationAdapterType = 'manual'
export type UpstreamCostCalibrationRunStatus = 'running' | 'success' | 'failed'
export type UpstreamCostCalibrationTestStatus = 'success' | 'failed' | 'skipped'

export interface UpstreamCostCalibrationAccount {
  account_id: number
  account_name?: string
  platform?: string
  current_priority?: number | null
  adapter_config: Record<string, unknown>
  created_at?: string
}

export interface UpstreamCostCalibrationTask {
  id: number
  name: string
  enabled: boolean
  target_group_id: number
  target_group_name?: string
  model: string
  adapter_type: UpstreamCostCalibrationAdapterType
  unit: string
  test_prompt: string
  sample_count: number
  priority_start: number
  priority_step: number
  accounts?: UpstreamCostCalibrationAccount[]
  latest_run?: UpstreamCostCalibrationRun | null
  last_run_id?: number | null
  last_run_at?: string | null
  created_at: string
  updated_at: string
}

export interface UpstreamCostCalibrationTaskInput {
  name: string
  enabled?: boolean
  target_group_id: number
  model: string
  adapter_type: UpstreamCostCalibrationAdapterType
  unit: string
  test_prompt: string
  sample_count: number
  priority_start: number
  priority_step: number
  accounts: UpstreamCostCalibrationAccount[]
}

export interface UpstreamCostCalibrationRun {
  id: number
  task_id: number
  target_group_id: number
  status: UpstreamCostCalibrationRunStatus
  total_accounts: number
  valid_accounts: number
  invalid_accounts: number
  suggestion_count: number
  applied: boolean
  applied_by?: number | null
  applied_at?: string | null
  error_message?: string
  started_at: string
  finished_at?: string | null
  created_at: string
  results?: UpstreamCostCalibrationResult[]
  suggestions?: UpstreamCostCalibrationSuggestion[]
}

export interface UpstreamCostCalibrationResult {
  id?: number
  run_id?: number
  task_id?: number
  account_id: number
  account_name?: string
  account_platform?: string
  current_priority?: number | null
  before_balance?: number | null
  after_balance?: number | null
  cost_delta?: number | null
  unit: string
  test_status: UpstreamCostCalibrationTestStatus
  latency_ms?: number | null
  valid: boolean
  rank?: number | null
  suggested_priority?: number | null
  error_message?: string
  created_at?: string
}

export interface UpstreamCostCalibrationSuggestion {
  id?: number
  run_id?: number
  task_id?: number
  account_id: number
  old_priority?: number | null
  new_priority: number
  reason: string
  applied: boolean
  applied_by?: number | null
  applied_at?: string | null
  created_at?: string
}

export async function list(params?: {
  page?: number
  page_size?: number
  enabled?: boolean
  target_group_id?: number
}): Promise<PaginatedResponse<UpstreamCostCalibrationTask>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamCostCalibrationTask>>(
    '/admin/upstream-cost-calibrations',
    { params }
  )
  return data
}

export async function get(id: number): Promise<UpstreamCostCalibrationTask> {
  const { data } = await apiClient.get<UpstreamCostCalibrationTask>(
    `/admin/upstream-cost-calibrations/${id}`
  )
  return data
}

export async function create(
  payload: UpstreamCostCalibrationTaskInput
): Promise<UpstreamCostCalibrationTask> {
  const { data } = await apiClient.post<UpstreamCostCalibrationTask>(
    '/admin/upstream-cost-calibrations',
    payload
  )
  return data
}

export async function update(
  id: number,
  payload: UpstreamCostCalibrationTaskInput
): Promise<UpstreamCostCalibrationTask> {
  const { data } = await apiClient.put<UpstreamCostCalibrationTask>(
    `/admin/upstream-cost-calibrations/${id}`,
    payload
  )
  return data
}

export async function remove(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/upstream-cost-calibrations/${id}`
  )
  return data
}

export async function run(id: number): Promise<UpstreamCostCalibrationRun> {
  const { data } = await apiClient.post<UpstreamCostCalibrationRun>(
    `/admin/upstream-cost-calibrations/${id}/run`
  )
  return data
}

export async function listRuns(
  id: number,
  params?: { page?: number; page_size?: number }
): Promise<PaginatedResponse<UpstreamCostCalibrationRun>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamCostCalibrationRun>>(
    `/admin/upstream-cost-calibrations/${id}/runs`,
    { params }
  )
  return data
}

export async function getRun(id: number, runId: number): Promise<UpstreamCostCalibrationRun> {
  const { data } = await apiClient.get<UpstreamCostCalibrationRun>(
    `/admin/upstream-cost-calibrations/${id}/runs/${runId}`
  )
  return data
}

export async function applyRun(id: number, runId: number): Promise<UpstreamCostCalibrationRun> {
  const { data } = await apiClient.post<UpstreamCostCalibrationRun>(
    `/admin/upstream-cost-calibrations/${id}/runs/${runId}/apply`
  )
  return data
}

export const upstreamCostCalibrationsAPI = {
  list,
  get,
  create,
  update,
  delete: remove,
  run,
  listRuns,
  getRun,
  applyRun
}

export default upstreamCostCalibrationsAPI
