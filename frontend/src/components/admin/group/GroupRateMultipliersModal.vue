<template>
  <BaseDialog :show="show" :title="t('admin.groups.rateMultipliersTitle')" width="wide" @close="handleClose">
    <div v-if="group" class="space-y-4">
      <!-- 分组信息 -->
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5" :class="platformColorClass">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t('admin.groups.platforms.' + group.platform) }}
        </span>
        <span class="text-gray-400">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.columns.rateMultiplier') }}: {{ group.rate_multiplier }}x
        </span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.visibleRateMultiplier') }}: {{ group.visible_rate_multiplier ?? group.rate_multiplier }}x
        </span>
      </div>

      <!-- 操作区 -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <!-- 添加用户 -->
        <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.groups.addUserRate') }}
        </h4>
        <div class="flex items-end gap-2">
          <div class="relative flex-1">
            <input
              v-model="searchQuery"
              type="text"
              autocomplete="off"
              class="input w-full"
              :placeholder="t('admin.groups.searchUserPlaceholder')"
              @input="handleSearchUsers"
              @focus="showDropdown = true"
            />
            <div
              v-if="showDropdown && searchResults.length > 0"
              class="absolute left-0 right-0 top-full z-10 mt-1 max-h-48 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-500 dark:bg-dark-700"
            >
              <button
                v-for="user in searchResults"
                :key="user.id"
                type="button"
                class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-600"
                @click="selectUser(user)"
              >
                <span class="text-gray-400">#{{ user.id }}</span>
                <span class="text-gray-900 dark:text-white">{{ user.username || user.email }}</span>
                <span v-if="user.username" class="text-xs text-gray-400">{{ user.email }}</span>
              </button>
            </div>
          </div>
          <div class="w-24">
            <input
              v-model="newRateInput"
              type="number"
              step="0.01"
              min="0.01"
              autocomplete="off"
              class="hide-spinner input w-full"
              placeholder="1.00"
            />
          </div>
          <button
            type="button"
            class="btn btn-primary shrink-0"
            :disabled="!selectedUser || !isRateInputValid(newRateInput)"
            @click="handleAddLocal"
          >
            {{ t('common.add') }}
          </button>
        </div>

        <!-- 批量统一设置 + 全部清空 -->
        <div v-if="localEntries.length > 0" class="mt-3 flex items-center gap-3 border-t border-gray-100 pt-3 dark:border-dark-600">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.batchAdjust') }}</span>
          <span class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ t('admin.groups.selectedCount', { count: selectedEntryCount }) }}
          </span>
          <div class="flex items-center gap-1.5">
            <input
              v-model="bulkRateInput"
              type="number"
              step="0.01"
              min="0.01"
              autocomplete="off"
              class="hide-spinner w-20 rounded border border-gray-200 bg-white px-2 py-1 text-center text-sm transition-colors focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
              :placeholder="t('admin.groups.bulkRatePlaceholder')"
            />
            <button
              type="button"
              class="btn btn-primary btn-sm shrink-0 px-2.5 py-1 text-xs"
              :disabled="selectedEntryCount === 0 || !isRateInputValid(bulkRateInput)"
              @click="applyBulkRate"
            >
              {{ t('admin.groups.applyMultiplier') }}
            </button>
          </div>
          <div class="ml-auto">
            <button
              type="button"
              class="rounded-lg border border-red-200 bg-red-50 px-3 py-1.5 text-sm font-medium text-red-600 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/40"
              @click="clearAllLocal"
            >
              {{ t('admin.groups.clearAll') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-6">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <!-- 已设置的用户列表 -->
      <div v-else>
        <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.groups.rateMultipliers') }} ({{ localEntries.length }})
        </h4>

        <div v-if="localEntries.length === 0" class="py-6 text-center text-sm text-gray-400 dark:text-gray-500">
          {{ t('admin.groups.noRateMultipliers') }}
        </div>

        <div v-else>
          <!-- 表格 -->
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <div class="max-h-[420px] overflow-auto">
              <table class="w-full min-w-max text-sm">
                <thead class="sticky top-0 z-[1]">
                  <tr class="border-b border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700">
                    <th class="w-10 px-2 py-2 text-center">
                      <input
                        type="checkbox"
                        class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
                        :checked="isAllCurrentPageSelected"
                        :aria-label="t('admin.groups.selectCurrentPage')"
                        @change="toggleCurrentPageSelection(($event.target as HTMLInputElement).checked)"
                      />
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userEmail') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">ID</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userName') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userNotes') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userStatus') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.rateMultiplier') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.visibleRateMultiplier') }}</th>
                    <th class="w-10 px-2 py-2"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                  <tr
                    v-for="entry in paginatedLocalEntries"
                    :key="entry.user_id"
                    class="hover:bg-gray-50 dark:hover:bg-dark-700/50"
                  >
                    <td class="px-2 py-2 text-center">
                      <input
                        type="checkbox"
                        class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
                        :checked="selectedEntryIds.has(entry.user_id)"
                        :aria-label="t('admin.groups.selectRateEntry')"
                        @change="toggleEntrySelection(entry.user_id, ($event.target as HTMLInputElement).checked)"
                      />
                    </td>
                    <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{{ entry.user_email }}</td>
                    <td class="whitespace-nowrap px-3 py-2 text-gray-400 dark:text-gray-500">{{ entry.user_id }}</td>
                    <td class="whitespace-nowrap px-3 py-2 text-gray-900 dark:text-white">{{ entry.user_name || '-' }}</td>
                    <td class="max-w-[160px] truncate px-3 py-2 text-gray-500 dark:text-gray-400" :title="entry.user_notes">{{ entry.user_notes || '-' }}</td>
                    <td class="whitespace-nowrap px-3 py-2">
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                          entry.user_status === 'active'
                            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                        ]"
                      >
                        {{ entry.user_status }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-3 py-2">
                      <input
                        type="number"
                        step="0.01"
                        min="0.01"
                        autocomplete="off"
                        :value="entry.rate_multiplier ?? ''"
                        :placeholder="String(props.group?.rate_multiplier ?? 1)"
                        class="hide-spinner w-20 rounded border border-gray-200 bg-white px-2 py-1 text-center text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                        @change="updateLocalRate(entry.user_id, ($event.target as HTMLInputElement).value)"
                      />
                    </td>
                    <td class="whitespace-nowrap px-3 py-2">
                      <input
                        type="number"
                        step="0.01"
                        min="0.01"
                        autocomplete="off"
                        :value="entry.visible_rate_multiplier ?? ''"
                        :placeholder="String(effectiveVisibleRatePlaceholder(entry))"
                        class="hide-spinner w-20 rounded border border-gray-200 bg-white px-2 py-1 text-center text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                        @change="updateLocalVisibleRate(entry.user_id, ($event.target as HTMLInputElement).value)"
                      />
                    </td>
                    <td class="px-2 py-2">
                      <button
                        type="button"
                        class="rounded p-1 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                        @click="removeLocal(entry.user_id)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- 分页 -->
          <Pagination
            :total="localEntries.length"
            :page="currentPage"
            :page-size="pageSize"
            @update:page="currentPage = $event"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </div>

      <!-- 底部操作栏 -->
      <div class="flex items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-600">
        <!-- 左侧：未保存提示 + 撤销 -->
        <template v-if="isDirty">
          <span class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.groups.unsavedChanges') }}</span>
          <button
            type="button"
            class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            @click="handleCancel"
          >
            {{ t('admin.groups.revertChanges') }}
          </button>
        </template>
        <!-- 右侧：关闭 / 保存 -->
        <div class="ml-auto flex items-center gap-3">
          <button type="button" class="btn btn-sm px-4 py-1.5" @click="handleClose">
            {{ t('common.close') }}
          </button>
          <button
            v-if="isDirty"
            type="button"
            class="btn btn-primary btn-sm px-4 py-1.5"
            :disabled="saving"
            @click="handleSave"
          >
            <Icon v-if="saving" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </BaseDialog>

</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { GroupRateMultiplierEntry } from '@/api/admin/groups'
import type { AdminGroup, AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

interface LocalEntry extends GroupRateMultiplierEntry {}

type GroupRateMultiplierPayloadEntry = {
  user_id: number
  rate_multiplier?: number | null
  visible_rate_multiplier?: number | null
}

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const serverEntries = ref<GroupRateMultiplierEntry[]>([])
const localEntries = ref<LocalEntry[]>([])
const searchQuery = ref('')
const searchResults = ref<AdminUser[]>([])
const showDropdown = ref(false)
const selectedUser = ref<AdminUser | null>(null)
const newRateInput = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const bulkRateInput = ref('')
const selectedEntryIds = ref<Set<number>>(new Set())
const changedRateEntryIds = ref<Set<number>>(new Set())
const changedVisibleRateEntryIds = ref<Set<number>>(new Set())

let searchTimeout: ReturnType<typeof setTimeout>

const platformColorClass = computed(() => {
  switch (props.group?.platform) {
    case 'anthropic': return 'text-orange-700 dark:text-orange-400'
    case 'openai': return 'text-emerald-700 dark:text-emerald-400'
    case 'antigravity': return 'text-purple-700 dark:text-purple-400'
    default: return 'text-blue-700 dark:text-blue-400'
  }
})

// 检测是否有未保存的修改
const isDirty = computed(() => {
  if (localEntries.value.length !== serverEntries.value.length) return true
  const serverMap = new Map(serverEntries.value.map(e => [
    e.user_id,
    {
      rate: e.rate_multiplier ?? null,
      visibleRate: e.visible_rate_multiplier ?? null
    }
  ]))
  return localEntries.value.some(e => {
    const server = serverMap.get(e.user_id)
    return !server ||
      server.rate !== (e.rate_multiplier ?? null) ||
      server.visibleRate !== (e.visible_rate_multiplier ?? null)
  })
})

const paginatedLocalEntries = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return localEntries.value.slice(start, start + pageSize.value)
})

