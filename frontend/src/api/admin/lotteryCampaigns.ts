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
  threshold_tokens: number
  entry_step_tokens: number
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

export async function listLotteryWinners(id: number, batchId: number): Promise<{ items: LotteryWinner[] }> {
  const { data } = await apiClient.get<{ items: LotteryWinner[] }>(`/admin/lottery-campaigns/${id}/draw-batches/${batchId}/winners`)
  return data
}

export type { LotteryCampaign, LotteryPrizeTier, LotteryWinner }

export const lotteryCampaignsAdminAPI = {
  listLotteryCampaigns,
  createLotteryCampaign,
  updateLotteryCampaign,
  publishLotteryCampaign,
  cancelLotteryCampaign,
  syncLotteryEntries,
  drawLotteryCampaign,
  listLotteryDrawBatches,
  listLotteryWinners,
}

export default lotteryCampaignsAdminAPI
