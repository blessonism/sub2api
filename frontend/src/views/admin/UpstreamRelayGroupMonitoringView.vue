<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ tM('title') }}</h1>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-2 text-sm text-gray-500 dark:text-gray-400">
            <span class="inline-flex items-center gap-1.5" :title="lastRefreshedFreshness.absolute">
              <span class="text-gray-500 dark:text-gray-400">{{ tM('lastRefreshed') }}:</span>
              <span class="font-medium text-gray-700 tabular-nums dark:text-gray-200">{{ lastRefreshedFreshness.relative }}</span>
              <span
                :class="['inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold', freshnessBadgeClass(lastRefreshedFreshness.tone)]"
                data-testid="freshness-badge-refreshed"
              >{{ lastRefreshedFreshness.label }}</span>
            </span>
            <span class="inline-flex items-center gap-1.5" :title="lastSyncedFreshness.absolute">
              <span class="text-gray-500 dark:text-gray-400">{{ tM('lastSynced') }}:</span>
              <span class="font-medium text-gray-700 tabular-nums dark:text-gray-200">{{ lastSyncedFreshness.relative }}</span>
              <span
                :class="['inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold', freshnessBadgeClass(lastSyncedFreshness.tone)]"
                data-testid="freshness-badge-synced"
              >{{ lastSyncedFreshness.label }}</span>
            </span>
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ tM('statsNote') }}</span>
          </div>
        </div>
        <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end lg:justify-end">
          <div class="space-y-1">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('actionGroups.data') }}</div>
            <div class="flex flex-wrap justify-start gap-2 lg:justify-end">
              <AutoRefreshButton
                :enabled="autoRefreshEnabled"
                :interval-seconds="autoRefreshInterval"
                :countdown="autoRefreshCountdown"
                :intervals="AUTO_REFRESH_INTERVALS"
                button-class="btn btn-secondary inline-flex items-center gap-2"
                @update:enabled="autoRefreshEnabled = $event"
                @update:interval="autoRefreshInterval = $event"
              />
              <button
                class="btn btn-secondary inline-flex items-center gap-2"
                type="button"
                :disabled="loading || refreshingMetrics || connectors.length === 0"
                :title="tM('manualRefreshHint')"
                @click="() => refreshMonitoringData()"
              >
                <Icon name="refresh" size="sm" />
                {{ refreshingMetrics ? tM('refreshingMonitoring') : tM('refreshMonitoring') }}
              </button>
            </div>
          </div>
          <div class="space-y-1">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('actionGroups.recommendation') }}</div>
            <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="generating" @click="generateRun">
              <Icon name="chart" size="sm" />
              {{ generating ? tM('generating') : tM('generateSuggestions') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="error" data-testid="page-error" class="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
        <span class="rounded-md bg-white/70 px-2 py-0.5 text-xs font-medium dark:bg-black/20">{{ pageFeedbackSource }}</span>
        <div class="min-w-0 flex-1 break-words">
          <div>{{ pageErrorMessage }}</div>
          <details v-if="pageErrorTechnicalDetail" class="mt-2 text-xs text-red-600/80 dark:text-red-300/80">
            <summary class="cursor-pointer select-none">{{ tM('metricsRefresh.technicalDetails') }}</summary>
            <div class="mt-1 font-mono">{{ pageErrorTechnicalDetail }}</div>
          </details>
        </div>
        <button type="button" class="rounded p-1 hover:bg-white/70 dark:hover:bg-black/20" :aria-label="tM('operationResult.dismiss')" @click="error = ''"><Icon name="x" size="sm" /></button>
      </div>
      <div v-if="successMessage" data-testid="page-success" class="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200">
        <span class="rounded-md bg-white/70 px-2 py-0.5 text-xs font-medium dark:bg-black/20">{{ pageFeedbackSource }}</span>
        <span class="min-w-0 flex-1 break-words">{{ successMessage }}</span>
        <button type="button" class="rounded p-1 hover:bg-white/70 dark:hover:bg-black/20" :aria-label="tM('operationResult.dismiss')" @click="successMessage = ''"><Icon name="x" size="sm" /></button>
      </div>
      <OperationResultPanel
        v-if="monitoringOperationFeedback"
        v-bind="monitoringOperationFeedback"
        data-testid="monitoring-operation-result"
        @action="showMonitoringConnectorDetails"
        @dismiss="metricsRefreshResult = null; monitoringRefreshRequestError = ''"
      />

      <div
        data-testid="runner-status-bar"
        class="rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ tM('runnerStatus.title') }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ tM('runnerStatus.subtitle') }}</div>
          </div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:min-w-[820px] xl:grid-cols-4">
            <div
              v-for="card in runnerStatusCards"
              :key="card.key"
              :data-testid="`runner-status-card-${card.key}`"
              class="rounded-md border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ card.label }}</span>
                <span
                  class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold"
                  :class="{
                    'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-gray-300': card.stateTone === 'disabled',
                    'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-200': card.stateTone === 'running',
                    'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200': card.stateTone === 'success',
                    'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200': card.stateTone === 'failed',
                    'bg-gray-100 text-gray-600 dark:bg-dark-700/60 dark:text-gray-300': card.stateTone === 'idle'
                  }"
                >
                  {{ card.stateLabel }}
                </span>
              </div>
              <div class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{{ card.hintText }}</div>
              <details v-if="card.technicalDetails.length > 0" class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">
                <summary class="cursor-pointer select-none">{{ tM('metricsRefresh.technicalDetails') }}</summary>
                <div v-for="detail in card.technicalDetails" :key="detail" class="mt-1 break-words font-mono">{{ detail }}</div>
              </details>
              <div v-if="card.enabled" class="mt-0.5 text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                {{ tM('runnerStatus.hints.lastFinished', { time: card.lastFinishedAt ? formatDate(card.lastFinishedAt) : '-' }) }}
                <span v-if="card.intervalMinutes > 0"> · {{ tM('runnerStatus.hints.interval', { minutes: card.intervalMinutes }) }}</span>
              </div>
              <div v-if="card.toggleKey" class="mt-1.5 flex items-center justify-between border-t border-dashed border-gray-200 pt-1.5 dark:border-dark-700">
                <span class="text-[11px] text-gray-500 dark:text-gray-400">
                  {{ tM('runnerStatus.toggle.label') }} · {{ card.autoEnabled ? tM('runnerStatus.toggle.enabled') : tM('runnerStatus.toggle.disabled') }}
                </span>
                <button
                  type="button"
                  role="switch"
                  :aria-checked="card.autoEnabled"
                  :aria-label="card.autoEnabled ? tM('runnerStatus.toggle.disableAria', { job: card.label }) : tM('runnerStatus.toggle.enableAria', { job: card.label })"
                  :disabled="card.toggleDisabled"
                  :data-testid="`runner-toggle-${card.key}`"
                  class="relative inline-flex h-4 w-7 shrink-0 items-center rounded-full transition disabled:cursor-not-allowed disabled:opacity-60"
                  :class="card.autoEnabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
                  @click="toggleRunnerAutoJob(card.toggleKey)"
                >
                  <span
                    class="inline-block h-3 w-3 transform rounded-full bg-white shadow transition"
                    :class="card.autoEnabled ? 'translate-x-3.5' : 'translate-x-0.5'"
                  />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <button
          v-for="item in overviewCards"
          :key="item.key"
          type="button"
          class="card p-4 text-left transition hover:border-primary-200 hover:bg-primary-50/40 dark:hover:border-primary-900/50 dark:hover:bg-primary-950/20"
          @click="activeSection = item.section"
        >
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white" :data-testid="`overview-card-value-${item.key}`">{{ item.value }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.hint }}</div>
        </button>
      </section>

      <div class="flex flex-col gap-3 border-b border-gray-200 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex flex-wrap gap-2">
          <button
            v-for="section in sections"
            :key="section.key"
            type="button"
            class="inline-flex items-center gap-2 rounded-t-lg px-4 py-2 text-sm font-medium transition"
            :class="activeSection === section.key ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
            @click="activeSection = section.key"
          >
            <span>{{ section.label }}</span>
            <span
              v-if="section.badge !== undefined"
              class="inline-flex min-w-5 items-center justify-center rounded-full px-1.5 py-0.5 text-[11px] font-semibold leading-4 tabular-nums"
              :class="activeSection === section.key ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/50 dark:text-primary-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
            >
              {{ section.badge }}
            </span>
          </button>
        </div>
        <div class="flex flex-wrap gap-2 pb-3 lg:pb-2">
          <button v-if="activeSection === 'candidates'" class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="bulkProbing || enabledCandidateCount === 0" @click="probeAllCandidates">
            <Icon name="play" size="sm" />
            {{ bulkProbing ? tM('candidates.probingAll') : tM('candidates.probeAll') }}
          </button>
          <button v-if="activeSection === 'candidates'" class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateCandidate">
            <Icon name="plus" size="sm" />
            {{ tM('candidates.newCandidate') }}
          </button>
          <button v-if="activeSection === 'connectors'" class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateConnector">
            <Icon name="plus" size="sm" />
            {{ tM('connectors.newConnector') }}
          </button>
          <button v-if="activeSection === 'snapshotChanges'" class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="bulkSyncing || connectors.length === 0" @click="syncAllConnectors">
            <Icon name="refresh" size="sm" />
            {{ bulkSyncing ? tM('fetchingSnapshots') : tM('fetchSnapshots') }}
          </button>
          <button v-if="activeSection === 'monitoring'" class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="savingMonitoringPolicy" @click="saveMonitoringPolicy">
            <Icon name="save" size="sm" />
            {{ savingMonitoringPolicy ? tM('monitoring.saving') : tM('monitoring.save') }}
          </button>
          <button v-if="activeSection === 'policy'" class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="previewLoading" @click="previewPolicy">
            <Icon name="chart" size="sm" />
            {{ previewLoading ? tM('policy.previewing') : tM('policy.preview') }}
          </button>
          <button v-if="activeSection === 'policy'" class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="savingPolicy" @click="savePolicy">
            <Icon name="save" size="sm" />
            {{ savingPolicy ? tM('policy.saving') : tM('policy.save') }}
          </button>
        </div>
      </div>

      <section v-if="activeSection === 'candidates'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('candidates.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('candidates.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('candidates.count', { total: candidates.length, enabled: enabledCandidateCount }) }}</span>
            <button
              v-if="incompleteCandidateCount > 0"
              type="button"
              class="btn btn-secondary inline-flex items-center gap-1.5 px-2.5 py-1 text-xs"
              :class="candidateConfigurationOnlyIncomplete ? 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200' : ''"
              data-testid="candidate-incomplete-filter"
              @click="candidateConfigurationOnlyIncomplete = !candidateConfigurationOnlyIncomplete"
            >
              <Icon name="filter" size="xs" />
              {{ candidateConfigurationOnlyIncomplete ? tM('candidates.showAll') : tM('candidates.filterIncomplete', { count: incompleteCandidateCount }) }}
            </button>
          </div>
        </div>
        <div
          v-if="candidateProbeFeedback"
          data-testid="candidate-probe-feedback"
          class="border-b px-4 py-3 text-sm"
          :class="candidateProbeFeedbackPanelClass"
        >
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <Icon :name="candidateProbeFeedbackIcon" size="sm" :class="candidateProbeFeedbackIconClass" />
                <span class="font-semibold">{{ candidateProbeFeedbackTitle }}</span>
                <span class="rounded-md bg-white/60 px-2 py-0.5 text-xs font-medium dark:bg-black/20">
                  {{ candidateProbeFeedback.status === 'running' ? tM('candidates.probeStatusRunning') : candidateProbeFeedback.success ? tM('candidates.probeStatusSuccess') : tM('candidates.probeStatusFailed') }}
                </span>
              </div>
              <div class="mt-1 text-xs leading-5">{{ candidateProbeFeedbackDetail }}</div>
              <div v-if="candidateProbeFeedbackError" class="mt-2 break-words rounded-md bg-white/70 px-3 py-2 text-xs leading-5 dark:bg-black/20">
                {{ candidateProbeFeedbackError }}
                <details v-if="candidateProbeFeedbackRawError" class="mt-2 text-gray-500 dark:text-gray-400">
                  <summary class="cursor-pointer select-none">{{ tM('metricsRefresh.technicalDetails') }}</summary>
                  <div class="mt-1 font-mono">{{ candidateProbeFeedbackRawError }}</div>
                </details>
              </div>
            </div>
            <button class="btn btn-secondary whitespace-nowrap px-3 py-1.5 text-xs" type="button" @click="candidateProbeFeedback = null">
              {{ tM('candidates.dismissProbeFeedback') }}
            </button>
          </div>
        </div>
        <div v-if="candidateBulkOperationFeedback" class="border-b px-4 py-3" data-testid="candidate-bulk-probe-feedback">
          <OperationResultPanel v-bind="candidateBulkOperationFeedback" @dismiss="candidateBulkProbeFeedback = null" />
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1320px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colCandidate') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colMapping') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colTodayUsage') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colRate') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colHealth') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colPriority') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="candidate in filteredCandidates" :key="candidate.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">{{ tM('candidates.identity', { candidate: candidate.id, account: candidate.account_id }) }} · {{ candidate.account_name || '-' }}</span>
                    <span :class="candidate.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'" class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium">
                      {{ candidate.enabled ? tM('candidates.enabled') : tM('candidates.disabled') }}
                    </span>
                    <span v-if="candidateConfigurationIncomplete(candidate)" class="inline-flex rounded-md bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                      {{ tM('candidates.configurationIncomplete') }}
                    </span>
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ candidate.account_platform || '-' }} · {{ candidate.probe_model }} · {{ candidate.probe_protocol }}</div>
                </td>
                <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <div>{{ candidate.connector_name || `Connector #${candidate.connector_id}` }}</div>
                  <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ candidateMappingLabel(candidate) }}</div>
                  <div class="mt-0.5 font-mono text-gray-400 dark:text-gray-500">{{ candidate.upstream_group_id }}</div>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ candidateTodayUsageCostLabel(candidate) }}</div>
                  <div class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ candidateTodayUsageMetaLabel(candidate) }}</div>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ candidateRateLabel(candidate) }}</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span :class="candidateHealthClass(candidate)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ candidateHealthLabel(candidate) }}</span>
                    <span v-if="candidate.latest_probe" class="text-xs text-gray-500 dark:text-gray-400">{{ candidate.latest_probe.latency_ms ?? '-' }}ms</span>
                  </div>
                  <div v-if="candidateLatestProbeErrorSummary(candidate)" class="mt-1 max-w-[240px] truncate text-xs text-red-500 dark:text-red-300">
                    {{ candidateLatestProbeErrorSummary(candidate) }}
                  </div>
                  <div class="mt-1.5"><HealthRateBar :rate="candidate.health?.success_rate ?? null" /></div>
                  <button class="mt-1 text-xs text-primary-600 hover:underline dark:text-primary-400" type="button" @click="healthDialogCandidate = candidate">{{ tM('candidates.healthDetail') }}</button>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="tabular-nums text-gray-900 dark:text-white">{{ candidate.current_priority ?? '-' }}</div>
                  <div v-if="candidatePendingSuggestion(candidate)" class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-300">
                    {{ tM('candidates.pendingSuggestion', { action: suggestionActionLabel(candidatePendingSuggestion(candidate)!) }) }}
                    <span class="text-gray-400 dark:text-gray-500">({{ suggestionChangeLabel(candidatePendingSuggestion(candidate)!) }})</span>
                  </div>
                  <div v-else class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ tM('candidates.noPendingSuggestion') }}</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editCandidate(candidate)">{{ tM('candidates.edit') }}</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="bulkProbing || probingId === candidate.id || togglingCandidateId === candidate.id" @click="toggleCandidateEnabled(candidate)">
                      {{ candidate.enabled ? tM('candidates.disable') : tM('candidates.enable') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="reuseCandidateAccount(candidate)">{{ tM('candidates.reuseAccount') }}</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="bulkProbing || probingId === candidate.id" @click="probe(candidate)">
                      {{ probingId === candidate.id ? tM('candidates.probing') : tM('candidates.probe') }}
                    </button>
                    <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeCandidate(candidate)">{{ tM('candidates.delete') }}</button>
                  </div>
                </td>
              </tr>
              <tr v-if="filteredCandidates.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ tM('candidates.noFilteredCandidates') }}</td>
              </tr>
              <tr v-if="candidates.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">{{ tM('candidates.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="activeSection === 'connectors'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('connectors.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('connectors.description') }}</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('connectors.count', { n: connectors.length }) }}</span>
        </div>
        <div v-if="metricsRefreshResult" data-testid="metrics-refresh-summary" class="border-b border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span :class="metricsRefreshSummaryClass(metricsRefreshSummary)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                  {{ tM('metricsRefresh.summaryTitle') }}
                </span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ tM('metricsRefresh.summaryCounts', { success: metricsRefreshSummary.success, partial: metricsRefreshSummary.partial, failed: metricsRefreshSummary.failed }) }}
                </span>
              </div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ tM('metricsRefresh.summaryMeta', { balance: metricsRefreshSummary.balance, total: metricsRefreshSummary.total, usage: metricsRefreshSummary.usage, missing: metricsRefreshSummary.missingGroups }) }}
                <span class="ml-2">{{ tM('metricsRefresh.updatedAt', { time: formatDate(metricsRefreshResult.updatedAt) }) }}</span>
              </div>
            </div>
            <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" @click="metricsRefreshResult.expanded = !metricsRefreshResult.expanded">
              {{ metricsRefreshResult.expanded ? tM('metricsRefresh.hideDetails') : tM('metricsRefresh.showDetails') }}
            </button>
          </div>
          <div v-if="metricsRefreshResult.expanded" data-testid="metrics-refresh-details" class="mt-3 grid gap-2">
            <div
              v-for="item in metricsRefreshResult.items"
              :key="item.connector_id"
              class="rounded-md border border-gray-100 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">{{ monitoringRefreshItemConnectorLabel(item) }}</div>
                  <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ monitoringSnapshotDetailLabel(item) }}</span>
                    <span>{{ monitoringBalanceDetailLabel(item) }}</span>
                    <span>{{ monitoringUsageDetailLabel(item) }}</span>
                  </div>
                </div>
                <span :class="metricsRefreshStatusClass(item.status)" class="inline-flex w-fit rounded-md px-2 py-1 text-xs font-medium">
                  {{ metricsRefreshStatusLabel(item.status) }}
                </span>
              </div>
              <div
                v-if="monitoringRefreshItemGuidance(item)"
                data-testid="metrics-refresh-guidance"
                class="mt-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div class="font-semibold">{{ monitoringRefreshItemGuidance(item)?.summary }}</div>
                    <div class="mt-1">{{ monitoringRefreshItemGuidance(item)?.guidance }}</div>
                  </div>
                  <button
                    v-if="monitoringRefreshItemGuidance(item)?.action"
                    class="btn btn-secondary w-fit px-2 py-1 text-xs"
                    type="button"
                    @click="handleMonitoringIssueAction(item)"
                  >
                    {{ monitoringRefreshItemGuidance(item)?.actionLabel }}
                  </button>
                </div>
                <details v-if="monitoringRefreshItemGuidance(item)?.rawDetail" class="mt-2 text-gray-500 dark:text-gray-400">
                  <summary class="cursor-pointer select-none">{{ tM('metricsRefresh.technicalDetails') }}</summary>
                  <div class="mt-1 break-words font-mono">{{ monitoringRefreshItemGuidance(item)?.rawDetail }}</div>
                </details>
              </div>
              <div v-else-if="monitoringRefreshItemStandaloneError(item)" class="mt-2 text-xs text-red-600 dark:text-red-300">
                {{ monitoringRefreshItemStandaloneError(item) }}
              </div>
              <div v-if="monitoringMissingGroups(item).length > 0" class="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
                <span class="text-gray-500 dark:text-gray-400">{{ tM('metricsRefresh.affectedGroups') }}</span>
                <span
                  v-for="group in monitoringMissingGroups(item).slice(0, 6)"
                  :key="`${item.connector_id}-${group.upstream_group_id}`"
                  class="inline-flex rounded bg-amber-50 px-2 py-1 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200"
                >
                  {{ group.name || group.upstream_group_id }} · {{ metricsMissingGroupReasonLabel(group.reason) }}
                </span>
              </div>
            </div>
          </div>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1120px] table-fixed text-sm">
            <colgroup>
              <col class="w-[26%]" />
              <col class="w-[12%]" />
              <col class="w-[20%]" />
              <col class="w-[16%]" />
              <col class="w-[13%]" />
              <col class="w-[13%]" />
            </colgroup>
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colName') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colStatus') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colCredentials') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colVerifiedSynced') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('connectors.colAccountBalance') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('connectors.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <template v-for="connector in connectors" :key="connector.id">
              <tr class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-gray-200 text-gray-600 transition hover:border-primary-200 hover:text-primary-600 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-800 dark:hover:text-primary-300"
                      :aria-expanded="isConnectorExpanded(connector.id)"
                      :aria-label="isConnectorExpanded(connector.id) ? tM('connectors.collapseGroups') : tM('connectors.expandGroups')"
                      @click="toggleConnectorExpanded(connector.id)"
                    >
                      <Icon :name="isConnectorExpanded(connector.id) ? 'chevronDown' : 'chevronRight'" size="sm" />
                    </button>
                    <div class="min-w-0">
                      <div class="font-medium text-gray-900 dark:text-white">{{ connector.name }}</div>
                      <a
                        v-if="connectorExternalUrl(connector)"
                        :href="connectorExternalUrl(connector)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="mt-1 block max-w-[360px] truncate text-xs text-primary-600 hover:text-primary-700 hover:underline dark:text-primary-300 dark:hover:text-primary-200"
                      >
                        {{ connector.base_url }}
                      </a>
                      <div v-else class="mt-1 max-w-[360px] truncate text-xs text-gray-500 dark:text-gray-400">{{ connector.base_url }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span :class="statusClass(connector.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ connectorStatusLabel(connector.status) }}
                  </span>
                  <div v-if="connector.last_error" class="mt-1 max-w-[280px] truncate text-xs text-red-500">{{ friendlyUpstreamRelayError(connector.last_error) }}</div>
                </td>
                <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <div>{{ tM('connectors.authModeLabel') }}: {{ authModeLabel(connector.auth_mode) }}</div>
                  <div>{{ connectorCredentialSummary(connector) }}</div>
                  <div v-if="connector.has_login_email">{{ tM('connectors.emailLabel') }}: {{ connector.login_email_masked || tM('connectors.credentialSaved') }}</div>
                </td>
                <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <div>{{ tM('connectors.verifiedLabel') }}: {{ formatDate(connector.last_verified_at) }}</div>
                  <div class="mt-1">{{ tM('connectors.syncedLabel') }}: {{ formatDate(connector.last_synced_at) }}</div>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatAccountBalance(connector.upstream_account_balance) }}</div>
                  <div class="mt-1 whitespace-normal break-words text-xs leading-4 text-gray-400 dark:text-gray-500">{{ accountBalanceCheckedLabel(connector, 'connectors') }}</div>
                  <div v-if="connectorMetricsRefreshResult(connector.id)" class="mt-1 whitespace-normal break-words text-xs leading-4" :class="metricsRefreshStatusTextClass(connectorMetricsRefreshResult(connector.id)!.status)">
                    {{ connectorMetricsRefreshInlineLabel(connectorMetricsRefreshResult(connector.id)!) }}
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap justify-end gap-2">
                    <button class="btn btn-secondary whitespace-nowrap px-2 py-1 text-xs" type="button" @click="editConnector(connector)">{{ tM('connectors.edit') }}</button>
                    <button class="btn btn-secondary whitespace-nowrap px-2 py-1 text-xs" type="button" :disabled="manualSnapshotRefreshingId === connector.id || refreshingMetrics" @click="sync(connector)">
                      {{ manualSnapshotRefreshingId === connector.id ? tM('connectors.fetchingSnapshots') : tM('connectors.fetchSnapshots') }}
                    </button>
                    <button class="btn btn-secondary whitespace-nowrap px-2 py-1 text-xs" type="button" :disabled="refreshingMetrics" @click="refreshMetricsForSingleConnector(connector)">
                      {{ connectorMetricsRefreshingLabel(connector) }}
                    </button>
                    <button class="btn btn-secondary whitespace-nowrap px-3 py-1 text-xs" type="button" @click="openSnapshotDialog(connector)">{{ tM('connectors.snapshots') }}</button>
                    <button class="btn btn-danger whitespace-nowrap px-2 py-1 text-xs" type="button" @click="removeConnector(connector)">{{ tM('connectors.delete') }}</button>
                  </div>
                </td>
              </tr>
              <tr class="bg-gray-50/70 dark:bg-dark-800/50">
                <td colspan="6" class="p-0">
                  <div
                    class="grid transition-[grid-template-rows,opacity] duration-200 ease-out motion-reduce:transition-none"
                    :class="isConnectorExpanded(connector.id) ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'"
                    :aria-hidden="!isConnectorExpanded(connector.id)"
                  >
                    <div class="overflow-hidden">
                      <div class="py-3">
                        <div
                          v-if="connectorGroupRows(connector).length > 0"
                          data-testid="connector-group-expansion"
                          class="grid gap-2"
                        >
                          <div
                            v-for="row in connectorGroupRows(connector)"
                            :key="row.key"
                            data-testid="connector-group-item"
                            class="grid grid-cols-[26%_12%_20%_16%_13%_13%] items-center rounded border border-gray-100 bg-white text-xs leading-5 dark:border-dark-700 dark:bg-dark-900"
                          >
                            <div class="min-w-0 px-4 py-2.5">
                              <div class="truncate font-medium text-gray-900 dark:text-white">{{ connectorGroupRowName(row) }}</div>
                              <div class="truncate font-mono text-[11px] leading-4 text-gray-400 dark:text-gray-500">{{ connectorGroupRowGroupID(row) }}</div>
                            </div>
                            <div class="px-4 py-2.5">
                              <span :class="connectorGroupRowStatusClass(row)" class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium leading-4">{{ connectorGroupRowStatusLabel(row) }}</span>
                            </div>
                            <div class="px-4 py-2.5 text-right font-semibold tabular-nums text-gray-900 dark:text-white">{{ connectorGroupRowRateLabel(row) }}</div>
                            <div class="px-4 py-2.5 text-right tabular-nums text-gray-700 dark:text-gray-200">{{ connectorGroupRowPriorityLabel(row) }}</div>
                            <div class="min-w-0 px-4 py-2.5 text-right">
                              <div class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ connectorGroupRowUsageCostLabel(row) }}</div>
                              <div class="truncate text-[11px] leading-4 text-gray-400 dark:text-gray-500">{{ connectorGroupRowUsageMetaLabel(row) }}</div>
                            </div>
                            <div class="px-4 py-2.5 text-right">
                              <button v-if="row.kind === 'snapshot'" class="btn btn-secondary whitespace-nowrap px-2 py-1 text-xs" type="button" @click="createCandidateFromSnapshot(row.snapshot)">
                                {{ tM('connectors.createCandidate') }}
                              </button>
                            </div>
                          </div>
                        </div>
                        <div v-else class="rounded-md border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                          {{ tM('connectors.noGroups') }}
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
              </template>
              <tr v-if="connectors.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">{{ tM('connectors.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="activeSection === 'usageHistory'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('usageHistory.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('usageHistory.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span v-if="usageHistoryFiltersDirty" class="inline-flex rounded-md bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">{{ tM('usageHistory.filtersPending') }}</span>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('usageHistory.count', { n: usageHistoryTotal }) }}</span>
          </div>
        </div>
        <div class="grid gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 xl:grid-cols-[minmax(150px,180px)_minmax(150px,180px)_minmax(180px,220px)_minmax(160px,220px)_1fr_auto_auto_auto]">
          <label class="block space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.startDate') }}</span>
            <input v-model="usageHistoryStartDate" class="input w-full" type="date" />
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.endDate') }}</span>
            <input v-model="usageHistoryEndDate" class="input w-full" type="date" />
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.connector') }}</span>
            <select v-model.number="usageHistoryConnectorId" class="input w-full">
              <option :value="0">{{ tM('usageHistory.allConnectors') }}</option>
              <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
            </select>
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.groupId') }}</span>
            <input v-model.trim="usageHistoryGroupId" class="input w-full" type="search" :placeholder="tM('usageHistory.groupPlaceholder')" @keyup.enter="reloadUsageHistory" />
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.keyword') }}</span>
            <input v-model.trim="usageHistorySearch" class="input w-full" type="search" :placeholder="tM('usageHistory.searchPlaceholder')" @keyup.enter="reloadUsageHistory" />
          </label>
          <label class="flex items-end gap-2 pb-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="usageHistoryIncludeZeroUsage" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            {{ tM('usageHistory.includeZeroUsage') }}
          </label>
          <label class="flex items-end gap-2 pb-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="usageHistoryOnlyAnomalies" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            {{ tM('usageHistory.onlyAnomalies') }}
          </label>
          <div class="flex items-end">
            <button class="btn btn-secondary inline-flex w-full items-center justify-center gap-2" type="button" :disabled="usageHistoryLoading || usageHistoryDateRangeInvalid" @click="reloadUsageHistory">
              <Icon name="refresh" size="sm" />
              {{ usageHistoryFiltersDirty ? tM('usageHistory.applyFilters') : tM('usageHistory.search') }}
            </button>
          </div>
        </div>
        <div v-if="usageHistoryDateRangeInvalid" class="border-b border-amber-100 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-100">
          {{ tM('usageHistory.invalidDateRange') }}
        </div>
        <div v-if="usageHistorySummary.pendingFinalize > 0" class="border-b border-sky-100 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-800 dark:border-sky-900/50 dark:bg-sky-950/30 dark:text-sky-100">
          {{ tM('usageHistory.pendingFinalizeHint', { n: usageHistorySummary.pendingFinalize }) }}
        </div>
        <div class="flex flex-wrap gap-2 border-b border-gray-100 px-4 py-2 dark:border-dark-700">
          <button
            v-for="shortcut in usageHistoryDateShortcuts"
            :key="shortcut.key"
            class="btn btn-secondary px-3 py-1.5 text-xs"
            type="button"
            :disabled="usageHistoryLoading"
            @click="applyUsageHistoryDateShortcut(shortcut.key)"
          >
            {{ shortcut.label }}
          </button>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="usageHistoryLoading" @click="resetUsageHistoryFilters">
            {{ tM('usageHistory.resetFilters') }}
          </button>
        </div>
        <div v-if="usageHistory.length > 0" class="border-b border-gray-100 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="px-4 pt-3 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ tM('usageHistory.currentPageScopeHint', { shown: usageHistoryDisplayItems.length, loaded: usageHistory.length, total: usageHistoryTotal }) }}
          </div>
          <div class="grid gap-3 px-4 py-3 md:grid-cols-2 xl:grid-cols-7">
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryCost') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatUsageCost(usageHistorySummary.cost) }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryTokens') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatUsageTokenMillions(usageHistorySummary.tokens) }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryConnectors') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ usageHistorySummary.connectors }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryGroups') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ usageHistorySummary.groups }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryCostPerMillion') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCostPerMillionTokens(usageHistorySummary.cost, usageHistorySummary.tokens) }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryLatestCheckedAt') }}</div>
            <div class="mt-1 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatDate(usageHistorySummary.latestCheckedAt) }}</div>
            <div class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">{{ usageHistorySummary.dateRange }}</div>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('usageHistory.summaryPendingFinalize') }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ usageHistorySummary.pendingFinalize }}</div>
          </div>
          </div>
        </div>
        <div v-if="!usageHistoryLoading" class="border-b border-gray-100 px-4 py-2 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
          {{ usageHistoryAppliedFilterLabel }}
          <span v-if="usageHistoryFiltersDirty" class="ml-2 font-medium text-amber-700 dark:text-amber-300">{{ tM('usageHistory.resultUsesAppliedFilters') }}</span>
        </div>
        <div v-if="usageHistoryLoading" data-testid="usage-history-skeleton" class="space-y-4 px-4 py-5" role="status" :aria-label="tM('usageHistory.loading')">
          <span class="sr-only">{{ tM('usageHistory.loading') }}</span>
          <div v-for="group in 2" :key="group" class="space-y-3">
            <div class="flex items-center justify-between">
              <div class="skeleton h-4 w-32"></div>
              <div class="skeleton h-3 w-20"></div>
            </div>
            <div class="space-y-2">
              <div v-for="row in 3" :key="row" class="grid grid-cols-[minmax(90px,120px)_1fr_minmax(80px,110px)_minmax(80px,110px)_minmax(120px,160px)] gap-3">
                <div class="skeleton h-4"></div>
                <div class="skeleton h-4"></div>
                <div class="skeleton h-4"></div>
                <div class="skeleton h-4"></div>
                <div class="skeleton h-4"></div>
              </div>
            </div>
          </div>
        </div>
        <div v-else-if="usageHistoryGroups.length === 0" class="px-4 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ usageHistoryOnlyAnomalies && usageHistory.length > 0 ? tM('usageHistory.emptyAnomalies') : tM('usageHistory.empty') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="group in usageHistoryGroups" :key="group.connectorId" class="px-4 py-4">
            <div class="mb-3 flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
              <div>
                <div class="font-semibold text-gray-900 dark:text-white">{{ group.connectorName }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ tM('usageHistory.groupCount', { n: group.items.length }) }}</div>
              </div>
              <div class="flex flex-wrap gap-2 text-xs">
                <span class="inline-flex rounded-md bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ tM('usageHistory.groupSubtotalCost', { value: formatUsageCost(group.summary.cost) }) }}</span>
                <span class="inline-flex rounded-md bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ tM('usageHistory.groupSubtotalTokens', { value: formatUsageTokenMillions(group.summary.tokens) }) }}</span>
                <span class="inline-flex rounded-md bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ tM('usageHistory.groupSubtotalGroups', { n: group.summary.groups }) }}</span>
                <span class="inline-flex rounded-md bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ tM('usageHistory.groupLatestCheckedAt', { time: formatDate(group.summary.latestCheckedAt) }) }}</span>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[1040px] text-sm">
                <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-3 py-2 text-left">{{ tM('usageHistory.colDate') }}</th>
                    <th class="px-3 py-2 text-left">{{ tM('usageHistory.colGroup') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('usageHistory.colCost') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('usageHistory.colTokens') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('usageHistory.colTokensPerQuota') }}</th>
                    <th class="px-3 py-2 text-left">{{ tM('usageHistory.colCheckedAt') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="item in group.items" :key="item.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70" :class="usageHistoryRowFlags(item).length > 0 ? 'bg-amber-50/40 dark:bg-amber-950/10' : ''">
                    <td class="px-3 py-3 whitespace-nowrap tabular-nums">
                      <div>{{ item.usage_date }}</div>
                      <span :class="usageHistoryStatusBadgeClass(item)" class="mt-1 inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium">
                        {{ usageHistoryStatusLabel(item) }}
                      </span>
                    </td>
                    <td class="px-3 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">{{ item.group_name || item.upstream_group_id }}</div>
                      <div class="mt-0.5 font-mono text-xs text-gray-400 dark:text-gray-500">{{ item.upstream_group_id }}</div>
                      <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ item.platform || '-' }}</div>
                      <div v-if="usageHistoryRowFlags(item).length > 0" class="mt-2 flex flex-wrap gap-1.5">
                        <span
                          v-for="flag in usageHistoryRowFlags(item)"
                          :key="flag"
                          class="inline-flex rounded bg-amber-100 px-1.5 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900/50 dark:text-amber-100"
                        >
                          {{ flag }}
                        </span>
                      </div>
                    </td>
                    <td class="px-3 py-3 text-right font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatUsageCost(item.actual_cost) }}</td>
                    <td class="px-3 py-3 text-right tabular-nums">{{ formatUsageTokenMillions(item.total_tokens) }}</td>
                    <td class="px-3 py-3 text-right tabular-nums">{{ formatUsageTokenMillionsPerQuota(item.actual_cost, item.total_tokens) }}</td>
                    <td class="px-3 py-3 whitespace-nowrap">
                      <div>{{ usageHistoryCheckedAtLabel(item) }}</div>
                      <button
                        v-if="usageHistoryRowStatus(item) === 'pending'"
                        class="btn btn-secondary mt-2 px-2 py-1 text-xs"
                        type="button"
                        :disabled="finalizingUsageKey === usageHistoryFinalizeKey(item)"
                        @click="finalizeUsageHistoryRow(item)"
                      >
                        {{ finalizingUsageKey === usageHistoryFinalizeKey(item) ? tM('usageHistory.manualFinalizeRunning') : tM('usageHistory.manualFinalizeButton') }}
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="flex flex-col gap-2 border-t border-gray-100 px-4 py-3 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400 sm:flex-row sm:items-center sm:justify-between">
          <span>{{ tM('usageHistory.pageInfo', { page: usageHistoryPage, pages: usageHistoryPages }) }}</span>
          <div class="flex gap-2">
            <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="usageHistoryLoading || usageHistoryFiltersDirty || usageHistoryDateRangeInvalid || usageHistoryPage <= 1" @click="changeUsageHistoryPage(usageHistoryPage - 1)">{{ tM('usageHistory.prev') }}</button>
            <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="usageHistoryLoading || usageHistoryFiltersDirty || usageHistoryDateRangeInvalid || usageHistoryPage >= usageHistoryPages" @click="changeUsageHistoryPage(usageHistoryPage + 1)">{{ tM('usageHistory.next') }}</button>
          </div>
        </div>
      </section>

      <section v-if="activeSection === 'snapshotChanges'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('snapshotChanges.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('snapshotChanges.description') }}</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('snapshotChanges.count', { n: snapshotChangeTotal }) }}</span>
        </div>
        <div v-if="bulkOperationFeedback && bulkOperationSection === 'snapshotChanges'" class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <OperationResultPanel v-bind="bulkOperationFeedback" @dismiss="clearBulkOperationFeedback" />
        </div>
        <div class="grid gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:grid-cols-[minmax(180px,220px)_minmax(160px,200px)_1fr_auto]">
          <select v-model.number="snapshotChangeConnectorId" class="input w-full">
            <option :value="0">{{ tM('snapshotChanges.allConnectors') }}</option>
            <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
          </select>
          <select v-model="snapshotChangeType" class="input w-full">
            <option v-for="option in snapshotChangeTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
          <input v-model.trim="snapshotChangeSearch" class="input w-full" type="search" :placeholder="tM('snapshotChanges.searchPlaceholder')" @keyup.enter="reloadSnapshotChanges" />
          <button class="btn btn-secondary inline-flex items-center justify-center gap-2" type="button" :disabled="snapshotChangesLoading" @click="reloadSnapshotChanges">
            <Icon name="refresh" size="sm" />
            {{ tM('snapshotChanges.search') }}
          </button>
        </div>
        <div v-if="snapshotChangesLoading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="snapshotChangeGroups.length === 0" class="px-4 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ tM('snapshotChanges.empty') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="group in snapshotChangeGroups" :key="group.connectorId" class="px-4 py-4">
            <div class="mb-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div class="font-semibold text-gray-900 dark:text-white">{{ group.connectorName }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ tM('snapshotChanges.groupCount', { n: group.items.length }) }}</div>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[980px] text-sm">
                <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-3 py-2 text-left">{{ tM('snapshotChanges.colChangedAt') }}</th>
                    <th class="px-3 py-2 text-left">{{ tM('snapshotChanges.colGroup') }}</th>
                    <th class="px-3 py-2 text-left">{{ tM('snapshotChanges.colType') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('snapshotChanges.colOldRate') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('snapshotChanges.colNewRate') }}</th>
                    <th class="px-3 py-2 text-right">{{ tM('snapshotChanges.colDelta') }}</th>
                    <th class="px-3 py-2 text-left">{{ tM('snapshotChanges.colSource') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="change in group.items" :key="change.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                    <td class="px-3 py-3 whitespace-nowrap">{{ formatDate(change.changed_at) }}</td>
                    <td class="px-3 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">{{ change.group_name || change.upstream_group_id }}</div>
                      <div class="mt-0.5 font-mono text-xs text-gray-400 dark:text-gray-500">{{ change.upstream_group_id }}</div>
                      <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ change.platform || '-' }}</div>
                    </td>
                    <td class="px-3 py-3">
                      <span :class="snapshotChangeTypeClass(change)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                        {{ snapshotChangeTypeLabel(change) }}
                      </span>
                    </td>
                    <td class="px-3 py-3 text-right tabular-nums">{{ formatNullableRate(change.old_final_rate_multiplier) }}</td>
                    <td class="px-3 py-3 text-right tabular-nums">{{ formatNullableRate(change.new_final_rate_multiplier) }}</td>
                    <td :class="snapshotRateDeltaClass(change)" class="px-3 py-3 text-right font-medium tabular-nums">{{ snapshotRateDeltaLabel(change) }}</td>
                    <td class="px-3 py-3"><RateSourceTag :source="change.source || 'none'" /></td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="flex flex-col gap-2 border-t border-gray-100 px-4 py-3 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400 sm:flex-row sm:items-center sm:justify-between">
          <span>{{ tM('snapshotChanges.pageInfo', { page: snapshotChangePage, pages: snapshotChangePages }) }}</span>
          <div class="flex gap-2">
            <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="snapshotChangesLoading || snapshotChangePage <= 1" @click="changeSnapshotChangePage(snapshotChangePage - 1)">{{ tM('snapshotChanges.prev') }}</button>
            <button class="btn btn-secondary px-3 py-1.5 text-xs" type="button" :disabled="snapshotChangesLoading || snapshotChangePage >= snapshotChangePages" @click="changeSnapshotChangePage(snapshotChangePage + 1)">{{ tM('snapshotChanges.next') }}</button>
          </div>
        </div>
      </section>

      <section v-if="activeSection === 'recommendations'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('recommendations.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('recommendations.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-3 sm:justify-end">
            <div class="flex flex-wrap items-center gap-2">
              <span class="inline-flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                <span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
                {{ tM('recommendations.pendingSummary', { n: pendingSuggestionCount }) }}
              </span>
              <span class="inline-flex items-center rounded-md bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                {{ recommendationFilterSummary }}
              </span>
            </div>
            <label
              class="inline-flex cursor-pointer items-center gap-2 text-xs font-medium transition"
              :class="recommendationOnlyWithSuggestions ? 'text-primary-700 dark:text-primary-200' : 'text-gray-600 dark:text-gray-300'"
            >
              <input v-model="recommendationOnlyWithSuggestions" type="checkbox" class="sr-only" @change="reloadRecommendationRuns" />
              <span class="relative inline-flex h-5 w-9 items-center rounded-full transition" :class="recommendationOnlyWithSuggestions ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'">
                <span class="inline-block h-4 w-4 rounded-full bg-white shadow transition" :class="recommendationOnlyWithSuggestions ? 'translate-x-4' : 'translate-x-0.5'"></span>
              </span>
              {{ tM('recommendations.onlyWithSuggestions') }}
            </label>
          </div>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1080px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colRun') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colCandidates') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colSuggestions') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colStatus') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colSource') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colCreatedAt') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="run in recommendationRuns" :key="run.id">
                <td class="px-4 py-3 font-mono">#{{ run.id }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.total_candidates }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.suggestion_count }}</td>
                <td class="px-4 py-3" :data-testid="`recommendation-run-status-${run.id}`">
                  <span :class="recommendationRunStatusClass(run)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ recommendationRunStatusLabel(run) }}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <div class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
                    <div>{{ recommendationRunCreatedByLabel(run) }}</div>
                    <div v-if="run.applied">{{ recommendationRunAppliedByLabel(run) }}</div>
                  </div>
                </td>
                <td class="px-4 py-3">{{ formatDate(run.created_at) }}</td>
                <td class="px-4 py-3 text-right">
                  <div class="flex flex-nowrap justify-end gap-2">
                    <button class="btn btn-secondary whitespace-nowrap px-3 py-1.5 text-xs" type="button" @click="openApplyDialog(run)">
                      {{ canApplyRecommendationRun(run) ? tM('recommendations.viewAndApply') : tM('recommendations.viewDetails') }}
                    </button>
                    <button v-if="canApplyRecommendationRun(run)" class="btn btn-secondary whitespace-nowrap px-3 py-1.5 text-xs" type="button" :disabled="closingRecommendationRun || restoringRecommendationRun" @click="closeRecommendationRunFromList(run)">
                      {{ closingRecommendationRun ? tM('applyDialog.closingSuggestion') : tM('recommendations.closeSuggestion') }}
                    </button>
                    <button v-if="canRestoreRecommendationRun(run)" class="btn btn-secondary whitespace-nowrap px-3 py-1.5 text-xs" type="button" :disabled="closingRecommendationRun || restoringRecommendationRun" @click="restoreRecommendationRunFromList(run)">
                      {{ restoringRecommendationRun ? tM('applyDialog.restoringSuggestion') : tM('recommendations.restoreSuggestion') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="recommendationRuns.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">
                  <div class="font-medium text-gray-700 dark:text-gray-200">
                    {{ recommendationOnlyWithSuggestions ? tM('recommendations.emptyFilteredTitle') : tM('recommendations.emptyTitle') }}
                  </div>
                  <div class="mt-1 text-sm">
                    {{ recommendationOnlyWithSuggestions ? tM('recommendations.emptyFiltered') : tM('recommendations.empty') }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="flex flex-col gap-3 border-t border-gray-100 bg-gray-50/70 px-4 py-3 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900/30 dark:text-gray-400 sm:flex-row sm:items-center sm:justify-between">
          <div class="space-y-0.5">
            <div class="font-medium text-gray-700 dark:text-gray-200">
              {{ recommendationRunsLoading ? tM('recommendations.loadingPage') : recommendationPageStatus }}
            </div>
            <div class="text-xs">{{ recommendationRangeStatus }}</div>
          </div>
          <div class="inline-flex w-fit overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <button class="px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:text-gray-300 dark:text-gray-200 dark:hover:bg-dark-800 dark:disabled:text-gray-600" type="button" :disabled="recommendationRunsLoading || recommendationRunPage <= 1" @click="changeRecommendationRunPage(recommendationRunPage - 1)">{{ tM('recommendations.prev') }}</button>
            <button class="border-l border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:text-gray-300 dark:border-dark-700 dark:text-gray-200 dark:hover:bg-dark-800 dark:disabled:text-gray-600" type="button" :disabled="recommendationRunsLoading || recommendationRunPage >= recommendationRunPages" @click="changeRecommendationRunPage(recommendationRunPage + 1)">{{ tM('recommendations.next') }}</button>
          </div>
        </div>
      </section>

      <section v-if="activeSection === 'monitoring'" class="space-y-4">
        <div class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('monitoring.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('monitoring.description') }}</p>
            </div>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('monitoring.updatedAt', { time: formatDate(monitoringPolicyForm.updated_at) }) }}</span>
          </div>
          <div
            data-testid="auto-monitoring-status"
            class="border-b px-4 py-3"
            :class="autoMonitoringStatusPanelClass"
          >
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex min-w-0 items-center gap-2">
                <span class="relative flex h-3 w-3 shrink-0">
                  <span v-if="autoMonitoringEnabled" class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60"></span>
                  <span class="relative inline-flex h-3 w-3 rounded-full" :class="autoMonitoringStatusDotClass"></span>
                </span>
                <span class="text-sm font-semibold">{{ tM(autoMonitoringStatusTitleKey) }}</span>
              </div>
              <p class="text-sm">{{ tM(autoMonitoringStatusDetailKey, { sync: savedMonitoringSyncInterval, probe: savedMonitoringProbeInterval, recommendation: savedMonitoringRecommendationInterval }) }}</p>
            </div>
            <div class="mt-3 flex flex-wrap gap-2">
              <span v-for="item in autoMonitoringStatusChips" :key="item.key" class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium" :class="item.className">
                <span class="h-1.5 w-1.5 rounded-full" :class="item.dotClass"></span>
                {{ item.label }}
              </span>
            </div>
          </div>
          <div class="grid gap-4 p-4 lg:grid-cols-4">
            <label class="flex items-center gap-2 rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
              <input v-model="monitoringPolicyForm.auto_sync_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ tM('monitoring.autoSyncEnabled') }}
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.syncInterval') }}</span>
              <input v-model.number="monitoringPolicyForm.sync_interval_minutes" class="input w-full" type="number" min="1" />
            </label>
            <label class="flex items-center gap-2 rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
              <input v-model="monitoringPolicyForm.auto_probe_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ tM('monitoring.autoProbeEnabled') }}
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.probeInterval') }}</span>
              <input v-model.number="monitoringPolicyForm.probe_interval_minutes" class="input w-full" type="number" min="1" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.failureRetryInterval') }}</span>
              <input v-model.number="monitoringPolicyForm.failure_retry_interval_minutes" class="input w-full" type="number" min="1" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.syncConcurrency') }}</span>
              <input v-model.number="monitoringPolicyForm.sync_concurrency" class="input w-full" type="number" min="1" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.probeConcurrency') }}</span>
              <input v-model.number="monitoringPolicyForm.probe_concurrency" class="input w-full" type="number" min="1" />
            </label>
          </div>
          <div class="border-t border-gray-100 px-4 py-4 dark:border-dark-700">
            <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">{{ tM('monitoring.recommendationAutomationTitle') }}</div>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('monitoring.recommendationAutomationDescription') }}</p>
              </div>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ tM('monitoring.recommendationIntervalHint', { recommendation: normalizedMonitoringRecommendationInterval }) }}</span>
            </div>
            <div class="mt-4 grid gap-4 lg:grid-cols-4">
              <label class="flex items-center gap-2 rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
                <input v-model="monitoringPolicyForm.auto_recommendation_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                {{ tM('monitoring.autoRecommendationEnabled') }}
              </label>
              <label class="block space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.recommendationInterval') }}</span>
                <input v-model.number="monitoringPolicyForm.recommendation_interval_minutes" class="input w-full" type="number" min="1" />
              </label>
              <label class="flex items-center gap-2 rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
                <input v-model="monitoringPolicyForm.auto_apply_recommendations_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                {{ tM('monitoring.autoApplyRecommendationsEnabled') }}
              </label>
              <label class="block space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.minAutoApplyConfidence') }}</span>
                <select v-model="monitoringPolicyForm.min_auto_apply_confidence" class="input w-full">
                  <option value="high">{{ tM('confidence.high') }}</option>
                  <option value="medium">{{ tM('confidence.medium') }}</option>
                  <option value="low">{{ tM('confidence.low') }}</option>
                  <option value="unknown">{{ tM('confidence.unknown') }}</option>
                </select>
              </label>
              <label class="block space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.maxAutoApplySuggestions') }}</span>
                <input v-model.number="monitoringPolicyForm.max_auto_apply_suggestions" class="input w-full" type="number" min="1" />
              </label>
              <label class="block space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('monitoring.maxAutoApplyPriorityDelta') }}</span>
                <input v-model.number="monitoringPolicyForm.max_auto_apply_priority_delta" class="input w-full" type="number" min="0" />
              </label>
              <label class="flex items-center gap-2 rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300 lg:col-span-2">
                <input v-model="monitoringPolicyForm.allow_auto_apply_degraded_health" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                {{ tM('monitoring.allowAutoApplyDegradedHealth') }}
              </label>
            </div>
            <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
              {{ monitoringPolicyForm.auto_apply_recommendations_enabled ? tM('monitoring.autoApplyEnabledHint') : tM('monitoring.autoApplyDisabledHint') }}
            </p>
          </div>
          <div class="border-t border-gray-100 px-4 py-3 dark:border-dark-700">
            <div class="text-sm font-medium text-gray-900 dark:text-white">{{ tM('monitoring.derivedTitle') }}</div>
            <div class="mt-2 grid gap-3 text-sm text-gray-600 dark:text-gray-300 md:grid-cols-3">
              <div>{{ tM('monitoring.snapshotStaleDerived', { interval: normalizedMonitoringSyncInterval, stale: monitoringSnapshotStaleAfterMinutes }) }}</div>
              <div>{{ tM('monitoring.usageStaleDerived', { interval: normalizedMonitoringSyncInterval, stale: monitoringUsageDeltaStaleAfterMinutes }) }}</div>
              <div>{{ tM('monitoring.probeStaleDerived', { interval: normalizedMonitoringProbeInterval, stale: monitoringProbeStaleAfterMinutes }) }}</div>
            </div>
          </div>
          <div class="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <div class="text-sm text-gray-500 dark:text-gray-400">
              <span v-if="monitoringPolicySavedAt">{{ tM('monitoring.savedAt', { time: formatDate(monitoringPolicySavedAt) }) }}</span>
              <span v-else>{{ tM('monitoring.saveHint') }}</span>
            </div>
            <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="savingMonitoringPolicy" @click="saveMonitoringPolicy">
              <Icon name="save" size="sm" />
              {{ savingMonitoringPolicy ? tM('monitoring.saving') : tM('monitoring.save') }}
            </button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ tM('monitoring.manualActionsTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('monitoring.manualActionsDescription') }}</p>
          </div>
          <div class="grid gap-3 p-4 md:grid-cols-2">
            <button class="btn btn-secondary inline-flex items-center justify-center gap-2" type="button" :disabled="bulkSyncing || connectors.length === 0" @click="syncAllConnectors">
              <Icon name="refresh" size="sm" />
              {{ bulkSyncing ? tM('monitoring.fetchSnapshotsAllRunning') : tM('monitoring.fetchSnapshotsAll') }}
            </button>
            <button class="btn btn-secondary inline-flex items-center justify-center gap-2" type="button" :disabled="bulkProbing || enabledCandidateCount === 0" @click="probeAllCandidates">
              <Icon name="play" size="sm" />
              {{ bulkProbing ? tM('monitoring.probeAllRunning') : tM('monitoring.probeAll') }}
            </button>
          </div>
          <div v-if="bulkOperationFeedback && bulkOperationSection === 'monitoring'" class="border-t border-gray-100 px-4 py-3 dark:border-dark-700">
            <OperationResultPanel v-bind="bulkOperationFeedback" @dismiss="clearBulkOperationFeedback" />
          </div>
        </div>
      </section>

      <section v-if="activeSection === 'policy'" class="space-y-4">
        <div class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('policy.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('policy.description') }}</p>
            </div>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('policy.updatedAt', { time: formatDate(policyForm.updated_at) }) }}</span>
          </div>
          <div class="grid gap-4 p-4 lg:grid-cols-4">
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.minSuccessRate') }}</span>
              <input v-model.number="policyMinSuccessRatePercent" class="input w-full" type="number" min="0" max="100" step="1" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.minSampleSize') }}</span>
              <input v-model.number="policyForm.min_sample_size" class="input w-full" type="number" min="1" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.priorityStart') }}</span>
              <input v-model.number="policyForm.priority_start" class="input w-full" type="number" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.priorityStep') }}</span>
              <input v-model.number="policyForm.priority_step" class="input w-full" type="number" min="1" />
            </label>
            <label class="flex items-center gap-2 self-end rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
              <input v-model="policyForm.exclude_consecutive_failures" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ tM('policy.excludeFailures') }}
            </label>
          </div>
          <div class="border-t border-gray-100 px-4 py-3 dark:border-dark-700">
            <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.sortFields') }}</div>
            <div class="grid gap-2 md:grid-cols-3">
              <select v-for="(_, index) in policyForm.sort_fields" :key="index" v-model="policyForm.sort_fields[index]" class="input w-full">
                <option v-for="option in sortFieldOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </div>
          </div>
          <div class="border-t border-gray-100 px-4 py-3 dark:border-dark-700">
            <div class="mb-3">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ tM('policy.pauseStrategiesTitle') }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ tM('policy.pauseStrategiesDescription') }}</div>
            </div>
            <div class="grid gap-3 lg:grid-cols-3">
              <label class="rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
                <span class="flex items-center gap-2">
                  <input v-model="policyForm.pause_rate_gap_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" data-testid="policy-pause-rate-gap-enabled" />
                  <span class="font-medium">{{ tM('policy.pauseRateGap') }}</span>
                </span>
                <input v-if="policyForm.pause_rate_gap_enabled" v-model.number="policyForm.pause_rate_gap_threshold" class="input mt-2 w-full" type="number" min="0.0001" step="0.001" data-testid="policy-pause-rate-gap-threshold" />
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ tM('policy.pauseRateGapHint') }}</span>
              </label>
              <label class="rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
                <span class="flex items-center gap-2">
                  <input v-model="policyForm.pause_consecutive_failures_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" data-testid="policy-pause-consecutive-failures-enabled" />
                  <span class="font-medium">{{ tM('policy.pauseConsecutiveFailures') }}</span>
                </span>
                <input v-if="policyForm.pause_consecutive_failures_enabled" v-model.number="policyForm.pause_consecutive_failures_threshold" class="input mt-2 w-full" type="number" min="1" step="1" data-testid="policy-pause-consecutive-failures-threshold" />
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ tM('policy.pauseConsecutiveFailuresHint') }}</span>
              </label>
              <label class="rounded-lg border border-gray-100 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
                <span class="flex items-center gap-2">
                  <input v-model="policyForm.pause_success_rate_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" data-testid="policy-pause-success-rate-enabled" />
                  <span class="font-medium">{{ tM('policy.pauseSuccessRate') }}</span>
                </span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ tM('policy.pauseSuccessRateHint') }}</span>
              </label>
            </div>
          </div>
          <div class="border-t border-gray-100 px-4 py-3 text-sm text-gray-600 dark:border-dark-700 dark:text-gray-300">
            <div class="font-medium text-gray-900 dark:text-white">{{ tM('policy.derivedFreshnessTitle') }}</div>
            <div class="mt-2 grid gap-3 md:grid-cols-3">
              <div>{{ tM('policy.derivedSnapshotFreshness', { minutes: monitoringSnapshotStaleAfterMinutes }) }}</div>
              <div>{{ tM('policy.derivedUsageFreshness', { minutes: monitoringUsageDeltaStaleAfterMinutes }) }}</div>
              <div>{{ tM('policy.derivedProbeFreshness', { minutes: monitoringProbeStaleAfterMinutes }) }}</div>
            </div>
          </div>
        </div>

        <div v-if="previewLoading || policyPreviewLastUpdatedAt" class="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-700 dark:border-blue-900/50 dark:bg-blue-950/30 dark:text-blue-200">
          {{ previewLoading ? tM('policy.previewLoadingHint') : tM('policy.previewReady', { time: formatDate(policyPreviewLastUpdatedAt) }) }}
        </div>

        <div v-if="policyPreview" ref="policyPreviewResultRef" class="grid scroll-mt-24 gap-4 lg:grid-cols-3">
          <div class="card p-4">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.previewTotal') }}</div>
            <div class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ policyPreview.total_candidates }}</div>
          </div>
          <div class="card p-4">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.previewSuggestions') }}</div>
            <div class="mt-2 text-2xl font-semibold tabular-nums text-emerald-600 dark:text-emerald-300">{{ policyPreview.suggestion_count }}</div>
          </div>
          <div class="card p-4">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('policy.previewExcluded') }}</div>
            <div class="mt-2 text-2xl font-semibold tabular-nums text-amber-600 dark:text-amber-300">{{ policyPreview.excluded_count }}</div>
          </div>
        </div>

        <div v-if="policyPreview" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ tM('policy.suggestionsTitle') }}</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[920px] text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ tM('applyDialog.colAccount') }}</th>
                  <th class="px-4 py-3 text-left">{{ tM('applyDialog.colAction') }}</th>
                  <th class="px-4 py-3 text-left">{{ tM('candidates.colMapping') }}</th>
                  <th class="px-4 py-3 text-right">{{ tM('applyDialog.colChange') }}</th>
                  <th class="px-4 py-3 text-right">{{ tM('applyDialog.colRate') }}</th>
                  <th class="px-4 py-3 text-left">{{ tM('policy.colReason') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="suggestion in policyPreview.suggestions" :key="suggestion.candidate_id">
                  <td class="px-4 py-3">#{{ suggestion.account_id }} {{ suggestion.account_name || '-' }}</td>
                  <td class="px-4 py-3">
                    <span :class="suggestionActionClass(suggestion)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ suggestionActionLabel(suggestion) }}</span>
                  </td>
                  <td class="px-4 py-3">{{ suggestionMappingLabel(suggestion) }}</td>
                  <td class="px-4 py-3 text-right">{{ suggestionChangeLabel(suggestion) }}</td>
                  <td class="px-4 py-3 text-right">{{ formatRate(suggestion.final_rate_multiplier) }}</td>
                  <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ suggestion.reason }}</td>
                </tr>
                <tr v-if="policyPreview.suggestions.length === 0">
                  <td colspan="6" class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">{{ tM('policy.noPreviewSuggestions') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-if="policyPreview" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ tM('policy.exclusionsTitle') }}</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[980px] text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ tM('candidates.colCandidate') }}</th>
                  <th class="px-4 py-3 text-left">{{ tM('candidates.colMapping') }}</th>
                  <th class="px-4 py-3 text-right">{{ tM('candidates.colRate') }}</th>
                  <th class="px-4 py-3 text-left">{{ tM('policy.colReason') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in policyPreview.exclusions" :key="item.candidate_id">
                  <td class="px-4 py-3">#{{ item.account_id }} {{ item.account_name || '-' }}</td>
                  <td class="px-4 py-3">{{ suggestionMappingLabel(item) }}</td>
                  <td class="px-4 py-3 text-right">{{ formatNullableRate(item.final_rate_multiplier) }}</td>
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ exclusionReasonLabel(item.reason_code) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.reason }}</div>
                  </td>
                </tr>
                <tr v-if="policyPreview.exclusions.length === 0">
                  <td colspan="4" class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">{{ tM('policy.noPreviewExclusions') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>

  <BaseDialog
    :show="connectorDialogOpen"
    :title="connectorForm.id ? tM('connectorForm.titleEdit') : tM('connectorForm.titleCreate')"
    width="wide"
    :close-on-escape="false"
    :close-on-click-outside="false"
    :show-close-button="false"
    :animated="false"
    @close="keepConnectorDialogOpen"
  >
    <form
      id="connector-form"
      class="space-y-4"
      data-ignore-foreground-activity="true"
      @pointerdown.stop
      @pointerup.stop
      @mousedown.stop
      @mouseup.stop
      @touchstart.stop
      @touchend.stop
      @click.stop
      @submit.prevent="submitConnector"
    >
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelName') }}</span>
        <input v-model.trim="connectorForm.name" class="input w-full" type="text" />
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelBaseUrl') }}</span>
        <input v-model.trim="connectorForm.base_url" class="input w-full" type="url" placeholder="https://upstream.example.com" />
      </label>
      <div
        class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800"
        @pointerdown.stop
        @pointerup.stop
        @mousedown.stop
        @mouseup.stop
        @touchstart.stop
        @touchend.stop
        @click.stop
      >
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'manual_session' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @pointerdown.stop
          @pointerup.stop
          @mousedown.stop
          @mouseup.stop
          @touchstart.stop
          @touchend.stop
          @click.stop.prevent="setConnectorAuthMode('manual_session')"
        >
          {{ tM('connectorForm.authManual') }}
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'password_login' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @pointerdown.stop
          @pointerup.stop
          @mousedown.stop
          @mouseup.stop
          @touchstart.stop
          @touchend.stop
          @click.stop.prevent="setConnectorAuthMode('password_login')"
        >
          {{ tM('connectorForm.authPassword') }}
        </button>
      </div>
      <template v-if="connectorForm.auth_mode === 'manual_session'">
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelBearerToken') }}</span>
          <input v-model.trim="connectorForm.bearer_token" class="input w-full" type="password" autocomplete="new-password" :placeholder="tM('connectorForm.placeholderBearerToken')" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelRefreshToken') }}</span>
          <input v-model.trim="connectorForm.refresh_token" class="input w-full" type="password" autocomplete="new-password" :disabled="connectorForm.clear_refresh_token" :placeholder="tM('connectorForm.placeholderRefreshToken')" />
        </label>
        <label v-if="connectorForm.id && connectorForm.has_refresh_token" class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
          <input v-model="connectorForm.clear_refresh_token" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
          <span>{{ tM('connectorForm.clearRefreshToken') }}</span>
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelCookie') }}</span>
          <textarea v-model.trim="connectorForm.cookie" class="input min-h-[74px] w-full" :placeholder="tM('connectorForm.placeholderCookie')" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelUserAgent') }}</span>
          <input v-model.trim="connectorForm.user_agent" class="input w-full" type="text" :placeholder="tM('connectorForm.placeholderUserAgent')" />
        </label>
      </template>
      <div
        v-else
        class="space-y-4"
        @pointerdown.stop
        @pointerup.stop
        @mousedown.stop
        @mouseup.stop
        @touchstart.stop
        @touchend.stop
        @click.stop
        @input.stop
        @change.stop
      >
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelEmail') }}</span>
          <input v-model.trim="connectorForm.login_email" class="input w-full" type="email" autocomplete="username" :placeholder="connectorForm.id ? tM('connectorForm.placeholderEmailEdit') : tM('connectorForm.placeholderEmail')" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelPassword') }}</span>
          <input v-model.trim="connectorForm.login_password" class="input w-full" type="password" autocomplete="current-password" :placeholder="tM('connectorForm.placeholderPassword')" />
        </label>
      </div>
      <div v-if="connectorFormError" data-testid="connector-form-error" role="alert" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
        <div class="font-medium">{{ tM('connectorForm.saveFailedTitle') }}</div>
        <div class="mt-1 whitespace-pre-wrap break-words text-xs leading-5">{{ connectorFormError }}</div>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="closeConnectorDialog">{{ tM('connectorForm.cancel') }}</button>
        <button class="btn btn-primary" type="submit" form="connector-form" :disabled="savingConnector">
          {{ savingConnector ? tM('connectorForm.saving') : connectorForm.id ? tM('connectorForm.updateAndVerify') : tM('connectorForm.saveAndVerify') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog :show="candidateDialogOpen" :title="candidateForm.id ? tM('candidateForm.titleEdit') : tM('candidateForm.titleCreate')" width="wide" @close="closeCandidateDialog">
    <form id="candidate-form" class="space-y-4" @submit.prevent="submitCandidate()">
      <div v-if="candidateSourceSnapshot" class="rounded-lg border border-primary-100 bg-primary-50 px-4 py-3 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-950/30 dark:text-primary-100">
        <div class="font-medium">{{ tM('candidateForm.snapshotSourceTitle') }}{{ candidateSourceSnapshot.name || candidateSourceSnapshot.upstream_group_id }}</div>
        <div class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs">
          <span>{{ tM('candidateForm.snapshotConnector') }}{{ snapshotConnector?.name || `Connector #${candidateSourceSnapshot.connector_id}` }}</span>
          <span>{{ tM('candidateForm.snapshotPlatform') }}{{ candidateSourceSnapshot.platform || '-' }}</span>
          <span>{{ tM('candidateForm.snapshotRate') }}{{ formatRate(candidateSourceSnapshot.final_rate_multiplier) }}</span>
          <span class="inline-flex items-center gap-1">{{ tM('candidateForm.snapshotSource') }}<RateSourceTag :source="candidateSourceSnapshot.source" /></span>
        </div>
      </div>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelConnector') }}</span>
        <select v-model.number="candidateForm.connector_id" class="input w-full" :class="candidateFormErrors.connector ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''" :aria-invalid="Boolean(candidateFormErrors.connector)">
          <option :value="0">{{ tM('candidateForm.placeholderSelect') }}</option>
          <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
        </select>
        <p v-if="candidateFormErrors.connector" class="text-xs text-red-600 dark:text-red-300">{{ candidateFormErrors.connector }}</p>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelAccount') }}</span>
        <select v-model.number="candidateForm.account_id" class="input w-full" :class="candidateFormErrors.account ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''" :aria-invalid="Boolean(candidateFormErrors.account)">
          <option :value="0">{{ tM('candidateForm.placeholderSelect') }}</option>
          <option v-for="account in accounts" :key="account.id" :value="account.id">{{ accountOptionLabel(account) }}</option>
        </select>
        <p v-if="candidateFormErrors.account" class="text-xs text-red-600 dark:text-red-300">{{ candidateFormErrors.account }}</p>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelUpstreamGroupId') }}</span>
        <select
          class="input w-full"
          :class="candidateFormErrors.upstreamGroup ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''"
          data-testid="candidate-upstream-group-select"
          :value="candidateGroupSelectValue"
          :disabled="!candidateForm.connector_id"
          @change="selectCandidateGroupOption"
        >
          <option value="">{{ candidateGroupPlaceholder }}</option>
          <option v-for="option in candidateGroupOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          <option :value="MANUAL_CANDIDATE_GROUP_OPTION">{{ tM('candidateForm.manualGroupId') }}</option>
        </select>
        <input
          v-if="candidateGroupManualInputVisible"
          v-model.trim="candidateForm.upstream_group_id"
          class="input w-full"
          :class="candidateFormErrors.upstreamGroup ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''"
          data-testid="candidate-upstream-group-manual-input"
          type="text"
          :placeholder="tM('candidateForm.placeholderManualGroupId')"
          @input="syncCandidateGroupManualInput"
        />
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ candidateGroupSelectHint }}</p>
        <p v-if="candidateFormErrors.upstreamGroup" class="text-xs text-red-600 dark:text-red-300">{{ candidateFormErrors.upstreamGroup }}</p>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelUpstreamApiKey') }}</span>
        <select v-model.number="candidateForm.upstream_api_key_id" class="input w-full" :class="candidateFormErrors.upstreamApiKey ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''" :aria-invalid="Boolean(candidateFormErrors.upstreamApiKey)" :disabled="!candidateForm.connector_id || loadingConnectorAPIKeys" @change="syncSelectedCandidateAPIKey">
          <option :value="null">{{ loadingConnectorAPIKeys ? tM('candidateForm.loadingApiKeys') : tM('candidateForm.placeholderApiKey') }}</option>
          <option v-for="apiKey in connectorAPIKeys" :key="apiKey.id" :value="apiKey.id">{{ apiKeyOptionLabel(apiKey) }}</option>
        </select>
        <p v-if="candidateFormErrors.upstreamApiKey" class="text-xs text-red-600 dark:text-red-300">{{ candidateFormErrors.upstreamApiKey }}</p>
      </label>
      <div v-if="duplicateCandidate" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
        {{ tM('candidateForm.duplicateAccountBinding', { id: duplicateCandidate.id }) }}
      </div>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelProbeModel') }}</span>
          <input v-model.trim="candidateForm.probe_model" class="input w-full" :class="candidateFormErrors.probeModel ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''" :aria-invalid="Boolean(candidateFormErrors.probeModel)" type="text" />
          <p v-if="candidateFormErrors.probeModel" class="text-xs text-red-600 dark:text-red-300">{{ candidateFormErrors.probeModel }}</p>
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelProtocol') }}</span>
          <select v-model="candidateForm.probe_protocol" class="input w-full">
            <option value="chat_completions">chat_completions</option>
            <option value="responses">responses</option>
            <option value="anthropic">anthropic</option>
          </select>
        </label>
      </div>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="candidateForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
        {{ tM('candidateForm.enableCandidate') }}
      </label>
      <div v-if="candidateFormServerError" data-testid="candidate-form-server-error" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
        <div class="font-medium">{{ tM('candidateForm.saveFailedTitle') }}</div>
        <div class="mt-1 text-xs">{{ tM('candidateForm.saveFailedImpact') }}</div>
        <div class="mt-1 text-xs">{{ tM('candidateForm.saveFailedAdvice') }}</div>
        <details class="mt-2 text-xs">
          <summary class="cursor-pointer select-none">{{ tM('metricsRefresh.technicalDetails') }}</summary>
          <div class="mt-1 break-words font-mono">{{ candidateFormServerError }}</div>
        </details>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="closeCandidateDialog">{{ tM('candidateForm.cancel') }}</button>
        <button v-if="!candidateForm.id" class="btn btn-secondary" type="button" :disabled="savingCandidate" @click="submitCandidate({ continueAdding: true })">
          {{ savingCandidate ? tM('candidateForm.saving') : tM('candidateForm.saveAndContinue') }}
        </button>
        <button class="btn btn-primary" type="submit" form="candidate-form" :disabled="savingCandidate">
          {{ savingCandidate ? tM('candidateForm.saving') : candidateForm.id ? tM('candidateForm.updateCandidate') : tM('candidateForm.createCandidate') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog :show="snapshotDialogOpen" :title="snapshotDialogTitle" width="extra-wide" @close="snapshotDialogOpen = false">
    <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ tM('snapshotDialog.description') }}</p>
      <button
        class="btn btn-secondary inline-flex items-center justify-center gap-2"
        type="button"
        :disabled="!snapshotConnector || manualSnapshotRefreshingId === snapshotConnector.id"
        @click="snapshotConnector && sync(snapshotConnector)"
      >
        <Icon name="refresh" size="sm" />
        {{ snapshotConnector && manualSnapshotRefreshingId === snapshotConnector.id ? tM('snapshotDialog.fetching') : tM('snapshotDialog.fetchLatest') }}
      </button>
    </div>
    <div v-if="snapshotLoading" class="flex min-h-56 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[920px] text-sm">
        <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
          <tr>
            <th class="px-4 py-3 text-left">{{ tM('snapshotDialog.colGroupId') }}</th>
            <th class="px-4 py-3 text-left">{{ tM('snapshotDialog.colName') }}</th>
            <th class="px-4 py-3 text-left">{{ tM('snapshotDialog.colPlatform') }}</th>
            <th class="px-4 py-3 text-right">{{ tM('snapshotDialog.colDefaultRate') }}</th>
            <th class="px-4 py-3 text-right">{{ tM('snapshotDialog.colOverrideRate') }}</th>
            <th class="px-4 py-3 text-right">{{ tM('snapshotDialog.colFinalRate') }}</th>
            <th class="px-4 py-3 text-left">{{ tM('snapshotDialog.colSource') }}</th>
            <th class="px-4 py-3 text-left">{{ tM('snapshotDialog.colLastSeen') }}</th>
            <th class="px-4 py-3 text-right">{{ tM('snapshotDialog.colActions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="snapshot in snapshots" :key="snapshot.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
            <td class="px-4 py-3 font-mono text-xs">{{ snapshot.upstream_group_id }}</td>
            <td class="px-4 py-3">{{ snapshot.name || '-' }}</td>
            <td class="px-4 py-3">{{ snapshot.platform || '-' }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatRate(snapshot.default_rate_multiplier) }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatNullableRate(snapshot.override_rate_multiplier) }}</td>
            <td class="px-4 py-3 text-right font-semibold tabular-nums">{{ formatRate(snapshot.final_rate_multiplier) }}</td>
            <td class="px-4 py-3"><RateSourceTag :source="snapshot.source" /></td>
            <td class="px-4 py-3">{{ formatDate(snapshot.last_seen_at) }}</td>
            <td class="px-4 py-3 text-right">
              <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="createCandidateFromSnapshot(snapshot)">{{ tM('snapshotDialog.createCandidate') }}</button>
            </td>
          </tr>
          <tr v-if="snapshots.length === 0">
            <td colspan="9" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">{{ tM('snapshotDialog.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </BaseDialog>

  <BaseDialog :show="applyDialogOpen" :title="applyDialogTitle" width="extra-wide" @close="applyDialogOpen = false">
    <div class="space-y-4">
      <div class="rounded-lg border px-4 py-3 text-sm" :class="applyDialogNoticeClass">
        {{ applyDialogNotice }}
        <span v-if="applyRun?.applied_at" class="ml-1">{{ tM('applyDialog.appliedAt', { date: formatDate(applyRun.applied_at) }) }}</span>
      </div>
      <OperationResultPanel
        v-if="applyOperationFeedback"
        v-bind="applyOperationFeedback"
        :dismissible="!applying"
        data-testid="apply-operation-result"
        @dismiss="lastApplyError = ''; lastAppliedRun = null"
      />
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div v-for="item in applyRiskCards" :key="item.label" class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</div>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[860px] text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colAccount') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colAction') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('candidates.colMapping') }}</th>
              <th class="px-3 py-3 text-right">{{ tM('applyDialog.colChange') }}</th>
              <th class="px-3 py-3 text-right">{{ tM('applyDialog.colRate') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colConfidence') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colSummary') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="suggestion in applyRun?.suggestions || []" :key="suggestion.id || suggestion.candidate_id">
              <td class="px-3 py-3">#{{ suggestion.account_id }} {{ suggestion.account_name || '-' }}</td>
              <td class="px-3 py-3">
                <span :class="suggestionActionClass(suggestion)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ suggestionActionLabel(suggestion) }}</span>
              </td>
              <td class="px-3 py-3">{{ suggestionMappingLabel(suggestion) }}</td>
              <td class="px-3 py-3 text-right">
                <div v-if="suggestionActionType(suggestion) === 'priority_update'" class="inline-flex items-center gap-1 tabular-nums">
                  <span class="text-gray-400 dark:text-gray-500">{{ suggestion.old_priority ?? '-' }}</span>
                  <Icon :name="(priorityDeltaIcon(suggestion) as 'arrowUp' | 'arrowDown' | 'arrowRight')" size="xs" :class="priorityDeltaClass(suggestion)" />
                  <span class="font-semibold" :class="priorityDeltaClass(suggestion)">{{ suggestion.new_priority ?? '-' }}</span>
                </div>
                <span v-else :class="suggestionActionClass(suggestion)" class="font-medium">{{ suggestionSchedulableTransitionLabel(suggestion) }}</span>
              </td>
              <td class="px-3 py-3 text-right">
                <div class="tabular-nums">{{ formatRate(suggestion.final_rate_multiplier) }}</div>
              </td>
              <td class="px-3 py-3">
                <span :class="confidenceClass(suggestion.confidence)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ confidenceLabel(suggestion.confidence) }}</span>
              </td>
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">
                <div>{{ suggestion.health_summary || '-' }}</div>
                <div v-if="suggestion.reason" class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ suggestion.reason }}</div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="applyDialogOpen = false">{{ tM('applyDialog.close') }}</button>
        <button v-if="canApplySelectedRun" class="btn btn-secondary" type="button" :disabled="applying || closingRecommendationRun || restoringRecommendationRun" @click="closeSelectedRecommendationRun">
          {{ closingRecommendationRun ? tM('applyDialog.closingSuggestion') : tM('applyDialog.closeSuggestion') }}
        </button>
        <button v-if="canRestoreSelectedRun" class="btn btn-secondary" type="button" :disabled="applying || closingRecommendationRun || restoringRecommendationRun" @click="restoreSelectedRecommendationRun">
          {{ restoringRecommendationRun ? tM('applyDialog.restoringSuggestion') : tM('applyDialog.restoreSuggestion') }}
        </button>
        <button v-if="canApplySelectedRun" class="btn btn-primary" type="button" :disabled="applying" @click="applySelectedRun">
          {{ applying ? tM('applyDialog.applying') : tM('applyDialog.confirmApply') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <CandidateHealthDialog :show="!!healthDialogCandidate" :candidate="healthDialogCandidate" @close="healthDialogCandidate = null" />

  <ConfirmDialog
    :show="!!pendingDeleteConnector"
    :title="tM('confirmDeleteConnector.title')"
    :message="tM('confirmDeleteConnector.message', { name: pendingDeleteConnector?.name })"
    :danger="true"
    @confirm="confirmDeleteConnector"
    @cancel="pendingDeleteConnector = null"
  />

  <ConfirmDialog
    :show="!!pendingDeleteCandidate"
    :title="tM('confirmDeleteCandidate.title')"
    :message="tM('confirmDeleteCandidate.message', { id: pendingDeleteCandidate?.id })"
    :danger="true"
    @confirm="confirmDeleteCandidate"
    @cancel="pendingDeleteCandidate = null"
  />

</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AutoRefreshButton from '@/components/common/AutoRefreshButton.vue'
import HealthRateBar from '@/components/admin/upstreamRelay/HealthRateBar.vue'
import RateSourceTag from '@/components/admin/upstreamRelay/RateSourceTag.vue'
import CandidateHealthDialog from '@/components/admin/upstreamRelay/CandidateHealthDialog.vue'
import OperationResultPanel from '@/components/admin/upstreamRelay/OperationResultPanel.vue'
import accountsAPI from '@/api/admin/accounts'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatRelativeTime } from '@/utils/format'
import { sanitizeUrl } from '@/utils/url'
import upstreamRelayAPI, {
  type UpstreamRelayCandidate,
  type UpstreamRelayConnector,
  type UpstreamRelayAPIKeyOption,
  type UpstreamRelayBulkOperationItem,
  type UpstreamRelayBulkOperationResult,
  type UpstreamRelayConnectorMetricsRefreshResult,
  type UpstreamRelayGroupRateSnapshot,
  type UpstreamRelayGroupRateSnapshotChange,
  type UpstreamRelayGroupUsageHistory,
  type UpstreamRelayMetricsIssueDetail,
  type UpstreamRelayMetricsMissingGroupDetail,
  type UpstreamRelayMetricsUsageDetail,
  type UpstreamRelayMetricsRefreshStatus,
  type UpstreamRelayMonitoringRefreshItem,
  type UpstreamRelayMonitoringRefreshResult,
  type UpstreamRelayMonitoringJobStatus,
  type UpstreamRelayMonitoringPolicy,
  type UpstreamRelayMonitoringPolicyInput,
  type UpstreamRelayMonitoringRunnerStatus,
  type UpstreamRelayProbeProtocol,
  type UpstreamRelayRecommendationActionType,
  type UpstreamRelayRecommendationPolicy,
  type UpstreamRelayRecommendationPreview,
  type UpstreamRelayRecommendationSuggestion,
  type UpstreamRelayRecommendationRun,
  type UpstreamRelayRecommendationSortField,
  type UpstreamRelaySnapshotChangeType,
  type UpstreamRelayUsageHistorySummary
} from '@/api/admin/upstreamRelayGroupMonitors'
import type { Account, PaginatedResponse } from '@/types'

type SectionKey = 'candidates' | 'connectors' | 'usageHistory' | 'snapshotChanges' | 'monitoring' | 'recommendations' | 'policy'
type BulkOperationKind = 'sync' | 'probe'
type UsageHistoryDateShortcutKey = 'today' | 'yesterday' | 'last7d' | 'last30d'
type SuggestionActionType = UpstreamRelayRecommendationActionType
type UsageHistorySubtotal = {
  cost: number
  tokens: number
  groups: number
  latestCheckedAt: string | null
}
type UsageHistoryStatus = 'live' | 'finalized' | 'pending'
type ConnectorGroupRow =
  | { kind: 'candidate'; key: string; candidate: UpstreamRelayCandidate }
  | { kind: 'snapshot'; key: string; snapshot: UpstreamRelayGroupRateSnapshot }
type CandidateGroupOption = {
  value: string
  label: string
}
type TodayUsageOverview = {
  loaded: boolean
  cost: number
  tokens: number
  records: number
}
type MetricsRefreshResultState = {
  items: UpstreamRelayMonitoringRefreshItem[]
  updatedAt: string
  expanded: boolean
}
type MonitoringIssueAction = 'edit_candidate' | 'create_candidate' | 'sync_connector' | 'edit_connector' | 'retry_metrics'
type MonitoringIssuePresentation = {
  summary: string
  guidance: string
  action?: MonitoringIssueAction
  actionLabel?: string
  candidateId?: number
  accountId?: number
  rawDetail?: string
}
type OperationFeedback = {
  status: 'running' | 'success' | 'partial' | 'failed'
  source: string
  title: string
  action: string
  result: string
  impact: string
  nextStep: string
  technicalDetails: string[]
  actionLabel?: string
}
type CandidateProbeFeedback = {
  status: 'running' | 'done'
  candidateId: number
  accountLabel: string
  mappingLabel: string
  startedAt: string
  completedAt?: string
  success?: boolean
  latencyMs?: number | null
  httpStatus?: number | null
  errorClass?: string
  errorMessage?: string
}
type CandidateBulkProbeFeedback = {
  status: 'running' | 'done'
  startedAt: string
  completedAt?: string
  result?: UpstreamRelayBulkOperationResult
  errorMessage?: string
}

const USAGE_HISTORY_STALE_MS = 24 * 60 * 60 * 1000
const OVERVIEW_TODAY_USAGE_PAGE_SIZE = 200
const MANUAL_CANDIDATE_GROUP_OPTION = '__manual__'
const BULK_OPERATION_DETAIL_LIMIT = 5
const UPSTREAM_RELAY_USAGE_TIME_ZONE = 'Asia/Shanghai'
const DEFAULT_PAUSE_RATE_GAP_THRESHOLD = 0.04
const DEFAULT_PAUSE_CONSECUTIVE_FAILURES_THRESHOLD = 3
const METRICS_MISSING_REASON_KEYS = new Set([
  'no_snapshot',
  'missing_upstream_api_key_binding',
  'upstream_api_key_not_visible',
  'upstream_api_key_group_unavailable',
  'upstream_usage_request_failed',
  'usage_refresh_aborted',
  'usage_refresh_failed'
])

const loading = ref(false)
const error = ref('')
const connectors = ref<UpstreamRelayConnector[]>([])
const snapshots = ref<UpstreamRelayGroupRateSnapshot[]>([])
const overviewSnapshots = ref<UpstreamRelayGroupRateSnapshot[]>([])
const overviewTodayUsage = ref<TodayUsageOverview>({ loaded: false, cost: 0, tokens: 0, records: 0 })
const snapshotChanges = ref<UpstreamRelayGroupRateSnapshotChange[]>([])
const usageHistory = ref<UpstreamRelayGroupUsageHistory[]>([])
const usageHistoryBackendSummary = ref<UpstreamRelayUsageHistorySummary | null>(null)
const candidates = ref<UpstreamRelayCandidate[]>([])
const recommendationRuns = ref<UpstreamRelayRecommendationRun[]>([])
const accounts = ref<Account[]>([])
const connectorAPIKeys = ref<UpstreamRelayAPIKeyOption[]>([])
const selectedConnectorId = ref(0)
const manualSnapshotRefreshingId = ref<number | null>(null)
const refreshingMetrics = ref(false)
const refreshingMetricsConnectorId = ref<number | null>(null)
const probingId = ref<number | null>(null)
const togglingCandidateId = ref<number | null>(null)
const bulkSyncing = ref(false)
const bulkProbing = ref(false)
const generating = ref(false)
const applying = ref(false)
const closingRecommendationRun = ref(false)
const restoringRecommendationRun = ref(false)
const snapshotLoading = ref(false)
const snapshotChangesLoading = ref(false)
const usageHistoryLoading = ref(false)
const recommendationRunsLoading = ref(false)
const savingConnector = ref(false)
const savingCandidate = ref(false)
const candidateFormSubmitted = ref(false)
const candidateFormServerError = ref('')
const candidateRepairRefreshConnectorId = ref<number | null>(null)
const candidateConfigurationOnlyIncomplete = ref(false)
const loadingConnectorAPIKeys = ref(false)
const savingMonitoringPolicy = ref(false)
const savingPolicy = ref(false)
const previewLoading = ref(false)
const applyDialogOpen = ref(false)
const applyRun = ref<UpstreamRelayRecommendationRun | null>(null)
const lastAppliedRun = ref<UpstreamRelayRecommendationRun | null>(null)
const lastApplyError = ref('')
const successMessage = ref('')
const policyPreview = ref<UpstreamRelayRecommendationPreview | null>(null)
const policyPreviewLastUpdatedAt = ref<string | null>(null)
const policyPreviewResultRef = ref<HTMLElement | null>(null)
const monitoringPolicySavedAt = ref<string | null>(null)
const runnerStatus = ref<UpstreamRelayMonitoringRunnerStatus | null>(null)
const savingRunnerToggleKey = ref<RunnerJobKey | ''>('')
const runnerToggleOverrides = ref<Partial<Record<RunnerJobKey, boolean>>>({})
const freshnessClock = ref(Date.now())
const bulkOperationResult = ref<{ kind: BulkOperationKind; result: UpstreamRelayBulkOperationResult; updatedAt: string } | null>(null)
const bulkOperationRequestError = ref('')
const bulkOperationSection = ref<SectionKey | null>(null)
const preserveFeedbackOnNextSectionChange = ref(false)
const metricsRefreshResult = ref<MetricsRefreshResultState | null>(null)
const monitoringRefreshRequestError = ref('')
const candidateProbeFeedback = ref<CandidateProbeFeedback | null>(null)
const candidateBulkProbeFeedback = ref<CandidateBulkProbeFeedback | null>(null)
const activeSection = ref<SectionKey>('candidates')
const connectorDialogOpen = ref(false)
const connectorFormError = ref('')
const candidateDialogOpen = ref(false)
const snapshotDialogOpen = ref(false)
const snapshotConnector = ref<UpstreamRelayConnector | null>(null)
const lastLoadedAt = ref<string | null>(null)
const healthDialogCandidate = ref<UpstreamRelayCandidate | null>(null)
const pendingDeleteConnector = ref<UpstreamRelayConnector | null>(null)
const pendingDeleteCandidate = ref<UpstreamRelayCandidate | null>(null)
const candidateSourceSnapshot = ref<UpstreamRelayGroupRateSnapshot | null>(null)
const expandedConnectorIds = ref<Set<number>>(new Set())
const snapshotChangeConnectorId = ref(0)
const snapshotChangeType = ref<UpstreamRelaySnapshotChangeType | ''>('')
const snapshotChangeSearch = ref('')
const snapshotChangePage = ref(1)
const snapshotChangePageSize = 50
const snapshotChangeTotal = ref(0)
const snapshotChangePages = ref(1)
const usageHistoryConnectorId = ref(0)
const usageHistoryStartDate = ref(localUsageDate())
const usageHistoryEndDate = ref(localUsageDate())
const usageHistoryGroupId = ref('')
const usageHistorySearch = ref('')
const usageHistoryIncludeZeroUsage = ref(false)
const usageHistoryOnlyAnomalies = ref(false)
const usageHistoryPage = ref(1)
const usageHistoryPageSize = 50
const usageHistoryTotal = ref(0)
const usageHistoryPages = ref(1)
const usageHistoryFiltersDirty = ref(false)
const finalizingUsageKey = ref('')
const recommendationRunPage = ref(1)
const recommendationRunPageSize = 20
const recommendationRunTotal = ref(0)
const recommendationRunPages = ref(1)
const recommendationOnlyWithSuggestions = ref(false)
const appliedUsageHistoryFilters = reactive({
  connectorId: 0,
  startDate: usageHistoryStartDate.value,
  endDate: usageHistoryEndDate.value,
  groupId: '',
  search: '',
  includeZeroUsage: false
})

const AUTO_REFRESH_INTERVALS = [15, 30, 60] as const
const PASSWORD_LOGIN_NEEDS_MANUAL_SESSION_CODE = 'UPSTREAM_RELAY_PASSWORD_LOGIN_NEEDS_MANUAL_SESSION'
const autoRefreshEnabled = ref(false)
const autoRefreshInterval = ref(30)
const autoRefreshCountdown = ref(30)
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null
let freshnessTimer: ReturnType<typeof setInterval> | null = null

const { t } = useI18n()
const tM = (k: string, params?: Record<string, unknown>) => t(`admin.upstreamRelayGroupMonitoring.${k}`, params ?? {})

const DEFAULT_POLICY_SORT_FIELDS: UpstreamRelayRecommendationSortField[] = ['rate_asc', 'success_rate_desc', 'latency_asc']

const connectorForm = reactive({
  id: 0,
  name: '',
  base_url: '',
  auth_mode: 'manual_session' as 'manual_session' | 'password_login',
  bearer_token: '',
  refresh_token: '',
  clear_refresh_token: false,
  has_refresh_token: false,
  login_email: '',
  login_password: '',
  cookie: '',
  user_agent: ''
})

const candidateForm = reactive({
  id: 0,
  connector_id: 0,
  account_id: 0,
  upstream_group_id: '',
  upstream_api_key_id: null as number | null,
  upstream_api_key_name: '',
  upstream_api_key_masked: '',
  probe_model: '',
  probe_protocol: 'chat_completions' as UpstreamRelayProbeProtocol,
  enabled: true,
  notes: ''
})

const policyForm = reactive<UpstreamRelayRecommendationPolicy>({
  snapshot_freshness_minutes: 1440,
  usage_delta_freshness_minutes: 1440,
  probe_freshness_minutes: 30,
  min_success_rate: 0.5,
  min_sample_size: 3,
  exclude_consecutive_failures: true,
  priority_start: 10,
  priority_step: 10,
  sort_fields: [...DEFAULT_POLICY_SORT_FIELDS],
  pause_rate_gap_enabled: false,
  pause_rate_gap_threshold: DEFAULT_PAUSE_RATE_GAP_THRESHOLD,
  pause_consecutive_failures_enabled: false,
  pause_consecutive_failures_threshold: DEFAULT_PAUSE_CONSECUTIVE_FAILURES_THRESHOLD,
  pause_success_rate_enabled: false
})

const monitoringPolicyForm = reactive<UpstreamRelayMonitoringPolicy>({
  auto_sync_enabled: false,
  sync_interval_minutes: 60,
  auto_probe_enabled: false,
  probe_interval_minutes: 30,
  auto_recommendation_enabled: false,
  recommendation_interval_minutes: 60,
  auto_apply_recommendations_enabled: false,
  max_auto_apply_suggestions: 20,
  max_auto_apply_priority_delta: 100,
  min_auto_apply_confidence: 'medium',
  allow_auto_apply_degraded_health: false,
  failure_retry_interval_minutes: 10,
  sync_concurrency: 2,
  probe_concurrency: 5,
  snapshot_stale_after_minutes: 180,
  usage_delta_stale_after_minutes: 180,
  probe_stale_after_minutes: 90
})
const savedMonitoringPolicy = ref<UpstreamRelayMonitoringPolicy | null>(null)

const activeConnectors = computed(() => connectors.value.filter((item) => item.status === 'active'))
const enabledCandidateCount = computed(() => candidates.value.filter((item) => item.enabled).length)
const failedCandidateCount = computed(() => candidates.value.filter((item) => candidateHealthSeverity(item) === 'failed').length)
const pendingRuns = computed(() => recommendationRuns.value.filter((item) => item.status === 'success' && !item.applied && !item.closed && item.suggestion_count > 0))
const pendingSuggestionCount = computed(() => pendingRuns.value.reduce((total, item) => total + item.suggestion_count, 0))
const latestPendingRun = computed(() => pendingRuns.value[0] || null)
const recommendationFilterSummary = computed(() => {
  return recommendationOnlyWithSuggestions.value
    ? tM('recommendations.filterOnlyWithSuggestions')
    : tM('recommendations.filterAllRuns')
})
const recommendationPageStatus = computed(() => {
  return tM('recommendations.pageInfo', {
    page: recommendationRunPage.value,
    pages: recommendationRunPages.value,
    total: recommendationRunTotal.value
  })
})
const recommendationRangeStatus = computed(() => {
  const total = recommendationRunTotal.value
  if (total <= 0) {
    return recommendationOnlyWithSuggestions.value
      ? tM('recommendations.rangeEmptyFiltered')
      : tM('recommendations.rangeEmpty')
  }
  const start = Math.min((recommendationRunPage.value - 1) * recommendationRunPageSize + 1, total)
  const end = Math.min(recommendationRunPage.value * recommendationRunPageSize, total)
  return tM('recommendations.rangeInfo', { start, end, total })
})
const pendingSuggestionMap = computed(() => {
  const map = new Map<number, UpstreamRelayRecommendationSuggestion>()
  for (const suggestion of latestPendingRun.value?.suggestions || []) {
    map.set(suggestion.candidate_id, suggestion)
  }
  return map
})
const duplicateCandidate = computed(() => {
  const upstreamGroupId = candidateForm.upstream_group_id.trim()
  if (!candidateForm.connector_id || !candidateForm.account_id || !upstreamGroupId) return null
  return candidates.value.find((item) => {
    return item.id !== candidateForm.id
      && item.connector_id === candidateForm.connector_id
      && item.account_id === candidateForm.account_id
      && item.upstream_group_id.trim() === upstreamGroupId
  }) || null
})
const candidateFormErrors = computed(() => {
  if (!candidateFormSubmitted.value) {
    return { connector: '', account: '', upstreamGroup: '', upstreamApiKey: '', probeModel: '' }
  }
  return {
    connector: candidateForm.connector_id > 0 ? '' : tM('candidateForm.requiredConnector'),
    account: candidateForm.account_id > 0 ? '' : tM('candidateForm.requiredAccount'),
    upstreamGroup: candidateForm.upstream_group_id.trim() ? '' : tM('candidateForm.requiredUpstreamGroup'),
    upstreamApiKey: Number(candidateForm.upstream_api_key_id || 0) > 0 ? '' : tM('candidateForm.requiredUpstreamApiKey'),
    probeModel: candidateForm.probe_model.trim() ? '' : tM('candidateForm.requiredProbeModel')
  }
})
const candidateFormValid = computed(() => Object.values(candidateFormErrors.value).every((message) => !message))
const incompleteCandidateCount = computed(() => candidates.value.filter(candidateConfigurationIncomplete).length)
const filteredCandidates = computed(() => candidateConfigurationOnlyIncomplete.value
  ? candidates.value.filter(candidateConfigurationIncomplete)
  : candidates.value
)
const selectedCandidateAPIKey = computed(() => {
  const id = Number(candidateForm.upstream_api_key_id || 0)
  if (!id) return null
  return connectorAPIKeys.value.find((item) => item.id === id) || null
})
const candidateCurrentAPIKeyOption = computed<UpstreamRelayAPIKeyOption | null>(() => {
  const id = Number(candidateForm.upstream_api_key_id || 0)
  if (!id) return null
  return {
    id,
    name: candidateForm.upstream_api_key_name || `Key #${id}`,
    masked_key: candidateForm.upstream_api_key_masked || undefined
  }
})
const candidateGroupOptions = computed<CandidateGroupOption[]>(() => {
  const connectorId = Number(candidateForm.connector_id || 0)
  if (!connectorId) return []
  const snapshotsByGroupId = new Map<string, UpstreamRelayGroupRateSnapshot>()
  for (const snapshot of [...overviewSnapshots.value, ...snapshots.value]) {
    if (snapshot.connector_id !== connectorId || !snapshot.upstream_group_id) continue
    snapshotsByGroupId.set(snapshot.upstream_group_id, snapshot)
  }
  return Array.from(snapshotsByGroupId.values())
    .sort((a, b) => snapshotGroupLabel(a).localeCompare(snapshotGroupLabel(b)))
    .map((snapshot) => ({
      value: snapshot.upstream_group_id,
      label: candidateGroupOptionLabel(snapshot)
    }))
})
const candidateGroupSelectValue = computed(() => {
  const groupId = candidateForm.upstream_group_id.trim()
  if (!groupId) return ''
  return candidateGroupOptions.value.some((option) => option.value === groupId)
    ? groupId
    : MANUAL_CANDIDATE_GROUP_OPTION
})
const candidateGroupManualInputVisible = computed(() => {
  const groupId = candidateForm.upstream_group_id.trim()
  return !candidateGroupOptions.value.some((option) => option.value === groupId)
})
const candidateGroupPlaceholder = computed(() => {
  if (candidateGroupOptions.value.length > 0) return tM('candidateForm.placeholderGroup')
  return candidateForm.connector_id ? tM('candidateForm.placeholderGroupEmpty') : tM('candidateForm.placeholderSelect')
})
const candidateGroupSelectHint = computed(() => {
  if (!candidateForm.connector_id) return tM('candidateForm.groupHintSelectConnector')
  if (candidateGroupOptions.value.length === 0) return tM('candidateForm.groupHintNoSnapshots')
  if (candidateGroupManualInputVisible.value) return tM('candidateForm.groupHintManual', { count: candidateGroupOptions.value.length })
  return tM('candidateForm.groupHintOptions', { count: candidateGroupOptions.value.length })
})
const candidateProbeFeedbackPanelClass = computed(() => {
  if (candidateProbeFeedback.value?.status === 'running') {
    return 'border-blue-100 bg-blue-50 text-blue-800 dark:border-blue-900/50 dark:bg-blue-950/30 dark:text-blue-100'
  }
  return candidateProbeFeedback.value?.success
    ? 'border-emerald-100 bg-emerald-50 text-emerald-800 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-100'
    : 'border-red-100 bg-red-50 text-red-800 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-100'
})
const candidateProbeFeedbackIcon = computed(() => {
  if (candidateProbeFeedback.value?.status === 'running') return 'refresh'
  return candidateProbeFeedback.value?.success ? 'check' : 'x'
})
const candidateProbeFeedbackIconClass = computed(() => (
  candidateProbeFeedback.value?.status === 'running'
    ? 'animate-spin text-blue-500 dark:text-blue-200'
    : ''
))
const candidateProbeFeedbackTitle = computed(() => {
  const feedback = candidateProbeFeedback.value
  if (!feedback) return ''
  if (feedback.status === 'running') return tM('candidates.probeRunningTitle', { account: feedback.accountLabel })
  return feedback.success
    ? tM('candidates.probeSuccessTitle', { account: feedback.accountLabel })
    : tM('candidates.probeFailedTitle', { account: feedback.accountLabel })
})
const candidateProbeFeedbackDetail = computed(() => {
  const feedback = candidateProbeFeedback.value
  if (!feedback) return ''
  if (feedback.status === 'running') {
    return tM('candidates.probeRunningDetail', {
      mapping: feedback.mappingLabel,
      time: formatDate(feedback.startedAt)
    })
  }
  return tM('candidates.probeCompletedDetail', {
    mapping: feedback.mappingLabel,
    latency: formatProbeLatency(feedback.latencyMs),
    http: feedback.httpStatus ?? '-',
    time: formatDate(feedback.completedAt || feedback.startedAt)
  })
})
const candidateProbeFeedbackError = computed(() => {
  const feedback = candidateProbeFeedback.value
  if (!feedback || feedback.status === 'running' || feedback.success) return ''
  const reason = feedback.errorMessage || feedback.errorClass || tM('candidates.probeFailedUnknown')
  const friendlyReason = friendlyUpstreamRelayError(reason)
  return feedback.errorClass ? `${errorClassLabel(feedback.errorClass)}: ${friendlyReason}` : friendlyReason
})
const candidateProbeFeedbackRawError = computed(() => {
  const raw = candidateProbeFeedback.value?.errorMessage?.trim() || ''
  return raw && raw !== candidateProbeFeedbackError.value ? raw : ''
})
const sections = computed((): Array<{ key: SectionKey; label: string; badge?: number }> => [
  { key: 'candidates', label: tM('tabs.candidates') },
  { key: 'connectors', label: tM('tabs.connectors') },
  { key: 'usageHistory', label: tM('tabs.usageHistory') },
  { key: 'snapshotChanges', label: tM('tabs.snapshotChanges') },
  { key: 'monitoring', label: tM('tabs.monitoring') },
  { key: 'recommendations', label: tM('tabs.recommendations'), badge: pendingSuggestionCount.value },
  { key: 'policy', label: tM('tabs.policy') }
])
const pageFeedbackSource = computed(() => tM(`operationResult.sources.${activeSection.value}`))
const pageErrorMessage = computed(() => friendlyUpstreamRelayError(error.value))
const pageErrorTechnicalDetail = computed(() => error.value && error.value !== pageErrorMessage.value ? error.value : '')

watch(() => candidateForm.connector_id, async (connectorId, previousConnectorId) => {
  if (!candidateDialogOpen.value) return
  if (connectorId !== previousConnectorId) {
    candidateForm.upstream_group_id = ''
    candidateSourceSnapshot.value = null
    candidateForm.upstream_api_key_id = null
    candidateForm.upstream_api_key_name = ''
    candidateForm.upstream_api_key_masked = ''
  }
  await loadConnectorAPIKeys(connectorId)
})

const normalizedMonitoringSyncInterval = computed(() => positiveInteger(monitoringPolicyForm.sync_interval_minutes, 1))
const normalizedMonitoringProbeInterval = computed(() => positiveInteger(monitoringPolicyForm.probe_interval_minutes, 1))
const normalizedMonitoringRecommendationInterval = computed(() => positiveInteger(monitoringPolicyForm.recommendation_interval_minutes, 1))
const savedMonitoringSyncInterval = computed(() => positiveInteger(savedMonitoringPolicy.value?.sync_interval_minutes, normalizedMonitoringSyncInterval.value))
const savedMonitoringProbeInterval = computed(() => positiveInteger(savedMonitoringPolicy.value?.probe_interval_minutes, normalizedMonitoringProbeInterval.value))
const savedMonitoringRecommendationInterval = computed(() => positiveInteger(savedMonitoringPolicy.value?.recommendation_interval_minutes, normalizedMonitoringRecommendationInterval.value))
const monitoringSnapshotStaleAfterMinutes = computed(() => normalizedMonitoringSyncInterval.value * 3)
const monitoringUsageDeltaStaleAfterMinutes = computed(() => normalizedMonitoringSyncInterval.value * 3)
const monitoringProbeStaleAfterMinutes = computed(() => normalizedMonitoringProbeInterval.value * 3)
const savedMonitoringSnapshotStaleAfterMinutes = computed(() => savedMonitoringSyncInterval.value * 3)
const autoMonitoringEnabled = computed(() => Boolean(savedMonitoringPolicy.value?.auto_sync_enabled || savedMonitoringPolicy.value?.auto_probe_enabled || savedMonitoringPolicy.value?.auto_recommendation_enabled))
const autoMonitoringStatusTitleKey = computed(() => autoMonitoringEnabled.value ? 'monitoring.status.running' : 'monitoring.status.disabled')
const autoMonitoringStatusDetailKey = computed(() => {
  if (savedMonitoringPolicy.value?.auto_recommendation_enabled && savedMonitoringPolicy.value?.auto_apply_recommendations_enabled) return 'monitoring.status.detailAutoApply'
  if (savedMonitoringPolicy.value?.auto_recommendation_enabled) return 'monitoring.status.detailRecommendationOnly'
  if (savedMonitoringPolicy.value?.auto_sync_enabled && savedMonitoringPolicy.value?.auto_probe_enabled) return 'monitoring.status.detailBoth'
  if (savedMonitoringPolicy.value?.auto_sync_enabled) return 'monitoring.status.detailSyncOnly'
  if (savedMonitoringPolicy.value?.auto_probe_enabled) return 'monitoring.status.detailProbeOnly'
  return 'monitoring.status.detailDisabled'
})
const autoMonitoringStatusPanelClass = computed(() => autoMonitoringEnabled.value
  ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-100'
  : 'border-gray-100 bg-gray-50 text-gray-600 dark:border-dark-700 dark:bg-dark-800/70 dark:text-gray-300'
)
const autoMonitoringStatusDotClass = computed(() => autoMonitoringEnabled.value ? 'bg-emerald-500' : 'bg-gray-400 dark:bg-gray-500')
const autoMonitoringStatusChips = computed(() => {
  const policy = savedMonitoringPolicy.value
  return [
    autoMonitoringStatusChip('sync', Boolean(policy?.auto_sync_enabled), savedMonitoringSyncInterval.value),
    autoMonitoringStatusChip('probe', Boolean(policy?.auto_probe_enabled), savedMonitoringProbeInterval.value),
    autoMonitoringStatusChip('recommendation', Boolean(policy?.auto_recommendation_enabled), savedMonitoringRecommendationInterval.value),
    autoMonitoringStatusChip('autoApply', Boolean(policy?.auto_recommendation_enabled && policy?.auto_apply_recommendations_enabled))
  ]
})

const monitoringOperationFeedback = computed<OperationFeedback | null>(() => {
  if (monitoringRefreshRequestError.value) {
    return {
      status: 'failed',
      source: tM('operationResult.sources.globalRefresh'),
      title: tM('operationResult.titles.refresh'),
      action: tM('operationResult.actions.refresh'),
      result: tM('operationResult.results.requestFailed'),
      impact: tM('operationResult.impacts.requestFailed'),
      nextStep: tM('operationResult.nextSteps.retryAfterCheck'),
      technicalDetails: [monitoringRefreshRequestError.value]
    }
  }
  if (!metricsRefreshResult.value) return null
  const summary = metricsRefreshSummary.value
  const status: OperationFeedback['status'] = summary.failed > 0
    ? (summary.success > 0 || summary.partial > 0 ? 'partial' : 'failed')
    : summary.partial > 0 ? 'partial' : 'success'
  const skipped = summary.missingGroups
  return {
    status,
    source: tM('operationResult.sources.globalRefresh'),
    title: tM('operationResult.titles.refresh'),
    action: tM('operationResult.actions.refresh'),
    result: tM('operationResult.results.monitoring', {
      success: summary.success,
      partial: summary.partial,
      failed: summary.failed,
      successItems: monitoringOperationItemNames('success'),
      failedItems: monitoringOperationItemNames('problem')
    }),
    impact: skipped > 0
      ? tM('operationResult.impacts.refreshPartial', { skipped })
      : tM('operationResult.impacts.refreshSuccess'),
    nextStep: status === 'success'
      ? tM('operationResult.nextSteps.none')
      : tM('operationResult.nextSteps.inspectConnectors'),
    technicalDetails: monitoringOperationTechnicalDetails(),
    actionLabel: status === 'success' ? undefined : tM('operationResult.actions.viewConnectorDetails')
  }
})

const bulkOperationFeedback = computed<OperationFeedback | null>(() => {
  const source = tM(`operationResult.sources.${bulkOperationSection.value || activeSection.value}`)
  if (bulkOperationRequestError.value) {
    return {
      status: 'failed', source,
      title: tM(`operationResult.titles.${bulkOperationResult.value?.kind || 'sync'}`),
      action: tM(`operationResult.actions.${bulkOperationResult.value?.kind || 'sync'}`),
      result: tM('operationResult.results.requestFailed'),
      impact: tM('operationResult.impacts.requestFailed'),
      nextStep: tM('operationResult.nextSteps.retryAfterCheck'),
      technicalDetails: [bulkOperationRequestError.value]
    }
  }
  const operation = bulkOperationResult.value
  if (!operation) return null
  return buildBulkOperationFeedback(operation.kind, operation.result, source)
})

const candidateBulkOperationFeedback = computed<OperationFeedback | null>(() => {
  const feedback = candidateBulkProbeFeedback.value
  if (!feedback) return null
  const source = tM('operationResult.sources.candidates')
  if (feedback.status === 'running') {
    return {
      status: 'running', source,
      title: tM('operationResult.titles.probe'),
      action: tM('operationResult.actions.probe'),
      result: tM('operationResult.results.running'),
      impact: tM('operationResult.impacts.probeRunning'),
      nextStep: tM('operationResult.nextSteps.wait'),
      technicalDetails: []
    }
  }
  if (feedback.errorMessage) {
    return {
      status: 'failed', source,
      title: tM('operationResult.titles.probe'),
      action: tM('operationResult.actions.probe'),
      result: tM('operationResult.results.requestFailed'),
      impact: tM('operationResult.impacts.requestFailed'),
      nextStep: tM('operationResult.nextSteps.retryAfterCheck'),
      technicalDetails: [feedback.errorMessage]
    }
  }
  return feedback.result ? buildBulkOperationFeedback('probe', feedback.result, source) : null
})

const applyOperationFeedback = computed<OperationFeedback | null>(() => {
  if (applying.value) {
    return {
      status: 'running', source: tM('operationResult.sources.priorityApply'), title: tM('operationResult.titles.apply'),
      action: tM('operationResult.actions.apply'), result: tM('operationResult.results.running'),
      impact: tM('operationResult.impacts.applyRunning'), nextStep: tM('operationResult.nextSteps.wait'), technicalDetails: []
    }
  }
  if (lastApplyError.value && applyRun.value) {
    return {
      status: 'failed', source: tM('operationResult.sources.priorityApply'), title: tM('operationResult.titles.apply'),
      action: tM('operationResult.actions.apply'), result: tM('operationResult.results.applyFailed', { id: applyRun.value.id }),
      impact: tM('operationResult.impacts.applyFailed'), nextStep: tM('operationResult.nextSteps.retryApply'),
      technicalDetails: [lastApplyError.value]
    }
  }
  if (lastAppliedRun.value && applyRun.value?.id === lastAppliedRun.value.id) {
    return {
      status: 'success', source: tM('operationResult.sources.priorityApply'), title: tM('operationResult.titles.apply'),
      action: tM('operationResult.actions.apply'), result: tM('operationResult.results.applySuccess', { id: lastAppliedRun.value.id, success: lastAppliedRun.value.suggestion_count }),
      impact: tM('operationResult.impacts.applySuccess'), nextStep: tM('operationResult.nextSteps.none'), technicalDetails: []
    }
  }
  return null
})

const metricsRefreshResultByConnectorId = computed(() => {
  const out = new Map<number, UpstreamRelayMonitoringRefreshItem>()
  for (const item of metricsRefreshResult.value?.items || []) {
    out.set(item.connector_id, item)
  }
  return out
})

const metricsRefreshSummary = computed(() => {
  const items = metricsRefreshResult.value?.items || []
  return items.reduce(
    (summary, item) => {
      summary.total++
      if (item.status === 'success') summary.success++
      else if (item.status === 'partial') summary.partial++
      else summary.failed++
      if (item.metrics?.balance_detail.status === 'success') summary.balance++
      if (item.metrics?.usage_detail.status === 'success' || item.metrics?.usage_detail.status === 'partial') summary.usage++
      summary.missingGroups += monitoringMissingGroups(item).length
      return summary
    },
    { total: 0, success: 0, partial: 0, failed: 0, balance: 0, usage: 0, missingGroups: 0 }
  )
})

const policyMinSuccessRatePercent = computed({
  get: () => Math.round((policyForm.min_success_rate || 0) * 100),
  set: (value: number) => {
    const numeric = Number.isFinite(value) ? value : 0
    policyForm.min_success_rate = Math.max(0, Math.min(100, numeric)) / 100
  }
})

const sortFieldOptions = computed((): Array<{ value: UpstreamRelayRecommendationSortField; label: string }> => [
  { value: 'rate_asc', label: tM('policy.sortRateAsc') },
  { value: 'success_rate_desc', label: tM('policy.sortSuccessRateDesc') },
  { value: 'latency_asc', label: tM('policy.sortLatencyAsc') }
])

const snapshotChangeTypeOptions = computed((): Array<{ value: UpstreamRelaySnapshotChangeType | ''; label: string }> => [
  { value: '', label: tM('snapshotChanges.allTypes') },
  { value: 'rate_changed', label: tM('snapshotChanges.typeRateChanged') },
  { value: 'added', label: tM('snapshotChanges.typeAdded') },
  { value: 'removed', label: tM('snapshotChanges.typeRemoved') }
])

const usageHistoryDateShortcuts = computed((): Array<{ key: UsageHistoryDateShortcutKey; label: string }> => [
  { key: 'today', label: tM('usageHistory.shortcutToday') },
  { key: 'yesterday', label: tM('usageHistory.shortcutYesterday') },
  { key: 'last7d', label: tM('usageHistory.shortcutLast7d') },
  { key: 'last30d', label: tM('usageHistory.shortcutLast30d') }
])

const usageHistoryDateRangeInvalid = computed(() => {
  if (!usageHistoryStartDate.value || !usageHistoryEndDate.value) return false
  return usageHistoryStartDate.value > usageHistoryEndDate.value
})

function emptyUsageHistorySubtotal(): UsageHistorySubtotal {
  return { cost: 0, tokens: 0, groups: 0, latestCheckedAt: null }
}

function summarizeUsageHistoryItems(items: UpstreamRelayGroupUsageHistory[]): UsageHistorySubtotal {
  const groupIds = new Set<string>()
  const summary = emptyUsageHistorySubtotal()
  for (const item of items) {
    groupIds.add(`${item.connector_id}:${item.upstream_group_id}`)
    const cost = Number(item.actual_cost ?? 0)
    const tokens = Number(item.total_tokens ?? 0)
    if (Number.isFinite(cost)) summary.cost += cost
    if (Number.isFinite(tokens)) summary.tokens += tokens
    if (isLaterDate(item.checked_at, summary.latestCheckedAt)) summary.latestCheckedAt = item.checked_at
  }
  summary.groups = groupIds.size
  return summary
}

const snapshotChangeGroups = computed(() => {
  const groups = new Map<number, { connectorId: number; connectorName: string; items: UpstreamRelayGroupRateSnapshotChange[] }>()
  for (const item of snapshotChanges.value) {
    const connectorName = item.connector_name || `Connector #${item.connector_id}`
    if (!groups.has(item.connector_id)) {
      groups.set(item.connector_id, { connectorId: item.connector_id, connectorName, items: [] })
    }
    groups.get(item.connector_id)!.items.push(item)
  }
  return Array.from(groups.values())
})

const usageHistoryGroups = computed(() => {
  const groups = new Map<number, { connectorId: number; connectorName: string; items: UpstreamRelayGroupUsageHistory[]; summary: UsageHistorySubtotal }>()
  for (const item of usageHistoryDisplayItems.value) {
    const connectorName = item.connector_name || `Connector #${item.connector_id}`
    if (!groups.has(item.connector_id)) {
      groups.set(item.connector_id, { connectorId: item.connector_id, connectorName, items: [], summary: emptyUsageHistorySubtotal() })
    }
    groups.get(item.connector_id)!.items.push(item)
  }
  return Array.from(groups.values()).map((group) => ({
    ...group,
    summary: summarizeUsageHistoryItems(group.items)
  }))
})

const usageHistorySummary = computed(() => {
  const backendSummary = usageHistoryBackendSummary.value
  const summary = backendSummary
    ? {
        cost: Number(backendSummary.total_cost ?? 0),
        tokens: Number(backendSummary.total_tokens ?? 0),
        groups: Number(backendSummary.group_count ?? 0),
        latestCheckedAt: backendSummary.latest_checked_at || null,
        connectors: Number(backendSummary.connector_count ?? 0),
        pendingFinalize: Number(backendSummary.pending_finalize ?? 0)
      }
    : { ...emptyUsageHistorySubtotal(), connectors: 0, pendingFinalize: 0 }
  return {
    ...summary,
    dateRange: usageHistoryDateRangeLabel.value
  }
})

const usageHistoryDisplayItems = computed(() => {
  if (!usageHistoryOnlyAnomalies.value) return usageHistory.value
  return usageHistory.value.filter((item) => usageHistoryRowFlags(item).length > 0)
})

const usageHistoryDateRangeLabel = computed(() => {
  const start = appliedUsageHistoryFilters.startDate || usageHistoryStartDate.value
  const end = appliedUsageHistoryFilters.endDate || usageHistoryEndDate.value
  if (start && end) return start === end ? start : `${start} - ${end}`
  if (start) return tM('usageHistory.rangeFrom', { date: start })
  if (end) return tM('usageHistory.rangeUntil', { date: end })
  return tM('usageHistory.rangeAll')
})

const usageHistoryAppliedFilterLabel = computed(() => {
  const connectorName = appliedUsageHistoryFilters.connectorId
    ? connectors.value.find((item) => item.id === appliedUsageHistoryFilters.connectorId)?.name || `Connector #${appliedUsageHistoryFilters.connectorId}`
    : tM('usageHistory.allConnectors')
  const usageVisibilityFilter = appliedUsageHistoryFilters.includeZeroUsage
    ? tM('usageHistory.includeZeroUsageFilter')
    : tM('usageHistory.hideZeroUsageFilter')
  const extraFilters = [usageVisibilityFilter, appliedUsageHistoryFilters.groupId, appliedUsageHistoryFilters.search].filter(Boolean)
  return tM('usageHistory.appliedFilterSummary', {
    range: usageHistoryDateRangeLabel.value,
    connector: connectorName,
    filters: extraFilters.length > 0 ? extraFilters.join(' / ') : tM('usageHistory.noExtraFilters')
  })
})

const overviewStats = computed(() => {
  const needsReauth = connectors.value.filter((item) => item.status === 'needs_reauth').length
  const latestSyncedAt = connectors.value
    .map((item) => item.last_synced_at)
    .filter((value): value is string => Boolean(value))
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0]

  return {
    activeConnectors: activeConnectors.value.length,
    totalConnectors: connectors.value.length,
    needsReauth,
    enabledCandidates: enabledCandidateCount.value,
    totalCandidates: candidates.value.length,
    failedCandidates: failedCandidateCount.value,
    pendingSuggestions: pendingSuggestionCount.value,
    todayUsageTokens: overviewTodayUsage.value.loaded ? formatUsageTokenMillions(overviewTodayUsage.value.tokens) : '-',
    todayUsageCost: overviewTodayUsage.value.loaded ? formatUsageCost(overviewTodayUsage.value.cost) : '-',
    todayUsageRecords: overviewTodayUsage.value.records,
    latestSyncedAt: formatDate(latestSyncedAt),
    latestSyncedAtRaw: latestSyncedAt ?? null
  }
})

