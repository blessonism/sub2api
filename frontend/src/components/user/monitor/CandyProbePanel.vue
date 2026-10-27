<template>
  <section
    class="my-5 overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-card dark:border-dark-700/70 dark:bg-dark-800"
  >
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700/70">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold leading-tight text-gray-900 dark:text-gray-100">
              {{ t('channelStatus.candyProbe.title') }}
            </h2>
            <span
              v-if="probeTotal > 0"
              class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
              :class="summaryBadgeClass"
            >
              <span class="mr-1.5 h-1.5 w-1.5 rounded-full" :class="summaryDotClass"></span>
              {{ t('channelStatus.candyProbe.summary', { ok: probeOk, total: probeTotal }) }}
            </span>
          </div>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('channelStatus.candyProbe.description') }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="loading && items.length === 0" class="grid grid-cols-1 gap-4 p-5 md:grid-cols-2 xl:grid-cols-4">
      <div v-for="i in 4" :key="i" class="h-48 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-900/60"></div>
    </div>

    <div v-else-if="error && items.length === 0" class="px-5 py-8 text-center">
      <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelStatus.candyProbe.loadErrorTitle') }}</p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ error }}</p>
    </div>

    <div v-else-if="items.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('channelStatus.candyProbe.noData') }}
    </div>

    <div v-else class="grid grid-cols-1 gap-4 p-5 md:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="item in items"
        :key="item.model"
        class="rounded-2xl border p-4 shadow-sm transition-shadow hover:shadow-md"
        :class="cardTintClass(item)"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="min-w-0 truncate font-mono text-sm font-bold text-gray-900 dark:text-gray-100" :title="item.model">
            {{ item.model }}
          </span>
          <span
            class="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold"
            :class="verdictBadgeClass(item.latest?.status)"
          >
            <span class="h-1.5 w-1.5 rounded-full" :class="item.latest?.status === 'pass' ? 'bg-emerald-500' : item.latest?.status === 'wrong_answer' ? 'bg-red-500' : 'bg-gray-400'"></span>
            {{ verdictLabel(item) }}
          </span>
        </div>

        <p v-if="item.latest?.marked_for_exclusion" class="mt-2 text-xs font-medium text-amber-700 dark:text-amber-400">
          {{ t('channelStatus.candyProbe.markedForExclusion') }}
        </p>

        <div class="mt-3 flex items-stretch gap-3">
          <div
            class="flex min-w-0 flex-1 flex-col justify-center rounded-xl bg-white/80 px-3 py-2.5 ring-1 ring-gray-200/70 dark:bg-dark-800/70 dark:ring-dark-700/60"
          >
            <div class="text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
              {{ t('channelStatus.candyProbe.latestAnswer') }}
            </div>
            <div
              v-if="replyText(item)"
              class="candy-reply mt-1 max-h-10 overflow-hidden text-xs leading-5 text-gray-600 dark:text-gray-300"
              :title="fullReply(item)"
              v-html="renderReply(item)"
            ></div>
            <p v-else class="mt-1 text-xs leading-5 text-gray-400 dark:text-gray-500">
              {{ t('channelStatus.candyProbe.answerFallback') }}
            </p>
          </div>

          <div class="flex shrink-0 flex-col items-center justify-center">
            <div class="relative h-14 w-14">
              <svg viewBox="0 0 48 48" class="h-14 w-14 -rotate-90">
                <circle cx="24" cy="24" r="20" fill="none" stroke-width="5" class="stroke-gray-200/80 dark:stroke-dark-700/70" />
                <circle
                  cx="24"
                  cy="24"
                  r="20"
                  fill="none"
                  stroke-width="5"
                  stroke-linecap="round"
                  :class="ringStrokeClass(item)"
                  :stroke-dasharray="RING_CIRCUMFERENCE"
                  :stroke-dashoffset="ringOffset(item)"
                />
              </svg>
              <span
                class="absolute inset-0 flex items-center justify-center text-xs font-bold tabular-nums"
                :class="rateTextClass(item)"
              >
                {{ rateText(item) }}
              </span>
            </div>
            <div class="mt-0.5 text-[10px] leading-3 text-gray-400 dark:text-gray-500">
              {{ t('channelStatus.candyProbe.rateLabel', { n: validCount(item) }) }}
            </div>
          </div>
        </div>

        <div class="mt-3 mb-1.5 flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
          <div class="flex min-w-0 items-center gap-1.5 text-[10px] text-gray-400 dark:text-gray-500">
            <span class="font-medium uppercase tracking-wider">{{ t('channelStatus.candyProbe.stripLabel', { n: STRIP_SLOTS }) }}</span>
            <span class="text-gray-300 dark:text-gray-600">{{ t('channelStatus.candyProbe.stripOrder') }}</span>
          </div>
          <!-- 常显计数徽标：不依赖 hover 或颜色语义，直接告诉用户红色事件已被管理端处理。
               与底部 streak 徽章同构（琥珀色变体），保持卡片徽章语言统一。 -->
          <div class="flex flex-wrap items-center gap-2">
          <span
            v-if="excludedCount(item) > 0"
            class="inline-flex shrink-0 items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[10px] font-semibold text-amber-600 dark:bg-amber-500/10 dark:text-amber-400"
          >
            <Icon name="check" size="xs" />
            <span class="tabular-nums">{{ t('channelStatus.candyProbe.excludedCountChip', { n: excludedCount(item) }) }}</span>
          </span>
          <button type="button" class="inline-flex items-center gap-1 text-xs font-medium text-primary-600 hover:text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-400" @click="historyModel = item.model">
            {{ t('channelStatus.candyProbe.viewHistory') }} <Icon name="arrowRight" size="xs" />
          </button>
          </div>
        </div>
        <div
          class="grid h-4 w-full min-w-0 items-end gap-[2px]"
          :style="{ gridTemplateColumns: `repeat(${STRIP_SLOTS}, minmax(0, 1fr))` }"
          role="img"
          :aria-label="t('channelStatus.card.probeCount', { n: item.recent.length })"
        >
          <div
            v-for="(bar, idx) in displayBars(item)"
            :key="idx"
            class="candy-bar rounded-sm transition-opacity hover:opacity-75"
            :class="bar.colorClass"
            :style="{ height: bar.heightPct + '%' }"
            :title="bar.title"
          ></div>
        </div>

        <div class="mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-gray-400 dark:text-gray-500">
          <span class="flex items-center gap-1">
            <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>{{ t('channelStatus.candyProbe.verdictPass') }}
          </span>
          <span v-if="hasUnmarkedWrongBars(item)" class="flex items-center gap-1">
            <span class="h-1.5 w-1.5 rounded-full bg-red-500"></span>{{ t('channelStatus.candyProbe.verdictWrong') }}
          </span>
          <span v-if="hasExcludedBars(item)" class="flex items-center gap-1">
            <span class="h-1.5 w-1.5 rounded-full bg-red-300 dark:bg-red-500/40"></span>{{ t('channelStatus.candyProbe.verdictWrong') }} · {{ t('channelStatus.candyProbe.markedForExclusion') }}
          </span>
          <span class="flex items-center gap-1">
            <span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>{{ t('channelStatus.candyProbe.verdictError') }}
          </span>
          <span class="ml-auto flex items-center gap-1.5">
            <span
              v-if="streak(item) > 0"
              class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400"
            >
              <Icon name="fire" size="xs" />
              {{ t('channelStatus.candyProbe.streak', { n: streak(item) }) }}
            </span>
            <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-900/70 dark:text-gray-400">
              <Icon name="clock" size="xs" />
              {{ formatRelativeTime(item.latest?.checked_at, nowMs) }}
            </span>
          </span>
        </div>
      </div>
    </div>
    <CandyProbeHistoryDrawer :model="historyModel" @close="historyModel = null" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import type { ModelIqProbeStatus, ModelIqProbeStatusPoint, ModelIqProbeSummaryItem } from '@/api/modelIqProbe'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import Icon from '@/components/icons/Icon.vue'
