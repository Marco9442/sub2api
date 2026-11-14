import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'

const mode = vi.fn(() => 'v2')

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV1Mode: () => mode() === 'v1',
  isChannelMonitorActiveMode: () => mode() === 'v1' || mode() === 'custom',
  getChannelMonitorMode: () => mode(),
}))

vi.mock('../ChannelStatusV1View.vue', () => ({
  default: defineComponent({
    name: 'ChannelStatusV1View',
    props: { metric: { type: String, default: 'latency' } },
    setup: (props) => () => h('div', { 'data-testid': 'v1', 'data-metric': props.metric }),
  }),
}))
vi.mock('../ChannelStatusV2View.vue', () => ({
  default: defineComponent({ name: 'ChannelStatusV2View', setup: () => () => h('div', { 'data-testid': 'v2' }) }),
}))

import ChannelStatusView from '../ChannelStatusView.vue'

describe('ChannelStatusView mode switch', () => {
  beforeEach(() => {
    mode.mockReset()
  })

  it('renders V2 when mode is v2', () => {
    mode.mockReturnValue('v2')
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v1"]').exists()).toBe(false)
  })

  it('renders V1 latency view when mode is v1', () => {
    mode.mockReturnValue('v1')
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v1"]').attributes('data-metric')).toBe('latency')
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(false)
  })

  it('renders the active view with TTFT when mode is custom', () => {
    mode.mockReturnValue('custom')
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v1"]').attributes('data-metric')).toBe('ttft')
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(false)
  })
})
