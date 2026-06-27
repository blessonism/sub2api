<template>
  <div class="flex items-center gap-2">
    <div class="h-1.5 w-20 flex-shrink-0 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
      <div :class="barClass" class="h-full rounded-full transition-all" :style="{ width: `${pct}%` }" />
    </div>
    <span class="tabular-nums text-xs text-gray-700 dark:text-gray-200">{{ label }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ rate?: number | null }>()

const pct = computed(() => (props.rate == null || Number.isNaN(props.rate) ? 0 : Math.round(props.rate * 100)))
const label = computed(() => (props.rate == null ? '-' : `${pct.value}%`))
const barClass = computed(() => {
  if (props.rate == null) return 'bg-gray-300 dark:bg-gray-600'
  if (pct.value >= 90) return 'bg-emerald-500 dark:bg-emerald-400'
  if (pct.value >= 70) return 'bg-amber-500 dark:bg-amber-400'
  return 'bg-red-500 dark:bg-red-400'
})
</script>