const selectedEntryCount = computed(() => selectedEntryIds.value.size)

const isAllCurrentPageSelected = computed(() => {
  return paginatedLocalEntries.value.length > 0 &&
    paginatedLocalEntries.value.every(entry => selectedEntryIds.value.has(entry.user_id))
})

const buildSaveEntries = (): GroupRateMultiplierPayloadEntry[] => {
  const localIDs = new Set(localEntries.value.map(entry => entry.user_id))
  const localByID = new Map(localEntries.value.map(entry => [entry.user_id, entry]))
  const serverIDs = new Set(serverEntries.value.map(entry => entry.user_id))
  const userIDs = new Set([...serverIDs, ...localIDs])
  const entries: GroupRateMultiplierPayloadEntry[] = []

  for (const userID of userIDs) {
    const local = localByID.get(userID)
    const payload: GroupRateMultiplierPayloadEntry = { user_id: userID }
    if (!local) {
      payload.rate_multiplier = null
      payload.visible_rate_multiplier = null
    } else {
      if (!serverIDs.has(userID) || changedRateEntryIds.value.has(userID)) {
        payload.rate_multiplier = local.rate_multiplier ?? null
      }
      if (!serverIDs.has(userID) || changedVisibleRateEntryIds.value.has(userID)) {
        payload.visible_rate_multiplier = local.visible_rate_multiplier ?? null
      }
    }
    entries.push(payload)
  }

  return entries
}

