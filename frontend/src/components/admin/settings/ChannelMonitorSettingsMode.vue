<template>
  <div
    class="mt-1.5 inline-flex w-full max-w-md rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-900/40"
    data-testid="channel-monitor-settings-mode"
  >
    <button
      type="button"
      data-mode="v2"
      :data-selected="mode === 'v2'"
      class="inline-flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
      :class="
        mode === 'v2'
          ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300'
          : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'
      "
      @click="emit('update:mode', 'v2')"
    >
      {{ t('admin.settings.features.channelMonitor.modeV2') }}
    </button>
    <button
      type="button"
      data-mode="v1"
      :data-selected="mode === 'v1'"
      class="inline-flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
      :class="
        mode === 'v1'
          ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300'
          : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'
      "
      @click="emit('update:mode', 'v1')"
    >
      {{ t('admin.settings.features.channelMonitor.modeV1') }}
    </button>
    <span
      v-if="mode === 'custom'"
      data-mode="custom"
      data-selected="true"
      aria-current="true"
      class="inline-flex flex-1 cursor-default items-center justify-center gap-2 rounded-md bg-white px-3 py-2 text-sm font-medium text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300"
    >
      {{ t('channelMonitorCustom.modeCustom') }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  mode: 'v1' | 'v2' | 'custom'
}>()

const emit = defineEmits<{
  'update:mode': [mode: 'v1' | 'v2']
}>()

const { t } = useI18n()
</script>
