<template>
  <div class="space-y-6">
    <div v-if="loading && !home" class="flex justify-center py-12">
      <LoadingSpinner />
    </div>

    <EmptyState
      v-else-if="!home?.campaign"
      class="card min-h-80"
      :title="t('campaignRewards.emptyTitle')"
      :description="home?.estimate_notice || t('campaignRewards.emptyDescription')"
    />

    <template v-else>
      <div class="rounded-2xl bg-gradient-to-br from-primary-600 via-sky-600 to-emerald-600 p-6 text-white shadow-lg">
        <div class="grid gap-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(300px,0.85fr)] lg:items-stretch">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <div class="inline-flex rounded-full bg-white/15 px-3 py-1 text-xs font-medium">
                {{ statusLabel(home.campaign.status) }}
              </div>
              <div v-if="lifecycleState.isUrgent" class="inline-flex rounded-full bg-amber-300/25 px-3 py-1 text-xs font-medium text-amber-50">
                {{ t('campaignRewards.lifecycle.urgent') }}
              </div>
            </div>
            <h1 class="mt-4 text-2xl font-semibold sm:text-3xl">{{ home.campaign.name }}</h1>
            <p class="mt-2 max-w-3xl text-sm text-white/80">{{ home.campaign.description || t('campaignRewards.defaultDescription') }}</p>
            <p class="mt-3 max-w-3xl text-sm font-medium text-white">{{ t(lifecycleState.descriptionKey) }}</p>
            <div class="mt-5 max-w-3xl space-y-3">
              <div class="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-white/75">
                <span class="inline-flex items-center gap-1.5">
                  <Icon name="calendar" size="sm" class="text-white/60" />
                  <span>{{ t('campaignRewards.campaignPeriod') }}</span>
                </span>
                <span class="tabular-nums text-white/90">{{ campaignPeriodText }}</span>
                <span class="hidden h-1 w-1 rounded-full bg-white/35 sm:inline-block" />
                <span class="text-white/70">{{ campaignProgressText }}</span>
              </div>
              <div
                class="relative h-3 w-full max-w-lg"
                role="progressbar"
                :aria-label="t('campaignRewards.campaignPeriodProgress')"
                :aria-valuenow="campaignElapsedPercent"
                aria-valuemin="0"
                aria-valuemax="100"
                :data-progress-value="campaignElapsedPercent"
              >
                <span class="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 bg-white/30" />
                <span
                  class="absolute left-0 top-1/2 h-0.5 -translate-y-1/2 rounded-full bg-white/85 transition-all"
                  :style="{ width: `${campaignElapsedPercent}%` }"
                />
                <span class="absolute left-0 top-1/2 h-2 w-2 -translate-y-1/2 rounded-full bg-white/85" />
                <span class="absolute right-0 top-1/2 h-2 w-2 -translate-y-1/2 rounded-full bg-white/45" />
              </div>
            </div>
            <div class="mt-5 flex flex-wrap gap-3">
              <button
                class="btn bg-white text-primary-700 hover:bg-white/90"
                type="button"
                :disabled="!canCopyInviteLink"
                @click="copyLink"
              >
                <Icon name="copy" size="sm" />
                {{ t('campaignRewards.copyInviteLinkPrimary') }}
              </button>
              <span v-if="!lifecycleState.isActionable" class="inline-flex items-center text-xs text-white/75">
                {{ t('campaignRewards.ctaDisabledHint') }}
              </span>
            </div>
          </div>
          <div class="grid gap-3">
            <div class="rounded-2xl bg-white/15 p-4 backdrop-blur">
              <p class="text-sm text-white/75">{{ t(lifecycleState.labelKey) }}</p>
              <p class="mt-2 text-3xl font-semibold tabular-nums">{{ lifecycleCountdownText }}</p>
              <p class="mt-2 text-xs text-white/75">{{ lifecycleTargetText }}</p>
            </div>
            <div class="rounded-2xl bg-white/15 p-4 backdrop-blur">
              <p class="text-sm text-white/75">{{ t('campaignRewards.estimatedPool') }}</p>
              <p class="mt-2 text-2xl font-semibold">{{ formatCents(home.pool?.estimated_total_pool_cents) }}</p>
              <p class="mt-2 text-xs text-white/75">{{ home.estimate_notice || t('campaignRewards.estimateNoticeFallback') }}</p>
            </div>
          </div>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="metric in metrics" :key="metric.label" class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ metric.label }}</p>
            <Icon :name="metric.icon" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</p>
        </div>
      </div>

      <div data-testid="campaign-rules-layout" class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)] xl:items-stretch">
        <div class="card h-full p-6">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.inviteEntry') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.inviteEntryDesc') }}</p>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              {{ t('campaignRewards.threshold') }} {{ formatCents(home.config?.recharge_threshold_cents) }}
            </div>
          </div>

          <div class="mt-5 grid gap-4">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaignRewards.myCode') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ inviteCode }}</code>
                <button class="btn btn-secondary btn-sm" type="button" :disabled="!inviteCode" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('campaignRewards.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaignRewards.inviteLink') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm" type="button" :disabled="!canCopyInviteLink" @click="copyLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('campaignRewards.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 grid gap-3 sm:grid-cols-3">
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('campaignRewards.rankReward') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatCents(myData?.estimated_rank_reward_cents) }}</p>
            </div>
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('campaignRewards.contributionReward') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatCents(myData?.estimated_contribution_reward_cents) }}</p>
            </div>
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('campaignRewards.inviteeRecharge') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatCents(myData?.invitee_recharge_amount_cents) }}</p>
            </div>
          </div>
          <div class="mt-5 rounded-xl border border-primary-100 bg-primary-50 p-4 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-200">
            {{ progressHint }}
          </div>
        </div>

        <div data-testid="campaign-rules-card" class="card h-full p-6">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.rulesTitle') }}</h2>
            </div>
          </div>
          <div class="mt-4 space-y-4">
            <div class="grid gap-3">
              <div
                v-for="(rule, index) in ruleSummaryItems"
                :key="rule.title"
                data-testid="campaign-rule-summary-item"
                class="rounded-xl border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-700 dark:bg-dark-900/50"
              >
                <div class="flex gap-3">
                  <span class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-100 text-xs font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
                    {{ index + 1 }}
                  </span>
                  <div class="min-w-0">
                    <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ rule.title }}</p>
                    <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ rule.description }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-6">
        <div class="card overflow-hidden">
          <div class="flex items-start justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.leaderboard') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.leaderboardDesc') }}</p>
            </div>
            <button
              class="btn btn-secondary btn-sm shrink-0"
              type="button"
              :disabled="refreshing"
              @click="refresh"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': refreshing }" />
              {{ t('common.refresh') }}
            </button>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ t('campaignRewards.rank') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('campaignRewards.user') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('campaignRewards.validInvites') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('campaignRewards.pendingRechargeInvites') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('campaignRewards.estimatedReward') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr
                  v-for="row in leaderboard"
                  :key="row.user_id"
                  :class="{ 'bg-primary-50/70 dark:bg-primary-900/20': row.user_id === myData?.user_id }"
                >
                  <td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">
                    <span class="inline-flex min-w-10 items-center justify-center rounded-full px-2 py-1 text-xs font-semibold" :class="leaderboardRankClass(row.rank)">
                      #{{ row.rank }}
                    </span>
                  </td>
                  <td class="px-4 py-3">{{ displayUser(row.username, row.masked_email) }}</td>
                  <td class="px-4 py-3 text-right">{{ row.valid_invite_count }}</td>
                  <td class="px-4 py-3 text-right">{{ formatCount(row.pending_invite_count) }}</td>
                  <td class="px-4 py-3 text-right font-medium">{{ formatCents(row.estimated_reward_cents) }}</td>
                </tr>
                <tr v-if="leaderboard.length === 0">
                  <td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('campaignRewards.noLeaderboard') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.inviteRecords') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.inviteRecordsDesc') }}</p>
          </div>
          <div class="hidden md:block">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ t('campaignRewards.invitee') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('campaignRewards.status') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('campaignRewards.rechargeProgress') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('campaignRewards.timeline') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in inviteRecords" :key="record.id">
                  <td class="px-4 py-3">{{ displayUser(record.invitee_username, record.invitee_masked_email) }}</td>
                  <td class="px-4 py-3">
                    <span class="rounded-md px-2 py-1 text-xs font-medium" :class="inviteStatusClass(record.status)">
                      {{ inviteStatusLabel(record.status) }}
                    </span>
                    <p class="mt-2 max-w-xs text-xs text-gray-500 dark:text-dark-400">{{ inviteStatusNote(record) || '-' }}</p>
                  </td>
                  <td class="px-4 py-3 text-right">
                    <p class="font-medium text-gray-900 dark:text-white">{{ formatCents(record.effective_recharge_amount_cents) }}</p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">/ {{ formatCents(record.threshold_snapshot_cents) }}</p>
                    <div class="mt-1.5 ml-auto h-1.5 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                      <div
                        class="h-full rounded-full bg-primary-500 transition-all"
                        :style="{ width: `${rechargeProgressPct(record)}%` }"
                      />
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <p class="text-gray-900 dark:text-white">{{ t('campaignRewards.registeredAt') }}: {{ formatDateTime(record.registered_at) }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      {{ t('campaignRewards.qualifiedAt') }}: {{ record.qualified_at ? formatDateTime(record.qualified_at) : t('campaignRewards.notQualifiedYet') }}
                    </p>
                  </td>
                </tr>
                <tr v-if="inviteRecords.length === 0">
                  <td colspan="4" class="px-4 py-10 text-center text-gray-500">
                    <div class="space-y-3">
                      <p>{{ t('campaignRewards.noInvites') }}</p>
                      <button class="btn btn-primary btn-sm" type="button" :disabled="!canCopyInviteLink" @click="copyLink">
                        {{ t('campaignRewards.copyInviteLinkPrimary') }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="space-y-3 p-4 md:hidden">
            <div
              v-for="record in inviteRecords"
              :key="record.id"
              class="rounded-xl border border-gray-100 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                    {{ displayUser(record.invitee_username, record.invitee_masked_email) }}
                  </p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ inviteStatusNote(record) || '-' }}</p>
                </div>
                <span class="shrink-0 rounded-md px-2 py-1 text-xs font-medium" :class="inviteStatusClass(record.status)">
                  {{ inviteStatusLabel(record.status) }}
                </span>
              </div>
              <div class="mt-4 grid gap-3 text-sm">
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                  <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('campaignRewards.rechargeProgress') }}</p>
                  <p class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ formatCents(record.effective_recharge_amount_cents) }}
                    <span class="font-normal text-gray-500 dark:text-dark-400">/ {{ formatCents(record.threshold_snapshot_cents) }}</span>
                  </p>
                  <div class="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                    <div
                      class="h-full rounded-full bg-primary-500 transition-all"
                      :style="{ width: `${rechargeProgressPct(record)}%` }"
                    />
                  </div>
                </div>
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                  <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('campaignRewards.timeline') }}</p>
                  <p class="mt-1 text-gray-900 dark:text-white">{{ t('campaignRewards.registeredAt') }}: {{ formatDateTime(record.registered_at) }}</p>
                  <p class="mt-1 text-gray-500 dark:text-dark-400">
                    {{ t('campaignRewards.qualifiedAt') }}: {{ record.qualified_at ? formatDateTime(record.qualified_at) : t('campaignRewards.notQualifiedYet') }}
                  </p>
                </div>
              </div>
            </div>
            <div v-if="inviteRecords.length === 0" class="rounded-xl border border-gray-100 bg-white px-4 py-8 text-center dark:border-dark-700 dark:bg-dark-900">
              <div class="space-y-3">
                <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.noInvites') }}</p>
                <button class="btn btn-primary btn-sm" type="button" :disabled="!canCopyInviteLink" @click="copyLink">
                  {{ t('campaignRewards.copyInviteLinkPrimary') }}
                </button>
              </div>
            </div>
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
import campaignsAPI, {
  type CampaignHome,
  type CampaignInviteRecord,
  type CampaignLeaderboardRow,
  type CampaignMyData,
} from '@/api/campaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useClipboard } from '@/composables/useClipboard'
import { getCampaignLifecycleState } from '@/utils/campaignLifecycle'

