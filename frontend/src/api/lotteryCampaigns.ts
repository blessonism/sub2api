import { apiClient } from './client'

export interface LotteryPrizeTier {
  id: number
  campaign_id: number
  tier_name: string
  winner_count: number
  reward_amount_cents: number
  sort_order: number
  created_at: string
}

export interface LotteryCampaign {
  id: number
  name: string
  description: string
  rules_text: string
  status: string
  participation_mode: 'auto' | 'manual'
  draw_schedule_type: 'single' | 'daily'
  prize_mode: 'single' | 'multi'
  entry_mode: 'daily_once' | 'stepped'
  threshold_tokens: number
  entry_step_tokens: number
  max_entries_per_user: number
  start_at: string
  end_at: string
  draw_at?: string | null
  daily_draw_time: string
  created_by?: number | null
  updated_by?: number | null
  created_at: string
  updated_at: string
  is_featured: boolean
  prize_tiers?: LotteryPrizeTier[]
}

export interface LotteryWinner {
  id: number
  batch_id: number
  campaign_id: number
  user_id: number
  prize_tier_id: number
  prize_name?: string
  entry_date: string
  reward_amount_cents: number
  status: string
  balance_before_snapshot?: number | null
  balance_after_snapshot?: number | null
  idempotency_key: string
  error_message: string
  created_at: string
  processed_at?: string | null
}

export interface LotteryMyData {
  campaign: LotteryCampaign
  today_tokens: number
  threshold_tokens: number
  entry_count: number
  entry_status: string
  next_draw_at?: string | null
  participant_count: number
  winners: LotteryWinner[]
}

export interface LotteryPublicWinner {
  masked_email: string
  prize_name: string
  reward_amount_cents: number
  created_at: string
}

export interface LotteryParticipant {
  masked_email: string
}

export async function getActiveLotteryCampaign(): Promise<{ campaign: LotteryCampaign | null }> {
  const { data } = await apiClient.get<{ campaign: LotteryCampaign | null }>('/lottery-campaigns/active')
  return data
}

export async function getMyLotteryCampaignData(campaignId: number): Promise<LotteryMyData> {
  const { data } = await apiClient.get<LotteryMyData>(`/lottery-campaigns/${campaignId}/me`)
  return data
}

export async function enrollLotteryCampaign(campaignId: number) {
  const { data } = await apiClient.post(`/lottery-campaigns/${campaignId}/enroll`)
  return data
}

export async function getRecentLotteryWinners(campaignId: number, limit?: number): Promise<{ items: LotteryPublicWinner[] }> {
  const { data } = await apiClient.get<{ items: LotteryPublicWinner[] }>(`/lottery-campaigns/${campaignId}/winners`, {
    params: limit ? { limit } : undefined,
  })
  return data
}

export async function getLotteryParticipants(campaignId: number): Promise<{ items: LotteryParticipant[] }> {
  const { data } = await apiClient.get<{ items: LotteryParticipant[] }>(`/lottery-campaigns/${campaignId}/participants`)
  return data
}

export const lotteryCampaignsAPI = {
  getActiveLotteryCampaign,
  getMyLotteryCampaignData,
  enrollLotteryCampaign,
  getRecentLotteryWinners,
  getLotteryParticipants,
}

export default lotteryCampaignsAPI
