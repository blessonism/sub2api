<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center py-12">
      <LoadingSpinner />
    </div>

    <EmptyState
      v-else-if="!home?.campaign"
      class="card min-h-80"
      :title="t('lotteryCampaign.emptyTitle')"
      :description="t('lotteryCampaign.emptyDescription')"
    />

    <template v-else>
      <div class="rounded-2xl bg-gradient-to-br from-indigo-600 via-sky-600 to-emerald-600 p-6 text-white shadow-lg">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <div class="inline-flex rounded-full bg-white/15 px-3 py-1 text-xs font-medium">
              {{ t(`lotteryCampaign.statuses.${home.campaign.status}`) }}
            </div>
            <h1 class="mt-4 text-2xl font-semibold sm:text-3xl">{{ home.campaign.name }}</h1>
            <p class="mt-2 max-w-3xl text-sm text-white/80">{{ home.campaign.description || t('lotteryCampaign.defaultDescription') }}</p>
          </div>
          <div class="rounded-2xl bg-white/15 p-4 backdrop-blur">
            <p class="text-sm text-white/75">{{ t('lotteryCampaign.nextDraw') }}</p>
            <p class="mt-2 text-2xl font-semibold tabular-nums">{{ nextDrawText }}</p>
          </div>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-3">
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.todayTokens') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatTokens(myData?.today_tokens) }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.threshold') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatTokens(home.campaign.threshold_tokens) }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.entryCount') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ myData?.entry_count ?? 0 }}</p>
        </div>
      </div>

      <div class="card p-5">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ entryStatusTitle }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ entryStatusDescription }}</p>
          </div>
          <button
            v-if="home.campaign.participation_mode === 'manual'"
            class="btn btn-primary"
            type="button"
            :disabled="enrolling || myData?.entry_status === 'enrolled' || (myData?.entry_count ?? 0) <= 0"
            @click="enroll"
          >
            <Icon name="sparkles" size="sm" />
            {{ enrolling ? t('common.processing') : t('lotteryCampaign.enroll') }}
          </button>
        </div>
        <div class="mt-4 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
          <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPct}%` }" />
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="border-b border-gray-100 p-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.myWinners') }}</h2>
        </div>
        <div v-if="(myData?.winners.length ?? 0) === 0" class="p-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('lotteryCampaign.noWinners') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="winner in myData?.winners" :key="winner.id" class="flex items-center justify-between gap-4 p-4">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ winner.prize_name || t('lotteryCampaign.prize') }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(winner.created_at) }}</p>
            </div>
            <p class="font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import lotteryCampaignsAPI, { type LotteryCampaign, type LotteryMyData } from '@/api/lotteryCampaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const enrolling = ref(false)
const home = ref<{ campaign: LotteryCampaign | null } | null>(null)
const myData = ref<LotteryMyData | null>(null)

const progressPct = computed(() => {
  const threshold = home.value?.campaign?.threshold_tokens ?? 0
  if (threshold <= 0) return 0
  return Math.min(100, Math.round(((myData.value?.today_tokens ?? 0) / threshold) * 100))
})

const nextDrawText = computed(() => {
  const raw = myData.value?.next_draw_at
  return raw ? formatDateTime(raw) : t('lotteryCampaign.pendingDraw')
})

const entryStatusTitle = computed(() => {
  const status = myData.value?.entry_status ?? 'not_eligible'
  return t(`lotteryCampaign.entryStatuses.${status}`)
})

const entryStatusDescription = computed(() => {
  const mode = home.value?.campaign?.participation_mode
  if ((myData.value?.entry_count ?? 0) <= 0) return t('lotteryCampaign.needMoreTokens')
  if (mode === 'manual' && myData.value?.entry_status !== 'enrolled') return t('lotteryCampaign.manualReady')
  return t('lotteryCampaign.readyForDraw')
})

async function load(): Promise<void> {
  loading.value = true
  try {
    home.value = await lotteryCampaignsAPI.getActiveLotteryCampaign()
    if (home.value.campaign) {
      myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function enroll(): Promise<void> {
  if (!home.value?.campaign) return
  enrolling.value = true
  try {
    await lotteryCampaignsAPI.enrollLotteryCampaign(home.value.campaign.id)
    myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
    appStore.showSuccess(t('lotteryCampaign.enrolled'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.enrollFailed')))
  } finally {
    enrolling.value = false
  }
}

function formatTokens(value?: number): string {
  return new Intl.NumberFormat(locale.value).format(value ?? 0)
}

function formatCents(cents?: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY' }).format((cents ?? 0) / 100)
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

onMounted(() => {
  void load()
})
</script>
