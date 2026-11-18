import { describe, expect, it } from 'vitest'

import type { UserMonitorView } from '@/api/channelMonitor'
import { groupMonitorSections } from '../channelMonitorDisplayGroups'

function item(overrides: Partial<UserMonitorView>): UserMonitorView {
  return {
    id: 1,
    name: 'channel',
    provider: 'openai',
    group_name: '',
    primary_model: 'gpt',
    primary_status: 'operational',
    primary_latency_ms: null,
    primary_ping_latency_ms: null,
    availability_7d: null,
    extra_models: [],
    timeline: [],
    ...overrides,
  }
}

describe('groupMonitorSections', () => {
  it('stays flat when every enabled channel is ungrouped', () => {
    const grouped = groupMonitorSections(
      [item({ id: 1 }), item({ id: 2 })],
      [{ id: 9, name: 'empty' }],
      'Ungrouped',
    )
    expect(grouped.flat).toBe(true)
    expect(grouped.sections).toEqual([])
  })

  it('keeps backend order and puts ungrouped last', () => {
    const grouped = groupMonitorSections(
      [
        item({ id: 2, group_name: 'beta', provider: 'anthropic' }),
        item({ id: 1, group_name: 'beta', provider: 'openai' }),
        item({ id: 8, group_name: 'legacy', provider: 'grok' }),
        item({ id: 4, group_name: '' }),
      ],
      [{ name: 'beta' }],
      'Ungrouped',
    )
    expect(grouped.flat).toBe(false)
    expect(grouped.sections.map((section) => section.title)).toEqual(['beta', 'legacy', 'Ungrouped'])
    expect(grouped.sections[0].items.map((row) => row.id)).toEqual([2, 1])
    expect(grouped.sections[0].provider).toBeNull()
    expect(grouped.sections[1].provider).toBe('grok')
    expect(grouped.sections[1].key).toBe('orphan:legacy')
  })
})
