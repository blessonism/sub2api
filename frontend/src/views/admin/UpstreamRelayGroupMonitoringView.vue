<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">上游倍率监控</h1>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
            <span>最近刷新: {{ formatDate(lastLoadedAt) }}</span>
            <span>最近同步: {{ overviewStats.latestSyncedAt }}</span>
            <span>统计基于当前已加载数据</span>
          </div>
        </div>
        <div class="flex flex-wrap justify-start gap-2 lg:justify-end">
          <button class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="generating" @click="generateRun">
            <Icon name="chart" size="sm" />
            {{ generating ? '生成中...' : '生成建议' }}
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
            新建候选
          </button>
          <button v-if="activeSection === 'connectors'" class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreateConnector">
            <Icon name="plus" size="sm" />
            新建连接器
          </button>
        </div>
      </div>

      <section v-if="activeSection === 'candidates'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">候选映射</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">核心决策视图：倍率来源、健康状态和待应用 Priority 变化。</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ candidates.length }} 个候选，{{ enabledCandidateCount }} 个启用</span>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1180px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">候选</th>
                <th class="px-4 py-3 text-left">映射</th>
                <th class="px-4 py-3 text-right">倍率</th>
                <th class="px-4 py-3 text-left">健康</th>
                <th class="px-4 py-3 text-right">Priority</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="candidate in candidates" :key="candidate.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">#{{ candidate.account_id }} {{ candidate.account_name || '-' }}</span>
                    <span :class="candidate.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'" class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium">
                      {{ candidate.enabled ? '启用' : '停用' }}
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
                  <span :class="rateSourceClass(candidateRateSourceKey(candidate))" class="mt-1 inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ candidateRateSourceLabel(candidate) }}
                  </span>
                  <div v-if="candidate.latest_usage_delta" class="mt-1 text-xs" :class="usageDeltaClass(candidate.latest_usage_delta.status)">
                    usage: {{ usageDeltaLabel(candidate) }}
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span :class="candidateHealthClass(candidate)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ candidateHealthLabel(candidate) }}
                  </span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ candidate.latest_probe ? `${candidate.latest_probe.latency_ms ?? '-'}ms · ${candidate.latest_probe.http_status ?? '-'}` : '尚无探测样本' }}
                  </div>
                  <div v-if="candidate.health" class="mt-1 text-xs text-gray-600 dark:text-gray-300">
                    成功率 {{ formatPercent(candidate.health.success_rate) }} · 连成 {{ candidate.health.consecutive_successes }} · 连败 {{ candidate.health.consecutive_failures }}
                  </div>
                  <div v-if="candidate.health?.p95_latency_ms" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    p95 {{ candidate.health.p95_latency_ms }}ms · 样本 {{ candidate.health.sample_size }}/20
                  </div>
                  <div v-if="candidate.health?.last_error_class" class="mt-1 text-xs text-amber-600 dark:text-amber-300">{{ errorClassLabel(candidate.health.last_error_class) }}</div>
                  <div v-if="candidate.latest_probe?.error_message" class="mt-1 max-w-[280px] truncate text-xs text-red-500">{{ candidate.latest_probe.error_message }}</div>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="tabular-nums text-gray-900 dark:text-white">{{ candidate.current_priority ?? '-' }}</div>
                  <div v-if="candidatePendingSuggestion(candidate)" class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-300">
                    建议 {{ candidatePendingSuggestion(candidate)?.new_priority }}
                    <span class="text-gray-400 dark:text-gray-500">({{ priorityDeltaLabel(candidatePendingSuggestion(candidate)!) }})</span>
                  </div>
                  <div v-else class="mt-1 text-xs text-gray-400 dark:text-gray-500">暂无待应用建议</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editCandidate(candidate)">编辑</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="probingId === candidate.id" @click="probe(candidate)">
                      {{ probingId === candidate.id ? '探测中' : '探测' }}
                    </button>
                    <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeCandidate(candidate)">删除</button>
                  </div>
                </td>
              </tr>
              <tr v-if="candidates.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无候选映射，点击“新建候选”开始配置。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="activeSection === 'connectors'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">连接器状态</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">维护上游凭证、同步可见分组，并从行内查看倍率快照。</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ connectors.length }} 个连接器</span>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">名称</th>
                <th class="px-4 py-3 text-left">状态</th>
                <th class="px-4 py-3 text-left">凭证</th>
                <th class="px-4 py-3 text-left">最近验证/同步</th>
                <th class="px-4 py-3 text-right">操作</th>
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
                  <div>模式: {{ authModeLabel(connector.auth_mode) }}</div>
                  <div>{{ connectorCredentialSummary(connector) }}</div>
                  <div v-if="connector.has_login_email">邮箱: {{ connector.login_email_masked || '已保存' }}</div>
                </td>
                <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <div>验证: {{ formatDate(connector.last_verified_at) }}</div>
                  <div class="mt-1">同步: {{ formatDate(connector.last_synced_at) }}</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editConnector(connector)">编辑</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="syncingId === connector.id" @click="sync(connector)">
                      {{ syncingId === connector.id ? '同步中' : '同步' }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="openSnapshotDialog(connector)">快照</button>
                    <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeConnector(connector)">删除</button>
                  </div>
                </td>
              </tr>
              <tr v-if="connectors.length === 0">
                <td colspan="5" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无连接器，点击“新建连接器”开始配置。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="activeSection === 'recommendations'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Priority 建议</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">应用前先查看风险摘要，避免低置信或陈旧建议被误用。</p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ pendingSuggestionCount }} 条待应用建议</span>
        </div>
        <div v-if="loading" class="flex min-h-56 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">Run</th>
                <th class="px-4 py-3 text-right">候选数</th>
                <th class="px-4 py-3 text-right">建议数</th>
                <th class="px-4 py-3 text-left">状态</th>
                <th class="px-4 py-3 text-left">创建时间</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="run in recommendationRuns" :key="run.id">
                <td class="px-4 py-3 font-mono">#{{ run.id }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.total_candidates }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ run.suggestion_count }}</td>
                <td class="px-4 py-3">
                  <span :class="run.applied ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                    {{ run.applied ? '已应用' : '待确认' }}
                  </span>
                </td>
                <td class="px-4 py-3">{{ formatDate(run.created_at) }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="btn btn-primary px-3 py-1.5 text-xs" type="button" :disabled="run.applied || run.suggestion_count === 0" @click="openApplyDialog(run)">
                    查看并应用
                  </button>
                </td>
              </tr>
              <tr v-if="recommendationRuns.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无建议</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>

  <BaseDialog :show="connectorDialogOpen" :title="connectorForm.id ? '编辑上游连接器' : '新建上游连接器'" width="wide" @close="closeConnectorDialog">
    <form id="connector-form" class="space-y-4" @submit.prevent="submitConnector">
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">名称</span>
        <input v-model.trim="connectorForm.name" class="input w-full" type="text" />
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Base URL</span>
        <input v-model.trim="connectorForm.base_url" class="input w-full" type="url" placeholder="https://upstream.example.com" />
      </label>
      <div class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'manual_session' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @click="connectorForm.auth_mode = 'manual_session'"
        >
          手动登录态
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-2 text-sm font-medium transition"
          :class="connectorForm.auth_mode === 'password_login' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
          @click="connectorForm.auth_mode = 'password_login'"
        >
          账号密码
        </button>
      </div>
      <template v-if="connectorForm.auth_mode === 'manual_session'">
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Authorization Bearer token</span>
          <input v-model.trim="connectorForm.bearer_token" class="input w-full" type="password" autocomplete="new-password" placeholder="保存后不回显，留空表示不修改" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Cookie</span>
          <textarea v-model.trim="connectorForm.cookie" class="input min-h-[74px] w-full" placeholder="必要 Cookie，保存后不回显" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">User-Agent</span>
          <input v-model.trim="connectorForm.user_agent" class="input w-full" type="text" placeholder="浏览器请求里的 User-Agent" />
        </label>
      </template>
      <template v-else>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">上游邮箱</span>
          <input v-model.trim="connectorForm.login_email" class="input w-full" type="email" autocomplete="username" :placeholder="connectorForm.id ? '留空沿用已保存邮箱' : 'admin@example.com'" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">上游密码</span>
          <input v-model.trim="connectorForm.login_password" class="input w-full" type="password" autocomplete="new-password" placeholder="仅用于本次换取 token，不保存" />
        </label>
      </template>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="closeConnectorDialog">取消</button>
        <button class="btn btn-primary" type="submit" form="connector-form" :disabled="savingConnector">
          {{ savingConnector ? '保存中...' : connectorForm.id ? '更新并验证' : '保存并验证' }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog :show="candidateDialogOpen" :title="candidateForm.id ? '编辑候选映射' : '新建候选映射'" width="wide" @close="closeCandidateDialog">
    <form id="candidate-form" class="space-y-4" @submit.prevent="submitCandidate">
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">连接器</span>
        <select v-model.number="candidateForm.connector_id" class="input w-full">
          <option :value="0">请选择</option>
          <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
        </select>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">本地账号</span>
        <select v-model.number="candidateForm.account_id" class="input w-full">
          <option :value="0">请选择</option>
          <option v-for="account in accounts" :key="account.id" :value="account.id">#{{ account.id }} {{ account.name }} · {{ account.platform }}</option>
        </select>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">上游 Group ID</span>
        <input v-model.trim="candidateForm.upstream_group_id" class="input w-full" type="text" />
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">本地 priority 目标分组</span>
        <select v-model.number="candidateForm.target_group_id" class="input w-full">
          <option :value="0">请选择</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">#{{ group.id }} {{ group.name }} · {{ group.platform }}</option>
        </select>
      </label>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">探测模型</span>
          <input v-model.trim="candidateForm.probe_model" class="input w-full" type="text" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">协议</span>
          <select v-model="candidateForm.probe_protocol" class="input w-full">
            <option value="chat_completions">chat_completions</option>
            <option value="responses">responses</option>
          </select>
        </label>
      </div>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="candidateForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
        启用候选
      </label>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="closeCandidateDialog">取消</button>
        <button class="btn btn-primary" type="submit" form="candidate-form" :disabled="savingCandidate">
          {{ savingCandidate ? '保存中...' : candidateForm.id ? '更新候选' : '创建候选' }}
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
            <th class="px-4 py-3 text-left">上游 Group ID</th>
            <th class="px-4 py-3 text-left">名称</th>
            <th class="px-4 py-3 text-left">平台</th>
            <th class="px-4 py-3 text-right">默认倍率</th>
            <th class="px-4 py-3 text-right">覆盖倍率</th>
            <th class="px-4 py-3 text-right">最终倍率</th>
            <th class="px-4 py-3 text-left">来源</th>
            <th class="px-4 py-3 text-left">Last Seen</th>
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
            <td class="px-4 py-3">{{ rateSourceLabel(snapshot.source) }}</td>
            <td class="px-4 py-3">{{ formatDate(snapshot.last_seen_at) }}</td>
          </tr>
          <tr v-if="snapshots.length === 0">
            <td colspan="8" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无快照</td>
          </tr>
        </tbody>
      </table>
    </div>
  </BaseDialog>

  <BaseDialog :show="applyDialogOpen" title="确认应用 priority 建议" width="extra-wide" @close="applyDialogOpen = false">
    <div class="space-y-4">
      <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
        Run #{{ applyRun?.id || '-' }} 创建于 {{ formatDate(applyRun?.created_at) }}。应用前请确认低置信与未探测候选。
      </div>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div v-for="item in applyRiskCards" :key="item.label" class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</div>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[1040px] text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-3 py-3 text-left">账号</th>
              <th class="px-3 py-3 text-left">目标分组</th>
              <th class="px-3 py-3 text-right">原 priority</th>
              <th class="px-3 py-3 text-right">新 priority</th>
              <th class="px-3 py-3 text-right">倍率</th>
              <th class="px-3 py-3 text-left">置信度</th>
              <th class="px-3 py-3 text-left">健康摘要</th>
              <th class="px-3 py-3 text-left">原因</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="suggestion in applyRun?.suggestions || []" :key="suggestion.id || suggestion.candidate_id">
              <td class="px-3 py-3">#{{ suggestion.account_id }} {{ suggestion.account_name || '-' }}</td>
              <td class="px-3 py-3">{{ suggestion.target_group_name || `#${suggestion.target_group_id}` }}</td>
              <td class="px-3 py-3 text-right tabular-nums">{{ suggestion.old_priority ?? '-' }}</td>
              <td class="px-3 py-3 text-right font-semibold tabular-nums">{{ suggestion.new_priority }}</td>
              <td class="px-3 py-3 text-right tabular-nums">{{ formatRate(suggestion.final_rate_multiplier) }}</td>
              <td class="px-3 py-3">
                <span :class="confidenceClass(suggestion.confidence)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                  {{ confidenceLabel(suggestion.confidence) }}
                </span>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ rateSourceLabel(suggestion.rate_source) }}</div>
              </td>
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ suggestion.health_summary || '-' }}</td>
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ suggestion.reason }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="applyDialogOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="applying" @click="applySelectedRun">
          {{ applying ? '应用中...' : '确认应用' }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import accountsAPI from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import upstreamRelayAPI, {
  type UpstreamRelayCandidate,
  type UpstreamRelayConnector,
  type UpstreamRelayGroupRateSnapshot,
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

const sections: Array<{ key: SectionKey; label: string }> = [
  { key: 'candidates', label: '候选映射' },
  { key: 'connectors', label: '连接器' },
  { key: 'recommendations', label: 'Priority 建议' }
]

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
    label: '可用连接器',
    value: overviewStats.value.activeConnectors,
    hint: `共 ${connectors.value.length} 个已加载`,
    section: 'connectors' as SectionKey
  },
  {
    key: 'reauth-connectors',
    label: '需重新登录',
    value: overviewStats.value.needsReauth,
    hint: '优先处理凭证状态',
    section: 'connectors' as SectionKey
  },
  {
    key: 'enabled-candidates',
    label: '启用候选',
    value: overviewStats.value.enabledCandidates,
    hint: `共 ${candidates.value.length} 个已加载`,
    section: 'candidates' as SectionKey
  },
  {
    key: 'failed-candidates',
    label: '探测失败',
    value: overviewStats.value.failedCandidates,
    hint: '失败与认证异常候选',
    section: 'candidates' as SectionKey
  },
  {
    key: 'pending-suggestions',
    label: '待应用建议',
    value: overviewStats.value.pendingSuggestions,
    hint: '应用前需确认风险',
    section: 'recommendations' as SectionKey
  },
  {
    key: 'latest-sync',
    label: '最近同步',
    value: overviewStats.value.latestSyncedAt,
    hint: '基于当前连接器列表',
    section: 'connectors' as SectionKey
  }
])

