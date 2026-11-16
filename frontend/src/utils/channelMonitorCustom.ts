import type { ChannelMonitorCustomConfig, ChannelMonitorCustomOverride } from '@/api/admin/channelMonitor'

export type CustomConfigErrors = Partial<Record<'prompt' | 'thresholds' | 'ttft' | 'tps' | 'status', true>>

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
    status_yellow_percent: 30,
    status_red_percent: 30,
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
  if (!validPercent(cfg.status_yellow_percent) || !validPercent(cfg.status_red_percent)) errors.status = true
  return errors
}

function validPercent(value: unknown): boolean {
  return isNumber(value) && Number.isInteger(value) && value >= 1 && value <= 100
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

export interface AvailabilityWindow {
  amount: number
  unit: 'm' | 'd'
}

const defaultWindows: AvailabilityWindow[] = [
  { amount: 7, unit: 'd' },
  { amount: 15, unit: 'd' },
  { amount: 30, unit: 'd' },
]

// 解析 "15m,1d,5d"。空文本返回默认档，非法返回 null。规则与后端一致。
export function parseAvailabilityWindows(raw: string): AvailabilityWindow[] | null {
  const text = raw.trim()
  if (!text) return defaultWindows.map((w) => ({ ...w }))
  const parts = text.split(',').map((p) => p.trim())
  if (parts.length === 0 || parts.length > 6) return null
  const seen = new Set<string>()
  const out: AvailabilityWindow[] = []
  for (const part of parts) {
    const match = /^(\d+)([md])$/.exec(part)
    if (!match) return null
    const amount = Number(match[1])
    const unit = match[2] as 'm' | 'd'
    if (amount < 1 || amount > 60) return null
    if (unit === 'd' && amount > 30) return null
    const key = `${amount}${unit}`
    if (seen.has(key)) return null
    seen.add(key)
    out.push({ amount, unit })
  }
  return out.sort((a, b) => secondsOf(a) - secondsOf(b))
}

function secondsOf(w: AvailabilityWindow): number {
  return w.unit === 'd' ? w.amount * 86400 : w.amount * 60
}

// 把时间档显示成文案；不认识的内容原样返回。
export function windowLabel(key: string, t: (key: string, n: number) => string): string {
  const match = /^(\d+)([md])$/.exec(key)
  if (!match) return key
  return t(`channelMonitorCustom.window${match[2] === 'm' ? 'Minutes' : 'Days'}`, Number(match[1]))
}
