<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap justify-end gap-2">
        <button class="btn btn-secondary inline-flex items-center gap-2" type="button" @click="loadTasks">
          <Icon name="refresh" size="sm" />
          {{ t('common.refresh') }}
        </button>
        <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateDialog">
          <Icon name="plus" size="sm" />
          {{ t('admin.upstreamCostCalibrations.create') }}
        </button>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex min-h-64 items-center justify-center">
          <LoadingSpinner />
        </div>
        <EmptyState
          v-else-if="tasks.length === 0"
          class="min-h-64"
          :title="t('admin.upstreamCostCalibrations.emptyTitle')"
          :description="t('admin.upstreamCostCalibrations.emptyDescription')"
          :action-text="t('admin.upstreamCostCalibrations.create')"
          @action="openCreateDialog"
        />
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1120px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.task') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.targetGroup') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.model') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.upstreamCostCalibrations.accounts') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.status') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.lastRun') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.latestResult') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="task in tasks" :key="task.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">{{ task.name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.upstreamCostCalibrations.priorityRule', { start: task.priority_start, step: task.priority_step }) }}
                  </div>
                </td>
                <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                  {{ task.target_group_name || groupName(task.target_group_id) }}
                </td>
                <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                  <div>{{ task.model }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ task.unit }}</div>
                </td>
                <td class="px-4 py-3 text-right tabular-nums">{{ task.accounts?.length || 0 }}</td>
                <td class="px-4 py-3">
                  <span :class="task.enabled ? statusPillClass('enabled') : statusPillClass('disabled')" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ task.enabled ? t('common.enabled') : t('common.disabled') }}
                  </span>
                  <span v-if="task.latest_run" :class="runStatusClass(task.latest_run.status)" class="ml-2 inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ runStatusLabel(task.latest_run.status) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(task.last_run_at) }}</td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-300">
                  <span v-if="task.latest_run">
                    {{ t('admin.upstreamCostCalibrations.latestResultSummary', {
                      valid: task.latest_run.valid_accounts,
                      invalid: task.latest_run.invalid_accounts,
                      suggestions: task.latest_run.suggestion_count
                    }) }}
                  </span>
                  <span v-else>-</span>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1" type="button" @click="openRuns(task)">
                      {{ t('admin.upstreamCostCalibrations.history') }}
                    </button>
                    <button class="btn btn-primary px-2 py-1" type="button" :disabled="runningTaskId === task.id" @click="runTask(task)">
                      {{ runningTaskId === task.id ? t('admin.upstreamCostCalibrations.running') : t('admin.upstreamCostCalibrations.runNow') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1" type="button" @click="openEditDialog(task)">
                      {{ t('common.edit') }}
                    </button>
                    <button class="btn btn-danger px-2 py-1" type="button" @click="deleteTask(task)">
                      {{ t('common.delete') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="onPageChange"
        @update:pageSize="onPageSizeChange"
      />
    </div>
  </AppLayout>

  <BaseDialog :show="formDialogOpen" :title="formMode === 'create' ? t('admin.upstreamCostCalibrations.create') : t('admin.upstreamCostCalibrations.edit')" width="full" @close="formDialogOpen = false">
    <form class="space-y-6" @submit.prevent="submitForm">
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.name') }}</span>
            <input v-model.trim="form.name" class="input w-full" type="text" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.targetGroup') }}</span>
            <Select v-model="form.target_group_id" :options="groupOptions" searchable @change="onFormGroupChange" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.model') }}</span>
            <input v-model.trim="form.model" class="input w-full" type="text" :placeholder="t('admin.upstreamCostCalibrations.modelPlaceholder')" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.unit') }}</span>
            <input v-model.trim="form.unit" class="input w-full" type="text" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.sampleCount') }}</span>
            <input v-model.number="form.sample_count" class="input w-full" type="number" min="1" max="10" step="1" @change="syncFormSampleCounts" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.adapterType') }}</span>
            <Select v-model="form.adapter_type" :options="adapterOptions" disabled />
          </label>
        </div>

        <label class="mt-4 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          {{ t('admin.upstreamCostCalibrations.enabled') }}
        </label>

        <label class="mt-4 block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.testPrompt') }}</span>
          <textarea v-model.trim="form.test_prompt" class="input min-h-[92px] w-full" />
        </label>
      </section>

      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamCostCalibrations.prioritySettings') }}</h3>
        <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.priorityStart') }}</span>
            <input v-model.number="form.priority_start" class="input w-full" type="number" min="1" step="1" />
          </label>
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.priorityStep') }}</span>
            <input v-model.number="form.priority_step" class="input w-full" type="number" min="1" step="1" />
          </label>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamCostCalibrations.accountSamples') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.accountSamplesHint') }}</p>
          </div>
          <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" :disabled="accountsLoading || !form.target_group_id" @click="loadCandidateAccounts">
            {{ accountsLoading ? t('common.loading') : t('admin.upstreamCostCalibrations.loadGroupAccounts') }}
          </button>
        </div>

        <div v-if="form.accounts.length === 0" class="mt-4 rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800 dark:text-gray-400">
          {{ t('admin.upstreamCostCalibrations.noAccountsSelected') }}
        </div>
        <div v-else class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[960px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.account') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.currentPriority') }}</th>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.samples') }}</th>
                <th class="px-3 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="account in form.accounts" :key="account.account_id">
                <td class="px-3 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">{{ account.account_name || `#${account.account_id}` }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    ID {{ account.account_id }} · {{ account.platform || '-' }}
                  </div>
                </td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatNumber(account.current_priority) }}</td>
                <td class="px-3 py-3">
                  <div class="space-y-2">
                    <div
                      v-for="(sample, sampleIndex) in account.samples"
                      :key="sampleIndex"
                      class="grid grid-cols-1 gap-2 rounded-md bg-gray-50 p-2 dark:bg-dark-800 md:grid-cols-[64px_1fr_1fr_1fr]"
                    >
                      <div class="flex items-center text-xs font-medium text-gray-500 dark:text-gray-400">
                        {{ t('admin.upstreamCostCalibrations.sampleIndex', { index: sampleIndex + 1 }) }}
                      </div>
                      <label class="space-y-1">
                        <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.beforeBalance') }}</span>
                        <input v-model.number="sample.before_balance" class="input w-full text-right" type="number" min="0" step="0.000001" />
                      </label>
                      <label class="space-y-1">
                        <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.afterBalance') }}</span>
                        <input v-model.number="sample.after_balance" class="input w-full text-right" type="number" min="0" step="0.000001" />
                      </label>
                      <label class="space-y-1">
                        <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreamCostCalibrations.latencyMs') }}</span>
                        <input v-model.number="sample.latency_ms" class="input w-full text-right" type="number" min="0" step="1" />
                      </label>
                    </div>
                  </div>
                </td>
                <td class="px-3 py-3 text-right">
                  <button class="btn btn-danger px-2 py-1" type="button" @click="removeFormAccount(account.account_id)">
                    {{ t('common.delete') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" @click="formDialogOpen = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" type="button" :disabled="saving" @click="submitForm">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog :show="runsDialogOpen" :title="t('admin.upstreamCostCalibrations.history')" width="full" @close="runsDialogOpen = false">
    <div v-if="runsLoading" class="flex min-h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="runs.length === 0" class="rounded-lg bg-gray-50 p-6 text-center text-sm text-gray-500 dark:bg-dark-800 dark:text-gray-400">
      {{ t('admin.upstreamCostCalibrations.noRuns') }}
    </div>
    <div v-else class="space-y-4">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[980px] text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.status') }}</th>
              <th class="px-4 py-3 text-right">{{ t('admin.upstreamCostCalibrations.totalAccounts') }}</th>
              <th class="px-4 py-3 text-right">{{ t('admin.upstreamCostCalibrations.validAccounts') }}</th>
              <th class="px-4 py-3 text-right">{{ t('admin.upstreamCostCalibrations.invalidAccounts') }}</th>
              <th class="px-4 py-3 text-right">{{ t('admin.upstreamCostCalibrations.suggestions') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.applied') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.upstreamCostCalibrations.startedAt') }}</th>
              <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="run in runs" :key="run.id" :class="selectedRun?.id === run.id ? 'bg-primary-50/60 dark:bg-primary-950/20' : ''">
              <td class="px-4 py-3">
                <span :class="runStatusClass(run.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                  {{ runStatusLabel(run.status) }}
                </span>
              </td>
              <td class="px-4 py-3 text-right tabular-nums">{{ run.total_accounts }}</td>
              <td class="px-4 py-3 text-right tabular-nums">{{ run.valid_accounts }}</td>
              <td class="px-4 py-3 text-right tabular-nums">{{ run.invalid_accounts }}</td>
              <td class="px-4 py-3 text-right tabular-nums">{{ run.suggestion_count }}</td>
              <td class="px-4 py-3">{{ run.applied ? t('common.yes') : t('common.no') }}</td>
              <td class="px-4 py-3">{{ formatDateTime(run.started_at) }}</td>
              <td class="px-4 py-3 text-right">
                <button class="btn btn-secondary px-2 py-1" type="button" @click="loadRunDetail(run.id)">
                  {{ t('admin.upstreamCostCalibrations.viewDetails') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="runDetailLoading" class="flex min-h-40 items-center justify-center rounded-lg border border-gray-200 dark:border-dark-700">
        <LoadingSpinner />
      </div>
      <section v-else-if="selectedRun" class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.upstreamCostCalibrations.runDetailTitle', { id: selectedRun.id }) }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ selectedRun.error_message || t('admin.upstreamCostCalibrations.runEvidenceHint') }}
            </p>
          </div>
          <button
            class="btn btn-primary px-3 py-1.5 text-sm"
            type="button"
            :disabled="!canApplySelectedRun || applyingRun"
            @click="applySelectedRun"
          >
            {{ applyingRun ? t('admin.upstreamCostCalibrations.applying') : t('admin.upstreamCostCalibrations.applySuggestions') }}
          </button>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[1080px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.rank') }}</th>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.account') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.beforeBalance') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.afterBalance') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.costDelta') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.latencyMs') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.currentPriority') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.suggestedPriority') }}</th>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.resultStatus') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="result in selectedRun.results || []" :key="result.account_id">
                <td class="px-3 py-3">{{ formatNumber(result.rank) }}</td>
                <td class="px-3 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">{{ result.account_name || `#${result.account_id}` }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">ID {{ result.account_id }} · {{ result.account_platform || '-' }}</div>
                </td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatDecimal(result.before_balance) }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatDecimal(result.after_balance) }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatDecimal(result.cost_delta) }} {{ result.unit }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatNumber(result.latency_ms) }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatNumber(result.current_priority) }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatNumber(result.suggested_priority) }}</td>
                <td class="px-3 py-3">
                  <span :class="result.valid ? statusPillClass('enabled') : statusPillClass('failed')" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ result.valid ? t('admin.upstreamCostCalibrations.valid') : t('admin.upstreamCostCalibrations.invalid') }}
                  </span>
                  <div v-if="result.error_message" class="mt-1 text-xs text-red-600 dark:text-red-300">{{ result.error_message }}</div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[760px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.account') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.oldPriority') }}</th>
                <th class="px-3 py-3 text-right">{{ t('admin.upstreamCostCalibrations.newPriority') }}</th>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.reason') }}</th>
                <th class="px-3 py-3 text-left">{{ t('admin.upstreamCostCalibrations.applied') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="suggestion in selectedRun.suggestions || []" :key="suggestion.account_id">
                <td class="px-3 py-3">#{{ suggestion.account_id }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ formatNumber(suggestion.old_priority) }}</td>
                <td class="px-3 py-3 text-right tabular-nums">{{ suggestion.new_priority }}</td>
                <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ suggestion.reason }}</td>
                <td class="px-3 py-3">{{ suggestion.applied ? t('common.yes') : t('common.no') }}</td>
              </tr>
              <tr v-if="!selectedRun.suggestions?.length">
                <td colspan="5" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.upstreamCostCalibrations.noSuggestions') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { accountsAPI, groupsAPI } from '@/api/admin'
