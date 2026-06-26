<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.tokenLeaderboard.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.tokenLeaderboard.description') }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2 self-start md:self-auto">
          <button
            class="btn btn-secondary inline-flex items-center gap-2"
            type="button"
            :aria-pressed="showUserIds"
            @click="showUserIds = !showUserIds"
          >
            <Icon :name="showUserIds ? 'eyeOff' : 'eye'" size="sm" />
            {{ showUserIds ? t('admin.tokenLeaderboard.hideUserIds') : t('admin.tokenLeaderboard.showUserIds') }}
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="loadLeaderboard">
            <Icon name="refresh" size="sm" />
            {{ t('admin.tokenLeaderboard.refresh') }}
          </button>
        </div>
      </div>

      <div class="card p-4">
        <div class="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(240px,1.2fr)_repeat(5,minmax(140px,1fr))_auto]">
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.timeRange') }}</span>
            <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="onDateRangeChange" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.emailSearch') }}</span>
            <input
              v-model.trim="emailSearch"
              type="search"
              class="input w-full"
              :placeholder="t('admin.tokenLeaderboard.emailPlaceholder')"
              @keyup.enter="applyFilters"
            />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.group') }}</span>
            <Select v-model="groupId" :options="groupOptions" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.modelSearch') }}</span>
            <input
              v-model.trim="modelSearch"
              type="search"
              class="input w-full"
              :placeholder="t('admin.tokenLeaderboard.modelPlaceholder')"
              @keyup.enter="applyFilters"
            />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.status') }}</span>
            <Select v-model="userStatus" :options="statusOptions" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.topN') }}</span>
            <Select v-model="limit" :options="limitOptions" />
          </label>
          <div class="flex items-end gap-2">
            <button class="btn btn-secondary" type="button" @click="resetFilters">
              {{ t('admin.tokenLeaderboard.reset') }}
            </button>
            <button class="btn btn-primary" type="button" @click="applyFilters">
              {{ t('common.search') }}
            </button>
          </div>
        </div>
      </div>

      <div class="card p-4">
        <div class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(260px,1fr)_auto] lg:items-end">
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.commonGroup') }}</span>
            <Select
              v-model="commonGroupId"
              :options="commonGroupOptions"
              :placeholder="t('admin.tokenLeaderboard.commonGroupPlaceholder')"
              :disabled="settingsLoading || settingsSaving"
              searchable
              clearable
              data-testid="token-leaderboard-common-group-select"
            />
          </label>
          <button
            class="btn btn-primary inline-flex items-center justify-center gap-2"
            type="button"
            :disabled="!canSaveCommonGroup"
            @click="saveCommonGroup"
            data-testid="token-leaderboard-common-group-save"
          >
            <Icon name="save" size="sm" />
            {{ settingsSaving ? t('admin.tokenLeaderboard.savingCommonGroup') : t('admin.tokenLeaderboard.saveCommonGroup') }}
          </button>
        </div>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ commonGroupHint }}
        </p>
      </div>

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div v-for="metric in summaryMetrics" :key="metric.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
        </div>
      </div>

      <div v-if="ranking.length > 0" class="card p-4">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div class="space-y-2">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.tokenLeaderboard.grantTop10Title') }}
            </div>
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.tokenLeaderboard.selectedUsers', { count: selectedRows.length, total: topTenRows.length }) }}
            </div>
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" @click="selectCurrentTop10">
                {{ t('admin.tokenLeaderboard.selectTop10') }}
              </button>
              <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" :disabled="selectedRows.length === 0" @click="clearSelectedUsers">
                {{ t('admin.tokenLeaderboard.clearSelection') }}
              </button>
            </div>
          </div>
          <div class="grid w-full grid-cols-1 gap-3 md:grid-cols-[minmax(140px,180px)_minmax(220px,1fr)_auto] xl:w-auto">
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.grantAmount') }}</span>
              <input v-model.number="grantAmount" type="number" min="0" step="any" class="input w-full" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.grantNotes') }}</span>
              <input v-model.trim="grantNotes" type="text" class="input w-full" :placeholder="t('admin.tokenLeaderboard.grantNotesPlaceholder')" />
            </label>
            <div class="flex items-end">
              <button class="btn btn-primary inline-flex w-full items-center justify-center gap-2" type="button" :disabled="!canOpenGrantDialog" @click="openGrantDialog">
                <Icon name="gift" size="sm" />
                {{ t('admin.tokenLeaderboard.grantBalance') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex min-h-80 items-center justify-center">
          <LoadingSpinner />
        </div>
        <EmptyState
          v-else-if="ranking.length === 0"
          class="min-h-80"
          :title="t('admin.tokenLeaderboard.noData')"
          :description="t('admin.tokenLeaderboard.noDataDescription')"
          :action-text="t('admin.tokenLeaderboard.refresh')"
          @action="loadLeaderboard"
        />
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1080px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="w-12 px-4 py-3 text-left">{{ t('admin.tokenLeaderboard.select') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenLeaderboard.rank') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenLeaderboard.user') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenLeaderboard.status') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenLeaderboard.registeredAt') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.tokenLeaderboard.requests') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.tokenLeaderboard.tokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.tokenLeaderboard.actualCost') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.tokenLeaderboard.lastUsedAt') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.tokenLeaderboard.details') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <template v-for="row in ranking" :key="row.user_id">
                <tr class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                  <td class="px-4 py-3">
                    <input
                      type="checkbox"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-40"
                      :checked="isSelected(row.user_id)"
                      :disabled="!isTopTen(row)"
                      :aria-label="t('admin.tokenLeaderboard.selectUser', { email: row.email })"
                      @change="onRowSelectionChange(row, $event)"
                    />
                  </td>
                  <td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">#{{ row.rank }}</td>
                  <td class="px-4 py-3">
                    <div class="min-w-0">
                      <div class="truncate font-medium text-gray-900 dark:text-white">{{ displayUser(row) }}</div>
                      <div v-if="secondaryUserText(row)" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                        {{ secondaryUserText(row) }}
                      </div>
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <span :class="statusClass(row.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                      {{ statusLabel(row.status) }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(row.registered_at) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(row.requests) }}</td>
                  <td class="px-4 py-3 text-right font-medium tabular-nums text-gray-900 dark:text-white">{{ formatNumber(row.tokens) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(row.actual_cost) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300">{{ formatLastUsedAt(row.last_used_at) }}</td>
                  <td class="px-4 py-3 text-right">
                    <button class="btn btn-secondary px-2 py-1" type="button" @click="toggleDetails(row.user_id)">
                      <Icon name="chevronDown" size="sm" :class="['transition-transform', expandedUserId === row.user_id && 'rotate-180']" />
                    </button>
                  </td>
                </tr>
                <tr v-if="expandedUserId === row.user_id">
                  <td colspan="10" class="bg-gray-50 px-4 py-4 dark:bg-dark-900/60">
                    <div v-if="detailLoading[row.user_id]" class="flex h-24 items-center justify-center">
                      <LoadingSpinner />
                    </div>
                    <div v-else-if="detailsCache[row.user_id]" class="space-y-4">
                      <div
                        v-if="detailsCache[row.user_id]?.calibration_tokens"
                        class="flex items-center justify-between rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm dark:border-amber-900/60 dark:bg-amber-950/30"
                      >
                        <span class="font-medium text-amber-800 dark:text-amber-200">{{ t('admin.tokenLeaderboard.calibrationTokens') }}</span>
                        <span class="font-semibold tabular-nums" :class="calibrationClass(detailsCache[row.user_id]?.calibration_tokens || 0)">
                          {{ formatSignedTokens(detailsCache[row.user_id]?.calibration_tokens || 0) }}
                        </span>
                      </div>
                      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
                        <section>
                          <h3 class="mb-2 text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.tokenLeaderboard.apiKeyDetails') }}</h3>
                          <DetailTable
                            :rows="detailsCache[row.user_id]?.api_keys || []"
                            name-key="api_key_name"
                            id-key="api_key_id"
                            :empty-name="t('admin.tokenLeaderboard.noName')"
                          />
                        </section>
                        <section>
                          <h3 class="mb-2 text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.tokenLeaderboard.groupDetails') }}</h3>
                          <DetailTable
                            :rows="detailsCache[row.user_id]?.groups || []"
                            name-key="group_name"
                            id-key="group_id"
                            :empty-name="t('admin.tokenLeaderboard.noGroup')"
                          />
                        </section>
                        <section>
                          <h3 class="mb-2 text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.tokenLeaderboard.modelDetails') }}</h3>
                          <DetailTable
                            :rows="detailsCache[row.user_id]?.models || []"
                            name-key="model"
                            :empty-name="t('admin.tokenLeaderboard.noModel')"
                          />
                        </section>
                      </div>
                    </div>
                    <div v-else class="text-sm text-red-600 dark:text-red-300">
                      {{ t('admin.tokenLeaderboard.failedToLoadDetails') }}
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>

      <BaseDialog :show="grantDialogOpen" :title="t('admin.tokenLeaderboard.confirmGrantTitle')" width="narrow" @close="grantDialogOpen = false">
        <div class="space-y-4">
          <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.tokenLeaderboard.confirmGrantUsers') }}</div>
            <div class="mt-2 max-h-40 space-y-2 overflow-y-auto">
              <div v-for="row in selectedRows" :key="row.user_id" class="flex items-center justify-between gap-3 text-sm">
                <span class="truncate font-medium text-gray-900 dark:text-white">{{ displayUser(row) }}</span>
                <span class="shrink-0 text-gray-500 dark:text-gray-400">#{{ row.rank }} · {{ row.email }}</span>
              </div>
            </div>
          </div>
          <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm dark:border-emerald-900/60 dark:bg-emerald-950/30">
            <div class="flex items-center justify-between gap-3">
              <span class="text-gray-600 dark:text-gray-300">{{ t('admin.tokenLeaderboard.grantAmount') }}</span>
              <span class="font-semibold text-gray-900 dark:text-white">{{ formatCost(grantAmount) }}</span>
            </div>
            <div v-if="grantNotes" class="mt-2 text-gray-600 dark:text-gray-300">
              {{ grantNotes }}
            </div>
          </div>
        </div>
        <template #footer>
          <div class="flex justify-end gap-3">
            <button class="btn btn-secondary" type="button" :disabled="granting" @click="grantDialogOpen = false">
              {{ t('common.cancel') }}
            </button>
            <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="granting" @click="confirmGrantBalance">
              <Icon name="gift" size="sm" />
              {{ granting ? t('admin.tokenLeaderboard.granting') : t('common.confirm') }}
            </button>
          </div>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatMultiplier as formatAdaptiveMultiplier } from '@/utils/formatters'
import type {
  AdminTokenLeaderboardParams,
  AdminTokenLeaderboardAPIKeyUsage,
  AdminTokenLeaderboardGroupUsage,
  AdminTokenLeaderboardModelUsage,
  AdminTokenLeaderboardResponse,
  AdminTokenLeaderboardUser,
  AdminTokenLeaderboardUserDetailsResponse
} from '@/api/admin/dashboard'
import type { AdminGroup } from '@/types'
import { formatCostFixed, formatDateTime, formatNumber } from '@/utils/format'

type LimitOption = 10 | 20 | 50 | 100
type UserStatusFilter = '' | 'active' | 'disabled'
type ModelSource = 'requested' | 'upstream' | 'mapping'
type DetailRow =
  | AdminTokenLeaderboardAPIKeyUsage
  | AdminTokenLeaderboardGroupUsage
  | AdminTokenLeaderboardModelUsage

const DetailTable = defineComponent({
  props: {
    rows: { type: Array as () => DetailRow[], required: true },
    nameKey: { type: String, required: true },
    idKey: { type: String, default: '' },
    emptyName: { type: String, required: true }
  },
  setup(props) {
    const fmtNumber = (value: unknown) => formatNumber(Number(value || 0))
    const fmtCost = (value: unknown) => formatCostFixed(Number(value || 0), 4)
    const labelFor = (row: DetailRow) => {
      const record = row as unknown as Record<string, unknown>
      const raw = String(record[props.nameKey] || '').trim()
      const id = props.idKey ? Number(record[props.idKey] || 0) : 0
      if (raw && id > 0) return `${raw} (#${id})`
      if (raw) return raw
      return id > 0 ? `#${id}` : props.emptyName
    }

    return () =>
      h('div', { class: 'overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800' }, [
        h('table', { class: 'w-full text-xs' }, [
          h('thead', { class: 'bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-gray-400' }, [
            h('tr', [
              h('th', { class: 'px-3 py-2 text-left font-medium' }, ''),
              h('th', { class: 'px-3 py-2 text-right font-medium' }, t('admin.tokenLeaderboard.requests')),
              h('th', { class: 'px-3 py-2 text-right font-medium' }, t('admin.tokenLeaderboard.tokens')),
              h('th', { class: 'px-3 py-2 text-right font-medium' }, t('admin.tokenLeaderboard.actualCost'))
            ])
          ]),
          h('tbody', { class: 'divide-y divide-gray-100 dark:divide-dark-700' },
            props.rows.length
              ? props.rows.map((row) =>
                  h('tr', [
                    h('td', { class: 'px-3 py-2 text-gray-700 dark:text-gray-200' }, labelFor(row)),
                    h('td', { class: 'px-3 py-2 text-right tabular-nums text-gray-500 dark:text-gray-400' }, fmtNumber(row.requests)),
                    h('td', { class: 'px-3 py-2 text-right tabular-nums text-gray-500 dark:text-gray-400' }, fmtNumber(row.tokens)),
                    h('td', { class: 'px-3 py-2 text-right tabular-nums text-gray-500 dark:text-gray-400' }, fmtCost(row.actual_cost))
                  ])
                )
              : h('tr', [
                  h('td', { class: 'px-3 py-3 text-center text-gray-500 dark:text-gray-400', colspan: 4 }, props.emptyName)
                ])
          )
        ])
      ])
  }
})

const { t } = useI18n()
const appStore = useAppStore()

const todayString = () => {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const startDate = ref(todayString())
const endDate = ref(todayString())
const emailSearch = ref('')
const modelSearch = ref('')
const modelSource = ref<ModelSource>('requested')
const groupId = ref<number | null>(null)
const userStatus = ref<UserStatusFilter>('')
const limit = ref<LimitOption>(10)
const groups = ref<AdminGroup[]>([])
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const commonGroupId = ref<number | null>(null)
const savedCommonGroupId = ref<number | null>(null)
const leaderboard = ref<AdminTokenLeaderboardResponse | null>(null)
const loading = ref(false)
const expandedUserId = ref<number | null>(null)
const detailsCache = reactive<Record<number, AdminTokenLeaderboardUserDetailsResponse | undefined>>({})
const detailLoading = reactive<Record<number, boolean>>({})
const detailsCacheVersion = ref(0)
const selectedUserIds = ref<number[]>([])
const grantAmount = ref(0)
const grantNotes = ref('')
const grantDialogOpen = ref(false)
const granting = ref(false)
const showUserIds = ref(false)

const ranking = computed(() => leaderboard.value?.ranking || [])
const latestLastUsedAt = computed(() => {
  let latest = 0
  let latestValue = ''
  ranking.value.forEach((row) => {
    const timestamp = Date.parse(row.last_used_at)
    if (Number.isFinite(timestamp) && timestamp > latest) {
      latest = timestamp
      latestValue = row.last_used_at
    }
  })
  return latestValue
})
const topTenRows = computed(() => ranking.value.filter((row) => isTopTen(row)))
const selectedRows = computed(() => {
  const selected = new Set(selectedUserIds.value)
  return topTenRows.value.filter((row) => selected.has(row.user_id))
})
const canOpenGrantDialog = computed(() => selectedRows.value.length > 0 && grantAmount.value > 0 && !granting.value)
const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.tokenLeaderboard.allGroups') },
  ...groups.value.map((group) => ({ value: group.id, label: `${group.name} (#${group.id})` }))
])
const commonGroups = computed(() => groups.value.filter((group) => (
  group.status === 'active' && group.subscription_type === 'standard' && !group.is_exclusive
)))
const commonGroupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.tokenLeaderboard.commonGroupUnset') },
  ...commonGroups.value.map((group) => ({
    value: group.id,
    label: `${group.name} · ${formatMultiplier(group.rate_multiplier)} (#${group.id})`
  }))
])
const selectedCommonGroup = computed(() => commonGroups.value.find((group) => group.id === commonGroupId.value) || null)
const canSaveCommonGroup = computed(() => !settingsLoading.value && !settingsSaving.value && commonGroupId.value !== savedCommonGroupId.value)
const commonGroupHint = computed(() => {
  if (selectedCommonGroup.value) {
    return t('admin.tokenLeaderboard.commonGroupSelectedHint', {
      group: selectedCommonGroup.value.name,
      multiplier: formatMultiplier(selectedCommonGroup.value.rate_multiplier)
    })
  }
  return t('admin.tokenLeaderboard.commonGroupUnsetHint')
})
const statusOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.tokenLeaderboard.allStatuses') },
  { value: 'active', label: t('admin.tokenLeaderboard.active') },
  { value: 'disabled', label: t('admin.tokenLeaderboard.disabled') }
])
const limitOptions: SelectOption[] = [10, 20, 50, 100].map((value) => ({ value, label: `Top${value}` }))
const summaryMetrics = computed(() => [
  { label: t('admin.tokenLeaderboard.requests'), value: formatNumber(leaderboard.value?.total_requests || 0) },
  { label: t('admin.tokenLeaderboard.tokens'), value: formatNumber(leaderboard.value?.total_tokens || 0) },
  { label: t('admin.tokenLeaderboard.actualCost'), value: formatCost(leaderboard.value?.total_actual_cost || 0) },
  { label: t('admin.tokenLeaderboard.lastUsedAt'), value: formatLastUsedAt(latestLastUsedAt.value) }
])

