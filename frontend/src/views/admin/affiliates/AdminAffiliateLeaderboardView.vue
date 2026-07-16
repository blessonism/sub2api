<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-80">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="search"
              data-testid="leaderboard-search"
              type="search"
              class="input pl-10"
              :placeholder="t('admin.affiliates.leaderboard.searchPlaceholder')"
              @input="debounceLoad"
            />
          </div>
          <button
            type="button"
            class="btn btn-secondary px-2 md:px-3"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="loadLeaderboard"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="rows" :loading="loading" row-key="user_id">
          <template #cell-rank="{ row }">
            <span class="font-semibold text-gray-900 dark:text-white">#{{ row.rank }}</span>
          </template>
          <template #cell-inviter="{ row }">
            <div class="space-y-0.5">
              <div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ row.user_id }}</div>
              <div class="max-w-64 truncate font-medium text-gray-900 dark:text-white">{{ row.email || '-' }}</div>
              <div class="max-w-64 truncate text-xs text-gray-500 dark:text-dark-400">{{ row.username || '-' }}</div>
            </div>
          </template>
          <template #cell-aff_code="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ row.aff_code || '-' }}</span>
          </template>
          <template #cell-invite_count="{ row }">
            <span class="font-semibold text-gray-900 dark:text-white">{{ row.invite_count }}</span>
          </template>
          <template #cell-all_credit_amount="{ row }">
            <span class="font-medium text-gray-900 dark:text-white">${{ formatAmount(row.all_credit_amount) }}</span>
          </template>
          <template #cell-payment_redeem_amount="{ row }">
            <span class="font-semibold text-emerald-600 dark:text-emerald-400">${{ formatAmount(row.payment_redeem_amount) }}</span>
          </template>
          <template #empty>
            <div class="py-4 text-center">
              <p class="font-medium text-gray-900 dark:text-white">{{ t('admin.affiliates.leaderboard.empty') }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.leaderboard.emptyDescription') }}</p>
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
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { affiliatesAPI, type AffiliateLeaderboardEntry } from '@/api/admin/affiliates'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const rows = ref<AffiliateLeaderboardEntry[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'rank', label: t('admin.affiliates.leaderboard.rank') },
  { key: 'inviter', label: t('admin.affiliates.leaderboard.inviter') },
  { key: 'aff_code', label: t('admin.affiliates.leaderboard.affCode') },
  { key: 'invite_count', label: t('admin.affiliates.leaderboard.inviteCount') },
  { key: 'all_credit_amount', label: t('admin.affiliates.leaderboard.allCreditAmount') },
  { key: 'payment_redeem_amount', label: t('admin.affiliates.leaderboard.paymentRedeemAmount') },
])

async function loadLeaderboard(): Promise<void> {
  loading.value = true
  try {
    const result = await affiliatesAPI.listLeaderboard({
      page: pagination.page,
      page_size: pagination.page_size,
      search: search.value.trim(),
    })
    rows.value = result.items || []
    pagination.total = result.total || 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.affiliates.leaderboard.loadFailed')))
  } finally {
    loading.value = false
  }
}

function debounceLoad(): void {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    pagination.page = 1
    void loadLeaderboard()
  }, 300)
}

function handlePageChange(page: number): void {
  pagination.page = page
  void loadLeaderboard()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLeaderboard()
}

function formatAmount(value: number): string {
  return Number(value || 0).toFixed(2)
}

onMounted(() => void loadLeaderboard())
onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
