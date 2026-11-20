<template>
  <!--
    列数由内容区实际宽度决定，而不是视口宽度：侧边栏展开时内容区可能只有
    视口的一半宽，按视口取断点会把卡片压成竖条。
  -->
  <div class="[--card-min:20rem]">
    <div
      v-if="loading && items.length === 0"
      class="grid gap-4"
      :style="gridStyle"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="animate-pulse rounded-2xl border border-gray-200/70 bg-white/70 p-4 dark:border-dark-700/70 dark:bg-dark-800/60"
      >
        <div class="flex items-start gap-3">
          <div class="h-9 w-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-4 grid grid-cols-2 gap-2">
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="!hideEmpty && items.length === 0"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />

    <div v-else class="grid gap-4" :style="gridStyle">
      <MonitorCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :window="window"
        :availability-value="resolveAvailability(item)"
        :countdown-seconds="countdownSeconds"
        :timeline-length="timelineLength"
        :metric="metric"
        :hide-group-badge="hideGroupBadge"
        @click="emit('cardClick', item)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorCard from './MonitorCard.vue'

withDefaults(defineProps<{
  items: UserMonitorView[]
  window: string
  countdownSeconds: number
  loading: boolean
  timelineLength?: number
  metric?: 'latency' | 'ttft'
  hideEmpty?: boolean
  hideGroupBadge?: boolean
}>(), { metric: 'latency', hideEmpty: false, hideGroupBadge: false, timelineLength: 20 })

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()

// auto-fill 保留空列位：容器窄就少列，宽就多列，不需要为侧边栏展开
// 状态单独维护一套断点。用 auto-fill 而不是 auto-fit，单卡分组不会被
// 拉到整行宽，各分组之间卡片宽度保持一致。
const gridStyle = { gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 20rem), 1fr))' }

function resolveAvailability(item: UserMonitorView): number | null {
  return item.window_availability ?? null
}
</script>

