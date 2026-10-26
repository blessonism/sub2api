<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-violet-50 text-violet-500 dark:bg-violet-900/30 dark:text-violet-400">
            <Icon name="sparkles" size="sm" />
          </span>
          {{ t('admin.modelIqMonitor.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.modelIqMonitor.description') }}
        </p>
        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="tabs inline-flex w-full max-w-xl flex-wrap sm:w-auto" role="tablist">
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="activeTab === 'results' ? 'tab-active' : ''"
              :aria-selected="activeTab === 'results'"
              @click="switchTab('results')"
            >
              {{ t('admin.modelIqMonitor.tabResults') }}
            </button>
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="activeTab === 'config' ? 'tab-active' : ''"
              :aria-selected="activeTab === 'config'"
              @click="switchTab('config')"
            >
              {{ t('admin.modelIqMonitor.tabConfig') }}
            </button>
          </div>
        </div>
      </header>

      <!-- 探测结果 -->
      <TablePageLayout v-if="activeTab === 'results'">
        <template #filters>
          <div class="flex flex-wrap items-center gap-2">
            <input
              v-model.trim="resultFilters.model"
              type="text"
              class="input h-9 w-52"
              :placeholder="t('admin.modelIqMonitor.filterModelPlaceholder')"
              @input="debouncedReloadResults"
            />
            <Select
              v-model="resultFilters.status"
              :options="statusFilterOptions"
              class="w-40"
              :aria-label="t('admin.modelIqMonitor.filterStatus')"
            />
            <input
              v-model.trim="resultFilters.account_id"
              type="text"
              inputmode="numeric"
              class="input h-9 w-40"
              :placeholder="t('admin.modelIqMonitor.filterAccountPlaceholder')"
              @input="debouncedReloadResults"
            />
            <div class="ml-auto flex items-center gap-2">
              <label class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                <Toggle v-model="autoRefreshEnabled" />
                {{ t('admin.modelIqMonitor.autoRefresh') }}
              </label>
              <button type="button" class="btn btn-primary h-9" :disabled="running || !savedEnabled" @click="handleRunNow">
                <Icon name="play" size="sm" class="mr-1" />
                {{ running ? t('admin.modelIqMonitor.running') : t('admin.modelIqMonitor.runNow') }}
              </button>
            </div>
          </div>
          <p v-if="runState?.status === 'running'" class="mt-3 text-sm text-gray-600 dark:text-gray-300" role="status">
            {{ t('admin.modelIqMonitor.runProgress', { done: runState.summary.pass + runState.summary.wrong_answer + runState.summary.error, total: runState.summary.total }) }}
          </p>
          <p v-else-if="runState?.status === 'failed'" class="mt-3 text-sm text-red-600 dark:text-red-400" role="status">
            {{ t('admin.modelIqMonitor.runFailed') }}: {{ runState.error_message }}
          </p>
        </template>

        <template #table>
          <DataTable :columns="resultColumns" :data="results" :loading="resultsLoading">
            <template #cell-created_at="{ row }">
              <span class="text-sm text-gray-600 dark:text-gray-300">{{ formatDateTime(row.created_at) }}</span>
            </template>

            <template #cell-account_name="{ row }">
              <span class="text-sm text-gray-900 dark:text-gray-100">{{ row.account_name || '-' }}</span>
            </template>

            <template #cell-model="{ row }">
              <span class="font-mono text-sm text-gray-900 dark:text-gray-100">{{ row.model }}</span>
            </template>

            <template #cell-status="{ row }">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
                :class="statusBadgeClass(row.effective_status)"
              >
                {{ statusText(row.effective_status) }}
              </span>
              <span
                v-if="row.override_status"
                class="ml-1 inline-flex items-center rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-600 dark:bg-blue-900/30 dark:text-blue-300"
              >
                {{ t('admin.modelIqMonitor.overridden') }}
              </span>
            </template>

            <template #cell-actual_answer="{ row }">
              <button
                type="button"
                class="max-w-[18rem] truncate text-left text-sm text-gray-600 hover:text-primary-600 dark:text-gray-300 dark:hover:text-primary-300"
                :title="t('admin.modelIqMonitor.viewAnswer')"
                @click="openAnswerDialog(row)"
              >
                {{ row.actual_answer || row.error_message || '-' }}
              </button>
            </template>

            <template #cell-latency_ms="{ row }">
              <span class="text-sm tabular-nums text-gray-600 dark:text-gray-300">{{ formatLatency(row.latency_ms) }}</span>
            </template>

            <template #cell-tokens="{ row }">
              <span class="text-sm tabular-nums text-gray-600 dark:text-gray-300">{{ formatTokens(row) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <button
                type="button"
                class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                @click="openOverrideDialog(row)"
              >
                {{ t('admin.modelIqMonitor.overrideAction') }}
              </button>
            </template>

            <template #empty>
              <EmptyState
                :title="t('admin.modelIqMonitor.noResults')"
                :description="t('admin.modelIqMonitor.noResultsHint')"
              />
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="resultPagination.total > 0"
            :page="resultPagination.page"
            :total="resultPagination.total"
            :page-size="resultPagination.page_size"
            @update:page="onResultPageChange"
            @update:pageSize="onResultPageSizeChange"
          />
        </template>
      </TablePageLayout>

      <!-- 探测配置 -->
      <div v-else class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div v-if="configLoading" class="space-y-4">
          <div v-for="i in 4" :key="i" class="h-10 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-900/60"></div>
        </div>

        <form v-else class="space-y-5" @submit.prevent="saveConfig">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('admin.modelIqMonitor.enabledLabel') }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.enabledHint') }}</p>
            </div>
            <Toggle v-model="configForm.enabled" />
          </div>

          <div class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div>
              <label class="input-label">{{ t('admin.modelIqMonitor.visibilityLabel') }}</label>
              <Select v-model="configForm.visibility" :options="visibilityOptions" class="mt-1 w-72" :aria-label="t('admin.modelIqMonitor.visibilityLabel')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.modelIqMonitor.displayModeLabel') }}</label>
              <Select v-model="configForm.user_display_mode" :options="displayModeOptions" class="mt-1 w-72" :aria-label="t('admin.modelIqMonitor.displayModeLabel')" />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.displayModeHint') }}</p>
            </div>
            <div class="flex items-center justify-between gap-4">
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('admin.modelIqMonitor.showExclusionLabel') }}</p>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.showExclusionHint') }}</p>
              </div>
              <Toggle v-model="configForm.show_exclusion" />
            </div>
          </div>

          <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
            <label class="input-label">{{ t('admin.modelIqMonitor.targetModeLabel') }}</label>
            <Select
              v-model="configForm.target_mode"
              :options="targetModeOptions"
              class="mt-1 w-72"
              :aria-label="t('admin.modelIqMonitor.targetModeLabel')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.targetModeHint') }}</p>
          </div>

          <div v-if="configForm.target_mode === 'account_model'" class="border-t border-gray-100 pt-4 dark:border-dark-700">
            <label class="input-label">{{ t('admin.modelIqMonitor.accountsLabel') }}</label>
            <AccountMultiSelector
              v-model="configForm.account_ids"
              :options="accountOptions"
              :loading="accountOptionsLoading"
              :placeholder="t('admin.modelIqMonitor.accountsPlaceholder')"
              :search-placeholder="t('admin.modelIqMonitor.accountsSearch')"
              :empty-text="t('admin.modelIqMonitor.accountsEmpty')"
              :loading-text="t('admin.modelIqMonitor.accountsLoading')"
              :inactive-label="t('admin.modelIqMonitor.accountsInactive')"
              :selected-label="t('admin.modelIqMonitor.accountsSelected', { count: configForm.account_ids.length })"
              :count-label="t('admin.modelIqMonitor.accountsCount', { count: accountOptions.length })"
              :aria-label="t('admin.modelIqMonitor.accountsLabel')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.accountsHint') }}</p>
          </div>

          <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
            <label class="input-label">{{ t('admin.modelIqMonitor.modelsLabel') }}</label>
            <ModelTagInput
              :models="configForm.models"
              :placeholder="t('admin.modelIqMonitor.modelsPlaceholder')"
              @update:models="configForm.models = $event"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.modelsHint') }}</p>
          </div>

          <div class="grid gap-4 border-t border-gray-100 pt-4 dark:border-dark-700 sm:grid-cols-2">
            <div>
              <label class="input-label" for="iq-interval">{{ t('admin.modelIqMonitor.intervalLabel') }}</label>
              <input
                id="iq-interval"
                v-model.number="configForm.interval_minutes"
                type="number"
                min="5"
                max="10080"
                class="input mt-1 w-full"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.intervalHint') }}</p>
            </div>
            <div>
              <label class="input-label" for="iq-expected">{{ t('admin.modelIqMonitor.expectedLabel') }}</label>
              <input id="iq-expected" v-model.trim="configForm.expected_answer" type="text" maxlength="200" class="input mt-1 w-full" />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.expectedHint') }}</p>
            </div>
          </div>

          <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
            <label class="input-label" for="iq-question">{{ t('admin.modelIqMonitor.questionLabel') }}</label>
            <textarea
              id="iq-question"
              v-model="configForm.question"
              rows="8"
              class="input mt-1 w-full font-mono text-xs leading-5"
            ></textarea>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelIqMonitor.questionHint') }}</p>
          </div>

          <div class="flex items-center gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
            <button type="submit" class="btn btn-primary" :disabled="savingConfig">
              {{ savingConfig ? t('common.saving') : t('common.save') }}
            </button>
            <button type="button" class="btn" :disabled="configLoading" @click="loadConfig">
              {{ t('common.cancel') }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 回答全文弹窗 -->
    <BaseDialog :show="showAnswerDialog" :title="t('admin.modelIqMonitor.answerDialogTitle')" width="wide" @close="closeAnswerDialog">
      <div v-if="answerRow" class="space-y-3 text-sm">
        <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span class="font-mono text-sm text-gray-900 dark:text-gray-100">{{ answerRow.model }}</span>
          <span>·</span>
          <span>{{ answerRow.account_name || '-' }}</span>
          <span>·</span>
          <span>{{ formatDateTime(answerRow.created_at) }}</span>
          <span
            class="ml-auto inline-flex items-center rounded-full px-2 py-0.5 font-semibold"
            :class="statusBadgeClass(answerRow.effective_status)"
          >
            {{ statusText(answerRow.effective_status) }}
          </span>
        </div>
        <div class="rounded-xl bg-gray-50 p-3 text-xs text-gray-500 dark:bg-dark-900/60 dark:text-gray-400">
          <p>{{ t('admin.modelIqMonitor.expectedAnswer') }}: <span class="font-semibold">{{ answerRow.expected_answer }}</span></p>
          <p v-if="answerRow.error_message" class="mt-1 text-red-500">{{ answerRow.error_message }}</p>
        </div>
        <pre class="max-h-96 overflow-auto whitespace-pre-wrap rounded-xl bg-gray-50 p-3 text-xs leading-5 text-gray-800 dark:bg-dark-900/60 dark:text-gray-200">{{ answerRow.actual_answer || t('admin.modelIqMonitor.noAnswerText') }}</pre>
      </div>
    </BaseDialog>

    <!-- 人工改判弹窗 -->
    <BaseDialog :show="showOverrideDialog" :title="t('admin.modelIqMonitor.overrideDialogTitle')" width="narrow" @close="closeOverrideDialog">
      <div v-if="overrideRow" class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ overrideRow.model }} · {{ formatDateTime(overrideRow.created_at) }} ·
          {{ t('admin.modelIqMonitor.autoStatus') }}: {{ statusText(overrideRow.status) }}
        </p>
        <div class="space-y-1.5">
          <label
            v-for="option in overrideOptions"
            :key="option.value"
            class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm"
            :class="overrideChoice === option.value ? 'border-primary-400 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-600'"
          >
            <input v-model="overrideChoice" type="radio" name="iq-override" :value="option.value" class="accent-primary-500" />
            <span class="text-gray-800 dark:text-gray-200">{{ option.label }}</span>
          </label>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn" @click="closeOverrideDialog">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="overriding" @click="confirmOverride">
            {{ t('common.confirm') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { adminAPI } from '@/api/admin'
import type {
  ModelIqProbeConfig,
  ModelIqProbeResult,
  ModelIqProbeStatus,
  ModelIqProbeRunState,
} from '@/api/admin/modelIqProbe'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import AccountMultiSelector from '@/components/admin/AccountMultiSelector.vue'
import type { AccountListItem } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelTagInput from '@/components/admin/channel/ModelTagInput.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

const activeTab = ref<'results' | 'config'>('results')
function switchTab(tab: 'results' | 'config') {
  activeTab.value = tab
  if (tab === 'results' && results.value.length === 0) void reloadResults()
}

// ---------- 配置 ----------

const configLoading = ref(false)
const savingConfig = ref(false)
const savedEnabled = ref(false)
const configForm = reactive<ModelIqProbeConfig>({
  enabled: false,
  visibility: 'admin_only',
  user_display_mode: 'real',
  show_exclusion: false,
  question: '',
  expected_answer: '21',
  interval_minutes: 360,
  target_mode: 'account_model',
  models: [],
  account_ids: [],
})

// 账号选项：分页拉全量后前端过滤出 OpenAI 平台账号，供多选下拉使用。
interface AccountOption {
  id: number
  name: string
  platform: string
  status: string
}
const accountOptions = ref<AccountOption[]>([])
const accountOptionsLoading = ref(false)

async function loadAccountOptions() {
  if (accountOptionsLoading.value) return
  accountOptionsLoading.value = true
  try {
    const collected = new Map<number, AccountListItem>()
    const pageSize = 100
    // 用响应里的 pages 驱动翻页：后端可能压小 page_size，按 items 数量判断会提前终止。
    const first = await adminAPI.accounts.list(1, pageSize)
    for (const item of first.items) collected.set(item.id, item)
    for (let page = 2; page <= first.pages; page++) {
      const res = await adminAPI.accounts.list(page, pageSize)
      for (const item of res.items) collected.set(item.id, item)
    }
    accountOptions.value = [...collected.values()]
      .filter((account) => account.platform === 'openai')
      .map((account) => ({ id: account.id, name: account.name, platform: account.platform, status: account.status }))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.loadAccountsFailed')))
  } finally {
    accountOptionsLoading.value = false
  }
}

const targetModeOptions = computed(() => [
  { label: t('admin.modelIqMonitor.targetModeAccountModel'), value: 'account_model' },
  { label: t('admin.modelIqMonitor.targetModeModelOnly'), value: 'model_only' },
])
const visibilityOptions = computed(() => [
  { label: t('admin.modelIqMonitor.visibilityAdmin'), value: 'admin_only' },
  { label: t('admin.modelIqMonitor.visibilityAll'), value: 'all_users' },
])
const displayModeOptions = computed(() => [
  { label: t('admin.modelIqMonitor.displayReal'), value: 'real' },
  { label: t('admin.modelIqMonitor.displayGreen'), value: 'always_green' },
])

async function loadConfig() {
  configLoading.value = true
  try {
    const cfg = await adminAPI.modelIqProbe.getConfig()
    savedEnabled.value = cfg.enabled
    Object.assign(configForm, {
      enabled: cfg.enabled,
      visibility: cfg.visibility || 'admin_only',
      user_display_mode: cfg.user_display_mode || 'real',
      show_exclusion: cfg.show_exclusion ?? false,
      question: cfg.question,
      expected_answer: cfg.expected_answer,
      interval_minutes: cfg.interval_minutes,
      target_mode: cfg.target_mode,
      models: cfg.models || [],
      account_ids: cfg.account_ids || [],
    })
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.loadConfigFailed')))
  } finally {
    configLoading.value = false
  }
}

async function saveConfig() {
  if (!configForm.question.trim()) {
    appStore.showError(t('admin.modelIqMonitor.questionRequired'))
    return
  }
  if (!configForm.expected_answer.trim()) {
    appStore.showError(t('admin.modelIqMonitor.expectedRequired'))
    return
  }
  savingConfig.value = true
  try {
    const saved = await adminAPI.modelIqProbe.updateConfig({
      ...configForm,
      question: configForm.question,
      expected_answer: configForm.expected_answer.trim(),
    })
    savedEnabled.value = saved.enabled
    appStore.showSuccess(t('admin.modelIqMonitor.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.saveFailed')))
  } finally {
    savingConfig.value = false
  }
}

// ---------- 探测结果 ----------

const results = ref<ModelIqProbeResult[]>([])
const resultsLoading = ref(false)
const running = ref(false)
const runState = ref<ModelIqProbeRunState | null>(null)
let runStatusTimer: ReturnType<typeof setTimeout> | null = null
let runStatusController: AbortController | null = null
let disposed = false
let recoveryRun: { previousId: string | null } | null = null
const autoRefreshEnabled = ref(false)
const resultPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const resultFilters = reactive({ model: '', status: '' as ModelIqProbeStatus | '', account_id: '' })

let abortController: AbortController | null = null
let filterTimeout: ReturnType<typeof setTimeout> | null = null
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

const statusFilterOptions = computed(() => [
  { label: t('admin.modelIqMonitor.filterAllStatus'), value: '' },
  { label: t('admin.modelIqMonitor.status.pass'), value: 'pass' },
  { label: t('admin.modelIqMonitor.status.wrong_answer'), value: 'wrong_answer' },
  { label: t('admin.modelIqMonitor.status.error'), value: 'error' },
])

const resultColumns = computed<Column[]>(() => [
  { key: 'created_at', label: t('admin.modelIqMonitor.columns.time'), sortable: false },
  { key: 'account_name', label: t('admin.modelIqMonitor.columns.account'), sortable: false },
  { key: 'model', label: t('admin.modelIqMonitor.columns.model'), sortable: false },
  { key: 'status', label: t('admin.modelIqMonitor.columns.status'), sortable: false },
  { key: 'actual_answer', label: t('admin.modelIqMonitor.columns.answer'), sortable: false },
  { key: 'latency_ms', label: t('admin.modelIqMonitor.columns.latency'), sortable: false },
  { key: 'tokens', label: t('admin.modelIqMonitor.columns.tokens'), sortable: false },
  { key: 'actions', label: t('admin.modelIqMonitor.columns.actions'), sortable: false },
])

async function reloadResults() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  resultsLoading.value = true
  try {
    const params: Parameters<typeof adminAPI.modelIqProbe.listResults>[0] = {
      page: resultPagination.page,
      page_size: resultPagination.page_size,
    }
    if (resultFilters.model) params.model = resultFilters.model
    if (resultFilters.status) params.status = resultFilters.status
    const accountId = Number.parseInt(resultFilters.account_id, 10)
    if (Number.isFinite(accountId) && accountId > 0) params.account_id = accountId

    const res = await adminAPI.modelIqProbe.listResults(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    results.value = res.items || []
    resultPagination.total = res.total
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.loadResultsFailed')))
  } finally {
    if (abortController === ctrl) {
      resultsLoading.value = false
      abortController = null
    }
  }
}

function debouncedReloadResults() {
  if (filterTimeout) clearTimeout(filterTimeout)
  filterTimeout = setTimeout(() => {
    resultPagination.page = 1
    void reloadResults()
  }, 300)
}

function onResultPageChange(page: number) {
  resultPagination.page = page
  void reloadResults()
}

function onResultPageSizeChange(size: number) {
  resultPagination.page_size = size
  resultPagination.page = 1
  void reloadResults()
}

async function handleRunNow() {
  if (running.value) return
  const previousRunId = runState.value?.id ?? null
  recoveryRun = null
  runStatusController?.abort()
  if (runStatusTimer) clearTimeout(runStatusTimer)
  running.value = true
  try {
    const state = await adminAPI.modelIqProbe.runProbe()
    if (disposed) return
    runState.value = state
    appStore.showSuccess(t('admin.modelIqMonitor.runStarted'))
    void reloadRunState()
  } catch (err: unknown) {
    if (disposed) return
    // POST 响应丢失或 409 均不能证明后台没有运行；先恢复共享任务状态。
    recoveryRun = { previousId: previousRunId }
    const state = await reloadRunState()
    if (!disposed && state && state.status !== 'running' && (previousRunId === null || !state.id || state.id === previousRunId)) {
      appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.runFailed')))
    }
  }
}

async function reloadRunState(): Promise<ModelIqProbeRunState | null> {
  runStatusController?.abort()
  const ctrl = new AbortController()
  runStatusController = ctrl
  let shouldPoll = running.value || recoveryRun !== null
  try {
    const state = await adminAPI.modelIqProbe.getRunState({ signal: ctrl.signal })
    if (disposed || ctrl.signal.aborted || runStatusController !== ctrl) return null
    const wasRunning = running.value
    const recovery = recoveryRun
    recoveryRun = null
    runState.value = state
    running.value = state.status === 'running'
    shouldPoll = running.value
    const notifyCompletion = recovery === null
      ? wasRunning
      : recovery.previousId !== null && Boolean(state.id) && state.id !== recovery.previousId
    if (!running.value && notifyCompletion) {
      if (state.status === 'failed') {
        appStore.showError(state.error_message || t('admin.modelIqMonitor.runFailed'))
      } else if (state.status === 'completed') {
        const summary = state.summary
        appStore.showSuccess(t('admin.modelIqMonitor.runSummary', {
          total: summary.total, pass: summary.pass, wrong: summary.wrong_answer, error: summary.error,
        }))
      }
      resultPagination.page = 1
      await reloadResults()
    }
    return state
  } catch (err: unknown) {
    const e = err as { code?: string }
    if (!disposed && !ctrl.signal.aborted && e.code !== 'ERR_CANCELED') {
      appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.loadRunFailed')))
    }
  } finally {
    if (!disposed && runStatusController === ctrl && shouldPoll) {
      runStatusTimer = setTimeout(() => { void reloadRunState() }, 2000)
    }
  }
  return null
}

// 回答全文弹窗
const showAnswerDialog = ref(false)
const answerRow = ref<ModelIqProbeResult | null>(null)
function openAnswerDialog(row: ModelIqProbeResult) {
  answerRow.value = row
  showAnswerDialog.value = true
}
function closeAnswerDialog() {
  showAnswerDialog.value = false
  answerRow.value = null
}

// 人工改判弹窗
const showOverrideDialog = ref(false)
const overrideRow = ref<ModelIqProbeResult | null>(null)
const overrideChoice = ref<ModelIqProbeStatus | ''>('')
const overriding = ref(false)
const overrideOptions = computed(() => [
  { label: t('admin.modelIqMonitor.status.pass'), value: 'pass' as const },
  { label: t('admin.modelIqMonitor.status.wrong_answer'), value: 'wrong_answer' as const },
  { label: t('admin.modelIqMonitor.status.error'), value: 'error' as const },
  { label: t('admin.modelIqMonitor.overrideClear'), value: '' as const },
])

function openOverrideDialog(row: ModelIqProbeResult) {
  overrideRow.value = row
  overrideChoice.value = row.override_status ?? ''
  showOverrideDialog.value = true
}
function closeOverrideDialog() {
  showOverrideDialog.value = false
  overrideRow.value = null
}

async function confirmOverride() {
  if (!overrideRow.value) return
  overriding.value = true
  try {
    await adminAPI.modelIqProbe.overrideResult(overrideRow.value.id, overrideChoice.value === '' ? null : overrideChoice.value)
    appStore.showSuccess(t('admin.modelIqMonitor.overrideSuccess'))
    showOverrideDialog.value = false
    overrideRow.value = null
    await reloadResults()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelIqMonitor.overrideFailed')))
  } finally {
    overriding.value = false
  }
}

// ---------- 展示工具 ----------

function statusText(status: ModelIqProbeStatus): string {
  return t(`admin.modelIqMonitor.status.${status}`)
}

function statusBadgeClass(status: ModelIqProbeStatus): string {
  switch (status) {
    case 'pass':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'wrong_answer':
      return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
    case 'error':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-900/70 dark:text-gray-300'
  }
}

function formatLatency(ms: number): string {
  if (!ms) return '-'
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`
}

function formatTokens(row: ModelIqProbeResult): string {
  if (row.output_tokens == null) return '-'
  const parts = [`${row.output_tokens} out`]
  if (row.input_tokens != null) parts.unshift(`${row.input_tokens} in`)
  return parts.join(' / ')
}

watch(autoRefreshEnabled, (enabled) => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
  if (enabled) {
    autoRefreshTimer = setInterval(() => {
      if (!document.hidden) void reloadResults()
    }, 30_000)
  }
})

watch(
  () => resultFilters.status,
  () => {
    resultPagination.page = 1
    void reloadResults()
  },
)

onMounted(() => {
  void reloadResults()
  void loadConfig()
  void loadAccountOptions()
  void reloadRunState()
})

onUnmounted(() => {
  disposed = true
  if (runStatusTimer) clearTimeout(runStatusTimer)
  runStatusController?.abort()
  if (filterTimeout) clearTimeout(filterTimeout)
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  abortController?.abort()
})
</script>
