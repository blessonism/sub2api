import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { LotteryCampaign, LotteryPrizeTier, LotteryWinner } from '@/api/lotteryCampaigns'

export interface LotteryPrizeTierInput {
  tier_name: string
  winner_count: number
  reward_amount_cents: number
  sort_order: number
}

export interface LotteryCampaignRequest {
  name: string
  description?: string
  rules_text?: string
  participation_mode: 'auto' | 'manual'
  draw_schedule_type: 'single' | 'daily'
  prize_mode: 'single' | 'multi'
  entry_mode: 'daily_once' | 'stepped'
  usage_mode: 'token' | 'usd'
  threshold_tokens: number
  entry_step_tokens: number
  threshold_cost_microusd: number
  entry_step_cost_microusd: number
  max_entries_per_user: number
  start_at: string
  end_at: string
  draw_at?: string | null
  daily_draw_time?: string
  prize_tiers: LotteryPrizeTierInput[]
}

export interface LotteryDrawBatch {
  id: number
  campaign_id: number
  draw_date: string
  scheduled_draw_at: string
  batch_no: string
  status: string
  trigger_type: string
  operator_id?: number | null
  total_entries: number
  total_winners: number
  error_message: string
  created_at: string
  drawn_at?: string | null
  finished_at?: string | null
}

export async function listLotteryCampaigns(params: { page?: number; page_size?: number } = {}): Promise<PaginatedResponse<LotteryCampaign>> {
  const { data } = await apiClient.get<PaginatedResponse<LotteryCampaign>>('/admin/lottery-campaigns', {
    params: { page: params.page ?? 1, page_size: params.page_size ?? 20 },
  })
  return data
}

export async function createLotteryCampaign(payload: LotteryCampaignRequest): Promise<LotteryCampaign> {
  const { data } = await apiClient.post<LotteryCampaign>('/admin/lottery-campaigns', payload)
  return data
}

export async function updateLotteryCampaign(id: number, payload: LotteryCampaignRequest): Promise<LotteryCampaign> {
  const { data } = await apiClient.put<LotteryCampaign>(`/admin/lottery-campaigns/${id}`, payload)
  return data
}

export async function publishLotteryCampaign(id: number): Promise<LotteryCampaign> {
  const { data } = await apiClient.post<LotteryCampaign>(`/admin/lottery-campaigns/${id}/publish`)
  return data
}

export async function cancelLotteryCampaign(id: number): Promise<LotteryCampaign> {
  const { data } = await apiClient.post<LotteryCampaign>(`/admin/lottery-campaigns/${id}/cancel`)
  return data
}

export async function featureLotteryCampaign(id: number): Promise<LotteryCampaign> {
  const { data } = await apiClient.post<LotteryCampaign>(`/admin/lottery-campaigns/${id}/feature`)
  return data
}

export async function deleteLotteryCampaign(id: number): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/admin/lottery-campaigns/${id}`)
  return data
}

export async function syncLotteryEntries(id: number, date: string): Promise<{ synced: number }> {
  const { data } = await apiClient.post<{ synced: number }>(`/admin/lottery-campaigns/${id}/sync-entries`, undefined, { params: { date } })
  return data
}

export async function drawLotteryCampaign(id: number, date: string): Promise<LotteryDrawBatch> {
  const { data } = await apiClient.post<LotteryDrawBatch>(`/admin/lottery-campaigns/${id}/draw`, undefined, { params: { date } })
  return data
}

export async function listLotteryDrawBatches(id: number): Promise<PaginatedResponse<LotteryDrawBatch>> {
  const { data } = await apiClient.get<PaginatedResponse<LotteryDrawBatch>>(`/admin/lottery-campaigns/${id}/draw-batches`)
  return data
}

export async function listLotteryWinners(id: number, batchId?: number): Promise<{ items: LotteryWinner[] }> {
  const path = batchId == null
    ? `/admin/lottery-campaigns/${id}/winners`
    : `/admin/lottery-campaigns/${id}/draw-batches/${batchId}/winners`
  const { data } = await apiClient.get<{ items: LotteryWinner[] }>(path)
  return data
}

export interface LotteryDesignationCandidate {
  user_id: number
  masked_email: string
  entry_count: number
  tokens: number
  cost_microusd: number
  designated_tier_id?: number | null
}

export interface LotteryWinnerDesignation {
  id: number
  campaign_id: number
  draw_date: string
  user_id: number
  prize_tier_id: number
  created_by?: number | null
  created_at: string
}

export interface LotteryDesignationView {
  candidates: LotteryDesignationCandidate[]
  designations: LotteryWinnerDesignation[]
  locked: boolean
}

export interface LotteryDesignationInput {
  user_id: number
  prize_tier_id: number
}

export async function getLotteryDesignations(id: number, date: string): Promise<LotteryDesignationView> {
  const { data } = await apiClient.get<LotteryDesignationView>(`/admin/lottery-campaigns/${id}/designations`, { params: { date } })
  return data
}

export async function replaceLotteryDesignations(id: number, date: string, assignments: LotteryDesignationInput[]): Promise<{ saved: number }> {
  const { data } = await apiClient.put<{ saved: number }>(`/admin/lottery-campaigns/${id}/designations`, { assignments }, { params: { date } })
  return data
}

export type { LotteryCampaign, LotteryPrizeTier, LotteryWinner }

export const lotteryCampaignsAdminAPI = {
  listLotteryCampaigns,
  createLotteryCampaign,
  updateLotteryCampaign,
  publishLotteryCampaign,
  cancelLotteryCampaign,
  featureLotteryCampaign,
  deleteLotteryCampaign,
  syncLotteryEntries,
  drawLotteryCampaign,
  listLotteryDrawBatches,
  listLotteryWinners,
  getLotteryDesignations,
  replaceLotteryDesignations,
}

export default lotteryCampaignsAdminAPI
