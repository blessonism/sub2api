<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ tM('title') }}</h1>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
            <span>{{ tM('lastRefreshed') }}: {{ formatDate(lastLoadedAt) }}</span>
            <span>{{ tM('lastSynced') }}: {{ overviewStats.latestSyncedAt }}</span>
            <span>{{ tM('statsNote') }}</span>
          </div>
        </div>
        <div class="flex flex-wrap justify-start gap-2 lg:justify-end">
          <AutoRefreshButton
            :enabled="autoRefreshEnabled"
            :interval-seconds="autoRefreshInterval"
            :countdown="autoRefreshCountdown"
            :intervals="AUTO_REFRESH_INTERVALS"
            @update:enabled="autoRefreshEnabled = $event"
            @update:interval="autoRefreshInterval = $event"
          />
          <button class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="sm" />
            {{ tM('refresh') }}
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="generating" @click="generateRun">
            <Icon name="chart" size="sm" />
            {{ generating ? tM('generating') : tM('generateSuggestions') }}
          </button>
        </div>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
        {{ error }}
      </div>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-6">
        <button
          v-for="item in overviewCards"
          :key="item.key"
          type="button"
          class="card p-4 text-left transition hover:border-primary-200 hover:bg-primary-50/40 dark:hover:border-primary-900/50 dark:hover:bg-primary-950/20"
          @click="activeSection = item.section"
        >
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.hint }}</div>
        </button>
      </section>

      <div class="flex flex-col gap-3 border-b border-gray-200 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex flex-wrap gap-2">
          <button
            v-for="section in sections"
            :key="section.key"
            type="button"
            class="rounded-t-lg px-4 py-2 text-sm font-medium transition"
            :class="activeSection === section.key ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
            @click="activeSection = section.key"
          >
            {{ section.label }}
          </button>
        </div>
        <div class="flex flex-wrap gap-2 pb-3 lg:pb-2">
          <button v-if="activeSection === 'candidates'" class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateCandidate">
            <Icon name="plus" size="sm" />
            {{ tM('candidates.newCandidate') }}
          </button>
          <button v-if="activeSection === 'connectors'" class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateConnector">
            <Icon name="plus" size="sm" />
            {{ tM('connectors.newConnector') }}
          </button>
        </div>
      </div>

      <section v-if="activeSection === 'candidates'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('candidates.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('candidates.description') }}</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('candidates.count', { total: candidates.length, enabled: enabledCandidateCount }) }}</span>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1180px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colCandidate') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colMapping') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colRate') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('candidates.colHealth') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colPriority') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('candidates.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="candidate in candidates" :key="candidate.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">#{{ candidate.account_id }} {{ candidate.account_name || '-' }}</span>
                    <span :class="candidate.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'" class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium">
                      {{ candidate.enabled ? tM('candidates.enabled') : tM('candidates.disabled') }}
                    </span>
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ candidate.account_platform || '-' }} · {{ candidate.probe_model }} · {{ candidate.probe_protocol }}</div>
                </td>
                <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <div>{{ candidate.connector_name || `Connector #${candidate.connector_id}` }}</div>
                  <div class="mt-1 font-mono">{{ candidate.upstream_group_id }} → {{ candidate.target_group_name || `Group #${candidate.target_group_id}` }}</div>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ candidateRateLabel(candidate) }}</div>
                  <div class="mt-1 flex justify-end"><RateSourceTag :source="candidateRateSourceKey(candidate)" /></div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span :class="candidateHealthClass(candidate)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">{{ candidateHealthLabel(candidate) }}</span>
                    <span v-if="candidate.latest_probe" class="text-xs text-gray-500 dark:text-gray-400">{{ candidate.latest_probe.latency_ms ?? '-' }}ms</span>
                  </div>
                  <div class="mt-1.5"><HealthRateBar :rate="candidate.health?.success_rate ?? null" /></div>
                  <button class="mt-1 text-xs text-primary-600 hover:underline dark:text-primary-400" type="button" @click="healthDialogCandidate = candidate">{{ tM('candidates.healthDetail') }}</button>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="tabular-nums text-gray-900 dark:text-white">{{ candidate.current_priority ?? '-' }}</div>
                  <div v-if="candidatePendingSuggestion(candidate)" class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-300">
                    {{ tM('candidates.pendingSuggestion', { priority: candidatePendingSuggestion(candidate)?.new_priority }) }}
                    <span class="text-gray-400 dark:text-gray-500">({{ priorityDeltaLabel(candidatePendingSuggestion(candidate)!) }})</span>
                  </div>
                  <div v-else class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ tM('candidates.noPendingSuggestion') }}</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editCandidate(candidate)">{{ tM('candidates.edit') }}</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="reuseCandidateAccount(candidate)">{{ tM('candidates.reuseAccount') }}</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="probingId === candidate.id" @click="probe(candidate)">
                      {{ probingId === candidate.id ? tM('candidates.probing') : tM('candidates.probe') }}
                    </button>
                    <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeCandidate(candidate)">{{ tM('candidates.delete') }}</button>
                  </div>
                </td>
              </tr>
              <tr v-if="candidates.length === 0">
                <td colspan=”6” class=”px-4 py-10 text-center text-gray-500 dark:text-gray-400”>{{ tM('candidates.empty') }}</td>
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
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colName') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colStatus') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colCredentials') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('connectors.colVerifiedSynced') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('connectors.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="connector in connectors" :key="connector.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">{{ connector.name }}</div>
                  <div class="mt-1 max-w-[360px] truncate text-xs text-gray-500 dark:text-gray-400">{{ connector.base_url }}</div>
                </td>
                <td class="px-4 py-3">
                  <span :class="statusClass(connector.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ connectorStatusLabel(connector.status) }}
                  </span>
                  <div v-if="connector.last_error" class="mt-1 max-w-[280px] truncate text-xs text-red-500">{{ connector.last_error }}</div>
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
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editConnector(connector)">{{ tM('connectors.edit') }}</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="syncingId === connector.id" @click="sync(connector)">
                      {{ syncingId === connector.id ? tM('connectors.syncing') : tM('connectors.syncAction') }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="openSnapshotDialog(connector)">{{ tM('connectors.snapshots') }}</button>
                    <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeConnector(connector)">{{ tM('connectors.delete') }}</button>
                  </div>
                </td>
              </tr>
              <tr v-if="connectors.length === 0">
                <td colspan=”5” class=”px-4 py-10 text-center text-gray-500 dark:text-gray-400”>{{ tM('connectors.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="activeSection === 'recommendations'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ tM('recommendations.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ tM('recommendations.description') }}</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ tM('recommendations.pendingCount', { n: pendingSuggestionCount }) }}</span>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colRun') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colCandidates') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colSuggestions') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colStatus') }}</th>
                <th class="px-4 py-3 text-left">{{ tM('recommendations.colCreatedAt') }}</th>
                <th class="px-4 py-3 text-right">{{ tM('recommendations.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="run in recommendationRuns" :key="run.id">
                <td class="px-4 py-3 font-mono">#{{ run.id }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.total_candidates }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.suggestion_count }}</td>
                <td class="px-4 py-3">
                  <span :class="run.applied ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ run.applied ? tM('recommendations.applied') : tM('recommendations.pending') }}
                  </span>
                </td>
                <td class="px-4 py-3">{{ formatDate(run.created_at) }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="btn btn-primary px-3 py-1.5 text-xs" type="button" :disabled="run.applied || run.suggestion_count === 0" @click="openApplyDialog(run)">
                    {{ tM('recommendations.viewAndApply') }}
                  </button>
                </td>
              </tr>
              <tr v-if="recommendationRuns.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">{{ tM('recommendations.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>

  <BaseDialog :show="connectorDialogOpen" :title="connectorForm.id ? tM('connectorForm.titleEdit') : tM('connectorForm.titleCreate')" width="wide" @close="closeConnectorDialog">
    <form id="connector-form" class="space-y-4" @submit.prevent="submitConnector">
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelName') }}</span>
        <input v-model.trim="connectorForm.name" class="input w-full" type="text" />
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelBaseUrl') }}</span>
        <input v-model.trim="connectorForm.base_url" class="input w-full" type="url" placeholder="https://upstream.example.com" />
      </label>
      <div class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'manual_session' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @click="connectorForm.auth_mode = 'manual_session'"
        >
          {{ tM('connectorForm.authManual') }}
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'password_login' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @click="connectorForm.auth_mode = 'password_login'"
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
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelCookie') }}</span>
          <textarea v-model.trim="connectorForm.cookie" class="input min-h-[74px] w-full" :placeholder="tM('connectorForm.placeholderCookie')" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelUserAgent') }}</span>
          <input v-model.trim="connectorForm.user_agent" class="input w-full" type="text" :placeholder="tM('connectorForm.placeholderUserAgent')" />
        </label>
      </template>
      <template v-else>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelEmail') }}</span>
          <input v-model.trim="connectorForm.login_email" class="input w-full" type="email" autocomplete="username" :placeholder="connectorForm.id ? tM('connectorForm.placeholderEmailEdit') : tM('connectorForm.placeholderEmail')" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('connectorForm.labelPassword') }}</span>
          <input v-model.trim="connectorForm.login_password" class="input w-full" type="password" autocomplete="new-password" :placeholder="tM('connectorForm.placeholderPassword')" />
        </label>
      </template>
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
        <select v-model.number="candidateForm.connector_id" class="input w-full">
          <option :value="0">{{ tM('candidateForm.placeholderSelect') }}</option>
          <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
        </select>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelAccount') }}</span>
        <select v-model.number="candidateForm.account_id" class="input w-full">
          <option :value="0">{{ tM('candidateForm.placeholderSelect') }}</option>
          <option v-for="account in accounts" :key="account.id" :value="account.id">#{{ account.id }} {{ account.name }} · {{ account.platform }}</option>
        </select>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelUpstreamGroupId') }}</span>
        <input v-model.trim="candidateForm.upstream_group_id" class="input w-full" type="text" />
      </label>
      <div v-if="duplicateCandidate" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
        已存在相同连接器、账号和上游 Group ID 的候选 #{{ duplicateCandidate.id }}，目标分组为 {{ duplicateCandidate.target_group_name || `Group #${duplicateCandidate.target_group_id}` }}。当前仅提示，不阻断保存。
      </div>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelTargetGroup') }}</span>
        <select v-model.number="candidateForm.target_group_id" class="input w-full">
          <option :value="0">{{ tM('candidateForm.placeholderSelect') }}</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">#{{ group.id }} {{ group.name }} · {{ group.platform }}</option>
        </select>
      </label>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelProbeModel') }}</span>
          <input v-model.trim="candidateForm.probe_model" class="input w-full" type="text" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ tM('candidateForm.labelProtocol') }}</span>
          <select v-model="candidateForm.probe_protocol" class="input w-full">
            <option value="chat_completions">chat_completions</option>
            <option value="responses">responses</option>
          </select>
        </label>
      </div>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="candidateForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
        {{ tM('candidateForm.enableCandidate') }}
      </label>
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

  <BaseDialog :show="applyDialogOpen" :title="tM('applyDialog.title')" width="extra-wide" @close="applyDialogOpen = false">
    <div class="space-y-4">
      <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
        {{ tM('applyDialog.warning', { id: applyRun?.id || '-', date: formatDate(applyRun?.created_at) }) }}
      </div>
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
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colTargetGroup') }}</th>
              <th class="px-3 py-3 text-right">{{ tM('applyDialog.colPriorityDelta') }}</th>
              <th class="px-3 py-3 text-right">{{ tM('applyDialog.colRate') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colConfidence') }}</th>
              <th class="px-3 py-3 text-left">{{ tM('applyDialog.colSummary') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="suggestion in applyRun?.suggestions || []" :key="suggestion.id || suggestion.candidate_id">
              <td class="px-3 py-3">#{{ suggestion.account_id }} {{ suggestion.account_name || '-' }}</td>
              <td class="px-3 py-3">{{ suggestion.target_group_name || `#${suggestion.target_group_id}` }}</td>
              <td class="px-3 py-3 text-right">
                <div class="inline-flex items-center gap-1 tabular-nums">
                  <span class="text-gray-400 dark:text-gray-500">{{ suggestion.old_priority ?? '-' }}</span>
                  <Icon :name="(priorityDeltaIcon(suggestion) as 'arrowUp' | 'arrowDown' | 'arrowRight')" size="xs" :class="priorityDeltaClass(suggestion)" />
                  <span class="font-semibold" :class="priorityDeltaClass(suggestion)">{{ suggestion.new_priority }}</span>
                </div>
              </td>
              <td class="px-3 py-3 text-right">
                <div class="tabular-nums">{{ formatRate(suggestion.final_rate_multiplier) }}</div>
                <div class="mt-1 flex justify-end"><RateSourceTag :source="suggestion.rate_source" /></div>
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
        <button class="btn btn-secondary" type="button" @click="applyDialogOpen = false">{{ tM('applyDialog.cancel') }}</button>
        <button class="btn btn-primary" type="button" :disabled="applying" @click="applySelectedRun">
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
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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
import accountsAPI from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import upstreamRelayAPI, {
  type UpstreamRelayCandidate,
  type UpstreamRelayConnector,
  type UpstreamRelayGroupRateSnapshot,
  type UpstreamRelayProbeProtocol,
  type UpstreamRelayRecommendationSuggestion,
  type UpstreamRelayRecommendationRun
} from '@/api/admin/upstreamRelayGroupMonitors'
import type { Account, AdminGroup } from '@/types'

type SectionKey = 'candidates' | 'connectors' | 'recommendations'
type RateSourceKey = 'login_user_group_rates' | 'login_available_groups' | 'usage_cost_delta' | 'none' | string

const loading = ref(false)
const error = ref('')
const connectors = ref<UpstreamRelayConnector[]>([])
const snapshots = ref<UpstreamRelayGroupRateSnapshot[]>([])
const candidates = ref<UpstreamRelayCandidate[]>([])
const recommendationRuns = ref<UpstreamRelayRecommendationRun[]>([])
const accounts = ref<Account[]>([])
const groups = ref<AdminGroup[]>([])
const selectedConnectorId = ref(0)
const syncingId = ref<number | null>(null)
const probingId = ref<number | null>(null)
const generating = ref(false)
const applying = ref(false)
const snapshotLoading = ref(false)
const savingConnector = ref(false)
const savingCandidate = ref(false)
const applyDialogOpen = ref(false)
const applyRun = ref<UpstreamRelayRecommendationRun | null>(null)
const activeSection = ref<SectionKey>('candidates')
const connectorDialogOpen = ref(false)
const candidateDialogOpen = ref(false)
const snapshotDialogOpen = ref(false)
const snapshotConnector = ref<UpstreamRelayConnector | null>(null)
const lastLoadedAt = ref<string | null>(null)
const healthDialogCandidate = ref<UpstreamRelayCandidate | null>(null)
const pendingDeleteConnector = ref<UpstreamRelayConnector | null>(null)
const pendingDeleteCandidate = ref<UpstreamRelayCandidate | null>(null)
const candidateSourceSnapshot = ref<UpstreamRelayGroupRateSnapshot | null>(null)

const AUTO_REFRESH_INTERVALS = [15, 30, 60] as const
const autoRefreshEnabled = ref(false)
const autoRefreshInterval = ref(30)
const autoRefreshCountdown = ref(30)
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

const { t } = useI18n()
const tM = (k: string, params?: Record<string, unknown>) => t(`admin.upstreamRelayGroupMonitoring.${k}`, params ?? {})

const connectorForm = reactive({
  id: 0,
  name: '',
  base_url: '',
  auth_mode: 'manual_session' as 'manual_session' | 'password_login',
  bearer_token: '',
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
  probe_model: '',
  probe_protocol: 'chat_completions' as 'chat_completions' | 'responses',
  target_group_id: 0,
  enabled: true,
  notes: ''
})

const activeConnectors = computed(() => connectors.value.filter((item) => item.status === 'active'))
const enabledCandidateCount = computed(() => candidates.value.filter((item) => item.enabled).length)
const failedCandidateCount = computed(() => candidates.value.filter((item) => candidateHealthSeverity(item) === 'failed').length)
const pendingRuns = computed(() => recommendationRuns.value.filter((item) => !item.applied && item.suggestion_count > 0))
const pendingSuggestionCount = computed(() => pendingRuns.value.reduce((total, item) => total + item.suggestion_count, 0))
const latestPendingRun = computed(() => pendingRuns.value[0] || null)
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

const sections = computed((): Array<{ key: SectionKey; label: string }> => [
  { key: 'candidates', label: tM('tabs.candidates') },
  { key: 'connectors', label: tM('tabs.connectors') },
  { key: 'recommendations', label: tM('tabs.recommendations') }
])

const overviewStats = computed(() => {
  const needsReauth = connectors.value.filter((item) => item.status === 'needs_reauth').length
  const latestSyncedAt = connectors.value
    .map((item) => item.last_synced_at)
    .filter((value): value is string => Boolean(value))
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0]

  return {
    activeConnectors: activeConnectors.value.length,
    needsReauth,
    enabledCandidates: enabledCandidateCount.value,
    failedCandidates: failedCandidateCount.value,
    pendingSuggestions: pendingSuggestionCount.value,
    latestSyncedAt: formatDate(latestSyncedAt)
  }
})

const overviewCards = computed(() => [
  {
    key: 'active-connectors',
    label: tM('overview.activeConnectors'),
    value: overviewStats.value.activeConnectors,
    hint: tM('overview.totalLoaded', { n: connectors.value.length }),
    section: 'connectors' as SectionKey
  },
  {
    key: 'reauth-connectors',
    label: tM('overview.needsReauth'),
    value: overviewStats.value.needsReauth,
    hint: tM('overview.needsReauthHint'),
    section: 'connectors' as SectionKey
  },
  {
    key: 'enabled-candidates',
    label: tM('overview.enabledCandidates'),
    value: overviewStats.value.enabledCandidates,
    hint: tM('overview.totalLoaded', { n: candidates.value.length }),
    section: 'candidates' as SectionKey
  },
  {
    key: 'failed-candidates',
    label: tM('overview.failedCandidates'),
    value: overviewStats.value.failedCandidates,
    hint: tM('overview.failedCandidatesHint'),
    section: 'candidates' as SectionKey
  },
  {
    key: 'pending-suggestions',
    label: tM('overview.pendingSuggestions'),
    value: overviewStats.value.pendingSuggestions,
    hint: tM('overview.pendingSuggestionsHint'),
    section: 'recommendations' as SectionKey
  },
  {
    key: 'latest-sync',
    label: tM('overview.latestSync'),
    value: overviewStats.value.latestSyncedAt,
    hint: tM('overview.latestSyncHint'),
    section: 'connectors' as SectionKey
  }
])

const snapshotDialogTitle = computed(() =>
  snapshotConnector.value
    ? tM('snapshotDialog.title', { name: snapshotConnector.value.name })
    : tM('snapshotDialog.titleFallback')
)

const applyRiskCards = computed(() => {
  const suggestions = applyRun.value?.suggestions || []
  const lowConfidenceCount = suggestions.filter((item) => item.confidence === 'low' || item.confidence === 'unknown').length
  const riskyCandidateIds = new Set(
    candidates.value
      .filter((item) => candidateHealthSeverity(item) !== 'success')
      .map((item) => item.id)
  )
  const riskySuggestionCount = suggestions.filter((item) => riskyCandidateIds.has(item.candidate_id)).length
  const connectorCount = new Set(suggestions.map((item) => item.connector_id)).size
  const maxPriorityDelta = suggestions.reduce((max, item) => {
    if (item.old_priority === null || item.old_priority === undefined) return max
    return Math.max(max, Math.abs(item.new_priority - item.old_priority))
  }, 0)

  return [
    { label: tM('applyDialog.riskTotal'), value: suggestions.length },
    { label: tM('applyDialog.riskLowConfidence'), value: lowConfidenceCount },
    { label: tM('applyDialog.riskFailedCandidates'), value: riskySuggestionCount },
    { label: tM('applyDialog.riskConnectors'), value: connectorCount },
    { label: tM('applyDialog.riskMaxDelta'), value: maxPriorityDelta || '-' }
  ]
})

onMounted(() => {
  loadAll()
})

onBeforeUnmount(() => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
})

watch(autoRefreshEnabled, (val) => {
  if (val) startAutoRefresh()
  else stopAutoRefresh()
})

watch(autoRefreshInterval, () => {
  if (autoRefreshEnabled.value) startAutoRefresh()
})

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [connectorRes, candidateRes, runRes, accountRes, groupRes] = await Promise.all([
      upstreamRelayAPI.listConnectors({ page: 1, page_size: 100 }),
      upstreamRelayAPI.listCandidates({ page: 1, page_size: 100 }),
      upstreamRelayAPI.listRecommendationRuns({ page: 1, page_size: 20 }),
      accountsAPI.list(1, 200, { status: 'active' }),
      groupsAPI.getAllIncludingInactive()
    ])
    connectors.value = connectorRes.items
    candidates.value = candidateRes.items
    recommendationRuns.value = runRes.items
    accounts.value = accountRes.items
    groups.value = groupRes
    if (!selectedConnectorId.value && connectors.value.length > 0) {
      selectedConnectorId.value = connectors.value[0].id
    }
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
    const res = await upstreamRelayAPI.listCandidates({ page: 1, page_size: 100 })
    candidates.value = res.items
    lastLoadedAt.value = new Date().toISOString()
    await hydrateLatestPendingRun()
  } catch { /* silent */ }
}

function startAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  autoRefreshCountdown.value = autoRefreshInterval.value
  autoRefreshTimer = setInterval(() => {
    autoRefreshCountdown.value--
    if (autoRefreshCountdown.value <= 0) {
      autoRefreshCountdown.value = autoRefreshInterval.value
      refreshCandidatesSilent()
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

async function hydrateLatestPendingRun() {
  const run = recommendationRuns.value.find((item) => !item.applied && item.suggestion_count > 0)
  if (!run || run.suggestions) return
  try {
    const detail = await upstreamRelayAPI.getRecommendationRun(run.id)
    recommendationRuns.value = recommendationRuns.value.map((item) => (item.id === detail.id ? detail : item))
  } catch {
    // 候选主表仍可正常使用；建议明细加载失败时仅不展示行内 priority 差异。
  }
}

function resetConnectorForm() {
  Object.assign(connectorForm, { id: 0, name: '', base_url: '', auth_mode: 'manual_session', bearer_token: '', login_email: '', login_password: '', cookie: '', user_agent: '' })
}

function openCreateConnector() {
  resetConnectorForm()
  connectorDialogOpen.value = true
}

function closeConnectorDialog() {
  connectorDialogOpen.value = false
  resetConnectorForm()
}

function editConnector(connector: UpstreamRelayConnector) {
  Object.assign(connectorForm, {
    id: connector.id,
    name: connector.name,
    base_url: connector.base_url,
    auth_mode: connector.auth_mode,
    bearer_token: '',
    login_email: '',
    login_password: '',
    cookie: '',
    user_agent: ''
  })
  connectorDialogOpen.value = true
}

async function submitConnector() {
  savingConnector.value = true
  error.value = ''
  try {
    const payload = {
      name: connectorForm.name,
      base_url: connectorForm.base_url,
      auth_mode: connectorForm.auth_mode,
      bearer_token: connectorForm.auth_mode === 'manual_session' ? connectorForm.bearer_token || undefined : undefined,
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
    error.value = err instanceof Error ? err.message : tM('errors.saveConnectorFailed')
  } finally {
    savingConnector.value = false
  }
}

async function sync(connector: UpstreamRelayConnector) {
  syncingId.value = connector.id
  error.value = ''
  try {
    snapshots.value = await upstreamRelayAPI.syncConnector(connector.id)
    selectedConnectorId.value = connector.id
    snapshotConnector.value = connector
    snapshotDialogOpen.value = true
    // 静默刷新连接器列表以更新 last_synced_at，不触发全页 loading
    const res = await upstreamRelayAPI.listConnectors({ page: 1, page_size: 100 })
    connectors.value = res.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.syncFailed')
  } finally {
    syncingId.value = null
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
  Object.assign(candidateForm, { id: 0, connector_id: activeConnectors.value[0]?.id || 0, account_id: 0, upstream_group_id: '', probe_model: '', probe_protocol: 'chat_completions', target_group_id: 0, enabled: true, notes: '' })
  candidateSourceSnapshot.value = null
}

function openCreateCandidate() {
  resetCandidateForm()
  candidateDialogOpen.value = true
}

function closeCandidateDialog() {
  candidateDialogOpen.value = false
  resetCandidateForm()
}

function editCandidate(candidate: UpstreamRelayCandidate) {
  candidateSourceSnapshot.value = null
  Object.assign(candidateForm, {
    id: candidate.id,
    connector_id: candidate.connector_id,
    account_id: candidate.account_id,
    upstream_group_id: candidate.upstream_group_id,
    probe_model: candidate.probe_model,
    probe_protocol: candidate.probe_protocol,
    target_group_id: candidate.target_group_id,
    enabled: candidate.enabled,
    notes: candidate.notes || ''
  })
  candidateDialogOpen.value = true
}

function reuseCandidateAccount(candidate: UpstreamRelayCandidate) {
  candidateSourceSnapshot.value = null
  Object.assign(candidateForm, {
    id: 0,
    connector_id: candidate.connector_id,
    account_id: candidate.account_id,
    upstream_group_id: '',
    probe_model: candidate.probe_model,
    probe_protocol: candidate.probe_protocol,
    target_group_id: 0,
    enabled: candidate.enabled,
    notes: ''
  })
  candidateDialogOpen.value = true
}

function createCandidateFromSnapshot(snapshot: UpstreamRelayGroupRateSnapshot) {
  const defaults = candidateDefaultsForConnector(snapshot.connector_id || selectedConnectorId.value)
  candidateSourceSnapshot.value = snapshot
  activeSection.value = 'candidates'
  Object.assign(candidateForm, {
    id: 0,
    connector_id: snapshot.connector_id || selectedConnectorId.value,
    account_id: 0,
    upstream_group_id: snapshot.upstream_group_id,
    probe_model: defaults.probe_model,
    probe_protocol: defaults.probe_protocol,
    target_group_id: 0,
    enabled: true,
    notes: ''
  })
  snapshotDialogOpen.value = false
  candidateDialogOpen.value = true
}

function candidateDefaultsForConnector(connectorId: number): { probe_model: string; probe_protocol: UpstreamRelayProbeProtocol } {
  const candidate = candidates.value.find((item) => item.connector_id === connectorId && item.probe_model)
  return {
    probe_model: candidate?.probe_model || '',
    probe_protocol: candidate?.probe_protocol || 'chat_completions'
  }
}

async function submitCandidate(options: { continueAdding?: boolean } = {}) {
  savingCandidate.value = true
  error.value = ''
  try {
    const payload = {
      connector_id: candidateForm.connector_id,
      account_id: candidateForm.account_id,
      upstream_group_id: candidateForm.upstream_group_id,
      probe_model: candidateForm.probe_model,
      probe_protocol: candidateForm.probe_protocol,
      target_group_id: candidateForm.target_group_id,
      enabled: candidateForm.enabled,
      notes: candidateForm.notes
    }
    if (candidateForm.id) {
      await upstreamRelayAPI.updateCandidate(candidateForm.id, payload)
    } else {
      await upstreamRelayAPI.createCandidate(payload)
    }
    if (options.continueAdding && !candidateForm.id) {
      Object.assign(candidateForm, {
        upstream_group_id: '',
        target_group_id: 0,
        notes: ''
      })
      candidateSourceSnapshot.value = null
    } else {
      candidateDialogOpen.value = false
      resetCandidateForm()
    }
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.saveCandidateFailed')
  } finally {
    savingCandidate.value = false
  }
}

async function probe(candidate: UpstreamRelayCandidate) {
  probingId.value = candidate.id
  error.value = ''
  try {
    const result = await upstreamRelayAPI.probeCandidate(candidate.id)
    // 即时更新行内探测结果，无需整页刷新
    candidates.value = candidates.value.map((c) =>
      c.id === candidate.id ? { ...c, latest_probe: result } : c
    )
    // 后台静默刷新健康聚合数据
    refreshCandidatesSilent()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.probeFailed')
  } finally {
    probingId.value = null
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

async function generateRun() {
  generating.value = true
  error.value = ''
  try {
    const run = await upstreamRelayAPI.generateRecommendations()
    const detail = await upstreamRelayAPI.getRecommendationRun(run.id)
    recommendationRuns.value = [detail, ...recommendationRuns.value.filter((item) => item.id !== detail.id)]
    activeSection.value = 'recommendations'
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.generateFailed')
  } finally {
    generating.value = false
  }
}

async function openApplyDialog(run: UpstreamRelayRecommendationRun) {
  applyRun.value = await upstreamRelayAPI.getRecommendationRun(run.id)
  applyDialogOpen.value = true
}

async function applySelectedRun() {
  if (!applyRun.value) return
  applying.value = true
  error.value = ''
  try {
    await upstreamRelayAPI.applyRecommendationRun(applyRun.value.id)
    applyDialogOpen.value = false
    applyRun.value = null
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : tM('errors.applyFailed')
  } finally {
    applying.value = false
  }
}

function connectorStatusLabel(status: string) {
  const map: Record<string, string> = { active: tM('status.active'), needs_reauth: tM('status.needsReauth'), invalid: tM('status.invalid'), paused: tM('status.paused') }
  return map[status] || status
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

function formatDate(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatRate(value: number) {
  return Number(value).toFixed(4).replace(/\.?0+$/, '')
}

function formatNullableRate(value?: number | null) {
  return value === null || value === undefined ? '-' : formatRate(value)
}

function candidateRateLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.latest_snapshot?.final_rate_multiplier !== undefined && candidate.latest_snapshot?.final_rate_multiplier !== null) {
    return formatRate(candidate.latest_snapshot.final_rate_multiplier)
  }
  return formatNullableRate(candidate.latest_usage_delta?.derived_rate_multiplier)
}

function candidateRateSourceKey(candidate: UpstreamRelayCandidate): RateSourceKey {
  if (candidate.latest_snapshot?.source) return candidate.latest_snapshot.source
  if (candidate.latest_usage_delta?.status === 'reliable') return 'usage_cost_delta'
  return '无可用倍率'
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

function priorityDeltaLabel(suggestion: UpstreamRelayRecommendationSuggestion) {
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return tM('priorityDelta.new')
  const delta = suggestion.new_priority - suggestion.old_priority
  if (delta === 0) return tM('priorityDelta.unchanged')
  return delta > 0 ? `+${delta}` : `${delta}`
}

function priorityDeltaIcon(suggestion: UpstreamRelayRecommendationSuggestion): string {
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return 'arrowRight'
  const delta = suggestion.new_priority - suggestion.old_priority
  if (delta > 0) return 'arrowUp'
  if (delta < 0) return 'arrowDown'
  return 'arrowRight'
}

function priorityDeltaClass(suggestion: UpstreamRelayRecommendationSuggestion): string {
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return 'text-primary-600 dark:text-primary-400'
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
