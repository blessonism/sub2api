<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.pageTitle') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.pageDesc') }}</p>
        </div>
        <div class="flex shrink-0 gap-2">
          <button class="btn btn-secondary inline-flex items-center gap-2" type="button" @click="loadCampaigns">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            {{ t('admin.campaignRewards.refresh') }}
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2 whitespace-nowrap" type="button" @click="openCreateDialog">
            <Icon name="plus" size="sm" />
            {{ t('admin.campaignRewards.createCampaign') }}
          </button>
        </div>
      </div>

      <div class="space-y-6">
        <div data-testid="campaign-list-card" class="card overflow-hidden">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.listTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.listDesc') }}</p>
          </div>
          <div v-if="loading" class="flex min-h-64 items-center justify-center">
            <LoadingSpinner />
          </div>
          <div v-else-if="campaigns.length === 0" class="p-4">
            <EmptyState :title="t('admin.campaignRewards.noCampaigns')" :description="t('admin.campaignRewards.noCampaignsDesc')" />
          </div>
          <div v-else data-testid="campaign-list-items" class="grid max-h-[24rem] gap-3 overflow-y-auto p-3 md:grid-cols-2 2xl:grid-cols-3">
            <button
              v-for="campaign in campaigns"
              :key="campaign.id"
              class="block w-full rounded-lg border border-gray-100 px-4 py-3 text-left transition hover:border-primary-200 hover:bg-gray-50 dark:border-dark-700 dark:hover:border-primary-800 dark:hover:bg-dark-800"
              :class="{ 'border-primary-200 bg-primary-50 dark:border-primary-800 dark:bg-primary-900/20': selectedCampaign?.id === campaign.id }"
              type="button"
              @click="selectCampaign(campaign.id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(campaign.start_at) }} → {{ formatDateTime(campaign.end_at) }}
                  </p>
                  <p class="mt-1 text-xs font-medium" :class="campaignLifecycle(campaign).isUrgent ? 'text-amber-600 dark:text-amber-300' : 'text-gray-500 dark:text-dark-400'">
                    {{ campaignLifecycleSummary(campaign) }}
                  </p>
                </div>
                <span class="shrink-0 rounded-md px-2 py-1 text-xs font-medium" :class="statusClass(campaign.status)">
                  {{ statusLabel(campaign.status) }}
                </span>
              </div>
            </button>
          </div>
        </div>

        <div class="space-y-6">
          <EmptyState
            v-if="!selectedCampaign"
            class="card min-h-80"
            :title="t('admin.campaignRewards.selectTitle')"
            :description="t('admin.campaignRewards.selectDesc')"
          />

          <template v-else>
            <div class="card p-5">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="min-w-0">
                  <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ selectedCampaign.name }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedCampaign.description || t('admin.campaignRewards.defaultDescription') }}</p>
                  <div class="mt-3 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span>{{ formatDateTime(selectedCampaign.start_at) }}</span>
                    <span>→</span>
                    <span>{{ formatDateTime(selectedCampaign.end_at) }}</span>
                  </div>
                </div>
                <div class="flex w-full flex-wrap items-center gap-2 lg:w-auto lg:justify-end">
                  <button class="btn btn-secondary whitespace-nowrap" type="button" @click="openEditDialog">
                    {{ t('admin.campaignRewards.editCampaign') }}
                  </button>
                  <button class="btn btn-secondary whitespace-nowrap" type="button" :disabled="!canPublish" @click="publishSelected">
                    {{ t('admin.campaignRewards.publish') }}
                  </button>
                  <button class="btn btn-secondary whitespace-nowrap" type="button" :disabled="!canFreeze" :title="canFreeze ? undefined : t('admin.campaignRewards.freezeDisabledHint')" @click="freezeSelected">
                    {{ t('admin.campaignRewards.freeze') }}
                  </button>
                  <button class="btn btn-secondary whitespace-nowrap" type="button" :disabled="!canPreview" :title="canPreview ? undefined : t('admin.campaignRewards.previewDisabledHint')" @click="previewRewards">
                    {{ t('admin.campaignRewards.preview') }}
                  </button>
                  <button class="btn btn-primary whitespace-nowrap" type="button" :disabled="!canFinalize" :title="canFinalize ? undefined : t('admin.campaignRewards.finalizeDisabledHint')" @click="finalizeRewards">
                    {{ t('admin.campaignRewards.finalize') }}
                  </button>
                  <button class="btn btn-primary whitespace-nowrap" type="button" :disabled="!canPayout" :title="canPayout ? undefined : (selectedCampaign?.status === 'paid' ? t('admin.campaignRewards.payoutAlreadyPaid') : payoutDisabledHint)" @click="payoutSelected">
                    {{ t('admin.campaignRewards.payout') }}
                  </button>
                  <span class="hidden h-5 w-px bg-gray-200 dark:bg-dark-600 sm:block" aria-hidden="true" />
                  <button class="btn btn-secondary whitespace-nowrap" type="button" @click="copySelected">
                    {{ t('admin.campaignRewards.copyAsDraft') }}
                  </button>
                  <button class="btn whitespace-nowrap border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/60 dark:text-red-300 dark:hover:bg-red-900/20" type="button" @click="deleteSelected">
                    {{ t('admin.campaignRewards.deleteOrArchive') }}
                  </button>
                </div>
              </div>
              <p v-if="!hasFinalCalculation" class="mt-3 text-xs text-amber-600 dark:text-amber-300">
                {{ payoutDisabledHint }}
              </p>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.campaignRewards.lifecycleOperationHint') }}
              </p>

              <div class="mt-5 rounded-xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.lifecycleTitle') }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ lifecycleHint }}</p>
                    <p v-if="selectedTimelineWarnings.length > 0" class="mt-2 text-xs text-amber-600 dark:text-amber-300">
                      {{ t(selectedTimelineWarnings[0]) }}
                    </p>
                  </div>
                  <span class="self-start rounded-full px-3 py-1 text-xs font-medium" :class="statusClass(selectedCampaign.status)">
                    {{ statusLabel(selectedCampaign.status) }}
                  </span>
                </div>
                <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
                  <div
                    v-for="step in lifecycleSteps"
                    :key="step.key"
                    class="rounded-lg border bg-white p-3 dark:bg-dark-900"
                    :class="step.current ? 'border-primary-300 ring-1 ring-primary-200 dark:border-primary-600 dark:ring-primary-900/50' : step.done ? 'border-emerald-200 dark:border-emerald-800' : 'border-gray-200 dark:border-dark-700'"
                  >
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-xs font-medium" :class="step.done ? 'text-emerald-700 dark:text-emerald-300' : step.current ? 'text-primary-700 dark:text-primary-300' : 'text-gray-500 dark:text-dark-400'">
                        {{ step.label }}
                      </span>
                      <Icon v-if="step.done" name="check" size="sm" class="text-emerald-500" />
                    </div>
                    <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ step.description }}</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="grid gap-6 xl:grid-cols-[minmax(280px,2fr)_minmax(0,3fr)]">
              <div class="card p-4">
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.userPreview') }}</h2>
                    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.userPreviewDesc') }}</p>
                  </div>
                  <span class="rounded-full px-3 py-1 text-xs font-medium" :class="statusClass(selectedCampaign.status)">
                    {{ statusLabel(selectedCampaign.status) }}
                  </span>
                </div>
                <div class="mt-4 rounded-xl border border-primary-100 bg-primary-50 p-4 dark:border-primary-900/50 dark:bg-primary-900/20">
                  <p class="text-xs font-medium text-primary-700 dark:text-primary-200">{{ t(selectedLifecycleState.labelKey) }}</p>
                  <p class="mt-2 text-2xl font-semibold tabular-nums text-primary-900 dark:text-primary-100">{{ selectedCountdownText }}</p>
                  <p class="mt-2 text-sm text-primary-800 dark:text-primary-200">{{ t(selectedLifecycleState.descriptionKey) }}</p>
                  <p class="mt-2 text-xs text-primary-700/80 dark:text-primary-200/80">{{ selectedTargetText }}</p>
                </div>
              </div>

              <div class="card p-4">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.timelineTitle') }}</h2>
                <div class="mt-4 space-y-3">
                  <div v-for="item in timelineItems" :key="item.key" class="flex gap-3">
                    <div class="mt-1 h-2.5 w-2.5 shrink-0 rounded-full" :class="item.active ? 'bg-primary-500' : item.missing ? 'bg-amber-400' : 'bg-gray-300 dark:bg-dark-600'" />
                    <div class="min-w-0 flex-1">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <p class="text-sm font-medium text-gray-900 dark:text-white">{{ item.label }}</p>
                        <p class="text-xs text-gray-500 dark:text-dark-400">{{ item.timeText }}</p>
                      </div>
                      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.description }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div v-for="metric in poolMetrics" :key="metric.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
                <div class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
              </div>
            </div>

            <div class="grid gap-6 xl:grid-cols-2">
              <div class="card p-4">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.adjustPool') }}</h2>
                <div class="mt-4 grid gap-3 lg:grid-cols-2 lg:items-end">
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.adjustType') }}</span>
                    <select v-model="adjustForm.adjustment_type" class="input">
                      <option value="additional_bonus">{{ t('admin.campaignRewards.additionalBonus') }}</option>
                      <option value="manual_compensation">{{ t('admin.campaignRewards.manualCompensation') }}</option>
                      <option value="exception_deduction">{{ t('admin.campaignRewards.manualDeduction') }}</option>
                    </select>
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.amountYuan') }}</span>
                    <input v-model.number="adjustAmountYuan" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
                    <input v-model.trim="adjustForm.reason" class="input" type="text" />
                  </label>
                  <button class="btn btn-secondary w-full whitespace-nowrap lg:w-auto" type="button" @click="submitPoolAdjustment">
                    {{ t('common.submit') }}
                  </button>
                </div>
                <div class="mt-4 rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900/60">
                  <div class="grid gap-2 sm:grid-cols-3">
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.adjustBefore') }}</p>
                      <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCents(pool?.final_pool_cents) }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.adjustDelta') }}</p>
                      <p class="mt-1 font-semibold" :class="adjustmentPreviewCents < 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'">
                        {{ formatSignedCents(adjustmentPreviewCents) }}
                      </p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.adjustAfter') }}</p>
                      <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCents(adjustmentAfterCents) }}</p>
                    </div>
                  </div>
                  <p v-if="adjustmentRequiresReason" class="mt-3 text-xs text-amber-600 dark:text-amber-300">
                    {{ t('admin.campaignRewards.adjustReasonRequired') }}
                  </p>
                </div>
              </div>

              <div class="card p-4">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.versioning') }}</h2>
                <div class="mt-4 grid gap-3 lg:grid-cols-2 lg:items-end">
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.thresholdYuan') }}</span>
                    <input v-model.number="versionThresholdYuan" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.injectRate') }}</span>
                    <input v-model.number="versionRate" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.poolInjectionScope') }}</span>
                    <select v-model="versionPoolInjectionScope" class="input">
                      <option value="invitees_only">{{ t('admin.campaignRewards.poolInjectionScopeInviteesOnly') }}</option>
                      <option value="all_users">{{ t('admin.campaignRewards.poolInjectionScopeAllUsers') }}</option>
                    </select>
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
                    <input v-model.trim="versionReason" class="input" type="text" />
                  </label>
                  <button class="btn btn-secondary w-full whitespace-nowrap lg:w-auto" type="button" @click="submitVersion">
                    {{ t('admin.campaignRewards.createVersion') }}
                  </button>
                </div>
              </div>
            </div>

            <template v-if="calculation">
              <div class="rounded-xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/30">
                <h2 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.settlementSection') }}</h2>
                <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                  <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                    <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.rankPool') }}</div>
                    <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.rank_pool_cents) }}</div>
                  </div>
                  <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                    <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.contributionPool') }}</div>
                    <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.contribution_pool_cents) }}</div>
                  </div>
                  <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                    <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.finalPayout') }}</div>
                    <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.total_final_payout_cents) }}</div>
                  </div>
                  <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                    <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.withheld') }}</div>
                    <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.total_withheld_cents) }}</div>
                  </div>
                </div>
              </div>

              <div class="card overflow-hidden">
                <div class="border-b border-gray-100 p-4 dark:border-dark-700">
                  <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.rewardResults') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rewardResultsDesc') }}</p>
                </div>
                <div class="overflow-x-auto">
                  <table class="w-full min-w-[980px] text-sm">
                    <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                      <tr>
                        <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.user') }}</th>
                        <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.rank') }}</th>
                        <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.rankRewardAmount') }}</th>
                        <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.contributionRewardAmount') }}</th>
                        <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.grossReward') }}</th>
                        <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.finalPayout') }}</th>
                        <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.withheld') }}</th>
                        <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.withheldReason') }}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                      <tr v-for="result in calculationRows" :key="result.id" :class="{ 'bg-amber-50/50 dark:bg-amber-900/10': result.withheld_amount_cents > 0 }">
                        <td class="px-4 py-3">{{ userDisplayMap.get(result.user_id) ?? `#${result.user_id}` }}</td>
                        <td class="px-4 py-3">{{ result.rank ? `#${result.rank}` : '-' }}</td>
                        <td class="px-4 py-3 text-right">{{ formatCents(result.rank_reward_amount_cents) }}</td>
                        <td class="px-4 py-3 text-right">{{ formatCents(result.contribution_reward_amount_cents) }}</td>
                        <td class="px-4 py-3 text-right">{{ formatCents(result.gross_reward_amount_cents) }}</td>
                        <td class="px-4 py-3 text-right font-semibold">{{ formatCents(result.final_payout_amount_cents) }}</td>
                        <td class="px-4 py-3 text-right">{{ formatCents(result.withheld_amount_cents) }}</td>
                        <td class="px-4 py-3">{{ result.withheld_reason || '-' }}</td>
                      </tr>
                      <tr v-if="calculationRows.length === 0">
                        <td colspan="8" class="px-4 py-10 text-center text-gray-500">{{ t('admin.campaignRewards.noRewardResults') }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div v-if="rewardResultsHiddenCount > 0" class="border-t border-gray-100 px-4 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  {{ t('admin.campaignRewards.rewardResultsHidden', { count: rewardResultsHiddenCount }) }}
                </div>
              </div>
            </template>

            <div class="card overflow-hidden">
              <div class="border-b border-gray-100 p-4 dark:border-dark-700">
                <div class="flex items-center justify-between gap-3">
                  <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.leaderboard') }}</h2>
                  <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="!canManualAdjust" @click="openLeaderboardAdjustment(null)">
                    {{ t('admin.campaignRewards.addManualAdjustment') }}
                  </button>
                </div>
              </div>
              <div class="overflow-x-auto">
                <table class="w-full min-w-[760px] text-sm">
                  <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                    <tr>
                      <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.rank') }}</th>
                      <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.user') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.validInvites') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.rechargeAmount') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.estimatedReward') }}</th>
                      <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.manualAdjustment') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr v-for="row in leaderboard" :key="row.user_id">
                      <td class="px-4 py-3 font-semibold">#{{ row.rank }}</td>
                      <td class="px-4 py-3">{{ row.username || row.masked_email || row.user_id }}</td>
                      <td class="px-4 py-3 text-right">{{ row.valid_invite_count }}</td>
                      <td class="px-4 py-3 text-right">{{ formatCents(row.invitee_recharge_amount_cents) }}</td>
                      <td class="px-4 py-3 text-right">{{ formatCents(row.estimated_reward_cents) }}</td>
                      <td class="px-4 py-3">
                        <span v-if="row.has_manual_adjustment" class="rounded-full bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-200">
                          {{ formatManualDelta(row) }}
                        </span>
                        <span v-else class="text-gray-400">-</span>
                      </td>
                      <td class="px-4 py-3 text-right">
                        <div class="flex justify-end gap-2">
                          <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="!canManualAdjust" @click="openLeaderboardAdjustment(row)">
                            {{ t('admin.campaignRewards.adjustScore') }}
                          </button>
                          <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="openInviteRecords(row)">
                            {{ t('admin.campaignRewards.inviteDetails') }}
                          </button>
                        </div>
                      </td>
                    </tr>
                    <tr v-if="leaderboard.length === 0">
                      <td colspan="7" class="px-4 py-10 text-center text-gray-500">{{ t('admin.campaignRewards.noLeaderboard') }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="!!pendingRiskAction"
      :title="riskDialogTitle"
      :message="riskDialogMessage"
      :confirm-text="riskDialogConfirmText"
      :danger="riskDialogDanger"
      @confirm="confirmRiskAction"
      @cancel="pendingRiskAction = null"
    >
      <div v-if="pendingRiskAction" class="space-y-3 rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900/60">
        <div v-for="item in riskDialogMetrics" :key="item.label" class="flex items-center justify-between gap-4">
          <span class="text-gray-500 dark:text-dark-400">{{ item.label }}</span>
          <span class="font-semibold text-gray-900 dark:text-white">{{ item.value }}</span>
        </div>
      </div>
    </ConfirmDialog>

    <BaseDialog :show="createDialogOpen" :title="t('admin.campaignRewards.createDialogTitle')" width="wide" @close="createDialogOpen = false">
      <div class="space-y-5">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.createBasicInfo') }}</h3>
          <div class="mt-3 grid gap-3 md:grid-cols-2">
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.campaignName') }}</span>
              <input v-model.trim="createForm.name" class="input" type="text" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.descriptionLabel') }}</span>
              <textarea v-model.trim="createForm.description" class="input min-h-20" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.startAt') }}</span>
              <input v-model="createForm.start_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.endAt') }}</span>
              <input v-model="createForm.end_at" class="input" type="datetime-local" />
            </label>
          </div>
        </div>

        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.createRewardRules') }}</h3>
          <div class="mt-3 grid gap-3 md:grid-cols-3">
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.thresholdYuan') }}</span>
              <input v-model.number="createForm.recharge_threshold_yuan" class="input" type="number" min="0" step="0.01" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.injectRate') }}</span>
              <input v-model.number="createForm.pool_injection_rate" class="input" type="number" min="0" step="0.01" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.poolInjectionScope') }}</span>
              <select v-model="createForm.pool_injection_scope" class="input">
                <option value="invitees_only">{{ t('admin.campaignRewards.poolInjectionScopeInviteesOnly') }}</option>
                <option value="all_users">{{ t('admin.campaignRewards.poolInjectionScopeAllUsers') }}</option>
              </select>
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.minPayoutYuan') }}</span>
              <input v-model.number="createForm.min_payout_yuan" class="input" type="number" min="0" step="0.01" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rankPoolRatio') }}</span>
              <input v-model.number="createForm.rank_pool_ratio" class="input" type="number" min="0" step="0.01" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.contributionPoolRatio') }}</span>
              <input v-model.number="createForm.contribution_pool_ratio" class="input" type="number" min="0" step="0.01" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rankRewardCount') }}</span>
              <input v-model.number="createForm.rank_reward_count" class="input" type="number" min="1" step="1" />
            </label>
            <label class="space-y-1 md:col-span-3">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rankWeights') }}</span>
              <input v-model.trim="createForm.rank_weights" class="input" type="text" />
            </label>
          </div>
          <p v-if="createValidationMessage" class="mt-3 text-xs text-amber-600 dark:text-amber-300">{{ createValidationMessage }}</p>
        </div>

        <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900/60">
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.createSummary') }}</h3>
          <div class="mt-3 grid gap-2 md:grid-cols-3">
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.thresholdYuan') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCents(yuanToCents(createForm.recharge_threshold_yuan)) }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.poolSplit') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ createForm.rank_pool_ratio }} / {{ createForm.contribution_pool_ratio }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.poolInjectionScope') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ poolInjectionScopeLabel(createForm.pool_injection_scope) }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.minPayoutYuan') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCents(yuanToCents(createForm.min_payout_yuan)) }}</p>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" :disabled="createSubmitting" @click="createDialogOpen = false">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="createSubmitting || !!createValidationMessage" @click="submitCreateCampaign">
            {{ createSubmitting ? t('common.processing') : t('admin.campaignRewards.createCampaign') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="editDialogOpen" :title="t('admin.campaignRewards.editDialogTitle')" width="wide" @close="editDialogOpen = false">
      <div class="space-y-5">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.editBasicInfo') }}</h3>
          <div class="mt-3 grid gap-3 md:grid-cols-2">
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.campaignName') }}</span>
              <input v-model.trim="editForm.name" class="input" type="text" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.descriptionLabel') }}</span>
              <textarea v-model.trim="editForm.description" class="input min-h-20" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rulesText') }}</span>
              <textarea v-model.trim="editForm.rules_text" class="input min-h-24" />
            </label>
          </div>
        </div>

        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.editTimelineSettings') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.optionalTimeHint') }}</p>
          <div class="mt-3 grid gap-3 md:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.warmupStartAt') }}</span>
              <input v-model="editForm.warmup_start_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.startAt') }}</span>
              <input v-model="editForm.start_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.endAt') }}</span>
              <input v-model="editForm.end_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.auditStartAt') }}</span>
              <input v-model="editForm.audit_start_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.auditEndAt') }}</span>
              <input v-model="editForm.audit_end_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.publicityStartAt') }}</span>
              <input v-model="editForm.publicity_start_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.publicityEndAt') }}</span>
              <input v-model="editForm.publicity_end_at" class="input" type="datetime-local" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.payoutDueAt') }}</span>
              <input v-model="editForm.payout_due_at" class="input" type="datetime-local" />
            </label>
          </div>
          <p v-if="editValidationMessage" class="mt-3 text-xs text-amber-600 dark:text-amber-300">{{ editValidationMessage }}</p>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" :disabled="editSubmitting" @click="editDialogOpen = false">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="editSubmitting || !!editValidationMessage" @click="submitEditCampaign">
            {{ editSubmitting ? t('common.processing') : t('admin.campaignRewards.saveCampaign') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="leaderboardAdjustDialogOpen" :title="t('admin.campaignRewards.adjustScoreTitle')" @close="leaderboardAdjustDialogOpen = false">
      <div class="space-y-4">
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.userId') }}</span>
          <input v-model.number="leaderboardAdjustForm.user_id" class="input" type="number" min="1" step="1" :disabled="!!selectedLeaderboardRow" />
        </label>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.validInviteDelta') }}</span>
            <input v-model.number="leaderboardAdjustForm.valid_invite_delta" class="input" type="number" step="1" />
          </label>
          <label class="space-y-1">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rechargeDeltaYuan') }}</span>
            <input v-model.number="leaderboardAdjustForm.recharge_delta_yuan" class="input" type="number" step="0.01" />
          </label>
        </div>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
          <textarea v-model="leaderboardAdjustForm.reason" class="input min-h-[88px]" />
        </label>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" :disabled="manualSubmitting" @click="leaderboardAdjustDialogOpen = false">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="manualSubmitting || !canSubmitLeaderboardAdjustment" @click="submitLeaderboardAdjustment">
            {{ manualSubmitting ? t('common.processing') : t('common.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="inviteRecordsDialogOpen" :title="t('admin.campaignRewards.inviteDetailsTitle')" width="wide" @close="inviteRecordsDialogOpen = false">
      <div class="space-y-4">
        <div class="overflow-x-auto rounded-lg border border-gray-100 dark:border-dark-700">
          <table class="w-full min-w-[760px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.user') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.status') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.rechargeAmount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="record in inviteRecords" :key="record.id">
                <td class="px-4 py-3">{{ record.invitee_username || record.invitee_masked_email || record.invitee_user_id }}</td>
                <td class="px-4 py-3">{{ inviteStatusLabel(record.status) }}</td>
                <td class="px-4 py-3 text-right">{{ formatCents(record.effective_recharge_amount_cents) }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="!canManualAdjust" @click="openInviteRecordAdjustment(record)">
                    {{ t('admin.campaignRewards.adjustInviteRecord') }}
                  </button>
                </td>
              </tr>
              <tr v-if="inviteRecords.length === 0">
                <td colspan="4" class="px-4 py-10 text-center text-gray-500">{{ t('admin.campaignRewards.noInviteRecords') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="selectedInviteRecord" class="rounded-lg border border-gray-100 p-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.adjustInviteRecord') }}</h3>
          <div class="mt-3 grid gap-4 sm:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.status') }}</span>
              <select v-model="inviteAdjustForm.status" class="input">
                <option value="registered">{{ inviteStatusLabel('registered') }}</option>
                <option value="recharge_unqualified">{{ inviteStatusLabel('recharge_unqualified') }}</option>
                <option value="pending_audit">{{ inviteStatusLabel('pending_audit') }}</option>
                <option value="effective">{{ inviteStatusLabel('effective') }}</option>
                <option value="invalid">{{ inviteStatusLabel('invalid') }}</option>
                <option value="risk_review">{{ inviteStatusLabel('risk_review') }}</option>
              </select>
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.rechargeAmount') }}</span>
              <input v-model.number="inviteAdjustForm.recharge_yuan" class="input" type="number" min="0" step="0.01" />
            </label>
          </div>
          <label class="mt-3 block space-y-1">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
            <textarea v-model="inviteAdjustForm.reason" class="input min-h-[80px]" />
          </label>
          <div class="mt-3 flex justify-end">
            <button class="btn btn-primary" type="button" :disabled="manualSubmitting || !canSubmitInviteAdjustment" @click="submitInviteRecordAdjustment">
              {{ manualSubmitting ? t('common.processing') : t('common.submit') }}
            </button>
          </div>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { Campaign, CampaignInviteRecord, CampaignLeaderboardRow, CampaignPoolSummary } from '@/api/campaigns'
import type { CampaignCalculationSummary, CampaignUpdateRequest } from '@/api/admin/campaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { getCampaignLifecycleState, getCampaignTimeWarnings } from '@/utils/campaignLifecycle'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const campaigns = ref<Campaign[]>([])
const selectedCampaign = ref<Campaign | null>(null)
const pool = ref<CampaignPoolSummary | null>(null)
const leaderboard = ref<CampaignLeaderboardRow[]>([])
const calculation = ref<CampaignCalculationSummary | null>(null)
const adjustAmountYuan = ref(0)
const versionThresholdYuan = ref(20)
const versionRate = ref(0.1)
const versionPoolInjectionScope = ref<'invitees_only' | 'all_users'>('invitees_only')
const versionReason = ref('')
const pendingRiskAction = ref<'publish' | 'freeze' | 'finalize' | 'payout' | 'deduct' | 'delete' | 'copy' | null>(null)
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | null = null
const createDialogOpen = ref(false)
const createSubmitting = ref(false)
const editDialogOpen = ref(false)
const editSubmitting = ref(false)
const adjustForm = reactive({
  adjustment_type: 'additional_bonus',
  reason: '',
})
const createForm = reactive({
  name: '',
  description: '',
  rules_text: '',
  start_at: '',
  end_at: '',
  recharge_threshold_yuan: 20,
  pool_injection_rate: 0.1,
  pool_injection_scope: 'invitees_only' as 'invitees_only' | 'all_users',
  rank_pool_ratio: 0.8,
  contribution_pool_ratio: 0.2,
  rank_reward_count: 10,
  rank_weights: '30,20,15,10,8,6,4,3,2,2',
  min_payout_yuan: 1,
})
const editForm = reactive({
  name: '',
  description: '',
  rules_text: '',
  warmup_start_at: '',
  start_at: '',
  end_at: '',
  audit_start_at: '',
  audit_end_at: '',
  publicity_start_at: '',
  publicity_end_at: '',
  payout_due_at: '',
})
const leaderboardAdjustDialogOpen = ref(false)
const inviteRecordsDialogOpen = ref(false)
const manualSubmitting = ref(false)
const selectedLeaderboardRow = ref<CampaignLeaderboardRow | null>(null)
const inviteRecords = ref<CampaignInviteRecord[]>([])
const selectedInviteRecord = ref<CampaignInviteRecord | null>(null)
const leaderboardAdjustForm = reactive({
  user_id: 0,
  valid_invite_delta: 0,
  recharge_delta_yuan: 0,
  reason: '',
})
const inviteAdjustForm = reactive({
  status: 'registered',
  recharge_yuan: 0,
  reason: '',
})

const poolMetrics = computed(() => [
  { label: t('admin.campaignRewards.confirmedPool'), value: formatCents(pool.value?.confirmed_pool_cents) },
  { label: t('admin.campaignRewards.pendingPool'), value: formatCents(pool.value?.pending_pool_cents) },
  { label: t('admin.campaignRewards.adjustmentPool'), value: formatCents(pool.value?.adjustment_total_cents) },
  { label: t('admin.campaignRewards.finalPool'), value: formatCents(pool.value?.final_pool_cents) },
])

const calculationRows = computed(() => calculation.value?.results?.slice(0, 20) || [])

const userDisplayMap = computed(() => {
  const map = new Map<number, string>()
  for (const row of leaderboard.value) {
    map.set(row.user_id, row.username || row.masked_email || `#${row.user_id}`)
  }
  return map
})

const rewardResultsHiddenCount = computed(() => Math.max((calculation.value?.results?.length || 0) - calculationRows.value.length, 0))

const backendNoFinalSettlementCodes = new Set(['CAMPAIGN_NO_FINAL_SETTLEMENT', 'CAMPAIGN_NOT_FOUND'])

const parsedRankWeights = computed(() => (
  createForm.rank_weights
    .split(',')
    .map(item => Number(item.trim()))
))

const rankWeightSum = computed(() => parsedRankWeights.value.reduce((sum, weight) => sum + weight, 0))

const createValidationMessage = computed(() => {
  if (!createForm.name.trim()) return t('admin.campaignRewards.createNameRequired')
  if (!createForm.start_at || !createForm.end_at) return t('admin.campaignRewards.createTimeRequired')
  if (!isValidDateRange(createForm.start_at, createForm.end_at)) return t('admin.campaignRewards.createTimeInvalid')
  if (!isFiniteNonNegative(createForm.recharge_threshold_yuan) || !isFiniteNonNegative(createForm.min_payout_yuan)) return t('admin.campaignRewards.createAmountInvalid')
  if (!isFiniteNonNegative(createForm.pool_injection_rate)) return t('admin.campaignRewards.createRateInvalid')
  if (!isFiniteNonNegative(createForm.rank_pool_ratio) || !isFiniteNonNegative(createForm.contribution_pool_ratio)) return t('admin.campaignRewards.createPoolRatioInvalid')
  if (!isApproximatelyEqual(createForm.rank_pool_ratio + createForm.contribution_pool_ratio, 1)) return t('admin.campaignRewards.createPoolRatioInvalid')
  if (!Number.isInteger(createForm.rank_reward_count) || createForm.rank_reward_count <= 0) return t('admin.campaignRewards.createRankCountInvalid')
  if (parsedRankWeights.value.length !== createForm.rank_reward_count) return t('admin.campaignRewards.createWeightsMismatch')
  if (parsedRankWeights.value.some(weight => !Number.isInteger(weight) || weight <= 0)) return t('admin.campaignRewards.createWeightsInvalid')
  if (rankWeightSum.value !== 100) return t('admin.campaignRewards.createWeightsSumInvalid')
  return ''
})

const editValidationMessage = computed(() => {
  if (!editForm.name.trim()) return t('admin.campaignRewards.createNameRequired')
  if (!editForm.start_at || !editForm.end_at) return t('admin.campaignRewards.createTimeRequired')
  if (!isValidDateRange(editForm.start_at, editForm.end_at)) return t('admin.campaignRewards.createTimeInvalid')
  if (hasInvalidOptionalDateTime([
    editForm.warmup_start_at,
    editForm.audit_start_at,
    editForm.audit_end_at,
    editForm.publicity_start_at,
    editForm.publicity_end_at,
    editForm.payout_due_at,
  ])) return t('admin.campaignRewards.editOptionalTimeInvalid')
  return ''
})

const hasFinalCalculation = computed(() => calculation.value?.calculation_status === 'final')

const canPublish = computed(() => selectedCampaign.value?.status === 'draft')
const canFreeze = computed(() => selectedCampaign.value?.status === 'active')
const canPreview = computed(() => !!selectedCampaign.value && selectedCampaign.value.status !== 'draft')
const canFinalize = computed(() => {
  const campaign = selectedCampaign.value
  if (!campaign || !['active', 'frozen', 'auditing', 'publicizing'].includes(campaign.status)) return false
  const endAt = new Date(campaign.end_at).getTime()
  return Number.isFinite(endAt) && now.value.getTime() >= endAt
})
const canPayout = computed(() => hasFinalCalculation.value && selectedCampaign.value?.status !== 'paid')
const canManualAdjust = computed(() => !!selectedCampaign.value && selectedCampaign.value.status !== 'paid')
const canSubmitLeaderboardAdjustment = computed(() => (
  canManualAdjust.value &&
  Number.isInteger(leaderboardAdjustForm.user_id) &&
  leaderboardAdjustForm.user_id > 0 &&
  (Number.isInteger(leaderboardAdjustForm.valid_invite_delta) || leaderboardAdjustForm.valid_invite_delta === 0) &&
  Number.isFinite(leaderboardAdjustForm.recharge_delta_yuan) &&
  (leaderboardAdjustForm.valid_invite_delta !== 0 || leaderboardAdjustForm.recharge_delta_yuan !== 0)
))
const canSubmitInviteAdjustment = computed(() => (
  !!selectedInviteRecord.value &&
  canManualAdjust.value &&
  Number.isFinite(inviteAdjustForm.recharge_yuan) &&
  inviteAdjustForm.recharge_yuan >= 0
))

const lifecycleLocale = computed(() => locale.value === 'zh' ? 'zh' : 'en')

const payoutDisabledHint = computed(() => t('admin.campaignRewards.payoutNeedsFinalLocal'))

const lifecycleStageIndex = computed(() => {
  const status = selectedCampaign.value?.status
  if (status === 'paid') return 5
  if (hasFinalCalculation.value || status === 'pending_payout' || status === 'auditing' || status === 'publicizing') return 4
  if (status === 'frozen') return 3
  if (status === 'active') return 2
  if (status === 'warmup') return 1
  return 0
})

const lifecycleSteps = computed(() => {
  const current = lifecycleStageIndex.value
  return [
    { key: 'draft', label: t('admin.campaignRewards.lifecycleDraft'), description: t('admin.campaignRewards.lifecycleDraftDesc') },
    { key: 'warmup', label: t('admin.campaignRewards.lifecycleWarmup'), description: t('admin.campaignRewards.lifecycleWarmupDesc') },
    { key: 'active', label: t('admin.campaignRewards.lifecycleActive'), description: t('admin.campaignRewards.lifecycleActiveDesc') },
    { key: 'frozen', label: t('admin.campaignRewards.lifecycleFrozen'), description: t('admin.campaignRewards.lifecycleFrozenDesc') },
    { key: 'settled', label: t('admin.campaignRewards.lifecycleSettled'), description: t('admin.campaignRewards.lifecycleSettledDesc') },
    { key: 'paid', label: t('admin.campaignRewards.lifecyclePaid'), description: t('admin.campaignRewards.lifecyclePaidDesc') },
  ].map((step, index) => ({
    ...step,
    current: index === current,
    done: index < current,
  }))
})

const lifecycleHint = computed(() => {
  if (!selectedCampaign.value) return ''
  if (selectedCampaign.value.status === 'draft') return t('admin.campaignRewards.lifecycleHintDraft')
  if (selectedCampaign.value.status === 'warmup') return t('admin.campaignRewards.lifecycleHintWarmup')
  if (selectedCampaign.value.status === 'active') return t('admin.campaignRewards.lifecycleHintActive')
  if (hasFinalCalculation.value) return t('admin.campaignRewards.lifecycleHintSettled')
  if (selectedCampaign.value.status === 'paid') return t('admin.campaignRewards.lifecycleHintPaid')
  return t('admin.campaignRewards.lifecycleHintDefault')
})

const selectedLifecycleState = computed(() => selectedCampaign.value ? getCampaignLifecycleState(selectedCampaign.value, now.value, lifecycleLocale.value) : getCampaignLifecycleState({
  id: 0,
  name: '',
  description: '',
  cover_url: '',
  rules_text: '',
  status: 'draft',
  start_at: '',
  end_at: '',
  created_at: '',
  updated_at: '',
}, now.value, lifecycleLocale.value))

const selectedCountdownText = computed(() => {
  if (selectedLifecycleState.value.countdownText) return selectedLifecycleState.value.countdownText
  if (selectedLifecycleState.value.hasExpiredTarget) return t('campaignRewards.lifecycle.expired')
  if (selectedLifecycleState.value.hasMissingTarget) return t('campaignRewards.lifecycle.targetMissing')
  return t('campaignRewards.lifecycle.noCountdown')
})

const selectedTargetText = computed(() => {
  if (!selectedLifecycleState.value.targetAt) return t('campaignRewards.lifecycle.targetPending')
  return t('campaignRewards.lifecycle.targetAt', { time: formatDateTime(selectedLifecycleState.value.targetAt) })
})

const selectedTimelineWarnings = computed(() => selectedCampaign.value ? getCampaignTimeWarnings(selectedCampaign.value) : [])

const timelineItems = computed(() => {
  const campaign = selectedCampaign.value
  if (!campaign) return []
  return [
    {
      key: 'warmup',
      label: t('admin.campaignRewards.timelineWarmup'),
      description: t('admin.campaignRewards.timelineWarmupDesc'),
      raw: campaign.warmup_start_at,
      active: selectedLifecycleState.value.phase === 'warmup',
    },
    {
      key: 'active',
      label: t('admin.campaignRewards.timelineActive'),
      description: t('admin.campaignRewards.timelineActiveDesc'),
      raw: campaign.start_at,
      active: selectedLifecycleState.value.phase === 'active',
    },
    {
      key: 'end',
      label: t('admin.campaignRewards.timelineEnd'),
      description: t('admin.campaignRewards.timelineEndDesc'),
      raw: campaign.end_at,
      active: false,
    },
    {
      key: 'audit-start',
      label: t('admin.campaignRewards.timelineAuditStart'),
      description: t('admin.campaignRewards.timelineAuditStartDesc'),
      raw: campaign.audit_start_at,
      active: false,
    },
    {
      key: 'audit-end',
      label: t('admin.campaignRewards.timelineAudit'),
      description: t('admin.campaignRewards.timelineAuditDesc'),
      raw: campaign.audit_end_at,
      active: selectedLifecycleState.value.phase === 'auditing',
    },
    {
      key: 'publicity-start',
      label: t('admin.campaignRewards.timelinePublicityStart'),
      description: t('admin.campaignRewards.timelinePublicityStartDesc'),
      raw: campaign.publicity_start_at,
      active: false,
    },
    {
      key: 'publicity-end',
      label: t('admin.campaignRewards.timelinePublicity'),
      description: t('admin.campaignRewards.timelinePublicityDesc'),
      raw: campaign.publicity_end_at,
      active: selectedLifecycleState.value.phase === 'publicizing',
    },
    {
      key: 'payout',
      label: t('admin.campaignRewards.timelinePayout'),
      description: t('admin.campaignRewards.timelinePayoutDesc'),
      raw: campaign.payout_due_at,
      active: selectedLifecycleState.value.phase === 'pending_payout',
    },
  ].map(item => ({
    ...item,
    missing: !item.raw,
    timeText: item.raw ? formatDateTime(item.raw) : t('admin.campaignRewards.timelineMissing'),
  })).filter(item => item.raw || item.active)
})

const adjustmentPreviewCents = computed(() => {
  const sign = adjustForm.adjustment_type === 'exception_deduction' ? -1 : 1
  return sign * yuanToCents(Math.abs(adjustAmountYuan.value || 0))
})

const adjustmentAfterCents = computed(() => (pool.value?.final_pool_cents || 0) + adjustmentPreviewCents.value)

const adjustmentRequiresReason = computed(() => adjustAmountYuan.value !== 0 && !adjustForm.reason.trim())

const riskDialogTitle = computed(() => {
  if (pendingRiskAction.value === 'publish') return t('admin.campaignRewards.confirmPublishTitle')
  if (pendingRiskAction.value === 'freeze') return t('admin.campaignRewards.confirmFreezeTitle')
  if (pendingRiskAction.value === 'finalize') return t('admin.campaignRewards.confirmFinalizeTitle')
  if (pendingRiskAction.value === 'payout') return t('admin.campaignRewards.confirmPayoutTitle')
  if (pendingRiskAction.value === 'deduct') return t('admin.campaignRewards.confirmDeductTitle')
  if (pendingRiskAction.value === 'delete') return t('admin.campaignRewards.confirmDeleteTitle')
  if (pendingRiskAction.value === 'copy') return t('admin.campaignRewards.confirmCopyTitle')
  return ''
})

const riskDialogMessage = computed(() => {
  if (pendingRiskAction.value === 'publish') return t('admin.campaignRewards.confirmPublishMessage')
  if (pendingRiskAction.value === 'freeze') return t('admin.campaignRewards.confirmFreezeMessage')
  if (pendingRiskAction.value === 'finalize') return t('admin.campaignRewards.confirmFinalizeMessage')
  if (pendingRiskAction.value === 'payout') return t('admin.campaignRewards.confirmPayoutMessage')
  if (pendingRiskAction.value === 'deduct') return t('admin.campaignRewards.confirmDeductMessage')
  if (pendingRiskAction.value === 'delete') return t('admin.campaignRewards.confirmDeleteMessage')
  if (pendingRiskAction.value === 'copy') return t('admin.campaignRewards.confirmCopyMessage')
  return ''
})

const riskDialogConfirmText = computed(() => {
  if (pendingRiskAction.value === 'publish') return t('admin.campaignRewards.publish')
  if (pendingRiskAction.value === 'freeze') return t('admin.campaignRewards.freeze')
  if (pendingRiskAction.value === 'finalize') return t('admin.campaignRewards.finalize')
  if (pendingRiskAction.value === 'payout') return t('admin.campaignRewards.payout')
  if (pendingRiskAction.value === 'deduct') return t('common.submit')
  if (pendingRiskAction.value === 'delete') return t('admin.campaignRewards.deleteOrArchive')
  if (pendingRiskAction.value === 'copy') return t('admin.campaignRewards.copyAsDraft')
  return t('common.confirm')
})

const riskDialogDanger = computed(() => pendingRiskAction.value === 'payout' || pendingRiskAction.value === 'deduct' || pendingRiskAction.value === 'delete')

const riskDialogMetrics = computed(() => {
  const items = [
    { label: t('admin.campaignRewards.campaign'), value: selectedCampaign.value?.name || '-' },
    { label: t('admin.campaignRewards.finalPool'), value: formatCents(pool.value?.final_pool_cents) },
  ]
  if (pendingRiskAction.value === 'payout') {
    items.push(
      { label: t('admin.campaignRewards.finalPayout'), value: formatCents(calculation.value?.total_final_payout_cents) },
      { label: t('admin.campaignRewards.withheld'), value: formatCents(calculation.value?.total_withheld_cents) },
      { label: t('admin.campaignRewards.payoutUsers'), value: String(calculation.value?.results?.filter(item => item.final_payout_amount_cents > 0).length || 0) },
      { label: t('admin.campaignRewards.batchNo'), value: calculation.value?.calculation_batch_no || '-' },
    )
  }
  if (pendingRiskAction.value === 'deduct') {
    items.push(
      { label: t('admin.campaignRewards.adjustDelta'), value: formatSignedCents(adjustmentPreviewCents.value) },
      { label: t('admin.campaignRewards.adjustAfter'), value: formatCents(adjustmentAfterCents.value) },
    )
  }
  if (pendingRiskAction.value === 'delete') {
    items.push(
      { label: t('admin.campaignRewards.status'), value: selectedCampaign.value ? statusLabel(selectedCampaign.value.status) : '-' },
      { label: t('admin.campaignRewards.leaderboardUsers'), value: String(leaderboard.value.length) },
      { label: t('admin.campaignRewards.deleteImpactHint'), value: t('admin.campaignRewards.deleteImpactValue') },
    )
  }
  if (pendingRiskAction.value === 'copy') {
    items.push(
      { label: t('admin.campaignRewards.status'), value: selectedCampaign.value ? statusLabel(selectedCampaign.value.status) : '-' },
      { label: t('admin.campaignRewards.copyImpactHint'), value: t('admin.campaignRewards.copyImpactValue') },
    )
  }
  return items
})

async function loadCampaigns(): Promise<void> {
  loading.value = true
  try {
    const resp = await adminAPI.campaigns.listCampaigns({ page: 1, page_size: 50 })
    campaigns.value = resp.items
    if (!selectedCampaign.value && resp.items.length > 0) {
      await selectCampaign(resp.items[0].id)
    } else if (selectedCampaign.value) {
      const fresh = resp.items.find(item => item.id === selectedCampaign.value?.id)
      selectedCampaign.value = fresh || selectedCampaign.value
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function selectCampaign(id: number): Promise<void> {
  try {
    const [campaign, poolResp, board] = await Promise.all([
      adminAPI.campaigns.getCampaign(id),
      adminAPI.campaigns.getPoolSummary(id),
      adminAPI.campaigns.getLeaderboard(id),
    ])
    selectedCampaign.value = campaign
    pool.value = poolResp
    leaderboard.value = board.items
    calculation.value = null
    await loadExistingFinalCalculation(id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
  }
}

async function loadExistingFinalCalculation(id: number): Promise<void> {
  try {
    calculation.value = await adminAPI.campaigns.getFinalRewardResults(id)
  } catch (error) {
    const code = extractApiErrorCode(error)
    if (!code || !backendNoFinalSettlementCodes.has(code)) {
      appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
    }
  }
}

function openLeaderboardAdjustment(row: CampaignLeaderboardRow | null): void {
  selectedLeaderboardRow.value = row
  leaderboardAdjustForm.user_id = row?.user_id ?? 0
  leaderboardAdjustForm.valid_invite_delta = 0
  leaderboardAdjustForm.recharge_delta_yuan = 0
  leaderboardAdjustForm.reason = ''
  leaderboardAdjustDialogOpen.value = true
}

async function openInviteRecords(row: CampaignLeaderboardRow): Promise<void> {
  if (!selectedCampaign.value) return
  selectedLeaderboardRow.value = row
  selectedInviteRecord.value = null
  inviteRecordsDialogOpen.value = true
  try {
    const resp = await adminAPI.campaigns.listInviterRecords(selectedCampaign.value.id, row.user_id)
    inviteRecords.value = resp.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
  }
}

function openInviteRecordAdjustment(record: CampaignInviteRecord): void {
  selectedInviteRecord.value = record
  inviteAdjustForm.status = record.status
  inviteAdjustForm.recharge_yuan = record.effective_recharge_amount_cents / 100
  inviteAdjustForm.reason = record.audit_note || ''
}

async function submitLeaderboardAdjustment(): Promise<void> {
  if (!selectedCampaign.value || !canSubmitLeaderboardAdjustment.value) return
  manualSubmitting.value = true
  try {
    await adminAPI.campaigns.addLeaderboardAdjustment(selectedCampaign.value.id, {
      user_id: leaderboardAdjustForm.user_id,
      valid_invite_delta: leaderboardAdjustForm.valid_invite_delta,
      recharge_amount_delta_cents: yuanToCents(leaderboardAdjustForm.recharge_delta_yuan),
      reason: leaderboardAdjustForm.reason,
    })
    leaderboardAdjustDialogOpen.value = false
    calculation.value = null
    await selectCampaign(selectedCampaign.value.id)
    appStore.showSuccess(t('admin.campaignRewards.manualAdjusted'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.actionFailed')))
  } finally {
    manualSubmitting.value = false
  }
}

async function submitInviteRecordAdjustment(): Promise<void> {
  if (!selectedCampaign.value || !selectedInviteRecord.value || !selectedLeaderboardRow.value || !canSubmitInviteAdjustment.value) return
  manualSubmitting.value = true
  try {
    await adminAPI.campaigns.adjustInviteRecord(selectedCampaign.value.id, selectedInviteRecord.value.id, {
      status: inviteAdjustForm.status,
      effective_recharge_amount_cents: yuanToCents(inviteAdjustForm.recharge_yuan),
      reason: inviteAdjustForm.reason,
    })
    const resp = await adminAPI.campaigns.listInviterRecords(selectedCampaign.value.id, selectedLeaderboardRow.value.user_id)
    inviteRecords.value = resp.items
    selectedInviteRecord.value = null
    calculation.value = null
    await selectCampaign(selectedCampaign.value.id)
    appStore.showSuccess(t('admin.campaignRewards.manualAdjusted'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.actionFailed')))
  } finally {
    manualSubmitting.value = false
  }
}

function openCreateDialog(): void {
  const start = new Date(Date.now() + 60 * 60 * 1000)
  const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
  createForm.name = t('admin.campaignRewards.defaultName')
  createForm.description = t('admin.campaignRewards.defaultDescription')
  createForm.rules_text = t('admin.campaignRewards.defaultRules')
  createForm.start_at = toDateTimeLocal(start)
  createForm.end_at = toDateTimeLocal(end)
  createForm.recharge_threshold_yuan = 20
  createForm.pool_injection_rate = 0.1
  createForm.pool_injection_scope = 'invitees_only'
  createForm.rank_pool_ratio = 0.8
  createForm.contribution_pool_ratio = 0.2
  createForm.rank_reward_count = 10
  createForm.rank_weights = '30,20,15,10,8,6,4,3,2,2'
  createForm.min_payout_yuan = 1
  createDialogOpen.value = true
}

function openEditDialog(): void {
  if (!selectedCampaign.value) return
  const campaign = selectedCampaign.value
  editForm.name = campaign.name
  editForm.description = campaign.description || ''
  editForm.rules_text = campaign.rules_text || ''
  editForm.warmup_start_at = toDateTimeLocalFromRaw(campaign.warmup_start_at)
  editForm.start_at = toDateTimeLocalFromRaw(campaign.start_at)
  editForm.end_at = toDateTimeLocalFromRaw(campaign.end_at)
  editForm.audit_start_at = toDateTimeLocalFromRaw(campaign.audit_start_at)
  editForm.audit_end_at = toDateTimeLocalFromRaw(campaign.audit_end_at)
  editForm.publicity_start_at = toDateTimeLocalFromRaw(campaign.publicity_start_at)
  editForm.publicity_end_at = toDateTimeLocalFromRaw(campaign.publicity_end_at)
  editForm.payout_due_at = toDateTimeLocalFromRaw(campaign.payout_due_at)
  editDialogOpen.value = true
}

async function submitCreateCampaign(): Promise<void> {
  if (createValidationMessage.value) {
    appStore.showError(createValidationMessage.value)
    return
  }
  createSubmitting.value = true
  try {
    const resp = await adminAPI.campaigns.createCampaign({
      name: createForm.name,
      description: createForm.description,
      rules_text: createForm.rules_text,
      start_at: new Date(createForm.start_at).toISOString(),
      end_at: new Date(createForm.end_at).toISOString(),
      initial_bonus_cents: 0,
      recharge_threshold_cents: yuanToCents(createForm.recharge_threshold_yuan),
      allow_accumulated_recharge: true,
      pool_injection_rate: createForm.pool_injection_rate,
      pool_injection_scope: createForm.pool_injection_scope,
      rank_pool_ratio: createForm.rank_pool_ratio,
      contribution_pool_ratio: createForm.contribution_pool_ratio,
      rank_reward_count: createForm.rank_reward_count,
      rank_weights: parsedRankWeights.value,
      min_payout_amount_cents: yuanToCents(createForm.min_payout_yuan),
    })
    appStore.showSuccess(t('admin.campaignRewards.created'))
    createDialogOpen.value = false
    await loadCampaigns()
    await selectCampaign(resp.campaign.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.createFailed')))
  } finally {
    createSubmitting.value = false
  }
}

async function submitEditCampaign(): Promise<void> {
  if (!selectedCampaign.value) return
  if (editValidationMessage.value) {
    appStore.showError(editValidationMessage.value)
    return
  }
  editSubmitting.value = true
  try {
    const updated = await adminAPI.campaigns.updateCampaign(selectedCampaign.value.id, buildCampaignUpdatePayload())
    selectedCampaign.value = updated
    appStore.showSuccess(t('admin.campaignRewards.updated'))
    editDialogOpen.value = false
    await loadCampaigns()
    await selectCampaign(updated.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.updateFailed')))
  } finally {
    editSubmitting.value = false
  }
}

function buildCampaignUpdatePayload(): CampaignUpdateRequest {
  return {
    name: editForm.name,
    description: editForm.description,
    rules_text: editForm.rules_text,
    warmup_start_at: optionalDateTimePayload(editForm.warmup_start_at),
    start_at: dateTimeLocalToISOString(editForm.start_at),
    end_at: dateTimeLocalToISOString(editForm.end_at),
    audit_start_at: optionalDateTimePayload(editForm.audit_start_at),
    audit_end_at: optionalDateTimePayload(editForm.audit_end_at),
    publicity_start_at: optionalDateTimePayload(editForm.publicity_start_at),
    publicity_end_at: optionalDateTimePayload(editForm.publicity_end_at),
    payout_due_at: optionalDateTimePayload(editForm.payout_due_at),
  }
}

async function publishSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  pendingRiskAction.value = 'publish'
}

async function executePublish(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    selectedCampaign.value = await adminAPI.campaigns.publishCampaign(selectedCampaign.value!.id)
    await loadCampaigns()
  }, t('admin.campaignRewards.published'))
}

async function freezeSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  pendingRiskAction.value = 'freeze'
}

async function executeFreeze(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.freezeLeaderboard(selectedCampaign.value!.id)
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.frozen'))
}

async function previewRewards(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    calculation.value = await adminAPI.campaigns.recalculateRewards(selectedCampaign.value!.id, 'preview')
  }, t('admin.campaignRewards.previewed'))
}

async function finalizeRewards(): Promise<void> {
  if (!selectedCampaign.value) return
  pendingRiskAction.value = 'finalize'
}

async function executeFinalizeRewards(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    const finalCalculation = await adminAPI.campaigns.recalculateRewards(selectedCampaign.value!.id, 'final')
    await selectCampaign(selectedCampaign.value!.id)
    calculation.value = finalCalculation
  }, t('admin.campaignRewards.finalized'))
}

async function payoutSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  if (!hasFinalCalculation.value) {
    appStore.showError(t('admin.campaignRewards.payoutNeedsFinal'))
    return
  }
  pendingRiskAction.value = 'payout'
}

async function deleteSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  pendingRiskAction.value = 'delete'
}

async function copySelected(): Promise<void> {
  if (!selectedCampaign.value) return
  pendingRiskAction.value = 'copy'
}

async function executeCopyCampaign(): Promise<void> {
  if (!selectedCampaign.value) return
  try {
    const result = await adminAPI.campaigns.copyCampaign(selectedCampaign.value.id)
    appStore.showSuccess(t('admin.campaignRewards.copiedAsDraft'))
    await loadCampaigns()
    await selectCampaign(result.campaign.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.copyFailed')))
  }
}

async function executeDeleteCampaign(): Promise<void> {
  if (!selectedCampaign.value) return
  const deletingID = selectedCampaign.value.id
  try {
    const result = await adminAPI.campaigns.deleteCampaign(deletingID)
    if (result.action === 'deleted') {
      selectedCampaign.value = null
      pool.value = null
      leaderboard.value = []
      calculation.value = null
      appStore.showSuccess(t('admin.campaignRewards.deleted'))
      await loadCampaigns()
      if (campaigns.value.length > 0) {
        await selectCampaign(campaigns.value[0].id)
      }
      return
    }
    if (result.campaign) {
      selectedCampaign.value = result.campaign
    }
    await loadCampaigns()
    await selectCampaign(deletingID)
    appStore.showSuccess(t('admin.campaignRewards.archived'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.deleteFailed')))
  }
}

async function executePayout(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.payoutCampaign(selectedCampaign.value!.id)
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.paid'))
}

async function submitPoolAdjustment(): Promise<void> {
  if (!selectedCampaign.value || adjustAmountYuan.value === 0) return
  if (adjustmentRequiresReason.value) {
    appStore.showError(t('admin.campaignRewards.adjustReasonRequired'))
    return
  }
  if (adjustForm.adjustment_type === 'exception_deduction' && pendingRiskAction.value !== 'deduct') {
    pendingRiskAction.value = 'deduct'
    return
  }
  await executePoolAdjustment()
}

async function executePoolAdjustment(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.addPoolAdjustment(selectedCampaign.value!.id, {
      adjustment_type: adjustForm.adjustment_type,
      amount_cents: adjustmentPreviewCents.value,
      reason: adjustForm.reason,
    })
    adjustAmountYuan.value = 0
    adjustForm.reason = ''
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.adjusted'))
}

async function confirmRiskAction(): Promise<void> {
  const action = pendingRiskAction.value
  pendingRiskAction.value = null
  if (action === 'publish') {
    await executePublish()
  } else if (action === 'freeze') {
    await executeFreeze()
  } else if (action === 'finalize') {
    await executeFinalizeRewards()
  } else if (action === 'payout') {
    await executePayout()
  } else if (action === 'deduct') {
    await executePoolAdjustment()
  } else if (action === 'delete') {
    await executeDeleteCampaign()
  } else if (action === 'copy') {
    await executeCopyCampaign()
  }
}

async function submitVersion(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.createConfigVersion(selectedCampaign.value!.id, {
      version_scope: 'threshold_adjustment',
      effective_at: new Date().toISOString(),
      recharge_threshold_cents: yuanToCents(versionThresholdYuan.value),
      pool_injection_rate: versionRate.value,
      pool_injection_scope: versionPoolInjectionScope.value,
      allow_accumulated_recharge: true,
      change_reason: versionReason.value || t('admin.campaignRewards.versionReasonDefault'),
    })
    versionReason.value = ''
  }, t('admin.campaignRewards.versionCreated'))
}

async function runAction(action: () => Promise<void>, success: string): Promise<void> {
  try {
    await action()
    appStore.showSuccess(success)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.actionFailed')))
  }
}

function yuanToCents(value: number): number {
  return Math.round((value || 0) * 100)
}

function poolInjectionScopeLabel(scope: 'invitees_only' | 'all_users'): string {
  return scope === 'all_users'
    ? t('admin.campaignRewards.poolInjectionScopeAllUsers')
    : t('admin.campaignRewards.poolInjectionScopeInviteesOnly')
}

function isFiniteNonNegative(value: number): boolean {
  return Number.isFinite(value) && value >= 0
}

function isApproximatelyEqual(left: number, right: number): boolean {
  return Math.abs(left - right) < 0.000001
}

function isValidDateRange(startRaw: string, endRaw: string): boolean {
  const start = new Date(startRaw).getTime()
  const end = new Date(endRaw).getTime()
  return Number.isFinite(start) && Number.isFinite(end) && start < end
}

function hasInvalidOptionalDateTime(values: string[]): boolean {
  return values.some(value => value.trim() !== '' && !Number.isFinite(new Date(value).getTime()))
}

function formatCents(value?: number | null): string {
  return `¥${((value || 0) / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatSignedCents(value: number): string {
  if (value === 0) return formatCents(0)
  return `${value > 0 ? '+' : '-'}${formatCents(Math.abs(value))}`
}

function formatSignedNumber(value: number): string {
  if (value === 0) return '0'
  return `${value > 0 ? '+' : ''}${value}`
}

function formatManualDelta(row: CampaignLeaderboardRow): string {
  return `${formatSignedNumber(row.manual_valid_invite_delta)} / ${formatSignedCents(row.manual_recharge_amount_delta_cents)}`
}

function inviteStatusLabel(status: string): string {
  return t(`campaignRewards.inviteStatuses.${status}`)
}

function formatDateTime(raw?: string | null): string {
  if (!raw) return '-'
  return new Date(raw).toLocaleString()
}

function toDateTimeLocal(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toDateTimeLocalFromRaw(raw?: string | null): string {
  if (!raw) return ''
  const date = new Date(raw)
  if (!Number.isFinite(date.getTime())) return ''
  return toDateTimeLocal(date)
}

function dateTimeLocalToISOString(value: string): string {
  return new Date(value).toISOString()
}

function optionalDateTimePayload(value: string): string | null {
  const trimmed = value.trim()
  return trimmed ? dateTimeLocalToISOString(trimmed) : null
}

function statusLabel(status: string): string {
  return t(`campaignRewards.statuses.${status}`, status)
}

function statusClass(status: string): string {
  if (status === 'active') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'draft' || status === 'warmup') return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  if (status === 'paid') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function campaignLifecycle(campaign: Campaign) {
  return getCampaignLifecycleState(campaign, now.value, lifecycleLocale.value)
}

function campaignLifecycleSummary(campaign: Campaign): string {
  const state = campaignLifecycle(campaign)
  const label = t(state.labelKey)
  if (state.countdownText) return `${label} · ${state.countdownText}`
  if (state.hasMissingTarget) return `${label} · ${t('campaignRewards.lifecycle.targetMissing')}`
  return label
}

onMounted(() => {
  clockTimer = setInterval(() => {
    now.value = new Date()
  }, 1000)
  void loadCampaigns()
})

onUnmounted(() => {
  if (clockTimer) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>
