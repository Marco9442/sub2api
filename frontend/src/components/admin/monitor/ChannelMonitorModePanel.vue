<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.previewTitle') }}</span>
        <div class="tabs inline-flex flex-wrap" role="radiogroup" :aria-label="t('channelMonitorCustom.previewTitle')">
          <button
            v-for="mode in modes"
            :key="mode"
            type="button"
            role="radio"
            class="tab inline-flex items-center gap-2"
            :class="preview === mode ? 'tab-active' : ''"
            :aria-checked="preview === mode"
            :data-mode="mode"
            @click="preview = mode"
          >
            {{ t(`channelMonitorCustom.mode${modeLabel[mode]}`) }}
            <span v-if="saved === mode" class="badge badge-success">{{ t('channelMonitorCustom.savedBadge') }}</span>
          </button>
        </div>
      </div>
      <button type="button" class="btn btn-primary" data-test="save-mode" :disabled="!canSave" @click="save">
        {{ saving ? t('channelMonitorCustom.saving') : t('channelMonitorCustom.save') }}
      </button>
    </div>

    <p
      v-if="dirty"
      class="flex items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="sm" class="shrink-0" />
      {{ t('channelMonitorCustom.previewDirty') }}
    </p>
    <p v-else class="input-hint">{{ t('channelMonitorCustom.previewHint') }}</p>

    <!--
      设置按用途分成三块：检测决定请求与阈值，展示决定用户看到的时间档和状态，
      指标处理只改写展示数值。每块用标题隔开，避免一排等高卡片里出现大片空白。
    -->
    <div v-if="preview === 'custom'" class="space-y-4">
      <section data-group="probe" :class="groupClass">
        <header :class="groupHeaderClass">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorCustom.sectionProbe') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('channelMonitorCustom.sectionProbeHint') }}</p>
        </header>
        <div :class="groupBodyClass">
          <div data-section="prompt" :class="rowClass">
            <label class="input-label" for="monitor-custom-prompt">{{ t('channelMonitorCustom.prompt') }}</label>
            <input
              id="monitor-custom-prompt"
              v-model="custom.prompt"
              class="input"
              :class="{ 'input-error': errors.prompt }"
              maxlength="500"
            />
            <p v-if="errors.prompt" class="input-error-text">{{ t('channelMonitorCustom.promptInvalid') }}</p>
            <p v-else class="input-hint">{{ t('channelMonitorCustom.promptHint') }}</p>
          </div>

          <div data-section="thresholds" :class="rowClass">
            <div class="grid gap-3 sm:grid-cols-2">
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.yellow') }}</span>
                <input v-model.number="custom.yellow_seconds" type="number" min="1" class="input" :class="{ 'input-error': errors.thresholds }" />
              </label>
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.red') }}</span>
                <input v-model.number="custom.red_seconds" type="number" min="2" class="input" :class="{ 'input-error': errors.thresholds }" />
              </label>
            </div>
            <p v-if="errors.thresholds" class="input-error-text">{{ t('channelMonitorCustom.thresholdsInvalid') }}</p>
            <p v-else class="input-hint">{{ t('channelMonitorCustom.redHint') }}</p>
          </div>
        </div>
      </section>

      <section data-group="display" :class="groupClass">
        <header :class="groupHeaderClass">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorCustom.sectionDisplay') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('channelMonitorCustom.sectionDisplayHint') }}</p>
        </header>
        <div :class="groupBodyClass">
          <div data-section="windows" :class="rowClass">
            <label class="input-label" for="monitor-custom-windows">{{ t('channelMonitorCustom.windows') }}</label>
            <input
              id="monitor-custom-windows"
              v-model="windowsText"
              class="input"
              :class="{ 'input-error': windowsInvalid }"
              placeholder="90m,1d,3d,7d"
            />
            <p v-if="windowsInvalid" class="input-error-text">{{ t('channelMonitorCustom.windowsInvalid') }}</p>
            <p v-else class="input-hint">{{ t('channelMonitorCustom.windowsHint') }}</p>
          </div>

          <div data-section="status" :class="rowClass">
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.status') }}</h4>
            <p class="input-hint">{{ t('channelMonitorCustom.statusHint') }}</p>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.statusYellow') }}</span>
                <input v-model.number="custom.status_yellow_percent" type="number" min="1" max="100" step="1" class="input" :class="{ 'input-error': errors.status }" />
              </label>
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.statusRed') }}</span>
                <input v-model.number="custom.status_red_percent" type="number" min="1" max="100" step="1" class="input" :class="{ 'input-error': errors.status }" />
              </label>
            </div>
            <p v-if="errors.status" class="input-error-text">{{ t('channelMonitorCustom.statusInvalid') }}</p>
          </div>
        </div>
      </section>

      <section data-group="metrics" :class="groupClass">
        <header :class="groupHeaderClass">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorCustom.sectionMetrics') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('channelMonitorCustom.sectionMetricsHint') }}</p>
        </header>
        <div :class="groupBodyClass">
          <div data-section="ttft" :class="rowClass">
            <div class="flex items-start justify-between gap-4">
              <div>
                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.ttftAdjust') }}</h4>
                <p class="input-hint">{{ t('channelMonitorCustom.ttftHint') }}</p>
              </div>
              <Toggle v-model="custom.ttft_adjust_enabled" :aria-label="t('channelMonitorCustom.ttftAdjust')" />
            </div>
            <div v-if="custom.ttft_adjust_enabled" class="mt-3 grid gap-3 sm:grid-cols-3">
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.ttftMultiplier') }}</span>
                <input v-model.number="custom.ttft_multiplier" type="number" min="0.01" step="0.1" class="input" :class="{ 'input-error': errors.ttft }" />
              </label>
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.ttftMin') }}</span>
                <input v-model.number="custom.ttft_min_ms" type="number" min="0" class="input" :class="{ 'input-error': errors.ttft }" />
              </label>
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.ttftMax') }}</span>
                <input v-model.number="custom.ttft_max_ms" type="number" min="1" class="input" :class="{ 'input-error': errors.ttft }" />
              </label>
            </div>
            <p v-if="errors.ttft" class="input-error-text">{{ t('channelMonitorCustom.ttftInvalid') }}</p>
          </div>

          <div data-section="tps" :class="rowClass">
            <div class="flex items-start justify-between gap-4">
              <div>
                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.tps') }}</h4>
                <p class="input-hint">{{ t('channelMonitorCustom.tpsHint') }}</p>
              </div>
              <Toggle v-model="custom.tps_enabled" :aria-label="t('channelMonitorCustom.tps')" />
            </div>
            <div v-if="custom.tps_enabled" class="mt-3 grid gap-3 sm:grid-cols-2">
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.tpsMin') }}</span>
                <input v-model.number="custom.tps_min" type="number" min="0" step="0.1" class="input" :class="{ 'input-error': errors.tps }" />
              </label>
              <label>
                <span class="input-label">{{ t('channelMonitorCustom.tpsMax') }}</span>
                <input v-model.number="custom.tps_max" type="number" min="0.1" step="0.1" class="input" :class="{ 'input-error': errors.tps }" />
              </label>
            </div>
            <p v-if="errors.tps" class="input-error-text">{{ t('channelMonitorCustom.tpsInvalid') }}</p>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { channelMonitorAPI, type ChannelMonitorCustomConfig } from '@/api/admin/channelMonitor'