import CandyProbeHistoryDrawer from './CandyProbeHistoryDrawer.vue'

marked.setOptions({
  breaks: true,
  gfm: true,
})

const props = defineProps<{
  items: ModelIqProbeSummaryItem[]
  loading?: boolean
  error?: string | null
}>()

const { t } = useI18n()
const historyModel = ref<string | null>(null)
const { formatRelativeTime } = useChannelMonitorFormat()

// 环形仪表：r=20 的圆周长，dashoffset 按正常率偏移。
const RING_CIRCUMFERENCE = (2 * Math.PI * 20).toFixed(2)

// 时间线条带：固定槽位（不足用幽灵条左补齐），与 MonitorTimeline 同规格
// （h-4 矮条 + rounded-sm + 60 槽密度），但失败色独立分级：
// 高绿=正常，短红=疑似降智（未处理），浅红=疑似降智（已剔除），短琥珀=探测失败，矮灰=无数据。
const STRIP_SLOTS = 60

const BAR_HEIGHT: Record<string, number> = {
  pass: 100,
  wrong_answer: 35,
  error: 35,
  unknown: 35,
  empty: 15,
}

const BAR_COLOR: Record<string, string> = {
  pass: 'bg-emerald-500',
  wrong_answer: 'bg-red-500',
  error: 'bg-amber-500',
  unknown: 'bg-gray-200 dark:bg-dark-700',
  empty: 'bg-gray-100 dark:bg-dark-800',
}

