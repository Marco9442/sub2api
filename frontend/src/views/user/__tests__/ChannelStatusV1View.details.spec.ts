import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import ChannelStatusV1View from '../ChannelStatusV1View.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), status: vi.fn() }))
vi.mock('@/api/channelMonitor', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { channel_monitor_enabled: true }, showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)
afterEach(() => { vi.useRealTimers(); localStorage.clear() })

beforeEach(() => {
  vi.useFakeTimers()
  vi.resetAllMocks()
  localStorage.clear()
  localStorage.setItem('channel-status-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 30 }))
  mocks.list.mockImplementation(async (opts: { window?: string }) => ({
    items: [{
      id: 1,
      name: 'Monitor',
      primary_model: 'model',
      primary_status: 'operational',
      availability_7d: 99,
      window_availability: opts?.window === '1d' ? 90 : opts?.window === '3d' ? 98 : 70,
    }],
    windows: [
      { amount: 90, unit: 'm' },
      { amount: 1, unit: 'd' },
      { amount: 3, unit: 'd' },
      { amount: 7, unit: 'd' },
    ],
    timeline_points: 20,
  }))
})

function mountView() {
  return shallowMount(ChannelStatusV1View, {
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      MonitorHero: { props: ['loading', 'window'], emits: ['update:window', 'refresh'], template: `<div :data-loading="loading" :data-window="window">
        <button class="one-day" @click="$emit('update:window', '1d')">1d</button>
        <button class="three-day" @click="$emit('update:window', '3d')">3d</button>
        <button class="refresh" @click="$emit('refresh')">Refresh</button>
      </div>` },
      MonitorCardGrid: false,
      MonitorCard: { props: ['availabilityValue'], template: '<output>{{ availabilityValue }}</output>' }
    } }
  })
}

describe('channel monitor window availability', () => {
  it('loads the default window from the list and does not call status', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-window]').attributes('data-window')).toBe('90m')
    expect(wrapper.get('output').text()).toBe('70')
    expect(mocks.list).toHaveBeenCalledTimes(1)
    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({ window: '90m' }))
    expect(mocks.status).not.toHaveBeenCalled()
  })

  it.each([
    ['one-day', '1d', '90'],
    ['three-day', '3d', '98'],
  ])('reloads the list for the %s window', async (tab, window, availability) => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.' + tab).trigger('click')
    await flushPromises()
    expect(wrapper.get('output').text()).toBe(availability)
    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({ window }))
    expect(mocks.status).not.toHaveBeenCalled()
    expect(wrapper.get('[data-loading]').attributes('data-loading')).toBe('false')
  })

  it('refreshes the selected window from the list on timer ticks', async () => {
    const wrapper = mountView()
    await flushPromises()
    mocks.list.mockImplementation(async () => ({
      items: [{ id: 1, name: 'Monitor', primary_model: 'model', primary_status: 'operational', availability_7d: 99, window_availability: 81 }],
      windows: [{ amount: 90, unit: 'm' }],
      timeline_points: 20,
    }))
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(mocks.list).toHaveBeenCalledTimes(2)
    expect(mocks.status).not.toHaveBeenCalled()
    expect(wrapper.get('output').text()).toBe('81')
  })
})
