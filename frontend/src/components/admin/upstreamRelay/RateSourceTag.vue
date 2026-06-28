<template>
  <HelpTooltip v-if="showTip && tip" :content="tip">
    <span :class="cls" class="inline-flex cursor-default rounded-md px-2 py-1 text-xs font-medium">{{ label }}</span>
  </HelpTooltip>
  <span v-else :class="cls" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

const props = withDefaults(defineProps<{ source?: string | null; showTip?: boolean }>(), {
  showTip: true
})

const CONFIG: Record<string, { label: string; cls: string; tip: string }> = {
  login_user_group_rates: {
    label: '用户专属',
    cls: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200',
    tip: '从登录账号的用户分组倍率直接读取，置信度最高。'
  },
  login_available_groups: {
    label: '可见分组',
    cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200',
    tip: '从登录账号可见的分组列表推得，置信度较高。'
  },
  usage_cost_delta: {
    label: '/v1/usage 兜底',
    cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200',
    tip: '无法直读倍率时，由探测前后成本差反推，置信度偏低。'
  }
}

const entry = computed(() => (props.source ? CONFIG[props.source] : null))
const label = computed(() => entry.value?.label ?? props.source ?? '-')
const cls = computed(() => entry.value?.cls ?? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300')
const tip = computed(() => entry.value?.tip ?? '')
const showTip = computed(() => props.showTip)
</script>