const parseRateInput = (value: string | number | null | undefined): number | null => {
  const trimmed = String(value ?? '').trim()
  if (!/^\d+(?:\.\d{1,2})?$/.test(trimmed)) return null
  const rate = Number(trimmed)
  if (!Number.isFinite(rate) || rate <= 0) return null
  return rate
}

const isRateInputValid = (value: string | number | null | undefined) => parseRateInput(value) != null

const cloneEntries = (entries: GroupRateMultiplierEntry[]): LocalEntry[] => {
  return entries.map(e => ({ ...e }))
}

const effectiveVisibleRatePlaceholder = (entry: LocalEntry) => {
  return entry.rate_multiplier ?? props.group?.visible_rate_multiplier ?? props.group?.rate_multiplier ?? 1
}

const resetSelection = () => {
  selectedEntryIds.value = new Set()
}

const resetChangedRateEntries = () => {
  changedRateEntryIds.value = new Set()
  changedVisibleRateEntryIds.value = new Set()
}

const markRateEntryChanged = (userId: number) => {
  const next = new Set(changedRateEntryIds.value)
  next.add(userId)
  changedRateEntryIds.value = next
}

const markVisibleRateEntryChanged = (userId: number) => {
  const next = new Set(changedVisibleRateEntryIds.value)
  next.add(userId)
  changedVisibleRateEntryIds.value = next
}

