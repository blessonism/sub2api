<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="tabs w-full overflow-x-auto sm:w-auto">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="tab shrink-0"
          :class="{ 'tab-active': activeTab === tab.key }"
          :aria-pressed="activeTab === tab.key"
          @click="selectTab(tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>

      <KeepAlive>
        <BalanceSummaryView v-if="activeTab === 'balance-summary'" embedded />
        <RedeemRecordsView v-else embedded />
      </KeepAlive>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BalanceSummaryView from './BalanceSummaryView.vue'
import RedeemRecordsView from './RedeemRecordsView.vue'

type BalanceRedemptionTab = 'balance-summary' | 'redeem-records'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const tabs = computed<Array<{ key: BalanceRedemptionTab; label: string }>>(() => [
  { key: 'balance-summary', label: t('admin.balanceRedemption.tabs.balanceSummary') },
  { key: 'redeem-records', label: t('admin.balanceRedemption.tabs.redeemRecords') }
])

const activeTab = computed<BalanceRedemptionTab>(() => {
  return route.query.tab === 'redeem-records' ? 'redeem-records' : 'balance-summary'
})

function selectTab(tab: BalanceRedemptionTab): void {
  if (tab === activeTab.value) return
  router.replace({
    path: '/admin/balance-redemption',
    query: tab === 'redeem-records' ? { tab } : {}
  })
}
</script>
