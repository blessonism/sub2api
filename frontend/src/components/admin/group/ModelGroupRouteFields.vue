<template>
  <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600" data-testid="model-group-routes">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h3 class="input-label mb-0">{{ t('admin.groups.modelGroupRoutes.title') }}</h3>
        <p class="input-hint">{{ t('admin.groups.modelGroupRoutes.hint') }}</p>
      </div>
      <button type="button" class="btn btn-secondary shrink-0" data-testid="route-add" @click="add">
        {{ t('admin.groups.modelGroupRoutes.add') }}
      </button>
    </div>
    <p v-if="loadError" role="alert" class="text-sm text-red-600">
      {{ t('admin.groups.modelGroupRoutes.loadFailed') }}
      <button type="button" class="underline" @click="loadGroups">{{ t('common.retry') }}</button>
    </p>
    <p v-if="modelValue.length === 0" class="text-sm text-gray-500">{{ t('admin.groups.modelGroupRoutes.empty') }}</p>
    <div v-for="(route, index) in modelValue" :key="index" class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="block">
          <span class="input-label">{{ t('admin.groups.modelGroupRoutes.model') }}</span>
          <input :value="route.model" class="input font-mono" placeholder="gpt-6-luna" data-testid="route-model" @input="update(index, { model: inputValue($event) })" />
        </label>
        <div>
          <label :for="`${idPrefix}-target-${index}`" class="input-label">{{ t('admin.groups.modelGroupRoutes.target') }}</label>
          <Select :id="`${idPrefix}-target-${index}`" :model-value="route.target_group_id || null" :options="targetOptions(route)" searchable data-testid="route-target" @update:model-value="update(index, { target_group_id: Number($event) })" />
        </div>
      </div>
      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-48 flex-1">
          <label :for="`${idPrefix}-mode-${index}`" class="input-label">{{ t('admin.groups.modelGroupRoutes.mode') }}</label>
          <Select :id="`${idPrefix}-mode-${index}`" :model-value="route.mode" :options="modeOptions" data-testid="route-mode" @update:model-value="setMode(index, $event)" />
        </div>
        <label class="flex items-center gap-2 py-2 text-sm">
          <input type="checkbox" :checked="route.enabled" data-testid="route-enabled" @change="update(index, { enabled: checked($event) })" />
          {{ t('admin.groups.modelGroupRoutes.enabled') }}
        </label>
        <button type="button" class="btn btn-secondary" data-testid="route-remove" @click="remove(index)">{{ t('admin.groups.modelGroupRoutes.remove') }}</button>
      </div>
      <p class="input-hint">{{ t(`admin.groups.modelGroupRoutes.${route.mode}Hint`) }}</p>
    </div>
    <p v-if="invalid" role="alert" class="text-sm text-red-600" data-testid="route-error">{{ t('admin.groups.modelGroupRoutes.invalid') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type { AdminGroup, ModelGroupRoute } from '@/types'

const props = defineProps<{ modelValue: ModelGroupRoute[]; sourceGroupId?: number; idPrefix: string }>()
const emit = defineEmits<{ (event: 'update:modelValue', value: ModelGroupRoute[]): void }>()
const { t } = useI18n()
const groups = ref<AdminGroup[]>([])
const loadError = ref(false)
const invalid = ref(false)
const modeOptions = computed(() => ['target_only', 'prefer_local'].map(value => ({ value, label: t(`admin.groups.modelGroupRoutes.${value}`) })))

async function loadGroups() {
  loadError.value = false
  try { groups.value = await adminAPI.groups.getAll() } catch { loadError.value = true }
}
onMounted(loadGroups)

function targetOptions(route: ModelGroupRoute) {
  const options = groups.value
    .filter(group => group.id !== props.sourceGroupId && group.platform === 'openai' && group.subscription_type === 'standard' && group.status === 'active')
    .map(group => ({ value: group.id, label: `${group.name} (#${group.id})` }))
  if (route.target_group_id && !options.some(option => option.value === route.target_group_id)) {
    options.push({ value: route.target_group_id, label: `#${route.target_group_id} (${t('admin.groups.modelGroupRoutes.unavailable')})` })
  }
  return options
}
function update(index: number, patch: Partial<ModelGroupRoute>) {
  emit('update:modelValue', props.modelValue.map((route, i) => i === index ? { ...route, ...patch } : route))
  invalid.value = false
}
function add() { emit('update:modelValue', [...props.modelValue, { model: '', target_group_id: 0, mode: 'target_only', enabled: true }]) }
function remove(index: number) { emit('update:modelValue', props.modelValue.filter((_, i) => i !== index)); invalid.value = false }
function inputValue(event: Event) { return (event.target as HTMLInputElement).value }
function checked(event: Event) { return (event.target as HTMLInputElement).checked }
function setMode(index: number, value: string | number | boolean | null) {
  if (value === 'target_only' || value === 'prefer_local') update(index, { mode: value })
}
function validate() {
  const seen = new Set<string>()
  invalid.value = props.modelValue.some(route => {
    const model = route.model.trim()
    const duplicate = seen.has(model)
    seen.add(model)
    return !model || model.includes('*') || duplicate || !route.target_group_id || route.target_group_id === props.sourceGroupId
  })
  return !invalid.value
}
defineExpose({ validate })
</script>
