<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.timeRate.title')"
    width="wide"
    @close="handleClose"
  >
    <div v-if="group" class="space-y-4">
      <div class="flex flex-wrap items-center gap-3 bg-gray-50 px-4 py-3 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5 text-gray-700 dark:text-gray-300">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t(`admin.groups.platforms.${group.platform}`) }}
        </span>
        <span class="text-gray-300 dark:text-dark-500">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-300 dark:text-dark-500">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.timeRate.baseActual') }}: {{ group.rate_multiplier }}x
        </span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.timeRate.baseVisible') }}:
          {{ group.visible_rate_multiplier ?? group.rate_multiplier }}x
        </span>
      </div>

      <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 dark:border-dark-600 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <label class="input-label">{{ t('admin.groups.timeRate.priority') }}</label>
          <div class="inline-flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <button
              v-for="option in priorityOptions"
              :key="option.value"
              type="button"
              :class="[
                'px-3 py-2 text-sm font-medium transition-colors',
                priority === option.value
                  ? 'bg-primary-600 text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-50 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
              ]"
              @click="priority = option.value"
            >
              {{ option.label }}
            </button>
          </div>
        </div>
        <button type="button" class="btn btn-secondary shrink-0" @click="addPeriod">
          <Icon name="plus" size="sm" class="mr-1.5" />
          {{ t('admin.groups.timeRate.addPeriod') }}
        </button>
      </div>

      <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
        <table class="w-full min-w-[720px] text-sm">
          <thead class="bg-gray-50 dark:bg-dark-700">
            <tr class="border-b border-gray-200 dark:border-dark-600">
              <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.timeRate.start') }}</th>
              <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.timeRate.end') }}</th>
              <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.timeRate.actual') }}</th>
              <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.timeRate.visible') }}</th>
              <th class="w-20 px-3 py-2 text-center font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.timeRate.enabled') }}</th>
              <th class="w-12 px-2 py-2"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
            <tr v-for="(period, index) in periods" :key="periodKeys[index]">
              <td class="px-3 py-2">
                <input v-model.trim="period.start_time" class="input w-28" inputmode="numeric" placeholder="00:00" />
              </td>
              <td class="px-3 py-2">
                <input v-model.trim="period.end_time" class="input w-28" inputmode="numeric" placeholder="24:00" />
              </td>
              <td class="px-3 py-2">
                <input v-model.number="period.rate_multiplier" type="number" min="0" step="0.0001" class="input w-28" />
              </td>
              <td class="px-3 py-2">
                <input v-model.number="period.visible_rate_multiplier" type="number" min="0" step="0.0001" class="input w-28" />
              </td>
              <td class="px-3 py-2 text-center">
                <input v-model="period.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              </td>
              <td class="px-2 py-2 text-center">
                <button
                  type="button"
                  class="rounded p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                  :title="t('common.delete')"
                  @click="removePeriod(index)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </td>
            </tr>
            <tr v-if="periods.length === 0">
              <td colspan="6" class="px-4 py-10 text-center text-gray-400 dark:text-gray-500">
                {{ t('admin.groups.timeRate.empty') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p v-if="validationError" class="text-sm text-red-600 dark:text-red-400">{{ validationError }}</p>

      <div class="flex items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-600">
        <template v-if="isDirty">
          <span class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.groups.unsavedChanges') }}</span>
          <button type="button" class="text-xs font-medium text-primary-600 dark:text-primary-400" @click="reset">
            {{ t('admin.groups.revertChanges') }}
          </button>
        </template>
        <div class="ml-auto flex items-center gap-3">
          <button type="button" class="btn btn-sm px-4 py-1.5" @click="handleClose">{{ t('common.close') }}</button>
          <button type="button" class="btn btn-primary btn-sm px-4 py-1.5" :disabled="!isDirty || saving" @click="save">
            <Icon v-if="saving" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminGroup, GroupTimeRatePeriod, TimeRatePriority } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

const props = defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ close: []; success: [] }>()
const { t } = useI18n()
const appStore = useAppStore()

const priority = ref<TimeRatePriority>('schedule_first')
const periods = ref<GroupTimeRatePeriod[]>([])
const original = ref('')
const saving = ref(false)
const validationError = ref('')
let nextKey = 0
const periodKeys = ref<number[]>([])

const priorityOptions = computed(() => [
  { value: 'schedule_first' as const, label: t('admin.groups.timeRate.scheduleFirst') },
  { value: 'user_first' as const, label: t('admin.groups.timeRate.userFirst') },
  { value: 'proportional' as const, label: t('admin.groups.timeRate.proportional') }
])

const snapshot = () => JSON.stringify({ priority: priority.value, periods: periods.value })
const isDirty = computed(() => snapshot() !== original.value)

function load() {
  priority.value = props.group?.time_rate_priority ?? 'schedule_first'
  periods.value = (props.group?.time_rate_periods ?? []).map(period => ({ ...period }))
  periodKeys.value = periods.value.map(() => ++nextKey)
  validationError.value = ''
  original.value = snapshot()
}

watch(() => [props.show, props.group?.id], ([show]) => { if (show) load() }, { immediate: true })

function addPeriod() {
  const rate = props.group?.rate_multiplier ?? 1
  periods.value.push({ start_time: '00:00', end_time: '01:00', rate_multiplier: rate, visible_rate_multiplier: rate, enabled: true })
  periodKeys.value.push(++nextKey)
}

function removePeriod(index: number) {
  periods.value.splice(index, 1)
  periodKeys.value.splice(index, 1)
}

function reset() { load() }

function validate(): string {
  const windows: Array<[number, number]> = []
  const parse = (value: string, allow24 = false) => {
    if (allow24 && value === '24:00') return 1440
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(value)) return -1
    const [hour, minute] = value.split(':').map(Number)
    return hour * 60 + minute
  }
  for (const period of periods.value) {
    const start = parse(period.start_time)
    const end = parse(period.end_time, true)
    if (start < 0 || end <= start) return t('admin.groups.timeRate.invalidTime')
    if (!Number.isFinite(period.rate_multiplier) || period.rate_multiplier < 0 || !Number.isFinite(period.visible_rate_multiplier) || period.visible_rate_multiplier < 0) {
      return t('admin.groups.timeRate.invalidRate')
    }
    if (period.enabled) windows.push([start, end])
  }
  windows.sort((a, b) => a[0] - b[0])
  if (windows.some((window, index) => index > 0 && window[0] < windows[index - 1][1])) return t('admin.groups.timeRate.overlap')
  return ''
}

async function save() {
  validationError.value = validate()
  if (validationError.value || !props.group) return
  saving.value = true
  try {
    await adminAPI.groups.update(props.group.id, { time_rate_priority: priority.value, time_rate_periods: periods.value })
    appStore.showSuccess(t('admin.groups.timeRate.saved'))
    original.value = snapshot()
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.groups.timeRate.saveFailed'))
  } finally {
    saving.value = false
  }
}

function handleClose() {
  if (isDirty.value && !window.confirm(t('admin.groups.timeRate.discardConfirm'))) return
  emit('close')
}
</script>