type FreshnessTone = 'fresh' | 'stale' | 'critical' | 'none'
interface FreshnessBadge {
  tone: FreshnessTone
  label: string
  relative: string
  absolute: string
}

function computeFreshnessBadge(iso: string | null | undefined, staleAfterMinutes: number): FreshnessBadge {
  if (!iso) {
    return {
      tone: 'none',
      label: tM('freshness.never'),
      relative: tM('freshness.never'),
      absolute: '-'
    }
  }
  const now = freshnessClock.value
  const at = new Date(iso).getTime()
  if (Number.isNaN(at)) {
    return { tone: 'none', label: tM('freshness.never'), relative: '-', absolute: '-' }
  }
  const diffMinutes = Math.max(0, Math.floor((now - at) / 60000))
  const stale = Math.max(1, staleAfterMinutes)
  let tone: FreshnessTone = 'fresh'
  if (diffMinutes >= stale * 2) tone = 'critical'
  else if (diffMinutes >= stale) tone = 'stale'
  const labelKey = tone === 'fresh' ? 'freshness.fresh' : tone === 'stale' ? 'freshness.stale' : 'freshness.critical'
  return {
    tone,
    label: tM(labelKey),
    relative: formatRelativeTime(iso),
    absolute: formatDate(iso)
  }
}

