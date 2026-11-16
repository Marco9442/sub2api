export type PageStatus = 'operational' | 'degraded' | 'failed'

const redStatuses = new Set(['failed', 'error'])

// overallMonitorStatus 按已启用监控的当前状态计算用户页右上角。
// 失败和出错算红色；降级、失败和出错都算黄色。两边都达到时红色优先。
export function overallMonitorStatus(
  statuses: Array<string | undefined>,
  yellowPercent: number,
  redPercent: number,
): PageStatus {
  const total = statuses.length
  if (total === 0) return 'operational'
  let red = 0
  let yellow = 0
  for (const status of statuses) {
    if (status === 'degraded' || redStatuses.has(status ?? '')) yellow += 1
    if (redStatuses.has(status ?? '')) red += 1
  }
  if (red * 100 >= redPercent * total) return 'failed'
  if (yellow * 100 >= yellowPercent * total) return 'degraded'
  return 'operational'
}

// legacyOverallStatus 是 V1 的旧规则：有一条不是正常，整页就是黄色。
export function legacyOverallStatus(statuses: Array<string | undefined>): PageStatus {
  if (statuses.some((status) => status !== 'operational')) return 'degraded'
  return 'operational'
}
