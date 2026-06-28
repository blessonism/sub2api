<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap justify-end gap-2">
        <button class="btn btn-secondary inline-flex items-center gap-2" type="button" @click="loadAll">
          <Icon name="refresh" size="sm" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <section class="card p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div class="space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span :class="pillClass(configForm.enabled ? 'success' : 'muted')" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                {{ captureStatusLabel }}
              </span>
              <span v-if="savingConfig" class="text-xs text-primary-600 dark:text-primary-300">{{ t('common.saving') }}</span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ captureStatusDescription }}</p>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.conversations.enabled') }}</span>
            <Toggle :model-value="configForm.enabled" :disabled="savingConfig" @update:modelValue="(value) => updateConfigFlag('enabled', value)" />
          </div>
        </div>
        <div class="mt-4 grid grid-cols-2 gap-3 md:grid-cols-5">
          <div v-for="item in captureSummaryItems" :key="item.label" class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ item.value }}</div>
          </div>
        </div>
      </section>

      <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(360px,0.8fr)]">
        <section class="card p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.captureRules') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.conversations.captureRulesDescription') }}</p>
            </div>
            <button class="btn btn-primary shrink-0" type="button" :disabled="savingConfig" @click="saveConfig">
              {{ savingConfig ? t('common.saving') : t('common.save') }}
            </button>
          </div>
          <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.samplePercent') }}</span>
              <input v-model.number="configForm.sample_percent" class="input w-full" min="0" max="100" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.maxPayload') }}</span>
              <input v-model.number="configForm.max_turn_payload_bytes" class="input w-full" min="1" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.previewChars') }}</span>
              <input v-model.number="configForm.payload_preview_chars" class="input w-full" min="1" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.sessionWindow') }}</span>
              <input v-model.number="configForm.session_window_minutes" class="input w-full" min="1" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.retentionDays') }}</span>
              <input v-model.number="configForm.retention_days" class="input w-full" min="1" type="number" />
            </label>
            <div class="space-y-2 md:col-span-2 xl:col-span-3">
              <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.conversations.subjectFilterMode') }}</div>
              <div class="grid grid-cols-1 gap-2 md:grid-cols-2">
                <button
                  type="button"
                  data-test="conversation-mode-blacklist"
                  :class="modeButtonClass('blacklist')"
                  @click="setSubjectFilterMode('blacklist')"
                >
                  <span class="text-sm font-semibold">{{ t('admin.conversations.blacklistMode') }}</span>
                  <span class="mt-1 block text-xs font-normal">{{ t('admin.conversations.blacklistModeHint') }}</span>
                </button>
                <button
                  type="button"
                  data-test="conversation-mode-whitelist"
                  :class="modeButtonClass('whitelist')"
                  @click="setSubjectFilterMode('whitelist')"
                >
                  <span class="text-sm font-semibold">{{ t('admin.conversations.whitelistMode') }}</span>
                  <span class="mt-1 block text-xs font-normal">{{ t('admin.conversations.whitelistModeHint') }}</span>
                </button>
              </div>
              <p v-if="emptyWhitelistActive" data-test="conversation-empty-whitelist-warning" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300">
                {{ t('admin.conversations.emptyWhitelistWarning') }}
              </p>
            </div>
            <div
              v-for="item in activeSubjectListItems"
              :key="item.kind"
              class="space-y-2 md:col-span-2 xl:col-span-3"
            >
              <div class="flex items-center justify-between gap-3">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</span>
                <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('admin.conversations.subjectListCount', { count: listValues(item.kind).length }) }}</span>
              </div>
              <div class="flex min-h-11 flex-wrap items-center gap-2 rounded-md border border-gray-200 bg-white px-2 py-2 dark:border-dark-700 dark:bg-dark-900">
                <span
                  v-for="id in listValues(item.kind)"
                  :key="`${item.kind}-${id}`"
                  class="inline-flex items-center gap-1 rounded-md bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-200"
                  :data-test="`conversation-id-chip-${item.kind}-${id}`"
                >
                  #{{ id }}
                  <button
                    type="button"
                    class="rounded px-1 text-primary-500 hover:bg-primary-100 hover:text-primary-700 dark:text-primary-200 dark:hover:bg-primary-900/60"
                    :aria-label="t('admin.conversations.removeSubjectId', { id })"
                    @click="removeListID(item.kind, id)"
                  >
                    x
                  </button>
                </span>
                <input
                  :value="idListDrafts[item.kind]"
                  class="min-w-[160px] flex-1 border-0 bg-transparent p-1 text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-white"
                  :data-test="`conversation-id-input-${item.kind}`"
                  :placeholder="item.placeholder"
                  @input="updateIDListDraft(item.kind, $event)"
                  @keydown="onIDListKeydown(item.kind, $event)"
                  @paste.prevent="onIDListPaste(item.kind, $event)"
                  @blur="commitIDListDraft(item.kind)"
                />
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.conversations.subjectListHint') }}</p>
            </div>
          </div>
        </section>

        <section class="card p-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.exportSettings') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.conversations.exportSettingsDescription') }}</p>
          </div>
          <div class="mt-4 space-y-4">
            <div class="flex items-center justify-between gap-4">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.conversations.exportEnabled') }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.conversations.exportEnabledHint') }}</div>
              </div>
              <Toggle :model-value="configForm.export_enabled" :disabled="savingConfig" @update:modelValue="(value) => updateConfigFlag('export_enabled', value)" />
            </div>
            <label class="flex items-start gap-3 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="exportOptions.redaction_enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>{{ t('admin.conversations.redactionEnabled') }}</span>
            </label>
            <label class="flex items-start gap-3 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="exportOptions.include_duplicates" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>{{ t('admin.conversations.includeDuplicates') }}</span>
            </label>
            <label class="flex items-start gap-3 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="exportOptions.include_heuristic" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>{{ t('admin.conversations.includeHeuristic') }}</span>
            </label>
          </div>
        </section>
      </div>

      <div class="grid grid-cols-1 items-start gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <section class="card overflow-hidden">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.dataBrowser') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ hasActiveFilters ? t('admin.conversations.filteredResultSummary', { count: pagination.total }) : t('admin.conversations.resultSummary', { count: pagination.total }) }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="exporting || !configForm.export_enabled" @click="exportJSONL">
                  <Icon name="download" size="sm" />
                  {{ exporting ? t('admin.conversations.exporting') : t('admin.conversations.exportCurrentFilters') }}
                </button>
              </div>
            </div>
            <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-7">
              <input v-model.number="filters.user_id" class="input" type="number" min="1" :placeholder="t('admin.conversations.userId')" />
              <input v-model.number="filters.api_key_id" class="input" type="number" min="1" :placeholder="t('admin.conversations.apiKeyId')" />
              <input v-model.trim="filters.model" class="input" :placeholder="t('admin.conversations.model')" />
              <input v-model.trim="filters.request_id" class="input" :placeholder="t('admin.conversations.requestId')" />
              <select v-model="filters.quality_status" class="input">
                <option :value="''">{{ t('admin.conversations.allQuality') }}</option>
                <option value="clean">{{ t('admin.conversations.qualityClean') }}</option>
                <option value="needs_review">{{ t('admin.conversations.qualityNeedsReview') }}</option>
                <option value="rejected">{{ t('admin.conversations.qualityRejected') }}</option>
                <option value="unchecked">{{ t('admin.conversations.qualityUnchecked') }}</option>
              </select>
              <select v-model="filters.exportable" class="input">
                <option :value="''">{{ t('admin.conversations.allExportable') }}</option>
                <option value="true">{{ t('admin.conversations.exportableOnly') }}</option>
                <option value="false">{{ t('admin.conversations.notExportable') }}</option>
              </select>
              <button class="btn btn-secondary" type="button" @click="applyFilters">{{ t('common.search') }}</button>
            </div>
          </div>
          <div v-if="loading" class="flex min-h-64 items-center justify-center">
            <LoadingSpinner />
          </div>
          <EmptyState
            v-else-if="sessions.length === 0"
            class="min-h-64"
            :title="emptyStateTitle"
            :description="emptyStateDescription"
          />
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[1080px] text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ t('admin.conversations.session') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.conversations.subject') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.conversations.model') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.conversations.tokens') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.conversations.cost') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.conversations.status') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.conversations.updated') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="session in sessions" :key="session.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ shortId(session.session_id) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ session.session_source }} · {{ session.turn_count }} turns
                      <span v-if="session.duplicate_turn_count > 0"> · {{ session.duplicate_turn_count }} {{ t('admin.conversations.duplicates') }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                    <div>user #{{ session.user_id }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">key #{{ session.api_key_id }}</div>
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-300">{{ session.model }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(session.total_tokens) }}</td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatCost(session.actual_cost) }}</td>
                  <td class="px-4 py-3">
                    <span :class="pillClass(session.exportable ? 'success' : 'muted')" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                      {{ session.exportable ? t('admin.conversations.exportable') : t('admin.conversations.notExportable') }}
                    </span>
                    <span :class="pillClass(session.capture_status === 'captured' ? 'success' : 'warn')" class="ml-2 inline-flex rounded-md px-2 py-1 text-xs font-medium">
                      {{ session.capture_status }}
                    </span>
                    <span :class="pillClass(qualityKind(session.quality_status))" class="ml-2 inline-flex rounded-md px-2 py-1 text-xs font-medium">
                      {{ session.quality_status }}
                    </span>
                    <div v-if="formatQualityErrors(session.quality_errors)" class="mt-1 max-w-xs truncate text-xs text-amber-700 dark:text-amber-300">
                      {{ formatQualityErrors(session.quality_errors) }}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDate(session.ended_at) }}</td>
                  <td class="px-4 py-3">
                    <div class="flex justify-end gap-2">
                      <button class="btn btn-secondary px-2 py-1" type="button" @click="selectSession(session)">{{ t('common.view') }}</button>
                      <button class="btn btn-secondary px-2 py-1" type="button" @click="toggleSessionExportable(session)">
                        {{ session.exportable ? t('admin.conversations.unmark') : t('admin.conversations.markExportable') }}
                      </button>
                      <button class="btn btn-secondary px-2 py-1" type="button" @click="markSessionQuality(session, 'clean')">
                        {{ t('admin.conversations.markClean') }}
                      </button>
                      <button class="btn btn-secondary px-2 py-1" type="button" @click="markSessionQuality(session, 'rejected')">
                        {{ t('admin.conversations.reject') }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="onPageChange"
            @update:pageSize="onPageSizeChange"
          />
        </section>

        <aside class="card min-h-[360px] self-start p-4">
          <div v-if="!selectedSession" class="flex h-full min-h-[320px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.conversations.selectHint') }}
          </div>
          <div v-else class="space-y-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ shortId(selectedSession.session_id) }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ selectedSession.request_path }} · {{ selectedSession.session_source }}
              </p>
            </div>
            <div class="grid grid-cols-1 gap-2 text-xs">
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="markSessionQuality(selectedSession, 'clean')">
                  {{ t('admin.conversations.markClean') }}
                </button>
                <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="markSessionQuality(selectedSession, 'needs_review')">
                  {{ t('admin.conversations.needsReview') }}
                </button>
                <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="markSessionQuality(selectedSession, 'rejected')">
                  {{ t('admin.conversations.reject') }}
                </button>
              </div>
              <label class="space-y-1">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.conversations.mergeSourceIds') }}</span>
                <div class="flex gap-2">
                  <input v-model="mergeSourceIDsText" class="input w-full" placeholder="12,13" />
                  <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="mergeIntoSelectedSession">
                    {{ t('admin.conversations.merge') }}
                  </button>
                </div>
              </label>
            </div>
            <div v-if="turnsLoading" class="flex min-h-40 items-center justify-center">
              <LoadingSpinner />
            </div>
            <div v-else class="space-y-3">
              <article v-for="turn in turns" :key="turn.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-gray-900 dark:text-white">
                      #{{ turn.turn_index }} · {{ turn.parse_status }}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ turn.request_id }} · {{ turn.stream ? 'stream' : 'non-stream' }}
                      <span v-if="turn.duplicate_count > 0"> · {{ t('admin.conversations.duplicate') }}</span>
                    </div>
                  </div>
                  <div class="flex flex-wrap justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="toggleTurnExportable(turn)">
                      {{ turn.exportable ? t('admin.conversations.unmark') : t('admin.conversations.markExportable') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="markTurnQuality(turn, 'clean')">
                      {{ t('admin.conversations.markClean') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="markTurnQuality(turn, 'rejected')">
                      {{ t('admin.conversations.reject') }}
                    </button>
                  </div>
                </div>
                <div class="mt-3 space-y-2 text-xs text-gray-700 dark:text-gray-300">
                  <div v-if="selectedTurnDetails[turn.id]" class="rounded bg-gray-50 p-2 dark:bg-dark-800">
                    <div class="mb-1 font-medium">{{ t('admin.conversations.userMessages') }}</div>
                    <pre class="whitespace-pre-wrap break-words">{{ renderMessages(selectedTurnDetails[turn.id].request_messages) }}</pre>
                  </div>
                  <div v-if="selectedTurnDetails[turn.id]" class="rounded bg-gray-50 p-2 dark:bg-dark-800">
                    <div class="mb-1 font-medium">{{ t('admin.conversations.assistantMessages') }}</div>
                    <pre class="whitespace-pre-wrap break-words">{{ renderMessages(selectedTurnDetails[turn.id].response_messages) }}</pre>
                  </div>
                  <div v-else class="rounded bg-gray-50 p-2 dark:bg-dark-800">
                    <div class="mb-1 font-medium">{{ t('admin.conversations.preview') }}</div>
                    <pre class="max-h-40 whitespace-pre-wrap break-words overflow-auto">{{ turn.payload_preview || '-' }}</pre>
                  </div>
                </div>
                <div class="mt-3 flex flex-wrap gap-2 text-xs">
                  <span :class="pillClass(turn.truncated ? 'warn' : 'success')" class="rounded-md px-2 py-1">{{ turn.truncated ? 'truncated' : 'complete' }}</span>
                  <span :class="pillClass(turn.client_disconnect ? 'warn' : 'success')" class="rounded-md px-2 py-1">{{ turn.client_disconnect ? 'disconnect' : 'connected' }}</span>
                  <span :class="pillClass(qualityKind(turn.quality_status))" class="rounded-md px-2 py-1">{{ turn.quality_status }}</span>
                  <span v-if="formatQualityErrors(turn.quality_errors)" class="rounded-md bg-amber-50 px-2 py-1 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                    {{ formatQualityErrors(turn.quality_errors) }}
                  </span>
                  <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="loadTurnDetail(turn.id)">
                    {{ selectedTurnDetails[turn.id] ? t('admin.conversations.hideDetail') : t('admin.conversations.loadDetail') }}
                  </button>
                  <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="splitFromTurn(turn.id)">
                    {{ t('admin.conversations.splitHere') }}
                  </button>
                </div>
                <div class="mt-2 flex gap-2">
                  <input v-model.number="moveTargets[turn.id]" class="input min-w-0 flex-1 text-xs" type="number" min="1" :placeholder="t('admin.conversations.moveTarget')" />
                  <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="moveTurnToSession(turn.id)">
                    {{ t('admin.conversations.move') }}
                  </button>
                </div>
              </article>
            </div>
          </div>
        </aside>
      </div>

      <section class="card p-4">
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.exportJobs') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.conversations.exportJobsDescription') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="jobsLoading" @click="loadExportJobs">
              <Icon name="refresh" size="sm" />
              {{ t('common.refresh') }}
            </button>
            <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="creatingJob || !configForm.export_enabled" @click="createBackgroundExportJob">
              <Icon name="download" size="sm" />
              {{ creatingJob ? t('admin.conversations.creatingJob') : t('admin.conversations.createJob') }}
            </button>
          </div>
        </div>
        <div v-if="jobsLoading" class="mt-4 flex min-h-24 items-center justify-center">
          <LoadingSpinner />
        </div>
        <EmptyState
          v-else-if="exportJobs.length === 0"
          class="mt-4 min-h-24"
          :title="t('admin.conversations.noJobs')"
          :description="t('admin.conversations.noJobsDescription')"
        />
        <div v-else class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[920px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.conversations.job') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.conversations.status') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.conversations.turns') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.conversations.fileSize') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.conversations.expiresAt') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="job in exportJobs" :key="job.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">#{{ job.id }} · {{ job.format }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ job.encoding }} · limit {{ job.filters.limit || 1000 }}
                    <span v-if="job.filters.redaction_enabled"> · {{ t('admin.conversations.redacted') }}</span>
                    <span v-if="job.filters.include_duplicates"> · {{ t('admin.conversations.duplicatesIncluded') }}</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span :class="pillClass(jobStatusKind(job.status))" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ job.status }}
                  </span>
                  <div v-if="job.error_message" class="mt-1 max-w-sm truncate text-xs text-rose-600 dark:text-rose-300">
                    {{ job.error_message }}
                  </div>
                </td>
                <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(job.turn_count) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ formatBytes(job.file_size) }}</td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDate(job.expires_at) }}</td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1" type="button" :disabled="job.status !== 'completed'" @click="downloadExportJob(job.id)">
                      {{ t('admin.conversations.downloadJob') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1" type="button" :disabled="job.status === 'running'" @click="deleteExportJob(job.id)">
                      {{ t('common.delete') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import conversationsAPI, {
  type ConversationCaptureConfig,
  type ConversationExportJob,
  type ConversationExportJobFilters,
  type ConversationQualityStatus,
  type ConversationSession,
  type ConversationSessionFilters,
  type ConversationSubjectFilterMode,
  type ConversationTurn,
  type ConversationTurnSummary,
} from '@/api/admin/conversations'

const { t } = useI18n()

const defaultConfig: ConversationCaptureConfig = {
  enabled: false,
  sample_percent: 100,
  capture_chat_completions: true,
  capture_responses: false,
  raw_archive_enabled: false,
  max_turn_payload_bytes: 1048576,
  payload_preview_chars: 8000,
  session_window_minutes: 30,
  retention_days: 30,
  export_enabled: true,
  subject_filter_mode: 'blacklist',
  excluded_user_ids: [],
  excluded_api_key_ids: [],
  included_user_ids: [],
  included_api_key_ids: [],
}

const configForm = reactive<ConversationCaptureConfig>({ ...defaultConfig })
type ConversationIDListKind = 'excluded_user_ids' | 'excluded_api_key_ids' | 'included_user_ids' | 'included_api_key_ids'

const excludedUserIDs = ref<number[]>([])
const excludedAPIKeyIDs = ref<number[]>([])
const includedUserIDs = ref<number[]>([])
const includedAPIKeyIDs = ref<number[]>([])
const idListDrafts = reactive<Record<ConversationIDListKind, string>>({
  excluded_user_ids: '',
  excluded_api_key_ids: '',
  included_user_ids: '',
  included_api_key_ids: '',
})
const loading = ref(false)
const turnsLoading = ref(false)
const jobsLoading = ref(false)
const savingConfig = ref(false)
const exporting = ref(false)
const creatingJob = ref(false)
const sessions = ref<ConversationSession[]>([])
const turns = ref<ConversationTurnSummary[]>([])
const exportJobs = ref<ConversationExportJob[]>([])
const selectedTurnDetails = reactive<Record<number, ConversationTurn>>({})
const selectedSession = ref<ConversationSession | null>(null)
const moveTargets = reactive<Record<number, number | null>>({})
const mergeSourceIDsText = ref('')
const filters = reactive({
  user_id: null as number | null,
  api_key_id: null as number | null,
  model: '',
  request_id: '',
  quality_status: '',
  exportable: '',
})
const exportOptions = reactive({
  redaction_enabled: true,
  include_duplicates: false,
  include_heuristic: false,
})
const pagination = reactive({ total: 0, page: 1, page_size: 20 })

const captureStatusLabel = computed(() => (
  configForm.enabled ? t('admin.conversations.captureOn') : t('admin.conversations.captureOff')
))
const captureStatusDescription = computed(() => (
  configForm.enabled
    ? t('admin.conversations.captureOnDescription')
    : t('admin.conversations.captureOffDescription')
))
const hasActiveFilters = computed(() => Boolean(
  (filters.user_id && filters.user_id > 0)
  || (filters.api_key_id && filters.api_key_id > 0)
  || filters.model
  || filters.request_id
  || filters.quality_status
  || filters.exportable
))
const captureSummaryItems = computed(() => [
  { label: t('admin.conversations.samplePercent'), value: `${configForm.sample_percent}%` },
  { label: t('admin.conversations.retentionDays'), value: formatDays(configForm.retention_days) },
  { label: t('admin.conversations.exportEnabled'), value: configForm.export_enabled ? t('common.enabled') : t('common.disabled') },
  { label: t('admin.conversations.sessionsListed'), value: formatNumber(pagination.total) },
  { label: t('admin.conversations.sessionWindow'), value: formatMinutes(configForm.session_window_minutes) },
])
const emptyStateTitle = computed(() => (
  configForm.enabled ? t('admin.conversations.emptyEnabledTitle') : t('admin.conversations.emptyDisabledTitle')
))
const emptyStateDescription = computed(() => (
  configForm.enabled ? t('admin.conversations.emptyEnabledDescription') : t('admin.conversations.emptyDisabledDescription')
))
const emptyWhitelistActive = computed(() => (
  configForm.subject_filter_mode === 'whitelist' &&
  includedUserIDs.value.length === 0 &&
  includedAPIKeyIDs.value.length === 0
))
const activeSubjectListItems = computed(() => (
  configForm.subject_filter_mode === 'whitelist'
    ? [
        {
          kind: 'included_user_ids' as const,
          label: t('admin.conversations.includedUsers'),
          placeholder: t('admin.conversations.subjectListPlaceholderUsers'),
        },
        {
          kind: 'included_api_key_ids' as const,
          label: t('admin.conversations.includedApiKeys'),
          placeholder: t('admin.conversations.subjectListPlaceholderApiKeys'),
        },
      ]
    : [
        {
          kind: 'excluded_user_ids' as const,
          label: t('admin.conversations.excludedUsers'),
          placeholder: t('admin.conversations.subjectListPlaceholderUsers'),
        },
        {
          kind: 'excluded_api_key_ids' as const,
          label: t('admin.conversations.excludedApiKeys'),
          placeholder: t('admin.conversations.subjectListPlaceholderApiKeys'),
        },
      ]
))

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadConfig(), loadSessions(), loadExportJobs()])
}

async function loadConfig() {
  const config = await conversationsAPI.getConfig()
  const normalized = {
    ...defaultConfig,
    ...config,
    subject_filter_mode: config.subject_filter_mode || defaultConfig.subject_filter_mode,
    excluded_user_ids: config.excluded_user_ids || [],
    excluded_api_key_ids: config.excluded_api_key_ids || [],
    included_user_ids: config.included_user_ids || [],
    included_api_key_ids: config.included_api_key_ids || [],
  }
  Object.assign(configForm, normalized)
  syncIDLists(normalized)
}

async function saveConfig() {
  savingConfig.value = true
  try {
    const updated = await conversationsAPI.updateConfig({
      ...configForm,
      subject_filter_mode: configForm.subject_filter_mode || 'blacklist',
      excluded_user_ids: excludedUserIDs.value,
      excluded_api_key_ids: excludedAPIKeyIDs.value,
      included_user_ids: includedUserIDs.value,
      included_api_key_ids: includedAPIKeyIDs.value,
    })
    const normalized = { ...defaultConfig, ...updated }
    Object.assign(configForm, normalized)
    syncIDLists(normalized)
  } catch (error) {
    await loadConfig()
    throw error
  } finally {
    savingConfig.value = false
  }
}

async function updateConfigFlag(field: 'enabled' | 'export_enabled', value: boolean) {
  configForm[field] = value
  try {
    await saveConfig()
  } catch {
    // saveConfig 已重新拉取后端配置，避免开关点击产生未处理异常。
  }
}

function setSubjectFilterMode(mode: ConversationSubjectFilterMode) {
  configForm.subject_filter_mode = mode
}

function modeButtonClass(mode: ConversationSubjectFilterMode): string {
  const base = 'rounded-md border px-3 py-2 text-left transition'
  if (configForm.subject_filter_mode === mode) {
    return `${base} border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/30 dark:text-primary-200`
  }
  return `${base} border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:border-primary-500`
}

function syncIDLists(config: ConversationCaptureConfig) {
  excludedUserIDs.value = uniquePositiveIDs(config.excluded_user_ids || [])
  excludedAPIKeyIDs.value = uniquePositiveIDs(config.excluded_api_key_ids || [])
  includedUserIDs.value = uniquePositiveIDs(config.included_user_ids || [])
  includedAPIKeyIDs.value = uniquePositiveIDs(config.included_api_key_ids || [])
  Object.keys(idListDrafts).forEach((key) => {
    idListDrafts[key as ConversationIDListKind] = ''
  })
}

function listValues(kind: ConversationIDListKind): number[] {
  if (kind === 'excluded_user_ids') return excludedUserIDs.value
  if (kind === 'excluded_api_key_ids') return excludedAPIKeyIDs.value
  if (kind === 'included_user_ids') return includedUserIDs.value
  return includedAPIKeyIDs.value
}

function setListValues(kind: ConversationIDListKind, values: number[]) {
  const normalized = uniquePositiveIDs(values)
  if (kind === 'excluded_user_ids') {
    excludedUserIDs.value = normalized
  } else if (kind === 'excluded_api_key_ids') {
    excludedAPIKeyIDs.value = normalized
  } else if (kind === 'included_user_ids') {
    includedUserIDs.value = normalized
  } else {
    includedAPIKeyIDs.value = normalized
  }
}

function updateIDListDraft(kind: ConversationIDListKind, event: Event) {
  idListDrafts[kind] = (event.target as HTMLInputElement).value
}

function onIDListKeydown(kind: ConversationIDListKind, event: KeyboardEvent) {
  if (event.key !== 'Enter' && event.key !== ',' && event.key !== ' ' && event.key !== 'Tab') return
  if (event.key !== 'Tab' || idListDrafts[kind].trim() !== '') {
    event.preventDefault()
  }
  commitIDListDraft(kind)
}

function onIDListPaste(kind: ConversationIDListKind, event: ClipboardEvent) {
  addListIDs(kind, event.clipboardData?.getData('text') || '')
}

function commitIDListDraft(kind: ConversationIDListKind) {
  addListIDs(kind, idListDrafts[kind])
  idListDrafts[kind] = ''
}

function addListIDs(kind: ConversationIDListKind, raw: string) {
  const ids = parseIDs(raw)
  if (ids.length === 0) return
  setListValues(kind, [...listValues(kind), ...ids])
}

function removeListID(kind: ConversationIDListKind, id: number) {
  setListValues(kind, listValues(kind).filter((item) => item !== id))
}

async function loadSessions() {
  loading.value = true
  try {
    const response = await conversationsAPI.listSessions(buildFilters())
    sessions.value = response.items
    pagination.total = response.total
    pagination.page = response.page
    pagination.page_size = response.page_size
  } finally {
    loading.value = false
  }
}

async function loadExportJobs() {
  jobsLoading.value = true
  try {
    const response = await conversationsAPI.listExportJobs({ page: 1, page_size: 20 })
    exportJobs.value = response.items
  } finally {
    jobsLoading.value = false
  }
}

function buildFilters(): ConversationSessionFilters {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    user_id: filters.user_id && filters.user_id > 0 ? filters.user_id : undefined,
    api_key_id: filters.api_key_id && filters.api_key_id > 0 ? filters.api_key_id : undefined,
    model: filters.model || undefined,
    request_id: filters.request_id || undefined,
    quality_status: filters.quality_status || undefined,
    exportable: filters.exportable === '' ? undefined : filters.exportable === 'true',
  }
}