const pruneSelection = () => {
  const existingIDs = new Set(localEntries.value.map(entry => entry.user_id))
  selectedEntryIds.value = new Set([...selectedEntryIds.value].filter(id => existingIDs.has(id)))
}

const loadEntries = async () => {
  if (!props.group) return
  loading.value = true
  try {
    const raw = await adminAPI.groups.getGroupRateMultipliers(props.group.id)
    // 仅显示已设置真实或可见倍率的条目；rpm_override 在另一个弹窗管理，保留不动
    serverEntries.value = raw.filter(e => e.rate_multiplier != null || e.visible_rate_multiplier != null)
    localEntries.value = cloneEntries(serverEntries.value)
    resetSelection()
    resetChangedRateEntries()
    adjustPage()
  } catch (error) {
    appStore.showError(t('admin.groups.failedToLoad'))
    console.error('Error loading group rate multipliers:', error)
  } finally {
    loading.value = false
  }
}

const adjustPage = () => {
  const totalPages = Math.max(1, Math.ceil(localEntries.value.length / pageSize.value))
  if (currentPage.value > totalPages) {
    currentPage.value = totalPages
  }
}

watch(() => props.show, (val) => {
  if (val && props.group) {
    currentPage.value = 1
    searchQuery.value = ''
    searchResults.value = []
    selectedUser.value = null
    newRateInput.value = ''
    bulkRateInput.value = ''
    resetSelection()
    resetChangedRateEntries()
    loadEntries()
  }
})

const handlePageSizeChange = (newSize: number) => {
  pageSize.value = newSize
  currentPage.value = 1
}

const handleSearchUsers = () => {
  clearTimeout(searchTimeout)
  selectedUser.value = null
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    showDropdown.value = false
    return
  }
  searchTimeout = setTimeout(async () => {
    try {
      const res = await adminAPI.users.list(1, 10, { search: searchQuery.value.trim() })
      searchResults.value = res.items
      showDropdown.value = true
    } catch {
      searchResults.value = []
    }
  }, 300)
}

const selectUser = (user: AdminUser) => {
  selectedUser.value = user
  searchQuery.value = user.email
  showDropdown.value = false
  searchResults.value = []
}

// 本地添加（或覆盖已有用户）
const handleAddLocal = () => {
  const parsedRate = parseRateInput(newRateInput.value)
  if (!selectedUser.value || parsedRate == null) {
    appStore.showError(t('admin.groups.invalidRateMultiplier'))
    return
  }
  const user = selectedUser.value
  const idx = localEntries.value.findIndex(e => e.user_id === user.id)
  const entry: LocalEntry = {
    user_id: user.id,
    user_name: user.username || '',
    user_email: user.email,
    user_notes: user.notes || '',
    user_status: user.status || 'active',
    rate_multiplier: parsedRate,
    visible_rate_multiplier: null,
    rpm_override: null
  }
  if (idx >= 0) {
    localEntries.value[idx] = entry
  } else {
    localEntries.value.push(entry)
  }
  markRateEntryChanged(user.id)
  searchQuery.value = ''
  selectedUser.value = null
  newRateInput.value = ''
  adjustPage()
}

// 本地修改倍率
const updateLocalRate = (userId: number, value: string) => {
  const entry = localEntries.value.find(e => e.user_id === userId)
  if (!entry) return
  if (value.trim() === '') {
    entry.rate_multiplier = null
    markRateEntryChanged(userId)
    return
  }
  const parsedRate = parseRateInput(value)
  if (parsedRate == null) {
    appStore.showError(t('admin.groups.invalidRateMultiplier'))
    return
  }
  entry.rate_multiplier = parsedRate
  markRateEntryChanged(userId)
}

