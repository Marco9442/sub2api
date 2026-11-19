<template>
  <button
    type="button"
    class="group relative flex w-full flex-col rounded-2xl border border-gray-200/70 bg-white/75 p-4 text-left shadow-card backdrop-blur-xl transition-all duration-200 ease-out hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-card-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:border-dark-700/70 dark:bg-dark-800/60 dark:hover:border-primary-500/30"
    @click="emit('click')"
  >
    <!-- 顶部状态色条：一眼看出整卡健康度，颜色与状态徽标同源 -->
    <span
      aria-hidden="true"
      class="absolute inset-x-4 top-0 h-0.5 rounded-full"
      :class="statusAccentClass"
    />

    <!-- Header: icon + name/model + status chip -->
    <div class="flex items-start gap-3">
      <span
        class="grid h-9 w-9 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradient(item.provider), providerTintClass]"
      >
        <ProviderIcon :provider="item.provider" :size="20" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-base font-semibold text-gray-900 dark:text-gray-100">
          {{ item.name }}
        </div>
        <div class="mt-0.5 flex min-w-0 items-center gap-1.5">
          <span
            class="inline-flex flex-shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
            :class="providerBadgeClass(item.provider)"
          >
            {{ providerLabel(item.provider) }}
          </span>
          <!-- 纯配额模式主模型是占位符 "quota"，展示层替换为本地化「配额」标签 -->
          <span class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">
            {{ formatMonitorModel(item.primary_model) }}
          </span>
          <span
            v-if="item.group_name && !hideGroupBadge"
            class="inline-flex flex-shrink-0 items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
          >
            {{ item.group_name }}
          </span>
        </div>
      </div>
      <span
        class="flex flex-shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold"
        :class="statusBadgeClass(item.primary_status)"
      >
        <span aria-hidden="true" class="h-1.5 w-1.5 rounded-full" :class="statusDotClass" />
        {{ statusLabel(item.primary_status) }}
      </span>
    </div>

    <!-- Metrics -->
    <MonitorMetricPair
      primary-icon="bolt"
      :primary-label="primaryMetricLabel"
      :primary-value="primaryMetricValue"
      :primary-unit="primaryMetricUnit"
      :primary-note="tokenRateNote"
      secondary-icon="clock"
      :secondary-label="t('monitorCommon.availabilityPrefix')"
      :secondary-value="availabilityDisplay"
      :secondary-unit="availabilityUnit"
      :secondary-tone="secondaryTone"
    />
    <p v-if="extraModelsCountLabel" class="mt-1 text-right text-[11px] text-gray-400">
      {{ extraModelsCountLabel }}
    </p>

    <!-- 配额模式：最新用量/余额快照（服务端已按系统开关剥离，此处 flag 为纵深防御） -->
    <MonitorQuotaView v-if="quotaVisible" :snapshot="item.latest_quota" conceal-error class="mt-2" />

    <!-- Divider -->
    <div class="mt-4 border-t border-gray-100 dark:border-dark-700/60"></div>

    <!-- Timeline -->
    <MonitorTimeline
      :buckets="item.timeline"
      :countdown-seconds="countdownSeconds"
      :length="timelineLength"
    />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import {
  useChannelMonitorFormat,
  providerGradient,
  availabilityTone,
} from '@/composables/useChannelMonitorFormat'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import { resolveMonitorMetricKind, tokenRateDisplay } from '@/utils/channelMonitorMetric'
import ProviderIcon from './ProviderIcon.vue'
import MonitorMetricPair from './MonitorMetricPair.vue'
import MonitorTimeline from './MonitorTimeline.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'

// 图标配色与 utils/platformColors.ts 的平台色对齐（新 4 家）。
const PROVIDER_TINT: Record<string, string> = {
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
  grok: 'text-zinc-700 dark:text-zinc-200',
  antigravity: 'text-purple-600 dark:text-purple-300',
  kimi: 'text-pink-600 dark:text-pink-300',
  zhipu: 'text-indigo-600 dark:text-indigo-300',
  deepseek: 'text-teal-600 dark:text-teal-300',
  opencode_go: 'text-amber-700 dark:text-amber-300',
}

const props = withDefaults(defineProps<{
  item: UserMonitorView
  window: string
  availabilityValue: number | null
  countdownSeconds: number
  timelineLength?: number
  metric?: 'latency' | 'ttft'
  hideGroupBadge?: boolean
}>(), { metric: 'latency', hideGroupBadge: false, timelineLength: 20 })

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()
const {
  statusLabel,
  statusBadgeClass,
  providerLabel,
  providerBadgeClass,
  formatLatency,
  formatMonitorModel,
} = useChannelMonitorFormat()

const providerTintClass = computed(() =>
  PROVIDER_TINT[props.item.provider] ?? 'text-gray-500 dark:text-gray-300'
)

// 色条和圆点只用于承载状态色，避免和头部徽标抢注意力。
const statusAccentClass = computed(() => {
  switch (props.item.primary_status) {
    case 'operational':
      return 'bg-emerald-500/60'
    case 'degraded':
      return 'bg-amber-500/70'
    case 'failed':
    case 'error':
      return 'bg-red-500/70'
    default:
      return 'bg-gray-300 dark:bg-dark-600'
  }
})

const statusDotClass = computed(() => {
  switch (props.item.primary_status) {
    case 'operational':
      return 'bg-emerald-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'failed':
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-current opacity-40'
  }
})

const rowKind = computed(() => resolveMonitorMetricKind(props.item.primary_metric_kind, props.metric))
const tokenRateNote = computed(() => {
  const display = tokenRateDisplay(rowKind.value, props.item.primary_latency_ms, props.item.primary_tokens_per_sec)
  if (display === 'omit') return undefined
  const value = display === 'hidden' ? t('channelMonitorCustom.hidden') : props.item.primary_tokens_per_sec?.toFixed(3)
  return `${value} ${t('channelMonitorCustom.tokensPerSec')}`
})
const primaryMetricLabel = computed(() => {
  if (rowKind.value === 'ttft') return t('channelMonitorCustom.ttft')
  if (rowKind.value === 'quota') return t('channelMonitorCustom.quota')
  return t('monitorCommon.dialogLatency')
})
const primaryMetricValue = computed(() => {
  if (rowKind.value === 'quota' || (rowKind.value === 'ttft' && props.item.primary_latency_ms == null)) {
    return t('channelMonitorCustom.hidden')
  }
  return formatLatency(props.item.primary_latency_ms)
})
const primaryMetricUnit = computed(() => (
  rowKind.value === 'quota' || (rowKind.value === 'ttft' && props.item.primary_latency_ms == null) ? '' : 'ms'
))

const quotaVisible = computed(
  () => isChannelMonitorQuotaVisible() && !!props.item.latest_quota
)

const availabilityDisplay = computed(() => {
  if (props.availabilityValue === null || Number.isNaN(props.availabilityValue)) {
    return t('monitorCommon.latencyEmpty')
  }
  return props.availabilityValue.toFixed(2)
})
const availabilityUnit = computed(() => (
  props.availabilityValue === null || Number.isNaN(props.availabilityValue) ? '' : '%'
))

// 可用率按阈值上色：低于 99 提示注意，低于 95 视为明显不可用。
// 复用状态徽标的语义色，与管理表格共用同一套判定。
const secondaryTone = computed(() => availabilityTone(props.availabilityValue))

const extraModelsCountLabel = computed(() => {
  const count = props.item.extra_models?.length ?? 0
  if (count === 0) return undefined
  return t('monitorCommon.extraModelsCount', { n: count })
})
</script>