function formatCost(value: number): string {
  return formatCostFixed(value, value > 0 && value < 0.01 ? 6 : 4)
}

function formatSignedTokens(value: number): string {
  const sign = value > 0 ? '+' : value < 0 ? '-' : ''
  return `${sign}${formatNumber(Math.abs(value))}`
}

function formatMultiplier(value: number): string {
  const normalized = Number.isFinite(value) && value > 0 ? value : 1
  return `${formatAdaptiveMultiplier(normalized)}x`
}

function calibrationClass(value: number): string {
  if (value > 0) return 'text-emerald-700 dark:text-emerald-300'
  if (value < 0) return 'text-rose-700 dark:text-rose-300'
  return 'text-gray-700 dark:text-gray-300'
}

function formatLastUsedAt(value: string): string {
  return formatDateTime(value) || '-'
}

function displayUser(row: AdminTokenLeaderboardUser): string {
  return row.username?.trim() || row.email || t('admin.tokenLeaderboard.unknownUser')
}

function secondaryUserText(row: AdminTokenLeaderboardUser): string {
  if (showUserIds.value) {
    const userId = `${t('admin.tokenLeaderboard.userId')} ${row.user_id}`
    return row.email ? `${userId} · ${row.email}` : userId
  }

  return row.username?.trim() && row.email ? row.email : ''
}

