<template>
  <AccountsView />
  <div v-if="open" class="fixed inset-0 z-50 flex items-end justify-end bg-black/20 p-6" @click.self="open = false">
    <form class="w-full max-w-sm rounded-xl bg-white p-5 shadow-lg dark:bg-dark-800" @submit.prevent="save">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('accountSim.title') }}</h2>
      <label class="mt-4 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="form.enabled" type="checkbox" />
        {{ t('accountSim.enabled') }}
      </label>
      <label class="mt-4 block text-sm text-gray-700 dark:text-gray-200">
        {{ t('accountSim.dailyNew') }}
        <span class="mt-1 flex gap-2">
          <input v-model.number="form.daily_new_min" class="input w-full" type="number" min="0" />
          <input v-model.number="form.daily_new_max" class="input w-full" type="number" min="0" />
        </span>
      </label>
      <label class="mt-4 block text-sm text-gray-700 dark:text-gray-200">
        {{ t('accountSim.lifetime') }}
        <span class="mt-1 flex gap-2">
          <input v-model.number="form.lifetime_days_min" class="input w-full" type="number" min="1" />
          <input v-model.number="form.lifetime_days_max" class="input w-full" type="number" min="1" />
        </span>
      </label>
      <label class="mt-4 block text-sm text-gray-700 dark:text-gray-200">
        {{ t('accountSim.linger') }}
        <span class="mt-1 flex gap-2">
          <input v-model.number="form.linger_days_min" class="input w-full" type="number" min="0" />
          <input v-model.number="form.linger_days_max" class="input w-full" type="number" min="0" />
        </span>
      </label>
      <label class="mt-4 block text-sm text-gray-700 dark:text-gray-200">
        {{ t('accountSim.usageLevel') }}
        <input v-model.number="form.usage_level" class="input mt-1 w-full" type="number" min="0" max="10" step="0.1" />
      </label>
      <p v-if="error" class="mt-3 text-sm text-red-600">{{ error }}</p>
      <p v-else-if="saved" class="mt-3 text-sm text-emerald-600">{{ t('common.saved') }}</p>
      <div class="mt-5 flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="open = false">{{ t('common.cancel') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || !loaded">{{ t('common.save') }}</button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { onMounted, provide, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import { hideGeminiSimulatedNoteKey, openAccountSimConfigKey } from '@/composables/accountSimPage'
import AccountsView from './AccountsView.vue'

interface SimConfig {
  enabled: boolean
  daily_new_min: number
  daily_new_max: number
  lifetime_days_min: number
  lifetime_days_max: number
  linger_days_min: number
  linger_days_max: number
  usage_level: number
  [key: string]: unknown
}

const { t } = useI18n()
const open = ref(false)
provide(openAccountSimConfigKey, () => {
  open.value = true
})
provide(hideGeminiSimulatedNoteKey, true)
const saving = ref(false)
const saved = ref(false)
const loaded = ref(false)
const error = ref('')
const form = reactive<SimConfig>({
  enabled: true,
  daily_new_min: 30,
  daily_new_max: 120,
  lifetime_days_min: 3,
  lifetime_days_max: 15,
  linger_days_min: 1,
  linger_days_max: 3,
  usage_level: 1
})

const load = async () => {
  const { data } = await apiClient.get<SimConfig>('/admin/account-sim/config')
  Object.assign(form, data)
  loaded.value = true
}

const save = async () => {
  if (!loaded.value || saving.value) return
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    const { data } = await apiClient.put<SimConfig>('/admin/account-sim/config', form)
    Object.assign(form, data)
    saved.value = true
  } catch {
    error.value = t('accountSim.saveFailed')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load().catch(() => {
    error.value = t('accountSim.saveFailed')
  })
})
</script>