// 管理端已确认剔除的降智点：降饱和红色，与未处理降智在条带上即可区分，
// 让用户看得出红色事件已被处理，而不是悬而未决。
const BAR_COLOR_EXCLUDED = 'bg-red-300 dark:bg-red-500/40'

// 相对时间基于 tick 重算，与 MonitorCard 的 nowMs 做法一致。
const nowMs = ref(Date.now())
let nowTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  nowTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 30_000)
})
onBeforeUnmount(() => {
  if (nowTimer) clearInterval(nowTimer)
})

const probeOk = computed(() => props.items.filter((it) => it.latest?.status === 'pass').length)
const probeTotal = computed(() => props.items.filter((it) => it.latest !== null).length)

const summaryBadgeClass = computed(() =>
  probeOk.value === probeTotal.value
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    : 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300',
)
const summaryDotClass = computed(() =>
  probeOk.value === probeTotal.value ? 'bg-emerald-500' : 'bg-red-500',
)

// 环形仪表只统计有效探测（正常/疑似降智）；探测失败多为账号或上游问题，
// 不代表降智，因此不计入分母、不打断连续正常计数，条带用琥珀色单独分级。
function validPoints(item: ModelIqProbeSummaryItem): ModelIqProbeStatusPoint[] {
  return item.recent.filter((p) => p.status === 'pass' || p.status === 'wrong_answer')
}

function validCount(item: ModelIqProbeSummaryItem): number {
  return validPoints(item).length
}

function successRate(item: ModelIqProbeSummaryItem): number | null {
  const valid = validPoints(item)
  if (valid.length === 0) return null
  const pass = valid.filter((p) => p.status === 'pass').length
  return Math.round((pass / valid.length) * 100)
}

function rateText(item: ModelIqProbeSummaryItem): string {
  const rate = successRate(item)
  return rate === null ? '—' : `${rate}%`
}

