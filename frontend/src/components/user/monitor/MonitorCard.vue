<template>
  <button
    type="button"
    class="monitor-card group flex w-full min-w-0 flex-col rounded-2xl p-5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2"
    :aria-label="`${item.name} · ${statusLabel(item.primary_status, item.primary_error_category)}`"
    @click="emit('click')"
  >
    <div class="flex min-w-0 items-start gap-3">
      <span class="grid h-11 w-11 shrink-0 place-items-center rounded-xl" :class="[providerGradient(item.provider), providerTintClass]">
        <ProviderIcon :provider="item.provider" :size="26" />
      </span>
      <div class="min-w-0 flex-1 pt-0.5">
        <div class="truncate text-base font-bold text-gray-900 dark:text-gray-100" :title="[item.name, item.group_name].filter(Boolean).join(' · ')">{{ item.name }}</div>
        <div class="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
          <span class="truncate" :title="formatMonitorModel(item.primary_model)">{{ formatMonitorModel(item.primary_model) }}</span>
          <span v-if="isGatewayGroup" class="shrink-0 rounded bg-primary-50 px-1 py-0.5 text-[10px] text-primary-700 dark:bg-primary-500/15 dark:text-primary-300">
            {{ t('monitorCommon.targetKind.gateway_group') }}
          </span>
        </div>
        <span v-if="item.group_name" class="sr-only">{{ item.group_name }}</span>
      </div>
      <span
        class="inline-flex min-w-0 max-w-[45%] shrink-0 items-center gap-1 rounded-lg border border-emerald-200 bg-emerald-50 px-2 py-1 text-sm font-semibold tracking-wide text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300"
        :title="t('channelStatus.card.rateHint')"
        data-testid="monitor-rate"
      >
        <Icon name="sparkles" size="xs" class="shrink-0" aria-hidden="true" />
        <span class="truncate" :title="rateValue">{{ rateValue }}</span>
      </span>
    </div>
    <div class="mt-4 flex items-center justify-between gap-2 text-sm">
      <span class="inline-flex min-w-0 items-center gap-2 font-semibold" :class="statusColor">
        <Icon :name="statusIcon" size="md" aria-hidden="true" />
        {{ statusLabel(item.primary_status, item.primary_error_category) }}
      </span>
      <span class="inline-flex shrink-0 items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400" :title="item.primary_checked_at || undefined" data-testid="monitor-checked-at">
        <Icon name="clock" size="sm" class="text-blue-500" aria-hidden="true" />
        {{ checkedAtLabel }}
      </span>
    </div>
    <div class="mt-5 grid grid-cols-3 divide-x divide-gray-100 border-y border-gray-100 py-4 dark:divide-dark-700 dark:border-dark-700">
      <div class="metric-column" :title="t('channelStatus.card.pingHint', { value: latencyValue(item.primary_ping_latency_ms) })">
        <div class="metric-value text-gray-900 dark:text-gray-100">
          <Icon name="bolt" size="sm" class="shrink-0 text-amber-500" aria-hidden="true" />
          <span :title="latencyValue(item.primary_latency_ms)" data-testid="monitor-latency">{{ latencyValue(item.primary_latency_ms) }}</span>
        </div>
        <span class="metric-label">{{ t('channelStatus.card.responseTime') }}</span>
      </div>
      <div class="metric-column">
        <div class="metric-value text-emerald-700 dark:text-emerald-400">
          <Icon name="checkCircle" size="sm" class="shrink-0" aria-hidden="true" />
          <span data-testid="monitor-availability">{{ availabilityValue == null ? '—' : `${availabilityValue.toFixed(1)}%` }}</span>
        </div>
        <span class="metric-label">{{ t('channelStatus.card.availability', { days: window.replace('d', '') }) }}</span>
      </div>
      <div class="metric-column" :title="t('channelStatus.card.cacheHint')">
        <div class="metric-value text-gray-900 dark:text-gray-100">
          <Icon name="database" size="sm" class="shrink-0 text-blue-500" aria-hidden="true" />
          <span data-testid="monitor-cache">{{ cacheValue }}</span>
        </div>
        <span class="metric-label">{{ t('channelStatus.card.cache24h') }}</span>
      </div>
    </div>
    <div class="mt-4 flex items-center justify-between gap-2 text-sm" :title="t('channelStatus.card.tpsHint')">
      <span class="inline-flex min-w-0 items-center gap-1.5 text-gray-500 dark:text-gray-400">
        <Icon name="bolt" size="sm" class="shrink-0 text-violet-500" aria-hidden="true" />
        {{ t('channelStatus.card.tps5m') }}
      </span>
      <strong class="shrink-0 tabular-nums text-gray-900 dark:text-gray-100" data-testid="monitor-tps">{{ tpsValue }}</strong>
    </div>
    <MonitorQuotaView v-if="quotaVisible" :snapshot="item.latest_quota" class="mt-3" />
    <MonitorTimeline :buckets="item.timeline" :countdown-seconds="countdownSeconds" compact class="mt-auto" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import { useChannelMonitorFormat, providerGradient } from '@/composables/useChannelMonitorFormat'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import Icon from '@/components/icons/Icon.vue'
