<template>
  <BaseDialog
    :show="show"
    :title="t('admin.announcements.emailBroadcast.title')"
    width="extra-wide"
    @close="handleClose"
  >
    <div v-if="loading && !overview" class="py-10 text-center text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="overview && !overview.broadcast" class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">
        {{ overview.can_send
          ? t('admin.announcements.emailBroadcast.confirmMessage', { count: overview.eligible_count })
          : t('admin.announcements.emailBroadcast.unavailable') }}
      </p>
    </div>

    <div v-else-if="broadcast" class="space-y-4">
      <div class="grid grid-cols-2 border border-gray-200 dark:border-dark-600 sm:grid-cols-4">
        <div class="p-3">
          <div class="text-xs text-gray-500">{{ t('admin.announcements.emailBroadcast.total') }}</div>
          <div class="mt-1 text-lg font-semibold">{{ broadcast.total_count }}</div>
        </div>
        <div class="border-l border-gray-200 p-3 dark:border-dark-600">
          <div class="text-xs text-gray-500">{{ t('admin.announcements.emailBroadcast.pending') }}</div>
          <div class="mt-1 text-lg font-semibold text-amber-600">{{ broadcast.pending_count }}</div>
        </div>
        <div class="border-t border-gray-200 p-3 dark:border-dark-600 sm:border-l sm:border-t-0">
          <div class="text-xs text-gray-500">{{ t('admin.announcements.emailBroadcast.sent') }}</div>
          <div class="mt-1 text-lg font-semibold text-green-600">{{ broadcast.sent_count }}</div>
        </div>
        <div class="border-l border-t border-gray-200 p-3 dark:border-dark-600 sm:border-t-0">
          <div class="text-xs text-gray-500">{{ t('admin.announcements.emailBroadcast.failed') }}</div>
          <div class="mt-1 text-lg font-semibold text-red-600">{{ broadcast.failed_count }}</div>
        </div>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row">
        <input v-model="search" class="input flex-1" :placeholder="t('admin.announcements.searchUsers')" @input="handleSearch" />
        <Select v-model="status" :options="statusOptions" class="w-full sm:w-40" @change="handleFilterChange" />
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadAll">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <DataTable :columns="columns" :data="deliveries" :loading="loading">
        <template #cell-email="{ value }"><span class="font-medium">{{ value }}</span></template>
        <template #cell-status="{ value }">
          <span :class="['badge', value === 'sent' ? 'badge-success' : value === 'failed' ? 'badge-danger' : 'badge-warning']">
            {{ deliveryStatusLabel(value) }}
          </span>
        </template>
        <template #cell-last_attempt_at="{ value }">
          <span class="text-sm text-gray-500">{{ value ? formatDateTime(value) : '-' }}</span>
        </template>
        <template #cell-error_message="{ value }">
          <span class="block max-w-80 truncate text-sm text-red-600" :title="value || ''">{{ value || '-' }}</span>
        </template>
      </DataTable>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <template #footer>
      <div class="flex w-full justify-between gap-3">
        <button
          v-if="broadcast?.failed_count"
          data-testid="retry-failed"
          class="btn btn-secondary"
          :disabled="retrying"
          @click="retryFailed"
        >
          <Icon name="refresh" size="sm" class="mr-1" />
          {{ t('admin.announcements.emailBroadcast.retryFailed') }}
        </button>
        <span v-else></span>
        <div class="flex gap-3">
          <button class="btn btn-secondary" @click="handleClose">{{ t('common.close') }}</button>
          <button
            v-if="overview && !overview.broadcast"
            data-testid="send-broadcast"
            class="btn btn-primary"
            :disabled="!overview.can_send || sending"
            @click="sendBroadcast"
          >
            <Icon name="mail" size="sm" class="mr-1" />
            {{ t('admin.announcements.emailBroadcast.confirmSend') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import type {
  AnnouncementEmailBroadcast,
  AnnouncementEmailBroadcastOverview,
  AnnouncementEmailDelivery,
  AnnouncementEmailDeliveryStatus
} from '@/types'
import type { Column } from '@/components/common/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; announcementId: number | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'broadcast-created', id: number): void }>()
const { t } = useI18n()
const appStore = useAppStore()

const overview = ref<AnnouncementEmailBroadcastOverview | null>(null)
const deliveries = ref<AnnouncementEmailDelivery[]>([])
const loading = ref(false)
const sending = ref(false)
const retrying = ref(false)
const search = ref('')
const status = ref<AnnouncementEmailDeliveryStatus | ''>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
let pollTimer: number | null = null
let searchTimer: number | null = null
let requestVersion = 0

const broadcast = computed<AnnouncementEmailBroadcast | null>(() => overview.value?.broadcast ?? null)
const columns = computed<Column[]>(() => [
  { key: 'email', label: t('common.email') },
  { key: 'status', label: t('common.status') },
  { key: 'attempt_count', label: t('admin.announcements.emailBroadcast.attempts') },
  { key: 'last_attempt_at', label: t('admin.announcements.emailBroadcast.lastAttempt') },
  { key: 'error_message', label: t('admin.announcements.emailBroadcast.error') }
])
const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  ...(['pending', 'processing', 'sent', 'failed'] as AnnouncementEmailDeliveryStatus[]).map(value => ({
    value,
    label: deliveryStatusLabel(value)
  }))
])

