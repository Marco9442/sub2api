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

    <div v-if="preview === 'custom'" class="grid gap-4 lg:grid-cols-2">
      <section data-section="prompt" :class="groupClass">
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
      </section>

      <section data-section="thresholds" :class="groupClass">
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
      </section>

      <section data-section="windows" :class="groupClass">
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
      </section>

      <section data-section="ttft" :class="groupClass">
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.ttftAdjust') }}</h3>
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
      </section>

      <section data-section="tps" :class="groupClass">
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.tps') }}</h3>
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
      </section>

      <section data-section="status" :class="groupClass">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('channelMonitorCustom.status') }}</h3>
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
const groupClass = 'rounded-2xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/30'

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
