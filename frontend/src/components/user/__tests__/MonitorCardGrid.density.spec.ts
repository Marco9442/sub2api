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

const item: UserMonitorView = {
  id: 1,
  name: 'main',
  provider: 'anthropic',
  group_name: 'beta',
  primary_model: 'claude-sonnet',
  primary_status: 'operational',
  primary_latency_ms: 100,
  primary_ping_latency_ms: 10,
  availability_7d: 99,
  extra_models: [],
  timeline: [],
}

describe('MonitorCardGrid density', () => {
  it('keeps the column breakpoints and passes the section badge flag', () => {
    const wrapper = mount(MonitorCardGrid, {
      props: {
        items: [item],
        window: '7d',
        countdownSeconds: 0,
        loading: false,
        hideGroupBadge: true,
      },
      global: {
        stubs: {
          MonitorCard: {
            props: ['hideGroupBadge'],
            template: '<div data-test="card" :data-hide="String(hideGroupBadge)" />',
          },
        },
      },
    })
    const grid = wrapper.get('.grid')
    const classes = grid.classes().join(' ')
    expect(classes).toContain('grid-cols-1')
    expect(classes).toContain('md:grid-cols-2')
    expect(classes).toContain('xl:grid-cols-3')
    expect(classes).toContain('2xl:grid-cols-4')
    expect(classes).toContain('gap-4')
    expect(wrapper.get('[data-test="card"]').attributes('data-hide')).toBe('true')
  })
})
