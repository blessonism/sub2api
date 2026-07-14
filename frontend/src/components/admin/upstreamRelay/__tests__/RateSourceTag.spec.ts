import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import RateSourceTag from '../RateSourceTag.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('RateSourceTag', () => {
  it('通过 i18n 展示倍率来源标签和说明', () => {
    const wrapper = mount(RateSourceTag, {
      props: { source: 'login_user_group_rates' },
      global: {
        stubs: {
          HelpTooltip: { props: ['content'], template: '<div><span data-testid="tip">{{ content }}</span><slot /></div>' },
        },
      },
    })

    expect(wrapper.text()).toContain('rateSource.loginUserGroupRates.label')
    expect(wrapper.get('[data-testid="tip"]').text()).toContain('rateSource.loginUserGroupRates.tip')
  })
})