const { t, locale } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const refreshing = ref(false)
const home = ref<CampaignHome | null>(null)
const myData = ref<CampaignMyData | null>(null)
const inviteRecords = ref<CampaignInviteRecord[]>([])
const leaderboard = ref<CampaignLeaderboardRow[]>([])
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | null = null

const inviteCode = computed(() => myData.value?.invite_code || '')
const inviteLink = computed(() => {
  const raw = myData.value?.invite_link || ''
  if (!raw) return ''
  if (/^https?:\/\//i.test(raw)) return raw
  if (typeof window === 'undefined') return raw
  return `${window.location.origin}${raw.startsWith('/') ? raw : `/${raw}`}`
})

function parseDateMs(raw?: string | null): number | null {
  if (!raw) return null
  const value = new Date(raw).getTime()
  return Number.isFinite(value) ? value : null
}

function formatShortDate(raw?: string | null): string {
  const time = parseDateMs(raw)
  if (time === null) return t('campaignRewards.campaignPeriodPending')
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'numeric',
    day: 'numeric',
  }).format(new Date(time))
}

const campaignPeriodText = computed(() => {
  const campaign = home.value?.campaign
  if (!campaign?.start_at || !campaign?.end_at) return t('campaignRewards.campaignPeriodPending')
  return `${formatShortDate(campaign.start_at)} - ${formatShortDate(campaign.end_at)}`
})

