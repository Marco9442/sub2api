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

function mountCard(overrides: Partial<UserMonitorView>) {
  const item: UserMonitorView = {
    id: 1,
    name: 'claude-main',
    provider: 'anthropic',
    group_name: '',
    primary_model: 'claude-sonnet',
    primary_status: 'operational',
    primary_latency_ms: 820,
    primary_ping_latency_ms: 40,
    availability_7d: 100,
    extra_models: [],
    timeline: [],
    ...overrides,
  }
  return mount(MonitorCard, {
    props: { item, window: '7d', availabilityValue: 100, countdownSeconds: 0, metric: 'ttft' },
    global: { stubs: { MonitorAvailabilityRow: true, MonitorTimeline: true } },
  })
}

describe('MonitorCard TTFT metric', () => {
  it('shows token/s inside the TTFT tile', () => {
    const wrapper = mountCard({ primary_metric_kind: 'ttft', primary_tokens_per_sec: 42.1234 })
    const tile = wrapper.get('[data-testid="metric-primary"]')

    expect(tile.text()).toContain('channelMonitorCustom.ttft')
    expect(tile.text()).toContain('820')
    expect(tile.get('[data-testid="metric-primary-note"]').text()).toBe('42.123 channelMonitorCustom.tokensPerSec')
  })

  it('hides token/s together with TTFT for a red result', () => {
    const wrapper = mountCard({
      primary_metric_kind: 'ttft',
      primary_status: 'failed',
      primary_latency_ms: null,
      primary_tokens_per_sec: null,
    })

    expect(wrapper.get('[data-testid="metric-primary-note"]').text()).toBe('channelMonitorCustom.hidden channelMonitorCustom.tokensPerSec')
  })

  it('omits token/s for latency monitors', () => {
    const wrapper = mountCard({ primary_metric_kind: 'latency' })

    expect(wrapper.find('[data-testid="metric-primary-note"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('channelMonitorCustom.tokensPerSec')
  })
})