function applyFilters() {
  pagination.page = 1
  void loadSessions()
}

async function selectSession(session: ConversationSession) {
  selectedSession.value = session
  turnsLoading.value = true
  try {
    const response = await conversationsAPI.listSessionTurns(session.id, { page: 1, page_size: 50 })
    turns.value = response.items
    Object.keys(selectedTurnDetails).forEach((key) => delete selectedTurnDetails[Number(key)])
    Object.keys(moveTargets).forEach((key) => delete moveTargets[Number(key)])
    mergeSourceIDsText.value = ''
  } finally {
    turnsLoading.value = false
  }
}

async function refreshSelectedSession() {
  if (!selectedSession.value) return
  const current = await conversationsAPI.getSession(selectedSession.value.id)
  selectedSession.value = current
  const index = sessions.value.findIndex((item) => item.id === current.id)
  if (index >= 0) {
    sessions.value[index] = current
  }
  await selectSession(current)
}

async function toggleSessionExportable(session: ConversationSession) {
  const next = !session.exportable
  await conversationsAPI.setSessionExportable(session.id, next)
  session.exportable = next
  if (selectedSession.value?.id === session.id) {
    selectedSession.value.exportable = next
  }
}

async function markSessionQuality(session: ConversationSession, quality_status: ConversationQualityStatus) {
  await conversationsAPI.setSessionQuality(session.id, {
    quality_status,
    quality_errors: quality_status === 'clean' || quality_status === 'unchecked'
      ? []
      : [{ code: quality_status, message: quality_status, source: 'manual' }],
  })
  session.quality_status = quality_status
  if (quality_status === 'rejected') {
    session.exportable = false
  }
  if (selectedSession.value?.id === session.id) {
    selectedSession.value.quality_status = quality_status
    selectedSession.value.exportable = session.exportable
  }
}