const campaignElapsedPercent = computed(() => {
  const campaign = home.value?.campaign
  const start = parseDateMs(campaign?.start_at)
  const end = parseDateMs(campaign?.end_at)
  if (start === null || end === null || end <= start) return 0
  const elapsed = ((now.value.getTime() - start) / (end - start)) * 100
  return Math.min(100, Math.max(0, Math.round(elapsed)))
})

const campaignProgressText = computed(() => t('campaignRewards.periodElapsed', { percent: campaignElapsedPercent.value }))

const metrics = computed(() => [
  {
    label: t('campaignRewards.validInvites'),
    value: formatCount(myData.value?.valid_invite_count),
    icon: 'userPlus' as const,
  },
  {
    label: t('campaignRewards.pendingInvites'),
    value: formatCount(myData.value?.pending_invite_count),
    icon: 'clock' as const,
  },
  {
    label: t('campaignRewards.currentRank'),
    value: myData.value?.current_rank ? `#${myData.value.current_rank}` : t('campaignRewards.unranked'),
    icon: 'trendingUp' as const,
  },
  {
    label: t('campaignRewards.estimatedReward'),
    value: formatCents(myData.value?.estimated_total_reward_cents),
    icon: 'gift' as const,
  },
])

const lifecycleState = computed(() => {
  if (!home.value?.campaign) {
    throw new Error('campaign is required after active campaign render')
  }
  return getCampaignLifecycleState(home.value.campaign, now.value, locale.value === 'zh' ? 'zh' : 'en')
})

