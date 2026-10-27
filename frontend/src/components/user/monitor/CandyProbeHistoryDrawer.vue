<template>
  <BaseDialog :show="model !== null" :title="t('channelStatus.candyProbe.historyTitle', { model })" width="wide" @close="emit('close')">
    <div class="space-y-5">
      <div class="space-y-3">
        <div>
          <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('channelStatus.candyProbe.historyPeriod') }}</p>
          <div class="flex flex-wrap gap-2" role="group" :aria-label="t('channelStatus.candyProbe.historyPeriod')">
            <button v-for="option in periods" :key="option.value" type="button" :aria-pressed="hours === option.value" :class="filterClass(hours === option.value)" @click="hours = option.value">{{ t(option.label) }}</button>
          </div>
        </div>
        <div>
          <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('channelStatus.candyProbe.historyResult') }}</p>
          <div class="flex flex-wrap gap-2" role="group" :aria-label="t('channelStatus.candyProbe.historyResult')">
            <button v-for="option in statuses" :key="option.value" type="button" :aria-pressed="status === option.value" :class="filterClass(status === option.value)" @click="status = option.value">{{ t(option.label) }}</button>
          </div>
        </div>
      </div>

      <div v-if="data" class="border-y border-gray-200 py-4 dark:border-dark-700">
        <div class="grid grid-cols-3 gap-2 text-sm">
          <div><p class="text-gray-500 dark:text-gray-400">{{ t('channelStatus.candyProbe.verdictPass') }}</p><p class="mt-1 font-semibold text-emerald-600">{{ data.counts.pass }}</p></div>
          <div><p class="text-gray-500 dark:text-gray-400">{{ t('channelStatus.candyProbe.verdictWrong') }}</p><p class="mt-1 font-semibold text-red-600">{{ data.counts.wrong_answer }}</p></div>
          <div><p class="text-gray-500 dark:text-gray-400">{{ t('channelStatus.candyProbe.verdictError') }}</p><p class="mt-1 font-semibold text-amber-600">{{ data.counts.error }}</p></div>
        </div>
        <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ rateText }}</p>
      </div>

      <p v-if="loading && !data" class="py-8 text-center text-sm text-gray-500" role="status">{{ t('channelStatus.candyProbe.historyLoading') }}</p>
      <div v-if="error" class="py-4 text-center text-sm text-red-600" role="alert">
        {{ t('channelStatus.candyProbe.historyError') }}
        <button type="button" class="ml-2 underline" @click="load(page === 0 ? 1 : page + 1)">{{ t('channelStatus.candyProbe.historyRetry') }}</button>
      </div>
      <p v-if="data && !loading && !error && data.items.length === 0" class="py-8 text-center text-sm text-gray-500">{{ status ? t('channelStatus.candyProbe.historyNoMatches') : t('channelStatus.candyProbe.historyEmpty') }}</p>
      <div v-if="data && data.items.length" class="divide-y divide-gray-100 dark:divide-dark-700">
        <article v-for="(item, index) in data.items" :key="item.id" class="py-4">
          <p v-if="index === 0 || dayKey(data.items[index - 1].checked_at) !== dayKey(item.checked_at)" class="mb-3 text-xs font-semibold text-gray-500 dark:text-gray-400">{{ dayLabel(item.checked_at) }}</p>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <time class="text-sm font-medium text-gray-800 dark:text-gray-200" :datetime="item.checked_at">{{ formatTime(item.checked_at) }}</time>
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-xs font-semibold" :class="statusColor(item.status)">{{ statusLabel(item.status) }}</span>
              <span v-if="item.marked_for_exclusion" class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-400">{{ t('channelStatus.candyProbe.markedForExclusion') }}</span>
            </div>
          </div>
          <p v-if="item.status === 'error'" class="mt-2 text-sm text-gray-500">{{ t('channelStatus.candyProbe.historyNoResponse') }}</p>
          <template v-else-if="item.answer">
            <p class="mt-2 whitespace-pre-wrap break-words text-sm text-gray-600 dark:text-gray-300" :class="{ 'line-clamp-2': !expanded.has(item.id) }">{{ item.answer }}</p>
            <button type="button" class="mt-2 text-xs font-medium text-primary-600 hover:underline dark:text-primary-400" :aria-expanded="expanded.has(item.id)" @click="toggle(item.id)">{{ t(expanded.has(item.id) ? 'channelStatus.candyProbe.historyCollapse' : 'channelStatus.candyProbe.historyExpand') }}</button>
          </template>
          <p v-else class="mt-2 text-sm text-gray-500">{{ t('channelStatus.candyProbe.answerFallback') }}</p>
        </article>
      </div>
      <button v-if="data && data.items.length < data.total && !error" type="button" class="w-full rounded-md border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:text-gray-200 dark:hover:bg-dark-700" :disabled="loading" @click="load(page + 1)">{{ t(loading ? 'channelStatus.candyProbe.historyLoading' : 'channelStatus.candyProbe.historyMore') }}</button>
      <p class="text-xs text-gray-400">{{ t('channelStatus.candyProbe.historyRetention') }}</p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { fetchModelIqProbeHistory, type ModelIqProbeHistory, type ModelIqProbeStatus } from '@/api/modelIqProbe'