async function markTurnQuality(turn: ConversationTurnSummary | ConversationTurn, quality_status: ConversationQualityStatus) {
  await conversationsAPI.setTurnQuality(turn.id, {
    quality_status,
    quality_errors: quality_status === 'clean' || quality_status === 'unchecked'
      ? []
      : [{ code: quality_status, message: quality_status, source: 'manual' }],
  })
  turn.quality_status = quality_status
  if (quality_status === 'rejected' || quality_status === 'needs_review') {
    turn.exportable = false
  }
  if (selectedTurnDetails[turn.id]) {
    selectedTurnDetails[turn.id].quality_status = quality_status
    selectedTurnDetails[turn.id].exportable = turn.exportable
  }
}

async function toggleTurnExportable(turn: ConversationTurnSummary | ConversationTurn) {
  const next = !turn.exportable
  await conversationsAPI.setTurnExportable(turn.id, next)
  turn.exportable = next
  if (selectedTurnDetails[turn.id]) {
    selectedTurnDetails[turn.id].exportable = next
  }
}

async function mergeIntoSelectedSession() {
  if (!selectedSession.value) return
  const sourceIDs = parseIDs(mergeSourceIDsText.value).filter((id) => id !== selectedSession.value?.id)
  if (sourceIDs.length === 0) return
  await conversationsAPI.mergeSessions({
    target_session_id: selectedSession.value.id,
    source_session_ids: sourceIDs,
  })
  await Promise.all([loadSessions(), refreshSelectedSession()])
}

