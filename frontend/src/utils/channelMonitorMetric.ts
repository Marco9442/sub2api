export type MonitorMetricKind = 'latency' | 'ttft' | 'quota'
export type TokenRateDisplay = 'value' | 'hidden' | 'omit'

/** 行上的指标种类优先于页面模式。纯配额不借用 TTFT 标签。 */
export function resolveMonitorMetricKind(
  kind: string | null | undefined,
  fallback: 'latency' | 'ttft' = 'latency',
): MonitorMetricKind {
  if (kind === 'ttft' || kind === 'latency' || kind === 'quota') return kind
  return fallback
}

/**
 * 红色 TTFT 的 token/s 已被后端置空，这时显示占位。
 * 只是没开启生成时不显示该行，避免把“没有这项”画成“已隐藏”。
 */
export function tokenRateDisplay(
  kind: string | null | undefined,
  latency: number | null | undefined,
  tokens: number | null | undefined,
): TokenRateDisplay {
  if (kind !== 'ttft') return 'omit'
  if (tokens != null) return 'value'
  if (latency == null) return 'hidden'
  return 'omit'
}

export function timelineMetricSuffix(
  point: { latency_ms: number | null; metric_kind?: string; tokens_per_sec?: number | null },
  formatMs: (ms: number | null) => string,
): string {
  const hideNumber = point.latency_ms == null && (point.metric_kind === 'ttft' || point.metric_kind === 'quota')
  const latencyLabel = point.metric_kind === 'ttft' ? 'TTFT ' : ''
  const latency = hideNumber ? '' : ` · ${latencyLabel}${formatMs(point.latency_ms)}ms`
  const tokens = point.tokens_per_sec == null ? '' : ` · ${point.tokens_per_sec.toFixed(3)} token/s`
  return `${latency}${tokens}`
}