const props = defineProps<{ model: string | null }>()
const emit = defineEmits<{ close: [] }>()
const { t, locale } = useI18n()
const hours = ref(168)
const status = ref('')
const page = ref(0)
const data = ref<ModelIqProbeHistory | null>(null)
const loading = ref(false)
const error = ref(false)
const expanded = ref(new Set<number>())
let controller: AbortController | null = null
const periods = [{ value: 24, label: 'channelStatus.candyProbe.history24h' }, { value: 168, label: 'channelStatus.candyProbe.history7d' }, { value: 720, label: 'channelStatus.candyProbe.history30d' }]
const statuses = [{ value: '', label: 'channelStatus.candyProbe.historyAll' }, { value: 'pass', label: 'channelStatus.candyProbe.verdictPass' }, { value: 'wrong_answer', label: 'channelStatus.candyProbe.verdictWrong' }, { value: 'error', label: 'channelStatus.candyProbe.verdictError' }]
const rateText = computed(() => {
  if (!data.value) return ''
  const valid = data.value.counts.pass + data.value.counts.wrong_answer
  return valid ? t('channelStatus.candyProbe.historyRate', { rate: Math.round(data.value.counts.pass * 100 / valid), n: valid }) : t('channelStatus.candyProbe.historyNoValid')
})
function filterClass(active: boolean) {
  return ['rounded-md border px-3 py-1.5 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500', active ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300' : 'border-gray-200 text-gray-600 dark:border-dark-700 dark:text-gray-300']
}
function statusLabel(value: ModelIqProbeStatus) {
  return t(value === 'pass' ? 'channelStatus.candyProbe.verdictPass' : value === 'wrong_answer' ? 'channelStatus.candyProbe.verdictWrong' : 'channelStatus.candyProbe.verdictError')
}
function statusColor(value: ModelIqProbeStatus) { return value === 'pass' ? 'text-emerald-600' : value === 'wrong_answer' ? 'text-red-600' : 'text-amber-600' }
function formatTime(value: string) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function dayKey(value: string) { return new Date(value).toDateString() }
function dayLabel(value: string) {
  const date = new Date(value)
  const today = new Date()
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1)
  if (date.toDateString() === today.toDateString()) return t('channelStatus.candyProbe.historyToday')
  if (date.toDateString() === yesterday.toDateString()) return t('channelStatus.candyProbe.historyYesterday')
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }).format(date)
}
function toggle(id: number) { const next = new Set(expanded.value); next.has(id) ? next.delete(id) : next.add(id); expanded.value = next }
async function load(nextPage: number) {
  if (!props.model) return
  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  loading.value = true
  error.value = false
  try {
    const result = await fetchModelIqProbeHistory({ model: props.model, hours: hours.value, status: status.value, page: nextPage }, ctrl.signal)
    if (ctrl.signal.aborted) return
    data.value = nextPage === 1 || !data.value ? result : { ...result, items: [...data.value.items, ...result.items] }
    page.value = nextPage
  } catch {
    if (!ctrl.signal.aborted) error.value = true
  } finally {
    if (controller === ctrl) { loading.value = false; controller = null }
  }
}
watch(() => props.model, (model) => {
  if (!model) { hours.value = 168; status.value = '' }
})
watch(() => [props.model, hours.value, status.value], () => {
  controller?.abort()
  data.value = null
  page.value = 0
  expanded.value = new Set()
  if (props.model) void load(1)
}, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>
