<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6" data-testid="leaderboard-view">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('leaderboard.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ displayRangeText }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-700 dark:bg-dark-800">
            <button
              v-for="option in periodOptions"
              :key="option.value"
              type="button"
              :class="[
                'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                leaderboardPeriod === option.value
                  ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300'
                  : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'
              ]"
              @click="setLeaderboardPeriod(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadLeaderboard">
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <section class="card p-5 lg:col-span-2">
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                {{ t('leaderboard.myRank') }}
              </p>
              <p class="mt-3 text-4xl font-semibold text-gray-950 dark:text-white">
                {{ rankLabel(myRank.rank) }}
              </p>
            </div>
            <span
              class="max-w-[14rem] break-all rounded-md border border-primary-100 bg-primary-50 px-3 py-1 text-right text-sm font-medium text-primary-700 dark:border-primary-900/50 dark:bg-primary-950/40 dark:text-primary-300"
            >
              {{ myRank.masked_email }}
            </span>
          </div>
          <div class="mt-6 grid grid-cols-2 gap-4">
            <div class="rounded-md bg-gray-50 p-4 dark:bg-dark-800">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('leaderboard.tokens') }}
              </p>
              <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">
                {{ formatTokenMillions(myRank.tokens) }}
              </p>
            </div>
            <div class="rounded-md bg-gray-50 p-4 dark:bg-dark-800">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('leaderboard.requests') }}
              </p>
              <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">
                {{ formatFullNumber(myRank.requests) }}
              </p>
            </div>
          </div>
        </section>

        <section class="card p-5">
          <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
            {{ t('leaderboard.topUsers', { limit: leaderboard?.limit || 10 }) }}
          </p>
          <p class="mt-3 text-4xl font-semibold text-gray-950 dark:text-white">
            {{ formatFullNumber(ranking.length) }}
          </p>
          <p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
            {{ t('leaderboard.topTokens') }}
          </p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">
            {{ formatTokenMillions(topTokenTotal) }}
          </p>
        </section>
      </div>

      <section class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('leaderboard.leaderboard') }}
          </h2>
          <span class="text-sm text-gray-500 dark:text-gray-400">
            {{ currentPeriodLabel }}
          </span>
        </div>

        <div v-if="loading" class="flex min-h-80 items-center justify-center">
          <LoadingSpinner size="lg" />
        </div>
        <div v-else-if="loadError" class="flex min-h-80 flex-col items-center justify-center gap-4 px-5 text-center">
          <p class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('leaderboard.failedToLoad') }}
          </p>
          <button class="btn btn-secondary" @click="loadLeaderboard">
            {{ t('leaderboard.retry') }}
          </button>
        </div>
        <div v-else-if="ranking.length === 0" class="min-h-80 px-5 py-12">
          <EmptyState
            :title="emptyStateTitle"
            :description="emptyStateDescription"
          />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[640px] text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="w-24 px-5 py-3 text-left">{{ t('leaderboard.rank') }}</th>
                <th class="px-5 py-3 text-left">{{ t('leaderboard.user') }}</th>
                <th class="px-5 py-3 text-right">{{ t('leaderboard.tokens') }}</th>
                <th class="px-5 py-3 text-right">{{ t('leaderboard.requests') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr
                v-for="item in ranking"
                :key="`${item.rank}-${item.masked_email}`"
                :class="item.is_current_user ? 'bg-primary-50/70 dark:bg-primary-950/20' : ''"
              >
                <td class="px-5 py-4">
                  <span :class="rankBadgeClass(item.rank)">
                    {{ rankLabel(item.rank) }}
                  </span>
                </td>
                <td class="px-5 py-4 font-medium text-gray-900 dark:text-white">
                  {{ item.masked_email }}
                </td>
                <td class="px-5 py-4 text-right font-semibold text-gray-900 dark:text-white">
                  {{ formatTokenMillions(item.tokens) }}
                </td>
                <td class="px-5 py-4 text-right text-gray-600 dark:text-gray-300">
                  {{ formatFullNumber(item.requests) }}
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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { TOKENS_PER_MILLION } from '@/utils/usagePricing'
import { usageAPI, type UserTokenLeaderboardItem, type UserTokenLeaderboardPeriod, type UserTokenLeaderboardResponse } from '@/api/usage'

const { t } = useI18n()

const emptyMyRank: UserTokenLeaderboardItem = {
  rank: 0,
  masked_email: '***',
  requests: 0,
  tokens: 0,
  is_current_user: true,
}

const leaderboard = ref<UserTokenLeaderboardResponse | null>(null)
const leaderboardPeriod = ref<UserTokenLeaderboardPeriod>('day')
const loading = ref(true)
const loadError = ref(false)
let leaderboardRequestSeq = 0

const ranking = computed(() => leaderboard.value?.ranking ?? [])
const myRank = computed(() => leaderboard.value?.my_rank ?? emptyMyRank)
const displayDate = computed(() => leaderboard.value?.start_date || new Date().toISOString().slice(0, 10))
const displayRangeText = computed(() => {
  if (leaderboardPeriod.value === 'week') {
    return t('leaderboard.weekRange', {
      start: leaderboard.value?.start_date || displayDate.value,
      end: leaderboard.value?.end_date || displayDate.value,
    })
  }
  return t('leaderboard.todayRange', { date: displayDate.value })
})
const periodOptions = computed(() => [
  { value: 'day' as const, label: t('leaderboard.periodDay') },
  { value: 'week' as const, label: t('leaderboard.periodWeek') },
])
const currentPeriodLabel = computed(() => (
  leaderboardPeriod.value === 'week' ? t('leaderboard.periodWeek') : t('leaderboard.periodDay')
))
const emptyStateTitle = computed(() => (
  leaderboardPeriod.value === 'week' ? t('leaderboard.noDataWeek') : t('leaderboard.noData')
))
const emptyStateDescription = computed(() => (
  leaderboardPeriod.value === 'week' ? t('leaderboard.noDataWeekDescription') : t('leaderboard.noDataDescription')
))
const topTokenTotal = computed(() => ranking.value.reduce((sum, item) => sum + item.tokens, 0))

function formatFullNumber(value: number): string {
  return value.toLocaleString()
}

function formatTokenMillions(value: number): string {
  return `${(value / TOKENS_PER_MILLION).toFixed(2)}M`
}

function rankLabel(rank: number): string {
  return rank > 0 ? `#${rank}` : t('leaderboard.unranked')
}

function rankBadgeClass(rank: number): string {
  const base = 'inline-flex h-8 min-w-12 items-center justify-center rounded-md px-3 text-sm font-semibold'
  if (rank === 1) return `${base} bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200`
  if (rank === 2) return `${base} bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200`
  if (rank === 3) return `${base} bg-orange-100 text-orange-800 dark:bg-orange-900/50 dark:text-orange-200`
  return `${base} bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200`
}

async function loadLeaderboard(): Promise<void> {
  const requestSeq = ++leaderboardRequestSeq
  const period = leaderboardPeriod.value
  loading.value = true
  loadError.value = false
  try {
    const response = await usageAPI.getDashboardLeaderboard({ period })
    if (requestSeq !== leaderboardRequestSeq || period !== leaderboardPeriod.value) return
    leaderboard.value = response
  } catch (error) {
    if (requestSeq !== leaderboardRequestSeq || period !== leaderboardPeriod.value) return
    console.error('Failed to load token leaderboard:', error)
    leaderboard.value = null
    loadError.value = true
  } finally {
    if (requestSeq === leaderboardRequestSeq && period === leaderboardPeriod.value) {
      loading.value = false
    }
  }
}

function setLeaderboardPeriod(period: UserTokenLeaderboardPeriod): void {
  if (leaderboardPeriod.value === period) return
  leaderboardPeriod.value = period
  void loadLeaderboard()
}

onMounted(() => {
  loadLeaderboard()
})
</script>
