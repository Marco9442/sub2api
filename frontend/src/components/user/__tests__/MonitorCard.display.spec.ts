import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { UserMonitorView } from '@/api/channelMonitor'
import MonitorCard from '../monitor/MonitorCard.vue'

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorQuotaVisible: () => false,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
  }
})

function mountCard(overrides: Partial<UserMonitorView> = {}, hideGroupBadge = false) {
  const item: UserMonitorView = {
    id: 1,
    name: 'claude-main',
    provider: 'anthropic',
    group_name: 'beta',
    primary_model: 'claude-sonnet',
    primary_status: 'operational',
    primary_latency_ms: 820,
    primary_ping_latency_ms: 40,
    availability_7d: 100,
    extra_models: [{ model: 'extra', status: 'operational', latency_ms: 1 }],
    timeline: [{ status: 'operational', latency_ms: 1, ping_latency_ms: 1, checked_at: '2026-11-18T00:00:00Z' }],
    ...overrides,
  }
  return mount(MonitorCard, {
    props: { item, window: '7d', availabilityValue: 100, countdownSeconds: 0, hideGroupBadge },
    global: { stubs: { MonitorTimeline: { props: ['buckets', 'length'], template: '<div data-test="timeline">{{ buckets.length }}:{{ length }}</div>' } } },
  })
}

describe('MonitorCard display density', () => {
  it('keeps the primary metric, availability and timeline when the badge is hidden', () => {
    const wrapper = mountCard({}, true)
    expect(wrapper.text()).toContain('820')
    expect(wrapper.text()).toContain('100.00')
    expect(wrapper.text()).toContain('monitorCommon.availabilityPrefix')
    expect(wrapper.text()).toContain('monitorCommon.extraModelsCount')
    expect(wrapper.text()).not.toContain('monitorCommon.endpointPing')
    expect(wrapper.text()).not.toContain('40')
    expect(wrapper.get('[data-test="timeline"]').text()).toBe('1:20')
    expect(wrapper.text()).not.toContain('beta')
    expect(wrapper.classes().join(' ')).toContain('p-4')
    expect(wrapper.classes().join(' ')).toContain('min-h-[240px]')
  })

  it('keeps the group badge on the flat fallback', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('beta')
  })
})
