<template>
  <BaseDialog :show="show" :title="`健康详情 · #${candidate?.account_id} ${candidate?.account_name || ''}`" width="normal" @close="emit('close')">
    <div v-if="candidate" class="space-y-4 text-sm">
      <div v-if="candidate.health" class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span>计算于 {{ formatHealthCalculatedAt(candidate.health.calculated_at) }}</span>
        <span v-if="candidate.health.stale" class="rounded bg-amber-100 px-2 py-0.5 font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-200">健康数据已过期</span>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div v-for="stat in stats" :key="stat.label" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</div>
          <div class="mt-1 text-xl font-semibold tabular-nums" :class="stat.cls">{{ stat.value }}</div>
        </div>
      </div>
      <div v-if="candidate.health?.last_error_class" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-950/30 dark:text-amber-200">
        最近错误：{{ errorClassLabel(candidate.health.last_error_class) }}
      </div>
      <div v-if="candidate.latest_probe?.error_message" class="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-950/30 dark:text-red-300">
        {{ candidate.latest_probe.error_message }}
      </div>
      <template v-if="candidate.latest_usage_delta">
        <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
          <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">Usage Delta</div>
          <div class="space-y-1 text-xs text-gray-600 dark:text-gray-300">
            <div>状态：<span :class="deltaClass">{{ deltaStatusLabel }}</span></div>
            <div v-if="candidate.latest_usage_delta.derived_rate_multiplier != null">推算倍率：{{ formatRate(candidate.latest_usage_delta.derived_rate_multiplier) }}</div>
            <div v-if="candidate.latest_usage_delta.unreliable_reason" class="text-amber-600 dark:text-amber-300">{{ candidate.latest_usage_delta.unreliable_reason }}</div>
          </div>
        </div>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { UpstreamRelayCandidate } from '@/api/admin/upstreamRelayGroupMonitors'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; candidate: UpstreamRelayCandidate | null }>()
const emit = defineEmits<{ close: [] }>()

const stats = computed(() => {
  const h = props.candidate?.health
  return [
    { label: '连成功次数', value: h?.consecutive_successes ?? '-', cls: '' },
    { label: '连失败次数', value: h?.consecutive_failures ?? '-', cls: (h?.consecutive_failures ?? 0) > 0 ? 'text-red-600 dark:text-red-400' : '' },
    { label: 'p95 延迟', value: h?.p95_latency_ms != null ? `${h.p95_latency_ms}ms` : '-', cls: '' },
    { label: '样本数', value: h ? `${h.sample_size} / ${h.window_minutes}min` : '-', cls: '' }
  ]
})

const deltaStatusMap: Record<string, string> = { reliable: '可信', insufficient: '样本不足', unavailable: '不可用' }
const deltaClassMap: Record<string, string> = {
  reliable: 'text-emerald-600 dark:text-emerald-300',
  insufficient: 'text-amber-600 dark:text-amber-300'
}
const deltaStatusLabel = computed(() => {
  const s = props.candidate?.latest_usage_delta?.status ?? ''
  return deltaStatusMap[s] ?? s
})
const deltaClass = computed(() => deltaClassMap[props.candidate?.latest_usage_delta?.status ?? ''] ?? 'text-gray-500 dark:text-gray-400')

function formatRate(v: number) { return Number(v).toFixed(4).replace(/\.?0+$/, '') }

function formatHealthCalculatedAt(value?: string) { return formatDateTime(value) || '-' }

function errorClassLabel(c: string) {
  const map: Record<string, string> = {
    auth_failed: '认证失败', rate_limited: '限流', upstream_5xx: '上游 5xx', timeout: '超时',
    model_unavailable: '模型不可用', insufficient_quota: '余额或额度不足', context_window_exceeded: '上下文超限',
    browser_challenge: '浏览器挑战', network_error: '网络错误', invalid_request: '请求参数错误', request_failed: '请求失败'
  }
  return map[c] ?? c
}
</script>
