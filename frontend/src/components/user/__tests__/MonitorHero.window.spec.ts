import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import MonitorHero from '../monitor/MonitorHero.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, n?: number) => (n === undefined ? key : `${key}:${n}`), te: () => true }) }
})

function mountHero(windows?: { amount: number; unit: 'm' | 'd' }[]) {
  return mount(MonitorHero, {
    props: { overallStatus: 'operational', intervalSeconds: 60, window: '7d', loading: false, windows },
    global: { stubs: { AutoRefreshButton: true, Icon: true } },
  })
}

describe('MonitorHero windows', () => {
  it('shows the fixed tabs by default', () => {
    const wrapper = mountHero()
    expect(wrapper.findAll('[role="tab"]').map((b) => b.text())).toEqual([
      'channelMonitorCustom.windowMinutes:90',
      'channelMonitorCustom.windowDays:1',
      'channelMonitorCustom.windowDays:3',
      'channelMonitorCustom.windowDays:7',
    ])
  })

  it('shows the configured windows when provided', () => {
    const wrapper = mountHero([{ amount: 15, unit: 'm' }, { amount: 1, unit: 'd' }])
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((b) => b.text())).toEqual(['channelMonitorCustom.windowMinutes:15', 'channelMonitorCustom.windowDays:1'])
  })
})
