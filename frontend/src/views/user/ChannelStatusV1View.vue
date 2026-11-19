<template>
  <AppLayout>
    <MonitorHero
      :overall-status="overallStatus"
      :interval-seconds="PAGE_REFRESH_SECONDS"
      :window="currentWindow"
      :loading="loading"
      :auto-refresh="autoRefresh"
      :windows="windows"
      :total-count="items.length"
      :degraded-count="degradedCount"
      @update:window="handleWindowChange"
      @refresh="refresh()"
    />

    <MonitorCardGrid
      v-if="grouped.flat"
      :items="items"
      :window="currentWindow"
      :countdown-seconds="countdown"
      :loading="loading"
      :timeline-length="timelinePoints"
      :metric="metric"
      @card-click="openDetail"
    />
    <div v-else class="space-y-7">
      <section v-for="section in grouped.sections" :key="section.key" class="space-y-3">
        <div class="flex min-w-0 items-center gap-2.5">
          <span
            class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gray-100/80 text-gray-500 dark:bg-dark-800 dark:text-gray-400"
          >
            <ProviderIcon v-if="section.provider" :provider="section.provider" :size="16" />
            <Icon v-else name="grid" size="sm" />
          </span>
          <h2 class="min-w-0 break-words text-sm font-semibold text-gray-900 dark:text-white">{{ section.title }}</h2>
          <span class="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            {{ t('channelStatus.groups.count', { n: section.items.length }) }}
          </span>
          <span class="h-px min-w-4 flex-1 bg-gray-200/70 dark:bg-dark-700/70" aria-hidden="true" />
        </div>
        <MonitorCardGrid
          :items="section.items"
          :window="currentWindow"
          :countdown-seconds="countdown"
          :loading="loading"
          :timeline-length="timelinePoints"
          :metric="metric"
          hide-empty
          hide-group-badge
          @card-click="openDetail"
        />
      </section>
    </div>

    <footer
      v-if="items.length > 0"
      class="mt-8 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-gray-100 bg-white/60 px-4 py-3 text-xs text-gray-500 dark:border-dark-700/60 dark:bg-dark-800/40 dark:text-gray-400"
    >
      <span class="font-medium text-gray-600 dark:text-gray-300">{{ t('channelStatus.legend.title') }}</span>
      <span v-for="status in legendStatuses" :key="status" class="inline-flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full" :class="legendColor[status]" />
        {{ t(`monitorCommon.status.${status}`) }}
      </span>
      <span class="inline-flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full bg-gray-300 dark:bg-dark-600" />
        {{ t('channelStatus.legend.noRecord') }}
      </span>
      <span class="ml-auto inline-flex items-center gap-1.5 text-gray-400 dark:text-gray-500">
        <Icon name="clock" size="xs" />
        {{ t('channelStatus.legend.windowHint', { window: t(`channelMonitorCustom.${windowUnitKey}`, { n: windowAmount }) }) }}
      </span>
    </footer>

    <MonitorDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.id ?? null"
      :title="detailTitle"
      :metric="metric"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listChannelMonitorViews,
  type UserMonitorView,
  type UserMonitorGroup,
} from '@/api/channelMonitor'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import MonitorHero, {
  type MonitorWindow,
  type OverallStatus,
} from '@/components/user/monitor/MonitorHero.vue'
import MonitorCardGrid from '@/components/user/monitor/MonitorCardGrid.vue'
import MonitorDetailDialog from '@/components/user/MonitorDetailDialog.vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { legacyOverallStatus, overallMonitorStatus } from '@/utils/channelMonitorStatus'
import { groupMonitorSections } from '@/utils/channelMonitorDisplayGroups'

const props = withDefaults(defineProps<{ metric?: 'latency' | 'ttft' }>(), { metric: 'latency' })
const metric = computed(() => props.metric)

const { t } = useI18n()
const appStore = useAppStore()

// ── State ──
const PAGE_REFRESH_SECONDS = 60
const DEFAULT_TIMELINE_POINTS = 20

const items = ref<UserMonitorView[]>([])
const groups = ref<UserMonitorGroup[]>([])
const loading = ref(false)
const currentWindow = ref<MonitorWindow>('90m')
const windows = ref<{ amount: number; unit: 'm' | 'd' }[]>([])
const timelinePoints = ref(DEFAULT_TIMELINE_POINTS)
const statusYellowPercent = ref(30)
const statusRedPercent = ref(30)
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)

let abortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'channel-status-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: PAGE_REFRESH_SECONDS,
  defaultEnabled: true,
  onRefresh: () => refresh(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

// ── Computed ──
const overallStatus = computed<OverallStatus>(() => {
  const statuses = items.value.map((it) => it.primary_status)
  if (metric.value !== 'ttft') return legacyOverallStatus(statuses)
  return overallMonitorStatus(statuses, statusYellowPercent.value, statusRedPercent.value)
})

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('channelStatus.detailTitle')
})

const grouped = computed(() => groupMonitorSections(items.value, groups.value, t('channelStatus.groups.ungrouped')))
const degradedCount = computed(
  () => items.value.filter((item) => item.primary_status !== 'operational').length,
)

// 顶部与页脚都要按当前档位显示「90 分钟 / 1 天」，统一在视图层解析一次。
const windowAmount = computed(() => {
  const match = /^(\d+)([md])$/.exec(currentWindow.value)
  return match ? Number(match[1]) : 0
})
const windowUnitKey = computed(() => {
  const match = /^(\d+)([md])$/.exec(currentWindow.value)
  return match?.[2] === 'd' ? 'windowDays' : 'windowMinutes'
})
const legendStatuses = ['operational', 'degraded', 'failed', 'error'] as const
const legendColor: Record<(typeof legendStatuses)[number], string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
}

// ── Loaders ──
async function reload(silent = false, windowAdjusted = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  let retryWindow = false
  try {
    const res = await listChannelMonitorViews({
      signal: ctrl.signal,
      window: currentWindow.value,
    })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
    groups.value = res.groups ?? []
    windows.value = res.windows ?? []
    timelinePoints.value = storedPoints(res.timeline_points)
    statusYellowPercent.value = storedPercent(res.status_yellow_percent)
    statusRedPercent.value = storedPercent(res.status_red_percent)
    const keys = windows.value.map((w) => `${w.amount}${w.unit}`)
    if (keys.length > 0 && !keys.includes(currentWindow.value)) {
      currentWindow.value = keys[0]
      retryWindow = !windowAdjusted
    }
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.loadError')))
  } finally {
    if (abortController === ctrl) {
      if (!silent) loading.value = false
      autoRefresh.resetCountdown()
      abortController = null
    }
  }
  if (retryWindow) await reload(silent, true)
}

function storedPercent(value: number | undefined): number {
  return typeof value === 'number' && value > 0 ? value : 30
}

function storedPoints(value: number | undefined): number {
  return typeof value === 'number' && value > 0 ? value : DEFAULT_TIMELINE_POINTS
}

async function refresh(silent = false) {
  await reload(silent)
}

async function handleWindowChange(value: MonitorWindow) {
  currentWindow.value = value
  await reload(true)
}

function openDetail(row: UserMonitorView) {
  detailTarget.value = row
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

watch(
  () => appStore.cachedPublicSettings?.channel_monitor_enabled,
  (enabled) => {
    if (enabled === false) autoRefresh.stop()
    else if (autoRefresh.enabled.value) autoRefresh.start()
  },
)

onMounted(() => {
  void reload(false)
  if (appStore.cachedPublicSettings?.channel_monitor_enabled !== false) {
    autoRefresh.setEnabled(autoRefresh.enabled.value)
  }
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})
</script>
