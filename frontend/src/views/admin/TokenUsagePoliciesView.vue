<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.tokenUsagePolicies.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.tokenUsagePolicies.description') }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary inline-flex items-center gap-2" type="button" @click="loadPolicies">
            <Icon name="refresh" size="sm" />
            {{ t('common.refresh') }}
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateDialog">
            <Icon name="plus" size="sm" />
            {{ t('admin.tokenUsagePolicies.create') }}
          </button>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex min-h-64 items-center justify-center">
          <LoadingSpinner />
        </div>
        <EmptyState
          v-else-if="policies.length === 0"
          class="min-h-64"
          :title="t('admin.tokenUsagePolicies.emptyTitle')"
          :description="t('admin.tokenUsagePolicies.emptyDescription')"
          :action-text="t('admin.tokenUsagePolicies.create')"
          @action="openCreateDialog"
        />
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[960px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.policy') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.targetGroup') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.runningRule') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.executionStatus') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <template v-for="policy in policies" :key="policy.id">
                <tr class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                  <td class="px-4 py-4 align-top">
                    <div class="font-medium text-gray-900 dark:text-white">{{ policy.name }}</div>
                    <div class="mt-1 max-w-[360px] text-xs leading-5 text-gray-500 dark:text-gray-400">
                      {{ tierSummary(policy) }}
                    </div>
                  </td>
                  <td class="px-4 py-4 align-top text-gray-700 dark:text-gray-300">
                    <div class="font-medium">{{ policy.target_group_name || groupName(policy.target_group_id) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ policyFilterSummary(policy) }}</div>
                  </td>
                  <td class="px-4 py-4 align-top">
                    <div class="space-y-1 text-gray-700 dark:text-gray-300">
                      <div v-for="line in policyRunRuleLines(policy)" :key="line" class="text-sm">{{ line }}</div>
                    </div>
                  </td>
                  <td class="px-4 py-4 align-top">
                    <div class="flex flex-wrap gap-2">
                      <span :class="policy.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                        {{ policy.enabled ? t('common.enabled') : t('common.disabled') }}
                      </span>
                      <span v-if="policy.latest_run" :class="runStatusClass(policy.latest_run.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                        {{ runStatusLabel(policy.latest_run.status) }}
                      </span>
                    </div>
                    <div class="mt-2 space-y-1 text-xs text-gray-500 dark:text-gray-400">
                      <div>{{ t('admin.tokenUsagePolicies.lastRun') }}: {{ formatDateTime(policy.last_run_at) }}</div>
                      <div>{{ t('admin.tokenUsagePolicies.nextRun') }}: {{ formatDateTime(policy.next_run_at) }}</div>
                    </div>
                  </td>
                  <td class="px-4 py-4 align-top">
                    <div class="flex justify-end gap-2">
                      <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" @click="openPreview(policy)">
                        <Icon name="eye" size="xs" />
                        {{ t('admin.tokenUsagePolicies.preview') }}
                      </button>
                      <button class="btn btn-primary px-3 py-1.5 text-xs" type="button" :disabled="runningPolicyId === policy.id" @click="runPolicy(policy)">
                        <Icon name="play" size="xs" />
                        {{ runningPolicyId === policy.id ? t('admin.tokenUsagePolicies.running') : t('admin.tokenUsagePolicies.runNow') }}
                      </button>
                      <button class="btn btn-secondary px-2.5 py-1.5 text-xs" type="button" @click="togglePolicyActions(policy.id)">
                        <Icon :name="isPolicyActionsExpanded(policy.id) ? 'chevronUp' : 'more'" size="xs" />
                        {{ isPolicyActionsExpanded(policy.id) ? t('admin.tokenUsagePolicies.collapseActions') : t('admin.tokenUsagePolicies.moreActions') }}
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="isPolicyActionsExpanded(policy.id)" class="bg-gray-50/80 dark:bg-dark-800/60">
                  <td colspan="5" class="px-4 py-3">
                    <div class="flex flex-wrap items-center justify-end gap-2">
                      <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" @click="openRuns(policy)">
                        <Icon name="clock" size="xs" />
                        {{ t('admin.tokenUsagePolicies.history') }}
                      </button>
                      <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" @click="openEditDialog(policy)">
                        <Icon name="edit" size="xs" />
                        {{ t('common.edit') }}
                      </button>
                      <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="clearingPolicyId === policy.id" @click="clearPolicy(policy)">
                        <Icon name="ban" size="xs" />
                        {{ clearingPolicyId === policy.id ? t('admin.tokenUsagePolicies.clearing') : t('admin.tokenUsagePolicies.clearAutoControl') }}
                      </button>
                      <button class="btn btn-danger px-3 py-1.5 text-xs" type="button" @click="deletePolicy(policy)">
                        <Icon name="trash" size="xs" />
                        {{ t('common.delete') }}
                      </button>
                    </div>
                  </td>
                </tr>
              </template>
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

  <BaseDialog :show="formDialogOpen" :title="formMode === 'create' ? t('admin.tokenUsagePolicies.create') : t('admin.tokenUsagePolicies.edit')" width="extra-wide" @close="formDialogOpen = false">
    <form class="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1fr)_320px]" @submit.prevent="submitForm">
      <div class="space-y-5">
        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tokenUsagePolicies.basicInfo') }}</h3>
          <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.name') }}</span>
              <input v-model.trim="form.name" class="input w-full" type="text" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.targetGroup') }}</span>
              <Select v-model="form.target_group_id" :options="groupOptions" searchable />
            </label>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tokenUsagePolicies.matchConditions') }}</h3>
          <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.filterGroup') }}</span>
              <Select v-model="form.filters.group_id" :options="filterGroupOptions" searchable clearable />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.filterModel') }}</span>
              <input v-model.trim="filterModelText" class="input w-full" type="text" :placeholder="t('admin.tokenUsagePolicies.filterModelPlaceholder')" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.filterRequestType') }}</span>
              <Select v-model="form.filters.request_type" :options="requestTypeOptions" clearable />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.filterBillingType') }}</span>
              <Select v-model="form.filters.billing_type" :options="billingTypeOptions" clearable />
            </label>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tokenUsagePolicies.tierRules') }}</h3>
            <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" @click="addTier">
              <Icon name="plus" size="xs" />
              {{ t('admin.tokenUsagePolicies.addTier') }}
            </button>
          </div>
          <div class="mt-4 space-y-3">
            <div v-for="(tier, index) in form.tiers" :key="index" class="grid grid-cols-1 gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800 md:grid-cols-[auto_1fr_1fr_1fr_1fr_auto] md:items-end">
              <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-white text-sm font-semibold text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                {{ index + 1 }}
              </div>
              <label class="space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.conditionMode') }}</span>
                <Select v-model="tier.condition_mode" :options="conditionModeOptions" />
              </label>
              <label class="space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.minTokens') }}</span>
                <input v-model.number="tier.min_tokens" class="input w-full" type="number" min="0" step="1" :disabled="tier.condition_mode === 'actual_cost'" />
              </label>
              <label class="space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.minActualCost') }}</span>
                <input v-model.number="tier.min_actual_cost" class="input w-full" type="number" min="0" step="0.000001" :disabled="tier.condition_mode === 'token'" />
              </label>
              <label class="space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.rateMultiplier') }}</span>
                <input v-model.number="tier.rate_multiplier" class="input w-full" type="number" min="0.0001" step="0.0001" />
              </label>
              <button class="btn btn-secondary px-3 py-2" type="button" :disabled="form.tiers.length === 1" @click="removeTier(index)">
                <Icon name="trash" size="xs" />
                {{ t('common.delete') }}
              </button>
            </div>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tokenUsagePolicies.executionControl') }}</h3>
          <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-3">
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.window') }}</span>
              <Select v-model="form.window_days" :options="windowOptions" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.actionMode') }}</span>
              <Select v-model="form.action_mode" :options="actionModeOptions" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.frequency') }}</span>
              <Select v-model="form.schedule_frequency" :options="frequencyOptions" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.conflictMode') }}</span>
              <Select v-model="form.conflict_mode" :options="conflictModeOptions" />
            </label>
            <label class="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:bg-dark-800 dark:text-gray-300">
              <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              {{ t('admin.tokenUsagePolicies.enabled') }}
            </label>
          </div>
        </section>
      </div>

      <aside class="h-fit rounded-lg border border-primary-100 bg-primary-50/60 p-4 dark:border-primary-900/40 dark:bg-primary-950/20">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tokenUsagePolicies.ruleSummary') }}</h3>
        <div class="mt-4 space-y-4 text-sm">
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.summaryScope') }}</div>
            <div class="mt-1 text-gray-900 dark:text-gray-100">{{ formRuleScope }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.summaryFilters') }}</div>
            <div class="mt-1 text-gray-900 dark:text-gray-100">{{ formFilterSummary }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.tokenUsagePolicies.summaryTiers') }}</div>
            <ul class="mt-2 space-y-2">
              <li v-for="line in formTierSummaryLines" :key="line" class="rounded-md bg-white px-3 py-2 text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-200">
                {{ line }}
              </li>
            </ul>
          </div>
        </div>
      </aside>
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

  <BaseDialog :show="previewDialogOpen" :title="t('admin.tokenUsagePolicies.preview')" width="full" @close="previewDialogOpen = false">
    <div v-if="previewLoading" class="flex min-h-60 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="previewResult" class="space-y-4">
      <div class="grid grid-cols-2 gap-3 md:grid-cols-6">
        <div v-for="metric in previewMetrics" :key="metric.label" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
        </div>
      </div>
      <EmptyState
        v-if="previewResult.changes.length === 0"
        class="min-h-48"
        :title="t('admin.tokenUsagePolicies.previewEmptyTitle')"
        :description="t('admin.tokenUsagePolicies.previewEmptyDescription')"
      />
      <div v-else class="space-y-4">
        <section v-for="group in previewGroups" :key="group.type" class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ group.label }}</h3>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ group.changes.length }}</span>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[1320px] text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.user') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tokenUsage') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.actualCost') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.tierConditionMode') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tierMinTokens') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tierMinActualCost') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.oldRate') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.newRate') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.reason') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="change in group.changes" :key="`${change.change_type}-${change.user_id}`">
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ change.user_name || change.user_email || change.user_id }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">ID {{ change.user_id }} · {{ change.user_email || '-' }}</div>
                  </td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(change.token_usage) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(change.actual_cost) }}</td>
                  <td class="px-4 py-3">{{ change.tier_condition_mode ? conditionModeLabel(change.tier_condition_mode) : '-' }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(change.tier_min_tokens) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(change.tier_min_actual_cost) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatRate(change.old_rate_multiplier) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatRate(change.new_rate_multiplier) }}</td>
                  <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ change.reason || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  </BaseDialog>

  <BaseDialog :show="runsDialogOpen" :title="t('admin.tokenUsagePolicies.history')" width="extra-wide" @close="runsDialogOpen = false">
    <div v-if="runsLoading" class="flex min-h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[960px] text-sm">
        <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
          <tr>
            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.runType') }}</th>
            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.status') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.totalUsers') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.changes') }}</th>
            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.startedAt') }}</th>
            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.error') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <template v-for="run in runs" :key="run.id">
            <tr>
              <td class="px-4 py-3">{{ runTypeLabel(run.run_type) }}</td>
              <td class="px-4 py-3">
                <span :class="runStatusClass(run.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                  {{ runStatusLabel(run.status) }}
                </span>
              </td>
              <td class="px-4 py-3 text-right">{{ run.total_users }}</td>
              <td class="px-4 py-3 text-right">
                <button
                  v-if="hasRunChanges(run)"
                  class="text-primary-600 hover:text-primary-700 dark:text-primary-400"
                  type="button"
                  @click="toggleRunDetails(run.id)"
                >
                  {{ runChangeCount(run) }}
                  · {{ isRunExpanded(run.id) ? t('admin.tokenUsagePolicies.hideDetails') : t('admin.tokenUsagePolicies.viewDetails') }}
                </button>
                <span v-else>{{ runChangeCount(run) }}</span>
              </td>
              <td class="px-4 py-3">{{ formatDateTime(run.started_at) }}</td>
              <td class="px-4 py-3 text-red-600 dark:text-red-300">{{ run.error_message || '-' }}</td>
            </tr>
            <tr v-if="isRunExpanded(run.id)">
              <td colspan="9" class="bg-gray-50 px-4 py-4 dark:bg-dark-800/60">
                <div v-if="isRunDetailsLoading(run.id)" class="flex min-h-32 items-center justify-center">
                  <LoadingSpinner />
                </div>
                <div v-else-if="isRunDetailsFailed(run.id)" class="flex min-h-32 flex-col items-center justify-center gap-3 text-sm text-gray-500 dark:text-gray-400">
                  <span>{{ t('admin.tokenUsagePolicies.historyDetailsFailed') }}</span>
                  <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" @click="loadRunChanges(run.id)">
                    {{ t('admin.tokenUsagePolicies.retryDetails') }}
                  </button>
                </div>
                <div v-else-if="!loadedRunChanges(run.id).length" class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.tokenUsagePolicies.historyDetailsEmpty') }}
                </div>
                <div v-else class="space-y-4">
                  <section v-for="group in changeGroups(loadedRunChanges(run.id))" :key="group.type" class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
                    <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
                      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ group.label }}</h3>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{{ group.changes.length }}</span>
                    </div>
                    <div class="overflow-x-auto">
                      <table class="w-full min-w-[1320px] text-sm">
                        <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                          <tr>
                            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.user') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tokenUsage') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.actualCost') }}</th>
                            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.tierConditionMode') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tierMinTokens') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.tierMinActualCost') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.oldRate') }}</th>
                            <th class="px-4 py-3 text-right">{{ t('admin.tokenUsagePolicies.newRate') }}</th>
                            <th class="px-4 py-3 text-left">{{ t('admin.tokenUsagePolicies.reason') }}</th>
                          </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                          <tr v-for="change in group.changes" :key="`${run.id}-${change.change_type}-${change.user_id}`">
                            <td class="px-4 py-3">
                              <div class="font-medium text-gray-900 dark:text-white">{{ change.user_name || change.user_email || change.user_id }}</div>
                              <div class="text-xs text-gray-500 dark:text-gray-400">ID {{ change.user_id }} · {{ change.user_email || '-' }}</div>
                            </td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(change.token_usage) }}</td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(change.actual_cost) }}</td>
                            <td class="px-4 py-3">{{ change.tier_condition_mode ? conditionModeLabel(change.tier_condition_mode) : '-' }}</td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(change.tier_min_tokens) }}</td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(change.tier_min_actual_cost) }}</td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatRate(change.old_rate_multiplier) }}</td>
                            <td class="px-4 py-3 text-right tabular-nums">{{ formatRate(change.new_rate_multiplier) }}</td>
                            <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ change.reason || '-' }}</td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
	                  </section>
                  <Pagination
                    v-if="runChangePagination(run.id).total > runChangePagination(run.id).page_size"
                    :page="runChangePagination(run.id).page"
                    :page-size="runChangePagination(run.id).page_size"
                    :total="runChangePagination(run.id).total"
                    :page-size-options="[20, 50, 100]"
                    @update:page="(page) => onRunChangePageChange(run.id, page)"
                    @update:pageSize="(pageSize) => onRunChangePageSizeChange(run.id, pageSize)"
                  />
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { groupsAPI } from '@/api/admin/groups'
import tokenUsagePoliciesAPI, {
  type TokenUsagePolicy,
  type TokenUsagePolicyActionMode,
  type TokenUsagePolicyChange,
  type TokenUsagePolicyChangeType,
  type TokenUsagePolicyConditionMode,
  type TokenUsagePolicyConflictMode,
  type TokenUsagePolicyFilters,
  type TokenUsagePolicyInput,
  type TokenUsagePolicyPreview,
  type TokenUsagePolicyRun,
  type TokenUsagePolicyRunStatus,
  type TokenUsagePolicyRunType,
  type TokenUsagePolicyScheduleFrequency,
  type TokenUsagePolicyTier
} from '@/api/admin/tokenUsagePolicies'
import type { AdminGroup, PaginatedResponse } from '@/types'
import { formatCostFixed } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const policies = ref<TokenUsagePolicy[]>([])
const groups = ref<AdminGroup[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const formDialogOpen = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const editingPolicyId = ref<number | null>(null)
const previewDialogOpen = ref(false)
const previewLoading = ref(false)
const previewResult = ref<TokenUsagePolicyPreview | null>(null)
const runsDialogOpen = ref(false)
const runsLoading = ref(false)
const runs = ref<TokenUsagePolicyRun[]>([])
const selectedRunsPolicyId = ref<number | null>(null)
const expandedRunIds = ref<Set<number>>(new Set())
const runChangePagesById = ref<Record<number, PaginatedResponse<TokenUsagePolicyChange>>>({})
const loadingRunChangeIds = ref<Set<number>>(new Set())
const failedRunChangeIds = ref<Set<number>>(new Set())
const runningPolicyId = ref<number | null>(null)
const clearingPolicyId = ref<number | null>(null)
const expandedPolicyActionIds = ref<Set<number>>(new Set())
const defaultRunChangePageSize = 20

type PolicyForm = {
  name: string
  enabled: boolean
  window_days: 7 | 30
  target_group_id: number | null
  action_mode: TokenUsagePolicyActionMode
  conflict_mode: TokenUsagePolicyConflictMode
  schedule_frequency: TokenUsagePolicyScheduleFrequency
  filters: TokenUsagePolicyFilters
  tiers: TokenUsagePolicyTier[]
}

const form = reactive<PolicyForm>(defaultForm())
const filterModelText = ref('')

const groupOptions = computed(() => groups.value.map((group) => ({ value: group.id, label: group.name })))
const filterGroupOptions = computed(() => [{ value: null, label: t('common.all') }, ...groupOptions.value])
const windowOptions = computed(() => [
  { value: 7, label: t('admin.tokenUsagePolicies.windowDays.7') },
  { value: 30, label: t('admin.tokenUsagePolicies.windowDays.30') }
])
const actionModeOptions = computed(() => [
  { value: 'rate_only', label: t('admin.tokenUsagePolicies.actionModes.rate_only') },
  { value: 'grant_group_and_rate', label: t('admin.tokenUsagePolicies.actionModes.grant_group_and_rate') }
])
const conflictModeOptions = computed(() => [
  { value: 'manual_priority', label: t('admin.tokenUsagePolicies.conflictModes.manual_priority') },
  { value: 'auto_priority', label: t('admin.tokenUsagePolicies.conflictModes.auto_priority') }
])
const conditionModeOptions = computed(() => [
  { value: 'token', label: t('admin.tokenUsagePolicies.conditionModes.token') },
  { value: 'actual_cost', label: t('admin.tokenUsagePolicies.conditionModes.actual_cost') },
  { value: 'both', label: t('admin.tokenUsagePolicies.conditionModes.both') }
])
const frequencyOptions = computed(() => [
  { value: 'every_6h', label: t('admin.tokenUsagePolicies.frequencies.every_6h') },
  { value: 'daily', label: t('admin.tokenUsagePolicies.frequencies.daily') },
  { value: 'weekly', label: t('admin.tokenUsagePolicies.frequencies.weekly') }
])
const requestTypeOptions = computed(() => [
  { value: null, label: t('common.all') },
  { value: 0, label: 'unknown' },
  { value: 1, label: 'sync' },
  { value: 2, label: 'stream' },
  { value: 3, label: 'ws_v2' },
  { value: 4, label: 'cyber' }
])
const billingTypeOptions = computed(() => [
  { value: null, label: t('common.all') },
  { value: 0, label: t('admin.tokenUsagePolicies.billingTypes.balance') },
  { value: 1, label: t('admin.tokenUsagePolicies.billingTypes.subscription') }
])
const previewMetrics = computed(() => {
  const stats = previewResult.value?.stats
  if (!stats) return []
  return [
    { label: t('admin.tokenUsagePolicies.totalUsers'), value: stats.total_users },
    { label: t('admin.tokenUsagePolicies.createCount'), value: stats.create_count },
    { label: t('admin.tokenUsagePolicies.updateCount'), value: stats.update_count },
    { label: t('admin.tokenUsagePolicies.downgradeCount'), value: stats.downgrade_count },
    { label: t('admin.tokenUsagePolicies.clearCount'), value: stats.clear_count },
    { label: t('admin.tokenUsagePolicies.skipCount'), value: stats.skip_count }
  ]
})
const previewGroups = computed(() => {
  const changes = previewResult.value?.changes ?? []
  return changeGroups(changes)
})
const formRuleScope = computed(() => {
  const targetGroup = form.target_group_id ? groupName(form.target_group_id) : t('admin.tokenUsagePolicies.targetGroupMissing')
  return [
    t(`admin.tokenUsagePolicies.windowDays.${form.window_days}`),
    targetGroup,
    actionModeLabel(form.action_mode),
    frequencyLabel(form.schedule_frequency)
  ].join(' · ')
})
const formFilterSummary = computed(() => summarizeFilters(form.filters, filterModelText.value))
const formTierSummaryLines = computed(() => {
  if (!form.tiers.length) return [t('admin.tokenUsagePolicies.noTiers')]
  return form.tiers
    .map((tier) =>
      t('admin.tokenUsagePolicies.tierSummaryLine', {
        conditions: tierConditionSummary(tier),
        rate: formatRate(Number(tier.rate_multiplier) || 0)
      })
    )
})

function changeGroups(changes: TokenUsagePolicyChange[]) {
  const order: TokenUsagePolicyChangeType[] = ['create', 'update', 'downgrade', 'clear', 'skip_manual']
  return order
    .map((type) => ({
      type,
      label: changeTypeLabel(type),
      changes: changes.filter((change) => change.change_type === type)
    }))
    .filter((group) => group.changes.length > 0)
}

onMounted(async () => {
  await Promise.all([loadGroups(), loadPolicies()])
})

function defaultForm(): PolicyForm {
  return {
    name: '',
    enabled: true,
    window_days: 30,
    target_group_id: null,
    action_mode: 'rate_only',
    conflict_mode: 'manual_priority',
    schedule_frequency: 'daily',
    filters: {},
    tiers: [{ condition_mode: 'token', min_tokens: 0, min_actual_cost: 0, rate_multiplier: 1 }]
  }
}

function resetForm(policy?: TokenUsagePolicy) {
  const next = policy
    ? {
        name: policy.name,
        enabled: policy.enabled,
        window_days: policy.window_days,
        target_group_id: policy.target_group_id,
        action_mode: policy.action_mode,
        conflict_mode: policy.conflict_mode,
        schedule_frequency: policy.schedule_frequency,
        filters: { ...policy.filters },
        tiers: policy.tiers.map((tier) => ({
          condition_mode: tier.condition_mode,
          min_tokens: tier.min_tokens,
          min_actual_cost: tier.min_actual_cost,
          rate_multiplier: tier.rate_multiplier
        }))
      }
    : defaultForm()
  Object.assign(form, next)
  filterModelText.value = next.filters.model || ''
}

async function loadGroups() {
  groups.value = await groupsAPI.getAllIncludingInactive()
}

async function loadPolicies() {
  loading.value = true
  try {
    const result = await tokenUsagePoliciesAPI.list({ page: pagination.page, page_size: pagination.page_size })
    policies.value = result.items || []
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  formMode.value = 'create'
  editingPolicyId.value = null
  resetForm()
  formDialogOpen.value = true
}

function openEditDialog(policy: TokenUsagePolicy) {
  formMode.value = 'edit'
  editingPolicyId.value = policy.id
  resetForm(policy)
  formDialogOpen.value = true
}

async function submitForm() {
  const payload = buildPayload()
  if (!payload) return
  saving.value = true
  try {
    if (formMode.value === 'create') {
      await tokenUsagePoliciesAPI.create(payload)
      appStore.showSuccess(t('admin.tokenUsagePolicies.created'))
    } else if (editingPolicyId.value) {
      await tokenUsagePoliciesAPI.update(editingPolicyId.value, payload)
      appStore.showSuccess(t('admin.tokenUsagePolicies.updated'))
    }
    formDialogOpen.value = false
    await loadPolicies()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.saveFailed'))
  } finally {
    saving.value = false
  }
}

function buildPayload(): TokenUsagePolicyInput | null {
  if (!form.name.trim()) {
    appStore.showWarning(t('admin.tokenUsagePolicies.nameRequired'))
    return null
  }
  if (!form.target_group_id) {
    appStore.showWarning(t('admin.tokenUsagePolicies.groupRequired'))
    return null
  }
  const tiers = form.tiers.map((tier) => ({
    condition_mode: tier.condition_mode,
    min_tokens: tier.condition_mode === 'actual_cost' ? 0 : Number(tier.min_tokens),
    min_actual_cost: tier.condition_mode === 'token' ? 0 : Number(tier.min_actual_cost),
    rate_multiplier: Number(tier.rate_multiplier)
  }))
  if (tiers.length === 0 || tiers.some((tier) => {
    const usesTokens = tier.condition_mode !== 'actual_cost'
    const usesActualCost = tier.condition_mode !== 'token'
    return !Number.isFinite(tier.rate_multiplier) || tier.rate_multiplier <= 0
      || (usesTokens && (!Number.isFinite(tier.min_tokens) || tier.min_tokens < 0))
      || (usesActualCost && (!Number.isFinite(tier.min_actual_cost) || tier.min_actual_cost < 0))
  })) {
    appStore.showWarning(t('admin.tokenUsagePolicies.tiersInvalid'))
    return null
  }
  const thresholds = new Set(tiers.map((tier) => `${tier.condition_mode}:${tier.min_tokens}:${tier.min_actual_cost}`))
  if (thresholds.size !== tiers.length) {
    appStore.showWarning(t('admin.tokenUsagePolicies.tiersDuplicate'))
    return null
  }
  return {
    name: form.name.trim(),
    enabled: form.enabled,
    window_days: form.window_days,
    target_group_id: form.target_group_id,
    action_mode: form.action_mode,
    conflict_mode: form.conflict_mode,
    schedule_frequency: form.schedule_frequency,
    filters: {
      group_id: form.filters.group_id || undefined,
      model: filterModelText.value.trim() || undefined,
      request_type: form.filters.request_type ?? undefined,
      billing_type: form.filters.billing_type ?? undefined
    },
    tiers
  }
}

function addTier() {
  const max = Math.max(0, ...form.tiers.map((tier) => Number(tier.min_tokens) || 0))
  form.tiers.push({ condition_mode: 'token', min_tokens: max + 1000000, min_actual_cost: 0, rate_multiplier: 1 })
}

function removeTier(index: number) {
  form.tiers.splice(index, 1)
}

function isPolicyActionsExpanded(policyId: number) {
  return expandedPolicyActionIds.value.has(policyId)
}

function togglePolicyActions(policyId: number) {
  const next = new Set(expandedPolicyActionIds.value)
  if (next.has(policyId)) {
    next.delete(policyId)
  } else {
    next.add(policyId)
  }
  expandedPolicyActionIds.value = next
}

async function openPreview(policy: TokenUsagePolicy) {
  previewDialogOpen.value = true
  previewLoading.value = true
  previewResult.value = null
  try {
    previewResult.value = await tokenUsagePoliciesAPI.preview(policy.id)
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.previewFailed'))
  } finally {
    previewLoading.value = false
  }
}

async function runPolicy(policy: TokenUsagePolicy) {
  if (!window.confirm(t('admin.tokenUsagePolicies.runConfirm', { name: policy.name }))) return
  runningPolicyId.value = policy.id
  try {
    await tokenUsagePoliciesAPI.run(policy.id)
    appStore.showSuccess(t('admin.tokenUsagePolicies.runQueued'))
    await loadPolicies()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.runFailed'))
  } finally {
    runningPolicyId.value = null
  }
}

async function clearPolicy(policy: TokenUsagePolicy) {
  if (!window.confirm(t('admin.tokenUsagePolicies.clearConfirm', { name: policy.name }))) return
  clearingPolicyId.value = policy.id
  try {
    await tokenUsagePoliciesAPI.clear(policy.id)
    appStore.showSuccess(t('admin.tokenUsagePolicies.cleared'))
    await loadPolicies()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.clearFailed'))
  } finally {
    clearingPolicyId.value = null
  }
}

async function openRuns(policy: TokenUsagePolicy) {
  runsDialogOpen.value = true
  runsLoading.value = true
  runs.value = []
  selectedRunsPolicyId.value = policy.id
  expandedRunIds.value = new Set()
  runChangePagesById.value = {}
  loadingRunChangeIds.value = new Set()
  failedRunChangeIds.value = new Set()
  try {
    const result = await tokenUsagePoliciesAPI.listRuns(policy.id, { page: 1, page_size: 30 })
    runs.value = result.items || []
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.historyFailed'))
  } finally {
    runsLoading.value = false
  }
}

function hasRunChanges(run: TokenUsagePolicyRun) {
  return runChangeCount(run) > 0
}

function runChangeCount(run: TokenUsagePolicyRun) {
  return run.create_count + run.update_count + run.downgrade_count + run.clear_count + run.skip_count
}

function isRunExpanded(runId: number) {
  return expandedRunIds.value.has(runId)
}

function loadedRunChanges(runId: number) {
  return runChangePagesById.value[runId]?.items ?? []
}

function runChangePagination(runId: number) {
  return runChangePagesById.value[runId] ?? {
    items: [],
    total: 0,
    page: 1,
    page_size: defaultRunChangePageSize,
    pages: 1
  }
}

function isRunDetailsLoading(runId: number) {
  return loadingRunChangeIds.value.has(runId)
}

function isRunDetailsFailed(runId: number) {
  return failedRunChangeIds.value.has(runId)
}

async function toggleRunDetails(runId: number) {
  const next = new Set(expandedRunIds.value)
  if (next.has(runId)) {
    next.delete(runId)
    expandedRunIds.value = next
  } else {
    next.add(runId)
    expandedRunIds.value = next
    await loadRunChanges(runId, 1)
  }
}

async function loadRunChanges(runId: number, page = runChangePagination(runId).page, pageSize = runChangePagination(runId).page_size) {
  const policyId = selectedRunsPolicyId.value
  if (!policyId || loadingRunChangeIds.value.has(runId)) return
  const failedNext = new Set(failedRunChangeIds.value)
  failedNext.delete(runId)
  failedRunChangeIds.value = failedNext
  loadingRunChangeIds.value = new Set([...loadingRunChangeIds.value, runId])
  try {
    const pageResult = await tokenUsagePoliciesAPI.listRunChanges(policyId, runId, { page, page_size: pageSize })
    runChangePagesById.value = { ...runChangePagesById.value, [runId]: pageResult }
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.historyDetailsFailed'))
    failedRunChangeIds.value = new Set([...failedRunChangeIds.value, runId])
  } finally {
    const next = new Set(loadingRunChangeIds.value)
    next.delete(runId)
    loadingRunChangeIds.value = next
  }
}

async function onRunChangePageChange(runId: number, page: number) {
  await loadRunChanges(runId, page, runChangePagination(runId).page_size)
}

async function onRunChangePageSizeChange(runId: number, pageSize: number) {
  await loadRunChanges(runId, 1, pageSize)
}

async function deletePolicy(policy: TokenUsagePolicy) {
  if (!window.confirm(t('admin.tokenUsagePolicies.deleteConfirm', { name: policy.name }))) return
  try {
    await tokenUsagePoliciesAPI.delete(policy.id)
    appStore.showSuccess(t('admin.tokenUsagePolicies.deleted'))
    await loadPolicies()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.tokenUsagePolicies.deleteFailed'))
  }
}

function onPageChange(page: number) {
  pagination.page = page
  void loadPolicies()
}

function onPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadPolicies()
}

function groupName(id: number) {
  return groups.value.find((group) => group.id === id)?.name || `#${id}`
}

function policyRunRuleLines(policy: TokenUsagePolicy) {
  return [
    `${t(`admin.tokenUsagePolicies.windowDays.${policy.window_days}`)} · ${frequencyLabel(policy.schedule_frequency)}`,
    actionModeLabel(policy.action_mode),
    conflictModeLabel(policy.conflict_mode)
  ]
}

function policyFilterSummary(policy: TokenUsagePolicy) {
  return summarizeFilters(policy.filters)
}

function summarizeFilters(filters: TokenUsagePolicyFilters, modelText = filters.model || '') {
  const segments = [
    filters.group_id ? groupName(filters.group_id) : t('admin.tokenUsagePolicies.allGroups'),
    modelText ? modelText : t('admin.tokenUsagePolicies.allModels'),
    optionLabel(requestTypeOptions.value, filters.request_type ?? null),
    optionLabel(billingTypeOptions.value, filters.billing_type ?? null)
  ]
  return segments.join(' · ')
}

function optionLabel(options: Array<{ value: string | number | null; label: string }>, value: string | number | null) {
  return options.find((option) => option.value === value)?.label || t('common.all')
}

function actionModeLabel(value: TokenUsagePolicyActionMode) {
  return t(`admin.tokenUsagePolicies.actionModes.${value}`)
}

function frequencyLabel(value: TokenUsagePolicyScheduleFrequency) {
  return t(`admin.tokenUsagePolicies.frequencies.${value}`)
}

function conflictModeLabel(value: TokenUsagePolicyConflictMode) {
  return t(`admin.tokenUsagePolicies.conflictModes.${value}`)
}

function conditionModeLabel(value: TokenUsagePolicyConditionMode) {
  return t(`admin.tokenUsagePolicies.conditionModes.${value}`)
}

function tierConditionSummary(tier: TokenUsagePolicyTier) {
  if (tier.condition_mode === 'actual_cost') {
    return t('admin.tokenUsagePolicies.tierConditions.actual_cost', { cost: formatCost(tier.min_actual_cost) })
  }
  if (tier.condition_mode === 'both') {
    return t('admin.tokenUsagePolicies.tierConditions.both', {
      tokens: formatNumber(tier.min_tokens),
      cost: formatCost(tier.min_actual_cost)
    })
  }
  return t('admin.tokenUsagePolicies.tierConditions.token', { tokens: formatNumber(tier.min_tokens) })
}

function changeTypeLabel(value: TokenUsagePolicyChangeType) {
  return t(`admin.tokenUsagePolicies.changeTypes.${value}`)
}

function runTypeLabel(value: TokenUsagePolicyRunType) {
  return t(`admin.tokenUsagePolicies.runTypes.${value}`)
}

function runStatusLabel(value: TokenUsagePolicyRunStatus) {
  return t(`admin.tokenUsagePolicies.runStatuses.${value}`)
}

function runStatusClass(status: TokenUsagePolicyRunStatus) {
  if (status === 'success') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300'
}

function tierSummary(policy: TokenUsagePolicy) {
  if (!policy.tiers?.length) return t('admin.tokenUsagePolicies.noTiers')
  return policy.tiers
    .map((tier) => t('admin.tokenUsagePolicies.tierSummaryLine', {
      conditions: tierConditionSummary(tier),
      rate: formatRate(tier.rate_multiplier)
    }))
    .join(' / ')
}

function formatNumber(value: number | null | undefined) {
  if (value == null) return '-'
  return new Intl.NumberFormat().format(value)
}

function formatRate(value: number | null | undefined) {
  if (value == null) return '-'
  return Number(value).toFixed(4)
}

function formatCost(value: number | null | undefined) {
  if (value == null || !Number.isFinite(Number(value))) return '-'
  const amount = Number(value)
  return `$${formatCostFixed(amount, amount > 0 && amount < 0.01 ? 6 : 4)}`
}

function formatDateTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
</script>
