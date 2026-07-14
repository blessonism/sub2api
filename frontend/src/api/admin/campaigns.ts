import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type {
  Campaign,
  CampaignConfigVersion,
  CampaignInviteRecord,
  CampaignLeaderboardRow,
  CampaignPoolSummary,
} from '@/api/campaigns'

export interface CampaignCreateRequest {
  name: string
  description?: string
  cover_url?: string
  rules_text?: string
  warmup_start_at?: string | null
  start_at: string
  end_at: string
  audit_start_at?: string | null
  audit_end_at?: string | null
  publicity_start_at?: string | null
  publicity_end_at?: string | null
  payout_due_at?: string | null
  initial_bonus_cents?: number
  recharge_threshold_cents?: number
  allow_accumulated_recharge?: boolean
  historical_invite_ratio?: number
  pool_injection_rate?: number
  pool_injection_scope?: 'invitees_only' | 'all_users'
  rank_pool_ratio?: number
  contribution_pool_ratio?: number
  rank_reward_count?: number
  rank_weights?: number[]
  min_payout_amount_cents?: number
}

export interface CampaignUpdateRequest {
  name?: string
  description?: string
  cover_url?: string
  rules_text?: string
  warmup_start_at?: string | null
  start_at?: string
  end_at?: string
  audit_start_at?: string | null
  audit_end_at?: string | null
  publicity_start_at?: string | null
  publicity_end_at?: string | null
  payout_due_at?: string | null
  historical_invite_ratio?: number
}

export interface CampaignConfigVersionRequest {
  version_scope: string
  effective_at: string
  recharge_threshold_cents?: number
  pool_injection_rate?: number
  pool_injection_scope?: 'invitees_only' | 'all_users'
  allow_accumulated_recharge?: boolean
  change_reason?: string
}

export interface CampaignPoolAdjustmentRequest {
  adjustment_type: string
  amount_cents: number
  reason?: string
}

export interface CampaignLeaderboardAdjustmentRequest {
  user_id: number
  valid_invite_delta: number
  recharge_amount_delta_cents: number
  reason?: string
}

export interface CampaignManualLeaderboardAdjustment {
  id: number
  campaign_id: number
  user_id: number
  adjustment_type: string
  valid_invite_delta: number
  recharge_amount_delta_cents: number
  reason: string
  operator_id?: number | null
  created_at: string
}

export interface CampaignInviteRecordAdjustmentRequest {
  status: string
  effective_recharge_amount_cents: number
  reason?: string
}

export interface CampaignCalculationSummary {
  campaign_id: number
  calculation_status: string
  calculation_batch_no: string
  final_pool_cents: number
  rank_pool_cents: number
  contribution_pool_cents: number
  total_gross_reward_cents: number
  total_final_payout_cents: number
  total_withheld_cents: number
  total_rounding_residual_cents: number
  results: CampaignRewardResult[]
}

export interface CampaignRewardResult {
  id: number
  campaign_id: number
  config_version_id: number
  calculation_batch_no: string
  user_id: number
  rank?: number | null
  rank_reward_amount_cents: number
  contribution_weight: string
  contribution_reward_amount_cents: number
  gross_reward_amount_cents: number
  min_payout_amount_snapshot_cents: number
  final_payout_amount_cents: number
  withheld_amount_cents: number
  withheld_reason: string
  rounding_residual_cents: number
  calculation_status: string
  calculated_at: string
}

export interface CampaignPayoutBatch {
  id: number
  campaign_id: number
  batch_no: string
  status: string
  operator_id?: number | null
  total_users: number
  total_amount_cents: number
  success_count: number
  failed_count: number
  started_at?: string | null
  finished_at?: string | null
  created_at: string
}

export interface CampaignDeleteImpact {
  participants: number
  invite_records: number
  pool_entries: number
  pool_adjustments: number
  leaderboard_snapshots: number
  reward_results: number
  payout_batches: number
  payout_items: number
}

export interface CampaignDeleteResult {
  action: 'deleted' | 'archived'
  campaign?: Campaign | null
  impact: CampaignDeleteImpact
}

export async function listCampaigns(params: { page?: number; page_size?: number } = {}): Promise<PaginatedResponse<Campaign>> {
  const { data } = await apiClient.get<PaginatedResponse<Campaign>>('/admin/campaigns', {
    params: { page: params.page ?? 1, page_size: params.page_size ?? 20 },
  })
  return data
}

export async function createCampaign(payload: CampaignCreateRequest): Promise<{ campaign: Campaign; config_version: CampaignConfigVersion }> {
  const { data } = await apiClient.post<{ campaign: Campaign; config_version: CampaignConfigVersion }>('/admin/campaigns', payload)
  return data
}

export async function updateCampaign(id: number, payload: CampaignUpdateRequest): Promise<Campaign> {
  const { data } = await apiClient.put<Campaign>(`/admin/campaigns/${id}`, payload)
  return data
}

