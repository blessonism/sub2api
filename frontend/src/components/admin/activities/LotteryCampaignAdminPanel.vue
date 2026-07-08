<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.description') }}</p>
      </div>
      <button class="btn btn-primary inline-flex items-center gap-2" type="button" @click="openCreate">
        <Icon name="plus" size="sm" />
        {{ t('admin.lotteryCampaigns.create') }}
      </button>
    </div>

    <div class="card overflow-hidden">
      <div v-if="loading" class="flex min-h-64 items-center justify-center">
        <LoadingSpinner />
      </div>
      <div v-else-if="campaigns.length === 0" class="p-4">
        <EmptyState :title="t('admin.lotteryCampaigns.emptyTitle')" :description="t('admin.lotteryCampaigns.emptyDescription')" />
      </div>
      <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
        <button
          v-for="campaign in campaigns"
          :key="campaign.id"
          type="button"
          class="block w-full px-4 py-3 text-left transition hover:bg-gray-50 dark:hover:bg-dark-800"
          :class="{ 'bg-primary-50 dark:bg-primary-900/20': selected?.id === campaign.id }"
          @click="selectCampaign(campaign)"
        >
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ formatDateTime(campaign.start_at) }} - {{ formatDateTime(campaign.end_at) }}
              </p>
            </div>
            <span class="rounded-md px-2 py-1 text-xs font-medium" :class="campaign.status === 'published' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">
              {{ t(`admin.lotteryCampaigns.statuses.${campaign.status}`) }}
            </span>
          </div>
        </button>
      </div>
    </div>

    <div v-if="selected" class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(320px,0.8fr)]">
      <div class="card p-5">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ selected.name }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selected.description || t('admin.lotteryCampaigns.defaultDescription') }}</p>
          </div>
          <div class="flex gap-2">
            <button class="btn btn-secondary" type="button" :disabled="selected.status !== 'draft'" @click="publishSelected">{{ t('admin.lotteryCampaigns.publish') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="selected.status === 'cancelled'" @click="cancelSelected">{{ t('admin.lotteryCampaigns.cancel') }}</button>
          </div>
        </div>
        <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="metric in selectedMetrics" :key="metric.label" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ metric.value }}</p>
          </div>
        </div>
        <div class="mt-5 flex flex-wrap gap-2">
          <input v-model="drawDate" class="input max-w-44" type="date" />
          <button class="btn btn-secondary" type="button" :disabled="selected.status !== 'published'" @click="syncSelected">{{ t('admin.lotteryCampaigns.syncEntries') }}</button>
          <button class="btn btn-primary" type="button" :disabled="selected.status !== 'published'" @click="drawSelected">{{ t('admin.lotteryCampaigns.drawNow') }}</button>
        </div>
      </div>

      <div class="card p-5">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.prizes') }}</h3>
        <div class="mt-4 space-y-3">
          <div v-for="tier in selected.prize_tiers ?? []" :key="tier.id" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
            <div class="flex items-center justify-between gap-3">
              <p class="font-medium text-gray-900 dark:text-white">{{ tier.tier_name }}</p>
              <p class="text-sm font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(tier.reward_amount_cents) }}</p>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.winnerCount', { count: tier.winner_count }) }}</p>
          </div>
        </div>
      </div>
    </div>

    <BaseDialog :show="dialogOpen" :title="t('admin.lotteryCampaigns.createDialogTitle')" width="wide" @close="dialogOpen = false">
      <div class="grid gap-4 md:grid-cols-2">
        <label class="space-y-1 md:col-span-2">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.name') }}</span>
          <input v-model.trim="form.name" class="input" type="text" />
        </label>
        <label class="space-y-1 md:col-span-2">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.descriptionLabel') }}</span>
          <textarea v-model.trim="form.description" class="input min-h-20" />
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.participationMode') }}</span>
          <select v-model="form.participation_mode" class="input">
            <option value="auto">{{ t('admin.lotteryCampaigns.auto') }}</option>
            <option value="manual">{{ t('admin.lotteryCampaigns.manual') }}</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.drawSchedule') }}</span>
          <select v-model="form.draw_schedule_type" class="input">
            <option value="single">{{ t('admin.lotteryCampaigns.singleDraw') }}</option>
            <option value="daily">{{ t('admin.lotteryCampaigns.dailyDraw') }}</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.entryMode') }}</span>
          <select v-model="form.entry_mode" class="input">
            <option value="daily_once">{{ t('admin.lotteryCampaigns.dailyOnce') }}</option>
            <option value="stepped">{{ t('admin.lotteryCampaigns.stepped') }}</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.prizeMode') }}</span>
          <select v-model="form.prize_mode" class="input" @change="normalizePrizes">
            <option value="single">{{ t('admin.lotteryCampaigns.singlePrize') }}</option>
            <option value="multi">{{ t('admin.lotteryCampaigns.multiPrize') }}</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.startAt') }}</span>
          <input v-model="form.start_at" class="input" type="datetime-local" />
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.endAt') }}</span>
          <input v-model="form.end_at" class="input" type="datetime-local" />
        </label>
        <label v-if="form.draw_schedule_type === 'single'" class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.drawAt') }}</span>
          <input v-model="form.draw_at" class="input" type="datetime-local" />
        </label>
        <label v-else class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.dailyDrawTime') }}</span>
          <input v-model="form.daily_draw_time" class="input" type="time" />
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.thresholdTokens') }}</span>
          <input v-model.number="form.threshold_tokens" class="input" type="number" min="1" step="1" />
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.entryStepTokens') }}</span>
          <input v-model.number="form.entry_step_tokens" class="input" type="number" min="0" step="1" :disabled="form.entry_mode !== 'stepped'" />
        </label>
        <label class="space-y-1">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.maxEntries') }}</span>
          <input v-model.number="form.max_entries_per_user" class="input" type="number" min="1" step="1" :disabled="form.entry_mode !== 'stepped'" />
        </label>
      </div>

      <div class="mt-5 space-y-3">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.prizes') }}</h3>
          <button v-if="form.prize_mode === 'multi'" class="btn btn-secondary btn-sm" type="button" @click="addPrize">{{ t('admin.lotteryCampaigns.addPrize') }}</button>
        </div>
        <div v-for="(tier, index) in form.prize_tiers" :key="index" class="grid gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700 md:grid-cols-4">
          <input v-model.trim="tier.tier_name" class="input" type="text" :placeholder="t('admin.lotteryCampaigns.prizeName')" />
          <input v-model.number="tier.winner_count" class="input" type="number" min="1" step="1" />
          <input v-model.number="tier.reward_amount_cents" class="input" type="number" min="1" step="1" />
          <button class="btn btn-secondary" type="button" :disabled="form.prize_tiers.length <= 1" @click="removePrize(index)">{{ t('common.delete') }}</button>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" @click="dialogOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="button" :disabled="submitting" @click="submitCreate">{{ submitting ? t('common.processing') : t('admin.lotteryCampaigns.create') }}</button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { LotteryCampaign } from '@/api/lotteryCampaigns'