const lastRefreshedFreshness = computed<FreshnessBadge>(() =>
  computeFreshnessBadge(lastLoadedAt.value, Math.max(2, Math.round(autoRefreshInterval.value / 60) * 3))
)

const lastSyncedFreshness = computed<FreshnessBadge>(() =>
  computeFreshnessBadge(overviewStats.value.latestSyncedAtRaw, savedMonitoringSnapshotStaleAfterMinutes.value)
)

function freshnessBadgeClass(tone: FreshnessTone): string {
  switch (tone) {
    case 'fresh':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    case 'stale':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

const overviewCards = computed(() => [
  {
    key: 'connector-status',
    label: tM('overview.connectorStatus'),
    value: `${overviewStats.value.activeConnectors} / ${overviewStats.value.totalConnectors}`,
    hint: tM('overview.connectorStatusHint', { n: overviewStats.value.needsReauth }),
    section: 'connectors' as SectionKey
  },
  {
    key: 'candidate-status',
    label: tM('overview.candidateStatus'),
    value: `${overviewStats.value.enabledCandidates} / ${overviewStats.value.totalCandidates}`,
    hint: tM('overview.candidateStatusHint', { n: overviewStats.value.failedCandidates }),
    section: 'candidates' as SectionKey
  },
  {
    key: 'today-usage',
    label: tM('overview.todayUsage'),
    value: overviewStats.value.todayUsageCost,
    hint: tM('overview.todayUsageHint', { tokens: overviewStats.value.todayUsageTokens, records: overviewStats.value.todayUsageRecords }),
    section: 'usageHistory' as SectionKey
  },
  {
    key: 'latest-sync',
    label: tM('overview.latestSync'),
    value: overviewStats.value.latestSyncedAt,
    hint: tM('overview.latestSyncHint'),
    section: 'connectors' as SectionKey
  }
])

type RunnerJobKey = 'sync' | 'probe' | 'recommendation' | 'finalize'
type RunnerToggleJobKey = Exclude<RunnerJobKey, 'finalize'>

interface RunnerStatusCard {
  key: RunnerJobKey
  label: string
  enabled: boolean
  autoEnabled: boolean
  inFlight: boolean
  lastFinishedAt: string | null
  lastSucceeded: boolean | null
  nextRunAt: string | null
  intervalMinutes: number
  stateLabel: string
  stateTone: 'idle' | 'running' | 'disabled' | 'success' | 'failed'
  hintText: string
  technicalDetails: string[]
  toggleKey?: RunnerToggleJobKey
  toggleDisabled: boolean
}

const RUNNER_JOB_TOGGLE_KEYS: Record<RunnerToggleJobKey, 'auto_sync_enabled' | 'auto_probe_enabled' | 'auto_recommendation_enabled'> = {
  sync: 'auto_sync_enabled',
  probe: 'auto_probe_enabled',
  recommendation: 'auto_recommendation_enabled'
}
type RunnerJobToggleField = (typeof RUNNER_JOB_TOGGLE_KEYS)[RunnerToggleJobKey]

const RUNNER_JOB_INTERVAL_KEYS: Record<RunnerToggleJobKey, 'sync_interval_minutes' | 'probe_interval_minutes' | 'recommendation_interval_minutes'> = {
  sync: 'sync_interval_minutes',
  probe: 'probe_interval_minutes',
  recommendation: 'recommendation_interval_minutes'
}

function savedRunnerAutoEnabled(key: RunnerJobKey, source?: UpstreamRelayMonitoringJobStatus): boolean {
  const override = runnerToggleOverrides.value[key]
  if (override !== undefined) return override
  const policy = savedMonitoringPolicy.value
  if (policy) return key === 'finalize' ? Boolean(policy.auto_sync_enabled) : Boolean(policy[RUNNER_JOB_TOGGLE_KEYS[key]])
  return Boolean(source?.enabled)
}

const runnerStatusCards = computed<RunnerStatusCard[]>(() => {
  const status = runnerStatus.value
  const jobs: { key: RunnerJobKey; source: UpstreamRelayMonitoringJobStatus | undefined }[] = [
    { key: 'sync', source: status?.sync },
    { key: 'probe', source: status?.probe },
    { key: 'recommendation', source: status?.recommendation },
    { key: 'finalize', source: status?.finalize }
  ]
  return jobs.map(({ key, source }) => {
    const autoEnabled = savedRunnerAutoEnabled(key, source)
    const enabled = autoEnabled
    const inFlight = !!source?.in_flight
    const lastFinishedAt = source?.last_finished_at ?? null
    const lastSucceeded = source?.last_succeeded ?? null
    const lastError = source?.last_error || ''
    const nextRunAt = source?.next_run_at ?? null
    const intervalMinutes = source?.interval_minutes ?? (key === 'finalize' ? 0 : positiveInteger(savedMonitoringPolicy.value?.[RUNNER_JOB_INTERVAL_KEYS[key]], 0))
    const failures = source?.last_failures || []

    let stateTone: RunnerStatusCard['stateTone'] = 'idle'
    let stateLabel = tM('runnerStatus.states.idle')
    if (inFlight) {
      stateTone = 'running'
      stateLabel = tM('runnerStatus.states.running')
    } else if (!enabled) {
      stateTone = 'disabled'
      stateLabel = tM('runnerStatus.states.disabled')
    } else if (lastSucceeded === false) {
      stateTone = 'failed'
      stateLabel = tM('runnerStatus.states.failed')
    } else if (lastSucceeded === true) {
      stateTone = 'success'
      stateLabel = tM('runnerStatus.states.success')
    }

    let hintText = ''
    if (inFlight) {
      hintText = tM('runnerStatus.hints.running')
    } else if (!enabled) {
      hintText = tM('runnerStatus.hints.disabled')
    } else if (lastSucceeded === false && failures.length > 0) {
      hintText = tM('runnerStatus.hints.finalizeFailures', {
        date: failures[0]?.date || '-',
        count: failures.length,
        connectors: failures.map((failure) => failure.connector_name || `#${failure.connector_id}`).join('、')
      })
    } else if (lastSucceeded === false && lastError) {
      hintText = tM('runnerStatus.hints.lastError', { error: friendlyUpstreamRelayError(lastError) })
    } else if (nextRunAt) {
      hintText = tM('runnerStatus.hints.nextRun', { time: formatDate(nextRunAt) })
    } else {
      hintText = tM('runnerStatus.hints.waiting')
    }

    return {
      key,
      label: tM(`runnerStatus.jobs.${key}`),
      enabled,
      autoEnabled,
      inFlight,
      lastFinishedAt,
      lastSucceeded,
      nextRunAt,
      intervalMinutes,
      stateLabel,
      stateTone,
      hintText,
      technicalDetails: failures.length > 0
        ? failures.map((failure) => `${failure.date} · ${failure.connector_name || `#${failure.connector_id}`}: ${failure.reason}`)
        : lastError ? [lastError] : [],
      toggleKey: key === 'finalize' ? undefined : key,
      toggleDisabled: loading.value || savingMonitoringPolicy.value || !savedMonitoringPolicy.value || savingRunnerToggleKey.value !== ''
    }
  })
})

async function toggleRunnerAutoJob(key: RunnerToggleJobKey) {
  if (loading.value || savingRunnerToggleKey.value || !savedMonitoringPolicy.value) return
  const field = RUNNER_JOB_TOGGLE_KEYS[key]
  const previous = savedRunnerAutoEnabled(key)
  const next = !previous
  runnerToggleOverrides.value = { ...runnerToggleOverrides.value, [key]: next }
  savingRunnerToggleKey.value = key
  error.value = ''
  try {
    const saved = await persistMonitoringPolicyPayload({
      ...normalizedMonitoringPolicyPayload(savedMonitoringPolicy.value),
      [field]: next
    })
    applySavedMonitoringPolicy(saved, { preserveDraft: true, changedFields: [field] })
    await refreshRunnerStatusSilent()
  } catch (err) {
    runnerToggleOverrides.value = { ...runnerToggleOverrides.value, [key]: previous }
    error.value = err instanceof Error ? err.message : tM('errors.saveMonitoringPolicyFailed')
  } finally {
    const { [key]: _removed, ...rest } = runnerToggleOverrides.value
    runnerToggleOverrides.value = rest
    savingRunnerToggleKey.value = ''
  }
}

const snapshotDialogTitle = computed(() =>
  snapshotConnector.value
    ? tM('snapshotDialog.title', { name: snapshotConnector.value.name })
    : tM('snapshotDialog.titleFallback')
)

const applyRiskCards = computed(() => {
  const suggestions = applyRun.value?.suggestions || []
  const prioritySuggestionCount = suggestions.filter((item) => suggestionActionType(item) === 'priority_update').length
  const pauseSuggestionCount = suggestions.filter((item) => suggestionActionType(item) === 'account_pause').length
  const resumeSuggestionCount = suggestions.filter((item) => suggestionActionType(item) === 'account_resume').length
  const lowConfidenceCount = suggestions.filter((item) => item.confidence === 'low' || item.confidence === 'unknown').length
  const riskyCandidateIds = new Set(
    candidates.value
      .filter((item) => candidateHealthSeverity(item) !== 'success')
      .map((item) => item.id)
  )
  const riskySuggestionCount = suggestions.filter((item) => riskyCandidateIds.has(item.candidate_id)).length
  const connectorCount = new Set(suggestions.map((item) => item.connector_id)).size
  const maxPriorityDelta = suggestions.reduce((max, item) => {
    if (suggestionActionType(item) !== 'priority_update') return max
    if (item.old_priority === null || item.old_priority === undefined) return max
    if (item.new_priority === null || item.new_priority === undefined) return max
    return Math.max(max, Math.abs(item.new_priority - item.old_priority))
  }, 0)

  return [
    { label: tM('applyDialog.riskTotal'), value: suggestions.length },
    { label: tM('applyDialog.riskPriority'), value: prioritySuggestionCount },
    { label: tM('applyDialog.riskPause'), value: pauseSuggestionCount },
    { label: tM('applyDialog.riskResume'), value: resumeSuggestionCount },
    { label: tM('applyDialog.riskLowConfidence'), value: lowConfidenceCount },
    { label: tM('applyDialog.riskFailedCandidates'), value: riskySuggestionCount },
    { label: tM('applyDialog.riskConnectors'), value: connectorCount },
    { label: tM('applyDialog.riskMaxDelta'), value: maxPriorityDelta || '-' }
  ]
})

const canApplySelectedRun = computed(() => !!applyRun.value && canApplyRecommendationRun(applyRun.value))
const canRestoreSelectedRun = computed(() => !!applyRun.value && canRestoreRecommendationRun(applyRun.value))
const applyDialogTitle = computed(() => canApplySelectedRun.value ? tM('applyDialog.title') : tM('applyDialog.detailTitle'))
const applyDialogNotice = computed(() => {
  if (!applyRun.value) return ''
  if (applyRun.value.applied) {
    return tM('applyDialog.appliedNotice', { id: applyRun.value.id, date: formatDate(applyRun.value.created_at) })
  }
  if (applyRun.value.closed) {
    return tM('applyDialog.closedNotice', { id: applyRun.value.id, date: formatDate(applyRun.value.created_at) })
  }
  if (applyRun.value.status !== 'success') {
    return applyRun.value.error_message || tM('applyDialog.notSuccessNotice', { id: applyRun.value.id, status: applyRun.value.status })
  }
  if (applyRun.value.suggestion_count === 0) {
    return tM('applyDialog.noSuggestionsNotice', { id: applyRun.value.id, date: formatDate(applyRun.value.created_at) })
  }
  return tM('applyDialog.warning', { id: applyRun.value.id, date: formatDate(applyRun.value.created_at) })
})
const applyDialogNoticeClass = computed(() => {
  if (!applyRun.value) return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200'
  if (canApplySelectedRun.value) return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200'
  if (applyRun.value.applied) return 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200'
  return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200'
})

onMounted(() => {
  loadAll()
  freshnessTimer = setInterval(() => {
    freshnessClock.value = Date.now()
  }, 60_000)
})

onBeforeUnmount(() => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  if (freshnessTimer) clearInterval(freshnessTimer)
})

watch(autoRefreshEnabled, (val) => {
  if (val) startAutoRefresh()
  else stopAutoRefresh()
})

watch(autoRefreshInterval, () => {
  if (autoRefreshEnabled.value) startAutoRefresh()
})

watch(activeSection, (section, previousSection) => {
  if (section !== previousSection) {
    error.value = ''
    successMessage.value = ''
    if (preserveFeedbackOnNextSectionChange.value) {
      preserveFeedbackOnNextSectionChange.value = false
    } else {
      metricsRefreshResult.value = null
      monitoringRefreshRequestError.value = ''
      clearBulkOperationFeedback()
    }
    if (section !== 'candidates') {
      candidateProbeFeedback.value = null
      candidateBulkProbeFeedback.value = null
      candidateConfigurationOnlyIncomplete.value = false
    }
  }
  if (section === 'snapshotChanges' && snapshotChanges.value.length === 0 && !snapshotChangesLoading.value) {
    void loadSnapshotChanges()
  }
  if (section === 'usageHistory' && usageHistory.value.length === 0 && !usageHistoryLoading.value) {
    void loadUsageHistory()
  }
})

watch([snapshotChangeConnectorId, snapshotChangeType], () => {
  if (activeSection.value === 'snapshotChanges') {
    snapshotChangePage.value = 1
    void loadSnapshotChanges()
  }
})

watch([usageHistoryConnectorId, usageHistoryStartDate, usageHistoryEndDate, usageHistoryGroupId, usageHistorySearch, usageHistoryIncludeZeroUsage], () => {
  if (activeSection.value === 'usageHistory') {
    usageHistoryFiltersDirty.value = usageHistoryDraftFiltersChanged()
  }
})

async function loadAllPages<T>(
  loader: (params: { page: number; page_size: number }) => Promise<PaginatedResponse<T>>,
  pageSize = 100
) {
  const first = await loader({ page: 1, page_size: pageSize })
  const items = [...first.items]
  const pages = Math.max(1, first.pages || Math.ceil((first.total || items.length) / pageSize))
  for (let page = 2; page <= pages; page++) {
    const res = await loader({ page, page_size: pageSize })
    items.push(...res.items)
  }
  return items
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [connectorItems, candidateItems, runRes, accountRes, policy, monitoringPolicy, todayUsage, runnerStatusRes] = await Promise.all([
      loadAllPages<UpstreamRelayConnector>((params) => upstreamRelayAPI.listConnectors(params)),
      loadAllPages<UpstreamRelayCandidate>((params) => upstreamRelayAPI.listCandidates(params)),
      upstreamRelayAPI.listRecommendationRuns({
        page: recommendationRunPage.value,
        page_size: recommendationRunPageSize,
        has_suggestions: recommendationOnlyWithSuggestions.value || undefined
      }),
      accountsAPI.list(1, 200, { status: 'active' }),
      upstreamRelayAPI.getRecommendationPolicy(),
      upstreamRelayAPI.getMonitoringPolicy(),
      loadTodayUsageOverview(),
      upstreamRelayAPI.getRunnerStatus().catch(() => null)
    ])
    connectors.value = connectorItems
    candidates.value = candidateItems
    recommendationRuns.value = runRes.items
    recommendationRunTotal.value = runRes.total
    recommendationRunPages.value = runRes.pages || 1
    recommendationRunPage.value = runRes.page || recommendationRunPage.value
    accounts.value = accountRes.items
    overviewTodayUsage.value = todayUsage
    runnerStatus.value = runnerStatusRes
    assignPolicyForm(policy)
    assignMonitoringPolicyForm(monitoringPolicy)
    if (!selectedConnectorId.value && connectors.value.length > 0) {
      selectedConnectorId.value = connectors.value[0].id
    }
    await loadOverviewSnapshots(connectors.value)
    lastLoadedAt.value = new Date().toISOString()
    await hydrateLatestPendingRun()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadFailed')
  } finally {
    loading.value = false
  }
}

async function refreshCandidatesSilent() {
  try {
    candidates.value = await loadAllPages<UpstreamRelayCandidate>((params) => upstreamRelayAPI.listCandidates(params))
    lastLoadedAt.value = new Date().toISOString()
    await hydrateLatestPendingRun()
  } catch { /* silent */ }
}

async function refreshRunnerStatusSilent() {
  try {
    runnerStatus.value = await upstreamRelayAPI.getRunnerStatus()
  } catch { /* silent */ }
}

async function autoRefreshTick() {
  if (connectors.value.length === 0) {
    await Promise.all([refreshCandidatesSilent(), refreshRunnerStatusSilent()])
    return
  }
  await Promise.all([
    refreshMonitoringData({ silent: true }),
    refreshCandidatesSilent(),
    refreshTodayUsageOverviewSilent(),
    refreshRunnerStatusSilent()
  ])
}

function startAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  autoRefreshCountdown.value = autoRefreshInterval.value
  autoRefreshTimer = setInterval(() => {
    autoRefreshCountdown.value--
    if (autoRefreshCountdown.value <= 0) {
      autoRefreshCountdown.value = autoRefreshInterval.value
      void autoRefreshTick()
    }
  }, 1000)
}

