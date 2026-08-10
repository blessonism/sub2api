import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type TokenUsagePolicyActionMode = 'rate_only' | 'grant_group_and_rate'
export type TokenUsagePolicyConditionMode = 'token' | 'actual_cost' | 'both'
export type TokenUsagePolicyConflictMode = 'manual_priority' | 'auto_priority'
export type TokenUsagePolicyScheduleFrequency = 'every_6h' | 'daily' | 'weekly'
export type TokenUsagePolicyRunType = 'preview' | 'manual' | 'scheduled' | 'clear'
export type TokenUsagePolicyRunStatus = 'running' | 'success' | 'failed'
export type TokenUsagePolicyChangeType = 'create' | 'update' | 'downgrade' | 'clear' | 'skip_manual'

export interface TokenUsagePolicyFilters {
  group_id?: number | null
  model?: string | null
  request_type?: number | null
  billing_type?: number | null
}

export interface TokenUsagePolicyTier {
  id?: number
  policy_id?: number
  condition_mode: TokenUsagePolicyConditionMode
  min_tokens: number
  min_actual_cost: number
  is_resident: boolean
  rate_multiplier: number
  sort_order?: number
  created_at?: string
  updated_at?: string
}

export interface TokenUsagePolicyRun {
  id: number
  policy_id: number
  run_type: TokenUsagePolicyRunType
  status: TokenUsagePolicyRunStatus
  total_users: number
  create_count: number
  update_count: number
  downgrade_count: number
  clear_count: number
  skip_count: number
  error_message?: string
  started_at: string
  finished_at?: string | null
  created_at: string
}

export interface TokenUsagePolicy {
  id: number
  name: string
  enabled: boolean
  window_days: 7 | 30
  target_group_id: number
  target_group_name?: string
  action_mode: TokenUsagePolicyActionMode
  conflict_mode: TokenUsagePolicyConflictMode
  schedule_frequency: TokenUsagePolicyScheduleFrequency
  filters: TokenUsagePolicyFilters
  tiers: TokenUsagePolicyTier[]
  last_run_at?: string | null
  next_run_at?: string | null
  latest_run?: TokenUsagePolicyRun | null
  created_at: string
  updated_at: string
}

export interface TokenUsagePolicyInput {
  name: string
  enabled?: boolean
  window_days: 7 | 30
  target_group_id: number
  action_mode: TokenUsagePolicyActionMode
  conflict_mode: TokenUsagePolicyConflictMode
  schedule_frequency: TokenUsagePolicyScheduleFrequency
  filters: TokenUsagePolicyFilters
  tiers: TokenUsagePolicyTier[]
}

export interface TokenUsagePolicyRunStats {
  total_users: number
  create_count: number
  update_count: number
  downgrade_count: number
  clear_count: number
  skip_count: number
}

export interface TokenUsagePolicyChange {
  change_type: TokenUsagePolicyChangeType
  user_id: number
  user_name?: string
  user_email?: string
  token_usage: number
  actual_cost: number
  total_token_usage: number
  total_actual_cost: number
  target_group_id: number
  tier_id?: number | null
  tier_min_tokens?: number | null
  tier_condition_mode?: TokenUsagePolicyConditionMode | null
  tier_min_actual_cost?: number | null
  resident_tier_id?: number | null
  resident_tier_min_tokens?: number | null
  resident_tier_condition_mode?: TokenUsagePolicyConditionMode | null
  resident_tier_min_actual_cost?: number | null
  old_rate_multiplier?: number | null
  new_rate_multiplier?: number | null
  reason?: string
  group_granted: boolean
  manual_takeover: boolean
}

export interface TokenUsagePolicyPreview {
  policy_id: number
  stats: TokenUsagePolicyRunStats
  changes: TokenUsagePolicyChange[]
}

export async function list(params?: {
  page?: number
  page_size?: number
  enabled?: boolean
  target_group_id?: number
}): Promise<PaginatedResponse<TokenUsagePolicy>> {
  const { data } = await apiClient.get<PaginatedResponse<TokenUsagePolicy>>('/admin/token-usage-policies', {
    params
  })
  return data
}

export async function get(id: number): Promise<TokenUsagePolicy> {
  const { data } = await apiClient.get<TokenUsagePolicy>(`/admin/token-usage-policies/${id}`)
  return data
}

export async function create(payload: TokenUsagePolicyInput): Promise<TokenUsagePolicy> {
  const { data } = await apiClient.post<TokenUsagePolicy>('/admin/token-usage-policies', payload)
  return data
}

export async function update(id: number, payload: TokenUsagePolicyInput): Promise<TokenUsagePolicy> {
  const { data } = await apiClient.put<TokenUsagePolicy>(`/admin/token-usage-policies/${id}`, payload)
  return data
}

export async function remove(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/token-usage-policies/${id}`)
  return data
}

export async function preview(id: number): Promise<TokenUsagePolicyPreview> {
  const { data } = await apiClient.post<TokenUsagePolicyPreview>(`/admin/token-usage-policies/${id}/preview`)
  return data
}

export async function run(id: number): Promise<TokenUsagePolicyRun> {
  const { data } = await apiClient.post<TokenUsagePolicyRun>(`/admin/token-usage-policies/${id}/run`)
  return data
}

export async function clear(id: number): Promise<TokenUsagePolicyRun> {
  const { data } = await apiClient.post<TokenUsagePolicyRun>(`/admin/token-usage-policies/${id}/clear`)
  return data
}

export async function listRuns(
  id: number,
  params?: { page?: number; page_size?: number }
): Promise<PaginatedResponse<TokenUsagePolicyRun>> {
  const { data } = await apiClient.get<PaginatedResponse<TokenUsagePolicyRun>>(
    `/admin/token-usage-policies/${id}/runs`,
    { params }
  )
  return data
}

export async function listRunChanges(
  id: number,
  runId: number,
  params?: { page?: number; page_size?: number }
): Promise<PaginatedResponse<TokenUsagePolicyChange>> {
  const { data } = await apiClient.get<PaginatedResponse<TokenUsagePolicyChange>>(
    `/admin/token-usage-policies/${id}/runs/${runId}/changes`,
    { params }
  )
  return data
}

export const tokenUsagePoliciesAPI = {
  list,
  get,
  create,
  update,
  delete: remove,
  preview,
  run,
  clear,
  listRuns,
  listRunChanges
}

export default tokenUsagePoliciesAPI
