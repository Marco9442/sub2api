import { describe, expect, it } from 'vitest'
import {
  resolveMonitorMetricKind,
  timelineMetricSuffix,
  tokenRateDisplay,
} from '../channelMonitorMetric'

describe('channel monitor metric display', () => {
  it('keeps quota rows on their own result instead of the page TTFT label', () => {
    expect(resolveMonitorMetricKind('quota', 'ttft')).toBe('quota')
    expect(resolveMonitorMetricKind(undefined, 'ttft')).toBe('ttft')
    expect(resolveMonitorMetricKind('latency', 'ttft')).toBe('latency')
  })

  it('hides token/s only when a red TTFT row has no number', () => {
    expect(tokenRateDisplay('ttft', null, null)).toBe('hidden')
    expect(tokenRateDisplay('ttft', 120, 18.5)).toBe('value')
    expect(tokenRateDisplay('ttft', 120, null)).toBe('omit')
    expect(tokenRateDisplay('quota', null, null)).toBe('omit')
    expect(tokenRateDisplay('latency', 400, null)).toBe('omit')
  })

  it('omits red TTFT and quota numbers from history tooltips', () => {
    const formatMs = (ms: number | null) => (ms == null ? '—' : String(ms))
    expect(timelineMetricSuffix({ latency_ms: null, metric_kind: 'ttft', tokens_per_sec: null }, formatMs)).toBe('')
    expect(timelineMetricSuffix({ latency_ms: null, metric_kind: 'quota' }, formatMs)).toBe('')
    expect(timelineMetricSuffix({ latency_ms: 80, metric_kind: 'ttft', tokens_per_sec: 12.5 }, formatMs)).toBe(' · TTFT 80ms · 12.500 token/s')
    expect(timelineMetricSuffix({ latency_ms: 800, metric_kind: 'latency' }, formatMs)).toBe(' · 800ms')
    expect(timelineMetricSuffix({ latency_ms: null, metric_kind: 'latency' }, formatMs)).toBe(' · —ms')
  })
})
