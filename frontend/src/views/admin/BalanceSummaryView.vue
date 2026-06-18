<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.balanceSummary.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.balanceSummary.description') }}
          </p>
        </div>
        <button
          class="btn btn-primary inline-flex items-center gap-2 self-start md:self-auto"
          type="button"
          :disabled="loading"
          @click="refreshSummary"
        >
          <Icon name="refresh" size="sm" />
          {{ t('admin.balanceSummary.refresh') }}
        </button>
      </div>

      <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200">
        {{ t('admin.balanceSummary.scopeNotice') }}
      </div>

      <div v-if="loading && !summary" class="flex min-h-80 items-center justify-center">
        <LoadingSpinner />
      </div>

      <template v-else-if="summary">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <SummaryMetric
            v-for="metric in topMetrics"
            :key="metric.label"
            :label="metric.label"
            :value="metric.value"
            :hint="metric.hint"
            :icon="metric.icon"
            :tone="metric.tone"
          />
        </div>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
          <BucketPanel
            :title="t('admin.balanceSummary.byStatus')"
            :rows="summary.by_status"
            :label-for-key="statusLabel"
          />
          <BucketPanel
            :title="t('admin.balanceSummary.byRole')"
            :rows="summary.by_role"
            :label-for-key="roleLabel"
          />
        </div>

        <section class="space-y-5">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.balanceSummary.exclusionsTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.balanceSummary.exclusionsDescription') }}
              </p>
            </div>
            <button
              class="btn btn-primary inline-flex items-center justify-center gap-2 self-start"
              type="button"
              :disabled="saving || !hasChanges"
              @click="saveExclusions"
            >
              <Icon name="check" size="sm" />
              {{ saving ? t('admin.balanceSummary.saving') : t('admin.balanceSummary.saveExclusions') }}
            </button>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-4 lg:grid-cols-[minmax(280px,420px)_1fr]">
            <section class="card p-4">
              <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.balanceSummary.searchUser') }}
              </label>
              <div class="mt-2 flex gap-2">
                <div class="relative flex-1">
                  <Icon
                    name="search"
                    size="sm"
                    class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                  />
                  <input
                    v-model.trim="searchQuery"
                    type="search"
                    class="input w-full pl-9"
                    :placeholder="t('admin.balanceSummary.searchPlaceholder')"
                    @keyup.enter="searchUsers"
                  />
                </div>
                <button class="btn btn-secondary" type="button" :disabled="searching" @click="searchUsers">
                  {{ t('common.search') }}
                </button>
              </div>

              <div class="mt-4 min-h-32">
                <div v-if="searching" class="flex h-32 items-center justify-center">
                  <LoadingSpinner />
                </div>
                <EmptyState
                  v-else-if="searchAttempted && searchResults.length === 0"
                  :title="t('admin.balanceSummary.noSearchResults')"
                  :description="t('admin.balanceSummary.noSearchResultsDescription')"
                />
                <div v-else class="space-y-2">
                  <button
                    v-for="user in searchResults"
                    :key="user.id"
                    type="button"
                    class="flex w-full items-center justify-between gap-3 rounded-lg border border-gray-200 p-3 text-left transition hover:border-primary-300 hover:bg-primary-50 dark:border-dark-700 dark:hover:border-primary-700 dark:hover:bg-primary-950/30"
                    :disabled="isExcluded(user.id)"
                    @click="addExcludedUser(user)"
                  >
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                        {{ displayUser(user) }}
                      </div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        {{ user.email }} · {{ roleLabel(user.role) }} · {{ statusLabel(user.status) }}
                      </div>
                    </div>
                    <span class="shrink-0 text-xs font-medium text-primary-600 dark:text-primary-300">
                      {{ isExcluded(user.id) ? t('admin.balanceSummary.added') : t('admin.balanceSummary.add') }}
                    </span>
                  </button>
                </div>
              </div>
            </section>

            <section class="card p-4">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.balanceSummary.currentExclusions') }}
                  </h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.balanceSummary.currentExclusionsHint', { count: draftExclusions.length }) }}
                  </p>
                </div>
                <button
                  class="btn btn-secondary px-3 py-1.5 text-sm"
                  type="button"
                  :disabled="draftExclusions.length === 0"
                  @click="clearDraftExclusions"
                >
                  {{ t('admin.balanceSummary.clearAll') }}
                </button>
              </div>

              <EmptyState
                v-if="draftExclusions.length === 0"
                class="min-h-48"
                :title="t('admin.balanceSummary.emptyExclusions')"
                :description="t('admin.balanceSummary.emptyExclusionsDescription')"
              />
              <div v-else class="mt-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
                <table class="w-full min-w-[680px] text-sm">
                  <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                    <tr>
                      <th class="px-4 py-3 text-left">{{ t('admin.balanceSummary.user') }}</th>
                      <th class="px-4 py-3 text-left">{{ t('admin.balanceSummary.status') }}</th>
                      <th class="px-4 py-3 text-left">{{ t('admin.balanceSummary.role') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.balanceSummary.balance') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.balanceSummary.action') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr
                      v-for="user in draftExclusions"
                      :key="user.user_id"
                      :class="user.valid ? '' : 'bg-amber-50 dark:bg-amber-950/20'"
                    >
                      <td class="px-4 py-3">
                        <div class="font-medium text-gray-900 dark:text-white">
                          {{ displayExcludedUser(user) }}
                        </div>
                        <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                          {{ user.email || invalidReasonLabel(user.reason) }}
                        </div>
                      </td>
                      <td class="px-4 py-3">
                        <span :class="statusClass(user.status)" class="inline-flex rounded-md px-2 py-1 text-xs font-medium">
                          {{ user.valid ? statusLabel(user.status || '') : t('admin.balanceSummary.invalid') }}
                        </span>
                      </td>
                      <td class="px-4 py-3 text-gray-600 dark:text-gray-300">
                        {{ user.valid ? roleLabel(user.role || '') : '-' }}
                      </td>
                      <td class="px-4 py-3 text-right tabular-nums text-gray-900 dark:text-white">
                        {{ user.valid ? formatMoney(user.balance || 0) : '-' }}
                      </td>
                      <td class="px-4 py-3 text-right">
                        <button
                          class="btn btn-secondary px-2 py-1"
                          type="button"
                          :aria-label="t('admin.balanceSummary.removeUser', { user: displayExcludedUser(user) })"
                          @click="removeExcludedUser(user.user_id)"
                        >
                          <Icon name="trash" size="sm" />
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        </section>

        <div
          v-if="summary.invalid_exclusions > 0"
          class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
        >
          {{ t('admin.balanceSummary.invalidExclusionsHint', { count: summary.invalid_exclusions }) }}
        </div>
      </template>

      <EmptyState
        v-else
        class="min-h-80"
        :title="t('admin.balanceSummary.failedTitle')"
        :description="t('admin.balanceSummary.failedDescription')"
        :action-text="t('admin.balanceSummary.refresh')"
        @action="refreshSummary"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AdminBalanceSummaryBucket,
  AdminBalanceSummaryExcludedUser,
  AdminBalanceSummaryResponse
} from '@/api/admin/dashboard'
import type { AdminUser } from '@/types'
import { formatNumber } from '@/utils/format'

