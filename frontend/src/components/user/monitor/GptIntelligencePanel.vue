<template>
  <section
    class="mb-5 overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-card dark:border-dark-700/70 dark:bg-dark-800"
  >
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700/70">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold leading-tight text-gray-900 dark:text-gray-100">
              {{ t('channelStatus.modelIq.title') }}
            </h2>
            <span
              class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold uppercase"
              :class="statusClass"
            >
              <span class="mr-1.5 h-1.5 w-1.5 rounded-full" :class="statusDotClass"></span>
              {{ statusLabel }}
            </span>
            <span
              class="inline-flex items-center rounded-full border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-900/70 dark:text-gray-300"
            >
              {{ probeLabel }}
            </span>
          </div>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('channelStatus.modelIq.description') }}
          </p>
        </div>

        <div class="flex flex-col items-start gap-1 text-xs text-gray-500 dark:text-gray-400 lg:items-end lg:text-right">
          <div>{{ updatedLabel }}</div>
          <a
            href="https://codexradar.com/current.json"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1 text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
          >
            {{ t('channelStatus.modelIq.source') }}
            <Icon name="externalLink" size="xs" />
          </a>
        </div>
      </div>
    </div>

    <div v-if="loading && !snapshot" class="p-5">
      <div class="grid gap-4 xl:grid-cols-[minmax(240px,0.85fr)_minmax(0,1.6fr)]">
        <div class="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-900/60"></div>
        <div class="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-900/60"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div
          v-for="i in 4"
          :key="i"
          class="h-20 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-900/60"
        ></div>
      </div>
    </div>

    <div
      v-else-if="error && !snapshot"
      class="m-5 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
    >
      <div class="flex items-start gap-2">
        <Icon name="exclamationTriangle" size="sm" class="mt-0.5 flex-shrink-0" />
        <div>
          <div class="font-semibold">{{ t('channelStatus.modelIq.loadErrorTitle') }}</div>
          <div class="mt-1">{{ t('channelStatus.modelIq.loadErrorDescription') }}</div>
        </div>
      </div>
    </div>

    <div v-else class="p-5">
      <div class="grid gap-4 xl:grid-cols-[minmax(240px,0.85fr)_minmax(0,1.6fr)]">
        <div
          class="rounded-xl border border-emerald-100 bg-emerald-50/70 p-5 dark:border-emerald-500/20 dark:bg-emerald-500/10"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs font-semibold uppercase tracking-wider text-emerald-700/80 dark:text-emerald-300/80">
                {{ t('channelStatus.modelIq.score') }}
              </div>
              <div class="mt-3 font-mono text-5xl font-bold leading-none text-emerald-950 tabular-nums dark:text-emerald-50 sm:text-6xl">
                {{ scoreLabel }}
              </div>
            </div>
            <span
              class="inline-flex flex-shrink-0 items-center rounded-full px-2.5 py-1 text-xs font-semibold uppercase"
              :class="statusClass"
            >
              <span class="mr-1.5 h-1.5 w-1.5 rounded-full" :class="statusDotClass"></span>
              {{ statusLabel }}
            </span>
          </div>

          <div class="mt-6 grid grid-cols-2 gap-3">
            <div class="rounded-lg border border-white/70 bg-white/70 p-3 dark:border-emerald-400/10 dark:bg-dark-900/30">
              <div class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('channelStatus.modelIq.passed') }}
              </div>
              <div class="mt-1 truncate font-mono text-base font-semibold text-gray-900 dark:text-gray-100" :title="passLabel">
                {{ passLabel }}
              </div>
            </div>
            <div class="rounded-lg border border-white/70 bg-white/70 p-3 dark:border-emerald-400/10 dark:bg-dark-900/30">
              <div class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('channelStatus.modelIq.baseline') }}
              </div>
              <div class="mt-1 truncate font-mono text-base font-semibold text-gray-900 dark:text-gray-100" :title="baselineLabel">
                {{ baselineLabel }}
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-lg border border-emerald-100/80 bg-white/60 px-3 py-2 text-xs text-gray-600 dark:border-emerald-400/10 dark:bg-dark-900/30 dark:text-gray-300">
            {{ sampledAtLabel }}
          </div>
        </div>

        <div class="rounded-xl border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700/50 dark:bg-dark-900/40">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                {{ t('channelStatus.modelIq.recentTrend') }}
              </div>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('channelStatus.modelIq.trendSubtitle') }}
              </div>
            </div>
            <div v-if="error" class="text-xs text-amber-600 dark:text-amber-300">
              {{ t('channelStatus.modelIq.staleNotice') }}
            </div>
          </div>

          <div v-if="trendChartData" class="mt-4 h-52 sm:h-56">
            <Line :data="trendChartData" :options="trendChartOptions" />
          </div>

          <div
            v-else
            class="mt-4 flex h-52 items-center justify-center rounded-lg border border-dashed border-gray-200 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400 sm:h-56"
          >
            {{ t('channelStatus.modelIq.emptyTrend') }}
          </div>
        </div>
      </div>

      <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div
          v-for="chip in chipRows"
          :key="chip.key"
          class="flex min-w-0 items-start gap-3 rounded-xl border border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-dark-700/50 dark:bg-dark-900/40"
        >
          <span
            class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-white text-gray-500 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:ring-dark-600"
          >
            <Icon :name="chip.icon" size="sm" />
          </span>
          <span class="min-w-0">
            <span class="block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ chip.label }}
            </span>
            <span class="mt-0.5 block truncate font-mono text-sm font-semibold text-gray-900 dark:text-gray-100" :title="chip.value">
              {{ chip.value }}
            </span>
            <span class="mt-0.5 block truncate text-xs text-gray-400 dark:text-gray-500" :title="chip.subValue">
              {{ chip.subValue }}
            </span>
          </span>
        </div>
      </div>

    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  Filler,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from 'chart.js'