import { normalizeChannelMonitorMode, type ChannelMonitorMode } from '@/utils/channelMonitorMode'
import { customConfigErrors, defaultCustomConfig, parseAvailabilityWindows } from '@/utils/channelMonitorCustom'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'

const preview = defineModel<ChannelMonitorMode>('preview', { required: true })
const saved = defineModel<ChannelMonitorMode>('saved', { required: true })

const { t } = useI18n()
const appStore = useAppStore()
const modes: ChannelMonitorMode[] = ['v1', 'v2', 'custom']
const modeLabel: Record<ChannelMonitorMode, string> = { v1: 'V1', v2: 'V2', custom: 'Custom' }
const groupClass = 'rounded-2xl border border-gray-100 dark:border-dark-700'
const groupHeaderClass = 'border-b border-gray-100 px-4 py-3 dark:border-dark-700/60'
const groupBodyClass = 'divide-y divide-gray-100 dark:divide-dark-700/60'
const rowClass = 'px-4 py-4'

// savedCustom 是服务端当前值；custom 是编辑中的草稿，只在 Custom 预览下提交。
const savedCustom = ref<ChannelMonitorCustomConfig>(defaultCustomConfig())
const custom = ref<ChannelMonitorCustomConfig>(defaultCustomConfig())
const saving = ref(false)
// 时间档用文本编辑，保存时解析成数组。空文本表示使用默认档。
const windowsText = ref('')
const savedWindowsText = ref('')

const editingCustom = computed(() => preview.value === 'custom')
const errors = computed(() => (editingCustom.value ? customConfigErrors(custom.value) : {}))
const parsedWindows = computed(() => parseAvailabilityWindows(windowsText.value))
const windowsInvalid = computed(() => editingCustom.value && parsedWindows.value === null)
const dirty = computed(
  () =>
    preview.value !== saved.value ||
    (editingCustom.value &&
      (JSON.stringify(custom.value) !== JSON.stringify(savedCustom.value) ||
        windowsText.value.trim() !== savedWindowsText.value)),
)
const canSave = computed(() => !saving.value && dirty.value && !windowsInvalid.value && Object.keys(errors.value).length === 0)

function applyView(view: { mode: string; custom: Partial<ChannelMonitorCustomConfig> }) {
  saved.value = normalizeChannelMonitorMode(view.mode)
  savedCustom.value = { ...defaultCustomConfig(), ...view.custom }
  custom.value = { ...savedCustom.value }
  const text = (view.custom.availability_windows ?? []).map((w) => `${w.amount}${w.unit}`).join(',')
  savedWindowsText.value = text
  windowsText.value = text
}

async function load() {
  try {
    applyView(await channelMonitorAPI.getMode())
    preview.value = saved.value
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelMonitorCustom.saveFailed')))
  }
}

async function save() {
  if (!canSave.value) return
  saving.value = true
  try {
    applyView(await channelMonitorAPI.updateMode(preview.value, editingCustom.value ? { ...custom.value, availability_windows: parsedWindows.value ?? undefined } : savedCustom.value))
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
