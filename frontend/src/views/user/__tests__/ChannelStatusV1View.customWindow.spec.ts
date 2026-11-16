import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import ChannelStatusV1View from '../ChannelStatusV1View.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), status: vi.fn() }))
vi.mock('@/api/channelMonitor', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { channel_monitor_enabled: true }, showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, te: () => false })
}))
enableAutoUnmount(afterEach)
afterEach(() => { localStorage.clear() })

beforeEach(() => {
  vi.resetAllMocks()
  localStorage.clear()
  mocks.list.mockImplementation(async (opts: { window?: string }) => ({
    items: [{ id: 1, name: 'Monitor', primary_model: 'model', primary_status: 'operational', availability_7d: 99, window_availability: opts?.window === '1d' ? 50 : 80 }],
    windows: [{ amount: 15, unit: 'm' }, { amount: 1, unit: 'd' }],
  }))
})

function mountView() {
  return shallowMount(ChannelStatusV1View, {
    props: { metric: 'ttft' },
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      MonitorHero: { props: ['window', 'windows'], emits: ['update:window', 'refresh'], template: `<div :data-window="window" :data-count="windows.length">
        <button class="day" @click="$emit('update:window', '1d')">1d</button>
      </div>` },
      MonitorCardGrid: false,
      MonitorCard: { props: ['availabilityValue'], template: '<output>{{ availabilityValue }}</output>' }
    } }
  })
}

describe('channel monitor custom windows', () => {
  it('uses the configured windows and reloads the list for the selected one', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-window]').attributes('data-window')).toBe('15m')
    expect(wrapper.get('[data-window]').attributes('data-count')).toBe('2')
    expect(wrapper.get('output').text()).toBe('80')

    await wrapper.get('.day').trigger('click')
    await flushPromises()

    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({ window: '1d' }))
    expect(wrapper.get('output').text()).toBe('50')
    expect(mocks.status).not.toHaveBeenCalled()
  })
})
