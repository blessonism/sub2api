<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-[220px] flex-1">
          <label class="input-label">{{ t('admin.channelMonitor.historyModelFilter') }}</label>
          <Select
            v-model="modelFilter"
            :options="modelOptions"
            clearable
            @change="() => loadHistory()"
          />
        </div>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          @click="() => loadHistory()"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div
        v-if="loading"
        class="rounded-md border border-gray-200 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t('admin.channelMonitor.historyLoading') }}
      </div>

      <div
        v-else-if="items.length === 0"
        class="rounded-md border border-gray-200 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t('admin.channelMonitor.historyEmpty') }}
      </div>

      <div v-else class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-600">
        <div class="max-h-[62vh] overflow-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
            <thead class="sticky top-0 z-10 bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.time') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.model') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.originalStatus') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.effectiveStatus') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.latency') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.message') }}
                </th>
                <th class="px-3 py-2 text-right font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.channelMonitor.historyColumns.actions') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in items" :key="item.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ formatDateTime(item.checked_at) }}
                </td>
                <td class="px-3 py-2 font-medium text-gray-900 dark:text-white">
                  {{ item.model }}
                </td>
                <td class="px-3 py-2">
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
                    :class="statusBadgeClass(item.status, item.error_category)"
                  >
                    {{ statusLabel(item.status, item.error_category) }}
                  </span>
                </td>
                <td class="px-3 py-2">
                  <div class="flex items-center gap-2">
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
                      :class="statusBadgeClass(item.effective_status, item.error_category)"
                    >
                      {{ statusLabel(item.effective_status, item.error_category) }}
                    </span>
                    <span
                      v-if="item.override_status"
                      class="rounded bg-primary-50 px-1.5 py-0.5 text-[11px] font-medium text-primary-700 dark:bg-primary-500/15 dark:text-primary-300"
                    >
                      {{ t('admin.channelMonitor.manualOverride') }}
                    </span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ formatHistoryLatency(item.latency_ms) }}
                </td>
                <td class="max-w-[260px] px-3 py-2 text-gray-600 dark:text-gray-300">
                  <span class="line-clamp-2">{{ item.message || '-' }}</span>
                </td>
                <td class="px-3 py-2">
                  <div class="flex justify-end gap-1">
                    <button
                      v-for="status in editableStatuses"
                      :key="status"
                      type="button"
                      class="rounded-md px-2 py-1 text-xs font-medium transition-colors"
                      :class="statusButtonClass(item, status)"
                      :disabled="updatingId === item.id"
                      @click="setOverride(item, status)"
                    >
                      {{ statusLabel(status) }}
                    </button>
                    <button
                      type="button"
                      class="rounded-md px-2 py-1 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-100"
                      :disabled="!item.override_status || updatingId === item.id"
                      @click="clearOverride(item)"
                    >
                      {{ t('admin.channelMonitor.clearOverride') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ChannelMonitor,
  HistoryItem,
  ManualOverrideStatus,
} from '@/api/admin/channelMonitor'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import {
  STATUS_OPERATIONAL,
  STATUS_DEGRADED,
  STATUS_FAILED,
} from '@/constants/channelMonitor'

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, formatLatency } = useChannelMonitorFormat()

const items = ref<HistoryItem[]>([])
const loading = ref(false)
const updatingId = ref<number | null>(null)
const modelFilter = ref<string | number | boolean | null>(null)
let loadSeq = 0

const editableStatuses: ManualOverrideStatus[] = [
  STATUS_OPERATIONAL,
  STATUS_DEGRADED,
  STATUS_FAILED,
]

const dialogTitle = computed(() => {
  const name = props.monitor?.name || ''
  return name
    ? t('admin.channelMonitor.historyTitleWithName', { name })
    : t('admin.channelMonitor.historyTitle')
})

const modelOptions = computed<SelectOption[]>(() => {
  const monitor = props.monitor
  if (!monitor) return []
  return [monitor.primary_model, ...monitor.extra_models]
    .filter((model): model is string => Boolean(model))
    .map((model) => ({ value: model, label: model }))
})

watch(
  () => props.show,
  (show) => {
    if (show) {
      modelFilter.value = props.monitor?.primary_model || null
      void loadHistory()
    } else {
      loadSeq += 1
      items.value = []
      loading.value = false
      updatingId.value = null
    }
  }
)

async function loadHistory() {
  const monitor = props.monitor
  if (!monitor) return
  const requestSeq = ++loadSeq
  const monitorId = monitor.id
  const selectedModel = typeof modelFilter.value === 'string' ? modelFilter.value : undefined
  loading.value = true
  try {
    const res = await adminAPI.channelMonitor.listHistory(monitorId, {
      model: selectedModel,
      limit: 200,
    })
    if (!isCurrentHistoryRequest(requestSeq, monitorId, selectedModel)) return
    items.value = res.items || []
  } catch (err: unknown) {
    if (!isCurrentHistoryRequest(requestSeq, monitorId, selectedModel)) return
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.historyLoadFailed')))
  } finally {
    if (loadSeq === requestSeq) {
      loading.value = false
    }
  }
}

async function setOverride(item: HistoryItem, status: ManualOverrideStatus) {
  if (!props.monitor || updatingId.value != null) return
  updatingId.value = item.id
  try {
    const updated = await adminAPI.channelMonitor.setHistoryOverride(props.monitor.id, item.id, status)
    replaceItem(updated)
    appStore.showSuccess(t('admin.channelMonitor.overrideSaved'))
    emit('updated')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.overrideSaveFailed')))
  } finally {
    updatingId.value = null
  }
}

async function clearOverride(item: HistoryItem) {
  if (!props.monitor || updatingId.value != null) return
  updatingId.value = item.id
  try {
    const updated = await adminAPI.channelMonitor.clearHistoryOverride(props.monitor.id, item.id)
    replaceItem(updated)
    appStore.showSuccess(t('admin.channelMonitor.overrideCleared'))
    emit('updated')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.overrideClearFailed')))
  } finally {
    updatingId.value = null
  }
}

function replaceItem(updated: HistoryItem) {
  items.value = items.value.map((item) => (item.id === updated.id ? updated : item))
}

function statusButtonClass(item: HistoryItem, status: ManualOverrideStatus): string {
  const active = item.override_status === status
  if (active) return statusBadgeClass(status)
  return 'bg-gray-50 text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
}

function isCurrentHistoryRequest(seq: number, monitorId: number, model?: string): boolean {
  const currentModel = typeof modelFilter.value === 'string' ? modelFilter.value : undefined
  return props.show && props.monitor?.id === monitorId && loadSeq === seq && currentModel === model
}

function formatHistoryLatency(ms: number | null): string {
  if (ms == null) return formatLatency(ms)
  return `${formatLatency(ms)} ms`
}

function formatDateTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}
</script>
