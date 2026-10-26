<template>
  <div ref="rootRef" class="relative">
    <!-- 触发框：已选账号标签 + 计数 + 下拉箭头 -->
    <div
      class="cursor-pointer rounded-lg border border-gray-300 bg-white px-3 py-2 dark:border-dark-500 dark:bg-dark-700"
      role="combobox"
      :aria-expanded="showDropdown"
      :aria-label="ariaLabel"
      @click="toggleDropdown"
    >
      <div v-if="selectedAccounts.length > 0" class="grid grid-cols-2 gap-1.5">
        <span
          v-for="account in selectedAccounts"
          :key="account.id"
          class="inline-flex items-center justify-between gap-1 rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
        >
          <span class="flex min-w-0 items-center gap-1">
            <PlatformIcon :platform="account.platform" size="xs" />
            <span class="truncate">#{{ account.id }} {{ account.name }}</span>
          </span>
          <button
            type="button"
            class="shrink-0 rounded-full hover:bg-gray-200 dark:hover:bg-dark-500"
            @click.stop="removeAccount(account.id)"
          >
            <Icon name="x" size="xs" class="h-3.5 w-3.5" :stroke-width="2" />
          </button>
        </span>
      </div>
      <span v-else class="text-sm text-gray-400">{{ placeholder }}</span>
      <div
        class="flex items-center justify-between"
        :class="selectedAccounts.length > 0 ? 'mt-2 border-t border-gray-200 pt-2 dark:border-dark-600' : 'mt-1'"
      >
        <span v-if="selectedAccounts.length > 0" class="text-xs text-gray-400">{{ selectedLabel }}</span>
        <span v-else class="text-xs text-gray-400">{{ countLabel }}</span>
        <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </div>
    </div>

    <!-- 下拉列表：搜索 + 勾选 -->
    <div
      v-if="showDropdown"
      class="absolute left-0 right-0 top-full z-50 mt-1 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700"
    >
      <div class="sticky top-0 border-b border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-700">
        <input
          v-model="searchQuery"
          type="text"
          class="input w-full text-sm"
          :placeholder="searchPlaceholder"
          @click.stop
        />
      </div>
      <div class="max-h-52 overflow-auto">
        <button
          v-for="account in filteredOptions"
          :key="account.id"
          type="button"
          data-testid="account-option"
          class="flex min-w-0 w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-600"
          @click="toggleAccount(account.id)"
        >
          <span
            :class="[
              'flex h-4 w-4 shrink-0 items-center justify-center rounded border',
              isSelected(account.id)
                ? 'border-primary-500 bg-primary-500 text-white'
                : 'border-gray-300 dark:border-dark-500'
            ]"
          >
            <svg v-if="isSelected(account.id)" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
            </svg>
          </span>
          <PlatformIcon :platform="account.platform" size="xs" />
          <span
            class="truncate"
            :class="account.status === 'active' ? 'text-gray-900 dark:text-white' : 'text-gray-400 dark:text-gray-500'"
          >#{{ account.id }} {{ account.name }}</span>
          <span v-if="account.status !== 'active'" class="ml-auto shrink-0 text-xs text-gray-400 dark:text-gray-500">
            {{ inactiveLabel }}
          </span>
        </button>
        <div v-if="filteredOptions.length === 0" class="px-3 py-6 text-center text-sm text-gray-400">
          {{ loading ? loadingText : emptyText }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

export interface AccountMultiSelectorOption {
  id: number
  name: string
  platform: string
  status: string
}

const props = defineProps<{
  modelValue: number[]
  options: AccountMultiSelectorOption[]
  loading?: boolean
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  loadingText?: string
  inactiveLabel?: string
  selectedLabel?: string
  countLabel?: string
  ariaLabel?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const rootRef = ref<HTMLElement | null>(null)
const showDropdown = ref(false)
const searchQuery = ref('')

const selectedAccounts = computed(() =>
  props.modelValue.map((id) => {
    // 配置里可能残留已删除账号的 ID：兜底渲染 chip，避免计数与展示不一致。
    return (
      props.options.find((option) => option.id === id) ?? {
        id,
        name: '',
        platform: '',
        status: 'active'
      }
    )
  })
)

const filteredOptions = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) {
    return props.options
  }
  return props.options.filter(
    (option) => option.name.toLowerCase().includes(query) || String(option.id).includes(query)
  )
})

function isSelected(id: number) {
  return props.modelValue.includes(id)
}

function toggleAccount(id: number) {
  const next = isSelected(id) ? props.modelValue.filter((v) => v !== id) : [...props.modelValue, id]
  emit('update:modelValue', next)
}

function removeAccount(id: number) {
  emit('update:modelValue', props.modelValue.filter((v) => v !== id))
}

function toggleDropdown() {
  showDropdown.value = !showDropdown.value
  if (showDropdown.value) {
    searchQuery.value = ''
  }
}

function onClickOutside(event: MouseEvent) {
  if (showDropdown.value && rootRef.value && !rootRef.value.contains(event.target as Node)) {
    showDropdown.value = false
  }
}

onMounted(() => document.addEventListener('click', onClickOutside))
onUnmounted(() => document.removeEventListener('click', onClickOutside))
</script>
