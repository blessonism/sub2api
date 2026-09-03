<template>
  <BaseDialog :show="show" :title="t('admin.users.groupConfig')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-6">
      <!-- 用户信息头部 -->
      <div class="flex items-center gap-4 rounded-2xl bg-gradient-to-r from-primary-50 to-primary-100 p-5 dark:from-primary-900/30 dark:to-primary-800/20">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.users.groupConfigHint', { email: user.email }) }}</p>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-12">
        <svg class="h-10 w-10 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div v-else class="space-y-6">
        <!-- 专属分组区域 -->
        <div v-if="exclusiveGroups.length > 0">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-purple-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.users.exclusiveGroups') }}</h4>
            <span class="text-xs text-gray-400">({{ exclusiveGroupConfigs.filter(c => c.isSelected).length }}/{{ exclusiveGroupConfigs.length }})</span>
          </div>
          <div class="grid gap-3">
            <div
              v-for="config in exclusiveGroupConfigs"
              :key="config.groupId"
              class="group relative overflow-hidden rounded-xl border-2 p-4 transition-all duration-200"
              :class="config.isSelected
                ? 'border-primary-400 bg-primary-50/50 shadow-sm dark:border-primary-500 dark:bg-primary-900/20'
                : 'border-gray-200 bg-white hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500'"
            >
              <div class="flex items-center gap-4">
                <!-- 复选框 -->
                <div class="flex-shrink-0">
                  <label class="relative flex h-6 w-6 cursor-pointer items-center justify-center">
                    <input
                      type="checkbox"
                      :checked="config.isSelected"
                      @change="toggleExclusiveGroup(config.groupId)"
                      class="peer sr-only"
                    />
                    <div class="h-5 w-5 rounded-md border-2 border-gray-300 transition-all peer-checked:border-primary-500 peer-checked:bg-primary-500 dark:border-dark-500 peer-checked:dark:border-primary-500">
                      <svg v-if="config.isSelected" class="h-full w-full text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    </div>
                  </label>
                </div>

                <!-- 分组信息 -->
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                    <span class="inline-flex items-center rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-900/40 dark:text-purple-300">
                      {{ t('admin.groups.exclusive') }}
                    </span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      <PlatformIcon :platform="config.platform" size="xs" />
                      <span>{{ config.platform }}</span>
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <!-- 专属倍率输入 -->
                <div class="grid flex-shrink-0 grid-cols-2 gap-3">
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.users.customRate') }}
                    <input
                      type="number"
                      step="0.001"
                      min="0.001"
                      :value="config.customRate ?? ''"
                      @input="updateCustomRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultRate)"
                      class="hide-spinner mt-1 w-24 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </label>
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.users.visibleRate') }}
                    <input
                      type="number"
                      step="0.001"
                      min="0.001"
                      :value="config.visibleRate ?? ''"
                      @input="updateVisibleRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultVisibleRate)"
                      class="hide-spinner mt-1 w-24 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </label>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 公开分组区域 -->
        <div v-if="publicGroups.length > 0">
          <div class="mb-3 flex items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-green-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">
              {{ restrictPublicGroups ? t('admin.users.publicGroupsRestricted') : t('admin.users.publicGroups') }}
            </h4>
            <span class="text-xs text-gray-400">({{ publicGroupConfigs.length }})</span>
            <label class="ml-auto flex cursor-pointer items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
              <input type="checkbox" :checked="restrictPublicGroups" @change="toggleRestrictPublicGroups" class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              {{ t('admin.users.restrictPublicGroups') }}
            </label>
          </div>
          <div class="grid gap-3">
            <div
              v-for="config in publicGroupConfigs"
              :key="config.groupId"
              class="relative overflow-hidden rounded-xl border-2 border-green-200 bg-green-50/50 p-4 dark:border-green-800/50 dark:bg-green-900/10"
            >
              <div class="flex items-center gap-4">
                <!-- 未限制时公开分组恒可用，开启限制后按 allowed_groups 选择。 -->
                <div class="flex-shrink-0">
                  <input
                    v-if="restrictPublicGroups"
                    type="checkbox"
                    :checked="config.isSelected"
                    @change="togglePublicGroup(config.groupId)"
                    class="h-5 w-5 cursor-pointer rounded-md border-2 border-green-400 text-green-600 focus:ring-green-500 dark:border-green-600"
                  />
                  <div v-else class="flex h-5 w-5 items-center justify-center rounded-md border-2 border-green-400 bg-green-500 dark:border-green-600 dark:bg-green-600">
                    <svg class="h-full w-full text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                </div>

                <!-- 分组信息 -->
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      <PlatformIcon :platform="config.platform" size="xs" />
                      <span>{{ config.platform }}</span>
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <!-- 专属倍率输入 -->
                <div class="grid flex-shrink-0 grid-cols-2 gap-3">
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.users.customRate') }}
                    <input
                      type="number"
                      step="0.001"
                      min="0.001"
                      :value="config.customRate ?? ''"
                      @input="updateCustomRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultRate)"
                      class="hide-spinner mt-1 w-24 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </label>
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.users.visibleRate') }}
                    <input
                      type="number"
                      step="0.001"
                      min="0.001"
                      :value="config.visibleRate ?? ''"
                      @input="updateVisibleRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultVisibleRate)"
                      class="hide-spinner mt-1 w-24 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </label>
                </div>
              </div>

              <div class="mt-4 border-t border-green-200 pt-4 dark:border-green-800/50">
                <label class="flex cursor-pointer items-center justify-between gap-4">
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.users.limitAccounts') }}</span>
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="config.bindingEnabled"
                    @change="toggleAccountBinding(config)"
                  />
                </label>

                <div v-if="config.bindingEnabled" class="mt-4 space-y-4">
                  <div class="flex flex-wrap gap-x-6 gap-y-2">
                    <label class="flex cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                      <input v-model="config.fallbackToGroup" type="radio" :value="false" class="text-primary-600 focus:ring-primary-500" />
                      {{ t('admin.users.accountBindingStrict') }}
                    </label>
                    <label class="flex cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                      <input v-model="config.fallbackToGroup" type="radio" :value="true" class="text-primary-600 focus:ring-primary-500" />
                      {{ t('admin.users.accountBindingFallback') }}
                    </label>
                  </div>

                  <div v-if="config.accountsLoading" class="flex justify-center py-4">
                    <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
                  </div>
                  <div v-else-if="config.accounts.length > 0" class="grid max-h-44 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
                    <label
                      v-for="account in config.accounts"
                      :key="account.id"
                      class="flex cursor-pointer items-center gap-2 border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800"
                    >
                      <input
                        type="checkbox"
                        class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                        :checked="config.bindingAccountIds.includes(account.id)"
                        @change="toggleBindingAccount(config, account.id)"
                      />
                      <span class="min-w-0 flex-1 truncate text-gray-800 dark:text-gray-200">{{ account.name }}</span>
                      <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ account.status }}</span>
                    </label>
                  </div>
                  <p v-else class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.noGroupAccounts') }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 无分组提示 -->
        <div v-if="groups.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
          <div class="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
            <svg class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <p class="text-gray-500 dark:text-gray-400">{{ t('common.noGroupsAvailable') }}</p>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary px-5">{{ t('common.cancel') }}</button>
        <button @click="handleSave" :disabled="submitting" class="btn btn-primary px-6">
          <svg v-if="submitting" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, AdminUser, Group, GroupPlatform, UserGroupAccountBinding } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

