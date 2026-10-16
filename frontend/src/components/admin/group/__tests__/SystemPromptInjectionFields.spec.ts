import { defineComponent, h, reactive } from 'vue'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import SystemPromptInjectionFields from '../SystemPromptInjectionFields.vue'
import type { GroupSystemPromptInjection } from '@/types'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

enableAutoUnmount(afterEach)

const SelectStub = defineComponent({
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
})

function mountFields(locale: 'en' | 'zh' = 'zh') {
  const config = reactive<GroupSystemPromptInjection>({ enabled: false, mode: 'append', content: '' })
  const host = defineComponent({
    setup: () => () => h(SystemPromptInjectionFields, {
      idPrefix: 'test',
      modelValue: config,
      'onUpdate:modelValue': (value: GroupSystemPromptInjection) => Object.assign(config, value)
    })
  })
  return {
    config,
    wrapper: mount(host, {
      global: {
        plugins: [createI18n({
          legacy: false,
          locale,
          messages: Object.fromEntries(Object.entries({ en, zh }).map(([language, messages]) => [language, {
            admin: { groups: { systemPromptInjection: Object.fromEntries(
              Object.entries(messages.admin.groups.systemPromptInjection).map(([key, value]) => [key, () => value])
            ) } }
          }]))
        })],
        stubs: { Select: SelectStub }
      }
    })
  }
}

describe('SystemPromptInjectionFields', () => {
  it('keeps content and mode across consecutive toggle and content updates', async () => {
    const { wrapper, config } = mountFields()
    expect(wrapper.find('textarea').exists()).toBe(false)
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('textarea').setValue('  保留原始空格\n系统指令  ')
    await wrapper.get('select').setValue('replace')
    await wrapper.get('[role="switch"]').trigger('click')
    expect(config).toEqual({ enabled: false, mode: 'replace', content: '  保留原始空格\n系统指令  ' })
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe(config.content)
    expect(wrapper.get<HTMLSelectElement>('select').element.value).toBe('replace')
  })

  it('rejects whitespace when enabled and clears the error when corrected or disabled', async () => {
    const { wrapper } = mountFields()
    const field = wrapper.findComponent(SystemPromptInjectionFields)
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('textarea').setValue(' \n\t ')
    expect(field.vm.validate()).toBe(false)
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[role="alert"]').text()).toBe(zh.admin.groups.systemPromptInjection.contentRequired)
    expect(wrapper.get('textarea').attributes('aria-invalid')).toBe('true')
    await wrapper.get('textarea').setValue('custom')
    expect(field.vm.validate()).toBe(true)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    await wrapper.get('textarea').setValue(' ')
    await wrapper.get('[role="switch"]').trigger('click')
    expect(field.vm.validate()).toBe(true)
  })

  it.each(['zh', 'en'] as const)('renders translated mode descriptions in %s', async locale => {
    const { wrapper } = mountFields(locale)
    const copy = { zh, en }[locale].admin.groups.systemPromptInjection
    expect(wrapper.text()).toContain(copy.title)
    await wrapper.get('[role="switch"]').trigger('click')
    for (const mode of ['prepend', 'append', 'replace'] as const) {
      await wrapper.get('select').setValue(mode)
      expect(wrapper.get('[data-testid="system-prompt-mode-hint"]').text()).toBe(copy[`${mode}Hint`])
    }
    expect(wrapper.text()).not.toContain('admin.groups.systemPromptInjection.')
  })
})