function stopAutoRefresh() {
  if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null }
  autoRefreshCountdown.value = autoRefreshInterval.value
}

async function loadSnapshots() {
  if (!selectedConnectorId.value) {
    snapshots.value = []
    return
  }
  snapshots.value = await upstreamRelayAPI.listSnapshots(selectedConnectorId.value)
}

async function loadConnectorAPIKeys(connectorId: number) {
  connectorAPIKeys.value = []
  if (!connectorId) return
  loadingConnectorAPIKeys.value = true
  try {
    connectorAPIKeys.value = mergeCurrentCandidateAPIKeyOption(await upstreamRelayAPI.listConnectorAPIKeys(connectorId))
    syncSelectedCandidateAPIKey()
  } catch (err) {
    connectorAPIKeys.value = mergeCurrentCandidateAPIKeyOption([])
    error.value = friendlyUpstreamRelayError(extractApiErrorMessage(err, tM('errors.loadApiKeysFailed')))
  } finally {
    loadingConnectorAPIKeys.value = false
  }
}

function mergeCurrentCandidateAPIKeyOption(items: UpstreamRelayAPIKeyOption[]) {
  const current = candidateCurrentAPIKeyOption.value
  if (!current || items.some((item) => item.id === current.id)) {
    return items
  }
  return [current, ...items]
}