const lifecycleCountdownText = computed(() => {
  if (lifecycleState.value.countdownText) return lifecycleState.value.countdownText
  if (lifecycleState.value.hasExpiredTarget) return t('campaignRewards.lifecycle.expired')
  if (lifecycleState.value.hasMissingTarget) return t('campaignRewards.lifecycle.targetMissing')
  return t('campaignRewards.lifecycle.noCountdown')
})

const canCopyInviteLink = computed(() => Boolean(inviteLink.value && lifecycleState.value.isActionable))

const lifecycleTargetText = computed(() => {
  if (!lifecycleState.value.targetAt) return t('campaignRewards.lifecycle.targetPending')
  const targetLabelKeys: Record<string, string> = {
    warmup: 'campaignRewards.lifecycle.startsAt',
    active: 'campaignRewards.lifecycle.endsAt',
    auditing: 'campaignRewards.lifecycle.auditEndsAt',
    publicizing: 'campaignRewards.lifecycle.publicityEndsAt',
    pending_payout: 'campaignRewards.lifecycle.payoutDueAt',
  }
  return t(targetLabelKeys[lifecycleState.value.phase] ?? 'campaignRewards.lifecycle.targetAt', {
    time: formatDateTime(lifecycleState.value.targetAt),
  })
})

const progressHint = computed(() => {
  const data = myData.value
  if (!data) return t('campaignRewards.progressStart')
  if (data.current_rank) {
    if (data.distance_to_previous > 0) {
      return t('campaignRewards.distanceToPrevious', { count: data.distance_to_previous })
    }
    return t('campaignRewards.rankSecured', { rank: data.current_rank })
  }
  if (data.distance_to_top10 > 0) {
    return t('campaignRewards.distanceToTop10', { count: data.distance_to_top10 })
  }
  return t('campaignRewards.progressStart')
})

