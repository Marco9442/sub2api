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

function mountCard(status: UserMonitorView['primary_status']) {
  const item: UserMonitorView = {
    id: 1,
    name: 'claude-main',
    provider: 'anthropic',
    group_name: 'beta',
    primary_model: 'claude-sonnet',
    primary_status: status,
    primary_latency_ms: 820,
    primary_ping_latency_ms: 40,
    availability_7d: 100,
    extra_models: [],
    timeline: [],
  }
  return mount(MonitorCard, {
    props: { item, window: '7d', availabilityValue: 100, countdownSeconds: 0 },
  })
}

describe('MonitorCard chrome', () => {
  it('does not paint a status stripe at the top of the card', () => {
    for (const status of ['operational', 'degraded', 'failed', 'error'] as const) {
      const wrapper = mountCard(status)
      // 状态已经由右上角徽标承担，卡片不再额外加一条顶部色条。
      expect(wrapper.findAll('.inset-x-4')).toHaveLength(0)
    }
  })

  it('draws a single separator between metrics and the timeline', () => {
    const wrapper = mountCard('operational')
    expect(wrapper.findAll('.border-t')).toHaveLength(1)
  })
})
