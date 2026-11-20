import type { Provider, UserMonitorView } from '@/api/channelMonitor'

export interface DisplayGroupRef {
  id?: number
  name: string
  sort_order?: number
}

export interface MonitorSection {
  key: string
  title: string
  items: UserMonitorView[]
  provider: Provider | null
}

export interface GroupedMonitors {
  flat: boolean
  sections: MonitorSection[]
}

function groupName(item: Pick<UserMonitorView, 'group_name'>): string {
  return item.group_name ?? ''
}

function sharedProvider(items: UserMonitorView[]): Provider | null {
  const provider = items[0]?.provider
  if (!provider || items.some((item) => item.provider !== provider)) return null
  return provider
}

/**
 * 按后端已经排好的顺序切段。空目录不生成标题；一旦出现有名字的段，未分组放在最后。
 */
export function groupMonitorSections(
  items: UserMonitorView[],
  groups: DisplayGroupRef[] | undefined,
  ungroupedTitle: string,
): GroupedMonitors {
  const catalog = new Set((groups ?? []).map((group) => group.name))
  const titled = items.some((item) => groupName(item) !== '')
  if (!titled) return { flat: true, sections: [] }

  const buckets = new Map<string, UserMonitorView[]>()
  const order: string[] = []
  for (const item of items) {
    const name = groupName(item)
    if (!buckets.has(name)) {
      buckets.set(name, [])
      order.push(name)
    }
    buckets.get(name)!.push(item)
  }

  const sections: MonitorSection[] = []
  for (const name of order) {
    if (name === '') continue
    const sectionItems = buckets.get(name) ?? []
    sections.push({
      key: catalog.has(name) ? `catalog:${name}` : `orphan:${name}`,
      title: name,
      items: sectionItems,
      provider: sharedProvider(sectionItems),
    })
  }
  const ungrouped = buckets.get('')
  if (ungrouped && ungrouped.length > 0) {
    sections.push({
      key: 'ungrouped',
      title: ungroupedTitle,
      items: ungrouped,
      provider: sharedProvider(ungrouped),
    })
  }
  return { flat: false, sections }
}
