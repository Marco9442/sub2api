export type ChannelMonitorMode = 'v1' | 'v2' | 'custom'

/** 空值或未知值保持 v1。只有明确的 v2 / custom 才离开 v1。 */
export function normalizeChannelMonitorMode(mode: string | null | undefined): ChannelMonitorMode {
  if (mode === 'v2' || mode === 'custom') return mode
  return 'v1'
}

export function isActiveChannelMonitorMode(mode: ChannelMonitorMode): boolean {
  return mode === 'v1' || mode === 'custom'
}
