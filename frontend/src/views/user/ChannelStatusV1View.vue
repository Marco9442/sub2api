<template>
  <AppLayout>
    <MonitorHero
      :overall-status="overallStatus"
      :interval-seconds="DEFAULT_INTERVAL_SECONDS"
      :window="currentWindow"
      :loading="loading"
      :auto-refresh="autoRefresh"
      :windows="windows"
      @update:window="handleWindowChange"
      @refresh="refresh()"
    />

    <MonitorCardGrid
      v-if="grouped.flat"
      :items="items"
      :window="currentWindow"
      :countdown-seconds="countdown"
      :loading="loading"
      :detail-cache="detailCache"
      :metric="metric"
      @card-click="openDetail"
    />
    <div v-else class="space-y-6">
      <section v-for="section in grouped.sections" :key="section.key" class="space-y-3">
        <div class="flex min-w-0 items-center gap-2">
          <span class="inline-flex shrink-0">
            <ProviderIcon v-if="section.provider" :provider="section.provider" :size="20" />
            <Icon v-else name="grid" size="sm" />
          </span>
          <h2 class="min-w-0 break-words text-sm font-semibold text-gray-900 dark:text-white">{{ section.title }}</h2>
          <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ t('channelStatus.groups.count', { n: section.items.length }) }}</span>
        </div>
        <MonitorCardGrid
          :items="section.items"
          :window="currentWindow"
          :countdown-seconds="countdown"
          :loading="loading"
          :detail-cache="detailCache"
          :metric="metric"
          hide-empty
          hide-group-badge
          @card-click="openDetail"
        />
      </section>
    </div>

    <footer v-if="items.length > 0" class="mt-8 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-gray-500 dark:text-gray-400">
      <span v-for="status in legendStatuses" :key="status" class="inline-flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full" :class="legendColor[status]" />
        {{ t(`monitorCommon.status.${status}`) }}
      </span>
      <span class="inline-flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full bg-gray-300 dark:bg-dark-600" />
        {{ t('channelStatus.legend.noRecord') }}
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
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listChannelMonitorViews,
  status as fetchChannelMonitorDetail,
  type UserMonitorView,
  type UserMonitorDetail,
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
import { DEFAULT_INTERVAL_SECONDS } from '@/constants/channelMonitor'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { legacyOverallStatus, overallMonitorStatus } from '@/utils/channelMonitorStatus'
import { groupMonitorSections } from '@/utils/channelMonitorDisplayGroups'

const props = withDefaults(defineProps<{ metric?: 'latency' | 'ttft' }>(), { metric: 'latency' })
const metric = computed(() => props.metric)

const { t } = useI18n()
const appStore = useAppStore()

// ── State ──
const items = ref<UserMonitorView[]>([])
const groups = ref<UserMonitorGroup[]>([])
const loading = ref(false)
const currentWindow = ref<MonitorWindow>('7d')
// Custom 模式下由管理员配置的时间档；为空时沿用 7/15/30 天。
const windows = ref<{ amount: number; unit: 'm' | 'd' }[]>([])
const statusYellowPercent = ref(30)
const statusRedPercent = ref(30)
const customWindows = computed(() => metric.value === 'ttft')
const detailCache = reactive<Record<number, UserMonitorDetail>>({})
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)

let abortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'channel-status-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  defaultEnabled: true,
  onRefresh: () => refresh(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

// ── Computed ──
const overallStatus = computed<OverallStatus>(() => {
  const statuses = items.value.map((it) => it.primary_status)
  if (!customWindows.value) return legacyOverallStatus(statuses)
  return overallMonitorStatus(statuses, statusYellowPercent.value, statusRedPercent.value)
})

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('channelStatus.detailTitle')
})

const grouped = computed(() => groupMonitorSections(items.value, groups.value, t('channelStatus.groups.ungrouped')))
const legendStatuses = ['operational', 'degraded', 'failed', 'error'] as const
const legendColor: Record<(typeof legendStatuses)[number], string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
}

// ── Loaders ──
async function reload(silent = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  try {
    const res = await listChannelMonitorViews({
      signal: ctrl.signal,
      window: customWindows.value ? currentWindow.value : undefined,
    })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
    groups.value = res.groups ?? []
    windows.value = res.windows ?? []
    statusYellowPercent.value = storedPercent(res.status_yellow_percent)
    statusRedPercent.value = storedPercent(res.status_red_percent)
    syncCustomWindow()
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
}

// 所选档不在配置里时（首次进入或管理员改了配置），切到第一档，与后端回退一致。
function storedPercent(value: number | undefined): number {
  return typeof value === 'number' && value > 0 ? value : 30
}

function syncCustomWindow() {
  if (!customWindows.value || windows.value.length === 0) return
  const keys = windows.value.map((w) => `${w.amount}${w.unit}`)
  if (!keys.includes(currentWindow.value)) currentWindow.value = keys[0]
}

async function refresh(silent = false) {
  await reload(silent)
  if (customWindows.value) return
  // After base reload, refresh any cached detail records so non-7d availability
  // values stay in sync without forcing the user to switch tabs again.
  if (currentWindow.value !== '7d') {
    await Promise.all(items.value.map(it => loadDetail(it.id, true)))
  }
}

async function loadDetail(id: number, force = false) {
  if (!force && detailCache[id]) return
  try {
    detailCache[id] = await fetchChannelMonitorDetail(id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
  }
}

async function ensureDetailsForWindow() {
  if (customWindows.value || currentWindow.value === '7d') return
  await Promise.all(items.value.map(it => loadDetail(it.id)))
}

// ── Handlers ──
async function handleWindowChange(value: MonitorWindow) {
  currentWindow.value = value
  if (customWindows.value) {
    await reload(true)
    return
  }
  await ensureDetailsForWindow()
}

function openDetail(row: UserMonitorView) {
  detailTarget.value = row
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

watch(items, () => {
  void ensureDetailsForWindow()
})

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