type MetricTone = 'blue' | 'emerald' | 'amber' | 'slate'
type MetricIconName = 'dollar' | 'users' | 'user' | 'calculator'

interface MetricItem {
  label: string
  value: string
  hint: string
  icon: MetricIconName
  tone: MetricTone
}

const SummaryMetric = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    hint: { type: String, required: true },
    icon: { type: String as PropType<MetricIconName>, required: true },
    tone: { type: String as PropType<MetricTone>, required: true }
  },
  setup(props) {
    const toneClasses: Record<string, string> = {
      blue: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300',
      emerald: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
      amber: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
      slate: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
    }

    return () =>
      h('div', { class: 'card p-4' }, [
        h('div', { class: 'flex items-start gap-3' }, [
          h('div', { class: ['rounded-lg p-2', toneClasses[props.tone]] }, [
            h(Icon, { name: props.icon, size: 'md', strokeWidth: 2 })
          ]),
          h('div', { class: 'min-w-0 flex-1' }, [
            h('p', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, props.label),
            h('p', { class: 'mt-1 truncate text-2xl font-semibold text-gray-900 dark:text-white' }, props.value),
            h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, props.hint)
          ])
        ])
      ])
  }
})

const BucketPanel = defineComponent({
  props: {
    title: { type: String, required: true },
    rows: { type: Array as () => AdminBalanceSummaryBucket[], required: true },
    labelForKey: { type: Function as unknown as () => (key: string) => string, required: true }
  },
  setup(props) {
    return () =>
      h('section', { class: 'card overflow-hidden' }, [
        h('div', { class: 'border-b border-gray-100 px-5 py-4 dark:border-dark-700' }, [
          h('h2', { class: 'text-lg font-semibold text-gray-900 dark:text-white' }, props.title)
        ]),
        h('div', { class: 'overflow-x-auto' }, [
          h('table', { class: 'w-full min-w-[520px] text-sm' }, [
            h('thead', { class: 'bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400' }, [
              h('tr', [
                h('th', { class: 'px-5 py-3 text-left' }, t('admin.balanceSummary.dimension')),
                h('th', { class: 'px-5 py-3 text-right' }, t('admin.balanceSummary.included')),
                h('th', { class: 'px-5 py-3 text-right' }, t('admin.balanceSummary.excluded')),
                h('th', { class: 'px-5 py-3 text-right' }, t('admin.balanceSummary.balance'))
              ])
            ]),
            h('tbody', { class: 'divide-y divide-gray-100 dark:divide-dark-700' },
              props.rows.map((row) =>
                h('tr', [
                  h('td', { class: 'px-5 py-3 font-medium text-gray-900 dark:text-white' }, props.labelForKey(row.key)),
                  h('td', { class: 'px-5 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300' }, formatNumber(row.user_count)),
                  h('td', { class: 'px-5 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300' }, formatNumber(row.excluded_count)),
                  h('td', { class: 'px-5 py-3 text-right tabular-nums font-medium text-gray-900 dark:text-white' }, formatMoney(row.balance))
                ])
              )
            )
          ])
        ])
      ])
  }
})

