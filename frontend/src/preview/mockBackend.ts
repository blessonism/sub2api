// 临时预览用假后端：axios adapter 的内存实现，仅 preview.html 使用，不进主构建。
// 覆盖智商监控的全部端点，并支持改配置 / 立即探测 / 改判的真实交互效果。
import type { AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { apiClient } from '@/api/client'

interface FakeRow {
  id: number
  account_id: number
  account_name: string
  model: string
  protocol: string
  status: 'pass' | 'wrong_answer' | 'error'
  override_status: string | null
  error_category: string
  error_message: string
  expected_answer: string
  actual_answer: string
  latency_ms: number
  input_tokens: number | null
  output_tokens: number | null
  cache_read_tokens: number | null
  created_at: string
}

const CANDY_QUESTION = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

苹果味 桃子味 西瓜味
圆形 7 9 8
五角星形 7 6 4`

const fakeConfig = {
  enabled: true,
  question: CANDY_QUESTION,
  expected_answer: '21',
  interval_minutes: 360,
  target_mode: 'account_model',
  models: [] as string[],
  account_ids: [12, 17, 21] as number[],
  // 预览展示「保留在名单中、临时停用」的开关效果：17 号账号处于停用态。
  disabled_account_ids: [17] as number[],
  // 与后端展示逻辑对齐：show_exclusion 开启时未通过点位带剔除标记，预览可见效果。
  user_display_mode: 'real' as const,
  show_exclusion: true,
}

// 预览用假账号列表：供账号下拉与「已添加账号开关」渲染。
const fakeAccounts = [
  { id: 12, name: 'OpenAI-Main', platform: 'openai', status: 'active' },
  { id: 17, name: 'Codex-Sparrow', platform: 'openai', status: 'active' },
  { id: 21, name: 'Codex-Backup', platform: 'openai', status: 'active' },
]

let nextId = 1
const minutesAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

function makeRow(p: Partial<FakeRow> & { model: string; created_at: string }): FakeRow {
  return {
    id: nextId++,
    account_id: 12,
    account_name: 'OpenAI-Main',
    protocol: 'responses',
    status: 'pass',
    override_status: null,
    error_category: '',
    error_message: '',
    expected_answer: '21',
    actual_answer: '',
    latency_ms: 3200,
    input_tokens: null,
    output_tokens: null,
    cache_read_tokens: null,
    ...p,
  } as FakeRow
}

const PASS_ANSWERS = [
  '题目要求保证"不同形状的苹果味和桃子味"……所以最少取出 **21 个**糖果：\n- 摸 12 个五角星必有星形苹果和星形桃子\n- 再摸 9 个圆形必有圆苹果或圆桃子与之配对',
  '设摸 r 个圆形、s 个五角星。星形非桃只有 11 个、非苹只有 10 个，故 s=12 时星形苹果、桃子各至少一个；圆形西瓜仅 8 个，r=9 时必有圆苹果或圆桃子。因此最少 21 个。',
  '最少取出 21 个糖果：12 个五角星 + 9 个圆形，即可保证不同形状的苹果味与桃子味配对。',
  '最少 21 个：星形 12 个保证既有星苹果又有星桃子，圆形 9 个保证有圆苹果或圆桃子。',
]
const WRONG_ANSWERS = [
  '答案是 20 个。',
  '考虑最坏情况，需要 23 个糖果才能保证。',
  '最坏情况下，先取完 12 个西瓜味，再取苹果味和桃子味各 7、6 个……所以最少需要取出 26 个糖果。',
]
const ERROR_MESSAGES = [
  'upstream 429: rate limited, retry after 60s',
  'context deadline exceeded',
  'upstream 500: internal server error',
]

// 按模型从新到旧的状态脚本（typical 场景：GPT 系模型通常只有 1~2 个）。
// 每模型 60 条填满条带，状态用确定性伪随机分布，模拟长期探测的密集历史。
const seriesScripts: Array<{
  model: string
  account?: Partial<FakeRow>
  count: number
  stepMinutes: number
  startMinutesAgo: number
  statusAt: (i: number) => 'pass' | 'wrong_answer' | 'error'
  overrideAt?: number
}> = [
  {
    model: 'gpt-5.2',
    count: 60,
    stepMinutes: 210,
    startMinutesAgo: 30,
    statusAt: (i) => (i % 9 === 8 ? 'wrong_answer' : 'pass'),
  },
  {
    model: 'gpt-5.1',
    account: { account_id: 17, account_name: 'Codex-Sparrow' },
    count: 60,
    stepMinutes: 230,
    startMinutesAgo: 55,
    statusAt: (i) => (i % 11 === 9 ? 'error' : i % 7 === 5 ? 'wrong_answer' : 'pass'),
    overrideAt: 52, // 从新到旧第 53 个：wrong_answer 被管理员改判为 pass
  },
  {
    // 展示「未通过 + 剔除标记」的完整效果：最新一轮疑似降智，条带可见多处失败。
    model: 'gpt-5',
    count: 60,
    stepMinutes: 240,
    startMinutesAgo: 45,
    statusAt: (i) => (i === 0 || i % 8 === 7 ? 'wrong_answer' : i % 13 === 11 ? 'error' : 'pass'),
  },
]

const rows: FakeRow[] = []
for (const s of seriesScripts) {
  for (let i = 0; i < s.count; i++) {
    const status = s.statusAt(i)
    const r = makeRow({
      model: s.model,
      status,
      created_at: minutesAgo(s.startMinutesAgo + i * s.stepMinutes),
      account_id: s.account?.account_id ?? 12,
      account_name: s.account?.account_name ?? 'OpenAI-Main',
      protocol: s.account?.protocol ?? 'responses',
      latency_ms: 2800 + ((i * 731) % 2900),
      actual_answer: status === 'pass' ? PASS_ANSWERS[(i + s.model.length) % PASS_ANSWERS.length] : status === 'wrong_answer' ? WRONG_ANSWERS[(i + 1) % WRONG_ANSWERS.length] : '',
      error_message: status === 'error' ? ERROR_MESSAGES[(i + 2) % ERROR_MESSAGES.length] : '',
    })
    if (s.overrideAt === i && status === 'wrong_answer') {
      r.override_status = 'pass'
    }
    rows.push(r)
  }
}

function sortedRows(): FakeRow[] {
  return [...rows].sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
}

function effectiveStatus(r: FakeRow): string {
  return r.override_status ?? r.status
}

function toPayload(r: FakeRow) {
  return { ...r, effective_status: effectiveStatus(r) }
}

function userSummary() {
  const latest = new Map<string, FakeRow>()
  const recent = new Map<string, FakeRow[]>()
  for (const r of sortedRows()) {
    if (!latest.has(r.model)) {
      latest.set(r.model, r)
      recent.set(r.model, [])
    }
    const list = recent.get(r.model)!
    if (list.length < 60) list.push(r)
  }
  // 与后端 UserSummary 同语义：show_exclusion 开启时未通过（wrong_answer）点位带剔除标记。
  const marked = (r: FakeRow) => fakeConfig.show_exclusion && effectiveStatus(r) === 'wrong_answer'
  const excerpt = (r: FakeRow) => (r.actual_answer || r.error_message || '').slice(0, 120)
  const items = [...latest.keys()].sort().map((model) => {
    const l = latest.get(model)!
    const list = (recent.get(model) ?? []).slice().reverse()
    return {
      model,
      latest: {
        status: effectiveStatus(l),
        answer_excerpt: excerpt(l),
        answer: l.actual_answer || l.error_message || '',
        checked_at: l.created_at,
        marked_for_exclusion: marked(l),
      },
      recent: list.map((r) => ({
        status: effectiveStatus(r),
        checked_at: r.created_at,
        answer_excerpt: excerpt(r),
        marked_for_exclusion: marked(r),
      })),
    }
  })
  return { items }
}

function paginate(list: FakeRow[], params: Record<string, unknown>) {
  const page = Math.max(1, Number(params.page ?? 1))
  const pageSize = Math.max(1, Number(params.page_size ?? 20))
  const total = list.length
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const start = (page - 1) * pageSize
  return { items: list.slice(start, start + pageSize).map(toPayload), total, page, page_size: pageSize, pages }
}

function runCycle() {
  const fresh: Array<Partial<FakeRow> & { model: string }> = [
    { model: 'gpt-5.2', latency_ms: 4300, actual_answer: '……所以最少取出 21 个糖果（12 个五角星 + 9 个圆形）。' },
    { model: 'gpt-5.1', account_id: 17, account_name: 'Codex-Sparrow', latency_ms: 5610, actual_answer: '最少取出 21 个糖果。' },
    { model: 'gpt-5', status: 'wrong_answer', latency_ms: 1930, actual_answer: '答案是 20 个。' },
  ]
  const created = fresh.map((p) => makeRow({ ...p, created_at: new Date().toISOString() }))
  rows.unshift(...created)
  return {
    total: created.length,
    pass: created.filter((r) => r.status === 'pass').length,
    wrong_answer: created.filter((r) => r.status === 'wrong_answer').length,
    error: 0,
  }
}

function envelope(data: unknown): AxiosResponse {
  return { status: 200, statusText: 'OK', headers: {}, config: {} as InternalAxiosRequestConfig, data: { code: 0, message: 'success', data } }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

export function installPreviewMockBackend() {
  apiClient.defaults.adapter = async (cfg: AxiosRequestConfig): Promise<AxiosResponse> => {
    await sleep(150)
    const url = (cfg.url || '').split('?')[0]
    const method = (cfg.method || 'get').toLowerCase()
    const params = (cfg.params ?? {}) as Record<string, unknown>
    const body = cfg.data ? (JSON.parse(String(cfg.data)) as Record<string, unknown>) : null

    if (method === 'get' && url === '/admin/model-iq-probe/config') return envelope(fakeConfig)
    if (method === 'put' && url === '/admin/model-iq-probe/config') {
      Object.assign(fakeConfig, body)
      return envelope({ ...fakeConfig })
    }
    if (method === 'get' && url === '/admin/accounts') {
      return envelope({ items: fakeAccounts, total: fakeAccounts.length, page: 1, page_size: 100, pages: 1 })
    }
    if (method === 'get' && url === '/admin/model-iq-probe/results') {
      let list = rows
      if (params.model) list = list.filter((r) => r.model === params.model)
      if (Number(params.account_id) > 0) list = list.filter((r) => r.account_id === Number(params.account_id))
      if (params.status) list = list.filter((r) => effectiveStatus(r) === params.status)
      return envelope(paginate(list, params))
    }
    if (method === 'post' && url === '/admin/model-iq-probe/run') return envelope(runCycle())
    // 模拟可用模型拉取：预览无真实上游目录，返回固定 gpt 系列表。
    if (method === 'get' && url === '/admin/model-iq-probe/models') {
      return envelope({ models: ['gpt-5', 'gpt-5.1', 'gpt-5.2', 'gpt-5.3-codex'] })
    }
    const overrideMatch = url.match(/^\/admin\/model-iq-probe\/results\/(\d+)\/override$/)
    if (method === 'post' && overrideMatch) {
      const row = rows.find((r) => r.id === Number(overrideMatch[1]))
      if (!row) return { ...envelope(null), status: 404, data: { code: 404, message: 'not found' } }
      const status = body && typeof body.status === 'string' ? body.status : null
      row.override_status = status
      if (body && typeof body.actual_answer === 'string' && body.actual_answer.trim()) {
        row.actual_answer = body.actual_answer
      }
      return envelope({ id: row.id })
    }
    const passAnswerMatch = url.match(/^\/admin\/model-iq-probe\/results\/(\d+)\/pass-answer$/)
    if (method === 'get' && passAnswerMatch) {
      const row = rows.find((r) => r.id === Number(passAnswerMatch[1]))
      if (!row) return { ...envelope(null), status: 404, data: { code: 404, message: 'not found' } }
      // 与后端同语义：优先同模型的通过回复，其次任意模型。
      const candidates = rows.filter((r) => r.id !== row.id && effectiveStatus(r) === 'pass' && r.actual_answer)
      const sameModel = candidates.filter((r) => r.model === row.model)
      const pool = sameModel.length > 0 ? sameModel : candidates
      if (pool.length === 0) return { ...envelope(null), status: 404, data: { code: 404, message: 'no pass answer' } }
      const pick = pool[Math.floor(Math.random() * pool.length)]
      return envelope({ answer: pick.actual_answer })
    }
    if (method === 'get' && url === '/channel-monitors/iq-probe') return envelope(userSummary())

    // 其余请求一律返回空成功，避免预览页弹错误 toast。
    return envelope(null)
  }
}
