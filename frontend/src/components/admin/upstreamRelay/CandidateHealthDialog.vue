<template>
  <BaseDialog :show="show" :title="dialogTitle" width="normal" @close="emit('close')">
    <div v-if="candidate" class="space-y-4 text-sm">
      <div v-if="candidate.health" class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ tM('calculatedAt', { time: formatHealthCalculatedAt(candidate.health.calculated_at) }) }}</span>
        <span v-if="candidate.health.stale" class="rounded bg-amber-100 px-2 py-0.5 font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-200">{{ tM('stale') }}</span>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div v-for="stat in stats" :key="stat.label" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</div>
          <div class="mt-1 text-xl font-semibold tabular-nums" :class="stat.cls">{{ stat.value }}</div>
        </div>
      </div>
      <div v-if="candidate.health?.last_error_class" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-950/30 dark:text-amber-200">
        {{ tM('recentError', { error: errorClassLabel(candidate.health.last_error_class) }) }}
      </div>
      <div v-if="probeError" class="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/30 dark:text-red-200">
        <div class="font-semibold">{{ probeError.reason }}</div>
        <div class="mt-1">{{ probeError.advice }}</div>
        <details v-if="probeError.raw" class="mt-2 text-red-600/80 dark:text-red-300/80">
          <summary class="cursor-pointer select-none">{{ tM('technicalDetails') }}</summary>
          <div class="mt-1 break-words font-mono">{{ probeError.raw }}</div>
        </details>
      </div>
      <template v-if="candidate.latest_usage_delta">
        <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
          <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageDeltaTitle') }}</div>
          <div class="space-y-1 text-xs text-gray-600 dark:text-gray-300">
            <div>{{ tM('status') }}：<span :class="deltaClass">{{ deltaStatusLabel }}</span></div>
            <div v-if="candidate.latest_usage_delta.derived_rate_multiplier != null">{{ tM('derivedRate') }}：{{ formatRate(candidate.latest_usage_delta.derived_rate_multiplier) }}</div>
            <div v-if="usageDeltaIssue" class="rounded-md bg-amber-50 px-3 py-2 text-amber-700 dark:bg-amber-950/30 dark:text-amber-200">
              <div class="font-semibold">{{ usageDeltaIssue.reason }}</div>
              <div class="mt-1">{{ usageDeltaIssue.advice }}</div>
              <details v-if="usageDeltaIssue.raw" class="mt-2 text-amber-600/80 dark:text-amber-300/80">
                <summary class="cursor-pointer select-none">{{ tM('technicalDetails') }}</summary>
                <div class="mt-1 break-words font-mono">{{ usageDeltaIssue.raw }}</div>
              </details>
            </div>
          </div>
        </div>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { UpstreamRelayCandidate } from '@/api/admin/upstreamRelayGroupMonitors'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; candidate: UpstreamRelayCandidate | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const tM = (key: string, params?: Record<string, unknown>) => t(`admin.upstreamRelayGroupMonitoring.healthDialog.${key}`, params ?? {})

const dialogTitle = computed(() => tM('title', {
  account: props.candidate?.account_id ?? '-',
  name: props.candidate?.account_name || '-'
}))
const stats = computed(() => {
  const health = props.candidate?.health
  return [
    { label: tM('stats.consecutiveSuccesses'), value: health?.consecutive_successes ?? '-', cls: '' },
    { label: tM('stats.consecutiveFailures'), value: health?.consecutive_failures ?? '-', cls: (health?.consecutive_failures ?? 0) > 0 ? 'text-red-600 dark:text-red-400' : '' },
    { label: tM('stats.p95Latency'), value: health?.p95_latency_ms != null ? tM('stats.latencyValue', { value: health.p95_latency_ms }) : '-', cls: '' },
    { label: tM('stats.sampleSize'), value: health ? tM('stats.sampleValue', { samples: health.sample_size, minutes: health.window_minutes }) : '-', cls: '' }
  ]
})

const errorClasses = new Set([
  'auth_failed', 'rate_limited', 'upstream_5xx', 'timeout', 'model_unavailable', 'insufficient_quota',
  'context_window_exceeded', 'browser_challenge', 'network_error', 'invalid_request', 'request_failed'
])
const probeError = computed(() => {
  const raw = props.candidate?.latest_probe?.error_message?.trim() || ''
  if (!raw) return null
  const errorClass = props.candidate?.latest_probe?.error_class || props.candidate?.health?.last_error_class || 'request_failed'
  const key = errorClasses.has(errorClass) ? errorClass : 'request_failed'
  return { reason: tM(`errors.${key}.reason`), advice: tM(`errors.${key}.advice`), raw }
})

const deltaStatusLabel = computed(() => {
  const status = props.candidate?.latest_usage_delta?.status || 'unavailable'
  return tM(`deltaStatus.${['reliable', 'insufficient', 'unavailable'].includes(status) ? status : 'unavailable'}`)
})
const deltaClass = computed(() => {
  const status = props.candidate?.latest_usage_delta?.status
  if (status === 'reliable') return 'text-emerald-600 dark:text-emerald-300'
  if (status === 'insufficient') return 'text-amber-600 dark:text-amber-300'
  return 'text-gray-500 dark:text-gray-400'
})
const usageDeltaIssue = computed(() => {
  const raw = props.candidate?.latest_usage_delta?.unreliable_reason?.trim() || ''
  if (!raw) return null
  const lower = raw.toLowerCase()
  const key = lower.includes('missing before or after')
    ? 'missingSnapshot'
    : lower.includes('probe did not succeed')
      ? 'probeFailed'
      : lower.includes('not positive')
        ? 'nonPositiveDelta'
        : 'requestFailed'
  return { reason: tM(`deltaIssues.${key}.reason`), advice: tM(`deltaIssues.${key}.advice`), raw }
})

function formatRate(value: number) { return Number(value).toFixed(4).replace(/\.?0+$/, '') }
function formatHealthCalculatedAt(value?: string) { return formatDateTime(value) || '-' }
function errorClassLabel(errorClass: string) {
  const key = errorClasses.has(errorClass) ? errorClass : 'request_failed'
  return tM(`errorClasses.${key}`)
}
</script>