const { t } = useI18n()
const appStore = useAppStore()

const summary = ref<AdminBalanceSummaryResponse | null>(null)
const loading = ref(false)
const saving = ref(false)
const searching = ref(false)
const searchAttempted = ref(false)
const searchQuery = ref('')
const searchResults = ref<AdminUser[]>([])
const draftExclusions = ref<AdminBalanceSummaryExcludedUser[]>([])
let searchRequestSeq = 0

const savedExclusionIds = computed(() => summary.value?.excluded_user_ids || [])
const draftExclusionIds = computed(() => draftExclusions.value.map((user) => user.user_id))
const hasChanges = computed(() => {
  const saved = savedExclusionIds.value
  const draft = draftExclusionIds.value
  if (saved.length !== draft.length) return true
  return saved.some((id, index) => id !== draft[index])
})

const topMetrics = computed<MetricItem[]>(() => {
  const data = summary.value
  return [
    {
      label: t('admin.balanceSummary.totalBalance'),
      value: formatMoney(data?.total_balance || 0),
      hint: t('admin.balanceSummary.totalBalanceHint'),
      icon: 'dollar',
      tone: 'emerald'
    },
    {
      label: t('admin.balanceSummary.includedUsers'),
      value: formatNumber(data?.included_users || 0),
      hint: t('admin.balanceSummary.includedUsersHint'),
      icon: 'users',
      tone: 'blue'
    },
    {
      label: t('admin.balanceSummary.excludedUsers'),
      value: formatNumber(data?.excluded_user_count || 0),
      hint: t('admin.balanceSummary.excludedUsersHint'),
      icon: 'user',
      tone: 'amber'
    },
    {
      label: t('admin.balanceSummary.totalUsers'),
      value: formatNumber(data?.total_users || 0),
      hint: t('admin.balanceSummary.totalUsersHint'),
      icon: 'calculator',
      tone: 'slate'
    }
  ]
})

