import { apiClient } from './client'

// 智商探针（糖果问题）用户端类型与请求封装。
// 状态与后端 service.ModelIQProbe* 常量对齐：pass / wrong_answer / error。
export type ModelIqProbeStatus = 'pass' | 'wrong_answer' | 'error' | 'unknown'

export interface ModelIqProbeStatusPoint {
  status: ModelIqProbeStatus
  checked_at: string
  // 该次探测的回答摘要（error 为错误信息），供时间线悬浮展示。
  answer_excerpt?: string
  marked_for_exclusion?: boolean
}

export interface ModelIqProbeUserLatest {
  status: ModelIqProbeStatus
  answer_excerpt?: string
  // 完整回答原文（后端入库时已截断），供悬浮展示完整回复。
  answer?: string
  checked_at: string
  marked_for_exclusion?: boolean
}

export interface ModelIqProbeSummaryItem {
  model: string
  latest: ModelIqProbeUserLatest | null
  recent: ModelIqProbeStatusPoint[]
}

export interface ModelIqProbeSummary {
  items: ModelIqProbeSummaryItem[]
}

export interface ModelIqProbeHistory {
  items: Array<{ id: number; status: ModelIqProbeStatus; checked_at: string; answer?: string; marked_for_exclusion?: boolean }>
  counts: { pass: number; wrong_answer: number; error: number }
  total: number
}

export async function fetchModelIqProbeHistory(params: { model: string; hours: number; status: string; page: number }, signal?: AbortSignal): Promise<ModelIqProbeHistory> {
  const { data } = await apiClient.get<ModelIqProbeHistory>('/channel-monitors/iq-probe/history', { params, signal })
  return data
}

interface FetchOptions {
  signal?: AbortSignal
  timeoutMs?: number
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function readString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function readStatus(value: unknown): ModelIqProbeStatus {
  if (value === 'pass' || value === 'wrong_answer' || value === 'error') return value
  return 'unknown'
}

function readTime(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function parseLatest(value: unknown): ModelIqProbeUserLatest | null {
  if (!isRecord(value)) return null
  const status = readStatus(value.status)
  const checkedAt = readTime(value.checked_at)
  if (status === 'unknown' && !checkedAt) return null
  return {
    status,
    answer_excerpt: readString(value.answer_excerpt) || undefined,
    answer: readString(value.answer) || undefined,
    checked_at: checkedAt,
    marked_for_exclusion: value.marked_for_exclusion === true,
  }
}

export function parseModelIqProbeSummary(value: unknown): ModelIqProbeSummary {
  if (!isRecord(value) || !Array.isArray(value.items)) {
    throw new Error('invalid model iq probe payload')
  }
  const items = value.items
    .map((raw): ModelIqProbeSummaryItem | null => {
      if (!isRecord(raw)) return null
      const model = readString(raw.model)
      if (!model) return null
      const recent = Array.isArray(raw.recent)
        ? raw.recent
            .map((point): ModelIqProbeStatusPoint | null => {
              if (!isRecord(point)) return null
              return {
                status: readStatus(point.status), checked_at: readTime(point.checked_at),
                answer_excerpt: readString(point.answer_excerpt) || undefined,
                marked_for_exclusion: point.marked_for_exclusion === true,
              }
            })
            .filter((point): point is ModelIqProbeStatusPoint => point !== null)
        : []
      return {
        model,
        latest: parseLatest(raw.latest),
        recent,
      }
    })
    .filter((item): item is ModelIqProbeSummaryItem => item !== null)
  return { items }
}

export async function fetchModelIqProbeSummary(options: FetchOptions = {}): Promise<ModelIqProbeSummary> {
  const { data } = await apiClient.get<ModelIqProbeSummary>('/channel-monitors/iq-probe', {
    signal: options.signal,
    timeout: options.timeoutMs,
  })
  return parseModelIqProbeSummary(data)
}
