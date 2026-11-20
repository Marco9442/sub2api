import { describe, expect, it } from 'vitest'
import { legacyOverallStatus, overallMonitorStatus } from '../channelMonitorStatus'

const row = (status: string, n: number) => Array.from({ length: n }, () => status)

describe('overallMonitorStatus', () => {
  it('stays operational when nothing is listed', () => {
    expect(overallMonitorStatus([], 30, 30)).toBe('operational')
  })

  it('shows red when both thresholds are met', () => {
    const statuses = [...row('degraded', 3), ...row('failed', 3), ...row('operational', 4)]
    expect(overallMonitorStatus(statuses, 30, 30)).toBe('failed')
  })

  it('shows yellow when only the yellow share is met', () => {
    const statuses = [...row('degraded', 3), ...row('operational', 7)]
    expect(overallMonitorStatus(statuses, 30, 30)).toBe('degraded')
  })

  it('counts failed and error toward yellow when red is not met', () => {
    const statuses = [...row('failed', 2), ...row('error', 1), ...row('operational', 7)]
    expect(overallMonitorStatus(statuses, 30, 40)).toBe('degraded')
  })

  it('stays operational below both thresholds', () => {
    const statuses = [...row('degraded', 1), ...row('failed', 1), ...row('operational', 8)]
    expect(overallMonitorStatus(statuses, 30, 30)).toBe('operational')
  })

  it('treats an exact percentage as reached', () => {
    expect(overallMonitorStatus([...row('error', 3), ...row('operational', 7)], 30, 30)).toBe('failed')
  })
})

describe('legacyOverallStatus', () => {
  it('turns yellow when any item is not operational', () => {
    expect(legacyOverallStatus(['operational', 'failed'])).toBe('degraded')
    expect(legacyOverallStatus(['operational'])).toBe('operational')
    expect(legacyOverallStatus([])).toBe('operational')
  })
})
