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
      <div v-if="comparisonSeries.length" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="series in comparisonSeries"
          :key="series.key"
          class="min-w-0 rounded-xl border p-4"
          :class="series.palette.cardClass"
        >
          <div class="truncate text-sm font-semibold text-gray-600 dark:text-gray-300" :title="series.title">
            {{ series.title }}
          </div>
          <div
            class="mt-3 font-mono text-4xl font-bold leading-none tabular-nums sm:text-5xl"
            :class="series.palette.scoreClass"
          >
            {{ series.scoreLabel }}
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
            <span class="truncate" :title="series.passLabel">{{ series.passLabel }}</span>
            <span class="truncate" :title="series.sampledAtLabel">{{ series.sampledAtLabel }}</span>
          </div>
        </div>
      </div>

      <div class="mt-4 rounded-xl border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ t('channelStatus.modelIq.overviewTrend') }}
            </div>
            <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('channelStatus.modelIq.overviewSubtitle') }}
            </div>
          </div>
          <div v-if="error" class="text-xs text-amber-600 dark:text-amber-300">
            {{ t('channelStatus.modelIq.staleNotice') }}
          </div>
        </div>

        <div v-if="overviewChartData" class="mt-4 h-72 sm:h-80">
          <Line :data="overviewChartData" :options="overviewChartOptions" />
        </div>

        <div
          v-else
          class="mt-4 flex h-72 items-center justify-center rounded-lg border border-dashed border-gray-200 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400 sm:h-80"
        >
          {{ t('channelStatus.modelIq.emptyTrend') }}
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

      <div class="mt-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ t('channelStatus.modelIq.reasoningTrends') }}
            </div>
            <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('channelStatus.modelIq.reasoningTrendsSubtitle') }}
            </div>
          </div>
        </div>

        <div class="mt-3 grid gap-4 xl:grid-cols-2">
          <div
            v-for="series in comparisonSeries"
            :key="`${series.key}-trend`"
            class="rounded-xl border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700/50 dark:bg-dark-900/40"
          >
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="h-2.5 w-2.5 flex-shrink-0 rounded-full" :style="{ backgroundColor: series.palette.line }"></span>
                  <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100" :title="series.title">
                    {{ series.legendLabel }}
                  </div>
                </div>
                <div class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="series.passLabel">
                  {{ series.passLabel }}
                </div>
              </div>
              <div class="font-mono text-lg font-bold leading-none tabular-nums" :class="series.palette.scoreClass">
                {{ series.scoreLabel }}
              </div>
            </div>

            <div v-if="series.chartData" class="mt-4 h-48 sm:h-52">
              <Line :data="series.chartData" :options="seriesChartOptions" />
            </div>

            <div
              v-else
              class="mt-4 flex h-48 items-center justify-center rounded-lg border border-dashed border-gray-200 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400 sm:h-52"
            >
              {{ t('channelStatus.modelIq.emptyTrend') }}
            </div>
          </div>
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

interface SeriesPalette {
  line: string
  fill: string
  point: string
  cardClass: string
  scoreClass: string
}

interface SeriesSource {
  key: string
  title: string
  legendLabel: string
  latest: GptIntelligenceRun | null
  runs: GptIntelligenceRun[]
}

interface PreparedSeries extends SeriesSource {
  scoreLabel: string
  passLabel: string
  sampledAtLabel: string
  palette: SeriesPalette
  chartData: ChartData<'line', (number | null)[], string> | null
}

const SERIES_PALETTES: SeriesPalette[] = [
  {
    line: '#16a34a',
    fill: '#16a34a18',
    point: '#16a34a',
    cardClass: 'border-emerald-200 bg-emerald-50/70 dark:border-emerald-500/25 dark:bg-emerald-500/10',
    scoreClass: 'text-emerald-600 dark:text-emerald-300',
  },
  {
    line: '#2563eb',
    fill: '#2563eb18',
    point: '#2563eb',
    cardClass: 'border-blue-200 bg-blue-50/70 dark:border-blue-500/25 dark:bg-blue-500/10',
    scoreClass: 'text-blue-600 dark:text-blue-300',
  },
  {
    line: '#ea580c',
    fill: '#ea580c18',
    point: '#ea580c',
    cardClass: 'border-orange-200 bg-orange-50/70 dark:border-orange-500/25 dark:bg-orange-500/10',
    scoreClass: 'text-orange-600 dark:text-orange-300',
  },
  {
    line: '#7c3aed',
    fill: '#7c3aed18',
    point: '#7c3aed',
    cardClass: 'border-violet-200 bg-violet-50/70 dark:border-violet-500/25 dark:bg-violet-500/10',
    scoreClass: 'text-violet-600 dark:text-violet-300',
  },
  {
    line: '#0891b2',
    fill: '#0891b218',
    point: '#0891b2',
    cardClass: 'border-cyan-200 bg-cyan-50/70 dark:border-cyan-500/25 dark:bg-cyan-500/10',
    scoreClass: 'text-cyan-600 dark:text-cyan-300',
  },
]

const props = defineProps<{
  snapshot: GptIntelligenceSnapshot | null
  loading: boolean
  error: string | null
}>()

const { t } = useI18n()

const latest = computed(() => props.snapshot?.latest ?? null)

