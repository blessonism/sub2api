<template>
  <div class="rounded-lg border px-4 py-3 text-sm" :class="panelClass" data-testid="operation-result-panel">
    <div class="flex items-start gap-3">
      <Icon :name="statusIcon" size="sm" class="mt-0.5 shrink-0" :class="iconClass" />
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-semibold">{{ title }}</span>
          <span class="rounded-md bg-white/60 px-2 py-0.5 text-xs font-medium dark:bg-black/20">{{ source }}</span>
        </div>
        <dl class="mt-3 grid gap-2 text-xs leading-5 sm:grid-cols-[6rem_minmax(0,1fr)]">
          <dt class="font-medium opacity-70">{{ tM('action') }}</dt>
          <dd class="break-words">{{ action }}</dd>
          <dt class="font-medium opacity-70">{{ tM('result') }}</dt>
          <dd class="break-words">{{ result }}</dd>
          <dt class="font-medium opacity-70">{{ tM('impact') }}</dt>
          <dd class="break-words">{{ impact }}</dd>
          <dt class="font-medium opacity-70">{{ tM('nextStep') }}</dt>
          <dd class="break-words">{{ nextStep }}</dd>
        </dl>
        <details v-if="technicalDetails.length > 0" class="mt-3 text-xs opacity-80">
          <summary class="cursor-pointer select-none font-medium">{{ tM('technicalDetails') }}</summary>
          <ul class="mt-2 space-y-1 rounded-md bg-white/60 px-3 py-2 font-mono dark:bg-black/20">
            <li v-for="(detail, index) in technicalDetails" :key="`${index}-${detail}`" class="break-words">{{ detail }}</li>
          </ul>
        </details>
        <button v-if="actionLabel" class="btn btn-secondary mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs" type="button" :disabled="actionDisabled" @click="emit('action')">
          <Icon name="arrowRight" size="xs" />
          {{ actionLabel }}
        </button>
      </div>
      <button v-if="dismissible" type="button" class="shrink-0 rounded p-1 opacity-70 transition hover:bg-white/60 hover:opacity-100 dark:hover:bg-black/20" :aria-label="tM('dismiss')" @click="emit('dismiss')">
        <Icon name="x" size="sm" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

type OperationResultStatus = 'running' | 'success' | 'partial' | 'failed'

const props = withDefaults(defineProps<{
  status: OperationResultStatus
  source: string
  title: string
  action: string
  result: string
  impact: string
  nextStep: string
  technicalDetails?: string[]
  actionLabel?: string
  actionDisabled?: boolean
  dismissible?: boolean
}>(), {
  technicalDetails: () => [],
  actionLabel: '',
  actionDisabled: false,
  dismissible: true
})

const emit = defineEmits<{ action: []; dismiss: [] }>()
const { t } = useI18n()
const tM = (key: string) => t(`admin.upstreamRelayGroupMonitoring.operationResult.${key}`)

const panelClass = computed(() => ({
  running: 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-100',
  success: 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-100',
  partial: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100',
  failed: 'border-red-200 bg-red-50 text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-100'
}[props.status]))

const statusIcon = computed(() => ({
  running: 'refresh',
  success: 'checkCircle',
  partial: 'exclamationTriangle',
  failed: 'xCircle'
}[props.status] as 'refresh' | 'checkCircle' | 'exclamationTriangle' | 'xCircle'))
const iconClass = computed(() => props.status === 'running' ? 'animate-spin' : '')
</script>
