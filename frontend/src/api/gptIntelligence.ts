const CODEX_RADAR_CURRENT_URL = 'https://codexradar.com/current.json'
const DEFAULT_FETCH_TIMEOUT_MS = 8000
const DEFAULT_TIMEZONE = 'UTC'

export type GptIntelligenceStatus = 'green' | 'yellow' | 'red' | 'unknown'

export interface GptIntelligenceRun {
  date: string
  score: number | null
  status: GptIntelligenceStatus
  passed: number | null
  tasks: number | null
  invalid: number | null
  total_tokens: number | null
  output_tokens: number | null
  wall_seconds: number | null
  wall_time_human: string
  model: string
  reasoning_effort: string
  cost_usd: number | null
}

export interface GptIntelligenceComparison {
  key: string
  label: string
  model: string
  reasoning_effort: string
  latest: GptIntelligenceRun | null
  recent_days: GptIntelligenceRun[]
}

export interface GptIntelligenceQuotaRadar {
  basis_window_label: string
  cost_usd: number | null
  rate: number | null
  adjusted_delta: number | null
  updated_at: string
}

export interface GptIntelligenceSnapshot {
  monitored_at: string
  timezone: string
  latest: GptIntelligenceRun | null
  recent_days: GptIntelligenceRun[]
  comparisons: GptIntelligenceComparison[]
  quota_radar: GptIntelligenceQuotaRadar | null
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

function readNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function readStatus(value: unknown): GptIntelligenceStatus {
  if (value === 'green' || value === 'yellow' || value === 'red') return value
  return 'unknown'
}

function readTimezone(value: unknown): string {
  const timezone = readString(value) || DEFAULT_TIMEZONE
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: timezone })
    return timezone
  } catch {
    return DEFAULT_TIMEZONE
  }
}

function createFetchSignal(options: FetchOptions): { signal: AbortSignal; cleanup: () => void } {
  const ctrl = new AbortController()
  const abort = () => ctrl.abort()
  const timeoutMs = options.timeoutMs && options.timeoutMs > 0
    ? options.timeoutMs
    : DEFAULT_FETCH_TIMEOUT_MS
  const timeoutId = globalThis.setTimeout(abort, timeoutMs)

  if (options.signal?.aborted) {
    abort()
  } else {
    options.signal?.addEventListener('abort', abort, { once: true })
  }

  return {
    signal: ctrl.signal,
    cleanup: () => {
      globalThis.clearTimeout(timeoutId)
      options.signal?.removeEventListener('abort', abort)
    },
  }
}

function parseRun(value: unknown): GptIntelligenceRun | null {
  if (!isRecord(value)) return null
  return {
    date: readString(value.date),
    score: readNumber(value.score),
    status: readStatus(value.status),
    passed: readNumber(value.passed),
    tasks: readNumber(value.tasks),
    invalid: readNumber(value.invalid),
    total_tokens: readNumber(value.total_tokens),
    output_tokens: readNumber(value.output_tokens),
    wall_seconds: readNumber(value.wall_seconds),
    wall_time_human: readString(value.wall_time_human),
    model: readString(value.model),
    reasoning_effort: readString(value.reasoning_effort),
    cost_usd: readNumber(value.cost_usd),
  }
}

function parseComparisons(value: unknown): GptIntelligenceComparison[] {
  if (!isRecord(value)) return []
  return Object.entries(value)
    .map(([key, raw]) => {
      if (!isRecord(raw)) return null
      return {
        key,
        label: readString(raw.label) || key,
        model: readString(raw.model),
        reasoning_effort: readString(raw.reasoning_effort),
        latest: parseRun(raw.latest),
        recent_days: Array.isArray(raw.recent_days)
          ? raw.recent_days.map(parseRun).filter((item): item is GptIntelligenceRun => item !== null)
          : [],
      }
    })
    .filter((item): item is GptIntelligenceComparison => item !== null)
}

function parseQuotaRadar(value: unknown): GptIntelligenceQuotaRadar | null {
  if (!isRecord(value)) return null
  return {
    basis_window_label: readString(value.basis_window_label),
    cost_usd: readNumber(value.cost_usd),
    rate: readNumber(value.rate),
    adjusted_delta: readNumber(value.adjusted_delta),
    updated_at: readString(value.updated_at),
  }
}

export function parseGptIntelligenceSnapshot(value: unknown): GptIntelligenceSnapshot {
  if (!isRecord(value)) {
    throw new Error('invalid codex radar payload')
  }
  const modelIq = value.model_iq
  if (!isRecord(modelIq)) {
    throw new Error('missing model_iq payload')
  }

  const recentDays = Array.isArray(modelIq.recent_days)
    ? modelIq.recent_days.map(parseRun).filter((item): item is GptIntelligenceRun => item !== null)
    : []

  return {
    monitored_at: readString(value.monitored_at),
    timezone: readTimezone(value.timezone),
    latest: parseRun(modelIq.latest),
    recent_days: recentDays,
    comparisons: parseComparisons(modelIq.comparisons),
    quota_radar: parseQuotaRadar(modelIq.quota_radar),
  }
}

export async function fetchGptIntelligenceSnapshot(
  options: FetchOptions = {},
): Promise<GptIntelligenceSnapshot> {
  const { signal, cleanup } = createFetchSignal(options)

  try {
    const res = await fetch(CODEX_RADAR_CURRENT_URL, {
      headers: {
        Accept: 'application/json',
      },
      signal,
    })

    if (!res.ok) {
      throw new Error(`codex radar request failed: ${res.status}`)
    }

    return parseGptIntelligenceSnapshot(await res.json())
  } finally {
    cleanup()
  }
}
