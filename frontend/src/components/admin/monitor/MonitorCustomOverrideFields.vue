<template>
  <div class="space-y-4">
    <section v-for="group in groups" :key="group.key" :data-section="group.key" class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t(group.title) }}</span>
        <div class="tabs inline-flex" role="radiogroup" :aria-label="t(group.title)">
          <button
            v-for="choice in choices"
            :key="choice.key"
            type="button"
            role="radio"
            class="tab px-3 py-1 text-xs"
            :class="choiceOf(group.toggle) === choice.key ? 'tab-active' : ''"
            :aria-checked="choiceOf(group.toggle) === choice.key"
            :data-choice="choice.key"
            @click="update(group.toggle, choice.value)"
          >
            {{ t(choice.label) }}
          </button>
        </div>
      </div>
      <div class="grid gap-3" :class="group.fields.length === 3 ? 'sm:grid-cols-3' : 'sm:grid-cols-2'">
        <label v-for="field in group.fields" :key="field.key">
          <span class="input-label">{{ t(field.label) }}</span>
          <input
            type="number"
            class="input"
            :step="field.step"
            :value="model[field.key] ?? ''"
            :placeholder="t('channelMonitorCustom.overrideInherit')"
            @input="update(field.key, parseNumber($event))"
          />
        </label>
      </div>
    </section>

    <button
      v-if="hasCustomOverride(model)"
      type="button"
      data-test="reset-override"
      class="text-xs font-medium text-primary-600 hover:underline dark:text-primary-400"
      @click="model = {}"
    >
      {{ t('channelMonitorCustom.overrideReset') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChannelMonitorCustomOverride } from '@/api/admin/channelMonitor'
import { hasCustomOverride } from '@/utils/channelMonitorCustom'

type ToggleKey = 'ttft_adjust_enabled' | 'tps_enabled'
type NumberKey = Exclude<keyof ChannelMonitorCustomOverride, ToggleKey>

// 每个字段为 null/缺省时继承全局设置；空对象表示整条覆盖清除。
const model = defineModel<ChannelMonitorCustomOverride>({ required: true })
const { t } = useI18n()

const choices = [
  { key: 'inherit', value: null, label: 'channelMonitorCustom.overrideInherit' },
  { key: 'on', value: true, label: 'channelMonitorCustom.overrideOn' },
  { key: 'off', value: false, label: 'channelMonitorCustom.overrideOff' },
] as const

const groups: { key: string; title: string; toggle: ToggleKey; fields: { key: NumberKey; label: string; step: string }[] }[] = [
  {
    key: 'ttft',
    title: 'channelMonitorCustom.ttftAdjust',
    toggle: 'ttft_adjust_enabled',
    fields: [
      { key: 'ttft_multiplier', label: 'channelMonitorCustom.ttftMultiplier', step: '0.1' },
      { key: 'ttft_min_ms', label: 'channelMonitorCustom.ttftMin', step: '1' },
      { key: 'ttft_max_ms', label: 'channelMonitorCustom.ttftMax', step: '1' },
    ],
  },
  {
    key: 'tps',
    title: 'channelMonitorCustom.tps',
    toggle: 'tps_enabled',
    fields: [
      { key: 'tps_min', label: 'channelMonitorCustom.tpsMin', step: '0.1' },
      { key: 'tps_max', label: 'channelMonitorCustom.tpsMax', step: '0.1' },
    ],
  },
]

function choiceOf(key: ToggleKey) {
  const value = model.value[key]
  return value == null ? 'inherit' : value ? 'on' : 'off'
}

function update(key: keyof ChannelMonitorCustomOverride, value: boolean | number | null) {
  model.value = { ...model.value, [key]: value }
}

function parseNumber(event: Event): number | null {
  const raw = (event.target as HTMLInputElement).value.trim()
  const parsed = Number(raw)
  return raw === '' || !Number.isFinite(parsed) ? null : parsed
}
</script>