import upstreamCostCalibrationsAPI, {
  type UpstreamCostCalibrationAdapterType,
  type UpstreamCostCalibrationRun,
  type UpstreamCostCalibrationRunStatus,
  type UpstreamCostCalibrationTask,
  type UpstreamCostCalibrationTaskInput
} from '@/api/admin/upstreamCostCalibrations'
import { useAppStore } from '@/stores/app'
import type { Account, AdminGroup } from '@/types'

type CalibrationAccountForm = {
  account_id: number
  account_name?: string
  platform?: string
  current_priority?: number | null
  samples: CalibrationSampleForm[]
}

type CalibrationSampleForm = {
  before_balance: number | null
  after_balance: number | null
  latency_ms: number | null
}

type CalibrationForm = {
  name: string
  enabled: boolean
  target_group_id: number | null
  model: string
  adapter_type: UpstreamCostCalibrationAdapterType
  unit: string
  test_prompt: string
  sample_count: number
  priority_start: number
  priority_step: number
  accounts: CalibrationAccountForm[]
}

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const accountsLoading = ref(false)
const tasks = ref<UpstreamCostCalibrationTask[]>([])
const groups = ref<AdminGroup[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const formDialogOpen = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const editingTaskId = ref<number | null>(null)
const runningTaskId = ref<number | null>(null)
const runsDialogOpen = ref(false)
const runsLoading = ref(false)
const runs = ref<UpstreamCostCalibrationRun[]>([])
const selectedRunsTaskId = ref<number | null>(null)
const selectedRun = ref<UpstreamCostCalibrationRun | null>(null)
const runDetailLoading = ref(false)
const applyingRun = ref(false)

const form = reactive<CalibrationForm>(defaultForm())

const groupOptions = computed(() => groups.value.map((group) => ({ value: group.id, label: group.name })))
const adapterOptions = computed(() => [
  { value: 'manual', label: t('admin.upstreamCostCalibrations.adapters.manual') }
])
const canApplySelectedRun = computed(() => {
  const run = selectedRun.value
  return !!run && run.status === 'success' && !run.applied && run.suggestion_count > 0
})

onMounted(async () => {
  await Promise.all([loadGroups(), loadTasks()])
})

function defaultForm(): CalibrationForm {
  return {
    name: '',
    enabled: true,
    target_group_id: null,
    model: '',
    adapter_type: 'manual',
    unit: 'credit',
    test_prompt: 'ping',
    sample_count: 1,
    priority_start: 10,
    priority_step: 10,
    accounts: []
  }
}

function resetForm(task?: UpstreamCostCalibrationTask) {
  const next = task
    ? {
        name: task.name,
        enabled: task.enabled,
        target_group_id: task.target_group_id,
        model: task.model,
        adapter_type: task.adapter_type,
        unit: task.unit,
        test_prompt: task.test_prompt,
        sample_count: task.sample_count,
        priority_start: task.priority_start,
        priority_step: task.priority_step,
        accounts: (task.accounts || []).map((account) => ({
          account_id: account.account_id,
          account_name: account.account_name,
          platform: account.platform,
          current_priority: account.current_priority ?? null,
          samples: samplesFromConfig(account.adapter_config, task.sample_count)
        }))
      }
    : defaultForm()
  Object.assign(form, next)
}

async function loadGroups() {
  groups.value = await groupsAPI.getAllIncludingInactive()
}

async function loadTasks() {
  loading.value = true
  try {
    const result = await upstreamCostCalibrationsAPI.list({ page: pagination.page, page_size: pagination.page_size })
    tasks.value = result.items || []
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  formMode.value = 'create'
  editingTaskId.value = null
  resetForm()
  formDialogOpen.value = true
}

function openEditDialog(task: UpstreamCostCalibrationTask) {
  formMode.value = 'edit'
  editingTaskId.value = task.id
  resetForm(task)
  formDialogOpen.value = true
}

async function onFormGroupChange() {
  if (!form.target_group_id) {
    form.accounts = []
    return
  }
  await loadCandidateAccounts()
}

async function loadCandidateAccounts() {
  if (!form.target_group_id) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.groupRequired'))
    return
  }
  const group = groups.value.find((item) => item.id === form.target_group_id)
  accountsLoading.value = true
  try {
    const result = await accountsAPI.list(1, 200, {
      group: String(form.target_group_id),
      platform: group?.platform,
      status: 'active',
      sort_by: 'priority',
      sort_order: 'asc'
    })
    mergeCandidateAccounts(result.items || [])
    if ((result.items || []).length === 0) {
      appStore.showWarning(t('admin.upstreamCostCalibrations.noGroupAccounts'))
    }
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.loadAccountsFailed'))
  } finally {
    accountsLoading.value = false
  }
}

function mergeCandidateAccounts(accounts: Account[]) {
  const existingByID = new Map(form.accounts.map((account) => [account.account_id, account]))
  form.accounts = accounts.map((account) => {
    const existing = existingByID.get(account.id)
    return {
      account_id: account.id,
      account_name: account.name,
      platform: account.platform,
      current_priority: existing?.current_priority ?? null,
      samples: normalizedSamples(existing?.samples || [], Number(form.sample_count))
    }
  })
}

function removeFormAccount(accountID: number) {
  form.accounts = form.accounts.filter((account) => account.account_id !== accountID)
}

async function submitForm() {
  const payload = buildPayload()
  if (!payload) return
  saving.value = true
  try {
    if (formMode.value === 'create') {
      await upstreamCostCalibrationsAPI.create(payload)
      appStore.showSuccess(t('admin.upstreamCostCalibrations.created'))
    } else if (editingTaskId.value) {
      await upstreamCostCalibrationsAPI.update(editingTaskId.value, payload)
      appStore.showSuccess(t('admin.upstreamCostCalibrations.updated'))
    }
    formDialogOpen.value = false
    await loadTasks()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.saveFailed'))
  } finally {
    saving.value = false
  }
}

function buildPayload(): UpstreamCostCalibrationTaskInput | null {
  if (!form.name.trim()) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.nameRequired'))
    return null
  }
  if (!form.target_group_id) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.groupRequired'))
    return null
  }
  if (!form.model.trim()) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.modelRequired'))
    return null
  }
  if (form.accounts.length === 0) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.accountsRequired'))
    return null
  }
  const sampleCount = Number(form.sample_count)
  const priorityStart = Number(form.priority_start)
  const priorityStep = Number(form.priority_step)
  if (!Number.isInteger(sampleCount) || sampleCount < 1 || sampleCount > 10) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.sampleCountInvalid'))
    return null
  }
  if (!Number.isInteger(priorityStart) || priorityStart <= 0 || !Number.isInteger(priorityStep) || priorityStep <= 0) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.priorityInvalid'))
    return null
  }
  const accounts = form.accounts.map((account) => ({
    account_id: account.account_id,
    account_name: account.account_name,
    platform: account.platform,
    current_priority: account.current_priority ?? null,
    adapter_config: {
      before_balance: nullableNumber(account.samples[0]?.before_balance),
      after_balance: nullableNumber(account.samples[0]?.after_balance),
      latency_ms: nullableNumber(account.samples[0]?.latency_ms),
      samples: normalizedSamples(account.samples, sampleCount).map((sample) => ({
        before_balance: nullableNumber(sample.before_balance),
        after_balance: nullableNumber(sample.after_balance),
        latency_ms: nullableNumber(sample.latency_ms)
      }))
    }
  }))
  if (accounts.some((account) => {
    const samples = account.adapter_config.samples as Array<{ before_balance: number | null; after_balance: number | null }>
    return samples.some((sample) => sample.before_balance == null || sample.after_balance == null)
  })) {
    appStore.showWarning(t('admin.upstreamCostCalibrations.balanceRequired'))
    return null
  }
  return {
    name: form.name.trim(),
    enabled: form.enabled,
    target_group_id: form.target_group_id,
    model: form.model.trim(),
    adapter_type: form.adapter_type,
    unit: form.unit.trim() || 'credit',
    test_prompt: form.test_prompt.trim() || 'ping',
    sample_count: sampleCount,
    priority_start: priorityStart,
    priority_step: priorityStep,
    accounts
  }
}

