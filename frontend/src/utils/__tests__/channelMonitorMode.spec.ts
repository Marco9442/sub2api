import { describe, expect, it } from 'vitest'
import { isActiveChannelMonitorMode, normalizeChannelMonitorMode } from '../channelMonitorMode'

describe('normalizeChannelMonitorMode', () => {
  it('keeps v2 and custom, and maps everything else to v1', () => {
    expect(normalizeChannelMonitorMode('v2')).toBe('v2')
    expect(normalizeChannelMonitorMode('custom')).toBe('custom')
    expect(normalizeChannelMonitorMode('v1')).toBe('v1')
    expect(normalizeChannelMonitorMode('')).toBe('v1')
    expect(normalizeChannelMonitorMode(undefined)).toBe('v1')
    expect(normalizeChannelMonitorMode('V2')).toBe('v1')
  })

  it('treats only v1 and custom as active probe modes', () => {
    expect(isActiveChannelMonitorMode('v1')).toBe(true)
    expect(isActiveChannelMonitorMode('custom')).toBe(true)
    expect(isActiveChannelMonitorMode('v2')).toBe(false)
  })
})
