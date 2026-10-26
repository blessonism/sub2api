<template>
  <div>
    <div class="mb-6 flex flex-wrap items-center gap-3">
      <span class="rounded-full bg-violet-100 px-3 py-1 text-xs font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">前端预览 · 内存 Mock</span>
      <p class="text-xs text-gray-500 dark:text-gray-400">数据为内置示例，交互（保存配置 / 立即探测 / 改判）均可用，刷新页面重置。</p>
      <button type="button" class="btn h-8 text-xs" @click="toggleTheme">{{ dark ? '浅色' : '深色' }}</button>
    </div>

    <h2 class="mb-3 text-sm font-bold uppercase tracking-widest text-gray-400">① 渠道状态页 · 模型可用性下方的糖果探针区块</h2>
    <CandyProbePanel :items="candyItems" />

    <h2 class="mb-3 mt-10 text-sm font-bold uppercase tracking-widest text-gray-400">② 管理端 · 智商监控页面（完整可交互）</h2>
    <ModelIqMonitorView />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import CandyProbePanel from '@/components/user/monitor/CandyProbePanel.vue'
import ModelIqMonitorView from '@/views/admin/ModelIqMonitorView.vue'
import { fetchModelIqProbeSummary, type ModelIqProbeSummaryItem } from '@/api/modelIqProbe'

const candyItems = ref<ModelIqProbeSummaryItem[]>([])
const dark = ref(document.documentElement.classList.contains('dark'))

function toggleTheme() {
  dark.value = !dark.value
  document.documentElement.classList.toggle('dark', dark.value)
}

onMounted(async () => {
  const summary = await fetchModelIqProbeSummary()
  candyItems.value = summary.items
})
</script>