async function runTask(task: UpstreamCostCalibrationTask) {
  if (!window.confirm(t('admin.upstreamCostCalibrations.runConfirm', { name: task.name }))) return
  runningTaskId.value = task.id
  try {
    const run = await upstreamCostCalibrationsAPI.run(task.id)
    appStore.showSuccess(t('admin.upstreamCostCalibrations.runCompleted'))
    await loadTasks()
    await openRuns(task, run.id)
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.runFailed'))
  } finally {
    runningTaskId.value = null
  }
}

async function openRuns(task: UpstreamCostCalibrationTask, preferredRunId?: number) {
  runsDialogOpen.value = true
  runsLoading.value = true
  runs.value = []
  selectedRun.value = null
  selectedRunsTaskId.value = task.id
  try {
    const result = await upstreamCostCalibrationsAPI.listRuns(task.id, { page: 1, page_size: 30 })
    runs.value = result.items || []
    const firstRunID = preferredRunId || runs.value[0]?.id
    if (firstRunID) {
      await loadRunDetail(firstRunID)
    }
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.historyFailed'))
  } finally {
    runsLoading.value = false
  }
}

async function loadRunDetail(runId: number) {
  const taskId = selectedRunsTaskId.value
  if (!taskId) return
  runDetailLoading.value = true
  try {
    selectedRun.value = await upstreamCostCalibrationsAPI.getRun(taskId, runId)
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.detailFailed'))
  } finally {
    runDetailLoading.value = false
  }
}

