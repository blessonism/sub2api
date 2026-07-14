<template>
  <BaseDialog :show="show" :title="t('admin.accounts.accountCollections.manage')" width="normal" @close="$emit('close')">
    <div class="space-y-4">
      <form class="flex gap-2" @submit.prevent="addCollection">
        <input v-model="newName" class="input flex-1" :placeholder="t('admin.accounts.accountCollections.namePlaceholder')" maxlength="100" />
        <button class="btn btn-primary" :disabled="saving || !newName.trim()">{{ t('common.add') }}</button>
      </form>
      <div v-if="items.length" class="divide-y divide-gray-200 dark:divide-dark-600">
        <div v-for="(item, index) in items" :key="item.id" class="flex items-center gap-2 py-2">
          <input v-if="editingId === item.id" v-model="editingName" class="input flex-1" maxlength="100" @keyup.enter="saveName(item.id)" />
          <span v-else class="min-w-0 flex-1 truncate text-sm font-medium">{{ item.name }}</span>
          <button class="btn btn-secondary btn-sm px-2" :disabled="index === 0" @click="move(index, -1)">↑</button>
          <button class="btn btn-secondary btn-sm px-2" :disabled="index === items.length - 1" @click="move(index, 1)">↓</button>
          <button v-if="editingId === item.id" class="btn btn-primary btn-sm" @click="saveName(item.id)">{{ t('common.save') }}</button>
          <button v-else class="btn btn-secondary btn-sm" @click="startEdit(item)">{{ t('common.edit') }}</button>
          <button class="btn btn-danger btn-sm" @click="deleteCollection(item)">{{ t('common.delete') }}</button>
        </div>
      </div>
      <p v-else class="py-6 text-center text-sm text-gray-500">{{ t('admin.accounts.accountCollections.empty') }}</p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI, type AccountCollection } from '@/api/admin'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ show: boolean; collections: AccountCollection[] }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const items = ref<AccountCollection[]>([])
const newName = ref('')
const saving = ref(false)
const editingId = ref<number | null>(null)
const editingName = ref('')
watch(() => props.collections, value => { items.value = [...value] }, { immediate: true })

async function addCollection() {
  saving.value = true
  try { await adminAPI.accountCollections.create(newName.value); newName.value = ''; emit('changed') }
  catch (error: any) { appStore.showError(error.message || t('common.error')) }
  finally { saving.value = false }
}
function startEdit(item: AccountCollection) { editingId.value = item.id; editingName.value = item.name }
async function saveName(id: number) {
  if (!editingName.value.trim()) return
  try { await adminAPI.accountCollections.update(id, editingName.value); editingId.value = null; emit('changed') }
  catch (error: any) { appStore.showError(error.message || t('common.error')) }
}
async function deleteCollection(item: AccountCollection) {
  if (!confirm(t('admin.accounts.accountCollections.deleteConfirm', { name: item.name }))) return
  try { await adminAPI.accountCollections.remove(item.id); emit('changed') }
  catch (error: any) { appStore.showError(error.message || t('common.error')) }
}
async function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= items.value.length) return
  const next = [...items.value]; [next[index], next[target]] = [next[target], next[index]]; items.value = next
  try { await adminAPI.accountCollections.updateSort(next.map(item => item.id)); emit('changed') }
  catch (error: any) { appStore.showError(error.message || t('common.error')); items.value = [...props.collections] }
}
</script>
