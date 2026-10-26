import { apiClient } from './client'

// 智商探针（糖果问题）用户端类型与请求封装。
// 状态与后端 service.ModelIQProbe* 常量对齐：pass / wrong_answer / error。
export type ModelIqProbeStatus = 'pass' | 'wrong_answer' | 'error' | 'unknown'

export interface ModelIqProbeStatusPoint {
  status: ModelIqProbeStatus
  checked_at: string
}

export interface ModelIqProbeUserLatest {
  status: ModelIqProbeStatus
  answer_excerpt?: string
  checked_at: string
}

export interface ModelIqProbeSummaryItem {
  model: string
  latest: ModelIqProbeUserLatest | null
  recent: ModelIqProbeStatusPoint[]
}

export interface ModelIqProbeSummary {
  items: ModelIqProbeSummaryItem[]
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
    checked_at: checkedAt,
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
              return { status: readStatus(point.status), checked_at: readTime(point.checked_at) }
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
