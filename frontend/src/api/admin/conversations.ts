import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export type ConversationParseStatus = 'success' | 'failed' | 'partial'
export type ConversationQualityStatus = 'unchecked' | 'clean' | 'needs_review' | 'rejected'

export interface ConversationQualityError {
  code: string
  message: string
  source?: string
}

export interface ConversationCaptureConfig {
  enabled: boolean
  sample_percent: number
  capture_chat_completions: boolean
  capture_responses: boolean
  raw_archive_enabled: boolean
  max_turn_payload_bytes: number
  payload_preview_chars: number
  session_window_minutes: number
  retention_days: number
  export_enabled: boolean
  excluded_user_ids: number[]
  excluded_api_key_ids: number[]
}

export interface ConversationSession {
  id: number
  session_id: string
  user_id: number
  api_key_id: number
  account_id?: number | null
  provider: string
  model: string
  request_path: string
  status: string
  turn_count: number
  source_request_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  actual_cost: number
  quality_status: ConversationQualityStatus
  quality_errors: ConversationQualityError[]
  exportable: boolean
  duplicate_turn_count: number
  capture_status: string
  session_source: string
  retention_until: string
  started_at: string
  ended_at: string
  created_at: string
  updated_at: string
}

export interface ConversationTurn {
  id: number
  session_id: string
  request_id: string
  upstream_request_id?: string | null
  client_request_id?: string | null
  turn_index: number
  provider: string
  model: string
  request_path: string
  request_messages: Array<Record<string, unknown>>
  response_messages: Array<Record<string, unknown>>
  tools: Array<Record<string, unknown>>
  usage: Record<string, unknown>
  meta: Record<string, unknown>
  input_tokens: number
  output_tokens: number
  total_tokens: number
  actual_cost: number
  stream: boolean
  client_disconnect: boolean
  truncated: boolean
  quality_status: ConversationQualityStatus
  quality_errors: ConversationQualityError[]
  exportable: boolean
  parse_status: ConversationParseStatus
  parse_error?: string | null
  dedupe_hash: string
  raw_archive_key?: string | null
  payload_preview?: string | null
  retention_until: string
  created_at: string
}

export interface ConversationTurnSummary {
  id: number
  session_id: string
  request_id: string
  upstream_request_id?: string | null
  client_request_id?: string | null
  turn_index: number
  provider: string
  model: string
  request_path: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
  actual_cost: number
  stream: boolean
  client_disconnect: boolean
  truncated: boolean
  quality_status: ConversationQualityStatus
  quality_errors: ConversationQualityError[]
  exportable: boolean
  parse_status: ConversationParseStatus
  parse_error?: string | null
  dedupe_hash: string
  duplicate_count: number
  payload_preview?: string | null
  retention_until: string
  created_at: string
}

export interface ConversationSessionFilters {
  page?: number
  page_size?: number
  user_id?: number
  api_key_id?: number
  model?: string
  request_id?: string
  quality_status?: string
  exportable?: boolean | null
  started_at_from?: string
  started_at_to?: string
}

export interface ConversationExportRequest {
  user_id?: number
  api_key_id?: number
  model?: string
  request_id?: string
  quality_status?: string
  started_at_from?: string
  started_at_to?: string
  include_heuristic?: boolean
  include_duplicates?: boolean
  redaction_enabled?: boolean
  dedupe?: boolean | null
  limit?: number
}

export type ConversationExportJobStatus = 'pending' | 'running' | 'completed' | 'failed' | 'expired' | 'deleted'
export type ConversationExportJobFormat = 'messages_jsonl'
export type ConversationExportJobEncoding = 'plain' | 'zstd'

export interface ConversationExportJobFilters {
  user_id?: number
  api_key_id?: number
  model?: string
  request_id?: string
  quality_status?: string
  started_at_from?: string
  started_at_to?: string
  include_heuristic: boolean
  include_duplicates: boolean
  redaction_enabled: boolean
  dedupe: boolean
  limit?: number
}

export interface ConversationQualityUpdateRequest {
  quality_status: ConversationQualityStatus
  quality_errors: ConversationQualityError[]
  exportable?: boolean
}

export interface ConversationBulkQualityUpdateRequest extends ConversationQualityUpdateRequest {
  session_ids?: number[]
  turn_ids?: number[]
}

export interface ConversationExportJob {
  id: number
  status: ConversationExportJobStatus
  filters: ConversationExportJobFilters
  format: ConversationExportJobFormat
  encoding: ConversationExportJobEncoding
  session_count: number
  turn_count: number
  file_size: number
  s3_key?: string | null
  download_url_expires_at?: string | null
  expires_at: string
  error_message?: string | null
  created_by: number
  created_at: string
  updated_at: string
  started_at?: string | null
  completed_at?: string | null
}

export interface ConversationCreateExportJobRequest {
  filters: ConversationExportJobFilters
  format: ConversationExportJobFormat
  encoding: ConversationExportJobEncoding
}

export interface ConversationExportDownloadTicket {
  download_url: string
  expires_at: string
}

export async function getConfig(): Promise<ConversationCaptureConfig> {
  const { data } = await apiClient.get<ConversationCaptureConfig>('/admin/conversations/config')
  return data
}