watch(summary, (value) => {
  draftExclusions.value = cloneExcludedUsers(value?.excluded_users || [])
}, { immediate: true })

function cloneExcludedUsers(users: AdminBalanceSummaryExcludedUser[]): AdminBalanceSummaryExcludedUser[] {
  return users.map((user) => ({ ...user }))
}

function normalizeDraft(): void {
  draftExclusions.value = [...draftExclusions.value].sort((a, b) => a.user_id - b.user_id)
}

function formatMoney(value: number): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function displayUser(user: Pick<AdminUser, 'username' | 'email' | 'id'>): string {
  return user.username?.trim() || user.email || `#${user.id}`
}

function displayExcludedUser(user: AdminBalanceSummaryExcludedUser): string {
  return user.username?.trim() || user.email || `#${user.user_id}`
}

function roleLabel(role: string): string {
  if (role === 'admin') return t('admin.balanceSummary.roleAdmin')
  if (role === 'user') return t('admin.balanceSummary.roleUser')
  return role || t('common.unknown')
}

function statusLabel(status: string): string {
  if (status === 'active') return t('admin.balanceSummary.statusActive')
  if (status === 'disabled') return t('admin.balanceSummary.statusDisabled')
  return status || t('common.unknown')
}

function statusClass(status?: string): string {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'disabled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
}

function invalidReasonLabel(reason?: string): string {
  if (reason === 'not_found_or_deleted') return t('admin.balanceSummary.notFoundOrDeleted')
  return t('admin.balanceSummary.invalidExclusion')
}

function isExcluded(userId: number): boolean {
  return draftExclusions.value.some((user) => user.user_id === userId)
}

function addExcludedUser(user: AdminUser): void {
  if (isExcluded(user.id)) return
  draftExclusions.value = [
    ...draftExclusions.value,
    {
      user_id: user.id,
      email: user.email,
      username: user.username,
      role: user.role,
      status: user.status,
      balance: user.balance,
      valid: true
    }
  ]
  normalizeDraft()
}

function removeExcludedUser(userId: number): void {
  draftExclusions.value = draftExclusions.value.filter((user) => user.user_id !== userId)
}

function clearDraftExclusions(): void {
  draftExclusions.value = []
}

async function loadSummary(options: { confirmDiscard?: boolean } = {}): Promise<void> {
  if (
    options.confirmDiscard &&
    hasChanges.value &&
    !window.confirm(t('admin.balanceSummary.discardUnsavedChangesConfirm'))
  ) {
    return
  }

  loading.value = true
  try {
    summary.value = await adminAPI.dashboard.getAdminBalanceSummary()
  } catch (error) {
    console.error('Failed to load balance summary:', error)
    summary.value = null
    appStore.showError(t('admin.balanceSummary.failedToLoad'))
  } finally {
    loading.value = false
  }
}

async function refreshSummary(): Promise<void> {
  await loadSummary({ confirmDiscard: true })
}

async function searchUsers(): Promise<void> {
  const query = searchQuery.value.trim()
  const requestSeq = ++searchRequestSeq
  searchAttempted.value = true
  if (!query) {
    searchResults.value = []
    searching.value = false
    return
  }

  searching.value = true
  try {
    const result = await adminAPI.users.list(1, 10, { search: query })
    if (requestSeq !== searchRequestSeq) return
    searchResults.value = result.items || []
  } catch (error) {
    if (requestSeq !== searchRequestSeq) return
    console.error('Failed to search users for balance summary:', error)
    searchResults.value = []
    appStore.showError(t('admin.balanceSummary.searchFailed'))
  } finally {
    if (requestSeq === searchRequestSeq) {
      searching.value = false
    }
  }
}

async function saveExclusions(): Promise<void> {
  saving.value = true
  try {
    summary.value = await adminAPI.dashboard.updateAdminBalanceSummaryExclusions(draftExclusionIds.value)
    appStore.showSuccess(t('admin.balanceSummary.saveSuccess'))
  } catch (error: any) {
    console.error('Failed to save balance summary exclusions:', error)
    appStore.showError(error.response?.data?.message || error.message || t('admin.balanceSummary.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadSummary()
})
</script>
