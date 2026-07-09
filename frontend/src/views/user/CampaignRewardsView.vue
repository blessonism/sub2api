<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="initialLoading" data-testid="activity-loading" class="space-y-6">
        <div class="card h-20 animate-pulse bg-gray-100 dark:bg-dark-800" />
        <div class="card min-h-80 animate-pulse bg-gray-100 dark:bg-dark-800" />
      </div>

      <template v-else>
        <div
          v-if="showSwitcher"
          data-testid="activity-switcher"
          class="card overflow-hidden p-2"
        >
          <div
            ref="tablistRef"
            role="tablist"
            :aria-label="t('activities.switcherLabel')"
            class="flex flex-wrap gap-2"
            @keydown="onTabKeydown"
          >
            <button
              v-for="activity in activities"
              :id="`activity-tab-${activity.id}`"
              :key="activity.id"
              type="button"
              role="tab"
              :aria-selected="activity.id === activeActivity.id"
              :aria-controls="`activity-panel-${activity.id}`"
              :tabindex="activity.id === activeActivity.id ? 0 : -1"
              class="flex min-w-0 items-center gap-2.5 rounded-xl px-4 py-3 text-left transition"
              :class="activity.id === activeActivity.id
                ? 'bg-primary-600 text-white shadow-sm'
                : 'bg-gray-50 text-gray-700 hover:bg-gray-100 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700'"
              @click="selectActivity(activity.id)"
            >
              <Icon
                :name="activity.icon"
                size="sm"
                :class="activity.id === activeActivity.id ? 'text-white' : 'text-primary-500'"
              />
              <span class="truncate text-sm font-semibold">{{ activity.title }}</span>
              <span
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                :class="activity.id === activeActivity.id
                  ? 'bg-white/15 text-white'
                  : TONE_BADGE[activity.tone]"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="[
                    TONE_DOT[activity.tone],
                    activity.tone === 'live' ? 'motion-safe:animate-pulse' : '',
                  ]"
                />
                {{ activity.statusLabel }}
              </span>
            </button>
          </div>
        </div>

        <Transition name="activity-fade" mode="out-in">
          <component
            :is="activeActivity.component"
            v-if="activeActivity"
            :id="`activity-panel-${activeActivity.id}`"
            :key="activeActivity.id"
            role="tabpanel"
            :aria-labelledby="showSwitcher ? `activity-tab-${activeActivity.id}` : undefined"
          />
        </Transition>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, markRaw, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import InviteCampaignActivity from '@/components/user/activities/InviteCampaignActivity.vue'
import LotteryCampaignActivity from '@/components/user/activities/LotteryCampaignActivity.vue'
import campaignsAPI from '@/api/campaigns'
import lotteryCampaignsAPI from '@/api/lotteryCampaigns'
import type { Campaign } from '@/api/campaigns'
import type { LotteryCampaign } from '@/api/lotteryCampaigns'

type ActivityTone = 'live' | 'upcoming' | 'settling' | 'paused' | 'ended' | 'neutral'

interface ActivityEntry {
  id: string
  title: string
  icon: 'users' | 'sparkles'
  tone: ActivityTone
  statusLabel: string
  component: object
}

const TONE_BADGE: Record<ActivityTone, string> = {
  live: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  upcoming: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  settling: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300',
  paused: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
  ended: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300',
  neutral: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300',
}

const TONE_DOT: Record<ActivityTone, string> = {
  live: 'bg-emerald-500',
  upcoming: 'bg-amber-500',
  settling: 'bg-sky-500',
  paused: 'bg-orange-500',
  ended: 'bg-gray-400',
  neutral: 'bg-gray-400',
}
const INVITE_TONE: Record<string, ActivityTone> = {
  active: 'live',
  warmup: 'upcoming',
  frozen: 'settling',
  auditing: 'settling',
  publicizing: 'settling',
  pending_payout: 'settling',
  paused: 'paused',
  paid: 'ended',
  cancelled: 'ended',
  terminated: 'ended',
  draft: 'neutral',
}

