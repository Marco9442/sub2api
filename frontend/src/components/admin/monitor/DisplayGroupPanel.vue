<template>
  <section class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.channelMonitor.displayGroups.title') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channelMonitor.displayGroups.hint') }}</p>
      </div>
      <button type="button" class="btn btn-primary btn-sm" :disabled="!canSave" @click="save">
        {{ t('common.save') }}
      </button>
    </div>

    <form class="mt-4 flex flex-wrap gap-2" @submit.prevent="createGroup">
      <input
        v-model="newName"
        type="text"
        class="input max-w-xs"
        :placeholder="t('admin.channelMonitor.displayGroups.namePlaceholder')"
      />
      <button type="submit" class="btn btn-secondary btn-sm" :disabled="busy || !newName.trim()">
        {{ t('common.add') }}
      </button>
    </form>

    <p v-if="orphanCount(draft) > 0" class="mt-3 text-xs text-amber-600 dark:text-amber-400">
      {{ t('admin.channelMonitor.displayGroups.orphanBlocked') }}
    </p>

    <div class="mt-4 space-y-3">
      <article
        v-for="section in sections"
        :key="section.key"
        class="rounded-2xl border border-gray-100 p-3 dark:border-dark-700"
      >
        <div class="flex flex-wrap items-center gap-2">
          <template v-if="section.group">
            <input v-model="renameName[section.group.id]" type="text" class="input max-w-xs" />
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="rename(section.group)">
              {{ t('admin.channelMonitor.displayGroups.rename') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || section.groupIndex === 0" :aria-label="t('admin.channelMonitor.displayGroups.moveUp')" @click="moveGroup(section.groupIndex, -1)">
              <Icon name="chevronUp" size="sm" />
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || section.groupIndex === draft.groups.length - 1" :aria-label="t('admin.channelMonitor.displayGroups.moveDown')" @click="moveGroup(section.groupIndex, 1)">
              <Icon name="chevronDown" size="sm" />
            </button>
            <button type="button" class="btn btn-secondary btn-sm text-red-600" :disabled="busy" @click="askDelete(section.group)">
              {{ t('common.delete') }}
            </button>
            <span v-if="disabledOnly(section.monitorIds)" class="text-xs text-gray-500">
              {{ t('admin.channelMonitor.displayGroups.disabledOnlyCount', { n: section.monitorIds.length }) }}
            </span>
          </template>
          <h3 v-else class="text-sm font-medium text-gray-900 dark:text-white">{{ section.title }}</h3>
        </div>

        <ul class="mt-2 space-y-1">
          <li v-for="(id, index) in section.monitorIds" :key="id" class="flex flex-wrap items-center gap-2 text-sm">
            <span :class="monitorOf(id)?.enabled ? 'text-gray-900 dark:text-white' : 'text-gray-400'">
              {{ monitorOf(id)?.name || id }}
            </span>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="index === 0" :aria-label="t('admin.channelMonitor.displayGroups.moveUp')" @click="moveMonitor(section, index, -1)">
              <Icon name="chevronUp" size="xs" />
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="index === section.monitorIds.length - 1" :aria-label="t('admin.channelMonitor.displayGroups.moveDown')" @click="moveMonitor(section, index, 1)">
              <Icon name="chevronDown" size="xs" />
            </button>
            <select class="input max-w-xs py-1" :value="locationKey(id)" @change="onRelocateEvent(id, $event)">
              <option v-if="section.orphan" :value="section.key">{{ section.title }}</option>
              <option value="ungrouped">{{ t('channelStatus.groups.ungrouped') }}</option>
              <option v-for="group in draft.groups" :key="group.id" :value="`group:${group.id}`">{{ group.name }}</option>
            </select>
          </li>
        </ul>
      </article>
    </div>

    <ConfirmDialog
      :show="showDelete"
      :title="t('common.delete')"
      :message="t('admin.channelMonitor.displayGroups.deleteConfirm', { name: deleteTarget?.name ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDelete = false"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ChannelMonitorDisplayGroupView } from '@/api/admin/channelMonitor'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  draftFromAdmin,
  layoutPayload,
  moveWithin,
  orphanCount,
  relocateMonitor,
  type DisplayDraft,
  type DisplayDraftGroup,
  type DisplayLayoutMonitor,
} from '@/utils/channelMonitorDisplayLayout'