async function loadOverviewSnapshots(connectorItems: UpstreamRelayConnector[]) {
  if (connectorItems.length === 0) {
    overviewSnapshots.value = []
    return
  }
  const snapshotGroups = await Promise.all(connectorItems.map((connector) => upstreamRelayAPI.listSnapshots(connector.id)))
  overviewSnapshots.value = snapshotGroups.flat()
}

async function loadTodayUsageOverview(): Promise<TodayUsageOverview> {
  const today = localUsageDate()
  let page = 1
  let pages = 1
  const total = { loaded: true, cost: 0, tokens: 0, records: 0 }
  do {
    const res = await upstreamRelayAPI.listUsageHistory({
      page,
      page_size: OVERVIEW_TODAY_USAGE_PAGE_SIZE,
      start_date: today,
      end_date: today
    })
    total.records += res.items.length
    for (const item of res.items) {
      const cost = Number(item.actual_cost ?? 0)
      const tokens = Number(item.total_tokens ?? 0)
      if (Number.isFinite(cost)) total.cost += cost
      if (Number.isFinite(tokens)) total.tokens += tokens
    }
    pages = Math.max(1, res.pages || 1)
    page++
  } while (page <= pages)
  return total
}

async function refreshTodayUsageOverviewSilent() {
  try {
    overviewTodayUsage.value = await loadTodayUsageOverview()
  } catch { /* silent */ }
}

