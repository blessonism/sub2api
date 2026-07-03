<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-end gap-2">
        <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="loadCampaign">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('campaignRewards.refresh') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
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
          <div class="grid gap-6 lg:grid-cols-[minmax(0,1.4fr)_minmax(280px,0.8fr)] lg:items-end">
            <div>
              <div class="inline-flex rounded-full bg-white/15 px-3 py-1 text-xs font-medium">
                {{ statusLabel(home.campaign.status) }}
              </div>
              <h1 class="mt-4 text-2xl font-semibold sm:text-3xl">{{ home.campaign.name }}</h1>
              <p class="mt-2 max-w-3xl text-sm text-white/80">{{ home.campaign.description || t('campaignRewards.defaultDescription') }}</p>
              <div class="mt-5 flex flex-wrap gap-3 text-sm text-white/85">
                <span>{{ formatDateTime(home.campaign.start_at) }}</span>
                <span>→</span>
                <span>{{ formatDateTime(home.campaign.end_at) }}</span>
              </div>
            </div>
            <div class="rounded-2xl bg-white/15 p-4 backdrop-blur">
              <p class="text-sm text-white/75">{{ t('campaignRewards.estimatedPool') }}</p>
              <p class="mt-2 text-3xl font-semibold">{{ formatCents(home.pool?.estimated_total_pool_cents) }}</p>
              <p class="mt-2 text-xs text-white/75">{{ home.estimate_notice }}</p>
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

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)]">
          <div class="card p-6">
            <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.inviteEntry') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.inviteEntryDesc') }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
                {{ t('campaignRewards.threshold') }} {{ formatCents(home.config?.recharge_threshold_cents) }}
              </div>
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <div class="space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaignRewards.myCode') }}</p>
                <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                  <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ inviteCode }}</code>
                  <button class="btn btn-secondary btn-sm" type="button" :disabled="!inviteCode" @click="copyCode">
                    <Icon name="copy" size="sm" />
                    {{ t('common.copy') }}
                  </button>
                </div>
              </div>
              <div class="space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaignRewards.myLink') }}</p>
                <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                  <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                  <button class="btn btn-secondary btn-sm" type="button" :disabled="!inviteLink" @click="copyLink">
                    <Icon name="copy" size="sm" />
                    {{ t('common.copy') }}
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
          </div>

          <div class="card p-6">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.rulesTitle') }}</h2>
            <div class="mt-4 space-y-3 text-sm text-gray-600 dark:text-dark-300">
              <p>{{ t('campaignRewards.rulePool') }}</p>
              <p>{{ t('campaignRewards.ruleSplit') }}</p>
              <p>{{ t('campaignRewards.ruleContribution') }}</p>
              <p>{{ t('campaignRewards.ruleNotice') }}</p>
            </div>
          </div>
        </div>

        <div class="grid gap-6 xl:grid-cols-2">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 p-4 dark:border-dark-700">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('campaignRewards.leaderboard') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaignRewards.leaderboardDesc') }}</p>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[620px] text-sm">
                <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  <tr>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.rank') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.user') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('campaignRewards.validInvites') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('campaignRewards.rechargeAmount') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('campaignRewards.estimatedReward') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="row in leaderboard" :key="row.user_id">
                    <td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">#{{ row.rank }}</td>
                    <td class="px-4 py-3">{{ displayUser(row.username, row.masked_email) }}</td>
                    <td class="px-4 py-3 text-right">{{ row.valid_invite_count }}</td>
                    <td class="px-4 py-3 text-right">{{ formatCents(row.invitee_recharge_amount_cents) }}</td>
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
            <div class="overflow-x-auto">
              <table class="w-full min-w-[720px] text-sm">
                <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  <tr>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.invitee') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.status') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('campaignRewards.threshold') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('campaignRewards.rechargeAmount') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.registeredAt') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('campaignRewards.qualifiedAt') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="record in inviteRecords" :key="record.id">
                    <td class="px-4 py-3">{{ displayUser(record.invitee_username, record.invitee_masked_email) }}</td>
                    <td class="px-4 py-3">
                      <span class="rounded-md px-2 py-1 text-xs font-medium" :class="inviteStatusClass(record.status)">
                        {{ inviteStatusLabel(record.status) }}
                      </span>
                    </td>
                    <td class="px-4 py-3 text-right">{{ formatCents(record.threshold_snapshot_cents) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatCents(record.effective_recharge_amount_cents) }}</td>
                    <td class="px-4 py-3">{{ formatDateTime(record.registered_at) }}</td>
                    <td class="px-4 py-3">{{ record.qualified_at ? formatDateTime(record.qualified_at) : '-' }}</td>
                  </tr>
                  <tr v-if="inviteRecords.length === 0">
                    <td colspan="6" class="px-4 py-10 text-center text-gray-500">{{ t('campaignRewards.noInvites') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
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

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const home = ref<CampaignHome | null>(null)
const myData = ref<CampaignMyData | null>(null)
const inviteRecords = ref<CampaignInviteRecord[]>([])
const leaderboard = ref<CampaignLeaderboardRow[]>([])

const inviteCode = computed(() => myData.value?.invite_code || '')
const inviteLink = computed(() => {
  const raw = myData.value?.invite_link || ''
  if (!raw) return ''
  if (/^https?:\/\//i.test(raw)) return raw
  if (typeof window === 'undefined') return raw
  return `${window.location.origin}${raw.startsWith('/') ? raw : `/${raw}`}`
})

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

async function loadCampaign(): Promise<void> {
  loading.value = true
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
    loading.value = false
  }
}

async function copyCode(): Promise<void> {
  if (inviteCode.value) await copyToClipboard(inviteCode.value, t('campaignRewards.copied'))
}

async function copyLink(): Promise<void> {
  if (inviteLink.value) await copyToClipboard(inviteLink.value, t('campaignRewards.copied'))
}

function formatCents(value?: number | null): string {
  return `¥${((value || 0) / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatCount(value?: number | null): string {
  return (value || 0).toLocaleString()
}

function formatDateTime(raw?: string | null): string {
  if (!raw) return '-'
  return new Date(raw).toLocaleString()
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

function inviteStatusClass(status: string): string {
  if (status === 'effective') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'invalid') return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

onMounted(() => {
  void loadCampaign()
})
</script>
