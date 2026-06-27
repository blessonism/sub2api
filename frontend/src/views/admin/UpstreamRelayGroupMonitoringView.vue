<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap justify-end gap-2">
        <button class="btn btn-secondary inline-flex items-center gap-2" type="button" :disabled="loading" @click="loadAll">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
        <button class="btn btn-primary inline-flex items-center gap-2" type="button" :disabled="generating" @click="generateRun">
          <Icon name="chart" size="sm" />
          生成建议
        </button>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
        {{ error }}
      </div>

      <section class="grid grid-cols-1 gap-6 xl:grid-cols-[420px_1fr]">
        <div class="card p-4">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">上游连接器</h2>
            <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" @click="resetConnectorForm">新建</button>
          </div>
          <form class="mt-4 space-y-3" @submit.prevent="submitConnector">
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
            <button class="btn btn-primary w-full" type="submit" :disabled="savingConnector">
              {{ savingConnector ? '保存中...' : connectorForm.id ? '更新并验证' : '保存并验证' }}
            </button>
          </form>
        </div>

        <div class="card overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">连接器列表</h2>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ connectors.length }} 个</span>
          </div>
          <div v-if="loading" class="flex min-h-56 items-center justify-center">
            <LoadingSpinner />
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[880px] text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3 text-left">名称</th>
                  <th class="px-4 py-3 text-left">状态</th>
                  <th class="px-4 py-3 text-left">凭证</th>
                  <th class="px-4 py-3 text-left">最近同步</th>
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
                    <div v-if="connector.has_login_email">邮箱: {{ connector.login_email_masked || '已保存' }}</div>
                    <div>Token: {{ connector.has_bearer_token ? connector.bearer_token_masked || '已保存' : '未保存' }}</div>
                    <div>Refresh: {{ connector.has_refresh_token ? connector.refresh_token_masked || '已保存' : '未保存' }}</div>
                    <div>Cookie: {{ connector.has_cookie ? connector.cookie_masked || '已保存' : '未保存' }}</div>
                    <div>UA: {{ connector.has_user_agent ? connector.user_agent_masked || '已保存' : '未保存' }}</div>
                  </td>
                  <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDate(connector.last_synced_at) }}</td>
                  <td class="px-4 py-3">
                    <div class="flex justify-end gap-2">
                      <button class="btn btn-secondary px-2 py-1 text-xs" type="button" @click="editConnector(connector)">编辑</button>
                      <button class="btn btn-secondary px-2 py-1 text-xs" type="button" :disabled="syncingId === connector.id" @click="sync(connector)">
                        {{ syncingId === connector.id ? '同步中' : '同步' }}
                      </button>
                      <button class="btn btn-danger px-2 py-1 text-xs" type="button" @click="removeConnector(connector)">删除</button>
                    </div>
                  </td>
                </tr>
                <tr v-if="connectors.length === 0">
                  <td colspan="5" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无连接器</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">上游倍率快照</h2>
          <select v-model.number="selectedConnectorId" class="input w-64" @change="loadSnapshots">
            <option :value="0">选择连接器</option>
            <option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option>
          </select>
        </div>
        <div class="overflow-x-auto">
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
                <td class="px-4 py-3">{{ snapshot.source }}</td>
                <td class="px-4 py-3">{{ formatDate(snapshot.last_seen_at) }}</td>
              </tr>
              <tr v-if="snapshots.length === 0">
                <td colspan="8" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无快照</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-6 xl:grid-cols-[420px_1fr]">
        <div class="card p-4">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">候选通道映射</h2>
            <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" @click="resetCandidateForm">新建</button>
          </div>
          <form class="mt-4 space-y-3" @submit.prevent="submitCandidate">
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
            <button class="btn btn-primary w-full" type="submit" :disabled="savingCandidate">
              {{ savingCandidate ? '保存中...' : candidateForm.id ? '更新候选' : '创建候选' }}
            </button>
          </form>
        </div>

        <div class="card overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">候选列表</h2>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ candidates.length }} 个</span>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[1100px] text-sm">
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
                    <div class="font-medium text-gray-900 dark:text-white">#{{ candidate.account_id }} {{ candidate.account_name || '-' }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ candidate.account_platform || '-' }} · {{ candidate.probe_model }} · {{ candidate.probe_protocol }}</div>
                  </td>
                  <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div>{{ candidate.connector_name || `Connector #${candidate.connector_id}` }}</div>
                    <div class="mt-1 font-mono">{{ candidate.upstream_group_id }} → {{ candidate.target_group_name || `Group #${candidate.target_group_id}` }}</div>
                  </td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ formatNullableRate(candidate.latest_snapshot?.final_rate_multiplier) }}</td>
                  <td class="px-4 py-3">
                    <span :class="candidate.latest_probe?.success ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                      {{ candidate.latest_probe?.success ? '成功' : candidate.latest_probe ? '失败' : '未探测' }}
                    </span>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ candidate.latest_probe ? `${candidate.latest_probe.latency_ms ?? '-'}ms · ${candidate.latest_probe.http_status ?? '-'}` : '-' }}
                    </div>
                    <div v-if="candidate.latest_probe?.error_message" class="mt-1 max-w-[280px] truncate text-xs text-red-500">{{ candidate.latest_probe.error_message }}</div>
                  </td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ candidate.current_priority ?? '-' }}</td>
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
                  <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-gray-400">暂无候选映射</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Priority 建议</h2>
          <span class="text-sm text-gray-500 dark:text-gray-400">应用前需确认影响范围</span>
        </div>
        <div class="overflow-x-auto">
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

  <div v-if="applyDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
    <div class="max-h-[86vh] w-full max-w-5xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900">
      <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">确认应用 priority 建议</h3>
        <button class="btn btn-secondary px-2 py-1 text-sm" type="button" @click="applyDialogOpen = false">关闭</button>
      </div>
      <div class="max-h-[62vh] overflow-auto p-5">
        <table class="w-full min-w-[860px] text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-3 py-3 text-left">账号</th>
              <th class="px-3 py-3 text-left">目标分组</th>
              <th class="px-3 py-3 text-right">原 priority</th>
              <th class="px-3 py-3 text-right">新 priority</th>
              <th class="px-3 py-3 text-right">倍率</th>
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
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ suggestion.reason }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex justify-end gap-2 border-t border-gray-100 px-5 py-4 dark:border-dark-700">
        <button class="btn btn-secondary" type="button" @click="applyDialogOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="applying" @click="applySelectedRun">
          {{ applying ? '应用中...' : '确认应用' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import accountsAPI from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import upstreamRelayAPI, {
  type UpstreamRelayCandidate,
  type UpstreamRelayConnector,
  type UpstreamRelayGroupRateSnapshot,
  type UpstreamRelayRecommendationRun
} from '@/api/admin/upstreamRelayGroupMonitors'
import type { Account, AdminGroup } from '@/types'

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
const savingConnector = ref(false)
const savingCandidate = ref(false)
const applyDialogOpen = ref(false)
const applyRun = ref<UpstreamRelayRecommendationRun | null>(null)

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
    await loadSnapshots()
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

function resetConnectorForm() {
  Object.assign(connectorForm, { id: 0, name: '', base_url: '', auth_mode: 'manual_session', bearer_token: '', login_email: '', login_password: '', cookie: '', user_agent: '' })
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
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '同步失败'
  } finally {
    syncingId.value = null
  }
}

async function removeConnector(connector: UpstreamRelayConnector) {
  if (!window.confirm(`确认删除连接器 ${connector.name}？`)) return
  await upstreamRelayAPI.deleteConnector(connector.id)
  await loadAll()
}

function resetCandidateForm() {
  Object.assign(candidateForm, { id: 0, connector_id: activeConnectors.value[0]?.id || 0, account_id: 0, upstream_group_id: '', probe_model: '', probe_protocol: 'chat_completions', target_group_id: 0, enabled: true, notes: '' })
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
  if (!window.confirm(`确认删除候选 #${candidate.id}？`)) return
  await upstreamRelayAPI.deleteCandidate(candidate.id)
  await loadAll()
}

async function generateRun() {
  generating.value = true
  error.value = ''
  try {
    const run = await upstreamRelayAPI.generateRecommendations()
    recommendationRuns.value = [run, ...recommendationRuns.value.filter((item) => item.id !== run.id)]
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
</script>
