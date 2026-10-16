<template>
  <section class="space-y-4 border-t border-gray-200 pt-4 dark:border-dark-600">
    <div class="flex items-start justify-between gap-4">
      <div>
        <label :id="`${idPrefix}-label`" class="input-label mb-0">
          {{ t('admin.groups.systemPromptInjection.title') }}
        </label>
        <p class="input-hint mb-0">{{ t('admin.groups.systemPromptInjection.hint') }}</p>
      </div>
      <button
        type="button"
        role="switch"
        :aria-labelledby="`${idPrefix}-label`"
        :aria-checked="modelValue.enabled"
        data-testid="system-prompt-toggle"
        class="relative inline-flex h-7 w-12 shrink-0 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500/30"
        :class="modelValue.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'"
        @click="update({ enabled: !modelValue.enabled })"
      >
        <span
          class="mt-1 inline-block h-5 w-5 rounded-full bg-white shadow transition-transform"
          :class="modelValue.enabled ? 'translate-x-6' : 'translate-x-1'"
        />
      </button>
    </div>
    <div v-if="modelValue.enabled" class="space-y-4">
      <div>
        <label :for="`${idPrefix}-mode`" class="input-label">
          {{ t('admin.groups.systemPromptInjection.mode') }}
        </label>
        <Select
          :id="`${idPrefix}-mode`"
          :model-value="modelValue.mode"
          :options="modeOptions"
          :searchable="false"
          :aria-label="t('admin.groups.systemPromptInjection.mode')"
          data-testid="system-prompt-mode"
          @update:model-value="updateMode"
        />
        <p class="input-hint" data-testid="system-prompt-mode-hint">
          {{ t(`admin.groups.systemPromptInjection.${modelValue.mode}Hint`) }}
        </p>
      </div>
      <div>
        <label :for="`${idPrefix}-content`" class="input-label">
          {{ t('admin.groups.systemPromptInjection.content') }}
        </label>
        <textarea
          :id="`${idPrefix}-content`"
          :value="modelValue.content"
          rows="6"
          class="input font-mono text-sm"
          :placeholder="t('admin.groups.systemPromptInjection.placeholder')"
          :aria-invalid="invalid"
          :aria-describedby="invalid ? `${idPrefix}-error` : undefined"
          data-testid="system-prompt-content"
          @input="updateContent"
        />
        <p
          v-if="invalid"
          :id="`${idPrefix}-error`"
          role="alert"
          class="mt-1 text-xs text-red-600 dark:text-red-400"
          data-testid="system-prompt-error"
        >
          {{ t('admin.groups.systemPromptInjection.contentRequired') }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { GroupSystemPromptInjection } from '@/types'

const props = defineProps<{
  idPrefix: string
  modelValue: GroupSystemPromptInjection
}>()
const emit = defineEmits<{
  (event: 'update:modelValue', value: GroupSystemPromptInjection): void
}>()
const { t } = useI18n()
const showValidation = ref(false)
const invalid = computed(() =>
  showValidation.value && props.modelValue.enabled && !props.modelValue.content.trim()
)
const modeOptions = computed(() => ['prepend', 'append', 'replace'].map(mode => ({
  value: mode,
  label: t(`admin.groups.systemPromptInjection.${mode}`)
})))

function update(patch: Partial<GroupSystemPromptInjection>) {
  emit('update:modelValue', { ...props.modelValue, ...patch })
}

function updateMode(value: string | number | boolean | null) {
  if (value === 'prepend' || value === 'append' || value === 'replace') {
    update({ mode: value })
  }
}

function updateContent(event: Event) {
  update({ content: (event.target as HTMLTextAreaElement).value })
}

function validate(): boolean {
  showValidation.value = true
  return !invalid.value
}

defineExpose({ validate })
</script>