function isTopTen(row: AdminTokenLeaderboardUser): boolean {
  return row.rank > 0 && row.rank <= 10
}

function isSelected(userId: number): boolean {
  return selectedUserIds.value.includes(userId)
}

function pruneSelectedUsers(): void {
  const eligible = new Set(topTenRows.value.map((row) => row.user_id))
  selectedUserIds.value = selectedUserIds.value.filter((userId) => eligible.has(userId))
}

function onRowSelectionChange(row: AdminTokenLeaderboardUser, event: Event): void {
  const checked = (event.target as HTMLInputElement).checked
  if (!isTopTen(row)) return
  if (checked) {
    if (!selectedUserIds.value.includes(row.user_id)) {
      selectedUserIds.value = [...selectedUserIds.value, row.user_id]
    }
    return
  }
  selectedUserIds.value = selectedUserIds.value.filter((userId) => userId !== row.user_id)
}

function selectCurrentTop10(): void {
  selectedUserIds.value = topTenRows.value.map((row) => row.user_id)
}

function clearSelectedUsers(): void {
  selectedUserIds.value = []
}

function clearDetailsCache(): void {
  detailsCacheVersion.value += 1
  Object.keys(detailsCache).forEach((key) => {
    delete detailsCache[Number(key)]
  })
  Object.keys(detailLoading).forEach((key) => {
    detailLoading[Number(key)] = false
  })
}