const props = defineProps<{ reloadToken?: number }>()
const emit = defineEmits<{
  loaded: [groups: ChannelMonitorDisplayGroupView[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const draft = ref<DisplayDraft>({ groups: [], ungroupedIds: [], orphans: [] })
const monitors = ref(new Map<number, DisplayLayoutMonitor>())
const renameName = ref<Record<number, string>>({})
const newName = ref('')
const dirty = ref(false)
const busy = ref(false)
const showDelete = ref(false)
const deleteTarget = ref<DisplayDraftGroup | null>(null)

const canSave = computed(() => dirty.value && orphanCount(draft.value) === 0 && !busy.value)

interface PanelSection {
  key: string
  title: string
  monitorIds: number[]
  group: DisplayDraftGroup | null
  groupIndex: number
  orphan: boolean
}

const sections = computed<PanelSection[]>(() => [
  ...draft.value.groups.map((group, groupIndex) => ({
    key: `group:${group.id}`,
    title: group.name,
    monitorIds: group.monitorIds,
    group,
    groupIndex,
    orphan: false,
  })),
  {
    key: 'ungrouped',
    title: t('channelStatus.groups.ungrouped'),
    monitorIds: draft.value.ungroupedIds,
    group: null,
    groupIndex: -1,
    orphan: false,
  },
  ...draft.value.orphans.map((orphan) => ({
    key: `orphan:${orphan.name}`,
    title: orphan.name,
    monitorIds: orphan.monitorIds,
    group: null,
    groupIndex: -1,
    orphan: true,
  })),
])

function monitorOf(id: number) {
  return monitors.value.get(id)
}

function disabledOnly(ids: number[]) {
  return ids.length > 0 && ids.every((id) => monitorOf(id)?.enabled === false)
}

function locationKey(id: number) {
  for (const group of draft.value.groups) {
    if (group.monitorIds.includes(id)) return `group:${group.id}`
  }
  if (draft.value.ungroupedIds.includes(id)) return 'ungrouped'
  const orphan = draft.value.orphans.find((item) => item.monitorIds.includes(id))
  return orphan ? `orphan:${orphan.name}` : 'ungrouped'
}

async function load() {
  busy.value = true
  try {
    const view = await adminAPI.channelMonitor.listDisplayGroups()
    const normalized = {
      groups: view.groups ?? [],
      ungrouped_ids: view.ungrouped_ids ?? [],
      monitors: view.monitors ?? [],
    }
    draft.value = draftFromAdmin(normalized)
    monitors.value = new Map(normalized.monitors.map((monitor) => [monitor.id, monitor]))
    renameName.value = Object.fromEntries(normalized.groups.map((group) => [group.id, group.name]))
    dirty.value = false
    emit('loaded', normalized.groups)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displayGroups.loadFailed')))
  } finally {
    busy.value = false
  }
}

function replaceDraft(next: DisplayDraft) {
  draft.value = next
  dirty.value = true
}

function moveGroup(index: number, delta: number) {
  const ids = draft.value.groups.map((group) => group.id)
  const nextIds = moveWithin(ids, index, delta)
  if (nextIds.join(',') === ids.join(',')) return
  const byId = new Map(draft.value.groups.map((group) => [group.id, group]))
  replaceDraft({ ...draft.value, groups: nextIds.map((id) => byId.get(id)!) })
}

function moveMonitor(section: PanelSection, index: number, delta: number) {
  const nextIds = moveWithin(section.monitorIds, index, delta)
  if (nextIds.join(',') === section.monitorIds.join(',')) return
  if (section.group) {
    replaceDraft({
      ...draft.value,
      groups: draft.value.groups.map((group) => group.id === section.group?.id ? { ...group, monitorIds: nextIds } : group),
    })
    return
  }
  if (section.orphan) {
    replaceDraft({
      ...draft.value,
      orphans: draft.value.orphans.map((orphan) => orphan.name === section.title ? { ...orphan, monitorIds: nextIds } : orphan),
    })
    return
  }
  replaceDraft({ ...draft.value, ungroupedIds: nextIds })
}

function onRelocate(id: number, target: string) {
  if (target === locationKey(id)) return
  replaceDraft(relocateMonitor(draft.value, id, target))
}

function onRelocateEvent(id: number, event: Event) {
  const target = event.target
  if (!(target instanceof HTMLSelectElement)) return
  onRelocate(id, target.value)
}

async function runMutation(work: () => Promise<void>, successKey: string) {
  busy.value = true
  try {
    await work()
    appStore.showSuccess(t(successKey))
    busy.value = false
    await load()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displayGroups.saveFailed')))
    busy.value = false
  }
}

async function createGroup() {
  const name = newName.value.trim()
  if (!name || busy.value) return
  await runMutation(async () => {
    await adminAPI.channelMonitor.createDisplayGroup(name)
    newName.value = ''
  }, 'admin.channelMonitor.displayGroups.created')
}

async function rename(group: DisplayDraftGroup) {
  const name = (renameName.value[group.id] ?? '').trim()
  if (!name || name === group.name || busy.value) return
  await runMutation(
    () => adminAPI.channelMonitor.renameDisplayGroup(group.id, name),
    'admin.channelMonitor.displayGroups.renamed',
  )
}

function askDelete(group: DisplayDraftGroup) {
  deleteTarget.value = group
  showDelete.value = true
}

async function confirmDelete() {
  const group = deleteTarget.value
  if (!group) return
  showDelete.value = false
  try {
    await runMutation(
      () => adminAPI.channelMonitor.deleteDisplayGroup(group.id),
      'admin.channelMonitor.displayGroups.deleted',
    )
  } finally {
    deleteTarget.value = null
  }
}

async function save() {
  if (!canSave.value) return
  await runMutation(
    () => adminAPI.channelMonitor.saveDisplayLayout(layoutPayload(draft.value)),
    'admin.channelMonitor.displayGroups.saved',
  )
}

watch(() => props.reloadToken, () => { void load() })
onMounted(() => { void load() })
</script>
