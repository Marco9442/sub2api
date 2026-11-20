<template>
  <div class="mt-4 grid grid-cols-2 gap-2">
    <div
      data-testid="metric-primary"
      class="rounded-xl border border-gray-100 bg-gray-50/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40"
    >
      <div class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
        <Icon :name="primaryIcon" size="xs" />
        <span>{{ primaryLabel }}</span>
      </div>
      <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-gray-900 dark:text-gray-100">
        {{ primaryValue }}<span class="ml-0.5 text-xs font-normal text-gray-400">{{ primaryUnit }}</span>
      </div>
      <div
        v-if="primaryNote"
        data-testid="metric-primary-note"
        class="mt-0.5 font-mono text-xs tabular-nums text-gray-500 dark:text-gray-400"
      >
        {{ primaryNote }}
      </div>
    </div>
    <div
      data-testid="metric-secondary"
      class="rounded-xl border border-gray-100 bg-gray-50/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40"
    >
      <div class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
        <Icon :name="secondaryIcon" size="xs" />
        <span>{{ secondaryLabel }}</span>
      </div>
      <div
        class="mt-1.5 flex items-baseline font-mono text-lg font-bold tabular-nums"
        :class="toneClass"
      >
        {{ secondaryValue }}<span class="ml-0.5 text-xs font-normal opacity-70">{{ secondaryUnit }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  primaryLabel: string
  primaryValue: string
  primaryUnit: string
  primaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
  /** 主指标下方的附注，例如 TTFT 对应的 token/s */
  primaryNote?: string
  secondaryLabel: string
  secondaryValue: string
  secondaryUnit: string
  secondaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
  /** 副指标语义色。默认中性，可用率等指标按阈值取 success/warning/danger */
  secondaryTone?: 'neutral' | 'success' | 'warning' | 'danger' | 'muted'
}>(), { secondaryTone: 'neutral' })

const toneClass = computed(() => {
  switch (props.secondaryTone) {
    case 'success':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'warning':
      return 'text-amber-600 dark:text-amber-400'
    case 'danger':
      return 'text-red-600 dark:text-red-400'
    case 'muted':
      return 'text-gray-400 dark:text-gray-500'
    default:
      return 'text-gray-900 dark:text-gray-100'
  }
})
</script>