interface GroupRateConfig {
  groupId: number
  groupName: string
  platform: GroupPlatform
  isExclusive: boolean
  defaultRate: number
  defaultVisibleRate: number
  customRate: number | null
  visibleRate: number | null
  isSelected: boolean
  bindingEnabled: boolean
  bindingAccountIds: number[]
  fallbackToGroup: boolean
  accounts: Account[]
  accountsLoading: boolean
  accountsLoaded: boolean
}

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<Group[]>([])
const groupConfigs = ref<GroupRateConfig[]>([])
const originalGroupRates = ref<Record<number, number>>({}) // 记录原始专属倍率，用于检测删除
const originalVisibleGroupRates = ref<Record<number, number>>({})
const restrictPublicGroups = ref(false)
const loading = ref(false)
const submitting = ref(false)

// 分离专属分组和公开分组
const exclusiveGroups = computed(() => groups.value.filter((g) => g.is_exclusive))
const publicGroups = computed(() => groups.value.filter((g) => !g.is_exclusive))

const exclusiveGroupConfigs = computed(() => groupConfigs.value.filter((c) => c.isExclusive))
const publicGroupConfigs = computed(() => groupConfigs.value.filter((c) => !c.isExclusive))

watch(
  () => props.show,
  (v) => {
    if (v && props.user) {
      load()
    }
  }
)

