export interface DisplayLayoutMonitor {
  id: number
  name: string
  provider: string
  enabled: boolean
  group_name: string
  display_sort: number
}

export interface DisplayLayoutGroup {
  id: number
  name: string
  sort_order: number
  monitor_ids: number[]
}

export interface DisplayLayoutView {
  groups: DisplayLayoutGroup[]
  ungrouped_ids: number[]
  monitors: DisplayLayoutMonitor[]
}

export interface DisplayDraftGroup {
  id: number
  name: string
  monitorIds: number[]
}

export interface DisplayOrphan {
  name: string
  monitorIds: number[]
}

export interface DisplayDraft {
  groups: DisplayDraftGroup[]
  ungroupedIds: number[]
  orphans: DisplayOrphan[]
}

export interface DisplayLayoutPayload {
  groups: { id: number; monitor_ids: number[] }[]
  ungrouped_ids: number[]
}

export function draftFromAdmin(view: DisplayLayoutView): DisplayDraft {
  const catalog = new Set(view.groups.map((group) => group.name))
  const orphans: DisplayOrphan[] = []
  for (const monitor of view.monitors) {
    if (monitor.group_name === '' || catalog.has(monitor.group_name)) continue
    const current = orphans.find((orphan) => orphan.name === monitor.group_name)
    if (current) current.monitorIds.push(monitor.id)
    else orphans.push({ name: monitor.group_name, monitorIds: [monitor.id] })
  }
  return {
    groups: view.groups.map((group) => ({
      id: group.id,
      name: group.name,
      monitorIds: [...group.monitor_ids],
    })),
    ungroupedIds: [...view.ungrouped_ids],
    orphans,
  }
}

export function orphanCount(draft: DisplayDraft): number {
  return draft.orphans.reduce((sum, orphan) => sum + orphan.monitorIds.length, 0)
}

export function moveWithin(ids: number[], index: number, delta: number): number[] {
  const next = index + delta
  if (index < 0 || next < 0 || index >= ids.length || next >= ids.length) return [...ids]
  const copy = [...ids]
  const [item] = copy.splice(index, 1)
  copy.splice(next, 0, item)
  return copy
}

function withoutMonitor(draft: DisplayDraft, id: number): DisplayDraft {
  return {
    groups: draft.groups.map((group) => ({
      ...group,
      monitorIds: group.monitorIds.filter((monitorId) => monitorId !== id),
    })),
    ungroupedIds: draft.ungroupedIds.filter((monitorId) => monitorId !== id),
    orphans: draft.orphans
      .map((orphan) => ({
        ...orphan,
        monitorIds: orphan.monitorIds.filter((monitorId) => monitorId !== id),
      }))
      .filter((orphan) => orphan.monitorIds.length > 0),
  }
}

export function relocateMonitor(draft: DisplayDraft, id: number, target: string): DisplayDraft {
  if (target.startsWith('orphan:')) return draft
  const next = withoutMonitor(draft, id)
  if (target === 'ungrouped') {
    next.ungroupedIds = [...next.ungroupedIds, id]
    return next
  }
  if (!target.startsWith('group:')) return draft
  const groupId = Number(target.slice('group:'.length))
  const group = next.groups.find((item) => item.id === groupId)
  if (!group) return draft
  group.monitorIds = [...group.monitorIds, id]
  return next
}

export function layoutPayload(draft: DisplayDraft): DisplayLayoutPayload {
  return {
    groups: draft.groups.map((group) => ({ id: group.id, monitor_ids: [...group.monitorIds] })),
    ungrouped_ids: [...draft.ungroupedIds],
  }
}
