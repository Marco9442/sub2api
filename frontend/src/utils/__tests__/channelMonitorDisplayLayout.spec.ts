import { describe, expect, it } from 'vitest'

import {
  draftFromAdmin,
  layoutPayload,
  moveWithin,
  orphanCount,
  relocateMonitor,
} from '../channelMonitorDisplayLayout'

const view = {
  groups: [
    { id: 1, name: 'alpha', sort_order: 0, monitor_ids: [10] },
    { id: 2, name: 'beta', sort_order: 1, monitor_ids: [] },
  ],
  ungrouped_ids: [11],
  monitors: [
    { id: 10, name: 'a', provider: 'openai', enabled: true, group_name: 'alpha', display_sort: 0 },
    { id: 11, name: 'b', provider: 'openai', enabled: false, group_name: '', display_sort: 0 },
    { id: 12, name: 'c', provider: 'grok', enabled: true, group_name: 'legacy', display_sort: 0 },
  ],
}

describe('channel monitor display layout draft', () => {
  it('keeps empty groups and does not fold orphans into ungrouped', () => {
    const draft = draftFromAdmin(view)
    expect(draft.groups.map((group) => group.name)).toEqual(['alpha', 'beta'])
    expect(draft.ungroupedIds).toEqual([11])
    expect(draft.orphans).toEqual([{ name: 'legacy', monitorIds: [12] }])
    expect(orphanCount(draft)).toBe(1)
  })

  it('places an orphan before the layout can be saved', () => {
    const moved = relocateMonitor(draftFromAdmin(view), 12, 'group:2')
    expect(orphanCount(moved)).toBe(0)
    expect(moved.groups[1].monitorIds).toEqual([12])
    expect(layoutPayload(moved).groups).toEqual([
      { id: 1, monitor_ids: [10] },
      { id: 2, monitor_ids: [12] },
    ])
    expect(layoutPayload(moved).ungrouped_ids).toEqual([11])
  })

  it('reorders inside a group without dropping ids', () => {
    expect(moveWithin([10, 11, 12], 2, -1)).toEqual([10, 12, 11])
    expect(moveWithin([10], 0, -1)).toEqual([10])
  })
})