const snapshotDialogTitle = computed(() => {
  return snapshotConnector.value ? `${snapshotConnector.value.name} 的倍率快照` : '上游倍率快照'
})

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
    { label: '建议总数', value: suggestions.length },
    { label: '低置信/未知', value: lowConfidenceCount },
    { label: '失败/未探测候选', value: riskySuggestionCount },
    { label: '涉及连接器', value: connectorCount },
    { label: '最大 Priority 变化', value: maxPriorityDelta || '-' }
  ]
})

onMounted(() => {
  loadAll()
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
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
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
    error.value = err instanceof Error ? err.message : '保存连接器失败'
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
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '同步失败'
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
    error.value = err instanceof Error ? err.message : '加载快照失败'
  } finally {
    snapshotLoading.value = false
  }
}

async function removeConnector(connector: UpstreamRelayConnector) {
  if (!window.confirm(`确认删除连接器 ${connector.name}？相关候选映射和待应用建议可能失去上游依据。`)) return
  await upstreamRelayAPI.deleteConnector(connector.id)
  await loadAll()
}

function resetCandidateForm() {
  Object.assign(candidateForm, { id: 0, connector_id: activeConnectors.value[0]?.id || 0, account_id: 0, upstream_group_id: '', probe_model: '', probe_protocol: 'chat_completions', target_group_id: 0, enabled: true, notes: '' })
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

async function submitCandidate() {
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
    candidateDialogOpen.value = false
    resetCandidateForm()
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存候选失败'
  } finally {
    savingCandidate.value = false
  }
}