const LOTTERY_TONE: Record<string, ActivityTone> = {
  published: 'live',
  cancelled: 'ended',
  archived: 'ended',
  draft: 'neutral',
}

const STORAGE_KEY = 'activity-center:selected'

const { t } = useI18n()
const initialLoading = ref(true)
const inviteCampaign = ref<Campaign | null>(null)
const lotteryCampaign = ref<LotteryCampaign | null>(null)
const tablistRef = ref<HTMLElement | null>(null)

function inviteTone(status: string): ActivityTone {
  return INVITE_TONE[status] ?? 'neutral'
}

function lotteryTone(status: string): ActivityTone {
  return LOTTERY_TONE[status] ?? 'neutral'
}
const activities = computed<ActivityEntry[]>(() => {
  const entries: ActivityEntry[] = []
  const invite = inviteCampaign.value
  if (invite || !lotteryCampaign.value) {
    entries.push({
      id: 'invite-campaign',
      title: t('activities.inviteCampaignTitle'),
      icon: 'users',
      tone: invite ? inviteTone(invite.status) : 'live',
      statusLabel: invite
        ? t(`campaignRewards.statuses.${invite.status}`)
        : t('activities.activeLabel'),
      component: markRaw(InviteCampaignActivity),
    })
  }
  const lottery = lotteryCampaign.value
  if (lottery) {
    entries.push({
      id: 'lottery-campaign',
      title: t('activities.lotteryCampaignTitle'),
      icon: 'sparkles',
      tone: lotteryTone(lottery.status),
      statusLabel: t(`lotteryCampaign.statuses.${lottery.status}`),
      component: markRaw(LotteryCampaignActivity),
    })
  }
  return entries
})

const selectedActivityId = ref('')

const activeActivity = computed<ActivityEntry>(() => {
  return activities.value.find(a => a.id === selectedActivityId.value) ?? activities.value[0]!
})

const showSwitcher = computed(() => activities.value.length > 1 && Boolean(activeActivity.value))

function selectActivity(id: string): void {
  selectedActivityId.value = id
  try {
    localStorage.setItem(STORAGE_KEY, id)
  } catch {
    /* storage unavailable, selection stays in-memory */
  }
}
function onTabKeydown(event: KeyboardEvent): void {
  const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End']
  if (!keys.includes(event.key)) return
  event.preventDefault()
  const list = activities.value
  const current = list.findIndex(a => a.id === activeActivity.value.id)
  if (current === -1) return
  let next = current
  if (event.key === 'ArrowLeft') next = (current - 1 + list.length) % list.length
  else if (event.key === 'ArrowRight') next = (current + 1) % list.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = list.length - 1
  const target = list[next]
  if (!target) return
  selectActivity(target.id)
  nextTick(() => {
    tablistRef.value?.querySelector<HTMLButtonElement>(`#activity-tab-${target.id}`)?.focus()
  })
}

onMounted(async () => {
  const [inviteResult, lotteryResult] = await Promise.allSettled([
    campaignsAPI.getActiveCampaign(),
    lotteryCampaignsAPI.getActiveLotteryCampaign(),
  ])
  inviteCampaign.value =
    inviteResult.status === 'fulfilled' ? inviteResult.value.campaign : null
  lotteryCampaign.value =
    lotteryResult.status === 'fulfilled' ? lotteryResult.value.campaign : null

  let stored: string | null = null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    stored = null
  }
  const available = activities.value
  selectedActivityId.value =
    (stored && available.some(a => a.id === stored) ? stored : available[0]?.id) ?? ''
  initialLoading.value = false
})
</script>

<style scoped>
.activity-fade-enter-active,
.activity-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.activity-fade-enter-from,
.activity-fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

@media (prefers-reduced-motion: reduce) {
  .activity-fade-enter-active,
  .activity-fade-leave-active {
    transition: none;
  }

  .activity-fade-enter-from,
  .activity-fade-leave-to {
    transform: none;
  }
}
</style>
