import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import BaseDialog from '@/components/common/BaseDialog.vue'

function mountDialog(props: Partial<InstanceType<typeof BaseDialog>['$props']> = {}) {
  return mount(BaseDialog, {
    attachTo: document.body,
    props: {
      show: true,
      title: '测试弹窗',
      ...props,
    },
    slots: {
      default: '<button type="button" data-testid="inside-button">内部按钮</button>',
    },
    global: {
      stubs: {
        Icon: true,
      },
    },
  })
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('拦截弹窗面板内的低层交互事件', async () => {
    const wrapper = mountDialog({ closeOnClickOutside: true })
    await nextTick()

    const bodyEventListener = vi.fn()
    const eventNames = ['pointerdown', 'pointerup', 'mousedown', 'mouseup', 'touchstart', 'touchend', 'click']
    for (const eventName of eventNames) {
      document.body.addEventListener(eventName, bodyEventListener)
    }

    const insideButton = document.body.querySelector('[data-testid="inside-button"]')
    expect(insideButton).toBeInstanceOf(HTMLButtonElement)

    for (const eventName of eventNames) {
      insideButton!.dispatchEvent(new Event(eventName, { bubbles: true }))
    }
    await nextTick()

    expect(bodyEventListener).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeUndefined()

    for (const eventName of eventNames) {
      document.body.removeEventListener(eventName, bodyEventListener)
    }
    wrapper.unmount()
  })

  it('仍然支持点击遮罩关闭', async () => {
    const wrapper = mountDialog({ closeOnClickOutside: true })
    await nextTick()

    const overlay = document.body.querySelector('.modal-overlay')
    expect(overlay).toBeInstanceOf(HTMLDivElement)

    overlay!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('resets body scroll position when reopened', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: false, title: 'Details' },
      slots: { default: '<div style="height: 2000px">content</div>' },
      global: { stubs: { Icon: true } }
    })

    await wrapper.setProps({ show: true })
    await nextTick()
    const body = document.body.querySelector<HTMLElement>('.modal-body')
    expect(body).not.toBeNull()
    body!.scrollTop = 480

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(document.body.querySelector<HTMLElement>('.modal-body')?.scrollTop).toBe(0)
    wrapper.unmount()
  })
})