async function splitFromTurn(turnId: number) {
  if (!selectedSession.value) return
  await conversationsAPI.splitSession(selectedSession.value.id, { turn_id: turnId })
  await Promise.all([loadSessions(), refreshSelectedSession()])
}

async function moveTurnToSession(turnId: number) {
  const target = moveTargets[turnId]
  if (!target || target <= 0) return
  await conversationsAPI.moveTurn(turnId, { target_session_id: target })
  await Promise.all([loadSessions(), refreshSelectedSession()])
}

async function loadTurnDetail(id: number) {
  if (selectedTurnDetails[id]) {
    delete selectedTurnDetails[id]
    return
  }
  selectedTurnDetails[id] = await conversationsAPI.getTurn(id)
}

async function exportJSONL() {
  exporting.value = true
  try {
    const currentFilters = buildFilters()
    const blob = await conversationsAPI.exportMessagesJSONL({
      user_id: currentFilters.user_id,
      api_key_id: currentFilters.api_key_id,
      model: currentFilters.model,
      request_id: currentFilters.request_id,
      quality_status: currentFilters.quality_status,
      started_at_from: currentFilters.started_at_from,
      started_at_to: currentFilters.started_at_to,
      include_heuristic: exportOptions.include_heuristic,
      include_duplicates: exportOptions.include_duplicates,
      redaction_enabled: exportOptions.redaction_enabled,
      dedupe: !exportOptions.include_duplicates,
      limit: 200,
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `conversation_messages_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '')}.jsonl`
    link.click()
    URL.revokeObjectURL(url)
  } finally {
    exporting.value = false
  }
}

function buildExportJobFilters(): ConversationExportJobFilters {
  const currentFilters = buildFilters()
  return {
    user_id: currentFilters.user_id,
    api_key_id: currentFilters.api_key_id,
    model: currentFilters.model,
    request_id: currentFilters.request_id,
    quality_status: currentFilters.quality_status,
    started_at_from: currentFilters.started_at_from,
    started_at_to: currentFilters.started_at_to,
    include_heuristic: exportOptions.include_heuristic,
    include_duplicates: exportOptions.include_duplicates,
    redaction_enabled: exportOptions.redaction_enabled,
    dedupe: !exportOptions.include_duplicates,
    limit: 5000,
  }
}

async function createBackgroundExportJob() {
  creatingJob.value = true
  try {
    await conversationsAPI.createExportJob({
      filters: buildExportJobFilters(),
      format: 'messages_jsonl',
      encoding: 'zstd',
    })
    await loadExportJobs()
  } finally {
    creatingJob.value = false
  }
}

async function downloadExportJob(id: number) {
  const ticket = await conversationsAPI.createExportDownloadTicket(id)
  window.open(ticket.download_url, '_blank', 'noopener,noreferrer')
  await loadExportJobs()
}

async function deleteExportJob(id: number) {
  await conversationsAPI.deleteExportJob(id)
  await loadExportJobs()
}

function onPageChange(page: number) {
  pagination.page = page
  loadSessions()
}

function onPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadSessions()
}

function parseIDs(value: string): number[] {
  return uniquePositiveIDs(value
    .split(/[\s,，;；]+/)
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0))
}

