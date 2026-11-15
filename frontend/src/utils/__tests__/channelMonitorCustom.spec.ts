import { describe, expect, it } from 'vitest'
import { customConfigErrors, defaultCustomConfig, hasCustomOverride, windowLabel, parseAvailabilityWindows } from '../channelMonitorCustom'

describe('customConfigErrors', () => {
  it('accepts the default config', () => {
    expect(customConfigErrors(defaultCustomConfig())).toEqual({})
  })

  it('rejects an empty prompt', () => {
    expect(customConfigErrors({ ...defaultCustomConfig(), prompt: '  ' })).toEqual({ prompt: true })
  })

  it('requires 0 < yellow < red', () => {
    expect(customConfigErrors({ ...defaultCustomConfig(), yellow_seconds: 0 }).thresholds).toBe(true)
    expect(customConfigErrors({ ...defaultCustomConfig(), yellow_seconds: 8, red_seconds: 8 }).thresholds).toBe(true)
  })

  it('checks TTFT processing only when enabled', () => {
    const off = { ...defaultCustomConfig(), ttft_min_ms: 500, ttft_max_ms: 100 }
    expect(customConfigErrors(off).ttft).toBeUndefined()
    expect(customConfigErrors({ ...off, ttft_adjust_enabled: true }).ttft).toBe(true)
    expect(customConfigErrors({ ...off, ttft_adjust_enabled: true, ttft_min_ms: 0, ttft_multiplier: 0 }).ttft).toBe(true)
    expect(customConfigErrors({ ...off, ttft_adjust_enabled: true, ttft_min_ms: 0 }).ttft).toBeUndefined()
  })

  it('checks token/s only when enabled', () => {
    const off = { ...defaultCustomConfig(), tps_min: 5, tps_max: 5 }
    expect(customConfigErrors(off).tps).toBeUndefined()
    expect(customConfigErrors({ ...off, tps_enabled: true }).tps).toBe(true)
    expect(customConfigErrors({ ...off, tps_enabled: true, tps_max: 50 }).tps).toBeUndefined()
  })

  it('treats non-numeric input as invalid', () => {
    const cfg = { ...defaultCustomConfig(), red_seconds: '' as unknown as number }
    expect(customConfigErrors(cfg).thresholds).toBe(true)
  })
})

describe('hasCustomOverride', () => {
  it('ignores missing and null fields', () => {
    expect(hasCustomOverride(null)).toBe(false)
    expect(hasCustomOverride({ ttft_multiplier: null })).toBe(false)
    expect(hasCustomOverride({ tps_enabled: false })).toBe(true)
  })
})

describe('availabilityWindows', () => {
  it('parses and sorts windows', () => {
    expect(parseAvailabilityWindows('5d, 15m')).toEqual([
      { amount: 15, unit: 'm' },
      { amount: 5, unit: 'd' },
    ])
  })

  it('rejects invalid windows', () => {
    expect(parseAvailabilityWindows('0d')).toBeNull()
    expect(parseAvailabilityWindows('31d')).toBeNull()
    expect(parseAvailabilityWindows('15m,15m')).toBeNull()
    expect(parseAvailabilityWindows('1m,2m,3m,4m,5m,6m,7m')).toBeNull()
  })

  it('treats an empty field as the default windows', () => {
    expect(parseAvailabilityWindows('')).toEqual([
      { amount: 7, unit: 'd' },
      { amount: 15, unit: 'd' },
      { amount: 30, unit: 'd' },
    ])
  })
})

describe('windowLabel', () => {
  const t = (key: string, n: number) => `${key}:${n}`

  it('formats minutes and days', () => {
    expect(windowLabel('15m', t)).toBe('channelMonitorCustom.windowMinutes:15')
    expect(windowLabel('5d', t)).toBe('channelMonitorCustom.windowDays:5')
  })

  it('keeps unknown keys as they are', () => {
    expect(windowLabel('abc', t)).toBe('abc')
  })
})
