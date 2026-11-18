import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { UserMonitorView } from '@/api/channelMonitor'
import MonitorCardGrid from '../monitor/MonitorCardGrid.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
  }
})

function item(overrides: Partial<UserMonitorView> = {}): UserMonitorView {
  return {
    id: 1,
    name: 'main',
    provider: 'anthropic',
    group_name: '',
    primary_model: 'claude-sonnet',
    primary_status: 'operational',
    primary_latency_ms: 100,
    primary_ping_latency_ms: 10,
    availability_7d: 99,
    extra_models: [],
    timeline: [],
    ...overrides,
  }
}

function mountGrid(metric: 'latency' | 'ttft', view: UserMonitorView) {
  return mount(MonitorCardGrid, {
    props: { items: [view], window: '7d', countdownSeconds: 0, loading: false, metric },
    global: { stubs: { MonitorCard: { props: ['availabilityValue'], template: '<div data-test="card">{{ availabilityValue }}</div>' } } },
  })
}

describe('MonitorCardGrid availability', () => {
  it('uses the selected window value in custom mode', () => {
    const wrapper = mountGrid('ttft', item({ window_availability: 80 }))
    expect(wrapper.get('[data-test="card"]').text()).toBe('80')
  })

  it('passes nothing when the selected window has no records', () => {
    const wrapper = mountGrid('ttft', item({ window_availability: null, availability_7d: 99 }))
    expect(wrapper.get('[data-test="card"]').text()).toBe('')
  })

  it('hides the page empty state inside a titled section', () => {
    const wrapper = mount(MonitorCardGrid, {
      props: { items: [], window: '7d', countdownSeconds: 0, loading: false, hideEmpty: true },
    })
    expect(wrapper.text()).not.toContain('channelStatus.empty.title')
  })

  it('uses the selected window value outside custom mode', () => {
    const wrapper = mountGrid('latency', item({ availability_7d: 99, window_availability: 10 }))
    expect(wrapper.get('[data-test="card"]').text()).toBe('10')
  })

  it('does not fall back to the seven day value when the window has no records', () => {
    const wrapper = mountGrid('latency', item({ window_availability: null, availability_7d: 99 }))
    expect(wrapper.get('[data-test="card"]').text()).toBe('')
  })
})
