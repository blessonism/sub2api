/**
 * Admin Dashboard API endpoints
 * Provides system-wide statistics and metrics
 */

import { apiClient } from '../client'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  GroupStat,
  ApiKeyUsageTrendPoint,
  UserUsageTrendPoint,
  UserSpendingRankingResponse,
  UserBreakdownItem,
  UsageRequestType
} from '@/types'

/**
 * Get dashboard statistics
 * @returns Dashboard statistics including users, keys, accounts, and token usage
 */
export async function getStats(): Promise<DashboardStats> {
  const { data } = await apiClient.get<DashboardStats>('/admin/dashboard/stats')
  return data
}

/**
 * Get real-time metrics
 * @returns Real-time system metrics
 */
export async function getRealtimeMetrics(): Promise<{
  active_requests: number
  requests_per_minute: number
  average_response_time: number
  error_rate: number
}> {
  const { data } = await apiClient.get<{
    active_requests: number
    requests_per_minute: number
    average_response_time: number
    error_rate: number
  }>('/admin/dashboard/realtime')
  return data
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  user_id?: number
  api_key_id?: number
  model?: string
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
	upstream_model_mismatch?: boolean
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data
 */
export async function getUsageTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/admin/dashboard/trend', { params })
  return data
}

export interface ModelStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
	upstream_model_mismatch?: boolean
}

export interface ModelStatsResponse {
  models: ModelStat[]
  start_date: string
  end_date: string
}

/**
 * Get model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics
 */
export async function getModelStats(params?: ModelStatsParams): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/admin/dashboard/models', { params })
  return data
}

export interface GroupStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
	upstream_model_mismatch?: boolean
}

export interface GroupStatsResponse {
  groups: GroupStat[]
  start_date: string
  end_date: string
}

export interface DashboardSnapshotV2Params extends TrendParams {
  include_stats?: boolean
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
  include_users_trend?: boolean
  users_trend_limit?: number
}

export interface DashboardSnapshotV2Stats extends DashboardStats {
  uptime: number
}

export interface DashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  stats?: DashboardSnapshotV2Stats
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
  users_trend?: UserUsageTrendPoint[]
}

/**
 * Get group usage statistics
 * @param params - Query parameters for filtering
 * @returns Group usage statistics
 */
export async function getGroupStats(params?: GroupStatsParams): Promise<GroupStatsResponse> {
  const { data } = await apiClient.get<GroupStatsResponse>('/admin/dashboard/groups', { params })
  return data
}