function rateTextClass(item: ModelIqProbeSummaryItem): string {
  const rate = successRate(item)
  if (rate === null) return 'text-gray-400 dark:text-gray-500'
  if (rate >= 80) return 'text-emerald-600 dark:text-emerald-400'
  if (rate >= 50) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

function ringStrokeClass(item: ModelIqProbeSummaryItem): string {
  const rate = successRate(item)
  if (rate === null) return 'stroke-gray-300 dark:stroke-dark-600'
  if (rate >= 80) return 'stroke-emerald-500'
  if (rate >= 50) return 'stroke-amber-500'
  return 'stroke-red-500'
}

function ringOffset(item: ModelIqProbeSummaryItem): string {
  const rate = successRate(item)
  return (Number(RING_CIRCUMFERENCE) * (1 - (rate ?? 0) / 100)).toFixed(2)
}

// 卡片按最新判定做状态色晕：绿/红/灰三档，一眼区分健康度。
function cardTintClass(item: ModelIqProbeSummaryItem): string {
  switch (item.latest?.status) {
    case 'pass':
      return 'border-emerald-200/80 bg-gradient-to-br from-emerald-50/90 via-white to-white dark:border-emerald-500/20 dark:from-emerald-500/[0.07] dark:via-dark-900/50 dark:to-dark-900/50'
    case 'wrong_answer':
      return 'border-red-200/80 bg-gradient-to-br from-red-50/90 via-white to-white dark:border-red-500/20 dark:from-red-500/[0.07] dark:via-dark-900/50 dark:to-dark-900/50'
    default:
      return 'border-gray-200/90 bg-gradient-to-br from-gray-50/90 via-white to-white dark:border-dark-700/70 dark:from-dark-900/60 dark:via-dark-900/50 dark:to-dark-900/50'
  }
}

// 连续正常次数：从最新一次往前数连续 pass；探测失败视为无判定，跳过。
function streak(item: ModelIqProbeSummaryItem): number {
  let n = 0
  for (let i = item.recent.length - 1; i >= 0; i--) {
    const status = item.recent[i].status
    if (status === 'error' || status === 'unknown') continue
    if (status !== 'pass') break
    n++
  }
  return n
}

function replyText(item: ModelIqProbeSummaryItem): string {
  return item.latest?.answer || item.latest?.answer_excerpt || ''
}

// 探测回复按 markdown 渲染（模型回答常带 markdown 语法），经 DOMPurify 清洗后再注入。
function renderReply(item: ModelIqProbeSummaryItem): string {
  const raw = replyText(item)
  if (!raw) return ''
  const html = marked.parse(raw) as string
  return DOMPurify.sanitize(html)
}

// 悬浮展示完整回复；旧数据只有摘要时退回摘要。
function fullReply(item: ModelIqProbeSummaryItem): string {
  return replyText(item) || t('channelStatus.candyProbe.answerFallback')
}

function verdictBadgeClass(status: ModelIqProbeStatus | undefined): string {
  switch (status) {
    case 'pass':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'wrong_answer':
      return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
    case 'error':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-900/70 dark:text-gray-300'
  }
}

function statusText(status: ModelIqProbeStatus): string {
  switch (status) {
    case 'pass':
      return t('channelStatus.candyProbe.verdictPass')
    case 'wrong_answer':
      return t('channelStatus.candyProbe.verdictWrong')
    case 'error':
      return t('channelStatus.candyProbe.verdictError')
    default:
      return t('channelStatus.candyProbe.verdictUnknown')
  }
}

function verdictLabel(item: ModelIqProbeSummaryItem): string {
  return statusText(item.latest?.status ?? 'unknown')
}

function barClass(status: ModelIqProbeStatus): string {
  return BAR_COLOR[status] ?? BAR_COLOR.unknown
}

function displayBars(item: ModelIqProbeSummaryItem): Array<{ colorClass: string; heightPct: number; title: string }> {
  const bars: Array<{ colorClass: string; heightPct: number; title: string }> = []
  const padCount = Math.max(0, STRIP_SLOTS - item.recent.length)
  for (let i = 0; i < padCount; i++) {
    bars.push({ colorClass: BAR_COLOR.empty, heightPct: BAR_HEIGHT.empty, title: '' })
  }
  for (const point of item.recent) {
    bars.push({
      colorClass: isExcludedWrong(point) ? BAR_COLOR_EXCLUDED : barClass(point.status),
      heightPct: BAR_HEIGHT[point.status] ?? BAR_HEIGHT.unknown,
      title: dotTitle(point),
    })
  }
  return bars
}

// 后端只在真实 wrong_answer 上附加剔除标记（error/改判后的 pass 不会带）。
function isExcludedWrong(point: ModelIqProbeStatusPoint): boolean {
  return point.status === 'wrong_answer' && point.marked_for_exclusion === true
}

function hasExcludedBars(item: ModelIqProbeSummaryItem): boolean {
  return item.recent.some(isExcludedWrong)
}

function excludedCount(item: ModelIqProbeSummaryItem): number {
  return item.recent.filter(isExcludedWrong).length
}

function hasUnmarkedWrongBars(item: ModelIqProbeSummaryItem): boolean {
  return item.recent.some((p) => p.status === 'wrong_answer' && !isExcludedWrong(p))
}

// 时间线点悬浮内容：时间 · 判定，另起一行展示该次回复摘要（error 为错误信息），
// 再带剔除标记。原生 title 按行折行展示。
function dotTitle(point: ModelIqProbeStatusPoint): string {
  const lines = [`${formatRelativeTime(point.checked_at, nowMs.value)} · ${statusText(point.status)}`]
  const excerpt = point.answer_excerpt?.trim()
  if (excerpt) lines.push(excerpt)
  if (point.marked_for_exclusion) lines.push(t('channelStatus.candyProbe.markedForExclusion'))
  return lines.join('\n')
}
</script>

<style scoped>
/* v-html 注入的 markdown 不经过 scoped 编译，用 :deep() 压缩块级元素默认间距，
   并配合 max-h-10 恰好裁到两行。 */
.candy-reply {
  overflow-wrap: anywhere;
}
.candy-reply :deep(p) {
  margin: 0;
}
.candy-reply :deep(ul),
.candy-reply :deep(ol) {
  margin: 0;
  padding-left: 1.1em;
}
.candy-reply :deep(pre) {
  margin: 0;
  white-space: pre-wrap;
}
</style>
