<template>
  <section class="pb-4 pt-1">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
      <!-- 左侧：整体状态，权重最高，放在视觉起点而不是挤在右端 -->
      <span
        class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold"
        :class="overallChipClass"
      >
        <span class="h-1.5 w-1.5 rounded-full" :class="overallDotClass" />
        {{ overallLabel }}
      </span>

      <!-- 窄容器下摘要会先被挤压，直接让位给时间档与刷新操作。 -->
      <span class="hidden min-w-0 flex-1 truncate text-xs text-gray-400 sm:inline dark:text-gray-500">
        {{ summaryLabel }}
      </span>

      <!-- 右侧：时间档与刷新操作作为一组，窄屏一起换行，不会把刷新按钮单独挤到下一行 -->
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <div
          role="tablist"
          :aria-label="t('channelStatus.windowTab.label')"
          class="inline-flex rounded-xl border border-gray-200/70 bg-gray-100/80 p-0.5 text-xs dark:border-dark-700/60 dark:bg-dark-800"
        >
          <button
            v-for="opt in windowOptions"
            :key="opt.value"
            type="button"
            role="tab"
            :aria-selected="window === opt.value"
            class="rounded-lg px-3 py-1 transition-colors"
            :class="window === opt.value
              ? 'bg-white font-semibold text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
            @click="emit('update:window', opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>

        <div class="flex items-center gap-1">
          <button
            type="button"
            class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="emit('refresh')"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>

          <AutoRefreshButton
            v-if="autoRefresh"
            :enabled="autoRefresh.enabled.value"
            :interval-seconds="autoRefresh.intervalSeconds.value"
            :countdown="autoRefresh.countdown.value"
            :intervals="autoRefresh.intervals"
            @update:enabled="autoRefresh.setEnabled"
            @update:interval="autoRefresh.setInterval"
          />
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { defaultAvailabilityWindows, windowLabel } from '@/utils/channelMonitorCustom'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import AutoRefreshButton from '@/components/common/AutoRefreshButton.vue'
export type MonitorWindow = string
export type OverallStatus = 'operational' | 'degraded' | 'failed'

const props = defineProps<{
  overallStatus: OverallStatus
  intervalSeconds: number
  window: MonitorWindow
  loading: boolean
  /** 当前时间档下的渠道总数，用于右侧摘要 */
  totalCount?: number
  /** 当前时间档内未全部正常的渠道数 */
  degradedCount?: number
  windows?: { amount: number; unit: 'm' | 'd' }[]
  autoRefresh?: {
    enabled: { value: boolean }
    intervalSeconds: { value: number }
    countdown: { value: number }
    intervals: readonly number[]
    setEnabled: (v: boolean) => void
    setInterval: (v: number) => void
  }
}>()

const emit = defineEmits<{
  (e: 'update:window', value: MonitorWindow): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const windowOptions = computed<{ value: MonitorWindow; label: string }[]>(() => {
  const source = props.windows && props.windows.length > 0 ? props.windows : defaultAvailabilityWindows
  return source.map((w) => {
    const value = `${w.amount}${w.unit}`
    return { value, label: windowLabel(value, t) }
  })
})

const overallLabel = computed(() => t(`channelStatus.overall.${props.overallStatus}`))

/** 一行说明当前档位的覆盖面，避免用户只能靠徽标猜测规模。 */
const summaryLabel = computed(() => {
  const total = props.totalCount ?? 0
  if (total === 0) return ''
  const issues = props.degradedCount ?? 0
  if (issues <= 0) return t('channelStatus.summary.allHealthy', { total })
  return t('channelStatus.summary.withIssues', { total, issues })
})

const overallChipClass = computed(() => {
  switch (props.overallStatus) {
    case 'operational':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'degraded':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    default:
      return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
  }
})

const overallDotClass = computed(() => {
  switch (props.overallStatus) {
    case 'operational':
      return 'bg-emerald-500 animate-pulse'
    case 'degraded':
      return 'bg-amber-500 animate-pulse'
    default:
      return 'bg-red-500 animate-pulse'
  }
})
</script>