async function applySelectedRun() {
  const taskId = selectedRunsTaskId.value
  const run = selectedRun.value
  if (!taskId || !run) return
  if (!window.confirm(t('admin.upstreamCostCalibrations.applyConfirm', { id: run.id }))) return
  applyingRun.value = true
  try {
    selectedRun.value = await upstreamCostCalibrationsAPI.applyRun(taskId, run.id)
    runs.value = runs.value.map((item) => (item.id === run.id ? { ...item, applied: true, applied_at: selectedRun.value?.applied_at || item.applied_at } : item))
    appStore.showSuccess(t('admin.upstreamCostCalibrations.appliedSuccess'))
    await loadTasks()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.applyFailed'))
  } finally {
    applyingRun.value = false
  }
}

async function deleteTask(task: UpstreamCostCalibrationTask) {
  if (!window.confirm(t('admin.upstreamCostCalibrations.deleteConfirm', { name: task.name }))) return
  try {
    await upstreamCostCalibrationsAPI.delete(task.id)
    appStore.showSuccess(t('admin.upstreamCostCalibrations.deleted'))
    await loadTasks()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.upstreamCostCalibrations.deleteFailed'))
  }
}

function onPageChange(page: number) {
  pagination.page = page
  void loadTasks()
}

function onPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadTasks()
}

