<template>
  <HelpTooltip v-if="showTip && tip" :content="tip">
    <span :class="cls" class="inline-flex cursor-default rounded-md px-2 py-1 text-xs font-medium">{{ label }}</span>
  </HelpTooltip>
  <span v-else :class="cls" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

const props = withDefaults(defineProps<{ source?: string | null; showTip?: boolean }>(), {
  showTip: true
})

const { t } = useI18n()
const tM = (key: string) => t(`admin.upstreamRelayGroupMonitoring.rateSource.${key}`)

const CONFIG: Record<string, { key: string; cls: string }> = {
  login_user_group_rates: {
    key: 'loginUserGroupRates',
    cls: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  },
  login_available_groups: {
    key: 'loginAvailableGroups',
    cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  },
  usage_cost_delta: {
    key: 'usageCostDelta',
    cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  }
}

const entry = computed(() => (props.source ? CONFIG[props.source] : null))
const label = computed(() => entry.value ? tM(`${entry.value.key}.label`) : props.source ?? '-')
const cls = computed(() => entry.value?.cls ?? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300')
const tip = computed(() => entry.value ? tM(`${entry.value.key}.tip`) : '')
const showTip = computed(() => props.showTip)
</script>