const ruleSummaryItems = computed(() => [
  {
    title: t('campaignRewards.ruleSummaryThresholdTitle'),
    description: t('campaignRewards.ruleSummaryThresholdDesc', {
      threshold: formatCents(home.value?.config?.recharge_threshold_cents),
    }),
  },
  {
    title: t('campaignRewards.ruleSummaryPoolTitle'),
    description: t(home.value?.config?.pool_injection_scope === 'all_users'
      ? 'campaignRewards.ruleSummaryPoolDescAllUsers'
      : 'campaignRewards.ruleSummaryPoolDescInviteesOnly', {
      rate: formatPercent(home.value?.config?.pool_injection_rate),
      rank: formatPercent(home.value?.config?.rank_pool_ratio),
      contribution: formatPercent(home.value?.config?.contribution_pool_ratio),
    }),
  },
  {
    title: t('campaignRewards.ruleSummarySettlementTitle'),
    description: t('campaignRewards.ruleSummarySettlementDesc'),
  },
])

async function loadCampaign(silent = false): Promise<void> {
  if (silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  try {
    const active = await campaignsAPI.getActiveCampaign()
    home.value = active
    if (!active.campaign) {
      myData.value = null
      inviteRecords.value = []
      leaderboard.value = []
      return
    }
    const campaignId = active.campaign.id
    const [me, invites, board] = await Promise.all([
      campaignsAPI.getMyCampaignData(campaignId),
      campaignsAPI.listCampaignInvites(campaignId, { page: 1, page_size: 100 }),
      campaignsAPI.getCampaignLeaderboard(campaignId, 50),
    ])
    myData.value = me
    inviteRecords.value = invites.items || me.invite_records || []
    leaderboard.value = board.items || active.leaderboard || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('campaignRewards.loadFailed')))
  } finally {
    if (silent) refreshing.value = false
    else loading.value = false
  }
}

async function refresh(): Promise<void> {
  if (refreshing.value) return
  await loadCampaign(true)
}

async function copyCode(): Promise<void> {
  if (inviteCode.value) await copyToClipboard(inviteCode.value, t('campaignRewards.codeCopied'))
}

async function copyLink(): Promise<void> {
  if (canCopyInviteLink.value) await copyToClipboard(inviteLink.value, t('campaignRewards.linkCopied'))
}

function formatCents(value?: number | null): string {
  return `¥${((value || 0) / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatCount(value?: number | null): string {
  return (value || 0).toLocaleString()
}

function formatDateTime(raw?: string | null): string {
  if (!raw) return '-'
  return new Date(raw).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}

function formatPercent(value?: string | number | null): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return t('campaignRewards.ratePending')
  const percent = numeric <= 1 ? numeric * 100 : numeric
  return `${percent.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`
}

function displayUser(username?: string, maskedEmail?: string): string {
  return username || maskedEmail || t('common.unknown')
}

function statusLabel(status: string): string {
  return t(`campaignRewards.statuses.${status}`, status)
}

function inviteStatusLabel(status: string): string {
  return t(`campaignRewards.inviteStatuses.${status}`, status)
}

function inviteStatusNote(record: CampaignInviteRecord): string {
  if (record.invalid_reason) return record.invalid_reason
  if (record.audit_note) return record.audit_note
  if (record.risk_level && record.risk_level !== 'none') return t('campaignRewards.riskLevelNote', { level: record.risk_level })
  return t(`campaignRewards.inviteStatusNotes.${record.status}`, '')
}

function inviteStatusClass(status: string): string {
  if (status === 'effective') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'invalid') return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function rechargeProgressPct(record: CampaignInviteRecord): number {
  if (record.threshold_snapshot_cents <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((record.effective_recharge_amount_cents / record.threshold_snapshot_cents) * 100)))
}

function leaderboardRankClass(rank: number): string {
  if (rank === 1) return 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
  if (rank === 2) return 'bg-gray-100 text-gray-800 dark:bg-dark-800 dark:text-dark-200'
  if (rank === 3) return 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-dark-300'
}

onMounted(() => {
  clockTimer = setInterval(() => {
    now.value = new Date()
    if (home.value?.campaign && lifecycleState.value.isTerminal) {
      clearInterval(clockTimer!)
      clockTimer = null
    }
  }, 1000)
  void loadCampaign()
})

onUnmounted(() => {
  if (clockTimer) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>
