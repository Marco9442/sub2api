/**
 * User-facing Channel Monitor API endpoints
 * Read-only views for end users to inspect channel availability/status.
 */

import { apiClient } from './client'
import type { MonitorQuotaSnapshot, Provider, MonitorStatus } from './admin/channelMonitor'

export type { Provider, MonitorStatus } from './admin/channelMonitor'

export interface UserMonitorExtraModel {
  model: string
  status: MonitorStatus
  latency_ms: number | null
  tokens_per_sec?: number | null
  metric_kind?: string
}

export interface MonitorTimelinePoint {
  status: MonitorStatus
  latency_ms: number | null
  tokens_per_sec?: number | null
  metric_kind?: string
  ping_latency_ms: number | null
  checked_at: string
}

export interface UserMonitorView {
  id: number
  name: string
  provider: Provider
  group_name: string
  display_sort?: number
  primary_model: string
  primary_status: MonitorStatus
  primary_latency_ms: number | null
  primary_tokens_per_sec?: number | null
  primary_metric_kind?: string
  primary_ping_latency_ms: number | null
  availability_7d: number | null
  // Custom 模式下所选时间档的可用率；null 表示该档内没有记录。
  window_availability?: number | null
  extra_models: UserMonitorExtraModel[]
  timeline: MonitorTimelinePoint[]
  /**
   * 主模型最近配额快照。仅当系统开启 channel_monitor_show_quota 时
   * 服务端才会下发（关闭时服务端已剥离，前端 flag 仅作纵深防御）。
   */
  latest_quota?: MonitorQuotaSnapshot | null
}

export interface UserMonitorGroup {
  id: number
  name: string
  sort_order: number
}

export interface UserMonitorListResponse {
  items: UserMonitorView[]
  groups?: UserMonitorGroup[]
  // Custom 模式下管理员配置的时间档；其他模式为空。
  windows?: { amount: number; unit: 'm' | 'd' }[]
  // Custom 模式下用户页右上角的黄、红百分比。其他模式不返回。
  status_yellow_percent?: number
  status_red_percent?: number
}

export interface UserMonitorModelDetail {
  model: string
  latest_status: MonitorStatus
  latest_latency_ms: number | null
  latest_tokens_per_sec?: number | null
  metric_kind?: string
  // 该时间段没有检测记录时为 null
  availability_7d: number | null
  availability_15d: number | null
  availability_30d: number | null
  avg_latency_7d_ms: number | null
}

export interface UserMonitorDetail {
  id: number
  name: string
  provider: Provider
  group_name: string
  models: UserMonitorModelDetail[]
}

/**
 * List all monitor views available to the current user.
 */
export async function list(options?: { signal?: AbortSignal; window?: string }): Promise<UserMonitorListResponse> {
  const { data } = await apiClient.get<UserMonitorListResponse>('/channel-monitors', {
    signal: options?.signal,
    params: options?.window ? { window: options.window } : undefined,
  })
  return data
}

/**
 * Get detailed status (multi-window availability + latency) for a single monitor.
 */
export async function status(id: number): Promise<UserMonitorDetail> {
  const { data } = await apiClient.get<UserMonitorDetail>(`/channel-monitors/${id}/status`)
  return data
}

export const channelMonitorUserAPI = {
  list,
  status,
}

export default channelMonitorUserAPI
