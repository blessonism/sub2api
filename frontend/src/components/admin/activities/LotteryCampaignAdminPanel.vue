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
              <div class="flex flex-wrap items-center gap-2">
                <p class="font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</p>
                <span v-if="campaign.is_featured" class="rounded-md bg-primary-100 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-200">
                  {{ t('admin.lotteryCampaigns.featuredBadge') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ formatDateTime(campaign.start_at) }} - {{ formatDateTime(campaign.end_at) }}
              </p>
              <div class="mt-2 flex flex-wrap gap-1.5">
                <span
                  v-for="tag in campaignTags(campaign)"
                  :key="tag"
                  class="rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                >
                  {{ tag }}
                </span>
              </div>
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
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" type="button" @click="openEdit(selected)">{{ t('admin.lotteryCampaigns.edit') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="selected.status !== 'draft'" @click="publishSelected">{{ t('admin.lotteryCampaigns.publish') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="selected.status === 'cancelled'" @click="cancelSelected">{{ t('admin.lotteryCampaigns.cancel') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="!canFeatureSelected" @click="featureSelected">{{ t('admin.lotteryCampaigns.setFeatured') }}</button>
          </div>
        </div>
        <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="metric in selectedMetrics" :key="metric.label" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ metric.value }}</p>
            <p v-if="metric.subValue" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ metric.subValue }}</p>
          </div>
        </div>
        <div class="mt-5 flex flex-wrap gap-2">
          <input v-model="drawDate" class="input max-w-44" type="date" />
          <button class="btn btn-secondary" type="button" :disabled="selected.status !== 'published'" @click="syncSelected">{{ t('admin.lotteryCampaigns.syncEntries') }}</button>
          <button class="btn btn-secondary" type="button" :disabled="selected.status !== 'published'" @click="openDesignate">{{ t('admin.lotteryCampaigns.designateWinners') }}</button>
          <button class="btn btn-primary" type="button" :disabled="selected.status !== 'published'" @click="drawSelected">{{ t('admin.lotteryCampaigns.drawNow') }}</button>
        </div>
        <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.dangerZone') }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.deleteHint') }}</p>
            </div>
            <button class="btn border-red-200 bg-red-50 text-red-700 hover:bg-red-100 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-200" type="button" @click="openDelete(selected)">
              {{ t('admin.lotteryCampaigns.delete') }}
            </button>
          </div>
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

    <BaseDialog :show="dialogOpen" :title="dialogTitle" width="wide" @close="dialogOpen = false">
      <div class="space-y-5">
        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.basicInformation') }}</h3>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.name') }}</span>
              <input v-model.trim="form.name" class="input" type="text" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.descriptionLabel') }}</span>
              <textarea v-model.trim="form.description" class="input min-h-20" />
            </label>
            <label class="space-y-1 md:col-span-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.rulesText') }}</span>
              <textarea v-model.trim="form.rules_text" class="input min-h-20" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.participationMode') }}</span>
              <select v-model="form.participation_mode" class="input">
                <option value="auto">{{ t('admin.lotteryCampaigns.auto') }}</option>
                <option value="manual">{{ t('admin.lotteryCampaigns.manual') }}</option>
              </select>
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.prizeMode') }}</span>
              <select v-model="form.prize_mode" class="input" @change="normalizePrizes">
                <option value="single">{{ t('admin.lotteryCampaigns.singlePrize') }}</option>
                <option value="multi">{{ t('admin.lotteryCampaigns.multiPrize') }}</option>
              </select>
            </label>
          </div>
        </section>

        <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.thresholdConfiguration') }}</h3>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.thresholdTokenMillions') }}</span>
              <span class="relative block">
                <input v-model.number="form.threshold_token_millions" class="input pr-12" type="number" min="0.01" step="0.01" inputmode="decimal" />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm font-medium text-gray-400">{{ t('admin.lotteryCampaigns.millionUnit') }}</span>
              </span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ thresholdRawTokenHint }}</span>
            </label>
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.entryMode') }}</span>
              <select v-model="form.entry_mode" class="input">
                <option value="daily_once">{{ t('admin.lotteryCampaigns.dailyOnce') }}</option>
                <option value="stepped">{{ t('admin.lotteryCampaigns.stepped') }}</option>
              </select>
            </label>
            <label v-if="form.entry_mode === 'stepped'" class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.entryStepTokenMillions') }}</span>
              <span class="relative block">
                <input v-model.number="form.entry_step_token_millions" class="input pr-12" type="number" min="0.01" step="0.01" inputmode="decimal" />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm font-medium text-gray-400">{{ t('admin.lotteryCampaigns.millionUnit') }}</span>
              </span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ entryStepRawTokenHint }}</span>
            </label>
            <label v-if="form.entry_mode === 'stepped'" class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.maxEntries') }}</span>
              <input v-model.number="form.max_entries_per_user" class="input" type="number" min="1" step="1" />
            </label>
          </div>
          <div class="rounded-lg border border-primary-100 bg-primary-50/60 p-3 text-sm text-primary-800 dark:border-primary-900/40 dark:bg-primary-900/20 dark:text-primary-100">
            <p class="font-medium">{{ t('admin.lotteryCampaigns.rulePreviewTitle') }}</p>
            <p class="mt-1">{{ rulePreviewText }}</p>
          </div>
        </section>

        <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.drawConfiguration') }}</h3>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.drawSchedule') }}</span>
              <select v-model="form.draw_schedule_type" class="input">
                <option value="single">{{ t('admin.lotteryCampaigns.singleDraw') }}</option>
                <option value="daily">{{ t('admin.lotteryCampaigns.dailyDraw') }}</option>
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
          </div>
        </section>

        <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.lotteryCampaigns.prizeConfiguration') }}</h3>
            <button v-if="form.prize_mode === 'multi'" class="btn btn-secondary btn-sm" type="button" @click="addPrize">{{ t('admin.lotteryCampaigns.addPrize') }}</button>
          </div>
          <div class="hidden grid-cols-[minmax(0,1fr)_7rem_8rem_6rem] gap-3 px-1 text-xs font-medium text-gray-500 dark:text-dark-400 md:grid">
            <span>{{ t('admin.lotteryCampaigns.prizeName') }}</span>
            <span>{{ t('admin.lotteryCampaigns.winnerCountLabel') }}</span>
            <span>{{ t('admin.lotteryCampaigns.rewardAmount') }}</span>
            <span>{{ t('admin.lotteryCampaigns.operation') }}</span>
          </div>
          <div v-for="(tier, index) in form.prize_tiers" :key="index" class="grid gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700 md:grid-cols-[minmax(0,1fr)_7rem_8rem_6rem]">
            <input v-model.trim="tier.tier_name" class="input" type="text" :aria-label="t('admin.lotteryCampaigns.prizeName')" :placeholder="t('admin.lotteryCampaigns.prizeName')" />
            <input v-model.number="tier.winner_count" class="input" type="number" min="1" step="1" :aria-label="t('admin.lotteryCampaigns.winnerCountLabel')" />
            <input v-model.number="tier.reward_amount_cents" class="input" type="number" min="1" step="1" :aria-label="t('admin.lotteryCampaigns.rewardAmount')" />
            <button class="btn btn-secondary" type="button" :disabled="form.prize_tiers.length <= 1" @click="removePrize(index)">{{ t('common.delete') }}</button>
          </div>
        </section>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" @click="dialogOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="button" :disabled="submitting" @click="submitSave">{{ submitting ? t('common.processing') : submitText }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="designateOpen" :title="t('admin.lotteryCampaigns.designateTitle', { date: drawDate })" width="wide" @close="designateOpen = false">
      <div class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.lotteryCampaigns.designateDescription') }}</p>
        <div v-if="designateLocked" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200">
          {{ t('admin.lotteryCampaigns.designateLocked') }}
        </div>
        <div v-if="designateTiers.length" class="flex flex-wrap gap-2">
          <span
            v-for="usage in designateTierUsage"
            :key="usage.id"
            class="rounded-md px-2 py-1 text-xs font-medium"
            :class="usage.used > usage.total ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'"
          >
            {{ t('admin.lotteryCampaigns.designateTierUsage', { name: usage.name, used: usage.used, total: usage.total }) }}
          </span>
        </div>
        <div v-if="designateLoading" class="flex min-h-40 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="designateCandidates.length === 0" class="rounded-lg border border-gray-100 p-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
          {{ t('admin.lotteryCampaigns.designateNoCandidates') }}
          <p class="mt-1 text-xs">{{ t('admin.lotteryCampaigns.designateHint') }}</p>
        </div>
        <div v-else class="overflow-hidden rounded-lg border border-gray-100 dark:border-dark-700">
          <div class="grid grid-cols-[minmax(0,1fr)_6rem_minmax(0,1fr)] gap-3 bg-gray-50 px-3 py-2 text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            <span>{{ t('admin.lotteryCampaigns.designateColumnUser') }}</span>
            <span>{{ t('admin.lotteryCampaigns.designateColumnEntries') }}</span>
            <span>{{ t('admin.lotteryCampaigns.designateColumnTier') }}</span>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="cand in designateCandidates" :key="cand.user_id" class="grid grid-cols-[minmax(0,1fr)_6rem_minmax(0,1fr)] items-center gap-3 px-3 py-2">
              <span class="truncate text-sm text-gray-900 dark:text-white">{{ cand.masked_email || `#${cand.user_id}` }}</span>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ cand.entry_count }}</span>
              <select v-model.number="designateSelection[cand.user_id]" class="input" :disabled="designateLocked">
                <option :value="0">{{ t('admin.lotteryCampaigns.designateNone') }}</option>
                <option v-for="tier in designateTiers" :key="tier.id" :value="tier.id">{{ tier.tier_name }}</option>
              </select>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" type="button" @click="designateOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="button" :disabled="designateLocked || designateSaving || designateLoading" @click="saveDesignations">
            {{ designateSaving ? t('common.processing') : t('admin.lotteryCampaigns.designateSave') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="!!deletingCampaign"
      :title="t('admin.lotteryCampaigns.confirmDeleteTitle')"
      :message="t('admin.lotteryCampaigns.confirmDeleteMessage')"
      :confirm-text="t('admin.lotteryCampaigns.confirmDeleteAction')"
      danger
      @confirm="deleteSelected"
      @cancel="deletingCampaign = null"
    >
      <div v-if="deletingCampaign" class="rounded-lg border border-red-100 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-200">
        {{ t('admin.lotteryCampaigns.deleteCascadeWarning', { name: deletingCampaign.name }) }}
      </div>
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { adminAPI } from '@/api/admin'
import type { LotteryCampaign } from '@/api/lotteryCampaigns'
import type { LotteryCampaignRequest, LotteryDesignationCandidate } from '@/api/admin/lotteryCampaigns'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { TOKENS_PER_MILLION, formatTokenMillions } from '@/utils/usagePricing'

type LotteryCampaignFormState = Omit<LotteryCampaignRequest, 'threshold_tokens' | 'entry_step_tokens'> & {
  threshold_token_millions: number
  entry_step_token_millions: number
}

interface SelectedMetric {
  label: string
  value: string
  subValue?: string
}

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const dialogOpen = ref(false)
const campaigns = ref<LotteryCampaign[]>([])
const selected = ref<LotteryCampaign | null>(null)
const editingCampaign = ref<LotteryCampaign | null>(null)
const deletingCampaign = ref<LotteryCampaign | null>(null)
const drawDate = ref(new Date().toISOString().slice(0, 10))

const designateOpen = ref(false)
const designateLoading = ref(false)
const designateSaving = ref(false)
const designateLocked = ref(false)
const designateCandidates = ref<LotteryDesignationCandidate[]>([])
const designateSelection = reactive<Record<number, number>>({})

const form = reactive<LotteryCampaignFormState>({
  name: '',
  description: '',
  rules_text: '',
  participation_mode: 'auto',
  draw_schedule_type: 'single',
  prize_mode: 'single',
  entry_mode: 'daily_once',
  threshold_token_millions: 0.1,
  entry_step_token_millions: 0.1,
  max_entries_per_user: 1,
  start_at: '',
  end_at: '',
  draw_at: '',
  daily_draw_time: '20:00',
  prize_tiers: [{ tier_name: '一等奖', winner_count: 1, reward_amount_cents: 1000, sort_order: 1 }],
})

const dialogTitle = computed(() => editingCampaign.value ? t('admin.lotteryCampaigns.editDialogTitle') : t('admin.lotteryCampaigns.createDialogTitle'))
const submitText = computed(() => editingCampaign.value ? t('admin.lotteryCampaigns.saveChanges') : t('admin.lotteryCampaigns.create'))
const canFeatureSelected = computed(() => {
  if (!selected.value || selected.value.is_featured) return false
  return selected.value.status === 'published' && isCampaignInWindow(selected.value)
})
const thresholdRawTokenHint = computed(() => rawTokenEquivalent(form.threshold_token_millions))
const entryStepRawTokenHint = computed(() => rawTokenEquivalent(form.entry_step_token_millions))
const rulePreviewText = computed(() => {
  const threshold = formatTokenMillions(tokenMillionsToTokensForDisplay(form.threshold_token_millions))
  if (form.entry_mode === 'stepped') {
    return t(form.draw_schedule_type === 'single' ? 'admin.lotteryCampaigns.oneTimeSteppedRulePreview' : 'admin.lotteryCampaigns.steppedRulePreview', {
      threshold,
      step: formatTokenMillions(tokenMillionsToTokensForDisplay(form.entry_step_token_millions)),
      max: form.max_entries_per_user,
    })
  }
  return t(form.draw_schedule_type === 'single' ? 'admin.lotteryCampaigns.oneTimeOnceRulePreview' : 'admin.lotteryCampaigns.dailyOnceRulePreview', { threshold })
})

const selectedMetrics = computed<SelectedMetric[]>(() => {
  if (!selected.value) return []
  return [
    { label: t('admin.lotteryCampaigns.participationMode'), value: t(`admin.lotteryCampaigns.${selected.value.participation_mode}`) },
    { label: t('admin.lotteryCampaigns.drawSchedule'), value: t(`admin.lotteryCampaigns.${selected.value.draw_schedule_type}Draw`) },
    {
      label: t('admin.lotteryCampaigns.thresholdTokens'),
      value: formatTokens(selected.value.threshold_tokens),
      subValue: rawTokenText(selected.value.threshold_tokens),
    },
    { label: t('admin.lotteryCampaigns.entryMode'), value: t(`admin.lotteryCampaigns.${selected.value.entry_mode}`) },
  ]
})

const designateTiers = computed(() => selected.value?.prize_tiers ?? [])

const designateTierUsage = computed(() =>
  designateTiers.value.map(tier => ({
    id: tier.id,
    name: tier.tier_name,
    total: tier.winner_count,
    used: Object.values(designateSelection).filter(tierId => tierId === tier.id).length,
  })),
)

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
  editingCampaign.value = null
  resetForm()
  form.name = t('admin.lotteryCampaigns.defaultName')
  form.description = t('admin.lotteryCampaigns.defaultDescription')
  form.start_at = toDateTimeLocal(start)
  form.end_at = toDateTimeLocal(end)
  form.draw_at = toDateTimeLocal(new Date(start.getTime() + 24 * 60 * 60 * 1000))
  dialogOpen.value = true
}

function openEdit(campaign: LotteryCampaign): void {
  editingCampaign.value = campaign
  fillForm(campaign)
  dialogOpen.value = true
}

function openDelete(campaign: LotteryCampaign): void {
  deletingCampaign.value = campaign
}

function selectCampaign(campaign: LotteryCampaign): void {
  selected.value = campaign
}

function campaignTags(campaign: LotteryCampaign): string[] {
  const tags = [
    t('admin.lotteryCampaigns.thresholdTag', { threshold: formatTokens(campaign.threshold_tokens) }),
    t(`admin.lotteryCampaigns.${campaign.draw_schedule_type}Draw`),
    t(`admin.lotteryCampaigns.${campaign.prize_mode}Prize`),
  ]
  if (campaign.is_featured) {
    tags.push(t('admin.lotteryCampaigns.featuredBadge'))
  }
  return tags
}

async function submitSave(): Promise<void> {
  if (!validateForm()) return
  submitting.value = true
  try {
    const payload = buildPayload()
    const saved = editingCampaign.value
      ? await adminAPI.lotteryCampaigns.updateLotteryCampaign(editingCampaign.value.id, payload)
      : await adminAPI.lotteryCampaigns.createLotteryCampaign(payload)
    appStore.showSuccess(editingCampaign.value ? t('admin.lotteryCampaigns.updated') : t('admin.lotteryCampaigns.created'))
    dialogOpen.value = false
    await load()
    selected.value = campaigns.value.find(item => item.id === saved.id) ?? saved
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, editingCampaign.value ? t('admin.lotteryCampaigns.updateFailed') : t('admin.lotteryCampaigns.createFailed')))
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

async function featureSelected(): Promise<void> {
  if (!selected.value) return
  try {
    selected.value = await adminAPI.lotteryCampaigns.featureLotteryCampaign(selected.value.id)
    await load()
    appStore.showSuccess(t('admin.lotteryCampaigns.featured'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.featureFailed')))
  }
}

async function deleteSelected(): Promise<void> {
  if (!deletingCampaign.value) return
  const id = deletingCampaign.value.id
  try {
    await adminAPI.lotteryCampaigns.deleteLotteryCampaign(id)
    deletingCampaign.value = null
    if (selected.value?.id === id) {
      selected.value = null
    }
    await load()
    appStore.showSuccess(t('admin.lotteryCampaigns.deleted'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.deleteFailed')))
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

async function openDesignate(): Promise<void> {
  if (!selected.value) return
  designateOpen.value = true
  designateLoading.value = true
  designateLocked.value = false
  designateCandidates.value = []
  for (const key of Object.keys(designateSelection)) delete designateSelection[Number(key)]
  try {
    const view = await adminAPI.lotteryCampaigns.getLotteryDesignations(selected.value.id, drawDate.value)
    designateCandidates.value = view.candidates
    designateLocked.value = view.locked
    for (const cand of view.candidates) {
      designateSelection[cand.user_id] = cand.designated_tier_id ?? 0
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.designateLoadFailed')))
    designateOpen.value = false
  } finally {
    designateLoading.value = false
  }
}

async function saveDesignations(): Promise<void> {
  if (!selected.value || designateLocked.value) return
  const overflow = designateTierUsage.value.find(usage => usage.used > usage.total)
  if (overflow) {
    appStore.showError(t('admin.lotteryCampaigns.designateTierOverflowError', { name: overflow.name }))
    return
  }
  const assignments = Object.entries(designateSelection)
    .filter(([, tierID]) => tierID > 0)
    .map(([userID, tierID]) => ({ user_id: Number(userID), prize_tier_id: tierID }))
  designateSaving.value = true
  try {
    const resp = await adminAPI.lotteryCampaigns.replaceLotteryDesignations(selected.value.id, drawDate.value, assignments)
    appStore.showSuccess(t('admin.lotteryCampaigns.designateSaved', { count: resp.saved }))
    designateOpen.value = false
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.lotteryCampaigns.designateSaveFailed')))
  } finally {
    designateSaving.value = false
  }
}

function buildPayload(): LotteryCampaignRequest {
  const { threshold_token_millions, entry_step_token_millions, ...rest } = form
  return {
    ...rest,
    threshold_tokens: tokenMillionsToTokens(threshold_token_millions),
    draw_at: form.draw_schedule_type === 'single' && form.draw_at ? new Date(form.draw_at).toISOString() : null,
    daily_draw_time: form.draw_schedule_type === 'daily' ? form.daily_draw_time : '',
    start_at: new Date(form.start_at).toISOString(),
    end_at: new Date(form.end_at).toISOString(),
    max_entries_per_user: form.entry_mode === 'daily_once' ? 1 : form.max_entries_per_user,
    entry_step_tokens: form.entry_mode === 'daily_once' ? 0 : tokenMillionsToTokens(entry_step_token_millions),
    prize_tiers: form.prize_tiers.map((tier, index) => ({ ...tier, sort_order: index + 1 })),
  }
}

function validateForm(): boolean {
  const start = new Date(form.start_at)
  const end = new Date(form.end_at)
  if (!form.start_at || !form.end_at || Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) {
    appStore.showError(t('admin.lotteryCampaigns.invalidTimeOrder'))
    return false
  }
  if (form.draw_schedule_type === 'single') {
    const draw = new Date(form.draw_at || '')
    if (!form.draw_at || Number.isNaN(draw.getTime()) || draw < start || draw > end) {
      appStore.showError(t('admin.lotteryCampaigns.invalidDrawTime'))
      return false
    }
  }
  if (form.draw_schedule_type === 'daily' && !dailyDrawFallsInWindow(start, end, form.daily_draw_time || '')) {
    appStore.showError(t('admin.lotteryCampaigns.invalidDailyDrawTime'))
    return false
  }
  if (!isPositiveFiniteNumber(form.threshold_token_millions)) {
    appStore.showError(t('admin.lotteryCampaigns.invalidThresholdTokens'))
    return false
  }
  if (form.entry_mode === 'stepped' && !isPositiveFiniteNumber(form.entry_step_token_millions)) {
    appStore.showError(t('admin.lotteryCampaigns.invalidEntryStepTokens'))
    return false
  }
  return true
}

function resetForm(): void {
  form.name = ''
  form.description = ''
  form.rules_text = ''
  form.participation_mode = 'auto'
  form.draw_schedule_type = 'single'
  form.prize_mode = 'single'
  form.entry_mode = 'daily_once'
  form.threshold_token_millions = 0.1
  form.entry_step_token_millions = 0.1
  form.max_entries_per_user = 1
  form.start_at = ''
  form.end_at = ''
  form.draw_at = ''
  form.daily_draw_time = '20:00'
  form.prize_tiers = [{ tier_name: t('admin.lotteryCampaigns.prizeName'), winner_count: 1, reward_amount_cents: 1000, sort_order: 1 }]
}

function fillForm(campaign: LotteryCampaign): void {
  form.name = campaign.name
  form.description = campaign.description
  form.rules_text = campaign.rules_text
  form.participation_mode = campaign.participation_mode
  form.draw_schedule_type = campaign.draw_schedule_type
  form.prize_mode = campaign.prize_mode
  form.entry_mode = campaign.entry_mode
  form.threshold_token_millions = tokensToMillions(campaign.threshold_tokens)
  form.entry_step_token_millions = tokensToMillions(campaign.entry_step_tokens)
  form.max_entries_per_user = campaign.max_entries_per_user
  form.start_at = toDateTimeLocal(new Date(campaign.start_at))
  form.end_at = toDateTimeLocal(new Date(campaign.end_at))
  form.draw_at = campaign.draw_at ? toDateTimeLocal(new Date(campaign.draw_at)) : ''
  form.daily_draw_time = campaign.daily_draw_time || '20:00'
  form.prize_tiers = (campaign.prize_tiers ?? []).map((tier, index) => ({
    tier_name: tier.tier_name,
    winner_count: tier.winner_count,
    reward_amount_cents: tier.reward_amount_cents,
    sort_order: tier.sort_order || index + 1,
  }))
  if (form.prize_tiers.length === 0) {
    form.prize_tiers = [{ tier_name: t('admin.lotteryCampaigns.prizeName'), winner_count: 1, reward_amount_cents: 1000, sort_order: 1 }]
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
  return formatTokenMillions(value)
}

function formatRawTokens(value: number): string {
  return new Intl.NumberFormat(locale.value).format(value)
}

function rawTokenText(value: number): string {
  return t('admin.lotteryCampaigns.rawTokens', { tokens: formatRawTokens(value) })
}

function rawTokenEquivalent(value: number): string {
  return t('admin.lotteryCampaigns.rawTokenEquivalent', { tokens: formatRawTokens(tokenMillionsToTokensForDisplay(value)) })
}

function formatCents(cents: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY' }).format(cents / 100)
}

function isCampaignInWindow(campaign: LotteryCampaign): boolean {
  const now = Date.now()
  return new Date(campaign.start_at).getTime() <= now && new Date(campaign.end_at).getTime() >= now
}

function dailyDrawFallsInWindow(start: Date, end: Date, dailyTime: string): boolean {
  const [hourRaw, minuteRaw] = dailyTime.split(':')
  const hour = Number(hourRaw)
  const minute = Number(minuteRaw)
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) return false
  const cursor = new Date(start)
  cursor.setHours(0, 0, 0, 0)
  const endDay = new Date(end)
  endDay.setHours(0, 0, 0, 0)
  while (cursor <= endDay) {
    const scheduled = new Date(cursor)
    scheduled.setHours(hour, minute, 0, 0)
    if (scheduled >= start && scheduled <= end) return true
    cursor.setDate(cursor.getDate() + 1)
  }
  return false
}

function tokensToMillions(tokens: number): number {
  return Number((tokens / TOKENS_PER_MILLION).toFixed(2))
}

function tokenMillionsToTokens(value: number): number {
  return Math.round(value * TOKENS_PER_MILLION)
}

function tokenMillionsToTokensForDisplay(value: number): number {
  return isPositiveFiniteNumber(value) ? tokenMillionsToTokens(value) : 0
}

function isPositiveFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

onMounted(() => {
  void load()
})
</script>