function uniquePositiveIDs(values: number[]): number[] {
  const seen = new Set<number>()
  const result: number[] = []
  values.forEach((value) => {
    if (!Number.isInteger(value) || value <= 0 || seen.has(value)) return
    seen.add(value)
    result.push(value)
  })
  return result
}

function shortId(value: string): string {
  return value.length > 18 ? `${value.slice(0, 10)}...${value.slice(-6)}` : value
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

function formatCost(value: number): string {
  return `$${Number(value || 0).toFixed(6)}`
}

function formatBytes(value: number): string {
  if (!value) return '-'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / 1024 / 1024).toFixed(1)} MiB`
}

function formatDays(value: number): string {
  return t('admin.conversations.daysValue', { count: value || 0 })
}

function formatMinutes(value: number): string {
  return t('admin.conversations.minutesValue', { count: value || 0 })
}

function formatDate(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function renderMessages(messages: Array<Record<string, unknown>>): string {
  if (!messages?.length) return '-'
  return messages
    .map((message) => {
      const role = String(message.role || 'message')
      const content = typeof message.content === 'string' ? message.content : JSON.stringify(message.content ?? message)
      return `${role}: ${content}`
    })
    .join('\n\n')
}

function formatQualityErrors(errors: Array<{ code?: string; message?: string }> | undefined): string {
  if (!errors?.length) return ''
  return errors
    .map((error) => error.message || error.code || '')
    .filter(Boolean)
    .join('; ')
}

function pillClass(kind: 'success' | 'warn' | 'muted'): string {
  if (kind === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (kind === 'warn') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function qualityKind(status: ConversationQualityStatus | string): 'success' | 'warn' | 'muted' {
  if (status === 'clean') return 'success'
  if (status === 'needs_review' || status === 'rejected') return 'warn'
  return 'muted'
}

function jobStatusKind(status: ConversationExportJob['status']): 'success' | 'warn' | 'muted' {
  if (status === 'completed') return 'success'
  if (status === 'failed' || status === 'expired' || status === 'deleted') return 'warn'
  return 'muted'
}
</script>
