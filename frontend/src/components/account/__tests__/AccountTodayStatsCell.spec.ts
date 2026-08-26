import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTodayStatsCell from '../AccountTodayStatsCell.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountTodayStatsCell', () => {
  it('renders cache hit tokens and the matching hit rate', () => {
    const wrapper = mount(AccountTodayStatsCell, {
      props: {
        stats: {
          requests: 2,
          input_tokens: 200,
          cache_creation_tokens: 300,
          cache_read_tokens: 500,
          tokens: 1200,
          cost: 1.2,
          user_cost: 1
        }
      }
    })

    expect(wrapper.text()).toContain('500')
    expect(wrapper.text()).toContain('(50.0%)')
  })

  it('shows a dash when the prompt-token denominator is zero', () => {
    const wrapper = mount(AccountTodayStatsCell, {
      props: {
        stats: {
          requests: 0,
          tokens: 0,
          cost: 0
        }
      }
    })

    expect(wrapper.text()).toContain('0(-)')
  })
})