import type { ChartData, ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency, formatNumber } from '@/utils/format'
import type {
  GptIntelligenceRun,
  GptIntelligenceSnapshot,
} from '@/api/gptIntelligence'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

type ChipIcon = 'cpu' | 'clock' | 'database' | 'dollar'

interface InfoChip {
  key: string
  label: string
  value: string
  subValue: string
  icon: ChipIcon
}

const props = defineProps<{
  snapshot: GptIntelligenceSnapshot | null
  loading: boolean
  error: string | null
}>()

const { t } = useI18n()

const latest = computed(() => props.snapshot?.latest ?? null)

const trendRuns = computed<GptIntelligenceRun[]>(() => {
  const byDate = new Map<string, GptIntelligenceRun>()
  for (const run of props.snapshot?.recent_days ?? []) {
    if (run.date) byDate.set(run.date, run)
  }
  const current = latest.value
  if (current?.date) byDate.set(current.date, current)
  return Array.from(byDate.values())
    .filter((run) => run.score !== null)
    .slice(-12)
})

const statusLabel = computed(() => {
  const status = latest.value?.status ?? 'unknown'
  return t(`channelStatus.modelIq.status.${status}`)
})

const statusClass = computed(() => {
  switch (latest.value?.status) {
    case 'green':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'yellow':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    case 'red':
      return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
})

const statusDotClass = computed(() => {
  switch (latest.value?.status) {
    case 'green':
      return 'bg-emerald-500'
    case 'yellow':
      return 'bg-amber-500'
    case 'red':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
})

const updatedLabel = computed(() => {
  const updatedAt = props.snapshot?.monitored_at
  if (!updatedAt) return t('channelStatus.modelIq.updatedUnknown')
  const timezone = props.snapshot?.timezone || 'UTC'
  return t('channelStatus.modelIq.updatedAt', {
    time: new Date(updatedAt).toLocaleString(undefined, { timeZone: timezone }),
    timezone,
  })
})

const scoreLabel = computed(() => formatScore(latest.value?.score ?? null))
const passLabel = computed(() => formatPass(latest.value?.passed ?? null, latest.value?.tasks ?? null))
const modelLabel = computed(() => latest.value?.model || t('monitorCommon.latencyEmpty'))
const reasoningLabel = computed(() => latest.value?.reasoning_effort
  ? t('channelStatus.modelIq.reasoning', { effort: latest.value.reasoning_effort })
  : t('channelStatus.modelIq.reasoningEmpty'))
const durationLabel = computed(() => latest.value?.wall_time_human || formatDuration(latest.value?.wall_seconds ?? null))
const dateLabel = computed(() => latest.value?.date || t('monitorCommon.latencyEmpty'))
const sampledAtLabel = computed(() => t('channelStatus.modelIq.sampledAt', { date: dateLabel.value }))
const costLabel = computed(() => formatCost(latest.value?.cost_usd ?? null))
const tokensLabel = computed(() => formatTokens(latest.value?.total_tokens ?? null))
const outputTokensLabel = computed(() => latest.value?.output_tokens == null
  ? t('channelStatus.modelIq.outputTokensEmpty')
  : t('channelStatus.modelIq.outputTokens', { tokens: formatNumber(latest.value.output_tokens) }))
const probeLabel = computed(() => {
  const tasks = latest.value?.tasks
  if (tasks === null || tasks === undefined) return t('channelStatus.modelIq.testBadgeFallback')
  return t('channelStatus.modelIq.testBadge', { tasks })
})
const baselineLabel = computed(() => {
  const tasks = latest.value?.tasks
  if (tasks === null || tasks === undefined) return t('channelStatus.modelIq.baselineEmpty')
  return t('channelStatus.modelIq.baselineValue', { tasks })
})
const quotaLabel = computed(() => {
  const radar = props.snapshot?.quota_radar
  if (!radar) return t('channelStatus.modelIq.quotaEmpty')
  return t('channelStatus.modelIq.quotaSummary', {
    window: radar.basis_window_label || '-',
    rate: radar.rate == null ? '-' : radar.rate.toFixed(2),
  })
})

const chipRows = computed<InfoChip[]>(() => [
  {
    key: 'model',
    label: t('channelStatus.modelIq.model'),
    value: modelLabel.value,
    subValue: reasoningLabel.value,
    icon: 'cpu',
  },
  {
    key: 'duration',
    label: t('channelStatus.modelIq.duration'),
    value: durationLabel.value,
    subValue: sampledAtLabel.value,
    icon: 'clock',
  },
  {
    key: 'tokens',
    label: t('channelStatus.modelIq.tokens'),
    value: tokensLabel.value,
    subValue: outputTokensLabel.value,
    icon: 'database',
  },
  {
    key: 'cost',
    label: t('channelStatus.modelIq.cost'),
    value: costLabel.value,
    subValue: quotaLabel.value,
    icon: 'dollar',
  },
])

const isDarkMode = computed(() => {
  if (typeof document === 'undefined') return false
  return document.documentElement.classList.contains('dark')
})

const chartColors = computed(() => ({
  line: '#10b981',
  lineFill: '#10b98122',
  point: '#059669',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280',
}))

const trendChartData = computed<ChartData<'line', number[], string> | null>(() => {
  if (!trendRuns.value.length) return null
  const colors = chartColors.value
  return {
    labels: trendRuns.value.map((run) => shortDate(run.date)),
    datasets: [
      {
        label: t('channelStatus.modelIq.score'),
        data: trendRuns.value.map((run) => run.score ?? 0),
        borderColor: colors.line,
        backgroundColor: colors.lineFill,
        pointBackgroundColor: colors.point,
        pointBorderColor: '#ffffff',
        pointBorderWidth: 2,
        pointRadius: 3,
        pointHoverRadius: 5,
        pointHitRadius: 10,
        fill: true,
        tension: 0.35,
      },
    ],
  }
})

const trendChartOptions = computed<ChartOptions<'line'>>(() => {
  const colors = chartColors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      intersect: false,
      mode: 'index',
    },
    plugins: {
      legend: {
        display: false,
      },
      tooltip: {
        callbacks: {
          title: (items) => {
            const index = items[0]?.dataIndex ?? 0
            return trendRuns.value[index]?.date ?? ''
          },
          label: (item) => `${t('channelStatus.modelIq.score')}: ${formatScore(Number(item.raw))}`,
        },
      },
    },
    scales: {
      x: {
        grid: {
          display: false,
        },
        ticks: {
          color: colors.text,
          maxRotation: 0,
          autoSkip: true,
          font: {
            size: 10,
          },
        },
      },
      y: {
        suggestedMin: 0,
        suggestedMax: 150,
        grid: {
          color: colors.grid,
        },
        ticks: {
          color: colors.text,
          font: {
            size: 10,
          },
        },
      },
    },
  }
})

function formatScore(value: number | null): string {
  if (value === null || Number.isNaN(value)) return t('monitorCommon.latencyEmpty')
  return value.toFixed(1)
}

function formatPass(passed: number | null, tasks: number | null): string {
  if (passed === null || tasks === null) return t('channelStatus.modelIq.passEmpty')
  return t('channelStatus.modelIq.passRate', { passed, tasks })
}

function formatCost(value: number | null): string {
  if (value === null) return t('monitorCommon.latencyEmpty')
  return formatCurrency(value)
}

function formatTokens(value: number | null): string {
  if (value === null) return t('monitorCommon.latencyEmpty')
  return formatNumber(value)
}

function formatDuration(value: number | null): string {
  if (value === null) return t('monitorCommon.latencyEmpty')
  return t('channelStatus.modelIq.seconds', { seconds: value })
}

function shortDate(value: string): string {
  const matched = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:-(am|pm))?$/)
  if (!matched) return value
  const [, , month, day, half] = matched
  return `${month}/${day}${half ? half.charAt(0) : ''}`
}
</script>
