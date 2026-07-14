<template>
  <BaseDialog :show="show" :title="t('admin.accounts.accountCollections.batchTitle')" width="normal" @close="$emit('close')">
    <div class="space-y-4">
      <div class="grid grid-cols-2 gap-2">
        <button class="btn" :class="operation === 'add' ? 'btn-primary' : 'btn-secondary'" @click="operation = 'add'">{{ t('admin.accounts.accountCollections.addTo') }}</button>
        <button class="btn" :class="operation === 'remove' ? 'btn-primary' : 'btn-secondary'" @click="operation = 'remove'">{{ t('admin.accounts.accountCollections.removeFrom') }}</button>
      </div>
      <div class="max-h-72 space-y-2 overflow-y-auto">
        <label v-for="item in collections" :key="item.id" class="flex cursor-pointer items-center gap-3 rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600">
          <input v-model="selectedIds" type="checkbox" :value="item.id" class="rounded border-gray-300 text-primary-600" />
          <span class="text-sm">{{ item.name }}</span>
        </label>
      </div>
      <button class="btn btn-primary w-full" :disabled="submitting || selectedIds.length === 0" @click="submit">{{ t('common.confirm') }}</button>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI, type AccountCollection } from '@/api/admin'
import { useAppStore } from '@/stores/app'
const props = defineProps<{ show: boolean; accountIds: number[]; collections: AccountCollection[] }>()
const emit = defineEmits<{ close: []; updated: [] }>()
const { t } = useI18n(); const appStore = useAppStore()
const operation = ref<'add' | 'remove'>('add'); const selectedIds = ref<number[]>([]); const submitting = ref(false)
watch(() => props.show, value => { if (value) { operation.value = 'add'; selectedIds.value = [] } })
async function submit() {
  submitting.value = true
  try { await adminAPI.accountCollections.batchMembers(props.accountIds, selectedIds.value, operation.value); appStore.showSuccess(t('admin.accounts.accountCollections.batchSuccess')); emit('updated'); emit('close') }
  catch (error: any) { appStore.showError(error.message || t('common.error')) }
  finally { submitting.value = false }
}
</script>
