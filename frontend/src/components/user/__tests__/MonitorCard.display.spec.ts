import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'
import type { UserMonitorView } from '@/api/channelMonitor'
import MonitorCard from '../monitor/MonitorCard.vue'
import MonitorCardGrid from '../monitor/MonitorCardGrid.vue'
import MonitorTimeline from '../monitor/MonitorTimeline.vue'

vi.mock('@/utils/featureFlags', () => ({ isChannelMonitorQuotaVisible: () => false }))
enableAutoUnmount(afterEach)
const now = Date.parse('2026-10-17T12:00:00Z')
const item: UserMonitorView = {
 id: 1, target_kind: 'endpoint', name: 'GPT 特惠不降智分组', provider: 'openai', group_name: '展示分组',
 primary_model: 'gpt-test', primary_status: 'operational', primary_latency_ms: 3000,
 primary_ping_latency_ms: 25, availability_7d: 89.1, extra_models: [], timeline: [],
 primary_checked_at: '2026-10-17T11:59:35Z', visible_rate_multiplier: 0.25,
 cache_rate_24h: 0.623, output_tps_5m: 15.2,
}
function compiled(value: unknown): unknown {
 // Vitest 使用不含编译器的 i18n runtime，预编译仓库内的可信语言消息。
 if (typeof value === 'string') return new Function(`return (${baseCompile(value).code})`)()
 if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, compiled(entry)]))
 return value
}
function plugin(locale = 'zh') { return createI18n({ legacy: false, locale, messages: { zh: compiled(zh), en: compiled(en) } as Record<string, Record<string, unknown>> }) }
function card(overrides: Partial<UserMonitorView> = {}, locale = 'zh') {
 return mount(MonitorCard, { props: { item: { ...item, ...overrides }, window: '7d', availabilityValue: 89.1, countdownSeconds: 30, nowMs: now }, global: { plugins: [plugin(locale)] } })
}
describe('V1 monitor display card', () => {
 it.each(['zh', 'en'])('shows real metrics and preserves details entry (%s)', async locale => {
  const wrapper = card({}, locale)
  expect(wrapper.get('[data-testid="monitor-rate"]').text()).toBe('0.25x')
  expect(wrapper.get('[data-testid="monitor-latency"]').text()).toBe('3.0s')
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('89.1%')
  expect(wrapper.get('[data-testid="monitor-cache"]').text()).toBe('62.3%')
  expect(wrapper.get('[data-testid="monitor-tps"]').text()).toBe('15.2 TPS')
  expect(wrapper.get('[data-testid="monitor-checked-at"]').text()).toContain('25')
  expect(wrapper.get('.metric-column').attributes('title')).toContain('25ms')
  await wrapper.trigger('click')
  expect(wrapper.emitted('click')).toHaveLength(1)
  await wrapper.setProps({ nowMs: now + 10000, window: '30d', availabilityValue: 98.3 })
  expect(wrapper.get('[data-testid="monitor-checked-at"]').text()).toContain('35')
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('98.3%')
  expect(wrapper.text()).toContain('30')
 })
 it('distinguishes unknown data from measured zero', async () => {
  const wrapper = card({ primary_checked_at: null, visible_rate_multiplier: null, cache_rate_24h: null, output_tps_5m: null, primary_latency_ms: null })
  for (const name of ['rate', 'tps', 'latency']) expect(wrapper.get(`[data-testid="monitor-${name}"]`).text()).toBe('—')
  expect(wrapper.get('[data-testid="monitor-cache"]').text()).toBe('0.0%')
  expect(wrapper.get('[data-testid="monitor-cache"]').element.closest('.metric-column')?.getAttribute('title')).toContain('暂不可用')
  await wrapper.setProps({ item: { ...item, visible_rate_multiplier: 0, cache_rate_24h: 0, output_tps_5m: 0, primary_latency_ms: 25 } })
  expect(wrapper.get('[data-testid="monitor-rate"]').text()).toBe('0.00x')
  expect(wrapper.get('[data-testid="monitor-cache"]').text()).toBe('0.0%')
  expect(wrapper.get('[data-testid="monitor-tps"]').text()).toBe('0.0 TPS')
  expect(wrapper.get('[data-testid="monitor-latency"]').text()).toBe('25ms')
 })
 it.each([[0.18, '0.18x'], [0.185, '0.185x'], [1, '1.00x'], [0.003, '0.003x'], [0.1801, '0.18x'], [0.0001, '0.000100x']])('formats multiplier %s as %s', (value, label) => {
  expect(card({ visible_rate_multiplier: Number(value) }).get('[data-testid="monitor-rate"]').text()).toBe(label)
 })
 it('explains manually adjusted values while preserving Ping details', () => {
  const wrapper = card({ display_overrides: { visible_rate_multiplier: 0.25, primary_latency_ms: 3000, cache_rate_24h: 0.623, output_tps_5m: 15.2, availability_7d: 89.1 } })
  expect(wrapper.get('[data-testid="monitor-rate"]').attributes('title')).toContain('手动')
  expect(wrapper.get('.metric-column').attributes('title')).toContain('手动')
  expect(wrapper.get('.metric-column').attributes('title')).toContain('25ms')
 })
 it('preserves capacity, degradation, and failure semantics', () => {
  const capacity = card({ primary_status: 'error', primary_error_category: 'rate_or_capacity' })
  expect(capacity.text()).toContain('限流')
  expect(capacity.find('.text-amber-700').exists()).toBe(true)
  expect(card({ primary_status: 'degraded' }).find('.text-amber-700').exists()).toBe(true)
  expect(card({ primary_status: 'failed' }).find('.text-red-600').exists()).toBe(true)
 })
 it('pads history on the left and places the newest probe on the right', () => {
  const wrapper = mount(MonitorTimeline, { props: { compact: true, countdownSeconds: 30, buckets: [
   { status: 'error', error_category: 'rate_or_capacity', checked_at: '2026-10-17T12:00:00Z', latency_ms: 10, ping_latency_ms: null },
   { status: 'operational', checked_at: '2026-10-17T11:59:00Z', latency_ms: 20, ping_latency_ms: null },
  ] }, global: { plugins: [plugin()] } })
  const bars = wrapper.get('[role="img"]').findAll('div')
  expect(bars).toHaveLength(60)
  expect(bars[0]?.classes()).toContain('bg-gray-300')
  expect(bars[58]?.classes()).toContain('bg-emerald-500')
  expect(bars[59]?.classes()).toContain('bg-amber-400')
  expect(bars[59]?.attributes('title')).toContain('10ms')
  expect(bars[59]?.attributes('title')).not.toContain('msms')
 })
 it('resolves the primary model for 7/15/30 day windows and forwards details clicks', async () => {
  const wrapper = mount(MonitorCardGrid, { props: { items: [item], window: '7d', loading: false, countdownSeconds: 30, detailCache: { 1: { id: 1, name: item.name, provider: item.provider, group_name: '', models: [
   { model: 'extra', latest_status: 'operational', latest_latency_ms: null, availability_7d: 1, availability_15d: 2, availability_30d: 3, avg_latency_7d_ms: null },
   { model: item.primary_model, latest_status: 'operational', latest_latency_ms: null, availability_7d: 89.1, availability_15d: 92.2, availability_30d: 93.3, avg_latency_7d_ms: null },
  ] } } }, global: { plugins: [plugin()] } })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('89.1%')
  await wrapper.setProps({ window: '15d' })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('92.2%')
  await wrapper.setProps({ window: '30d' })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('93.3%')
  await wrapper.get('button').trigger('click')
  expect(wrapper.emitted('cardClick')?.[0]?.[0]).toEqual(item)
  await wrapper.setProps({ detailCache: {} })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('—')
  await wrapper.setProps({ items: [{ ...item, display_overrides: { availability_15d: 0, availability_30d: 99 } }], window: '15d' })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('0.0%')
  await wrapper.setProps({ window: '30d' })
  expect(wrapper.get('[data-testid="monitor-availability"]').text()).toBe('99.0%')
  expect(wrapper.get('[data-testid="monitor-availability"]').element.closest('.metric-column')?.getAttribute('title')).toContain('手动')
 })
})
