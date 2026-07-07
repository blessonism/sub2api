import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export interface Campaign {
  id: number
  name: string
  description: string
  cover_url: string
  rules_text: string
  status: string
  warmup_start_at?: string | null
  start_at: string
  end_at: string
  audit_start_at?: string | null
  audit_end_at?: string | null
  publicity_start_at?: string | null
  publicity_end_at?: string | null
  payout_due_at?: string | null
  published_config_version_id?: number | null
  created_by?: number | null
  updated_by?: number | null
  created_at: string
  updated_at: string
}

export interface CampaignConfigVersion {
  id: number
  campaign_id: number
  version: number
  version_scope: string
  effective_at: string
  recharge_threshold_cents: number
  allow_accumulated_recharge: boolean
  pool_injection_rate: string
  pool_injection_scope: 'invitees_only' | 'all_users'
  rank_pool_ratio: string
  contribution_pool_ratio: string
  rank_reward_count: number
  rank_weights: number[]
  min_payout_amount_cents: number
  payout_method: string
  payout_channel: string
  change_reason: string
  created_by?: number | null
  created_at: string
}

export interface CampaignPoolSummary {
  campaign_id: number
  confirmed_pool_cents: number
  pending_pool_cents: number
  estimated_total_pool_cents: number
  adjustment_total_cents: number
  deducted_pool_cents: number
  final_pool_cents: number
}

export interface CampaignLeaderboardRow {
  rank: number
  user_id: number
  masked_email?: string
  username?: string
  valid_invite_count: number
  pending_invite_count: number
  invitee_recharge_amount_cents: number
  manual_valid_invite_delta: number
  manual_recharge_amount_delta_cents: number
  has_manual_adjustment: boolean
  reached_count_at: string
  joined_at: string
  estimated_reward_cents: number
  final_reward_cents: number
}

export interface CampaignInviteRecord {
  id: number
  campaign_id: number
  config_version_id: number
  inviter_user_id: number
  invitee_user_id: number
  invitee_masked_email?: string
  invitee_username?: string
  invite_source: string
  threshold_snapshot_cents: number
  registered_at: string
  qualified_at?: string | null
  effective_recharge_amount_cents: number
  status: string
  risk_level: string
  invalid_reason: string
  audit_status: string
  audit_by?: number | null
  audit_at?: string | null
  audit_note: string
}

export interface CampaignHome {
  campaign: Campaign | null
  config?: CampaignConfigVersion | null
  pool?: CampaignPoolSummary | null
  leaderboard?: CampaignLeaderboardRow[]
  data_delay_notice: string
  estimate_notice: string
}

export interface CampaignMyData {
  campaign_id: number
  user_id: number
  invite_code: string
  invite_link: string
  valid_invite_count: number
  pending_invite_count: number
  invalid_invite_count: number
  current_rank?: number | null
  estimated_rank_reward_cents: number
  estimated_contribution_reward_cents: number
  estimated_total_reward_cents: number
  invitee_recharge_amount_cents: number
  distance_to_previous: number
  distance_to_top10: number
  invite_records: CampaignInviteRecord[]
}

export async function getActiveCampaign(): Promise<CampaignHome> {
  const { data } = await apiClient.get<CampaignHome>('/campaigns/active')
  return data
}

export async function getMyCampaignData(campaignId: number): Promise<CampaignMyData> {
  const { data } = await apiClient.get<CampaignMyData>(`/campaigns/${campaignId}/me`)
  return data
}

export async function listCampaignInvites(
  campaignId: number,
  params: { page?: number; page_size?: number } = {},
): Promise<PaginatedResponse<CampaignInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<CampaignInviteRecord>>(
    `/campaigns/${campaignId}/invites`,
    { params: { page: params.page ?? 1, page_size: params.page_size ?? 20 } },
  )
  return data
}

export async function getCampaignLeaderboard(
  campaignId: number,
  limit = 50,
): Promise<{ items: CampaignLeaderboardRow[] }> {
  const { data } = await apiClient.get<{ items: CampaignLeaderboardRow[] }>(
    `/campaigns/${campaignId}/leaderboard`,
    { params: { limit } },
  )
  return data
}

export async function getCampaignRules(campaignId: number): Promise<{ rules_text: string }> {
  const { data } = await apiClient.get<{ rules_text: string }>(`/campaigns/${campaignId}/rules`)
  return data
}

export const campaignsAPI = {
  getActiveCampaign,
  getMyCampaignData,
  listCampaignInvites,
  getCampaignLeaderboard,
  getCampaignRules,
}

export default campaignsAPI
