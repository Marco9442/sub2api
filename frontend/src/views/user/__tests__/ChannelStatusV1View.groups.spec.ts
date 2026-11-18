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
        MonitorHero: { props: ['overallStatus'], template: '<div class="hero">{{ overallStatus }}</div>' },
        MonitorCardGrid: {
          props: { items: { type: Array, default: () => [] }, hideEmpty: Boolean },
          template: '<div class="grid" :data-hide="String(hideEmpty)" :data-count="items.length" />',
        },
        MonitorDetailDialog: true,
        ProviderIcon: true,
        Icon: true,
      },
    },
  })
}

describe('channel status display groups', () => {
  it('renders titled sections and hides empty catalog groups', async () => {
    mocks.list.mockResolvedValue({
      items: [
        { id: 2, name: 'b', provider: 'openai', group_name: 'beta', primary_status: 'operational' },
        { id: 4, name: 'u', provider: 'openai', group_name: '', primary_status: 'operational' },
      ],
      groups: [
        { id: 1, name: 'beta', sort_order: 0 },
        { id: 2, name: 'empty', sort_order: 1 },
      ],
      windows: null,
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('beta')
    expect(wrapper.text()).toContain('channelStatus.groups.ungrouped')
    expect(wrapper.text()).not.toContain('empty')
    expect(wrapper.findAll('.grid')).toHaveLength(2)
    expect(wrapper.findAll('.grid').every((grid) => grid.attributes('data-hide') === 'true')).toBe(true)
    expect(wrapper.get('.hero').text()).toBe('operational')
  })
})
