/**
 * User-facing Channel Monitor API endpoints
 * Read-only views for end users to inspect channel availability/status.
 */

import { apiClient } from './client'
import type {
  MonitorQuotaSnapshot,
  Provider,
  MonitorStatus,
  TargetKind,
  MonitorErrorCategory,
} from './admin/channelMonitor'

export type { Provider, MonitorStatus, TargetKind, MonitorErrorCategory } from './admin/channelMonitor'

export interface UserMonitorExtraModel {
  model: string
  status: MonitorStatus
  latency_ms: number | null
  /** 最近一次 error 的归类（rate_or_capacity 等；空 = 未归类） */
  error_category?: MonitorErrorCategory | ''
}

export interface MonitorTimelinePoint {
  status: MonitorStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  checked_at: string
  /** 错误归类（rate_or_capacity 等；空 = 未归类） */
  error_category?: MonitorErrorCategory | ''
}

export interface UserMonitorView {
  id: number
  name: string
  provider: Provider
  group_name: string
  primary_model: string
  primary_status: MonitorStatus
  primary_latency_ms: number | null
  primary_ping_latency_ms: number | null
  availability_7d: number
  extra_models: UserMonitorExtraModel[]
  timeline: MonitorTimelinePoint[]
  /**
   * 探测目标形态。gateway_group = 本站网关入口探测（绑定分组 API key），
   * 卡片语义为"分组级可用性"（任一上游可用即可用），前端展示"分组"徽标。
   */
  target_kind: TargetKind
  /** 主模型最近一次 error 的归类（rate_or_capacity 等；空 = 非 error 或未归类） */
  primary_error_category?: MonitorErrorCategory | ''
  /**
   * 主模型最近配额快照。仅当系统开启 channel_monitor_show_quota 时
   * 服务端才会下发（关闭时服务端已剥离，前端 flag 仅作纵深防御）。
   */
  latest_quota?: MonitorQuotaSnapshot | null
}

export interface UserMonitorListResponse {
  items: UserMonitorView[]
}

export interface UserMonitorModelDetail {
  model: string
  latest_status: MonitorStatus
  latest_latency_ms: number | null
  availability_7d: number
  availability_15d: number
  availability_30d: number
  avg_latency_7d_ms: number | null
  /** 最近一次 error 的归类（rate_or_capacity 等；空 = 未归类） */
  latest_error_category?: MonitorErrorCategory | ''
}

export interface UserMonitorDetail {
  id: number
  name: string
  provider: Provider
  group_name: string
  models: UserMonitorModelDetail[]
}

/**
 * List all monitor views available to the current user.
 */
export async function list(options?: { signal?: AbortSignal }): Promise<UserMonitorListResponse> {
  const { data } = await apiClient.get<UserMonitorListResponse>('/channel-monitors', {
    signal: options?.signal,
  })
  return data
}

/**
 * Get detailed status (multi-window availability + latency) for a single monitor.
 */
export async function status(id: number): Promise<UserMonitorDetail> {
  const { data } = await apiClient.get<UserMonitorDetail>(`/channel-monitors/${id}/status`)
  return data
}

export const channelMonitorUserAPI = {
  list,
  status,
}

export default channelMonitorUserAPI