function statusLabel(status: string): string {
  if (status === 'active') return t('admin.tokenLeaderboard.active')
  if (status === 'disabled') return t('admin.tokenLeaderboard.disabled')
  return status || '-'
}

function statusClass(status: string): string {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'disabled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
}

function buildParams(): AdminTokenLeaderboardParams {
  return {
    start_date: startDate.value,
    end_date: endDate.value,
    email: emailSearch.value || undefined,
    group_id: groupId.value || undefined,
    model: modelSearch.value || undefined,
    model_source: modelSource.value,
    user_status: userStatus.value || undefined,
    limit: limit.value
  }
}

async function loadGroups(): Promise<void> {
  try {
    groups.value = await adminAPI.groups.getAllIncludingInactive()
  } catch (error) {
    console.error('Failed to load groups:', error)
    groups.value = []
  }
}

async function loadSettings(): Promise<void> {
  settingsLoading.value = true
  try {
    const settings = await adminAPI.settings.getSettings()
    const groupId = settings.token_leaderboard_common_group_id > 0 ? settings.token_leaderboard_common_group_id : null
    commonGroupId.value = groupId
    savedCommonGroupId.value = groupId
  } catch (error) {
    console.error('Failed to load token leaderboard settings:', error)
    commonGroupId.value = null
    savedCommonGroupId.value = null
  } finally {
    settingsLoading.value = false
  }
}

