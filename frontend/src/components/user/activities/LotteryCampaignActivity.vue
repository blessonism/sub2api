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
      <!-- Hero with live countdown -->
      <div class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-slate-900 via-teal-900 to-teal-700 p-6 text-white shadow-lg">
        <div class="pointer-events-none absolute inset-0 bg-mesh-gradient opacity-60" />
        <div class="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-white/10 blur-3xl" />
        <div class="pointer-events-none absolute -bottom-24 -left-10 h-56 w-56 rounded-full bg-teal-300/20 blur-3xl" />
        <div class="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <div class="inline-flex rounded-full bg-white/15 px-3 py-1 text-xs font-medium">
                {{ t(`lotteryCampaign.statuses.${home.campaign.status}`) }}
              </div>
              <div v-if="isImminent" class="inline-flex items-center gap-1 rounded-full bg-amber-300/25 px-3 py-1 text-xs font-medium text-amber-50 motion-safe:animate-pulse-slow">
                <Icon name="fire" size="sm" />
                {{ t('lotteryCampaign.drawImminent') }}
              </div>
            </div>
            <h1 class="mt-4 text-2xl font-semibold sm:text-3xl">{{ home.campaign.name }}</h1>
            <p class="mt-2 max-w-3xl text-sm text-white/80">{{ campaignDescription }}</p>
            <div class="mt-5 flex flex-wrap gap-2.5">
              <div v-if="prizeTiers.length" class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur">
                <Icon name="gift" size="sm" class="text-amber-100" />
                <span class="tabular-nums">{{ formatCents(totalPoolCents) }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.totalPool') }}</span>
              </div>
              <div v-if="prizeTiers.length" class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur">
                <Icon name="gift" size="sm" class="text-sky-100" />
                <span class="tabular-nums">{{ totalWinnerSlots }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.totalWinners') }}</span>
              </div>
              <div class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur">
                <Icon name="users" size="sm" class="text-emerald-100" />
                <span class="tabular-nums">{{ participantCount }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.participants') }}</span>
              </div>
            </div>
          </div>
          <div class="rounded-2xl bg-white/15 p-4 backdrop-blur motion-safe:animate-glow sm:min-w-[260px]">
            <p class="text-center text-sm text-white/75">{{ drawTimeLabel }}</p>
            <div v-if="hasCountdown" class="mt-3 flex items-stretch justify-center gap-1.5">
              <template v-for="(segment, index) in countdownSegments" :key="segment.label">
                <div class="flex min-w-[3rem] flex-col items-center rounded-xl bg-white/15 px-2 py-2">
                  <span class="text-2xl font-semibold tabular-nums leading-none">{{ segment.value }}</span>
                  <span class="mt-1 text-[0.65rem] uppercase tracking-wide text-white/70">{{ segment.label }}</span>
                </div>
                <span v-if="index < countdownSegments.length - 1" class="self-center text-xl font-semibold text-white/50">:</span>
              </template>
            </div>
            <p v-else class="mt-3 text-center text-lg font-semibold leading-tight">{{ nextDrawText }}</p>
            <p v-if="hasCountdown" class="mt-3 text-center text-xs text-white/70">{{ nextDrawText }}</p>
          </div>
        </div>
      </div>

      <!-- Metrics -->
      <div class="grid gap-4 sm:grid-cols-3">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ usageMetricLabel }}</p>
            <Icon name="bolt" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatUsage(currentUsage) }}</p>
        </div>
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.threshold') }}</p>
            <Icon name="chart" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatUsage(thresholdUsage) }}</p>
        </div>
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.entryCount') }}</p>
            <Icon name="sparkles" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ entryCount }}</p>
        </div>
      </div>

      <!-- Prize wall -->
      <div v-if="prizeTiers.length" class="card p-5">
        <div class="flex items-center gap-2">
          <Icon name="gift" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.prizePool') }}</h2>
        </div>

        <!-- Single prize: featured banner -->
        <div
          v-if="isSinglePrize"
          class="relative mt-3 overflow-hidden rounded-xl border border-amber-200 bg-gradient-to-br from-amber-50 via-white to-amber-50/40 p-4 shadow-sm motion-safe:animate-scale-in motion-safe:transition motion-safe:duration-200 hover:shadow-card-hover dark:border-amber-500/30 dark:from-amber-900/20 dark:via-dark-800 dark:to-dark-800"
        >
          <div class="pointer-events-none absolute -right-6 -top-6 text-amber-200/50 motion-safe:animate-pulse-slow dark:text-amber-500/10">
            <Icon name="gift" size="xl" class="h-24 w-24" />
          </div>
          <div class="relative flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-center gap-3">
              <span class="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-amber-300 to-amber-500 text-white shadow-sm dark:from-amber-400 dark:to-amber-600">
                <Icon name="gift" size="md" />
              </span>
              <div>
                <p class="text-base font-semibold text-amber-700 dark:text-amber-200">{{ prizeTiers[0].tier_name || t('lotteryCampaign.prize') }}</p>
                <p class="mt-0.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
                  <Icon name="users" size="xs" />
                  {{ t('lotteryCampaign.winnerCount', { count: prizeTiers[0].winner_count }) }}
                </p>
              </div>
            </div>
            <p class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCents(prizeTiers[0].reward_amount_cents) }}</p>
          </div>
        </div>

        <!-- Multiple prizes: tier grid -->
        <div v-else class="mt-3 grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="(tier, index) in prizeTiers"
            :key="tier.id"
            class="group relative overflow-hidden rounded-lg border p-3 motion-safe:animate-scale-in motion-safe:transition motion-safe:duration-200 hover:-translate-y-0.5 hover:shadow-card-hover"
            :class="tierAccent(index).card"
          >
            <div class="flex items-center gap-2">
              <span class="inline-flex h-7 w-7 items-center justify-center rounded-full text-sm font-bold tabular-nums" :class="tierAccent(index).medal">
                {{ index + 1 }}
              </span>
              <p class="text-sm font-semibold" :class="tierAccent(index).label">{{ tier.tier_name || t('lotteryCampaign.prize') }}</p>
            </div>
            <p class="mt-2 text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCents(tier.reward_amount_cents) }}</p>
            <p class="mt-0.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
              <Icon name="users" size="xs" />
              {{ t('lotteryCampaign.winnerCount', { count: tier.winner_count }) }}
            </p>
          </div>
        </div>
      </div>
      <!-- Entry status + stepped ladder -->
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
            :disabled="enrolling || myData?.entry_status === 'enrolled' || entryCount <= 0"
            @click="enroll"
          >
            <Icon name="sparkles" size="sm" />
            {{ enrolling ? t('common.processing') : t('lotteryCampaign.enroll') }}
          </button>
        </div>
        <div
          class="mt-4 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
          role="progressbar"
          data-testid="lottery-threshold-progress"
          :aria-label="t('lotteryCampaign.thresholdProgress')"
          :aria-valuenow="progressPct"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPct}%` }" />
        </div>
        <div class="mt-2 flex items-center justify-between gap-2 text-sm">
          <p class="font-medium text-primary-600 dark:text-primary-300">{{ progressHintText }}</p>
          <p class="tabular-nums text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.thresholdProgressPercent', { percent: progressPct }) }}</p>
        </div>

        <!-- Stepped ladder -->
        <div v-if="showLadder" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('lotteryCampaign.entryLadder') }}</p>
            <p class="text-sm font-semibold text-primary-600 dark:text-primary-300 tabular-nums">{{ entryCount }} / {{ maxEntries }}</p>
          </div>
          <div class="mt-3 flex flex-wrap gap-1.5" :aria-label="t('lotteryCampaign.entryLadder')">
            <span
              v-for="slot in maxEntries"
              :key="slot"
              class="h-2.5 flex-1 rounded-full transition-colors"
              :class="slot <= entryCount ? 'bg-primary-500' : 'bg-gray-100 dark:bg-dark-700'"
            />
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ ladderHintText }}</p>
        </div>
      </div>

      <!-- Rules -->
      <div v-if="rulesText" class="card p-5">
        <div class="flex items-center gap-2">
          <Icon name="document" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.rulesTitle') }}</h2>
        </div>
        <p class="mt-3 whitespace-pre-line text-sm leading-relaxed text-gray-600 dark:text-dark-300">{{ rulesText }}</p>
      </div>

      <!-- Recent winners feed (social proof) -->
      <div class="card overflow-hidden">
        <div class="flex items-center gap-2 border-b border-gray-100 p-4 dark:border-dark-700">
          <Icon name="users" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.recentWinners') }}</h2>
        </div>
        <div v-if="recentWinners.length === 0" class="p-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('lotteryCampaign.noRecentWinners') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="(winner, index) in recentWinners"
            :key="`${winner.masked_email}-${winner.created_at}-${index}`"
            class="flex items-center justify-between gap-4 p-4"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/40 dark:text-primary-300">
                <Icon name="user" size="sm" />
              </span>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ winner.masked_email }}</p>
                <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">
                  {{ winner.prize_name || t('lotteryCampaign.prize') }} · {{ formatRelativeTime(winner.created_at) }}
                </p>
              </div>
            </div>
            <p class="shrink-0 text-sm font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</p>
          </div>
        </div>
      </div>

      <!-- My winners with ceremony -->
      <div class="card overflow-hidden">
        <div class="flex items-center gap-2 border-b border-gray-100 p-4 dark:border-dark-700">
          <Icon name="sparkles" size="sm" class="text-amber-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.myWinners') }}</h2>
        </div>
        <div v-if="(myData?.winners.length ?? 0) === 0" class="p-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('lotteryCampaign.noWinners') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="winner in myData?.winners"
            :key="winner.id"
            class="flex items-center justify-between gap-4 bg-gradient-to-r from-amber-50/60 to-transparent p-4 motion-safe:animate-scale-in dark:from-amber-900/10"
          >
            <div class="flex items-center gap-3">
              <span class="inline-flex h-10 w-10 items-center justify-center rounded-full bg-amber-100 text-amber-600 motion-safe:animate-glow dark:bg-amber-900/40 dark:text-amber-300">
                <Icon name="gift" size="sm" />
              </span>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ winner.prize_name || t('lotteryCampaign.prize') }}</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(winner.created_at) }}</p>
              </div>
            </div>
            <p class="text-lg font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import lotteryCampaignsAPI, { type LotteryCampaign, type LotteryMyData, type LotteryPrizeTier, type LotteryPublicWinner } from '@/api/lotteryCampaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatTokenMillions } from '@/utils/usagePricing'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const enrolling = ref(false)
const home = ref<{ campaign: LotteryCampaign | null } | null>(null)
const myData = ref<LotteryMyData | null>(null)
const recentWinners = ref<LotteryPublicWinner[]>([])
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | null = null

const isSingleDraw = computed(() => home.value?.campaign?.draw_schedule_type === 'single')
const isUSDMode = computed(() => home.value?.campaign?.usage_mode === 'usd')
const usageMetricLabel = computed(() => isUSDMode.value
  ? t(isSingleDraw.value ? 'lotteryCampaign.cumulativeCost' : 'lotteryCampaign.todayCost')
  : t(isSingleDraw.value ? 'lotteryCampaign.cumulativeTokens' : 'lotteryCampaign.todayTokens'))
const drawTimeLabel = computed(() => isSingleDraw.value ? t('lotteryCampaign.drawTime') : t('lotteryCampaign.nextDraw'))
const campaignDescription = computed(() => {
  const description = home.value?.campaign?.description
  if (description) return description
  return isSingleDraw.value ? t('lotteryCampaign.defaultOneTimeDescription') : t('lotteryCampaign.defaultDescription')
})

const progressPct = computed(() => {
  const threshold = thresholdUsage.value
  if (threshold <= 0) return 0
  return Math.min(100, Math.round((currentUsage.value / threshold) * 100))
})
const currentUsage = computed(() => isUSDMode.value ? (myData.value?.today_cost_microusd ?? 0) : (myData.value?.today_tokens ?? 0))
const thresholdUsage = computed(() => isUSDMode.value ? (home.value?.campaign?.threshold_cost_microusd ?? 0) : (home.value?.campaign?.threshold_tokens ?? 0))
const remainingUsage = computed(() => Math.max(0, thresholdUsage.value - currentUsage.value))
const progressHintText = computed(() => {
  if (remainingUsage.value <= 0) return t('lotteryCampaign.thresholdReached')
  return t(isUSDMode.value ? 'lotteryCampaign.costToThreshold' : 'lotteryCampaign.tokensToThreshold', { amount: formatUsage(remainingUsage.value) })
})

const nextDrawAt = computed(() => myData.value?.next_draw_at ?? home.value?.campaign?.draw_at ?? null)
const remainingMs = computed(() => {
  if (!nextDrawAt.value) return null
  const target = new Date(nextDrawAt.value).getTime()
  if (!Number.isFinite(target)) return null
  return Math.max(0, target - now.value.getTime())
})
const isImminent = computed(() => remainingMs.value !== null && remainingMs.value > 0 && remainingMs.value <= 60 * 60 * 1000)
const nextDrawText = computed(() => {
  const raw = nextDrawAt.value
  return raw ? formatDateTime(raw) : t('lotteryCampaign.pendingDraw')
})

interface CountdownSegment {
  value: string
  label: string
}
const countdownSegments = computed<CountdownSegment[]>(() => {
  const ms = remainingMs.value
  if (ms === null || ms <= 0) return []
  const totalSeconds = Math.floor(ms / 1000)
  const days = Math.floor(totalSeconds / 86_400)
  const hours = Math.floor((totalSeconds % 86_400) / 3_600)
  const minutes = Math.floor((totalSeconds % 3_600) / 60)
  const seconds = totalSeconds % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  const segments: CountdownSegment[] = []
  if (days > 0) segments.push({ value: String(days), label: t('lotteryCampaign.countdownDays') })
  segments.push({ value: pad(hours), label: t('lotteryCampaign.countdownHours') })
  segments.push({ value: pad(minutes), label: t('lotteryCampaign.countdownMinutes') })
  segments.push({ value: pad(seconds), label: t('lotteryCampaign.countdownSeconds') })
  return segments
})
const hasCountdown = computed(() => countdownSegments.value.length > 0)

const prizeTiers = computed<LotteryPrizeTier[]>(() => home.value?.campaign?.prize_tiers ?? [])
const isSinglePrize = computed(() => prizeTiers.value.length === 1)
const totalWinnerSlots = computed(() => prizeTiers.value.reduce((acc, tier) => acc + tier.winner_count, 0))
const totalPoolCents = computed(() => prizeTiers.value.reduce((acc, tier) => acc + tier.reward_amount_cents * tier.winner_count, 0))
const rulesText = computed(() => home.value?.campaign?.rules_text ?? '')

const participantCount = computed(() => myData.value?.participant_count ?? 0)
const entryCount = computed(() => myData.value?.entry_count ?? 0)
const maxEntries = computed(() => home.value?.campaign?.max_entries_per_user ?? 1)
const showLadder = computed(() => home.value?.campaign?.entry_mode === 'stepped' && maxEntries.value > 1 && maxEntries.value <= 12)
const ladderHintText = computed(() => {
  const campaign = home.value?.campaign
  if (!campaign) return ''
  if (entryCount.value >= maxEntries.value) return t('lotteryCampaign.ladderMaxed')
  const step = isUSDMode.value ? campaign.entry_step_cost_microusd : campaign.entry_step_tokens
  const tokens = currentUsage.value
  if (step <= 0 || entryCount.value <= 0) return t('lotteryCampaign.ladderStart')
  const nextAt = thresholdUsage.value + entryCount.value * step
  const need = Math.max(0, nextAt - tokens)
  return t('lotteryCampaign.ladderNext', { amount: formatUsage(need) })
})

const entryStatusTitle = computed(() => {
  const status = myData.value?.entry_status ?? 'not_eligible'
  return t(`lotteryCampaign.entryStatuses.${status}`)
})
const entryStatusDescription = computed(() => {
  const mode = home.value?.campaign?.participation_mode
  if (entryCount.value <= 0) return t(isUSDMode.value ? 'lotteryCampaign.needMoreCost' : 'lotteryCampaign.needMoreTokens')
  if (mode === 'manual' && myData.value?.entry_status !== 'enrolled') return t('lotteryCampaign.manualReady')
  return t('lotteryCampaign.readyForDraw')
})

interface TierAccent {
  card: string
  medal: string
  label: string
}
const TIER_ACCENTS: TierAccent[] = [
  {
    card: 'border-amber-200 bg-gradient-to-br from-amber-50 to-white dark:border-amber-500/30 dark:from-amber-900/20 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-amber-400 to-amber-500 text-white shadow-sm shadow-amber-500/30',
    label: 'text-amber-700 dark:text-amber-200',
  },
  {
    card: 'border-gray-200 bg-gradient-to-br from-gray-50 to-white dark:border-dark-600 dark:from-dark-700/40 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-gray-300 to-gray-400 text-white shadow-sm shadow-gray-400/30',
    label: 'text-gray-700 dark:text-dark-200',
  },
  {
    card: 'border-orange-200 bg-gradient-to-br from-orange-50 to-white dark:border-orange-500/30 dark:from-orange-900/20 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-orange-400 to-orange-500 text-white shadow-sm shadow-orange-500/30',
    label: 'text-orange-700 dark:text-orange-200',
  },
]
const DEFAULT_ACCENT: TierAccent = {
  card: 'border-gray-100 dark:border-dark-700',
  medal: 'bg-gradient-to-br from-primary-400 to-primary-500 text-white shadow-sm shadow-primary-500/30',
  label: 'text-gray-700 dark:text-dark-200',
}
function tierAccent(index: number): TierAccent {
  return TIER_ACCENTS[index] ?? DEFAULT_ACCENT
}

async function load(): Promise<void> {
  loading.value = true
  try {
    home.value = await lotteryCampaignsAPI.getActiveLotteryCampaign()
    if (home.value.campaign) {
      myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
      void loadWinners()
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadWinners(): Promise<void> {
  const campaignId = home.value?.campaign?.id
  if (!campaignId) return
  try {
    const { items } = await lotteryCampaignsAPI.getRecentLotteryWinners(campaignId)
    recentWinners.value = items
  } catch {
    // Recent winners are supplementary; keep the page usable if this fails.
  }
}

async function enroll(): Promise<void> {
  if (!home.value?.campaign) return
  enrolling.value = true
  try {
    await lotteryCampaignsAPI.enrollLotteryCampaign(home.value.campaign.id)
    myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
    void loadWinners()
    appStore.showSuccess(t('lotteryCampaign.enrolled'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.enrollFailed')))
  } finally {
    enrolling.value = false
  }
}

function formatTokens(value?: number): string {
  return formatTokenMillions(value ?? 0)
}

function formatUsage(value: number): string {
  if (!isUSDMode.value) return formatTokens(value)
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value / 1_000_000)
}

function formatCents(cents?: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY' }).format((cents ?? 0) / 100)
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatRelativeTime(value: string): string {
  const target = new Date(value).getTime()
  if (!Number.isFinite(target)) return ''
  const diffSec = Math.round((target - now.value.getTime()) / 1000)
  const abs = Math.abs(diffSec)
  const rtf = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  if (abs < 60) return rtf.format(Math.round(diffSec), 'second')
  if (abs < 3600) return rtf.format(Math.round(diffSec / 60), 'minute')
  if (abs < 86_400) return rtf.format(Math.round(diffSec / 3600), 'hour')
  return rtf.format(Math.round(diffSec / 86_400), 'day')
}

onMounted(() => {
  clockTimer = setInterval(() => {
    now.value = new Date()
  }, 1000)
  void load()
})

onUnmounted(() => {
  if (clockTimer) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>
