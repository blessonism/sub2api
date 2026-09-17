import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ChannelStatusApiDialog from '@/components/user/monitor/ChannelStatusApiDialog.vue'
import ChannelStatusV1View from '../ChannelStatusV1View.vue'
import ChannelStatusV2View from '../ChannelStatusV2View.vue'

const { copyToClipboard } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key,
    te: () => true,
    locale: ref('zh-CN'),
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAdmin: false }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { channel_monitor_enabled: false },
    showError: vi.fn(),
  }),
}))

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorThroughputHidden: () => false,
  isChannelMonitorUserRankingHidden: () => true,
}))

vi.mock('@/composables/useAutoRefresh', () => ({
  useAutoRefresh: () => ({
    enabled: ref(false),
    intervalSeconds: ref(60),
    countdown: ref(60),
    intervals: [30, 60, 120],
    setEnabled: vi.fn(),
    setInterval: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
    resetCountdown: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: ref(false), copyToClipboard }),
}))

vi.mock('@/api/channelMonitor', () => ({
  list: vi.fn().mockResolvedValue({ items: [] }),
  status: vi.fn(),
}))

vi.mock('@/api/gptIntelligence', () => ({
  fetchGptIntelligenceSnapshot: vi.fn().mockResolvedValue(null),
}))

vi.mock('@/api/channelMonitorV2', () => ({
  getDimensions: vi.fn().mockResolvedValue({ platforms: [], groups: [], models: [] }),
  getSnapshot: vi.fn().mockResolvedValue(null),
  getMatrix: vi.fn().mockResolvedValue({ items: [] }),
  getModels: vi.fn().mockResolvedValue({ items: [] }),
  getErrors: vi.fn().mockResolvedValue({ items: [] }),
  getUsers: vi.fn().mockResolvedValue({ items: [] }),
}))

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const MonitorHeroStub = defineComponent({
  name: 'MonitorHero',
  emits: ['open-api-docs'],
  template: '<button class="open-api-docs" @click="$emit(\'open-api-docs\')">API</button>',
})

const ApiDialogStub = defineComponent({
  name: 'ChannelStatusApiDialog',
  props: { show: Boolean },
  emits: ['close'],
  template: '<button v-if="show" class="close-api-docs" @click="$emit(\'close\')">close</button>',
})

const viewStubs = {
  AppLayout: AppLayoutStub,
  MonitorHero: MonitorHeroStub,
  ChannelStatusApiDialog: ApiDialogStub,
  RelayPulseMatrix: defineComponent({ template: '<div />' }),
}

describe('channel status API docs entry', () => {
  beforeEach(() => {
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)
  })

  it('opens and closes the guide from the V1 entry', async () => {
    const wrapper = shallowMount(ChannelStatusV1View, { global: { stubs: viewStubs } })
    await wrapper.get('.open-api-docs').trigger('click')

    expect(wrapper.getComponent(ApiDialogStub).props('show')).toBe(true)
    await wrapper.get('.close-api-docs').trigger('click')
    expect(wrapper.getComponent(ApiDialogStub).props('show')).toBe(false)
  })

  it('shows a text label and opens and closes the guide from the V2 entry', async () => {
    const wrapper = shallowMount(ChannelStatusV2View, { global: { stubs: viewStubs } })
    await flushPromises()

    const entry = wrapper.get('button[title="common.channelStatusApi.open"]')
    expect(entry.text()).toContain('common.channelStatusApi.shortLabel')
    await entry.trigger('click')

    expect(wrapper.getComponent(ApiDialogStub).props('show')).toBe(true)
    await wrapper.get('.close-api-docs').trigger('click')
    expect(wrapper.getComponent(ApiDialogStub).props('show')).toBe(false)
  })

  it('copies the documented curl example', async () => {
    const BaseDialogStub = defineComponent({
      name: 'BaseDialog',
      props: { show: Boolean, title: String },
      emits: ['close'],
      template: '<div v-if="show"><slot /><button class="close" @click="$emit(\'close\')">close</button></div>',
    })
    const wrapper = shallowMount(ChannelStatusApiDialog, {
      props: { show: true },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })

    expect(wrapper.text()).toContain('GET /v1/sub2api/channel-status?scope=visible')
    expect(wrapper.text()).toContain('common.channelStatusApi.responseStatus')
    expect(wrapper.text()).toContain('common.channelStatusApi.responseVisible')

    await wrapper.get('button[aria-label="common.channelStatusApi.copyExample"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('$BASE_URL/v1/sub2api/channel-status'),
      'common.channelStatusApi.copied',
    )

    await wrapper.get('.close').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