async function probe(candidate: UpstreamRelayCandidate) {
  probingId.value = candidate.id
  error.value = ''
  try {
    await upstreamRelayAPI.probeCandidate(candidate.id)
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '探测失败'
  } finally {
    probingId.value = null
  }
}

async function removeCandidate(candidate: UpstreamRelayCandidate) {
  if (!window.confirm(`确认删除候选 #${candidate.id}？相关 Priority 建议将不再适用于该候选。`)) return
  await upstreamRelayAPI.deleteCandidate(candidate.id)
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
    error.value = err instanceof Error ? err.message : '生成建议失败'
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
    error.value = err instanceof Error ? err.message : '应用建议失败'
  } finally {
    applying.value = false
  }
}

function connectorStatusLabel(status: string) {
  const map: Record<string, string> = {
    active: '可用',
    needs_reauth: '需重新登录',
    invalid: '无效',
    paused: '已暂停'
  }
  return map[status] || status
}

function authModeLabel(mode: string) {
  const map: Record<string, string> = {
    manual_session: '手动登录态',
    password_login: '账号密码'
  }
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

function formatPercent(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return `${Math.round(value * 100)}%`
}

function candidateRateLabel(candidate: UpstreamRelayCandidate) {
  if (candidate.latest_snapshot?.final_rate_multiplier !== undefined && candidate.latest_snapshot?.final_rate_multiplier !== null) {
    return formatRate(candidate.latest_snapshot.final_rate_multiplier)
  }
  return formatNullableRate(candidate.latest_usage_delta?.derived_rate_multiplier)
}

function candidateRateSourceLabel(candidate: UpstreamRelayCandidate) {
  return rateSourceLabel(candidateRateSourceKey(candidate))
}

function candidateRateSourceKey(candidate: UpstreamRelayCandidate): RateSourceKey {
  if (candidate.latest_snapshot?.source) return candidate.latest_snapshot.source
  if (candidate.latest_usage_delta?.status === 'reliable') return 'usage_cost_delta'
  return '无可用倍率'
}

function rateSourceClass(source: RateSourceKey) {
  if (source === 'login_user_group_rates') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (source === 'login_available_groups') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  if (source === 'usage_cost_delta') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
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
  if (severity === 'success') return '成功'
  if (severity === 'auth') return errorClassLabel(candidate.health?.last_error_class || 'auth_failed')
  if (severity === 'insufficient') return '样本不足'
  if (severity === 'failed') return '失败'
  return '未探测'
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
  if (suggestion.old_priority === null || suggestion.old_priority === undefined) return '新增'
  const delta = suggestion.new_priority - suggestion.old_priority
  if (delta === 0) return '不变'
  return delta > 0 ? `+${delta}` : `${delta}`
}

function connectorCredentialSummary(connector: UpstreamRelayConnector) {
  const saved: string[] = []
  if (connector.has_bearer_token) saved.push('Token')
  if (connector.has_refresh_token) saved.push('Refresh')
  if (connector.has_cookie) saved.push('Cookie')
  if (connector.has_user_agent) saved.push('UA')
  return saved.length > 0 ? `已保存: ${saved.join(' / ')}` : '未保存凭证'
}

function usageDeltaLabel(candidate: UpstreamRelayCandidate) {
  const sample = candidate.latest_usage_delta
  if (!sample) return '-'
  if (sample.status === 'reliable') {
    return `倍率 ${formatNullableRate(sample.derived_rate_multiplier)}`
  }
  return sample.unreliable_reason || usageDeltaStatusLabel(sample.status)
}

function usageDeltaStatusLabel(status: string) {
  const map: Record<string, string> = {
    reliable: '可信',
    insufficient: '样本不足',
    unavailable: '不可用'
  }
  return map[status] || status
}

function usageDeltaClass(status: string) {
  if (status === 'reliable') return 'text-emerald-600 dark:text-emerald-300'
  if (status === 'insufficient') return 'text-amber-600 dark:text-amber-300'
  return 'text-gray-500 dark:text-gray-400'
}

function rateSourceLabel(source: string) {
  const map: Record<string, string> = {
    login_user_group_rates: '用户专属倍率',
    login_available_groups: '可见分组倍率',
    usage_cost_delta: '/v1/usage 兜底'
  }
  return map[source] || source || '-'
}

function confidenceLabel(confidence: string) {
  const map: Record<string, string> = {
    high: '高',
    medium: '中',
    low: '低',
    unknown: '未知'
  }
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
    auth_failed: '认证失败',
    rate_limited: '限流',
    upstream_5xx: '上游 5xx',
    timeout: '超时',
    model_unavailable: '模型不可用',
    insufficient_quota: '余额或额度不足',
    context_window_exceeded: '上下文超限',
    browser_challenge: '浏览器挑战',
    network_error: '网络错误',
    invalid_request: '请求参数错误',
    request_failed: '请求失败'
  }
  return map[errorClass] || errorClass
}
</script>