// 本地修改用户可见倍率；留空表示按用户专属真实倍率、分组可见倍率、分组真实倍率依次兜底。
const updateLocalVisibleRate = (userId: number, value: string) => {
  const entry = localEntries.value.find(e => e.user_id === userId)
  if (!entry) return
  if (value.trim() === '') {
    entry.visible_rate_multiplier = null
    markVisibleRateEntryChanged(userId)
    return
  }
  const parsedRate = parseRateInput(value)
  if (parsedRate == null) {
    appStore.showError(t('admin.groups.invalidRateMultiplier'))
    return
  }
  entry.visible_rate_multiplier = parsedRate
  markVisibleRateEntryChanged(userId)
}

// 本地删除
const removeLocal = (userId: number) => {
  localEntries.value = localEntries.value.filter(e => e.user_id !== userId)
  pruneSelection()
  adjustPage()
}

const toggleEntrySelection = (userId: number, checked: boolean) => {
  const next = new Set(selectedEntryIds.value)
  if (checked) {
    next.add(userId)
  } else {
    next.delete(userId)
  }
  selectedEntryIds.value = next
}

const toggleCurrentPageSelection = (checked: boolean) => {
  const next = new Set(selectedEntryIds.value)
  for (const entry of paginatedLocalEntries.value) {
    if (checked) {
      next.add(entry.user_id)
    } else {
      next.delete(entry.user_id)
    }
  }
  selectedEntryIds.value = next
}

// 批量统一设置选中条目的倍率
const applyBulkRate = () => {
  if (selectedEntryCount.value === 0) {
    appStore.showError(t('admin.groups.selectRateEntriesFirst'))
    return
  }
  const parsedRate = parseRateInput(bulkRateInput.value)
  if (parsedRate == null) {
    appStore.showError(t('admin.groups.invalidRateMultiplier'))
    return
  }
  for (const entry of localEntries.value) {
    if (selectedEntryIds.value.has(entry.user_id)) {
      entry.rate_multiplier = parsedRate
      markRateEntryChanged(entry.user_id)
    }
  }
  bulkRateInput.value = ''
  resetSelection()
  appStore.showSuccess(t('admin.groups.rateAdjusted'))
}

// 本地清空
const clearAllLocal = () => {
  localEntries.value = []
  resetSelection()
  resetChangedRateEntries()
}

// 取消：恢复到服务器数据
const handleCancel = () => {
  localEntries.value = cloneEntries(serverEntries.value)
  bulkRateInput.value = ''
  resetSelection()
  resetChangedRateEntries()
  adjustPage()
}

// 保存：一次性提交真实/可见倍率；rpm_override 由独立弹窗管理
const handleSave = async () => {
  if (!props.group) return
  saving.value = true
  try {
    const entries = buildSaveEntries()
    if (entries.some(e => changedRateEntryIds.value.has(e.user_id) && e.rate_multiplier != null && !isRateInputValid(String(e.rate_multiplier)))) {
      appStore.showError(t('admin.groups.invalidRateMultiplier'))
      return
    }
    if (entries.some(e => changedVisibleRateEntryIds.value.has(e.user_id) && e.visible_rate_multiplier != null && !isRateInputValid(String(e.visible_rate_multiplier)))) {
      appStore.showError(t('admin.groups.invalidRateMultiplier'))
      return
    }
    await adminAPI.groups.batchSetGroupRateMultipliers(props.group.id, entries)
    appStore.showSuccess(t('admin.groups.rateSaved'))
    emit('success')
    emit('close')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error saving rate multipliers:', error)
  } finally {
    saving.value = false
  }
}

// 关闭时如果有未保存修改，先恢复
const handleClose = () => {
  if (isDirty.value) {
    localEntries.value = cloneEntries(serverEntries.value)
  }
  bulkRateInput.value = ''
  resetSelection()
  resetChangedRateEntries()
  emit('close')
}

// 点击外部关闭下拉
const handleClickOutside = () => {
  showDropdown.value = false
}

if (typeof document !== 'undefined') {
  document.addEventListener('click', handleClickOutside)
}
</script>

<style scoped>
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>
