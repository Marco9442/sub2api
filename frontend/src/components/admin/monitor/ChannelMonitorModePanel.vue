<template>
  <section class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorCustom.previewTitle') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('channelMonitorCustom.previewHint') }}</p>
      </div>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
        {{ saving ? t('channelMonitorCustom.saving') : t('channelMonitorCustom.save') }}
      </button>
    </div>

    <div class="mt-4 inline-flex w-full max-w-2xl flex-wrap rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-900/40">
      <button
        v-for="mode in modes"
        :key="mode"
        type="button"
        class="inline-flex flex-1 items-center justify-center rounded-md px-3 py-2 text-sm font-medium"
        :class="preview === mode ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300' : 'text-gray-600 dark:text-dark-300'"
        @click="setPreview(mode)"
      >
        {{ t(`channelMonitorCustom.mode${modeLabel[mode]}`) }}
      </button>
    </div>
    <p v-if="preview !== saved" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
      {{ t('channelMonitorCustom.previewDirty') }}
    </p>

    <div v-if="preview === 'custom'" class="mt-4 grid gap-3 sm:grid-cols-2">
      <label class="text-xs text-gray-500 sm:col-span-2">
        {{ t('channelMonitorCustom.prompt') }}
        <input v-model="custom.prompt" class="input mt-1" maxlength="500" />
      </label>
      <label class="text-xs text-gray-500">
        {{ t('channelMonitorCustom.yellow') }}
        <input v-model.number="custom.yellow_seconds" type="number" min="1" class="input mt-1" />
      </label>
      <label class="text-xs text-gray-500">
        {{ t('channelMonitorCustom.red') }}
        <input v-model.number="custom.red_seconds" type="number" min="2" class="input mt-1" />
      </label>
      <p class="text-xs text-gray-400 sm:col-span-2">{{ t('channelMonitorCustom.redHint') }}</p>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="custom.ttft_adjust_enabled" type="checkbox" />
        {{ t('channelMonitorCustom.ttftAdjust') }}
      </label>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="custom.tps_enabled" type="checkbox" />
        {{ t('channelMonitorCustom.tps') }}
      </label>
      <template v-if="custom.ttft_adjust_enabled">
        <label class="text-xs text-gray-500">
          {{ t('channelMonitorCustom.ttftMultiplier') }}
          <input v-model.number="custom.ttft_multiplier" type="number" min="0.01" step="0.1" class="input mt-1" />
        </label>
        <label class="text-xs text-gray-500">
          {{ t('channelMonitorCustom.ttftMin') }}
          <input v-model.number="custom.ttft_min_ms" type="number" min="0" class="input mt-1" />
        </label>
        <label class="text-xs text-gray-500">
          {{ t('channelMonitorCustom.ttftMax') }}
          <input v-model.number="custom.ttft_max_ms" type="number" min="1" class="input mt-1" />
        </label>
      </template>
      <template v-if="custom.tps_enabled">
        <label class="text-xs text-gray-500">
          {{ t('channelMonitorCustom.tpsMin') }}
          <input v-model.number="custom.tps_min" type="number" min="0" step="0.1" class="input mt-1" />
        </label>
        <label class="text-xs text-gray-500">
          {{ t('channelMonitorCustom.tpsMax') }}
          <input v-model.number="custom.tps_max" type="number" min="0.1" step="0.1" class="input mt-1" />
        </label>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { channelMonitorAPI, type ChannelMonitorCustomConfig } from '@/api/admin/channelMonitor'
import { normalizeChannelMonitorMode, type ChannelMonitorMode } from '@/utils/channelMonitorMode'

const preview = defineModel<ChannelMonitorMode>('preview', { required: true })
const saved = defineModel<ChannelMonitorMode>('saved', { required: true })

const { t } = useI18n()
const appStore = useAppStore()
const modes: ChannelMonitorMode[] = ['v1', 'v2', 'custom']
const modeLabel: Record<ChannelMonitorMode, string> = { v1: 'V1', v2: 'V2', custom: 'Custom' }

function defaultCustom(): ChannelMonitorCustomConfig {
  return {
    prompt: 'ping',
    yellow_seconds: 2,
    red_seconds: 8,
    ttft_adjust_enabled: false,
    ttft_multiplier: 1,
    ttft_min_ms: 0,
    ttft_max_ms: 0,
    tps_enabled: false,
    tps_min: 0,
    tps_max: 0,
  }
}

const custom = ref<ChannelMonitorCustomConfig>(defaultCustom())
const saving = ref(false)

function setPreview(mode: ChannelMonitorMode) {
  preview.value = mode
}

async function load() {
  try {
    const view = await channelMonitorAPI.getMode()
    saved.value = normalizeChannelMonitorMode(view.mode)
    preview.value = saved.value
    custom.value = { ...defaultCustom(), ...view.custom }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelMonitorCustom.saveFailed')))
  }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const view = await channelMonitorAPI.updateMode(preview.value, custom.value)
    saved.value = normalizeChannelMonitorMode(view.mode)
    custom.value = { ...defaultCustom(), ...view.custom }
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('channelMonitorCustom.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelMonitorCustom.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