import ProviderIcon from './ProviderIcon.vue'
import MonitorTimeline from './MonitorTimeline.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
  nowMs?: number
}>()
const emit = defineEmits<{ (e: 'click'): void }>()
const { t } = useI18n()
const { statusLabel, isRateLimited, formatMonitorModel, formatRelativeTime } = useChannelMonitorFormat()
const isGatewayGroup = computed(() => props.item.target_kind === 'gateway_group')
const quotaVisible = computed(() => isChannelMonitorQuotaVisible() && !!props.item.latest_quota)
const providerTintClass = computed(() => ({
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
  grok: 'text-zinc-700 dark:text-zinc-200',
  antigravity: 'text-purple-600 dark:text-purple-300',
  kimi: 'text-pink-600 dark:text-pink-300',
  zhipu: 'text-indigo-600 dark:text-indigo-300',
  deepseek: 'text-teal-600 dark:text-teal-300',
  minimax: 'text-red-600 dark:text-red-300',
  opencode_go: 'text-amber-700 dark:text-amber-300',
})[props.item.provider] ?? 'text-gray-500 dark:text-gray-300')
const statusColor = computed(() => {
  const item = props.item
  if (isRateLimited(item.primary_status, item.primary_error_category) || item.primary_status === 'degraded') return 'text-amber-700 dark:text-amber-400'
  if (item.primary_status === 'operational') return 'text-emerald-700 dark:text-emerald-400'
  if (item.primary_status === 'failed' || item.primary_status === 'error') return 'text-red-600 dark:text-red-400'
  return 'text-gray-500 dark:text-gray-400'
})
const statusIcon = computed(() => isRateLimited(props.item.primary_status, props.item.primary_error_category) ? 'clock' : props.item.primary_status === 'operational' ? 'checkCircle' :
  props.item.primary_status === 'failed' || props.item.primary_status === 'error' ? 'x' : 'clock')
const checkedAtLabel = computed(() => formatRelativeTime(props.item.primary_checked_at, props.nowMs))
const rateValue = computed(() => {
  const value = props.item.visible_rate_multiplier
  if (value == null || !Number.isFinite(value)) return '—'
  // 非零小倍率保留有效数字，避免显示成免费。
  return `${value !== 0 && Math.abs(value) < 0.0005 ? value.toPrecision(3) : value.toFixed(3)}x`
})
const cacheValue = computed(() => {
  const value = props.item.cache_rate_24h
  return value == null || !Number.isFinite(value) ? '—' : `${(value * 100).toFixed(1)}%`
})
const tpsValue = computed(() => {
  const value = props.item.output_tps_5m
  return value == null || !Number.isFinite(value) ? '—' : `${value.toFixed(1)} TPS`
})
function latencyValue(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`
}
</script>

<style scoped>
.monitor-card {
  --monitor-surface: #fff;
  --monitor-shadow: 0 2px 8px rgb(15 23 42 / 0.04), 0 10px 24px rgb(15 23 42 / 0.06);
  --monitor-shadow-hover: 0 4px 12px rgb(15 23 42 / 0.06), 0 14px 28px rgb(15 23 42 / 0.08);
  container-type: inline-size;
  border: 0;
  background: var(--monitor-surface);
  box-shadow: var(--monitor-shadow);
  transition: box-shadow 180ms ease-out;
}
.monitor-card:hover { box-shadow: var(--monitor-shadow-hover); }
:global(.dark .monitor-card) {
  --monitor-surface: #1f2937;
  --monitor-shadow: 0 2px 8px rgb(0 0 0 / 0.12), 0 10px 24px rgb(0 0 0 / 0.18);
  --monitor-shadow-hover: 0 4px 12px rgb(0 0 0 / 0.16), 0 14px 28px rgb(0 0 0 / 0.22);
}
.metric-column { display: flex; min-width: 0; flex-direction: column; align-items: center; gap: 8px; padding: 0 4px; }
.metric-value { display: flex; width: 100%; min-width: 0; align-items: center; justify-content: center; gap: 5px; font-size: clamp(16px, 5.8cqw, 20px); font-weight: 650; line-height: 1.3; font-variant-numeric: tabular-nums; white-space: nowrap; }
.metric-value > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.metric-label { color: #6b7280; font-size: 12px; line-height: 1.5; text-align: center; }
:global(.dark .monitor-card .metric-label) { color: #9ca3af; }
@container (max-width: 300px) { .metric-value { gap: 3px; } }
@media (prefers-reduced-motion: reduce) { .monitor-card { transition: none; } }
</style>