export async function copyCampaign(id: number): Promise<{ campaign: Campaign; config_version: CampaignConfigVersion }> {
  const { data } = await apiClient.post<{ campaign: Campaign; config_version: CampaignConfigVersion }>(`/admin/campaigns/${id}/copy`)
  return data
}

export async function getCampaign(id: number): Promise<Campaign> {
  const { data } = await apiClient.get<Campaign>(`/admin/campaigns/${id}`)
  return data
}

export async function getCampaignConfig(id: number): Promise<CampaignConfigVersion> {
  const { data } = await apiClient.get<CampaignConfigVersion>(`/admin/campaigns/${id}/config`)
  return data
}

export async function deleteCampaign(id: number): Promise<CampaignDeleteResult> {
  const { data } = await apiClient.delete<CampaignDeleteResult>(`/admin/campaigns/${id}`)
  return data
}

export async function publishCampaign(id: number): Promise<Campaign> {
  const { data } = await apiClient.post<Campaign>(`/admin/campaigns/${id}/publish`)
  return data
}

export async function createConfigVersion(id: number, payload: CampaignConfigVersionRequest): Promise<CampaignConfigVersion> {
  const { data } = await apiClient.post<CampaignConfigVersion>(`/admin/campaigns/${id}/config-versions`, payload)
  return data
}

export async function getPoolSummary(id: number): Promise<CampaignPoolSummary> {
  const { data } = await apiClient.get<CampaignPoolSummary>(`/admin/campaigns/${id}/pool`)
  return data
}

export async function addPoolAdjustment(id: number, payload: CampaignPoolAdjustmentRequest): Promise<{ ok: boolean }> {
  const { data } = await apiClient.post<{ ok: boolean }>(`/admin/campaigns/${id}/pool-adjustments`, payload)
  return data
}

export async function getLeaderboard(id: number): Promise<{ items: CampaignLeaderboardRow[] }> {
  const { data } = await apiClient.get<{ items: CampaignLeaderboardRow[] }>(`/admin/campaigns/${id}/leaderboard`)
  return data
}

export async function addLeaderboardAdjustment(id: number, payload: CampaignLeaderboardAdjustmentRequest): Promise<CampaignManualLeaderboardAdjustment> {
  const { data } = await apiClient.post<CampaignManualLeaderboardAdjustment>(`/admin/campaigns/${id}/leaderboard-adjustments`, payload)
  return data
}

export async function listInviterRecords(id: number, userID: number): Promise<PaginatedResponse<CampaignInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<CampaignInviteRecord>>(`/admin/campaigns/${id}/inviters/${userID}/invites`, {
    params: { page: 1, page_size: 100 },
  })
  return data
}

export async function adjustInviteRecord(id: number, recordID: number, payload: CampaignInviteRecordAdjustmentRequest): Promise<CampaignInviteRecord> {
  const { data } = await apiClient.patch<CampaignInviteRecord>(`/admin/campaigns/${id}/invite-records/${recordID}`, payload)
  return data
}

export async function freezeLeaderboard(id: number): Promise<{ ok: boolean }> {
  const { data } = await apiClient.post<{ ok: boolean }>(`/admin/campaigns/${id}/freeze`)
  return data
}

export async function pauseCampaign(id: number): Promise<Campaign> {
  const { data } = await apiClient.post<Campaign>(`/admin/campaigns/${id}/pause`)
  return data
}

export async function unfreezeCampaign(id: number): Promise<Campaign> {
  const { data } = await apiClient.post<Campaign>(`/admin/campaigns/${id}/unfreeze`)
  return data
}

export async function resumeCampaign(id: number): Promise<Campaign> {
  const { data } = await apiClient.post<Campaign>(`/admin/campaigns/${id}/resume`)
  return data
}

export async function recalculateRewards(id: number, status: 'preview' | 'frozen' | 'final' = 'preview'): Promise<CampaignCalculationSummary> {
  const { data } = await apiClient.post<CampaignCalculationSummary>(`/admin/campaigns/${id}/recalculate`, undefined, {
    params: { status },
  })
  return data
}

export async function getFinalRewardResults(id: number): Promise<CampaignCalculationSummary> {
  const { data } = await apiClient.get<CampaignCalculationSummary>(`/admin/campaigns/${id}/reward-results/final`)
  return data
}

export async function payoutCampaign(id: number): Promise<CampaignPayoutBatch> {
  const { data } = await apiClient.post<CampaignPayoutBatch>(`/admin/campaigns/${id}/payout`)
  return data
}

export const campaignsAdminAPI = {
  listCampaigns,
  createCampaign,
  updateCampaign,
  copyCampaign,
  getCampaign,
  getCampaignConfig,
  deleteCampaign,
  publishCampaign,
  createConfigVersion,
  getPoolSummary,
  addPoolAdjustment,
  getLeaderboard,
  addLeaderboardAdjustment,
  listInviterRecords,
  adjustInviteRecord,
  freezeLeaderboard,
  pauseCampaign,
  unfreezeCampaign,
  resumeCampaign,
  recalculateRewards,
  getFinalRewardResults,
  payoutCampaign,
}

export default campaignsAdminAPI
