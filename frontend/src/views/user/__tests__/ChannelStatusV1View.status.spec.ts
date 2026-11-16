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

const items = [
  ...Array.from({ length: 3 }, (_, i) => ({ id: i + 1, name: 'bad', primary_model: 'm', primary_status: 'failed', availability_7d: 0 })),
  ...Array.from({ length: 7 }, (_, i) => ({ id: i + 10, name: 'ok', primary_model: 'm', primary_status: 'operational', availability_7d: 100 })),
]

beforeEach(() => {
  vi.resetAllMocks()
  localStorage.clear()
  mocks.list.mockResolvedValue({
    items,
    windows: [{ amount: 7, unit: 'd' }],
    status_yellow_percent: 30,
    status_red_percent: 30,
  })
})

function mountView(metric: 'latency' | 'ttft') {
  return shallowMount(ChannelStatusV1View, {
    props: { metric },
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      MonitorHero: { props: ['overallStatus'], template: '<div :data-status="overallStatus" />' },
      MonitorCardGrid: true,
    } }
  })
}

describe('channel monitor overall status', () => {
  it('uses the saved percents on the custom page', async () => {
    const wrapper = mountView('ttft')
    await flushPromises()
    expect(wrapper.get('[data-status]').attributes('data-status')).toBe('failed')
  })

  it('keeps the old any-item rule outside custom mode', async () => {
    const wrapper = mountView('latency')
    await flushPromises()
    expect(wrapper.get('[data-status]').attributes('data-status')).toBe('degraded')
  })
})
