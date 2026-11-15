import type { ChannelMonitorCustomConfig, ChannelMonitorCustomOverride } from '@/api/admin/channelMonitor'

export type CustomConfigErrors = Partial<Record<'prompt' | 'thresholds' | 'ttft' | 'tps', true>>

export function defaultCustomConfig(): ChannelMonitorCustomConfig {
  return {
    prompt: 'ping',
    yellow_seconds: 2,
    red_seconds: 8,
    ttft_adjust_enabled: false,
    ttft_multiplier: 1,
    ttft_min_ms: 0,
    ttft_max_ms: 0,
    tps_enabled: false,
    tps_min: 0,
    tps_max: 0,
  }
}

// 与后端 validateCustomConfig 的同名规则保持一致；红色阈值与检测间隔的关系仍由后端校验。
export function customConfigErrors(cfg: ChannelMonitorCustomConfig): CustomConfigErrors {
  const errors: CustomConfigErrors = {}
  if (!cfg.prompt.trim()) errors.prompt = true
  if (!ordered(cfg.yellow_seconds, cfg.red_seconds, false)) errors.thresholds = true
  if (cfg.ttft_adjust_enabled && !(isNumber(cfg.ttft_multiplier) && cfg.ttft_multiplier > 0 && ordered(cfg.ttft_min_ms, cfg.ttft_max_ms, true))) {
    errors.ttft = true
  }
  if (cfg.tps_enabled && !ordered(cfg.tps_min, cfg.tps_max, true)) errors.tps = true
  return errors
}

export function hasCustomOverride(override: ChannelMonitorCustomOverride | null | undefined): boolean {
  return Object.values(override ?? {}).some((v) => v != null)
}

function isNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function ordered(low: unknown, high: unknown, allowZero: boolean): boolean {
  return isNumber(low) && isNumber(high) && (allowZero ? low >= 0 : low > 0) && low < high
}