const comparisonSeries = computed<PreparedSeries[]>(() => {
  const sources: SeriesSource[] = []
  const primaryRuns = mergeRuns(props.snapshot?.recent_days ?? [], latest.value)
  const primaryLatest = latest.value ?? findLatestRun(primaryRuns)
  if (primaryLatest || primaryRuns.length) {
    const title = buildSeriesTitle(
      primaryLatest?.model ?? '',
      primaryLatest?.reasoning_effort ?? '',
      t('channelStatus.modelIq.currentSeries'),
    )
    sources.push({
      key: 'current',
      title,
      legendLabel: buildLegendLabel(title),
      latest: primaryLatest,
      runs: primaryRuns,
    })
  }

  for (const comparison of props.snapshot?.comparisons ?? []) {
    const runs = mergeRuns(comparison.recent_days, comparison.latest)
    const comparisonLatest = comparison.latest ?? findLatestRun(runs)
    if (!comparisonLatest && !runs.length) continue

    const title = buildSeriesTitle(
      comparison.model || comparisonLatest?.model || '',
      comparison.reasoning_effort || comparisonLatest?.reasoning_effort || '',
      comparison.label,
    )
    sources.push({
      key: comparison.key,
      title,
      legendLabel: buildLegendLabel(title),
      latest: comparisonLatest,
      runs,
    })
  }

  return sources.map((series, index) => {
    const palette = SERIES_PALETTES[index % SERIES_PALETTES.length]
    const latestRun = series.latest
    const sampledAt = latestRun?.date || findLatestRun(series.runs)?.date || t('monitorCommon.latencyEmpty')
    return {
      ...series,
      scoreLabel: formatScore(latestRun?.score ?? null),
      passLabel: formatPass(latestRun?.passed ?? null, latestRun?.tasks ?? null),
      sampledAtLabel: t('channelStatus.modelIq.sampledAt', { date: sampledAt }),
      palette,
      chartData: buildSingleSeriesChartData(series, palette),
    }
  })
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
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280',
}))

const overviewLabels = computed(() => {
  const dates = new Set<string>()
  for (const series of comparisonSeries.value) {
    for (const run of series.runs) {
      if (run.date && run.score !== null) dates.add(run.date)
    }
  }
  return Array.from(dates).sort(compareSampleDate).slice(-12)
})

const overviewChartData = computed<ChartData<'line', (number | null)[], string> | null>(() => {
  const labels = overviewLabels.value
  if (!labels.length || !comparisonSeries.value.length) return null

  return {
    labels: labels.map(shortDate),
    datasets: comparisonSeries.value.map((series) => {
      const byDate = new Map(series.runs.map((run) => [run.date, run.score]))
      return {
        label: series.legendLabel,
        data: labels.map((date) => byDate.get(date) ?? null),
        borderColor: series.palette.line,
        backgroundColor: series.palette.fill,
        pointBackgroundColor: series.palette.point,
        pointBorderColor: '#ffffff',
        pointBorderWidth: 2,
        pointRadius: 4,
        pointHoverRadius: 6,
        pointHitRadius: 10,
        fill: false,
        tension: 0.35,
        spanGaps: true,
      }
    }),
  }
})

const overviewChartOptions = computed<ChartOptions<'line'>>(() => {
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
        display: true,
        position: 'bottom',
        labels: {
          color: colors.text,
          usePointStyle: true,
          boxWidth: 8,
          boxHeight: 8,
          padding: 18,
        },
      },
      tooltip: {
        callbacks: {
          title: (items) => {
            const index = items[0]?.dataIndex ?? 0
            return overviewLabels.value[index] ?? ''
          },
          label: (item) => `${item.dataset.label}: ${formatScore(readTooltipNumber(item.raw))}`,
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

const seriesChartOptions = computed<ChartOptions<'line'>>(() => {
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
          label: (item) => `${t('channelStatus.modelIq.score')}: ${formatScore(readTooltipNumber(item.raw))}`,
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

function buildSingleSeriesChartData(
  series: SeriesSource,
  palette: SeriesPalette,
): ChartData<'line', (number | null)[], string> | null {
  const runs = series.runs.filter((run) => run.score !== null).slice(-12)
  if (!runs.length) return null
  return {
    labels: runs.map((run) => shortDate(run.date)),
    datasets: [
      {
        label: series.legendLabel,
        data: runs.map((run) => run.score),
        borderColor: palette.line,
        backgroundColor: palette.fill,
        pointBackgroundColor: palette.point,
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
}

function mergeRuns(runs: GptIntelligenceRun[], latestRun: GptIntelligenceRun | null): GptIntelligenceRun[] {
  const byDate = new Map<string, GptIntelligenceRun>()
  for (const run of runs) {
    if (run.date) byDate.set(run.date, run)
  }
  if (latestRun?.date) byDate.set(latestRun.date, latestRun)
  return Array.from(byDate.values())
    .filter((run) => run.score !== null)
    .slice(-12)
}

function findLatestRun(runs: GptIntelligenceRun[]): GptIntelligenceRun | null {
  return runs.length ? runs[runs.length - 1] : null
}

function buildSeriesTitle(model: string, effort: string, fallback: string): string {
  const modelLabel = formatModelName(model)
  if (modelLabel && effort) return `${modelLabel}-${effort}`
  if (modelLabel) return modelLabel
  return fallback
}

function buildLegendLabel(title: string): string {
  return title.replace(/^GPT-/i, '')
}

function formatModelName(model: string): string {
  return model.replace(/^gpt-/i, 'GPT-')
}

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

function readTooltipNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function compareSampleDate(a: string, b: string): number {
  return sampleDateOrder(a) - sampleDateOrder(b)
}

function sampleDateOrder(value: string): number {
  const matched = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:-(am|pm))?$/)
  if (!matched) return Number.MAX_SAFE_INTEGER
  const [, year, month, day, half] = matched
  const halfOrder = half === 'pm' ? 1 : 0
  return Number(year) * 10000 + Number(month) * 100 + Number(day) + halfOrder / 10
}

function shortDate(value: string): string {
  const matched = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:-(am|pm))?$/)
  if (!matched) return value
  const [, , month, day, half] = matched
  return `${Number(month)}.${Number(day)}${half ? `_${half}` : ''}`
}
</script>
