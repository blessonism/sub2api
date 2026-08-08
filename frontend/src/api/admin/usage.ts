/**
 * Admin Usage API endpoints
 * Handles admin-level usage logs and statistics retrieval
 */

import { apiClient } from '../client'
import type { AdminUsageLog, UsageLog, UsageQueryParams, UsageStatsResponse, PaginatedResponse, UsageRequestType } from '@/types'
import type { EndpointStat } from '@/types'

// ==================== Types ====================

export interface SharedIPUsersSummary {
  ip_count: number
  user_count: number
  record_count: number
  ip_groups?: SharedIPGroupSummaryItem[]
  ip_groups_limit: number
  ip_groups_truncated: boolean
  hidden_ip_group_count: number
  users?: SharedIPUserSummaryItem[]
  users_limit: number
  users_truncated: boolean
  hidden_user_count: number
}

export interface SharedIPGroupSummaryItem {
  ip_address: string
  user_count: number
  record_count: number
  last_used_at?: string | null
  total_tokens: number
  actual_cost: number
  users?: SharedIPGroupUserSummaryItem[]
  users_limit: number
  users_truncated: boolean
  hidden_user_count: number
}

export interface SharedIPGroupUserSummaryItem {
  user_id: number
  email: string
  deleted: boolean
  record_count: number
  last_used_at?: string | null
  total_tokens: number
  actual_cost: number
}

export interface SharedIPUserSummaryItem {
  user_id: number
  email: string
  deleted: boolean
  ip_count: number
  record_count: number
  last_used_at?: string | null
  ip_addresses?: string[]
  total_tokens: number
  actual_cost: number
}

export interface AdminUsageListResponse extends PaginatedResponse<AdminUsageLog> {
  shared_ip_users_summary?: SharedIPUsersSummary | null
}

export interface AdminUsageStatsResponse {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_tokens: number
  calibration_tokens?: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  total_account_cost: number
  average_duration_ms: number
  endpoints?: EndpointStat[]
  upstream_endpoints?: EndpointStat[]
  endpoint_paths?: EndpointStat[]
}

export interface SimpleUser {
  id: number
  email: string
  deleted: boolean
}

export interface SimpleApiKey {
  id: number
  name: string
  user_id: number
}

export type AdminUsageCalibrationMode = 'delta' | 'target'

export interface AdminUsageTokenCalibrationInput {
  mode: AdminUsageCalibrationMode
  value: number
  start_date: string
  end_date: string
  timezone?: string
}

export interface AdminUsageBalanceCalibrationInput {
  mode: AdminUsageCalibrationMode
  value: number
}

export interface AdminUsageConsumptionCalibrationInput extends AdminUsageBalanceCalibrationInput {
  start_date: string
  end_date: string
  timezone?: string
}

export interface CreateAdminUsageCalibrationRequest {
  target_user_id: number
  token?: AdminUsageTokenCalibrationInput
  balance?: AdminUsageBalanceCalibrationInput
  consumption?: AdminUsageConsumptionCalibrationInput
}

export interface AdminUsageCalibrationDailyAllocation {
  id: number
  calibration_id: number
  target_user_id: number
  date: string
  original_tokens: number
  token_delta: number
  balance_delta?: number
  created_at: string
}

export interface AdminUsageCalibration {
  id: number
  target_user_id: number
  admin_user_id: number
  reason?: string
  token_mode?: AdminUsageCalibrationMode
  token_input_value?: number
  token_before_value?: number
  token_after_value?: number
  token_delta?: number
  token_calculation_start_date?: string
  token_calculation_end_date?: string
  token_calculation_timezone?: string
  balance_mode?: AdminUsageCalibrationMode
  balance_input_value?: number
  balance_before_value?: number
  balance_after_value?: number
  balance_delta?: number
  consumption_mode?: AdminUsageCalibrationMode
  consumption_input_value?: number
  consumption_before_value?: number
  consumption_after_value?: number
  consumption_delta?: number
  consumption_start_date?: string
  consumption_end_date?: string
  consumption_timezone?: string
  created_at: string
  allocations?: AdminUsageCalibrationDailyAllocation[]
}

export interface UsageCleanupFilters {
  start_time: string
  end_time: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
}