function groupName(id: number) {
  return groups.value.find((group) => group.id === id)?.name || `#${id}`
}

function runStatusLabel(value: UpstreamCostCalibrationRunStatus) {
  return t(`admin.upstreamCostCalibrations.runStatuses.${value}`)
}

function runStatusClass(status: UpstreamCostCalibrationRunStatus) {
  if (status === 'success') return statusPillClass('enabled')
  if (status === 'failed') return statusPillClass('failed')
  return 'bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300'
}

function statusPillClass(status: 'enabled' | 'disabled' | 'failed') {
  if (status === 'enabled') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function numberFromConfig(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function samplesFromConfig(config: Record<string, unknown>, sampleCount: number): CalibrationSampleForm[] {
  const rawSamples = Array.isArray(config.samples) ? config.samples : []
  const samples = rawSamples
    .map((sample) => {
      if (!sample || typeof sample !== 'object') return null
      const item = sample as Record<string, unknown>
      return {
        before_balance: numberFromConfig(item.before_balance),
        after_balance: numberFromConfig(item.after_balance),
        latency_ms: numberFromConfig(item.latency_ms)
      }
    })
    .filter((sample): sample is CalibrationSampleForm => sample !== null)
  if (samples.length === 0) {
    samples.push({
      before_balance: numberFromConfig(config.before_balance),
      after_balance: numberFromConfig(config.after_balance),
      latency_ms: numberFromConfig(config.latency_ms)
    })
  }
  return normalizedSamples(samples, sampleCount)
}

function normalizedSamples(samples: CalibrationSampleForm[], count: number): CalibrationSampleForm[] {
  const size = Number.isInteger(count) && count > 0 ? Math.min(count, 10) : 1
  const next = samples.slice(0, size).map((sample) => ({ ...sample }))
  while (next.length < size) {
    next.push({ before_balance: null, after_balance: null, latency_ms: null })
  }
  return next
}

function syncFormSampleCounts() {
  const sampleCount = Number(form.sample_count)
  form.sample_count = Number.isInteger(sampleCount) && sampleCount > 0 ? Math.min(sampleCount, 10) : 1
  form.accounts = form.accounts.map((account) => ({
    ...account,
    samples: normalizedSamples(account.samples, form.sample_count)
  }))
}

function nullableNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function formatNumber(value: number | null | undefined) {
  if (value == null) return '-'
  return new Intl.NumberFormat().format(value)
}

function formatDecimal(value: number | null | undefined) {
  if (value == null) return '-'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 8 }).format(value)
}

function formatDateTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
</script>