export interface UserBreakdownParams {
  start_date?: string
  end_date?: string
  group_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  endpoint?: string
  endpoint_type?: 'inbound' | 'upstream' | 'path'
  limit?: number
  // Sort column for the ranking (allowlisted server-side; falls back to actual_cost)
  sort_by?: 'total_tokens' | 'input_tokens' | 'output_tokens' | 'cache_tokens' | 'requests' | 'cost' | 'actual_cost'
  // Additional filter conditions
  user_id?: number
  api_key_id?: number
  account_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface UserBreakdownResponse {
  users: UserBreakdownItem[]
  start_date: string
  end_date: string
}

export async function getUserBreakdown(params: UserBreakdownParams): Promise<UserBreakdownResponse> {
  const { data } = await apiClient.get<UserBreakdownResponse>('/admin/dashboard/user-breakdown', {
    params
  })
  return data
}

/**
 * Get dashboard snapshot v2 (aggregated response for heavy admin pages).
 */
export async function getSnapshotV2(params?: DashboardSnapshotV2Params): Promise<DashboardSnapshotV2Response> {
  const { data } = await apiClient.get<DashboardSnapshotV2Response>('/admin/dashboard/snapshot-v2', {
    params
  })
  return data
}

export interface ApiKeyTrendParams extends TrendParams {
  limit?: number
}

export interface ApiKeyTrendResponse {
  trend: ApiKeyUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get API key usage trend data
 * @param params - Query parameters for filtering
 * @returns API key usage trend data
 */
export async function getApiKeyUsageTrend(
  params?: ApiKeyTrendParams
): Promise<ApiKeyTrendResponse> {
  const { data } = await apiClient.get<ApiKeyTrendResponse>('/admin/dashboard/api-keys-trend', {
    params
  })
  return data
}

export interface UserTrendParams extends TrendParams {
  limit?: number
}

export interface UserTrendResponse {
  trend: UserUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface UserSpendingRankingParams
  extends Pick<TrendParams, 'start_date' | 'end_date'> {
  limit?: number
}

export interface AdminTokenLeaderboardParams
  extends Pick<TrendParams, 'start_date' | 'end_date'> {
  email?: string
  group_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  user_status?: 'active' | 'disabled' | ''
  limit?: 10 | 20 | 50 | 100
}

export interface AdminTokenLeaderboardUser {
  rank: number
  user_id: number
  email: string
  username: string
  status: string
  registered_at: string
  last_used_at: string
  requests: number
  tokens: number
  cost: number
  actual_cost: number
  account_cost: number
}

export interface AdminTokenLeaderboardResponse {
  ranking: AdminTokenLeaderboardUser[]
  total_requests: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  total_account_cost: number
  start_date: string
  end_date: string
  limit: number
}

export interface AdminTokenLeaderboardMetricRow {
  requests: number
  tokens: number
  cost: number
  actual_cost: number
  account_cost: number
}

export interface AdminTokenLeaderboardAPIKeyUsage extends AdminTokenLeaderboardMetricRow {
  api_key_id: number
  api_key_name: string
}

export interface AdminTokenLeaderboardGroupUsage extends AdminTokenLeaderboardMetricRow {
  group_id: number
  group_name: string
}

export interface AdminTokenLeaderboardModelUsage extends AdminTokenLeaderboardMetricRow {
  model: string
}

export interface AdminTokenLeaderboardUserDetailsResponse {
  user_id: number
  calibration_tokens: number
  calibration_balance_delta: number
  api_keys: AdminTokenLeaderboardAPIKeyUsage[]
  groups: AdminTokenLeaderboardGroupUsage[]
  models: AdminTokenLeaderboardModelUsage[]
  start_date: string
  end_date: string
}

export interface AdminTokenLeaderboardGrantBalanceRequest {
  user_ids: number[]
  amount: number
  notes?: string
}

export interface AdminTokenLeaderboardGrantBalanceUser {
  user_id: number
  email: string
  username: string
  balance: number
  granted_amount: number
}

export interface AdminTokenLeaderboardGrantBalanceResponse {
  granted_count: number
  amount: number
  users: AdminTokenLeaderboardGrantBalanceUser[]
}

export interface AdminBalanceSummaryBucket {
  key: string
  user_count: number
  balance: number
  excluded_count: number
}

export interface AdminBalanceSummaryExcludedUser {
  user_id: number
  email?: string
  username?: string
  role?: 'admin' | 'user' | string
  status?: 'active' | 'disabled' | string
  balance?: number
  valid: boolean
  reason?: string
}

export interface AdminBalanceSummaryResponse {
  total_users: number
  included_users: number
  excluded_user_count: number
  invalid_exclusions: number
  total_balance: number
  by_role: AdminBalanceSummaryBucket[]
  by_status: AdminBalanceSummaryBucket[]
  excluded_user_ids: number[]
  excluded_users: AdminBalanceSummaryExcludedUser[]
  generated_at: string
}

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns User usage trend data
 */
export async function getUserUsageTrend(params?: UserTrendParams): Promise<UserTrendResponse> {
  const { data } = await apiClient.get<UserTrendResponse>('/admin/dashboard/users-trend', {
    params
  })
  return data
}

/**
 * Get user spending ranking data
 * @param params - Query parameters for filtering
 * @returns User spending ranking data
 */
export async function getUserSpendingRanking(
  params?: UserSpendingRankingParams
): Promise<UserSpendingRankingResponse> {
  const { data } = await apiClient.get<UserSpendingRankingResponse>('/admin/dashboard/users-ranking', {
    params
  })
  return data
}

export async function getAdminTokenLeaderboard(
  params?: AdminTokenLeaderboardParams
): Promise<AdminTokenLeaderboardResponse> {
  const { data } = await apiClient.get<AdminTokenLeaderboardResponse>(
    '/admin/dashboard/token-leaderboard',
    { params }
  )
  return data
}

export async function getAdminTokenLeaderboardUserDetails(
  userId: number,
  params?: AdminTokenLeaderboardParams
): Promise<AdminTokenLeaderboardUserDetailsResponse> {
  const { data } = await apiClient.get<AdminTokenLeaderboardUserDetailsResponse>(
    `/admin/dashboard/token-leaderboard/users/${userId}/details`,
    { params }
  )
  return data
}

export async function grantAdminTokenLeaderboardBalance(
  params: AdminTokenLeaderboardParams | undefined,
  request: AdminTokenLeaderboardGrantBalanceRequest
): Promise<AdminTokenLeaderboardGrantBalanceResponse> {
  const { data } = await apiClient.post<AdminTokenLeaderboardGrantBalanceResponse>(
    '/admin/dashboard/token-leaderboard/grant-balance',
    request,
    { params }
  )
  return data
}

export async function getAdminBalanceSummary(): Promise<AdminBalanceSummaryResponse> {
  const { data } = await apiClient.get<AdminBalanceSummaryResponse>(
    '/admin/dashboard/balance-summary'
  )
  return data
}

export async function updateAdminBalanceSummaryExclusions(
  userIds: number[]
): Promise<AdminBalanceSummaryResponse> {
  const { data } = await apiClient.put<AdminBalanceSummaryResponse>(
    '/admin/dashboard/balance-summary/exclusions',
    { user_ids: userIds }
  )
  return data
}

export interface PlatformUsage {
  platform: string
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchUserUsageStats {
  user_id: number
  today_actual_cost: number
  total_actual_cost: number
  by_platform?: PlatformUsage[]
}

export interface BatchUsersUsageResponse {
  stats: Record<string, BatchUserUsageStats>
}

/**
 * Get batch usage stats for multiple users
 * @param userIds - Array of user IDs
 * @returns Usage stats map keyed by user ID
 */
export async function getBatchUsersUsage(userIds: number[]): Promise<BatchUsersUsageResponse> {
  const { data } = await apiClient.post<BatchUsersUsageResponse>('/admin/dashboard/users-usage', {
    user_ids: userIds
  })
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for multiple API keys
 * @param apiKeyIds - Array of API key IDs
 * @returns Usage stats map keyed by API key ID
 */
export async function getBatchApiKeysUsage(
  apiKeyIds: number[]
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/admin/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    }
  )
  return data
}

export const dashboardAPI = {
  getStats,
  getRealtimeMetrics,
  getUsageTrend,
  getModelStats,
  getGroupStats,
  getSnapshotV2,
  getApiKeyUsageTrend,
  getUserUsageTrend,
  getUserSpendingRanking,
  getAdminTokenLeaderboard,
  getAdminTokenLeaderboardUserDetails,
  grantAdminTokenLeaderboardBalance,
  getAdminBalanceSummary,
  updateAdminBalanceSummaryExclusions,
  getBatchUsersUsage,
  getBatchApiKeysUsage
}

export default dashboardAPI
