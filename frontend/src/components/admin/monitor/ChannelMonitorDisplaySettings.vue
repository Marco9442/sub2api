<template>
  <section data-test="display-settings">
    <header class="mb-4">
      <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
        <Icon name="cog" size="sm" class="text-gray-400" />
        {{ t('admin.channelMonitor.displaySettings.title') }}
      </h2>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channelMonitor.displaySettings.hint') }}</p>
    </header>
    <div class="grid gap-3 sm:grid-cols-2">
      <label>
        <span class="input-label" for="monitor-default-interval">{{ t('admin.channelMonitor.displaySettings.interval') }}</span>
        <input
          id="monitor-default-interval"
          v-model.number="intervalSeconds"
          type="number"
          min="15"
          max="3600"
          class="input"
          :disabled="loading || saving"
        />
        <p class="input-hint">{{ t('admin.channelMonitor.displaySettings.intervalHint') }}</p>
      </label>
      <label>
        <span class="input-label" for="monitor-timeline-points">{{ t('admin.channelMonitor.displaySettings.timeline') }}</span>
        <input
          id="monitor-timeline-points"
          v-model.number="timelinePoints"
          type="number"
          min="1"
          max="200"
          class="input"
          :disabled="loading || saving"
        />
        <p class="input-hint">{{ t('admin.channelMonitor.displaySettings.timelineHint') }}</p>
      </label>
    </div>
    <div class="mt-3 flex flex-wrap items-center gap-3">
      <button type="button" class="btn btn-primary" data-test="save-display-settings" :disabled="!canSave" @click="save">
        {{ t('common.save') }}
      </button>
      <p v-if="invalid" class="input-error-text">{{ t('admin.channelMonitor.displaySettings.invalid') }}</p>
    </div>
    <p v-if="loadFailed" class="mt-3 flex flex-wrap items-center gap-2 text-xs text-red-600 dark:text-red-400">
      <span>{{ t('admin.channelMonitor.displaySettings.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" data-test="display-settings-retry" :disabled="loading || saving" @click="load">
        {{ t('admin.channelMonitor.displaySettings.loadRetry') }}
      </button>
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getDisplaySettings, saveDisplaySettings } from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const intervalSeconds = ref(300)
const timelinePoints = ref(20)
const savedInterval = ref(300)
const savedPoints = ref(20)
const loading = ref(false)
const saving = ref(false)
const loaded = ref(false)
const loadFailed = ref(false)

function inRange(value: number, min: number, max: number): boolean {
  return Number.isInteger(value) && value >= min && value <= max
}

const invalid = computed(() => !inRange(intervalSeconds.value, 15, 3600) || !inRange(timelinePoints.value, 1, 200))
const dirty = computed(() => intervalSeconds.value !== savedInterval.value || timelinePoints.value !== savedPoints.value)
const canSave = computed(() => loaded.value && !loading.value && !saving.value && !invalid.value && dirty.value)

function apply(interval: number, points: number) {
  intervalSeconds.value = interval
  timelinePoints.value = points
  savedInterval.value = interval
  savedPoints.value = points
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const settings = await getDisplaySettings()
    apply(settings.default_interval_seconds, settings.timeline_points)
    loaded.value = true
  } catch (err: unknown) {
    loaded.value = false
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displaySettings.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!canSave.value) return
  saving.value = true
  try {
    const settings = await saveDisplaySettings({
      default_interval_seconds: intervalSeconds.value,
      timeline_points: timelinePoints.value,
    })
    apply(settings.default_interval_seconds, settings.timeline_points)
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('admin.channelMonitor.displaySettings.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displaySettings.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