import type { LotteryCampaignRequest } from '@/api/admin/lotteryCampaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const dialogOpen = ref(false)
const campaigns = ref<LotteryCampaign[]>([])
const selected = ref<LotteryCampaign | null>(null)
const drawDate = ref(new Date().toISOString().slice(0, 10))

const form = reactive<LotteryCampaignRequest>({
  name: '',
  description: '',
  rules_text: '',
  participation_mode: 'auto',
  draw_schedule_type: 'single',
  prize_mode: 'single',
  entry_mode: 'daily_once',
  threshold_tokens: 100000,
  entry_step_tokens: 100000,
  max_entries_per_user: 1,
  start_at: '',
  end_at: '',
  draw_at: '',
  daily_draw_time: '20:00',
  prize_tiers: [{ tier_name: '一等奖', winner_count: 1, reward_amount_cents: 1000, sort_order: 1 }],
})

const selectedMetrics = computed(() => {
  if (!selected.value) return []
  return [
    { label: t('admin.lotteryCampaigns.participationMode'), value: t(`admin.lotteryCampaigns.${selected.value.participation_mode}`) },
    { label: t('admin.lotteryCampaigns.drawSchedule'), value: t(`admin.lotteryCampaigns.${selected.value.draw_schedule_type}Draw`) },
    { label: t('admin.lotteryCampaigns.thresholdTokens'), value: formatTokens(selected.value.threshold_tokens) },
    { label: t('admin.lotteryCampaigns.entryMode'), value: t(`admin.lotteryCampaigns.${selected.value.entry_mode}`) },
  ]
})

