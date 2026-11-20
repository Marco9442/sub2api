import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import ChannelStatusV1View from '../ChannelStatusV1View.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), status: vi.fn() }))
vi.mock('@/api/channelMonitor', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: {}, showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, te: () => false }),
}))
enableAutoUnmount(afterEach)

function mountView() {
  return shallowMount(ChannelStatusV1View, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        MonitorHero: true,
        MonitorCardGrid: {
          props: { hideGroupBadge: Boolean },
          template: '<div class="grid" :data-hide="String(hideGroupBadge)" />',
        },
        MonitorDetailDialog: true,
        ProviderIcon: true,
        Icon: true,
      },
    },
  })
}

describe('channel status card density', () => {
  it('hides the repeated badge inside a titled section and shows the legend', async () => {
    mocks.list.mockResolvedValue({
      items: [{ id: 2, name: 'b', provider: 'openai', group_name: 'beta', primary_status: 'operational' }],
      groups: [{ id: 1, name: 'beta', sort_order: 0 }],
      windows: null,
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.grid').attributes('data-hide')).toBe('true')
    expect(wrapper.text()).toContain('monitorCommon.status.operational')
    expect(wrapper.text()).toContain('channelStatus.legend.noRecord')
  })

  it('keeps the badge on the flat fallback', async () => {
    mocks.list.mockResolvedValue({
      items: [{ id: 2, name: 'b', provider: 'openai', group_name: '', primary_status: 'operational' }],
      groups: [],
      windows: null,
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.grid').attributes('data-hide')).toBe('false')
  })
})