function replaceOverviewSnapshotsForConnector(connectorId: number, nextSnapshots: UpstreamRelayGroupRateSnapshot[]) {
  overviewSnapshots.value = [
    ...overviewSnapshots.value.filter((snapshot) => snapshot.connector_id !== connectorId),
    ...nextSnapshots
  ]
}

async function loadSnapshotChanges() {
  snapshotChangesLoading.value = true
  error.value = ''
  try {
    const res = await upstreamRelayAPI.listSnapshotChanges({
      page: snapshotChangePage.value,
      page_size: snapshotChangePageSize,
      connector_id: snapshotChangeConnectorId.value || undefined,
      change_type: snapshotChangeType.value || undefined,
      search: snapshotChangeSearch.value || undefined
    })
    snapshotChanges.value = res.items
    snapshotChangeTotal.value = res.total
    snapshotChangePages.value = res.pages || 1
    snapshotChangePage.value = res.page || snapshotChangePage.value
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadSnapshotChangesFailed')
  } finally {
    snapshotChangesLoading.value = false
  }
}

function reloadSnapshotChanges() {
  snapshotChangePage.value = 1
  void loadSnapshotChanges()
}

function changeSnapshotChangePage(page: number) {
  snapshotChangePage.value = Math.max(1, Math.min(page, snapshotChangePages.value))
  void loadSnapshotChanges()
}

async function loadUsageHistory() {
  usageHistoryLoading.value = true
  error.value = ''
  try {
    const res = await upstreamRelayAPI.listUsageHistory({
      page: usageHistoryPage.value,
      page_size: usageHistoryPageSize,
      start_date: appliedUsageHistoryFilters.startDate || undefined,
      end_date: appliedUsageHistoryFilters.endDate || undefined,
      connector_id: appliedUsageHistoryFilters.connectorId || undefined,
      upstream_group_id: appliedUsageHistoryFilters.groupId || undefined,
      search: appliedUsageHistoryFilters.search || undefined,
      include_zero_usage: appliedUsageHistoryFilters.includeZeroUsage || undefined
    })
    usageHistory.value = res.items
    usageHistoryBackendSummary.value = res.summary
    usageHistoryTotal.value = Number(res.total || 0)
    const responsePageSize = Number(res.page_size || usageHistoryPageSize)
    const responsePages = Number(res.pages || 0)
    usageHistoryPages.value = Math.max(1, responsePages, Math.ceil(usageHistoryTotal.value / Math.max(1, responsePageSize)))
    usageHistoryPage.value = Math.min(res.page || usageHistoryPage.value, usageHistoryPages.value)
    usageHistoryFiltersDirty.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadUsageHistoryFailed')
  } finally {
    usageHistoryLoading.value = false
  }
}

function reloadUsageHistory() {
  if (usageHistoryDateRangeInvalid.value) {
    usageHistoryFiltersDirty.value = true
    return
  }
  Object.assign(appliedUsageHistoryFilters, {
    connectorId: usageHistoryConnectorId.value,
    startDate: usageHistoryStartDate.value,
    endDate: usageHistoryEndDate.value,
    groupId: usageHistoryGroupId.value,
    search: usageHistorySearch.value,
    includeZeroUsage: usageHistoryIncludeZeroUsage.value
  })
  usageHistoryPage.value = 1
  void loadUsageHistory()
}

function usageHistoryDraftFiltersChanged() {
  return usageHistoryConnectorId.value !== appliedUsageHistoryFilters.connectorId
    || usageHistoryStartDate.value !== appliedUsageHistoryFilters.startDate
    || usageHistoryEndDate.value !== appliedUsageHistoryFilters.endDate
    || usageHistoryGroupId.value !== appliedUsageHistoryFilters.groupId
    || usageHistorySearch.value !== appliedUsageHistoryFilters.search
    || usageHistoryIncludeZeroUsage.value !== appliedUsageHistoryFilters.includeZeroUsage
}

function resetUsageHistoryFilters() {
  const today = localUsageDate()
  usageHistoryConnectorId.value = 0
  usageHistoryStartDate.value = today
  usageHistoryEndDate.value = today
  usageHistoryGroupId.value = ''
  usageHistorySearch.value = ''
  usageHistoryIncludeZeroUsage.value = false
  usageHistoryOnlyAnomalies.value = false
  reloadUsageHistory()
}

function applyUsageHistoryDateShortcut(shortcut: UsageHistoryDateShortcutKey) {
  const today = localUsageDate()
  let start = today
  let end = today
  if (shortcut === 'yesterday') {
    start = addUsageDateDays(today, -1)
    end = start
  } else if (shortcut === 'last7d') {
    start = addUsageDateDays(today, -6)
  } else if (shortcut === 'last30d') {
    start = addUsageDateDays(today, -29)
  }
  usageHistoryStartDate.value = start
  usageHistoryEndDate.value = end
  reloadUsageHistory()
}

function changeUsageHistoryPage(page: number) {
  usageHistoryPage.value = Math.max(1, Math.min(page, usageHistoryPages.value))
  usageHistoryFiltersDirty.value = false
  void loadUsageHistory()
}

async function loadRecommendationRuns() {
  recommendationRunsLoading.value = true
  error.value = ''
  try {
    const res = await upstreamRelayAPI.listRecommendationRuns({
      page: recommendationRunPage.value,
      page_size: recommendationRunPageSize,
      has_suggestions: recommendationOnlyWithSuggestions.value || undefined
    })
    recommendationRuns.value = res.items
    recommendationRunTotal.value = res.total
    recommendationRunPages.value = res.pages || 1
    recommendationRunPage.value = res.page || recommendationRunPage.value
    await hydrateLatestPendingRun()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadRecommendationsFailed')
  } finally {
    recommendationRunsLoading.value = false
  }
}

function reloadRecommendationRuns() {
  recommendationRunPage.value = 1
  void loadRecommendationRuns()
}

function changeRecommendationRunPage(page: number) {
  recommendationRunPage.value = Math.max(1, Math.min(page, recommendationRunPages.value))
  void loadRecommendationRuns()
}

async function hydrateLatestPendingRun() {
  const run = recommendationRuns.value.find((item) => canApplyRecommendationRun(item))
  if (!run || run.suggestions) return
  try {
    const detail = await upstreamRelayAPI.getRecommendationRun(run.id)
    recommendationRuns.value = recommendationRuns.value.map((item) => (item.id === detail.id ? detail : item))
  } catch {
    // 候选主表仍可正常使用；建议明细加载失败时仅不展示行内 priority 差异。
  }
}

function resetConnectorForm() {
  Object.assign(connectorForm, { id: 0, name: '', base_url: '', auth_mode: 'manual_session', bearer_token: '', refresh_token: '', clear_refresh_token: false, has_refresh_token: false, login_email: '', login_password: '', cookie: '', user_agent: '' })
  connectorFormError.value = ''
}

function openCreateConnector() {
  resetConnectorForm()
  connectorDialogOpen.value = true
}

function setConnectorAuthMode(mode: 'manual_session' | 'password_login') {
  connectorForm.auth_mode = mode
  connectorDialogOpen.value = true
  connectorFormError.value = ''
}

function closeConnectorDialog() {
  connectorDialogOpen.value = false
  resetConnectorForm()
}

function keepConnectorDialogOpen() {
  connectorDialogOpen.value = true
}

function formatConnectorSaveError(err: unknown): string {
  const rawMessage = extractApiErrorMessage(err, tM('errors.saveConnectorFailed'))
  if (extractApiErrorCode(err) !== PASSWORD_LOGIN_NEEDS_MANUAL_SESSION_CODE) {
    return rawMessage
  }

  return [
    tM('connectorForm.passwordLoginNeedsManualSession.reason'),
    tM('connectorForm.passwordLoginNeedsManualSession.action'),
    tM('connectorForm.passwordLoginNeedsManualSession.original', { message: rawMessage })
  ].join('\n')
}

function editConnector(connector: UpstreamRelayConnector) {
  connectorFormError.value = ''
  Object.assign(connectorForm, {
    id: connector.id,
    name: connector.name,
    base_url: connector.base_url,
    auth_mode: connector.auth_mode,
    bearer_token: '',
    refresh_token: '',
    clear_refresh_token: false,
    has_refresh_token: connector.has_refresh_token,
    login_email: '',
    login_password: '',
    cookie: '',
    user_agent: ''
  })
  connectorDialogOpen.value = true
}

async function submitConnector() {
  savingConnector.value = true
  connectorFormError.value = ''
  try {
    const payload = {
      name: connectorForm.name,
      base_url: connectorForm.base_url,
      auth_mode: connectorForm.auth_mode,
      bearer_token: connectorForm.auth_mode === 'manual_session' ? connectorForm.bearer_token || undefined : undefined,
      refresh_token: connectorForm.auth_mode === 'manual_session'
        ? connectorForm.clear_refresh_token ? '' : connectorForm.refresh_token || undefined
        : undefined,
      login_email: connectorForm.auth_mode === 'password_login' ? connectorForm.login_email || undefined : undefined,
      login_password: connectorForm.auth_mode === 'password_login' ? connectorForm.login_password || undefined : undefined,
      cookie: connectorForm.auth_mode === 'manual_session' ? connectorForm.cookie || undefined : undefined,
      user_agent: connectorForm.auth_mode === 'manual_session' ? connectorForm.user_agent || undefined : undefined
    }
    if (connectorForm.id) {
      await upstreamRelayAPI.updateConnector(connectorForm.id, payload)
    } else {
      await upstreamRelayAPI.createConnector(payload)
    }
    connectorDialogOpen.value = false
    resetConnectorForm()
    await loadAll()
  } catch (err) {
    connectorFormError.value = formatConnectorSaveError(err)
  } finally {
    savingConnector.value = false
  }
}

async function sync(connector: UpstreamRelayConnector) {
  manualSnapshotRefreshingId.value = connector.id
  error.value = ''
  successMessage.value = ''
  try {
    snapshots.value = await upstreamRelayAPI.syncConnector(connector.id)
    replaceOverviewSnapshotsForConnector(connector.id, snapshots.value)
    selectedConnectorId.value = connector.id
    snapshotConnector.value = connector
    snapshotDialogOpen.value = true
    successMessage.value = tM('snapshotDialog.fetchSuccess', { count: snapshots.value.length })
    // 静默刷新连接器列表以更新 last_synced_at，不触发全页 loading
    connectors.value = await loadAllPages<UpstreamRelayConnector>((params) => upstreamRelayAPI.listConnectors(params))
    if (activeSection.value === 'snapshotChanges') {
      await loadSnapshotChanges()
    }
  } catch (err) {
    error.value = friendlyUpstreamRelayError(err instanceof Error ? err.message : tM('errors.syncFailed'))
  } finally {
    manualSnapshotRefreshingId.value = null
  }
}

async function syncAllConnectors() {
  if (connectors.value.length === 0) return
  bulkSyncing.value = true
  bulkOperationSection.value = activeSection.value
  bulkOperationRequestError.value = ''
  bulkOperationResult.value = null
  error.value = ''
  successMessage.value = ''
  try {
    const result = await upstreamRelayAPI.syncAllConnectors()
    bulkOperationResult.value = { kind: 'sync', result, updatedAt: new Date().toISOString() }
    await loadAll()
    if (activeSection.value === 'snapshotChanges') {
      await loadSnapshotChanges()
    }
  } catch (err) {
    bulkOperationRequestError.value = extractApiErrorMessage(err, tM('errors.syncAllFailed'))
  } finally {
    bulkSyncing.value = false
  }
}

async function refreshMetricsForConnector(connector: UpstreamRelayConnector): Promise<UpstreamRelayConnectorMetricsRefreshResult> {
  const result = await upstreamRelayAPI.refreshConnectorMetrics(connector.id)
  connectors.value = connectors.value.map((item) => (item.id === connector.id ? result.connector : item))
  replaceOverviewSnapshotsForConnector(connector.id, result.snapshots)
  if (selectedConnectorId.value === connector.id) {
    snapshots.value = result.snapshots
    snapshotConnector.value = result.connector
  }
  return result
}

function setMonitoringRefreshResult(result: UpstreamRelayMonitoringRefreshResult, expanded?: boolean) {
  metricsRefreshResult.value = {
    items: result.items || [],
    updatedAt: result.refreshed_at || new Date().toISOString(),
    expanded: expanded ?? result.status !== 'success'
  }
}

function monitoringRefreshItemFromMetrics(result: UpstreamRelayConnectorMetricsRefreshResult): UpstreamRelayMonitoringRefreshItem {
  return {
    connector_id: result.connector.id,
    connector_name: result.connector.name,
    connector: result.connector,
    status: result.status,
    snapshot_status: result.snapshots.length > 0 ? 'success' : 'skipped',
    snapshot_count: result.snapshots.length,
    snapshots: result.snapshots,
    metrics: result,
    error_reason: metricsRefreshWarning(result) || undefined
  }
}

function mergeMetricsRefreshResultItem(result: UpstreamRelayConnectorMetricsRefreshResult) {
  const currentItems = metricsRefreshResult.value?.items || []
  const nextItem = monitoringRefreshItemFromMetrics(result)
  const nextItems = currentItems.some((item) => item.connector_id === result.connector.id)
    ? currentItems.map((item) => (item.connector_id === result.connector.id ? nextItem : item))
    : [nextItem, ...currentItems]
  metricsRefreshResult.value = {
    items: nextItems,
    updatedAt: new Date().toISOString(),
    expanded: Boolean(metricsRefreshResult.value?.expanded) || result.status !== 'success'
  }
}

async function refreshMonitoringData(options: { silent?: boolean } = {}) {
  if (refreshingMetrics.value) return
  if (connectors.value.length === 0) return
  refreshingMetrics.value = true
  refreshingMetricsConnectorId.value = null
  monitoringRefreshRequestError.value = ''
  if (!options.silent) {
    error.value = ''
    successMessage.value = ''
  }
  try {
    const result = await upstreamRelayAPI.refreshMonitoringData()
    applyMonitoringRefreshResult(result, !options.silent)
    await refreshPostMonitoringData()
  } catch (err) {
    if (!options.silent) {
      monitoringRefreshRequestError.value = extractApiErrorMessage(err, tM('errors.refreshMonitoringFailed'))
    }
  } finally {
    refreshingMetrics.value = false
    refreshingMetricsConnectorId.value = null
  }
}

function applyMonitoringRefreshResult(result: UpstreamRelayMonitoringRefreshResult, showDetails: boolean) {
  const nextConnectors = new Map(connectors.value.map((item) => [item.id, item]))
  for (const item of result.items || []) {
    if (item.connector) nextConnectors.set(item.connector.id, item.connector)
    const itemSnapshots = Array.isArray(item.snapshots) ? item.snapshots : null
    const shouldApplySnapshots = Boolean(itemSnapshots && (item.snapshot_status === 'success' || itemSnapshots.length > 0))
    if (shouldApplySnapshots && itemSnapshots) replaceOverviewSnapshotsForConnector(item.connector_id, itemSnapshots)
    if (selectedConnectorId.value === item.connector_id) {
      if (shouldApplySnapshots && itemSnapshots) snapshots.value = itemSnapshots
      snapshotConnector.value = item.connector || snapshotConnector.value
    }
  }
  connectors.value = Array.from(nextConnectors.values())
  setMonitoringRefreshResult(result, showDetails ? undefined : false)
}

async function refreshPostMonitoringData() {
  await refreshCandidatesSilent()
  await refreshTodayUsageOverviewSilent()
  if (activeSection.value === 'snapshotChanges') {
    await loadSnapshotChanges()
  }
  if (activeSection.value === 'usageHistory' && !usageHistoryFiltersDirty.value) {
    await loadUsageHistory()
  }
}

async function refreshMetricsForSingleConnector(connector: UpstreamRelayConnector) {
  if (refreshingMetrics.value) return
  refreshingMetrics.value = true
  refreshingMetricsConnectorId.value = connector.id
  error.value = ''
  let result: UpstreamRelayConnectorMetricsRefreshResult
  try {
    try {
      result = await refreshMetricsForConnector(connector)
    } catch (err) {
      result = buildMetricsRefreshFailureResult(connector, err)
    }
    await refreshCandidatesSilent()
    await refreshTodayUsageOverviewSilent()
    if (activeSection.value === 'usageHistory') {
      if (!usageHistoryFiltersDirty.value) {
        await loadUsageHistory()
      }
    }
    mergeMetricsRefreshResultItem(result)
  } catch (err) {
    error.value = friendlyUpstreamRelayError(err instanceof Error ? err.message : tM('errors.refreshMetricsFailed'))
  } finally {
    refreshingMetrics.value = false
    refreshingMetricsConnectorId.value = null
  }
}

async function openSnapshotDialog(connector: UpstreamRelayConnector) {
  selectedConnectorId.value = connector.id
  snapshotConnector.value = connector
  snapshotDialogOpen.value = true
  snapshotLoading.value = true
  error.value = ''
  try {
    await loadSnapshots()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadSnapshotFailed')
  } finally {
    snapshotLoading.value = false
  }
}

function removeConnector(connector: UpstreamRelayConnector) {
  pendingDeleteConnector.value = connector
}

async function confirmDeleteConnector() {
  if (!pendingDeleteConnector.value) return
  await upstreamRelayAPI.deleteConnector(pendingDeleteConnector.value.id)
  pendingDeleteConnector.value = null
  await loadAll()
}

function resetCandidateForm() {
  Object.assign(candidateForm, {
    id: 0,
    connector_id: activeConnectors.value[0]?.id || 0,
    account_id: 0,
    upstream_group_id: '',
    upstream_api_key_id: null,
    upstream_api_key_name: '',
    upstream_api_key_masked: '',
    probe_model: '',
    probe_protocol: 'chat_completions',
    enabled: true,
    notes: ''
  })
  connectorAPIKeys.value = []
  candidateSourceSnapshot.value = null
  candidateFormSubmitted.value = false
  candidateFormServerError.value = ''
  candidateRepairRefreshConnectorId.value = null
}

async function openCreateCandidate() {
  resetCandidateForm()
  candidateDialogOpen.value = true
  await loadConnectorAPIKeys(candidateForm.connector_id)
}

function closeCandidateDialog() {
  candidateDialogOpen.value = false
  resetCandidateForm()
}

async function editCandidate(candidate: UpstreamRelayCandidate) {
  candidateSourceSnapshot.value = null
  Object.assign(candidateForm, {
    id: candidate.id,
    connector_id: candidate.connector_id,
    account_id: candidate.account_id,
    upstream_group_id: candidate.upstream_group_id,
    upstream_api_key_id: candidate.upstream_api_key_id || null,
    upstream_api_key_name: candidate.upstream_api_key_name || '',
    upstream_api_key_masked: candidate.upstream_api_key_masked || '',
    probe_model: candidate.probe_model,
    probe_protocol: candidate.probe_protocol,
    enabled: candidate.enabled,
    notes: candidate.notes || ''
  })
  candidateDialogOpen.value = true
  await loadConnectorAPIKeys(candidate.connector_id)
}

async function reuseCandidateAccount(candidate: UpstreamRelayCandidate) {
  candidateSourceSnapshot.value = null
  Object.assign(candidateForm, {
    id: 0,
    connector_id: candidate.connector_id,
    account_id: candidate.account_id,
    upstream_group_id: '',
    upstream_api_key_id: null,
    upstream_api_key_name: '',
    upstream_api_key_masked: '',
    probe_model: candidate.probe_model,
    probe_protocol: candidate.probe_protocol,
    enabled: candidate.enabled,
    notes: ''
  })
  candidateDialogOpen.value = true
  await loadConnectorAPIKeys(candidate.connector_id)
}

async function createCandidateFromSnapshot(snapshot: UpstreamRelayGroupRateSnapshot) {
  const defaults = candidateDefaultsForConnector(snapshot.connector_id || selectedConnectorId.value)
  candidateSourceSnapshot.value = snapshot
  activeSection.value = 'candidates'
  Object.assign(candidateForm, {
    id: 0,
    connector_id: snapshot.connector_id || selectedConnectorId.value,
    account_id: 0,
    upstream_group_id: snapshot.upstream_group_id,
    upstream_api_key_id: null,
    upstream_api_key_name: '',
    upstream_api_key_masked: '',
    probe_model: defaults.probe_model,
    probe_protocol: defaults.probe_protocol,
    enabled: true,
    notes: ''
  })
  snapshotDialogOpen.value = false
  candidateDialogOpen.value = true
  await loadConnectorAPIKeys(candidateForm.connector_id)
}

function candidateDefaultsForConnector(connectorId: number): { probe_model: string; probe_protocol: UpstreamRelayProbeProtocol } {
  const candidate = candidates.value.find((item) => item.connector_id === connectorId && item.probe_model)
  return {
    probe_model: candidate?.probe_model || '',
    probe_protocol: candidate?.probe_protocol || 'chat_completions'
  }
}

async function submitCandidate(options: { continueAdding?: boolean } = {}) {
  candidateFormSubmitted.value = true
  candidateFormServerError.value = ''
  if (!candidateFormValid.value) return
  savingCandidate.value = true
  error.value = ''
  const repairConnectorId = candidateRepairRefreshConnectorId.value
  let candidateSaved = false
  try {
    syncSelectedCandidateAPIKey()
    const payload = {
      connector_id: candidateForm.connector_id,
      account_id: candidateForm.account_id,
      upstream_group_id: candidateForm.upstream_group_id,
      upstream_api_key_id: candidateForm.upstream_api_key_id || null,
      upstream_api_key_name: candidateForm.upstream_api_key_name,
      upstream_api_key_masked: candidateForm.upstream_api_key_masked,
      probe_model: candidateForm.probe_model,
      probe_protocol: candidateForm.probe_protocol,
      enabled: candidateForm.enabled,
      notes: candidateForm.notes
    }
    if (candidateForm.id) {
      await upstreamRelayAPI.updateCandidate(candidateForm.id, payload)
    } else {
      await upstreamRelayAPI.createCandidate(payload)
    }
    candidateSaved = true
    if (options.continueAdding && !candidateForm.id) {
      Object.assign(candidateForm, {
        upstream_group_id: '',
        upstream_api_key_id: null,
        upstream_api_key_name: '',
        upstream_api_key_masked: '',
        notes: ''
      })
      candidateSourceSnapshot.value = null
      candidateFormSubmitted.value = false
    } else {
      candidateDialogOpen.value = false
      resetCandidateForm()
    }
    await loadAll()
    if (repairConnectorId) {
      const connector = connectors.value.find((item) => item.id === repairConnectorId)
      if (connector) {
        try {
          const refreshResult = await refreshMetricsForConnector(connector)
          await refreshCandidatesSilent()
          await refreshTodayUsageOverviewSilent()
          mergeMetricsRefreshResultItem(refreshResult)
        } catch (refreshError) {
          mergeMetricsRefreshResultItem(buildMetricsRefreshFailureResult(connector, refreshError))
          error.value = tM('candidateForm.refreshAfterSaveFailed')
        }
      }
    }
  } catch (err) {
    const message = extractApiErrorMessage(err, tM('errors.saveCandidateFailed'))
    if (candidateSaved) {
      error.value = tM('candidateForm.refreshAfterSaveFailed')
    } else {
      candidateFormServerError.value = message
    }
  } finally {
    savingCandidate.value = false
  }
}

function candidateConfigurationIncomplete(candidate: UpstreamRelayCandidate) {
  return !candidate.upstream_api_key_id
}

function syncSelectedCandidateAPIKey() {
  const selected = selectedCandidateAPIKey.value
  candidateForm.upstream_api_key_name = selected?.name || ''
  candidateForm.upstream_api_key_masked = selected?.masked_key || ''
  if (selected?.group_id) {
    candidateForm.upstream_group_id = selected.group_id
    candidateSourceSnapshot.value = candidateSnapshotByGroupId(candidateForm.connector_id, selected.group_id)
  }
  if (!selected) {
    candidateForm.upstream_api_key_id = null
  }
}

function selectCandidateGroupOption(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  if (!value) {
    candidateForm.upstream_group_id = ''
    candidateSourceSnapshot.value = null
    return
  }
  if (value === MANUAL_CANDIDATE_GROUP_OPTION) {
    candidateSourceSnapshot.value = null
    return
  }
  candidateForm.upstream_group_id = value
  candidateSourceSnapshot.value = candidateSnapshotByGroupId(candidateForm.connector_id, value)
}

function syncCandidateGroupManualInput(event: Event) {
  const groupId = (event.target as HTMLInputElement).value.trim()
  candidateSourceSnapshot.value = groupId
    ? candidateSnapshotByGroupId(candidateForm.connector_id, groupId)
    : null
}

async function probe(candidate: UpstreamRelayCandidate) {
  probingId.value = candidate.id
  error.value = ''
  candidateProbeFeedback.value = {
    status: 'running',
    candidateId: candidate.id,
    accountLabel: candidateAccountLabel(candidate),
    mappingLabel: candidateMappingLabel(candidate),
    startedAt: new Date().toISOString()
  }
  try {
    const result = await upstreamRelayAPI.probeCandidate(candidate.id)
    const completedAt = result.probed_at || new Date().toISOString()
    candidateProbeFeedback.value = {
      status: 'done',
      candidateId: candidate.id,
      accountLabel: candidateAccountLabel(candidate),
      mappingLabel: candidateMappingLabel(candidate),
      startedAt: candidateProbeFeedback.value?.startedAt || completedAt,
      completedAt,
      success: result.success,
      latencyMs: result.latency_ms,
      httpStatus: result.http_status,
      errorClass: result.error_class,
      errorMessage: result.error_message
    }
    // 即时更新行内探测结果，无需整页刷新
    candidates.value = candidates.value.map((c) =>
      c.id === candidate.id ? { ...c, latest_probe: result } : c
    )
    // 后台静默刷新健康聚合数据
    refreshCandidatesSilent()
  } catch (err) {
    const message = extractApiErrorMessage(err, tM('errors.probeFailed'))
    candidateProbeFeedback.value = {
      status: 'done',
      candidateId: candidate.id,
      accountLabel: candidateAccountLabel(candidate),
      mappingLabel: candidateMappingLabel(candidate),
      startedAt: candidateProbeFeedback.value?.startedAt || new Date().toISOString(),
      completedAt: new Date().toISOString(),
      success: false,
      errorMessage: message
    }
  } finally {
    probingId.value = null
  }
}

async function probeAllCandidates() {
  if (enabledCandidateCount.value === 0) return
  bulkProbing.value = true
  bulkOperationSection.value = activeSection.value
  bulkOperationRequestError.value = ''
  bulkOperationResult.value = null
  error.value = ''
  candidateBulkProbeFeedback.value = {
    status: 'running',
    startedAt: new Date().toISOString()
  }
  try {
    const result = await upstreamRelayAPI.probeAllCandidates()
    const completedAt = new Date().toISOString()
    bulkOperationResult.value = { kind: 'probe', result, updatedAt: completedAt }
    candidateBulkProbeFeedback.value = {
      status: 'done',
      startedAt: candidateBulkProbeFeedback.value?.startedAt || completedAt,
      completedAt,
      result
    }
    await refreshCandidatesSilent()
  } catch (err) {
    const message = extractApiErrorMessage(err, tM('errors.probeAllFailed'))
    bulkOperationRequestError.value = message
    candidateBulkProbeFeedback.value = {
      status: 'done',
      startedAt: candidateBulkProbeFeedback.value?.startedAt || new Date().toISOString(),
      completedAt: new Date().toISOString(),
      errorMessage: message
    }
  } finally {
    probingId.value = null
    bulkProbing.value = false
  }
}

