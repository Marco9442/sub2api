import { describe, expect, it } from 'vitest'
import { customConfigErrors, defaultCustomConfig, hasCustomOverride } from '../channelMonitorCustom'

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
