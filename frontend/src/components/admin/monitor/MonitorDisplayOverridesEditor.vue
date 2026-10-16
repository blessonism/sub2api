<template>
  <details class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900/40" data-testid="monitor-display-overrides">
    <summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('channelStatus.card.overridesTitle') }}</summary>
    <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t('channelStatus.card.overridesHint') }}</p>
    <div class="mt-4 grid gap-4 sm:grid-cols-2">
      <div v-for="field in fields" :key="field.key">
        <label :for="`monitor-override-${field.key}`" class="input-label">{{ t(`channelStatus.card.${field.label}`) }}</label>
        <input
          :id="`monitor-override-${field.key}`"
          :data-testid="`monitor-override-${field.key}`"
          :value="displayValue(field)"
          type="number"
          min="0"
          :max="field.max"
          :step="field.key === 'primary_latency_ms' ? 1 : 'any'"
          :placeholder="t('channelStatus.card.automaticValue')"
          class="input"
          @input="updateValue(field, $event)"
        />
      </div>
    </div>
    <button type="button" class="btn btn-secondary mt-4" data-testid="monitor-overrides-reset" @click="emit('update:modelValue', {})">
      {{ t('channelStatus.card.restoreAutomatic') }}
    </button>
  </details>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { MonitorDisplayOverrides } from '@/api/admin/channelMonitor'

const props = defineProps<{ modelValue: MonitorDisplayOverrides }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: MonitorDisplayOverrides): void }>()
const { t } = useI18n()
type Field = { key: keyof MonitorDisplayOverrides; label: string; scale?: number; max?: number }
const fields: Field[] = [
  { key: 'visible_rate_multiplier', label: 'overrideRate' },
  { key: 'primary_latency_ms', label: 'overrideLatency' },
  { key: 'cache_rate_24h', label: 'overrideCache', scale: 100, max: 100 },
  { key: 'output_tps_5m', label: 'overrideTPS' },
  { key: 'availability_7d', label: 'overrideAvailability7d', max: 100 },
  { key: 'availability_15d', label: 'overrideAvailability15d', max: 100 },
  { key: 'availability_30d', label: 'overrideAvailability30d', max: 100 },
]
function displayValue(field: Field): number | string {
  const value = props.modelValue[field.key]
  return value == null ? '' : Number((value * (field.scale ?? 1)).toPrecision(12))
}
function updateValue(field: Field, event: Event) {
  const input = event.target as HTMLInputElement
  const next = { ...props.modelValue }
  if (input.value === '') delete next[field.key]
  else if (Number.isFinite(input.valueAsNumber)) next[field.key] = input.valueAsNumber / (field.scale ?? 1)
  emit('update:modelValue', next)
}
</script>