const load = async () => {
  loading.value = true
  try {
    const [res, userDetail] = await Promise.all([
      adminAPI.groups.list(1, 1000),
      adminAPI.users.getById(props.user!.id),
    ])
    // 只显示标准类型且活跃的分组
    groups.value = res.items.filter((g) => g.subscription_type === 'standard' && g.status === 'active')

    // 初始化配置
    const userAllowedGroups = userDetail.allowed_groups || []
    const userGroupRates = userDetail.group_rates || {}
    const userVisibleGroupRates = userDetail.visible_group_rates || {}
    const userGroupAccountBindings = userDetail.group_account_bindings || {}
    restrictPublicGroups.value = userDetail.restrict_public_groups ?? props.user?.restrict_public_groups ?? false

    // 保存原始专属倍率，用于检测删除操作
    originalGroupRates.value = { ...userGroupRates }
    originalVisibleGroupRates.value = { ...userVisibleGroupRates }

    groupConfigs.value = groups.value.map((g) => {
      const binding = userGroupAccountBindings[g.id]
      return {
        groupId: g.id,
        groupName: g.name,
        platform: g.platform,
        isExclusive: g.is_exclusive,
        defaultRate: g.rate_multiplier,
        defaultVisibleRate: g.visible_rate_multiplier ?? g.rate_multiplier,
        customRate: userGroupRates[g.id] ?? null,
        visibleRate: userVisibleGroupRates[g.id] ?? null,
        // 专属分组：检查是否在 allowed_groups 中；公开分组始终选中。
        isSelected: g.is_exclusive || restrictPublicGroups.value ? userAllowedGroups.includes(g.id) : true,
        bindingEnabled: !g.is_exclusive && binding !== undefined,
        bindingAccountIds: binding?.account_ids ? [...binding.account_ids] : [],
        fallbackToGroup: binding?.fallback_to_group ?? false,
        accounts: [],
        accountsLoading: false,
        accountsLoaded: false,
      }
    })

    await Promise.all(publicGroupConfigs.value.filter((config) => config.bindingEnabled).map(loadGroupAccounts))
  } catch (error) {
    console.error('Failed to load groups:', error)
  } finally {
    loading.value = false
  }
}

const loadGroupAccounts = async (config: GroupRateConfig) => {
  if (config.accountsLoaded || config.accountsLoading) return
  config.accountsLoading = true
  try {
    const accounts: Account[] = []
    let page = 1
    let pages = 1
    do {
      const res = await adminAPI.accounts.list(page, 200, { group: String(config.groupId), lite: 'true' })
      accounts.push(...res.items)
      pages = res.pages
      page += 1
    } while (page <= pages)
    config.accounts = accounts
    const availableAccountIds = new Set(accounts.map((account) => account.id))
    config.bindingAccountIds = config.bindingAccountIds.filter((accountId) => availableAccountIds.has(accountId))
    config.accountsLoaded = true
  } finally {
    config.accountsLoading = false
  }
}

const toggleAccountBinding = async (config: GroupRateConfig) => {
  config.bindingEnabled = !config.bindingEnabled
  if (config.bindingEnabled) {
    try {
      await loadGroupAccounts(config)
    } catch (error) {
      config.bindingEnabled = false
      appStore.showError(t('admin.users.failedToLoadGroupAccounts'))
      console.error('Failed to load group accounts:', error)
    }
  }
}

