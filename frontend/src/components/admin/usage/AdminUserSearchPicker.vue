<template>
  <div class="space-y-2">
    <form
      class="flex min-w-0 flex-col gap-2 sm:flex-row"
      data-testid="admin-user-search-form"
      @submit.prevent="submitSearch"
    >
      <div class="relative min-w-0 flex-1">
        <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          <Icon name="search" size="sm" class="text-gray-400" />
        </div>
        <input
          :value="query"
          type="search"
          class="input w-full pl-9"
          :placeholder="placeholder"
          :disabled="loading"
          data-testid="admin-user-search-input"
          @input="handleInput"
        />
      </div>
      <button
        type="submit"
        class="btn btn-secondary inline-flex items-center justify-center gap-2 whitespace-nowrap"
        :disabled="loading || query.trim().length === 0"
        data-testid="admin-user-search-button"
      >
        <Icon name="search" size="sm" />
        <span>{{ loading ? loadingLabel : searchLabel }}</span>
      </button>
    </form>

    <div
      v-if="selectedUser"
      class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-primary-200 bg-primary-50 px-3 py-2 text-sm dark:border-primary-500/30 dark:bg-primary-500/10"
      data-testid="admin-user-selected"
    >
      <div class="flex min-w-0 items-center gap-2">
        <Icon name="user" size="sm" class="flex-shrink-0 text-primary-600 dark:text-primary-300" />
        <div class="min-w-0">
          <div class="truncate font-medium text-gray-900 dark:text-white">{{ selectedUser.email }}</div>
          <div class="text-xs text-gray-500 dark:text-dark-300">#{{ selectedUser.id }}</div>
        </div>
        <span
          v-if="selectedUser.deleted"
          class="rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-700 dark:bg-rose-500/20 dark:text-rose-300"
        >
          {{ deletedLabel }}
        </span>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="clearSelection">
        {{ clearLabel }}
      </button>
    </div>

    <div
      v-if="hasSubmittedSearch"
      class="rounded-lg border border-gray-200 bg-white text-sm shadow-sm dark:border-dark-700 dark:bg-dark-800"
      data-testid="admin-user-search-results"
    >
      <div class="border-b border-gray-100 px-3 py-2 text-xs font-medium text-gray-500 dark:border-dark-700 dark:text-dark-300">
        {{ resultsLabel }}
      </div>
      <div v-if="loading" class="px-3 py-3 text-gray-500 dark:text-dark-300">
        {{ loadingLabel }}
      </div>
      <div v-else-if="users.length === 0" class="px-3 py-3 text-gray-500 dark:text-dark-300">
        {{ emptyLabel }}
      </div>
      <div v-else class="max-h-64 overflow-y-auto py-1">
        <button
          v-for="user in users"
          :key="user.id"
          type="button"
          class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left transition-colors hover:bg-gray-50 focus:bg-gray-50 focus:outline-none dark:hover:bg-dark-700 dark:focus:bg-dark-700"
          :data-testid="`admin-user-result-${user.id}`"
          @click="selectUser(user)"
        >
          <span class="min-w-0">
            <span class="block truncate font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
            <span class="block text-xs text-gray-500 dark:text-dark-300">#{{ user.id }}</span>
          </span>
          <span
            v-if="user.deleted"
            class="flex-shrink-0 rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-700 dark:bg-rose-500/20 dark:text-rose-300"
          >
            {{ deletedLabel }}
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { SimpleUser } from '@/api/admin/usage'

defineProps<{
  users: SimpleUser[]
  selectedUser: SimpleUser | null
  loading: boolean
  placeholder: string
  searchLabel: string
  loadingLabel: string
  resultsLabel: string
  emptyLabel: string
  deletedLabel: string
  clearLabel: string
}>()

const emit = defineEmits<{
  (e: 'search', keyword: string): void
  (e: 'select', user: SimpleUser): void
  (e: 'clear'): void
}>()

const query = ref('')
const submittedQuery = ref('')
const hasSubmittedSearch = ref(false)

const handleInput = (event: Event) => {
  query.value = (event.target as HTMLInputElement).value
  if (query.value.trim() !== submittedQuery.value) {
    hasSubmittedSearch.value = false
  }
}

const submitSearch = () => {
  const keyword = query.value.trim()
  if (!keyword) return
  submittedQuery.value = keyword
  hasSubmittedSearch.value = true
  emit('search', keyword)
}

const selectUser = (user: SimpleUser) => {
  query.value = ''
  submittedQuery.value = ''
  hasSubmittedSearch.value = false
  emit('select', user)
}

const clearSelection = () => {
  query.value = ''
  submittedQuery.value = ''
  hasSubmittedSearch.value = false
  emit('clear')
}
</script>