async function saveCommonGroup(): Promise<void> {
  if (!canSaveCommonGroup.value) return
  settingsSaving.value = true
  try {
    const groupId = commonGroupId.value ?? 0
    const settings = await adminAPI.settings.updateSettings({
      token_leaderboard_common_group_id: groupId
    })
    const savedGroupId = settings.token_leaderboard_common_group_id > 0 ? settings.token_leaderboard_common_group_id : null
    commonGroupId.value = savedGroupId
    savedCommonGroupId.value = savedGroupId
    appStore.showSuccess(t('admin.tokenLeaderboard.commonGroupSaved'))
    await loadLeaderboard()
  } catch (error: any) {
    console.error('Failed to save token leaderboard common group:', error)
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.tokenLeaderboard.commonGroupSaveFailed'))
  } finally {
    settingsSaving.value = false
  }
}

async function loadLeaderboard(): Promise<void> {
  loading.value = true
  expandedUserId.value = null
  clearDetailsCache()
  try {
    leaderboard.value = await adminAPI.dashboard.getAdminTokenLeaderboard(buildParams())
    pruneSelectedUsers()
  } catch (error) {
    console.error('Failed to load admin token leaderboard:', error)
    leaderboard.value = null
    clearSelectedUsers()
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  void loadLeaderboard()
}

function resetFilters(): void {
  startDate.value = todayString()
  endDate.value = todayString()
  emailSearch.value = ''
  modelSearch.value = ''
  modelSource.value = 'requested'
  groupId.value = null
  userStatus.value = ''
  limit.value = 10
  clearSelectedUsers()
  void loadLeaderboard()
}

function openGrantDialog(): void {
  if (selectedRows.value.length === 0) {
    appStore.showError(t('admin.tokenLeaderboard.selectUsersRequired'))
    return
  }
  if (!grantAmount.value || grantAmount.value <= 0) {
    appStore.showError(t('admin.tokenLeaderboard.amountRequired'))
    return
  }
  grantDialogOpen.value = true
}

async function confirmGrantBalance(): Promise<void> {
  if (!canOpenGrantDialog.value) return

  granting.value = true
  try {
    const result = await adminAPI.dashboard.grantAdminTokenLeaderboardBalance(
      { ...buildParams(), limit: 10 },
      {
        user_ids: selectedRows.value.map((row) => row.user_id),
        amount: grantAmount.value,
        notes: grantNotes.value || undefined
      }
    )
    appStore.showSuccess(t('admin.tokenLeaderboard.grantSuccess', {
      count: result.granted_count,
      amount: formatCost(result.amount)
    }))
    grantDialogOpen.value = false
    clearSelectedUsers()
    grantAmount.value = 0
    grantNotes.value = ''
    await loadLeaderboard()
  } catch (error: any) {
    console.error('Failed to grant token leaderboard balance:', error)
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.tokenLeaderboard.grantFailed'))
  } finally {
    granting.value = false
  }
}

function onDateRangeChange(): void {
  void loadLeaderboard()
}

async function toggleDetails(userId: number): Promise<void> {
  if (expandedUserId.value === userId) {
    expandedUserId.value = null
    return
  }
  expandedUserId.value = userId
  if (detailsCache[userId]) return

  const requestVersion = detailsCacheVersion.value
  detailLoading[userId] = true
  try {
    const details = await adminAPI.dashboard.getAdminTokenLeaderboardUserDetails(userId, buildParams())
    if (requestVersion === detailsCacheVersion.value) {
      detailsCache[userId] = details
    }
  } catch (error) {
    console.error('Failed to load admin token leaderboard details:', error)
    if (requestVersion === detailsCacheVersion.value) {
      detailsCache[userId] = undefined
    }
  } finally {
    if (requestVersion === detailsCacheVersion.value) {
      detailLoading[userId] = false
    }
  }
}

onMounted(() => {
  void loadGroups()
  void loadSettings()
  void loadLeaderboard()
})
</script>