function deliveryStatusLabel(value: string) {
  return t(`admin.announcements.emailBroadcast.status.${value}`)
}

function schedulePoll() {
  if (pollTimer) window.clearTimeout(pollTimer)
  if (!props.show || !broadcast.value || !['pending', 'running'].includes(broadcast.value.status)) return
  pollTimer = window.setTimeout(loadAll, 2000)
}

async function loadOverview(announcementId: number, version: number) {
  const result = await adminAPI.announcements.getEmailBroadcast(announcementId)
  if (version !== requestVersion) return false
  overview.value = result
  if (result.broadcast) emit('broadcast-created', announcementId)
  return true
}

async function loadDeliveries(announcementId: number, version: number) {
  if (!broadcast.value) return
  const result = await adminAPI.announcements.listEmailDeliveries(
    announcementId,
    pagination.page,
    pagination.page_size,
    { status: status.value, search: search.value.trim() || undefined }
  )
  if (version !== requestVersion) return
  deliveries.value = result.items
  Object.assign(pagination, { page: result.page, page_size: result.page_size, total: result.total, pages: result.pages })
}

async function loadAll() {
  const announcementId = props.announcementId
  if (!props.show || !announcementId) return
  const version = ++requestVersion
  loading.value = true
  try {
    if (!await loadOverview(announcementId, version)) return
    await loadDeliveries(announcementId, version)
  } catch (error: any) {
    if (version !== requestVersion) return
    appStore.showError(error.response?.data?.detail || t('admin.announcements.emailBroadcast.loadFailed'))
  } finally {
    if (version === requestVersion) {
      loading.value = false
      schedulePoll()
    }
  }
}

async function sendBroadcast() {
  const announcementId = props.announcementId
  if (!announcementId || !overview.value?.can_send) return
  const version = ++requestVersion
  sending.value = true
  try {
    const created = await adminAPI.announcements.createEmailBroadcast(announcementId)
    if (version !== requestVersion) return
    overview.value = { broadcast: created, eligible_count: 0, can_send: false }
    emit('broadcast-created', announcementId)
    appStore.showSuccess(t('admin.announcements.emailBroadcast.started'))
    await loadDeliveries(announcementId, version)
  } catch (error: any) {
    if (version !== requestVersion) return
    appStore.showError(error.response?.data?.detail || t('admin.announcements.emailBroadcast.sendFailed'))
  } finally {
    if (version === requestVersion) {
      sending.value = false
      schedulePoll()
    }
  }
}

async function retryFailed() {
  const announcementId = props.announcementId
  if (!announcementId) return
  const version = ++requestVersion
  retrying.value = true
  try {
    const updated = await adminAPI.announcements.retryFailedEmailDeliveries(announcementId)
    if (version !== requestVersion) return
    overview.value = { broadcast: updated, eligible_count: 0, can_send: false }
    appStore.showSuccess(t('admin.announcements.emailBroadcast.retryStarted'))
    await loadDeliveries(announcementId, version)
  } catch (error: any) {
    if (version !== requestVersion) return
    appStore.showError(error.response?.data?.detail || t('admin.announcements.emailBroadcast.retryFailedMessage'))
  } finally {
    if (version === requestVersion) {
      retrying.value = false
      schedulePoll()
    }
  }
}

function handleFilterChange() { pagination.page = 1; loadAll() }
function handlePageChange(page: number) { pagination.page = page; loadAll() }
function handlePageSizeChange(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; loadAll() }
function handleSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => { pagination.page = 1; loadAll() }, 300)
}
function handleClose() { emit('close') }

watch(() => [props.show, props.announcementId], ([show]) => {
  requestVersion++
  if (!show) {
    if (pollTimer) window.clearTimeout(pollTimer)
    return
  }
  overview.value = null
  deliveries.value = []
  loading.value = false
  sending.value = false
  retrying.value = false
  search.value = ''
  status.value = ''
  pagination.page = 1
  loadAll()
}, { immediate: true })

onUnmounted(() => {
  requestVersion++
  if (pollTimer) window.clearTimeout(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>