export async function updateConfig(payload: ConversationCaptureConfig): Promise<ConversationCaptureConfig> {
  const { data } = await apiClient.put<ConversationCaptureConfig>('/admin/conversations/config', payload)
  return data
}

export async function listSessions(
  params: ConversationSessionFilters = {},
): Promise<BasePaginationResponse<ConversationSession>> {
  const { data } = await apiClient.get<BasePaginationResponse<ConversationSession>>('/admin/conversations/sessions', {
    params,
  })
  return data
}

export async function getSession(id: number): Promise<ConversationSession> {
  const { data } = await apiClient.get<ConversationSession>(`/admin/conversations/sessions/${id}`)
  return data
}

export async function listSessionTurns(
  id: number,
  params: { page?: number; page_size?: number } = {},
): Promise<BasePaginationResponse<ConversationTurnSummary>> {
  const { data } = await apiClient.get<BasePaginationResponse<ConversationTurnSummary>>(
    `/admin/conversations/sessions/${id}/turns`,
    { params },
  )
  return data
}

export async function getTurn(id: number): Promise<ConversationTurn> {
  const { data } = await apiClient.get<ConversationTurn>(`/admin/conversations/turns/${id}`)
  return data
}

export async function setSessionExportable(id: number, exportable: boolean): Promise<{ exportable: boolean }> {
  const { data } = await apiClient.put<{ exportable: boolean }>(
    `/admin/conversations/sessions/${id}/exportable`,
    { exportable },
  )
  return data
}

export async function setTurnExportable(id: number, exportable: boolean): Promise<{ exportable: boolean }> {
  const { data } = await apiClient.put<{ exportable: boolean }>(
    `/admin/conversations/turns/${id}/exportable`,
    { exportable },
  )
  return data
}

export async function setSessionQuality(
  id: number,
  payload: ConversationQualityUpdateRequest,
): Promise<{ quality_status: ConversationQualityStatus }> {
  const { data } = await apiClient.put<{ quality_status: ConversationQualityStatus }>(
    `/admin/conversations/sessions/${id}/quality`,
    payload,
  )
  return data
}

export async function setTurnQuality(
  id: number,
  payload: ConversationQualityUpdateRequest,
): Promise<{ quality_status: ConversationQualityStatus }> {
  const { data } = await apiClient.put<{ quality_status: ConversationQualityStatus }>(
    `/admin/conversations/turns/${id}/quality`,
    payload,
  )
  return data
}

export async function bulkSetQuality(payload: ConversationBulkQualityUpdateRequest): Promise<{ updated: boolean }> {
  const { data } = await apiClient.put<{ updated: boolean }>('/admin/conversations/quality/bulk', payload)
  return data
}

export async function mergeSessions(payload: {
  target_session_id: number
  source_session_ids: number[]
}): Promise<{ merged: boolean }> {
  const { data } = await apiClient.post<{ merged: boolean }>('/admin/conversations/sessions/merge', payload)
  return data
}

export async function splitSession(id: number, payload: { turn_id: number }): Promise<ConversationSession> {
  const { data } = await apiClient.post<ConversationSession>(`/admin/conversations/sessions/${id}/split`, payload)
  return data
}

export async function moveTurn(id: number, payload: { target_session_id: number }): Promise<{ moved: boolean }> {
  const { data } = await apiClient.post<{ moved: boolean }>(`/admin/conversations/turns/${id}/move`, payload)
  return data
}

export async function exportMessagesJSONL(payload: ConversationExportRequest): Promise<Blob> {
  const response = await apiClient.post('/admin/conversations/export/messages-jsonl', payload, {
    responseType: 'blob',
  })
  return response.data as Blob
}

export async function createExportJob(payload: ConversationCreateExportJobRequest): Promise<ConversationExportJob> {
  const { data } = await apiClient.post<ConversationExportJob>('/admin/conversations/export-jobs', payload)
  return data
}

export async function listExportJobs(
  params: { page?: number; page_size?: number } = {},
): Promise<BasePaginationResponse<ConversationExportJob>> {
  const { data } = await apiClient.get<BasePaginationResponse<ConversationExportJob>>(
    '/admin/conversations/export-jobs',
    { params },
  )
  return data
}

export async function getExportJob(id: number): Promise<ConversationExportJob> {
  const { data } = await apiClient.get<ConversationExportJob>(`/admin/conversations/export-jobs/${id}`)
  return data
}

export async function createExportDownloadTicket(id: number): Promise<ConversationExportDownloadTicket> {
  const { data } = await apiClient.post<ConversationExportDownloadTicket>(
    `/admin/conversations/export-jobs/${id}/download-ticket`,
  )
  return data
}

export async function deleteExportJob(id: number): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/admin/conversations/export-jobs/${id}`)
  return data
}

const conversationsAPI = {
  getConfig,
  updateConfig,
  listSessions,
  getSession,
  listSessionTurns,
  getTurn,
  setSessionExportable,
  setTurnExportable,
  setSessionQuality,
  setTurnQuality,
  bulkSetQuality,
  mergeSessions,
  splitSession,
  moveTurn,
  exportMessagesJSONL,
  createExportJob,
  listExportJobs,
  getExportJob,
  createExportDownloadTicket,
  deleteExportJob,
}

export default conversationsAPI