async function load(): Promise<void> {
  loading.value = true
  try {
    const resp = await adminAPI.lotteryCampaigns.listLotteryCampaigns()
    campaigns.value = resp.items
    selected.value = selected.value ? campaigns.value.find(item => item.id === selected.value?.id) ?? campaigns.value[0] ?? null : campaigns.value[0] ?? null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.loadFailed')))
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  const start = new Date(Date.now() + 60 * 60 * 1000)
  const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
  form.name = t('admin.lotteryCampaigns.defaultName')
  form.description = t('admin.lotteryCampaigns.defaultDescription')
  form.start_at = toDateTimeLocal(start)
  form.end_at = toDateTimeLocal(end)
  form.draw_at = toDateTimeLocal(new Date(start.getTime() + 24 * 60 * 60 * 1000))
  dialogOpen.value = true
}

function selectCampaign(campaign: LotteryCampaign): void {
  selected.value = campaign
}

async function submitCreate(): Promise<void> {
  submitting.value = true
  try {
    const payload = buildPayload()
    const created = await adminAPI.lotteryCampaigns.createLotteryCampaign(payload)
    appStore.showSuccess(t('admin.lotteryCampaigns.created'))
    dialogOpen.value = false
    await load()
    selected.value = campaigns.value.find(item => item.id === created.id) ?? created
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.createFailed')))
  } finally {
    submitting.value = false
  }
}

async function publishSelected(): Promise<void> {
  if (!selected.value) return
  try {
    selected.value = await adminAPI.lotteryCampaigns.publishLotteryCampaign(selected.value.id)
    await load()
    appStore.showSuccess(t('admin.lotteryCampaigns.published'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.actionFailed')))
  }
}

async function cancelSelected(): Promise<void> {
  if (!selected.value) return
  try {
    selected.value = await adminAPI.lotteryCampaigns.cancelLotteryCampaign(selected.value.id)
    await load()
    appStore.showSuccess(t('admin.lotteryCampaigns.cancelled'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.actionFailed')))
  }
}

async function syncSelected(): Promise<void> {
  if (!selected.value) return
  try {
    const resp = await adminAPI.lotteryCampaigns.syncLotteryEntries(selected.value.id, drawDate.value)
    appStore.showSuccess(t('admin.lotteryCampaigns.synced', { count: resp.synced }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.actionFailed')))
  }
}

async function drawSelected(): Promise<void> {
  if (!selected.value) return
  try {
    await adminAPI.lotteryCampaigns.drawLotteryCampaign(selected.value.id, drawDate.value)
    appStore.showSuccess(t('admin.lotteryCampaigns.drawn'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.actionFailed')))
  }
}

function buildPayload(): LotteryCampaignRequest {
  return {
    ...form,
    draw_at: form.draw_schedule_type === 'single' && form.draw_at ? new Date(form.draw_at).toISOString() : null,
    daily_draw_time: form.draw_schedule_type === 'daily' ? form.daily_draw_time : '',
    start_at: new Date(form.start_at).toISOString(),
    end_at: new Date(form.end_at).toISOString(),
    max_entries_per_user: form.entry_mode === 'daily_once' ? 1 : form.max_entries_per_user,
    entry_step_tokens: form.entry_mode === 'daily_once' ? 0 : form.entry_step_tokens,
    prize_tiers: form.prize_tiers.map((tier, index) => ({ ...tier, sort_order: index + 1 })),
  }
}

function normalizePrizes(): void {
  if (form.prize_mode === 'single') {
    form.prize_tiers.splice(1)
  }
}

function addPrize(): void {
  form.prize_tiers.push({ tier_name: t('admin.lotteryCampaigns.prizeName'), winner_count: 1, reward_amount_cents: 500, sort_order: form.prize_tiers.length + 1 })
}

function removePrize(index: number): void {
  form.prize_tiers.splice(index, 1)
}

function toDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatTokens(value: number): string {
  return new Intl.NumberFormat(locale.value).format(value)
}

function formatCents(cents: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY' }).format(cents / 100)
}

onMounted(() => {
  void load()
})
</script>
