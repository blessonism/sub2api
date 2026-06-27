<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <Select
            v-model="typeFilter"
            :options="typeOptions"
            class="w-56"
            @change="handleTypeChange"
          />
          <button
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="loadRecords"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="records" :loading="loading">
          <template #cell-user="{ row }">
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                {{ row.user?.email || t('admin.redeemRecords.userFallback', { id: row.used_by }) }}
              </p>
              <p class="text-xs text-gray-400 dark:text-dark-500">
                ID: {{ row.user?.id || row.used_by || '-' }}
              </p>
            </div>
          </template>

          <template #cell-type="{ row }">
            <span :class="['badge', getTypeBadgeClass(row.type)]">
              {{ getTypeLabel(row.type) }}
            </span>
          </template>

          <template #cell-value="{ row }">
            <span :class="['text-sm font-semibold', getValueColor(row)]">
              {{ formatValue(row) }}
            </span>
          </template>

          <template #cell-occurred_at="{ row }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(row.used_at || row.created_at) }}
            </span>
          </template>

          <template #cell-source="{ row }">
            <div class="max-w-xs">
              <p class="truncate text-sm text-gray-700 dark:text-gray-200" :title="getSourceText(row)">
                {{ getSourceText(row) }}
              </p>
              <p v-if="row.notes" class="truncate text-xs text-gray-400 dark:text-dark-500" :title="row.notes">
                {{ row.notes }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { BalanceHistoryItem } from '@/api/admin/users'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const records = ref<BalanceHistoryItem[]>([])
const loading = ref(false)
const typeFilter = ref('')

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 1
})

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.redeemRecords.columns.user') },
  { key: 'type', label: t('admin.redeemRecords.columns.type') },
  { key: 'value', label: t('admin.redeemRecords.columns.value') },
  { key: 'occurred_at', label: t('admin.redeemRecords.columns.occurredAt') },
  { key: 'source', label: t('admin.redeemRecords.columns.source') }
])

const typeOptions = computed(() => [
  { value: '', label: t('admin.users.allTypes') },
  { value: 'balance', label: t('admin.users.typeBalance') },
  { value: 'affiliate_balance', label: t('admin.users.typeAffiliateBalance') },
  { value: 'admin_balance', label: t('admin.users.typeAdminBalance') },
  { value: 'concurrency', label: t('admin.users.typeConcurrency') },
  { value: 'admin_concurrency', label: t('admin.users.typeAdminConcurrency') },
  { value: 'subscription', label: t('admin.users.typeSubscription') }
])

const loadRecords = async () => {
  loading.value = true
  try {
    const response = await adminAPI.users.getGlobalBalanceHistory(
      pagination.page,
      pagination.page_size,
      typeFilter.value || undefined
    )
    records.value = response.items || []
    pagination.total = response.total || 0
    pagination.pages = response.pages || 1
  } catch (error) {
    appStore.showError(t('admin.redeemRecords.failedToLoad'))
    console.error('Error loading global redeem records:', error)
  } finally {
    loading.value = false
  }
}

const handleTypeChange = () => {
  pagination.page = 1
  loadRecords()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadRecords()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRecords()
}

const isBalanceType = (type: string) =>
  type === 'balance' || type === 'admin_balance' || type === 'affiliate_balance'

const isSubscriptionType = (type: string) => type === 'subscription'

const getTypeLabel = (type: string) => {
  switch (type) {
    case 'balance':
      return t('admin.users.typeBalance')
    case 'affiliate_balance':
      return t('admin.users.typeAffiliateBalance')
    case 'admin_balance':
      return t('admin.users.typeAdminBalance')
    case 'concurrency':
      return t('admin.users.typeConcurrency')
    case 'admin_concurrency':
      return t('admin.users.typeAdminConcurrency')
    case 'subscription':
      return t('admin.users.typeSubscription')
    default:
      return t('common.unknown')
  }
}

const getTypeBadgeClass = (type: string) => {
  if (isBalanceType(type)) return 'badge-success'
  if (isSubscriptionType(type)) return 'badge-warning'
  return 'badge-primary'
}

const getValueColor = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'text-emerald-600 dark:text-emerald-400'
      : 'text-red-600 dark:text-red-400'
  }
  if (isSubscriptionType(item.type)) return 'text-purple-600 dark:text-purple-400'
  return item.value >= 0
    ? 'text-blue-600 dark:text-blue-400'
    : 'text-orange-600 dark:text-orange-400'
}

const formatValue = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  }
  if (isSubscriptionType(item.type)) {
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}d - ${groupName}` : `${days}d`
  }
  const sign = item.value >= 0 ? '+' : ''
  return `${sign}${item.value}`
}

const getSourceText = (item: BalanceHistoryItem) => {
  if (item.type === 'affiliate_balance') return t('admin.redeemRecords.sources.affiliate')
  if (item.type === 'admin_balance' || item.type === 'admin_concurrency') {
    return t('admin.redeemRecords.sources.adminAdjustment')
  }
  if (item.code) return `${item.code.slice(0, 8)}...`
  return t('admin.redeemRecords.sources.system')
}

onMounted(loadRecords)
</script>