function clearBulkOperationFeedback() {
  bulkOperationResult.value = null
  bulkOperationRequestError.value = ''
  bulkOperationSection.value = null
}

async function toggleCandidateEnabled(candidate: UpstreamRelayCandidate) {
  togglingCandidateId.value = candidate.id
  error.value = ''
  try {
    await upstreamRelayAPI.updateCandidate(candidate.id, {
      connector_id: candidate.connector_id,
      account_id: candidate.account_id,
      upstream_group_id: candidate.upstream_group_id,
      probe_model: candidate.probe_model,
      probe_protocol: candidate.probe_protocol,
      enabled: !candidate.enabled,
      notes: candidate.notes || ''
    })
    candidates.value = candidates.value.map((item) =>
      item.id === candidate.id ? { ...item, enabled: !item.enabled } : item
    )
    void refreshCandidatesSilent()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.toggleCandidateFailed')
  } finally {
    togglingCandidateId.value = null
  }
}

function removeCandidate(candidate: UpstreamRelayCandidate) {
  pendingDeleteCandidate.value = candidate
}

async function confirmDeleteCandidate() {
  if (!pendingDeleteCandidate.value) return
  await upstreamRelayAPI.deleteCandidate(pendingDeleteCandidate.value.id)
  pendingDeleteCandidate.value = null
  await loadAll()
}

function compareRecommendationRunsByBackendOrder(a: UpstreamRelayRecommendationRun, b: UpstreamRelayRecommendationRun) {
  const aTime = new Date(a.created_at).getTime()
  const bTime = new Date(b.created_at).getTime()
  const safeATime = Number.isNaN(aTime) ? 0 : aTime
  const safeBTime = Number.isNaN(bTime) ? 0 : bTime
  if (safeATime !== safeBTime) return safeBTime - safeATime
  return b.id - a.id
}

function insertRecommendationRunByBackendOrder(run: UpstreamRelayRecommendationRun) {
  const next = recommendationRuns.value.filter((item) => item.id !== run.id)
  const insertAt = next.findIndex((item) => compareRecommendationRunsByBackendOrder(run, item) < 0)
  if (insertAt === -1) {
    recommendationRuns.value = [...next, run]
    return
  }
  recommendationRuns.value = [...next.slice(0, insertAt), run, ...next.slice(insertAt)]
}

function replaceRecommendationRunPreservingOrder(run: UpstreamRelayRecommendationRun) {
  const existingIndex = recommendationRuns.value.findIndex((item) => item.id === run.id)
  if (existingIndex === -1) {
    insertRecommendationRunByBackendOrder(run)
    return
  }
  recommendationRuns.value = recommendationRuns.value.map((item, index) => index === existingIndex ? run : item)
}

function assignPolicyForm(policy: UpstreamRelayRecommendationPolicy) {
  Object.assign(policyForm, {
    ...policy,
    sort_fields: normalizePolicySortFields(policy.sort_fields),
    pause_rate_gap_enabled: Boolean(policy.pause_rate_gap_enabled),
    pause_rate_gap_threshold: positiveNumber(policy.pause_rate_gap_threshold, DEFAULT_PAUSE_RATE_GAP_THRESHOLD),
    pause_consecutive_failures_enabled: Boolean(policy.pause_consecutive_failures_enabled),
    pause_consecutive_failures_threshold: positiveInteger(policy.pause_consecutive_failures_threshold, DEFAULT_PAUSE_CONSECUTIVE_FAILURES_THRESHOLD),
    pause_success_rate_enabled: Boolean(policy.pause_success_rate_enabled)
  })
}

function assignMonitoringPolicyForm(policy: UpstreamRelayMonitoringPolicy) {
  applySavedMonitoringPolicy(policy)
}

function normalizeMonitoringPolicy(policy: UpstreamRelayMonitoringPolicy): UpstreamRelayMonitoringPolicy {
  return {
    ...policy,
    min_auto_apply_confidence: normalizeAutoApplyConfidence(policy.min_auto_apply_confidence)
  }
}

function applySavedMonitoringPolicy(
  policy: UpstreamRelayMonitoringPolicy,
  options: { preserveDraft?: boolean; changedFields?: RunnerJobToggleField[] } = {}
) {
  const normalized = normalizeMonitoringPolicy(policy)
  savedMonitoringPolicy.value = { ...normalized }
  if (options.preserveDraft) {
    for (const field of options.changedFields || []) {
      monitoringPolicyForm[field] = Boolean(normalized[field])
    }
    return
  }
  Object.assign(monitoringPolicyForm, normalized)
}

function positiveInteger(value: unknown, fallback: number) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 1) return fallback
  return Math.floor(numeric)
}

function positiveNumber(value: unknown, fallback: number) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric <= 0) return fallback
  return numeric
}

function normalizePolicySortFields(fields: UpstreamRelayRecommendationSortField[] = []): UpstreamRelayRecommendationSortField[] {
  const allowed = new Set<UpstreamRelayRecommendationSortField>(DEFAULT_POLICY_SORT_FIELDS)
  const normalized: UpstreamRelayRecommendationSortField[] = []
  for (const field of fields) {
    if (allowed.has(field) && !normalized.includes(field)) {
      normalized.push(field)
    }
  }
  for (const field of DEFAULT_POLICY_SORT_FIELDS) {
    if (!normalized.includes(field)) {
      normalized.push(field)
    }
  }
  return normalized
}

function normalizedPolicyPayload(): UpstreamRelayRecommendationPolicy {
  const sortFields = normalizePolicySortFields(policyForm.sort_fields)
  return {
    snapshot_freshness_minutes: monitoringSnapshotStaleAfterMinutes.value,
    usage_delta_freshness_minutes: monitoringUsageDeltaStaleAfterMinutes.value,
    probe_freshness_minutes: monitoringProbeStaleAfterMinutes.value,
    min_success_rate: Math.max(0, Math.min(1, Number(policyForm.min_success_rate) || 0)),
    min_sample_size: positiveInteger(policyForm.min_sample_size, 1),
    exclude_consecutive_failures: Boolean(policyForm.exclude_consecutive_failures),
    priority_start: Number(policyForm.priority_start) || 0,
    priority_step: positiveInteger(policyForm.priority_step, 1),
    sort_fields: sortFields,
    pause_rate_gap_enabled: Boolean(policyForm.pause_rate_gap_enabled),
    pause_rate_gap_threshold: positiveNumber(policyForm.pause_rate_gap_threshold, DEFAULT_PAUSE_RATE_GAP_THRESHOLD),
    pause_consecutive_failures_enabled: Boolean(policyForm.pause_consecutive_failures_enabled),
    pause_consecutive_failures_threshold: positiveInteger(policyForm.pause_consecutive_failures_threshold, DEFAULT_PAUSE_CONSECUTIVE_FAILURES_THRESHOLD),
    pause_success_rate_enabled: Boolean(policyForm.pause_success_rate_enabled)
  }
}

function normalizedMonitoringPolicyPayload(policy: UpstreamRelayMonitoringPolicy = monitoringPolicyForm): UpstreamRelayMonitoringPolicyInput {
  return {
    auto_sync_enabled: Boolean(policy.auto_sync_enabled),
    sync_interval_minutes: positiveInteger(policy.sync_interval_minutes, 1),
    auto_probe_enabled: Boolean(policy.auto_probe_enabled),
    probe_interval_minutes: positiveInteger(policy.probe_interval_minutes, 1),
    auto_recommendation_enabled: Boolean(policy.auto_recommendation_enabled),
    recommendation_interval_minutes: positiveInteger(policy.recommendation_interval_minutes, 1),
    auto_apply_recommendations_enabled: Boolean(policy.auto_apply_recommendations_enabled),
    max_auto_apply_suggestions: positiveInteger(policy.max_auto_apply_suggestions, 1),
    max_auto_apply_priority_delta: Math.max(0, Math.floor(Number(policy.max_auto_apply_priority_delta) || 0)),
    min_auto_apply_confidence: normalizeAutoApplyConfidence(policy.min_auto_apply_confidence),
    allow_auto_apply_degraded_health: Boolean(policy.allow_auto_apply_degraded_health),
    failure_retry_interval_minutes: positiveInteger(policy.failure_retry_interval_minutes, 1),
    sync_concurrency: positiveInteger(policy.sync_concurrency, 1),
    probe_concurrency: positiveInteger(policy.probe_concurrency, 1)
  }
}

async function persistMonitoringPolicyPayload(payload: UpstreamRelayMonitoringPolicyInput): Promise<UpstreamRelayMonitoringPolicy> {
  return upstreamRelayAPI.updateMonitoringPolicy(payload)
}

function normalizeAutoApplyConfidence(value: unknown): UpstreamRelayMonitoringPolicy['min_auto_apply_confidence'] {
  if (value === 'high' || value === 'medium' || value === 'low' || value === 'unknown') return value
  return 'medium'
}

async function saveMonitoringPolicy() {
  savingMonitoringPolicy.value = true
  error.value = ''
  try {
    const saved = await persistMonitoringPolicyPayload(normalizedMonitoringPolicyPayload())
    assignMonitoringPolicyForm(saved)
    monitoringPolicySavedAt.value = saved.updated_at || new Date().toISOString()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.saveMonitoringPolicyFailed')
  } finally {
    savingMonitoringPolicy.value = false
  }
}

async function savePolicy() {
  savingPolicy.value = true
  error.value = ''
  try {
    const saved = await upstreamRelayAPI.updateRecommendationPolicy(normalizedPolicyPayload())
    assignPolicyForm(saved)
    policyPreview.value = await upstreamRelayAPI.previewRecommendations(saved)
    policyPreviewLastUpdatedAt.value = new Date().toISOString()
    await scrollToPolicyPreviewResult()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.savePolicyFailed')
  } finally {
    savingPolicy.value = false
  }
}

async function previewPolicy() {
  previewLoading.value = true
  error.value = ''
  try {
    policyPreview.value = await upstreamRelayAPI.previewRecommendations(normalizedPolicyPayload())
    policyPreviewLastUpdatedAt.value = new Date().toISOString()
    await scrollToPolicyPreviewResult()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.previewPolicyFailed')
  } finally {
    previewLoading.value = false
  }
}

async function scrollToPolicyPreviewResult() {
  await nextTick()
  policyPreviewResultRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function generateRun() {
  generating.value = true
  error.value = ''
  try {
    const run = await upstreamRelayAPI.generateRecommendations()
    const detail = await upstreamRelayAPI.getRecommendationRun(run.id)
    insertRecommendationRunByBackendOrder(detail)
    activeSection.value = 'recommendations'
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.generateFailed')
  } finally {
    generating.value = false
  }
}

async function openApplyDialog(run: UpstreamRelayRecommendationRun) {
  error.value = ''
  successMessage.value = ''
  lastAppliedRun.value = null
  lastApplyError.value = ''
  try {
    applyRun.value = await upstreamRelayAPI.getRecommendationRun(run.id)
    applyDialogOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.loadRecommendationFailed')
  }
}

async function applySelectedRun() {
  if (!applyRun.value) return
  applying.value = true
  lastApplyError.value = ''
  try {
    const appliedRun = await upstreamRelayAPI.applyRecommendationRun(applyRun.value.id)
    applyRun.value = appliedRun
    lastAppliedRun.value = appliedRun
    replaceRecommendationRunPreservingOrder(appliedRun)
    await loadAll()
  } catch (err) {
    lastApplyError.value = extractApiErrorMessage(err, tM('errors.applyFailed'))
  } finally {
    applying.value = false
  }
}

async function closeRecommendationRun(runID: number) {
  closingRecommendationRun.value = true
  error.value = ''
  try {
    const closedRun = await upstreamRelayAPI.closeRecommendationRun(runID)
    replaceRecommendationRunPreservingOrder(closedRun)
    return closedRun
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.closeRecommendationFailed')
    return null
  } finally {
    closingRecommendationRun.value = false
  }
}

async function closeSelectedRecommendationRun() {
  if (!applyRun.value) return
  const closedRun = await closeRecommendationRun(applyRun.value.id)
  if (closedRun) {
    applyRun.value = closedRun
  }
}

async function closeRecommendationRunFromList(run: UpstreamRelayRecommendationRun) {
  await closeRecommendationRun(run.id)
}

async function restoreRecommendationRun(runID: number) {
  restoringRecommendationRun.value = true
  error.value = ''
  try {
    const restoredRun = await upstreamRelayAPI.restoreRecommendationRun(runID)
    replaceRecommendationRunPreservingOrder(restoredRun)
    return restoredRun
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.restoreRecommendationFailed')
    return null
  } finally {
    restoringRecommendationRun.value = false
  }
}

async function restoreSelectedRecommendationRun() {
  if (!applyRun.value) return
  const restoredRun = await restoreRecommendationRun(applyRun.value.id)
  if (restoredRun) {
    applyRun.value = restoredRun
  }
}

async function restoreRecommendationRunFromList(run: UpstreamRelayRecommendationRun) {
  await restoreRecommendationRun(run.id)
}

function connectorStatusLabel(status: string) {
  const map: Record<string, string> = { active: tM('status.active'), needs_reauth: tM('status.needsReauth'), invalid: tM('status.invalid'), paused: tM('status.paused') }
  return map[status] || status
}

function connectorExternalUrl(connector: UpstreamRelayConnector) {
  return sanitizeUrl(connector.base_url)
}

function authModeLabel(mode: string) {
  const map: Record<string, string> = { manual_session: tM('authMode.manualSession'), password_login: tM('authMode.passwordLogin') }
  return map[mode] || mode
}

function statusClass(status: string) {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'needs_reauth') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function recommendationRunStatusLabel(run: UpstreamRelayRecommendationRun) {
  if (run.status === 'failed') return tM('recommendations.failed')
  if (run.status === 'running') return tM('recommendations.running')
  if (run.status === 'success') {
    if (run.applied) return tM('recommendations.applied')
    if (run.closed) return tM('recommendations.closed')
    if (run.suggestion_count === 0) return tM('recommendations.noSuggestions')
    return tM('recommendations.pending')
  }
  return tM('recommendations.unknown')
}

function recommendationRunStatusClass(run: UpstreamRelayRecommendationRun) {
  if (run.status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
  if (run.status === 'running') return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200'
  if (run.status === 'success') {
    if (run.applied) return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
    if (run.closed) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
    if (run.suggestion_count === 0) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
    return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  }
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function canApplyRecommendationRun(run: UpstreamRelayRecommendationRun) {
  return run.status === 'success' && !run.applied && !run.closed && run.suggestion_count > 0
}

function canRestoreRecommendationRun(run: UpstreamRelayRecommendationRun) {
  return run.status === 'success' && !run.applied && run.closed && run.suggestion_count > 0
}

function recommendationRunCreatedByLabel(run: UpstreamRelayRecommendationRun) {
  return Number(run.created_by || 0) === 0 ? tM('recommendations.sourceSystem') : tM('recommendations.sourceManual')
}

function recommendationRunAppliedByLabel(run: UpstreamRelayRecommendationRun) {
  return Number(run.applied_by || 0) === 0 ? tM('recommendations.appliedBySystem') : tM('recommendations.appliedByManual')
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function localUsageDate(value: Date = new Date()) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: UPSTREAM_RELAY_USAGE_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(value)
  const year = parts.find((part) => part.type === 'year')?.value
  const month = parts.find((part) => part.type === 'month')?.value
  const day = parts.find((part) => part.type === 'day')?.value
  if (!year || !month || !day) {
    return value.toISOString().slice(0, 10)
  }
  return `${year}-${month}-${day}`
}

function addUsageDateDays(value: string, days: number) {
  const [year, month, day] = value.split('-').map((part) => Number(part))
  const next = new Date(Date.UTC(year, month - 1, day, 12))
  next.setUTCDate(next.getUTCDate() + days)
  return next.toISOString().slice(0, 10)
}

function isLaterDate(value?: string | null, compareTo?: string | null) {
  if (!value) return false
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return false
  if (!compareTo) return true
  const compareDate = new Date(compareTo)
  if (Number.isNaN(compareDate.getTime())) return true
  return date.getTime() > compareDate.getTime()
}

function formatRate(value: number) {
  return Number(value).toFixed(4).replace(/\.?0+$/, '')
}

function formatNullableRate(value?: number | null) {
  return value === null || value === undefined ? '-' : formatRate(value)
}

function operationItemNames(items: UpstreamRelayBulkOperationItem[], success: boolean) {
  const labels = items.filter((item) => item.success === success).map(bulkOperationItemLabel)
  if (labels.length === 0) return tM('operationResult.none')
  const visible = labels.slice(0, BULK_OPERATION_DETAIL_LIMIT)
  return labels.length > visible.length
    ? tM('operationResult.itemListWithMore', { items: visible.join('、'), count: labels.length - visible.length })
    : visible.join('、')
}

function buildBulkOperationFeedback(kind: BulkOperationKind, result: UpstreamRelayBulkOperationResult, source: string): OperationFeedback {
  const status: OperationFeedback['status'] = result.failed > 0 ? (result.success > 0 ? 'partial' : 'failed') : 'success'
  return {
    status,
    source,
    title: tM(`operationResult.titles.${kind}`),
    action: tM(`operationResult.actions.${kind}`),
    result: tM('operationResult.results.bulk', {
      success: result.success,
      failed: result.failed,
      successItems: operationItemNames(result.items || [], true),
      failedItems: operationItemNames(result.items || [], false)
    }),
    impact: result.failed > 0
      ? tM(`operationResult.impacts.${kind}Partial`, { failed: result.failed })
      : tM(`operationResult.impacts.${kind}Success`),
    nextStep: result.failed > 0 ? tM('operationResult.nextSteps.fixAndRetry') : tM('operationResult.nextSteps.none'),
    technicalDetails: (result.items || []).flatMap((item) => {
      if (!item.success && item.error_reason) {
        const meta = bulkOperationItemProbeMeta(item)
        return [`${bulkOperationItemLabel(item)}: ${item.error_reason}${meta ? ` · ${meta}` : ''}`]
      }
      if (kind === 'probe' && item.success) return [`${bulkOperationItemLabel(item)}: ${bulkOperationItemSuccessDetail(item)}`]
      return []
    })
  }
}

function monitoringOperationItemNames(kind: 'success' | 'problem') {
  const items = metricsRefreshResult.value?.items || []
  const labels = items
    .filter((item) => kind === 'success' ? item.status === 'success' : item.status !== 'success')
    .map(monitoringRefreshItemConnectorLabel)
  if (labels.length === 0) return tM('operationResult.none')
  return labels.join('、')
}

function monitoringOperationTechnicalDetails() {
  const details = (metricsRefreshResult.value?.items || []).flatMap((item) => {
    const prefix = monitoringRefreshItemConnectorLabel(item)
    const messages = [
      item.snapshot_error,
      item.error_reason,
      item.metrics?.balance_error,
      item.metrics?.usage_error,
      ...(item.metrics?.usage_detail.issues || []).map((issue) => issue.message)
    ].filter((message): message is string => Boolean(message))
    return messages.map((message) => `${prefix}: ${message}`)
  })
  return Array.from(new Set(details))
}

function showMonitoringConnectorDetails() {
  preserveFeedbackOnNextSectionChange.value = true
  activeSection.value = 'connectors'
  if (metricsRefreshResult.value) metricsRefreshResult.value.expanded = true
}

function bulkOperationItemLabel(item: UpstreamRelayBulkOperationItem) {
  if (item.connector_name) return item.connector_name
  if (item.account_name) return `#${item.account_id || item.id} ${item.account_name}`
  if (item.candidate_id) return `Candidate #${item.candidate_id}`
  if (item.connector_id) return `Connector #${item.connector_id}`
  return `#${item.id}`
}

function bulkOperationItemProbeMeta(item: UpstreamRelayBulkOperationItem) {
  const parts: string[] = []
  if (item.latency_ms !== null && item.latency_ms !== undefined) {
    parts.push(tM('candidates.bulkProbeItemLatency', { latency: formatProbeLatency(item.latency_ms) }))
  }
  if (item.http_status !== null && item.http_status !== undefined) {
    parts.push(tM('candidates.bulkProbeItemHttp', { http: item.http_status }))
  }
  if (item.probed_at) {
    parts.push(tM('candidates.bulkProbeItemTime', { time: formatDate(item.probed_at) }))
  }
  return parts.join(' · ')
}

function bulkOperationItemSuccessDetail(item: UpstreamRelayBulkOperationItem) {
  return bulkOperationItemProbeMeta(item) || tM('candidates.bulkProbeSuccessNoDetail')
}

function snapshotChangeTypeLabel(change: UpstreamRelayGroupRateSnapshotChange) {
  if (change.change_type === 'added') return tM('snapshotChanges.typeAdded')
  if (change.change_type === 'removed') return tM('snapshotChanges.typeRemoved')
  return tM('snapshotChanges.typeRateChanged')
}

function snapshotChangeTypeClass(change: UpstreamRelayGroupRateSnapshotChange) {
  if (change.change_type === 'added') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (change.change_type === 'removed') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  const oldRate = change.old_final_rate_multiplier
  const newRate = change.new_final_rate_multiplier
  if (oldRate !== null && oldRate !== undefined && newRate !== null && newRate !== undefined && newRate > oldRate) {
    return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
  }
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
}

function snapshotRateDeltaLabel(change: UpstreamRelayGroupRateSnapshotChange) {
  const oldRate = change.old_final_rate_multiplier
  const newRate = change.new_final_rate_multiplier
  if (change.change_type === 'added') return tM('snapshotChanges.deltaAdded')
  if (change.change_type === 'removed') return tM('snapshotChanges.deltaRemoved')
  if (oldRate === null || oldRate === undefined || newRate === null || newRate === undefined) return '-'
  const delta = newRate - oldRate
  if (delta === 0) return tM('priorityDelta.unchanged')
  return `${delta > 0 ? '+' : ''}${formatRate(delta)}`
}

function snapshotRateDeltaClass(change: UpstreamRelayGroupRateSnapshotChange) {
  const oldRate = change.old_final_rate_multiplier
  const newRate = change.new_final_rate_multiplier
  if (change.change_type === 'added') return 'text-emerald-600 dark:text-emerald-300'
  if (change.change_type === 'removed') return 'text-gray-500 dark:text-gray-400'
  if (oldRate !== null && oldRate !== undefined && newRate !== null && newRate !== undefined && newRate > oldRate) {
    return 'text-red-600 dark:text-red-300'
  }
  return 'text-blue-600 dark:text-blue-300'
}

function formatAccountBalance(value?: number | null) {
  if (value === null || value === undefined) return '-'
  return `$${new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 4
  }).format(value)}`
}

function formatUsageCost(value?: number | null) {
  if (value === null || value === undefined) return '$0.00'
  return `$${new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: value > 0 && value < 0.01 ? 6 : 4
  }).format(value)}`
}

function formatUsageTokenMillions(value?: number | null) {
  const numeric = Number(value ?? 0)
  return `${(Number.isFinite(numeric) ? numeric / 1_000_000 : 0).toFixed(2)}M`
}

function formatUsageTokenMillionsPerQuota(cost?: number | null, tokens?: number | null) {
  const numericCost = Number(cost ?? 0)
  const numericTokens = Number(tokens ?? 0)
  if (!Number.isFinite(numericCost) || !Number.isFinite(numericTokens) || numericCost <= 0) return '-'
  return `${(numericTokens / numericCost / 1_000_000).toFixed(2)}M`
}

function formatCostPerMillionTokens(cost?: number | null, tokens?: number | null) {
  const numericCost = Number(cost ?? 0)
  const numericTokens = Number(tokens ?? 0)
  if (!Number.isFinite(numericCost) || !Number.isFinite(numericTokens) || numericTokens <= 0) return '-'
  return formatUsageCost(numericCost / (numericTokens / 1_000_000))
}

function usageHistoryRowFlags(item: UpstreamRelayGroupUsageHistory) {
  const flags: string[] = []
  const cost = Number(item.actual_cost ?? 0)
  const tokens = Number(item.total_tokens ?? 0)
  if (cost > 0 && tokens <= 0) flags.push(tM('usageHistory.flagCostWithoutTokens'))
  if (tokens > 0 && cost <= 0) flags.push(tM('usageHistory.flagTokensWithoutCost'))
  const checkedAt = new Date(item.checked_at)
  if (usageHistoryRowStatus(item) === 'live' && !Number.isNaN(checkedAt.getTime()) && Date.now() - checkedAt.getTime() > USAGE_HISTORY_STALE_MS) {
    flags.push(tM('usageHistory.flagStaleCheckedAt'))
  }
  return flags
}

function usageHistoryRowStatus(item: UpstreamRelayGroupUsageHistory): UsageHistoryStatus {
  if (item.usage_date === localUsageDate()) return 'live'
  return item.finalized_at ? 'finalized' : 'pending'
}

function usageHistoryStatusLabel(item: UpstreamRelayGroupUsageHistory) {
  const status = usageHistoryRowStatus(item)
  if (status === 'live') return tM('usageHistory.badgeLive')
  if (status === 'finalized') return tM('usageHistory.badgeFinalized')
  return tM('usageHistory.badgePendingFinalize')
}

function usageHistoryStatusBadgeClass(item: UpstreamRelayGroupUsageHistory) {
  const status = usageHistoryRowStatus(item)
  if (status === 'live') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-100'
  if (status === 'finalized') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-100'
  return 'bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-100'
}

function usageHistoryCheckedAtLabel(item: UpstreamRelayGroupUsageHistory) {
  const status = usageHistoryRowStatus(item)
  if (status === 'live') return formatRelativeTime(item.checked_at)
  if (status === 'finalized') return tM('usageHistory.finalizedAtLabel')
  return formatDate(item.checked_at)
}

function usageHistoryFinalizeKey(item: UpstreamRelayGroupUsageHistory) {
  return `${item.connector_id}:${item.usage_date}`
}

async function finalizeUsageHistoryRow(item: UpstreamRelayGroupUsageHistory) {
  const key = usageHistoryFinalizeKey(item)
  finalizingUsageKey.value = key
  error.value = ''
  try {
    await upstreamRelayAPI.finalizeUsage(item.connector_id, item.usage_date)
    await loadUsageHistory()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.finalizeUsageHistoryFailed')
  } finally {
    if (finalizingUsageKey.value === key) {
      finalizingUsageKey.value = ''
    }
  }
}

function candidateTodayUsageCostLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.today_actual_cost === null || candidate.today_actual_cost === undefined) return tM('candidates.todayUsageNotRefreshed')
  return formatUsageCost(candidate.today_actual_cost)
}

function candidateTodayUsageMetaLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.today_actual_cost === null || candidate.today_actual_cost === undefined || candidate.today_total_tokens === null || candidate.today_total_tokens === undefined) {
    return tM('candidates.todayUsageSourceMissing')
  }
  const tokens = formatUsageTokenMillions(candidate.today_total_tokens)
  return tM('candidates.todayUsageTokens', { tokens })
}

function candidateTodayUsageCompactMetaLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.today_total_tokens === null || candidate.today_total_tokens === undefined) {
    return tM('candidates.todayUsageSourceMissing')
  }
  return formatUsageTokenMillions(candidate.today_total_tokens)
}

function accountBalanceCheckedLabel(
  item: Pick<UpstreamRelayConnector, 'upstream_account_balance_checked_at'>,
  scope: 'connectors' = 'connectors'
) {
  if (!item.upstream_account_balance_checked_at) return tM(`${scope}.accountBalanceNotSynced`)
  return tM(`${scope}.accountBalanceCheckedAt`, {
    time: formatDate(item.upstream_account_balance_checked_at)
  })
}

function connectorMetricsRefreshResult(connectorID: number) {
  return metricsRefreshResultByConnectorId.value.get(connectorID) || null
}

type AutoMonitoringStatusChipKey = 'sync' | 'probe' | 'recommendation' | 'autoApply'

