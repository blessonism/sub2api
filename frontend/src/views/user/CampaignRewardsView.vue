<template>
  <AppLayout>
    <div class="space-y-6">
      <div
        v-if="showSwitcher"
        data-testid="activity-switcher"
        class="card overflow-hidden p-2"
      >
        <div class="flex flex-wrap gap-2" :aria-label="t('activities.switcherLabel')">
          <button
            v-for="activity in activities"
            :key="activity.id"
            type="button"
            class="flex min-w-0 flex-1 items-center justify-between gap-3 rounded-xl px-4 py-3 text-left transition"
            :class="activity.id === activeActivity.id
              ? 'bg-primary-600 text-white shadow-sm'
              : 'bg-gray-50 text-gray-700 hover:bg-gray-100 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700'"
            @click="selectedActivityId = activity.id"
          >
            <span class="truncate text-sm font-semibold">{{ activity.title }}</span>
            <span
              class="rounded-full px-2.5 py-1 text-xs font-medium"
              :class="activity.id === activeActivity.id
                ? 'bg-white/15 text-white'
                : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'"
            >
              {{ activity.statusLabel }}
            </span>
          </button>
        </div>
      </div>

      <component :is="activeActivity.component" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, markRaw, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import InviteCampaignActivity from '@/components/user/activities/InviteCampaignActivity.vue'
import LotteryCampaignActivity from '@/components/user/activities/LotteryCampaignActivity.vue'
import campaignsAPI from '@/api/campaigns'
import lotteryCampaignsAPI from '@/api/lotteryCampaigns'

interface ActivityEntry {
  id: string
  title: string
  status: 'active' | 'upcoming' | 'ended'
  statusLabel: string
  component: object
}

const { t } = useI18n()
const hasInviteCampaign = ref(true)
const hasLotteryCampaign = ref(false)

const activities = computed<ActivityEntry[]>(() => {
  const entries: ActivityEntry[] = []
  if (hasInviteCampaign.value || !hasLotteryCampaign.value) {
    entries.push({
      id: 'invite-campaign',
      title: t('activities.inviteCampaignTitle'),
      status: 'active',
      statusLabel: t('activities.activeLabel'),
      component: markRaw(InviteCampaignActivity),
    })
  }
  if (hasLotteryCampaign.value) {
    entries.push({
      id: 'lottery-campaign',
      title: t('activities.lotteryCampaignTitle'),
      status: 'active',
      statusLabel: t('activities.activeLabel'),
      component: markRaw(LotteryCampaignActivity),
    })
  }
  return entries
})

const selectedActivityId = ref(activities.value[0]?.id ?? '')

const activeActivity = computed<ActivityEntry>(() => {
  return activities.value.find(activity => activity.id === selectedActivityId.value) ?? activities.value[0]!
})

const showSwitcher = computed(() => activities.value.length > 1 && Boolean(activeActivity.value))

onMounted(async () => {
  const [inviteResult, lotteryResult] = await Promise.allSettled([
    campaignsAPI.getActiveCampaign(),
    lotteryCampaignsAPI.getActiveLotteryCampaign(),
  ])
  hasInviteCampaign.value = inviteResult.status === 'fulfilled' && Boolean(inviteResult.value.campaign)
  hasLotteryCampaign.value = lotteryResult.status === 'fulfilled' && Boolean(lotteryResult.value.campaign)
  if (!activities.value.some(activity => activity.id === selectedActivityId.value)) {
    selectedActivityId.value = activities.value[0]?.id ?? ''
  }
})
</script>
