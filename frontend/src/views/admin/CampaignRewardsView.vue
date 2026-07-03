<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap justify-end gap-2">
        <button class="btn btn-secondary inline-flex items-center gap-2" type="button" @click="loadCampaigns">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.campaignRewards.refresh') }}
        </button>
        <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="createDraft">
          <Icon name="plus" size="sm" />
          {{ t('admin.campaignRewards.createDefault') }}
        </button>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(280px,0.85fr)_minmax(0,1.4fr)]">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.listTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.listDesc') }}</p>
          </div>
          <div v-if="loading" class="flex min-h-64 items-center justify-center">
            <LoadingSpinner />
          </div>
          <div v-else-if="campaigns.length === 0" class="p-4">
            <EmptyState :title="t('admin.campaignRewards.noCampaigns')" :description="t('admin.campaignRewards.noCampaignsDesc')" />
          </div>
          <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
            <button
              v-for="campaign in campaigns"
              :key="campaign.id"
              class="block w-full px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-dark-800"
              :class="{ 'bg-primary-50 dark:bg-primary-900/20': selectedCampaign?.id === campaign.id }"
              type="button"
              @click="selectCampaign(campaign.id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(campaign.start_at) }} → {{ formatDateTime(campaign.end_at) }}
                  </p>
                </div>
                <span class="rounded-md px-2 py-1 text-xs font-medium" :class="statusClass(campaign.status)">
                  {{ statusLabel(campaign.status) }}
                </span>
              </div>
            </button>
          </div>
        </div>

        <div class="space-y-6">
          <EmptyState
            v-if="!selectedCampaign"
            class="card min-h-80"
            :title="t('admin.campaignRewards.selectTitle')"
            :description="t('admin.campaignRewards.selectDesc')"
          />

          <template v-else>
            <div class="card p-5">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ selectedCampaign.name }}</h1>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedCampaign.description || t('admin.campaignRewards.defaultDescription') }}</p>
                  <div class="mt-3 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span>{{ formatDateTime(selectedCampaign.start_at) }}</span>
                    <span>→</span>
                    <span>{{ formatDateTime(selectedCampaign.end_at) }}</span>
                  </div>
                </div>
                <div class="flex flex-wrap gap-2">
                  <button class="btn btn-secondary" type="button" :disabled="selectedCampaign.status !== 'draft'" @click="publishSelected">
                    {{ t('admin.campaignRewards.publish') }}
                  </button>
                  <button class="btn btn-secondary" type="button" @click="freezeSelected">
                    {{ t('admin.campaignRewards.freeze') }}
                  </button>
                  <button class="btn btn-secondary" type="button" @click="previewRewards">
                    {{ t('admin.campaignRewards.preview') }}
                  </button>
                  <button class="btn btn-primary" type="button" @click="finalizeRewards">
                    {{ t('admin.campaignRewards.finalize') }}
                  </button>
                  <button class="btn btn-primary" type="button" @click="payoutSelected">
                    {{ t('admin.campaignRewards.payout') }}
                  </button>
                </div>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div v-for="metric in poolMetrics" :key="metric.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
                <div class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
              </div>
            </div>

            <div class="grid gap-6 xl:grid-cols-2">
              <div class="card p-4">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.adjustPool') }}</h2>
                <div class="mt-4 grid gap-3 sm:grid-cols-[minmax(120px,0.5fr)_minmax(140px,0.5fr)_minmax(0,1fr)_auto] sm:items-end">
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.adjustType') }}</span>
                    <select v-model="adjustForm.adjustment_type" class="input">
                      <option value="additional_bonus">{{ t('admin.campaignRewards.additionalBonus') }}</option>
                      <option value="manual_compensation">{{ t('admin.campaignRewards.manualCompensation') }}</option>
                      <option value="exception_deduction">{{ t('admin.campaignRewards.manualDeduction') }}</option>
                    </select>
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.amountYuan') }}</span>
                    <input v-model.number="adjustAmountYuan" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
                    <input v-model.trim="adjustForm.reason" class="input" type="text" />
                  </label>
                  <button class="btn btn-secondary" type="button" @click="submitPoolAdjustment">
                    {{ t('common.submit') }}
                  </button>
                </div>
              </div>

              <div class="card p-4">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.versioning') }}</h2>
                <div class="mt-4 grid gap-3 sm:grid-cols-[minmax(130px,0.5fr)_minmax(120px,0.4fr)_minmax(0,1fr)_auto] sm:items-end">
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.thresholdYuan') }}</span>
                    <input v-model.number="versionThresholdYuan" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.injectRate') }}</span>
                    <input v-model.number="versionRate" class="input" type="number" step="0.01" />
                  </label>
                  <label class="space-y-1">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.campaignRewards.reason') }}</span>
                    <input v-model.trim="versionReason" class="input" type="text" />
                  </label>
                  <button class="btn btn-secondary" type="button" @click="submitVersion">
                    {{ t('admin.campaignRewards.createVersion') }}
                  </button>
                </div>
              </div>
            </div>

            <div v-if="calculation" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.rankPool') }}</div>
                <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.rank_pool_cents) }}</div>
              </div>
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.contributionPool') }}</div>
                <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.contribution_pool_cents) }}</div>
              </div>
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.finalPayout') }}</div>
                <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.total_final_payout_cents) }}</div>
              </div>
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="text-xs text-gray-500">{{ t('admin.campaignRewards.withheld') }}</div>
                <div class="mt-2 text-xl font-semibold">{{ formatCents(calculation.total_withheld_cents) }}</div>
              </div>
            </div>

            <div class="card overflow-hidden">
              <div class="border-b border-gray-100 p-4 dark:border-dark-700">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.campaignRewards.leaderboard') }}</h2>
              </div>
              <div class="overflow-x-auto">
                <table class="w-full min-w-[760px] text-sm">
                  <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                    <tr>
                      <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.rank') }}</th>
                      <th class="px-4 py-3 text-left">{{ t('admin.campaignRewards.user') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.validInvites') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.rechargeAmount') }}</th>
                      <th class="px-4 py-3 text-right">{{ t('admin.campaignRewards.estimatedReward') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr v-for="row in leaderboard" :key="row.user_id">
                      <td class="px-4 py-3 font-semibold">#{{ row.rank }}</td>
                      <td class="px-4 py-3">{{ row.username || row.masked_email || row.user_id }}</td>
                      <td class="px-4 py-3 text-right">{{ row.valid_invite_count }}</td>
                      <td class="px-4 py-3 text-right">{{ formatCents(row.invitee_recharge_amount_cents) }}</td>
                      <td class="px-4 py-3 text-right">{{ formatCents(row.estimated_reward_cents) }}</td>
                    </tr>
                    <tr v-if="leaderboard.length === 0">
                      <td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('admin.campaignRewards.noLeaderboard') }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { Campaign, CampaignLeaderboardRow, CampaignPoolSummary } from '@/api/campaigns'
import type { CampaignCalculationSummary } from '@/api/admin/campaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const campaigns = ref<Campaign[]>([])
const selectedCampaign = ref<Campaign | null>(null)
const pool = ref<CampaignPoolSummary | null>(null)
const leaderboard = ref<CampaignLeaderboardRow[]>([])
const calculation = ref<CampaignCalculationSummary | null>(null)
const adjustAmountYuan = ref(0)
const versionThresholdYuan = ref(20)
const versionRate = ref(0.1)
const versionReason = ref('')
const adjustForm = reactive({
  adjustment_type: 'additional_bonus',
  reason: '',
})

const poolMetrics = computed(() => [
  { label: t('admin.campaignRewards.confirmedPool'), value: formatCents(pool.value?.confirmed_pool_cents) },
  { label: t('admin.campaignRewards.pendingPool'), value: formatCents(pool.value?.pending_pool_cents) },
  { label: t('admin.campaignRewards.adjustmentPool'), value: formatCents(pool.value?.adjustment_total_cents) },
  { label: t('admin.campaignRewards.finalPool'), value: formatCents(pool.value?.final_pool_cents) },
])

async function loadCampaigns(): Promise<void> {
  loading.value = true
  try {
    const resp = await adminAPI.campaigns.listCampaigns({ page: 1, page_size: 50 })
    campaigns.value = resp.items
    if (!selectedCampaign.value && resp.items.length > 0) {
      await selectCampaign(resp.items[0].id)
    } else if (selectedCampaign.value) {
      const fresh = resp.items.find(item => item.id === selectedCampaign.value?.id)
      selectedCampaign.value = fresh || selectedCampaign.value
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function selectCampaign(id: number): Promise<void> {
  try {
    const [campaign, poolResp, board] = await Promise.all([
      adminAPI.campaigns.getCampaign(id),
      adminAPI.campaigns.getPoolSummary(id),
      adminAPI.campaigns.getLeaderboard(id),
    ])
    selectedCampaign.value = campaign
    pool.value = poolResp
    leaderboard.value = board.items
    calculation.value = null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.loadFailed')))
  }
}

async function createDraft(): Promise<void> {
  const start = new Date(Date.now() + 60 * 60 * 1000)
  const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
  try {
    const resp = await adminAPI.campaigns.createCampaign({
      name: t('admin.campaignRewards.defaultName'),
      description: t('admin.campaignRewards.defaultDescription'),
      rules_text: t('admin.campaignRewards.defaultRules'),
      start_at: start.toISOString(),
      end_at: end.toISOString(),
      initial_bonus_cents: 0,
      recharge_threshold_cents: 2000,
      allow_accumulated_recharge: true,
      pool_injection_rate: 0.1,
      rank_pool_ratio: 0.8,
      contribution_pool_ratio: 0.2,
      rank_reward_count: 10,
      rank_weights: [30, 20, 15, 10, 8, 6, 4, 3, 2, 2],
      min_payout_amount_cents: 100,
    })
    appStore.showSuccess(t('admin.campaignRewards.created'))
    await loadCampaigns()
    await selectCampaign(resp.campaign.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.createFailed')))
  }
}

async function publishSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    selectedCampaign.value = await adminAPI.campaigns.publishCampaign(selectedCampaign.value!.id)
    await loadCampaigns()
  }, t('admin.campaignRewards.published'))
}

async function freezeSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.freezeLeaderboard(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.frozen'))
}

async function previewRewards(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    calculation.value = await adminAPI.campaigns.recalculateRewards(selectedCampaign.value!.id, 'preview')
  }, t('admin.campaignRewards.previewed'))
}