function autoMonitoringStatusChip(key: AutoMonitoringStatusChipKey, enabled: boolean, interval?: number) {
  const status = tM(`monitoring.statusChips.${enabled ? 'enabled' : 'disabled'}`)
  return {
    key,
    label: interval
      ? tM(`monitoring.statusChips.${key}`, { status, interval })
      : tM(`monitoring.statusChips.${key}`, { status }),
    className: enabled
      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
      : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
    dotClass: enabled ? 'bg-emerald-500' : 'bg-gray-400 dark:bg-gray-500'
  }
}

function connectorMetricsRefreshingLabel(connector: UpstreamRelayConnector) {
  return refreshingMetricsConnectorId.value === connector.id ? tM('connectors.refreshingMetrics') : tM('connectors.refreshMetrics')
}

function buildMetricsRefreshFailureResult(connector: UpstreamRelayConnector, err: unknown): UpstreamRelayConnectorMetricsRefreshResult {
  const message = err instanceof Error ? err.message : extractApiErrorMessage(err) || tM('errors.refreshMetricsFailed')
  return {
    connector,
    snapshots: [],
    status: 'failed',
    balance_detail: { status: 'failed', error: message },
    usage_detail: {
      status: 'failed',
      total_groups: connectorGroupItems(connector).length,
      updated_groups: 0,
      missing_groups: connectorGroupItems(connector).map((candidate) => ({
        upstream_group_id: candidate.upstream_group_id,
        name: candidate.upstream_group_name,
        reason: 'usage_refresh_failed',
        message
      })),
      error: message
    },
    balance_available: false,
    balance_error: message,
    usage_available: false,
    usage_error: message,
    refreshed_at: new Date().toISOString()
  }
}

function inferMetricsIssue(rawMessage: string): UpstreamRelayMetricsIssueDetail | null {
  const raw = rawMessage.trim()
  if (!raw) return null
  const missingBinding = raw.match(/candidate\s+(\d+)\s+for bound account\s+(\d+)\s+has no upstream api key binding/i)
  if (missingBinding) {
    return {
      code: 'missing_upstream_api_key_binding',
      message: raw,
      candidate_id: Number(missingBinding[1]),
      account_id: Number(missingBinding[2])
    }
  }
  if (/connector has no local account bindings/i.test(raw)) {
    return { code: 'no_candidate_bindings', message: raw }
  }
  return null
}

function isUpstreamRelayAuthError(rawMessage: string) {
  const raw = rawMessage.toLowerCase()
  return /(?:http|status)\s*(?:401|403)\b/.test(raw)
    || raw.includes('unauthorized')
    || raw.includes('forbidden')
    || raw.includes('needs reauth')
    || raw.includes('token is empty')
    || raw.includes('token expired')
    || raw.includes('refresh token is empty')
}

function friendlyUpstreamRelayError(rawMessage: string) {
  const raw = rawMessage.trim()
  if (!raw) return ''
  const issue = inferMetricsIssue(raw)
  if (issue?.code === 'missing_upstream_api_key_binding') {
    return tM('metricsRefresh.issues.missingApiKeyBinding', { account: `#${issue.account_id}` })
  }
  if (issue?.code === 'no_candidate_bindings') return tM('metricsRefresh.issues.noCandidateBindings')
  if (isUpstreamRelayAuthError(raw)) return tM('errors.connectorAuthExpired')
  const lower = raw.toLowerCase()
  if (/\b429\b/.test(lower) || lower.includes('rate limit') || lower.includes('too many requests')) return tM('errors.upstreamRateLimited')
  if (lower.includes('timeout') || lower.includes('deadline exceeded')) return tM('errors.upstreamTimeout')
  if (lower.includes('connection refused') || lower.includes('no such host') || lower.includes('network error')) return tM('errors.upstreamNetworkFailed')
  if (lower.includes('browser challenge') || lower.includes('cloudflare challenge')) return tM('errors.upstreamBrowserChallenge')
  if (/[\u3400-\u9fff]/.test(raw)) return raw
  return tM('errors.unknownUpstreamError')
}

function metricsUsageIssuePresentation(result: UpstreamRelayConnectorMetricsRefreshResult): MonitoringIssuePresentation | null {
  const detail = result.usage_detail
  const raw = detail.issue?.message || detail.error || result.usage_error || ''
  const issue = detail.issue || inferMetricsIssue(raw)
  if (issue?.code === 'missing_upstream_api_key_binding') {
    const account = issue.account_id ? `#${issue.account_id}` : tM('metricsRefresh.issues.unknownAccount')
    const summary = tM('metricsRefresh.issues.missingApiKeyBinding', { account })
    return {
      summary,
      guidance: tM('metricsRefresh.guidance.bindApiKey'),
      action: 'edit_candidate',
      actionLabel: tM('metricsRefresh.actions.bindApiKey'),
      candidateId: issue.candidate_id,
      accountId: issue.account_id,
      rawDetail: raw && raw !== summary ? raw : undefined
    }
  }
  if (issue?.code === 'upstream_api_key_not_visible' || issue?.code === 'upstream_api_key_group_unavailable') {
    const summary = tM(`metricsRefresh.issues.${issue.code === 'upstream_api_key_not_visible' ? 'apiKeyNotVisible' : 'apiKeyGroupUnavailable'}`)
    return {
      summary,
      guidance: tM('metricsRefresh.guidance.bindApiKey'),
      action: 'edit_candidate',
      actionLabel: tM('metricsRefresh.actions.bindApiKey'),
      candidateId: issue.candidate_id,
      accountId: issue.account_id,
      rawDetail: raw && raw !== summary ? raw : undefined
    }
  }
  if (issue?.code === 'no_candidate_bindings') {
    const summary = tM('metricsRefresh.issues.noCandidateBindings')
    return {
      summary,
      guidance: tM('metricsRefresh.guidance.createCandidate'),
      action: 'create_candidate',
      actionLabel: tM('metricsRefresh.actions.createCandidate'),
      rawDetail: raw && raw !== summary ? raw : undefined
    }
  }
  if (metricsMissingGroups(detail).some((group) => group.reason === 'no_snapshot')) {
    return {
      summary: tM('metricsRefresh.issues.noSnapshot'),
      guidance: tM('metricsRefresh.guidance.syncConnector'),
      action: 'sync_connector',
      actionLabel: tM('metricsRefresh.actions.syncConnector')
    }
  }
  if (issue?.code === 'upstream_usage_request_failed') {
    const account = issue.account_id ? `#${issue.account_id}` : tM('metricsRefresh.issues.unknownAccount')
    const summary = tM('metricsRefresh.issues.upstreamUsageRequestFailed', { account })
    return {
      summary,
      guidance: isUpstreamRelayAuthError(raw) ? tM('metricsRefresh.guidance.editConnectorAuth') : tM('metricsRefresh.guidance.retryUsage'),
      action: isUpstreamRelayAuthError(raw) ? 'edit_connector' : 'retry_metrics',
      actionLabel: isUpstreamRelayAuthError(raw) ? tM('metricsRefresh.actions.editConnector') : tM('metricsRefresh.actions.retry'),
      accountId: issue.account_id,
      rawDetail: raw && raw !== summary ? raw : undefined
    }
  }
  if (!raw || detail.status === 'success') return null
  const summary = friendlyUpstreamRelayError(raw)
  const authError = isUpstreamRelayAuthError(raw)
  return {
    summary,
    guidance: authError ? tM('metricsRefresh.guidance.editConnectorAuth') : tM('metricsRefresh.guidance.retryUsage'),
    action: authError ? 'edit_connector' : 'retry_metrics',
    actionLabel: authError ? tM('metricsRefresh.actions.editConnector') : tM('metricsRefresh.actions.retry'),
    rawDetail: raw !== summary ? raw : undefined
  }
}

function isActionableMetricsIssue(result: UpstreamRelayConnectorMetricsRefreshResult) {
  const raw = result.usage_detail.issue?.message || result.usage_detail.error || result.usage_error || ''
  const issue = result.usage_detail.issue || inferMetricsIssue(raw)
  return issue?.code === 'missing_upstream_api_key_binding'
    || issue?.code === 'upstream_api_key_not_visible'
    || issue?.code === 'upstream_api_key_group_unavailable'
    || issue?.code === 'no_candidate_bindings'
}

function metricsRefreshWarning(result: UpstreamRelayConnectorMetricsRefreshResult) {
  const failureReasons = metricsRefreshFailureReasons(result)
  const failureReason = failureReasons[0] || null
  if (isActionableMetricsIssue(result)) return null
  if (metricsMissingGroups(result.usage_detail).some((group) => group.reason === 'no_snapshot')) return tM('errors.metricsRefreshNeedsFullSync')
  if (result.status === 'failed') {
    if (failureReason) return friendlyUpstreamRelayError(failureReason)
    return failureReasons.length === 0 ? tM('errors.refreshMetricsFailed') : null
  }
  if (!result.balance_available && !result.usage_available && failureReason) return friendlyUpstreamRelayError(failureReason)
  return null
}

function metricsRefreshFailureReasons(result: Pick<UpstreamRelayConnectorMetricsRefreshResult, 'balance_error' | 'usage_error'>) {
  return [result.balance_error, result.usage_error].filter((reason): reason is string => Boolean(reason))
}

function metricsMissingGroups(detail: Pick<UpstreamRelayMetricsUsageDetail, 'missing_groups'>): UpstreamRelayMetricsMissingGroupDetail[] {
  return Array.isArray(detail.missing_groups) ? detail.missing_groups : []
}

function metricsRefreshStatusLabel(status: UpstreamRelayMetricsRefreshStatus) {
  return tM(`metricsRefresh.status.${status}`)
}

function metricsMissingGroupReasonLabel(reason: string) {
  return tM(`metricsRefresh.missingReasons.${METRICS_MISSING_REASON_KEYS.has(reason) ? reason : 'usage_refresh_failed'}`)
}

function metricsRefreshStatusClass(status: UpstreamRelayMetricsRefreshStatus) {
  if (status === 'success') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'partial' || status === 'skipped') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
}

function metricsRefreshStatusTextClass(status: UpstreamRelayMetricsRefreshStatus) {
  if (status === 'success') return 'text-emerald-600 dark:text-emerald-300'
  if (status === 'partial' || status === 'skipped') return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}

function metricsRefreshSummaryClass(summary: { failed: number; partial: number }) {
  if (summary.failed > 0) return metricsRefreshStatusClass('failed')
  if (summary.partial > 0) return metricsRefreshStatusClass('partial')
  return metricsRefreshStatusClass('success')
}

function metricsBalanceDetailLabel(result: UpstreamRelayConnectorMetricsRefreshResult) {
  if (result.balance_detail.status === 'success') {
    return tM('metricsRefresh.balanceSuccess', { balance: formatAccountBalance(result.balance_detail.value) })
  }
  return tM('metricsRefresh.balanceFailed', { reason: friendlyUpstreamRelayError(result.balance_detail.error || '-') })
}

function metricsUsageDetailLabel(result: UpstreamRelayConnectorMetricsRefreshResult) {
  const detail = result.usage_detail
  const missingGroups = metricsMissingGroups(detail)
  if (detail.status === 'success') {
    return tM('metricsRefresh.usageSuccess', { updated: detail.updated_groups, total: detail.total_groups })
  }
  if (detail.status === 'partial') {
    return tM('metricsRefresh.usagePartial', { updated: detail.updated_groups, total: detail.total_groups, missing: missingGroups.length })
  }
  if (detail.status === 'skipped') {
    const presentation = metricsUsageIssuePresentation(result)
    return presentation
      ? tM('metricsRefresh.usageNeedsAction', { reason: presentation.summary })
      : tM('metricsRefresh.usageSkipped', { missing: missingGroups.length })
  }
  const presentation = metricsUsageIssuePresentation(result)
  return tM('metricsRefresh.usageNeedsAction', { reason: presentation?.summary || friendlyUpstreamRelayError(detail.error || '-') })
}

function monitoringRefreshItemConnectorLabel(item: UpstreamRelayMonitoringRefreshItem) {
  return item.connector?.name || item.connector_name || `#${item.connector_id}`
}

function monitoringSnapshotDetailLabel(item: UpstreamRelayMonitoringRefreshItem) {
  if (item.snapshot_status === 'success') {
    return tM('metricsRefresh.snapshotSuccess', { count: item.snapshot_count || item.snapshots.length })
  }
  if (item.snapshot_status === 'skipped') return tM('metricsRefresh.snapshotSkipped')
  return tM('metricsRefresh.snapshotFailed', { reason: friendlyUpstreamRelayError(item.snapshot_error || '-') })
}

function monitoringBalanceDetailLabel(item: UpstreamRelayMonitoringRefreshItem) {
  if (!item.metrics) return tM('metricsRefresh.balanceSkipped')
  return metricsBalanceDetailLabel(item.metrics)
}

function monitoringUsageDetailLabel(item: UpstreamRelayMonitoringRefreshItem) {
  if (!item.metrics) return tM('metricsRefresh.usageSkipped', { missing: monitoringMissingGroups(item).length })
  return metricsUsageDetailLabel(item.metrics)
}

function monitoringRefreshItemGuidance(item: UpstreamRelayMonitoringRefreshItem) {
  return item.metrics ? metricsUsageIssuePresentation(item.metrics) : null
}

function monitoringRefreshItemStandaloneError(item: UpstreamRelayMonitoringRefreshItem) {
  if (item.metrics || item.snapshot_error) return ''
  return friendlyUpstreamRelayError(item.error_reason || '')
}

async function handleMonitoringIssueAction(item: UpstreamRelayMonitoringRefreshItem) {
  const presentation = monitoringRefreshItemGuidance(item)
  if (!presentation?.action) return
  const connector = item.connector || connectors.value.find((candidate) => candidate.id === item.connector_id)
  if (presentation.action === 'edit_candidate') {
    const candidate = candidates.value.find((entry) => entry.id === presentation.candidateId)
      || candidates.value.find((entry) => entry.connector_id === item.connector_id && entry.account_id === presentation.accountId)
    if (!candidate) {
      activeSection.value = 'candidates'
      error.value = tM('metricsRefresh.errors.candidateNotFound', { account: presentation.accountId ? `#${presentation.accountId}` : '-' })
      return
    }
    activeSection.value = 'candidates'
    await editCandidate(candidate)
    candidateRepairRefreshConnectorId.value = item.connector_id
    return
  }
  if (presentation.action === 'create_candidate') {
    activeSection.value = 'candidates'
    resetCandidateForm()
    candidateForm.connector_id = item.connector_id
    candidateRepairRefreshConnectorId.value = item.connector_id
    candidateDialogOpen.value = true
    await loadConnectorAPIKeys(item.connector_id)
    return
  }
  if (!connector) return
  if (presentation.action === 'sync_connector') {
    await sync(connector)
    return
  }
  if (presentation.action === 'edit_connector') {
    activeSection.value = 'connectors'
    editConnector(connector)
    return
  }
  await refreshMetricsForSingleConnector(connector)
}

function monitoringMissingGroups(item: UpstreamRelayMonitoringRefreshItem): UpstreamRelayMetricsMissingGroupDetail[] {
  return item.metrics ? metricsMissingGroups(item.metrics.usage_detail) : []
}

function connectorMetricsRefreshInlineLabel(item: UpstreamRelayMonitoringRefreshItem) {
  if (!item.metrics) return metricsRefreshStatusLabel(item.status)
  if (item.metrics.usage_detail.status === 'success') return tM('metricsRefresh.inlineUsageOk')
  const missingGroups = metricsMissingGroups(item.metrics.usage_detail)
  if (missingGroups.length > 0) {
    return tM('metricsRefresh.inlineUsagePartial', { missing: missingGroups.length })
  }
  return metricsRefreshStatusLabel(item.metrics.usage_detail.status)
}

function candidateRateLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.latest_snapshot?.final_rate_multiplier !== undefined && candidate.latest_snapshot?.final_rate_multiplier !== null) {
    return formatRate(candidate.latest_snapshot.final_rate_multiplier)
  }
  return formatNullableRate(candidate.latest_usage_delta?.derived_rate_multiplier)
}

function candidateHealthSeverity(candidate: UpstreamRelayCandidate) {
  if (!candidate.latest_probe) return 'unknown'
  if (candidate.latest_probe.success) return 'success'
  if (candidate.health?.last_error_class === 'auth_failed' || candidate.health?.last_error_class === 'browser_challenge') return 'auth'
  if (candidate.latest_usage_delta?.status === 'insufficient') return 'insufficient'
  return 'failed'
}

function candidateHealthLabel(candidate: UpstreamRelayCandidate) {
  const severity = candidateHealthSeverity(candidate)
  if (severity === 'success') return tM('health.success')
  if (severity === 'auth') return errorClassLabel(candidate.health?.last_error_class || 'auth_failed')
  if (severity === 'insufficient') return tM('health.insufficient')
  if (severity === 'failed') return tM('health.failed')
  return tM('health.unknown')
}

function candidateLatestProbeErrorSummary(candidate: UpstreamRelayCandidate) {
  const probe = candidate.latest_probe
  if (!probe || probe.success) return ''
  const reason = friendlyUpstreamRelayError(probe.error_message || probe.error_class || '')
  const errorClass = probe.error_class ? errorClassLabel(probe.error_class) : ''
  if (errorClass && reason && reason !== probe.error_class) return `${errorClass}: ${reason}`
  return errorClass || reason
}

function candidateHealthClass(candidate: UpstreamRelayCandidate) {
  const severity = candidateHealthSeverity(candidate)
  if (severity === 'success') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (severity === 'auth' || severity === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
  if (severity === 'insufficient') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function candidatePendingSuggestion(candidate: UpstreamRelayCandidate) {
  return pendingSuggestionMap.value.get(candidate.id)
}

function isConnectorExpanded(connectorID: number) {
  return expandedConnectorIds.value.has(connectorID)
}

function toggleConnectorExpanded(connectorID: number) {
  const next = new Set(expandedConnectorIds.value)
  if (next.has(connectorID)) next.delete(connectorID)
  else next.add(connectorID)
  expandedConnectorIds.value = next
}

function connectorGroupItems(connector: UpstreamRelayConnector) {
  return candidates.value
    .filter((candidate) => candidate.connector_id === connector.id)
    .slice()
    .sort((a, b) => {
      const priorityA = a.current_priority ?? Number.MAX_SAFE_INTEGER
      const priorityB = b.current_priority ?? Number.MAX_SAFE_INTEGER
      if (priorityA !== priorityB) return priorityA - priorityB
      return candidateUpstreamGroupLabel(a).localeCompare(candidateUpstreamGroupLabel(b))
    })
}

function connectorSnapshotItems(connector: UpstreamRelayConnector) {
  return overviewSnapshots.value
    .filter((snapshot) => snapshot.connector_id === connector.id)
    .slice()
    .sort((a, b) => {
      const rateA = Number.isFinite(a.final_rate_multiplier) ? a.final_rate_multiplier : Number.MAX_SAFE_INTEGER
      const rateB = Number.isFinite(b.final_rate_multiplier) ? b.final_rate_multiplier : Number.MAX_SAFE_INTEGER
      if (rateA !== rateB) return rateA - rateB
      return snapshotGroupLabel(a).localeCompare(snapshotGroupLabel(b))
    })
}

function connectorGroupRows(connector: UpstreamRelayConnector): ConnectorGroupRow[] {
  const rows: ConnectorGroupRow[] = []
  const mappedGroups = new Set<string>()
  for (const candidate of connectorGroupItems(connector)) {
    mappedGroups.add(candidate.upstream_group_id)
    rows.push({ kind: 'candidate', key: `candidate-${candidate.id}`, candidate })
  }
  for (const snapshot of connectorSnapshotItems(connector)) {
    if (mappedGroups.has(snapshot.upstream_group_id)) continue
    rows.push({ kind: 'snapshot', key: `snapshot-${snapshot.id}`, snapshot })
  }
  return rows
}

function candidateUpstreamGroupLabel(candidate: UpstreamRelayCandidate) {
  return candidate.upstream_group_name || candidate.upstream_group_id
}

function snapshotGroupLabel(snapshot: UpstreamRelayGroupRateSnapshot) {
  return snapshot.name || snapshot.upstream_group_id
}

function candidateSnapshotByGroupId(connectorId: number, groupId: string) {
  return [...overviewSnapshots.value, ...snapshots.value]
    .find((snapshot) => snapshot.connector_id === connectorId && snapshot.upstream_group_id === groupId) || null
}

function candidateGroupOptionLabel(snapshot: UpstreamRelayGroupRateSnapshot) {
  return `${snapshotGroupLabel(snapshot)} · ${snapshot.upstream_group_id} · ${formatRate(snapshot.final_rate_multiplier)}`
}

function connectorGroupRowName(row: ConnectorGroupRow) {
  return row.kind === 'candidate' ? candidateUpstreamGroupLabel(row.candidate) : snapshotGroupLabel(row.snapshot)
}

function connectorGroupRowGroupID(row: ConnectorGroupRow) {
  return row.kind === 'candidate' ? row.candidate.upstream_group_id : row.snapshot.upstream_group_id
}

function connectorGroupRowStatusLabel(row: ConnectorGroupRow) {
  if (row.kind === 'candidate') return candidateHealthLabel(row.candidate)
  return row.snapshot.status || tM('connectors.snapshotOnly')
}

function connectorGroupRowStatusClass(row: ConnectorGroupRow) {
  if (row.kind === 'candidate') return candidateHealthClass(row.candidate)
  if (row.snapshot.status === 'active') return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function connectorGroupRowRateLabel(row: ConnectorGroupRow) {
  return row.kind === 'candidate'
    ? candidateRateLabel(row.candidate)
    : formatRate(row.snapshot.final_rate_multiplier)
}

function connectorGroupRowPriorityLabel(row: ConnectorGroupRow) {
  return row.kind === 'candidate'
    ? `${tM('candidates.colPriority')} ${row.candidate.current_priority ?? '-'}`
    : tM('connectors.notBoundCandidate')
}

function connectorGroupRowUsageCostLabel(row: ConnectorGroupRow) {
  if (row.kind === 'candidate') return candidateTodayUsageCostLabel(row.candidate)
  if (row.snapshot.today_actual_cost === null || row.snapshot.today_actual_cost === undefined) {
    return tM('candidates.todayUsageNotRefreshed')
  }
  return formatUsageCost(row.snapshot.today_actual_cost)
}

function connectorGroupRowUsageMetaLabel(row: ConnectorGroupRow) {
  if (row.kind === 'candidate') return candidateTodayUsageCompactMetaLabel(row.candidate)
  if (row.snapshot.today_total_tokens === null || row.snapshot.today_total_tokens === undefined) {
    return tM('candidates.todayUsageSourceMissing')
  }
  return formatUsageTokenMillions(row.snapshot.today_total_tokens)
}

function candidateAccountLabel(candidate: Pick<UpstreamRelayCandidate, 'account_id' | 'account_name'>) {
  return `#${candidate.account_id} ${candidate.account_name || '-'}`
}

function candidateMappingLabel(candidate: UpstreamRelayCandidate) {
  return `${candidateUpstreamGroupLabel(candidate)} → ${candidateAccountLabel(candidate)}`
}

function formatProbeLatency(value?: number | null) {
  return value === null || value === undefined ? '-' : `${value}ms`
}

function suggestionMappingLabel(suggestion: Pick<UpstreamRelayRecommendationSuggestion, 'account_id' | 'account_name' | 'upstream_group_id' | 'upstream_group_name'>) {
  return `${suggestion.upstream_group_name || suggestion.upstream_group_id} → ${candidateAccountLabel(suggestion)}`
}

function suggestionActionType(suggestion: UpstreamRelayRecommendationSuggestion): SuggestionActionType {
  return suggestion.action_type || 'priority_update'
}

function suggestionActionLabel(suggestion: UpstreamRelayRecommendationSuggestion) {
  const map: Record<SuggestionActionType, string> = {
    priority_update: tM('suggestionActions.priorityUpdate'),
    account_pause: tM('suggestionActions.accountPause'),
    account_resume: tM('suggestionActions.accountResume')
  }
  return map[suggestionActionType(suggestion)] || suggestion.action_type || '-'
}

function suggestionActionClass(suggestion: UpstreamRelayRecommendationSuggestion) {
  const map: Record<SuggestionActionType, string> = {
    priority_update: 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-200',
    account_pause: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
    account_resume: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  }
  return map[suggestionActionType(suggestion)] || 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

function schedulableStatusLabel(value?: boolean | null) {
  if (value === true) return tM('schedulableStatus.enabled')
  if (value === false) return tM('schedulableStatus.paused')
  return '-'
}

function suggestionSchedulableTransitionLabel(suggestion: UpstreamRelayRecommendationSuggestion) {
  return `${schedulableStatusLabel(suggestion.old_schedulable)} → ${schedulableStatusLabel(suggestion.new_schedulable)}`
}

function suggestionChangeLabel(suggestion: UpstreamRelayRecommendationSuggestion) {
  if (suggestionActionType(suggestion) !== 'priority_update') return suggestionSchedulableTransitionLabel(suggestion)
  return `${suggestion.old_priority ?? '-'} → ${suggestion.new_priority ?? '-'}`
}

function exclusionReasonLabel(reasonCode: string) {
  const key = reasonCode.replace(/_([a-z])/g, (_, char: string) => char.toUpperCase())
  return tM(`policy.reasonCodes.${key}`)
}

function accountOptionLabel(account: Account) {
  return `#${account.id} ${account.name} · ${account.platform} · ${tM('candidates.colPriority')} ${account.priority}`
}

function apiKeyOptionLabel(apiKey: UpstreamRelayAPIKeyOption) {
  const name = apiKey.name || `Key #${apiKey.id}`
  return apiKey.masked_key ? `${name} · ${apiKey.masked_key}` : name
}

function priorityDeltaIcon(suggestion: UpstreamRelayRecommendationSuggestion): string {
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return 'arrowRight'
  if (suggestion.new_priority === null || suggestion.new_priority === undefined) return 'arrowRight'
  const delta = suggestion.new_priority - suggestion.old_priority
  if (delta > 0) return 'arrowUp'
  if (delta < 0) return 'arrowDown'
  return 'arrowRight'
}

function priorityDeltaClass(suggestion: UpstreamRelayRecommendationSuggestion): string {
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return 'text-primary-600 dark:text-primary-400'
  if (suggestion.new_priority === null || suggestion.new_priority === undefined) return 'text-gray-400 dark:text-gray-500'
  const delta = suggestion.new_priority - suggestion.old_priority
  if (delta > 0) return 'text-primary-600 dark:text-primary-400'
  if (delta < 0) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-400 dark:text-gray-500'
}

function connectorCredentialSummary(connector: UpstreamRelayConnector) {
  const saved: string[] = []
  if (connector.has_bearer_token) saved.push('Token')
  if (connector.has_refresh_token) saved.push('Refresh')
  if (connector.has_cookie) saved.push('Cookie')
  if (connector.has_user_agent) saved.push('UA')
  return saved.length > 0 ? tM('connectors.credentialsSaved', { items: saved.join(' / ') }) : tM('connectors.credentialsMissing')
}

function confidenceLabel(confidence: string) {
  const map: Record<string, string> = { high: tM('confidence.high'), medium: tM('confidence.medium'), low: tM('confidence.low'), unknown: tM('confidence.unknown') }
  return map[confidence] || confidence || '-'
}

function confidenceClass(confidence: string) {
  if (confidence === 'high') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (confidence === 'medium') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  if (confidence === 'low') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function errorClassLabel(errorClass: string) {
  const map: Record<string, string> = {
    auth_failed: tM('errorClass.authFailed'), rate_limited: tM('errorClass.rateLimited'),
    upstream_5xx: tM('errorClass.upstream5xx'), timeout: tM('errorClass.timeout'),
    model_unavailable: tM('errorClass.modelUnavailable'), insufficient_quota: tM('errorClass.insufficientQuota'),
    context_window_exceeded: tM('errorClass.contextWindowExceeded'), browser_challenge: tM('errorClass.browserChallenge'),
    network_error: tM('errorClass.networkError'), invalid_request: tM('errorClass.invalidRequest'),
    request_failed: tM('errorClass.requestFailed')
  }
  return map[errorClass] || errorClass
}
</script>