const toggleBindingAccount = (config: GroupRateConfig, accountId: number) => {
  const index = config.bindingAccountIds.indexOf(accountId)
  if (index >= 0) {
    config.bindingAccountIds.splice(index, 1)
  } else {
    config.bindingAccountIds.push(accountId)
  }
}

const toggleExclusiveGroup = (groupId: number) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config && config.isExclusive) {
    config.isSelected = !config.isSelected
  }
}

const togglePublicGroup = (groupId: number) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config && !config.isExclusive) config.isSelected = !config.isSelected
}

const toggleRestrictPublicGroups = () => {
  restrictPublicGroups.value = !restrictPublicGroups.value
  if (!restrictPublicGroups.value) {
    for (const config of publicGroupConfigs.value) config.isSelected = true
  }
}

const parseOptionalRate = (value: string): number | null => {
  if (value === '' || value === null || value === undefined) {
    return null
  }
  const numValue = Number(value)
  return Number.isFinite(numValue) && numValue > 0 ? numValue : null
}

const updateCustomRate = (groupId: number, value: string) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config) {
    config.customRate = parseOptionalRate(value)
  }
}

const updateVisibleRate = (groupId: number, value: string) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config) {
    config.visibleRate = parseOptionalRate(value)
  }
}

const handleSave = async () => {
  if (!props.user) return

  const invalidBinding = publicGroupConfigs.value.find((config) => config.bindingEnabled && config.bindingAccountIds.length === 0)
  if (invalidBinding) {
    appStore.showError(t('admin.users.accountBindingRequired', { group: invalidBinding.groupName }))
    return
  }
  submitting.value = true

  try {
    // 构建 allowed_groups（仅包含专属分组中被勾选的）
    const allowedGroups = groupConfigs.value.filter((c) => c.isSelected && (c.isExclusive || restrictPublicGroups.value)).map((c) => c.groupId)

    // 构建 group_rates
    // - 有新专属倍率: 设置为该值
    // - 原本有专属倍率但现在被清空: 设置为 null（表示删除）
    const groupRates: Record<number, number | null> = {}
    const visibleGroupRates: Record<number, number | null> = {}
    const groupAccountBindings: Record<number, UserGroupAccountBinding> = {}
    for (const c of groupConfigs.value) {
      const hadOriginalRate = originalGroupRates.value[c.groupId] !== undefined
      const hadOriginalVisibleRate = originalVisibleGroupRates.value[c.groupId] !== undefined

      if (c.customRate !== null) {
        // 有专属倍率
        groupRates[c.groupId] = c.customRate
      } else if (hadOriginalRate) {
        // 原本有专属倍率，现在被清空，需要显式删除
        groupRates[c.groupId] = null
      }

      if (c.visibleRate !== null) {
        visibleGroupRates[c.groupId] = c.visibleRate
      } else if (hadOriginalVisibleRate) {
        visibleGroupRates[c.groupId] = null
      }
    }

    for (const config of publicGroupConfigs.value) {
      if (config.bindingEnabled) {
        groupAccountBindings[config.groupId] = {
          account_ids: [...config.bindingAccountIds],
          fallback_to_group: config.fallbackToGroup,
        }
      }
    }

    await adminAPI.users.update(props.user.id, {
      allowed_groups: allowedGroups,
      restrict_public_groups: restrictPublicGroups.value,
      group_rates: Object.keys(groupRates).length > 0 ? groupRates : undefined,
      visible_group_rates: Object.keys(visibleGroupRates).length > 0 ? visibleGroupRates : undefined,
      group_account_bindings: groupAccountBindings,
    })

    appStore.showSuccess(t('admin.users.groupConfigUpdated'))
    emit('success')
    emit('close')
  } catch (error) {
    console.error('Failed to update user group config:', error)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
/* 隐藏数字输入框的箭头按钮 */
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>