async function finalizeRewards(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    calculation.value = await adminAPI.campaigns.recalculateRewards(selectedCampaign.value!.id, 'final')
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.finalized'))
}

async function payoutSelected(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.payoutCampaign(selectedCampaign.value!.id)
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.paid'))
}

async function submitPoolAdjustment(): Promise<void> {
  if (!selectedCampaign.value || adjustAmountYuan.value === 0) return
  const sign = adjustForm.adjustment_type === 'exception_deduction' ? -1 : 1
  await runAction(async () => {
    await adminAPI.campaigns.addPoolAdjustment(selectedCampaign.value!.id, {
      adjustment_type: adjustForm.adjustment_type,
      amount_cents: sign * yuanToCents(Math.abs(adjustAmountYuan.value)),
      reason: adjustForm.reason,
    })
    adjustAmountYuan.value = 0
    adjustForm.reason = ''
    await selectCampaign(selectedCampaign.value!.id)
  }, t('admin.campaignRewards.adjusted'))
}

async function submitVersion(): Promise<void> {
  if (!selectedCampaign.value) return
  await runAction(async () => {
    await adminAPI.campaigns.createConfigVersion(selectedCampaign.value!.id, {
      version_scope: 'threshold_adjustment',
      effective_at: new Date().toISOString(),
      recharge_threshold_cents: yuanToCents(versionThresholdYuan.value),
      pool_injection_rate: versionRate.value,
      allow_accumulated_recharge: true,
      change_reason: versionReason.value || t('admin.campaignRewards.versionReasonDefault'),
    })
    versionReason.value = ''
  }, t('admin.campaignRewards.versionCreated'))
}

async function runAction(action: () => Promise<void>, success: string): Promise<void> {
  try {
    await action()
    appStore.showSuccess(success)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.campaignRewards.actionFailed')))
  }
}

function yuanToCents(value: number): number {
  return Math.round((value || 0) * 100)
}

function formatCents(value?: number | null): string {
  return `¥${((value || 0) / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatDateTime(raw?: string | null): string {
  if (!raw) return '-'
  return new Date(raw).toLocaleString()
}

function statusLabel(status: string): string {
  return t(`campaignRewards.statuses.${status}`, status)
}

function statusClass(status: string): string {
  if (status === 'active') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'draft' || status === 'warmup') return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  if (status === 'paid') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

onMounted(() => {
  void loadCampaigns()
})
</script>
