<template>
  <BaseDialog :show="show" :title="t('common.channelStatusApi.title')" width="wide" @close="emit('close')">
    <div class="space-y-5 text-sm text-gray-600 dark:text-gray-300">
      <p>{{ t('common.channelStatusApi.description') }}</p>

      <section>
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('common.channelStatusApi.requestTitle') }}</h3>
        <div class="mt-2 overflow-x-auto rounded-lg bg-gray-900 p-3 font-mono text-xs leading-6 text-gray-100">
          <code>GET /v1/sub2api/channel-status</code>
          <div class="text-gray-400">GET /v1/sub2api/channel-status?scope=visible</div>
        </div>
      </section>

      <section>
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('common.channelStatusApi.authTitle') }}</h3>
        <ul class="mt-2 list-disc space-y-1 pl-5">
          <li>{{ t('common.channelStatusApi.authBearer') }}</li>
          <li>{{ t('common.channelStatusApi.authApiKey') }}</li>
          <li>{{ t('common.channelStatusApi.authQueryForbidden') }}</li>
        </ul>
      </section>

      <section>
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('common.channelStatusApi.exampleTitle') }}</h3>
        <div class="relative mt-2">
          <pre class="overflow-x-auto rounded-lg bg-gray-900 p-3 pr-20 font-mono text-xs leading-6 text-gray-100"><code>{{ curlExample }}</code></pre>
          <button
            type="button"
            class="absolute right-2 top-2 inline-flex items-center gap-1 rounded-md border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-gray-200 hover:bg-gray-700"
            :aria-label="t('common.channelStatusApi.copyExample')"
            @click="copyExample"
          >
            <Icon name="copy" size="xs" />
            {{ copied ? t('common.copied') : t('common.channelStatusApi.copyExample') }}
          </button>
        </div>
      </section>

      <section>
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('common.channelStatusApi.responseTitle') }}</h3>
        <ul class="mt-2 list-disc space-y-1 pl-5">
          <li>{{ t('common.channelStatusApi.responseConnected') }}</li>
          <li>{{ t('common.channelStatusApi.responseStatus') }}</li>
          <li>{{ t('common.channelStatusApi.responseVisible') }}</li>
        </ul>
      </section>

      <p class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
        {{ t('common.channelStatusApi.rateLimit') }}
      </p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const curlExample = computed(() => `curl -sS \\
  -H "Authorization: Bearer $API_KEY" \\
  "$BASE_URL/v1/sub2api/channel-status"`)

function copyExample() {
  void copyToClipboard(curlExample.value, t('common.channelStatusApi.copied'))
}
</script>