export interface UsageCleanupTask {
  id: number
  status: string
  filters: UsageCleanupFilters
  created_by: number
  deleted_rows: number
  error_message?: string | null
  canceled_by?: number | null
  canceled_at?: string | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateUsageCleanupTaskRequest {
  start_date: string
  end_date: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
  timezone?: string
}

export interface AdminUsageQueryParams extends UsageQueryParams {
  user_id?: number
  exact_total?: boolean
  billing_mode?: string
  shared_ip_users?: boolean
  shared_ip_summary_only?: boolean
  upstream_model_mismatch?: boolean
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  // 错误请求 tab 专属筛选(仅传给错误列表接口;共用同一 filters 对象)
  error_phase?: string | null
  error_category?: string | null
  status_code?: number | null
}

export interface AdminUserViewQueryParams extends UsageQueryParams {
  user_id: number
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

// ==================== API Functions ====================

/**
 * List all usage logs with optional filters (admin only)
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function list(
  params: AdminUsageQueryParams,
  options?: { signal?: AbortSignal }
): Promise<AdminUsageListResponse> {
  const { data } = await apiClient.get<AdminUsageListResponse>('/admin/usage', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Get usage statistics with optional filters (admin only)
 * @param params - Query parameters for filtering
 * @returns Usage statistics
 */
export async function getStats(params: {
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string
  request_type?: UsageRequestType
  stream?: boolean
  upstream_model_mismatch?: boolean
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
  nocache?: number
}): Promise<AdminUsageStatsResponse> {
  const { data } = await apiClient.get<AdminUsageStatsResponse>('/admin/usage/stats', {
    params
  })
  return data
}

export async function getUserView(
  params: AdminUserViewQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<UsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/admin/usage/user-view', {
    params,
    signal: options?.signal
  })
  return data
}

export async function getUserViewStats(params: {
  user_id: number
  api_key_id?: number
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
}): Promise<UsageStatsResponse> {
  const { data } = await apiClient.get<UsageStatsResponse>('/admin/usage/user-view/stats', {
    params
  })
  return data
}

/**
 * Search users by email keyword (admin only)
 * @param keyword - Email keyword to search
 * @returns List of matching users (max 30)
 */
export async function searchUsers(keyword: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>('/admin/usage/search-users', {
    params: { q: keyword }
  })
  return data
}

/**
 * Search API keys by user ID and/or keyword (admin only)
 * @param userId - Optional user ID to filter by
 * @param keyword - Optional keyword to search in key name
 * @returns List of matching API keys (max 30)
 */
export async function searchApiKeys(userId?: number, keyword?: string): Promise<SimpleApiKey[]> {
  const params: Record<string, unknown> = {}
  if (userId !== undefined) {
    params.user_id = userId
  }
  if (keyword) {
    params.q = keyword
  }
  const { data } = await apiClient.get<SimpleApiKey[]>('/admin/usage/search-api-keys', {
    params
  })
  return data
}

/**
 * List usage cleanup tasks (admin only)
 * @param params - Query parameters for pagination
 * @returns Paginated list of cleanup tasks
 */
export async function listCleanupTasks(
  params: { page?: number; page_size?: number },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<UsageCleanupTask>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageCleanupTask>>('/admin/usage/cleanup-tasks', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Create a usage cleanup task (admin only)
 * @param payload - Cleanup task parameters
 * @returns Created cleanup task
 */
export async function createCleanupTask(payload: CreateUsageCleanupTaskRequest): Promise<UsageCleanupTask> {
  const { data } = await apiClient.post<UsageCleanupTask>('/admin/usage/cleanup-tasks', payload)
  return data
}

/**
 * Cancel a usage cleanup task (admin only)
 * @param taskId - Task ID to cancel
 */
export async function cancelCleanupTask(taskId: number): Promise<{ id: number; status: string }> {
  const { data } = await apiClient.post<{ id: number; status: string }>(
    `/admin/usage/cleanup-tasks/${taskId}/cancel`
  )
  return data
}

export async function createCalibration(
  payload: CreateAdminUsageCalibrationRequest,
  idempotencyKey: string
): Promise<AdminUsageCalibration> {
  const { data } = await apiClient.post<AdminUsageCalibration>(
    '/admin/usage/calibrations',
    payload,
    {
      headers: {
        'Idempotency-Key': idempotencyKey
      }
    }
  )
  return data
}

export async function listCalibrations(params?: {
  user_id?: number
  page?: number
  page_size?: number
}): Promise<PaginatedResponse<AdminUsageCalibration>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminUsageCalibration>>('/admin/usage/calibrations', {
    params
  })
  return data
}

export const adminUsageAPI = {
  list,
  getStats,
  getUserView,
  getUserViewStats,
  searchUsers,
  searchApiKeys,
  listCleanupTasks,
  createCleanupTask,
  cancelCleanupTask,
  createCalibration,
  listCalibrations
}

export default adminUsageAPI
