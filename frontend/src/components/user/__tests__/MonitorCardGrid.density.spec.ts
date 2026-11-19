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
  it('sizes columns from the container and passes the section badge flag', () => {
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
    expect(grid.classes().join(' ')).toContain('gap-4')
    // 列数跟着内容区宽度走：auto-fit + 20rem 最小列宽，窄容器自动降列。
    expect(grid.attributes('style')).toContain('auto-fit')
    expect(grid.attributes('style')).toContain('20rem')
    expect(wrapper.get('[data-test="card"]').attributes('data-hide')).toBe('true')
  })
})
