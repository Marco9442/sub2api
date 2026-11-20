<template>
  <section class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
          <Icon name="grid" size="sm" class="text-gray-400" />
          {{ t('admin.channelMonitor.displayGroups.title') }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channelMonitor.displayGroups.hint') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <span
          v-if="dirty"
          class="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-300"
        >
          {{ t('admin.channelMonitor.displayGroups.unsaved') }}
        </span>
        <button type="button" class="btn btn-primary btn-sm" :disabled="!canSave" @click="save">
          {{ t('common.save') }}
        </button>
      </div>
    </div>

    <form class="mt-4 flex flex-wrap items-center gap-2" @submit.prevent="createGroup">
      <input
        v-model="newName"
        type="text"
        class="input max-w-xs"
        :placeholder="t('admin.channelMonitor.displayGroups.namePlaceholder')"
      />
      <button type="submit" class="btn btn-secondary btn-sm" :disabled="layoutLocked || !newName.trim()">
        <Icon name="plus" size="xs" class="mr-1" />
        {{ t('common.add') }}
      </button>
    </form>

    <p v-if="orphanCount(draft) > 0" class="mt-3 text-xs text-amber-600 dark:text-amber-400">
      {{ t('admin.channelMonitor.displayGroups.orphanBlocked') }}
    </p>
    <p v-if="loadFailed" class="mt-3 flex flex-wrap items-center gap-2 text-xs text-red-600 dark:text-red-400">
      <span>{{ t('admin.channelMonitor.displayGroups.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm display-group-retry" :disabled="reading || writing" @click="load">
        {{ t('admin.channelMonitor.displayGroups.loadRetry') }}
      </button>
    </p>

    <div class="mt-4 space-y-3">
      <article
        v-for="section in sections"
        :key="section.key"
        class="rounded-2xl border border-gray-100 dark:border-dark-700"
        :class="section.orphan ? 'bg-amber-50/40 dark:bg-amber-500/5' : 'bg-gray-50/40 dark:bg-dark-900/20'"
      >
        <!-- 分组头：名称即输入框，操作收进右侧一组图标按钮 -->
        <header class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700/70">
          <template v-if="section.group">
            <input
              v-model="renameName[section.group.id]"
              type="text"
              class="input max-w-xs py-1 text-sm font-medium"
              :aria-label="t('admin.channelMonitor.displayGroups.rename')"
            />
            <span class="rounded-md bg-white px-1.5 py-0.5 text-[11px] text-gray-500 ring-1 ring-gray-200/70 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-700">
              {{ t('channelStatus.groups.count', { n: section.monitorIds.length }) }}
            </span>
            <span v-if="disabledOnly(section.monitorIds)" class="text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('admin.channelMonitor.displayGroups.disabledOnlyCount', { n: section.monitorIds.length }) }}
            </span>
            <div class="ml-auto flex items-center gap-1">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="layoutLocked" @click="rename(section.group)">
                {{ t('admin.channelMonitor.displayGroups.rename') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="layoutLocked || section.groupIndex === 0"
                :aria-label="t('admin.channelMonitor.displayGroups.moveUp')"
                @click="moveGroup(section.groupIndex, -1)"
              >
                <Icon name="chevronUp" size="sm" />
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="layoutLocked || section.groupIndex === draft.groups.length - 1"
                :aria-label="t('admin.channelMonitor.displayGroups.moveDown')"
                @click="moveGroup(section.groupIndex, 1)"
              >
                <Icon name="chevronDown" size="sm" />
              </button>
              <button type="button" class="btn btn-secondary btn-sm text-red-600" :disabled="layoutLocked" @click="askDelete(section.group)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>
          <template v-else>
            <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ section.title }}</h3>
            <span class="rounded-md bg-white px-1.5 py-0.5 text-[11px] text-gray-500 ring-1 ring-gray-200/70 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-700">
              {{ t('channelStatus.groups.count', { n: section.monitorIds.length }) }}
            </span>
            <p v-if="section.orphan" class="ml-auto text-[11px] text-amber-600 dark:text-amber-400">
              {{ t('admin.channelMonitor.displayGroups.orphanHint') }}
            </p>
          </template>
        </header>

        <p
          v-if="section.monitorIds.length === 0"
          class="px-3 py-3 text-xs text-gray-400 dark:text-gray-500"
        >
          {{ t('admin.channelMonitor.displayGroups.emptyGroup') }}
        </p>

        <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700/60">
          <li
            v-for="(id, index) in section.monitorIds"
            :key="id"
            class="group flex flex-wrap items-center gap-2 px-3 py-1.5 text-sm"
          >
            <span
              class="min-w-0 flex-1 truncate"
              :class="monitorOf(id)?.enabled ? 'text-gray-900 dark:text-white' : 'text-gray-400 dark:text-gray-500'"
            >
              {{ monitorOf(id)?.name || id }}
            </span>
            <span
              v-if="monitorOf(id) && !monitorOf(id)!.enabled"
              class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-500 dark:bg-dark-700 dark:text-gray-400"
            >
              {{ t('admin.channelMonitor.displayGroups.disabledTag') }}
            </span>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="layoutLocked || index === 0"
              :aria-label="t('admin.channelMonitor.displayGroups.moveUp')"
              @click="moveMonitor(section, index, -1)"
            >
              <Icon name="chevronUp" size="xs" />
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="layoutLocked || index === section.monitorIds.length - 1"
              :aria-label="t('admin.channelMonitor.displayGroups.moveDown')"
              @click="moveMonitor(section, index, 1)"
            >
              <Icon name="chevronDown" size="xs" />
            </button>
            <select
              class="input max-w-[10rem] py-1 text-xs"
              :disabled="layoutLocked"
              :value="locationKey(id)"
              :aria-label="t('admin.channelMonitor.displayGroups.relocate')"
              @change="onRelocateEvent(id, $event)"
            >
              <option v-if="section.orphan" :value="section.key">{{ section.title }}</option>
              <option value="ungrouped">{{ t('channelStatus.groups.ungrouped') }}</option>
              <option v-for="group in draft.groups" :key="group.id" :value="`group:${group.id}`">{{ group.name }}</option>
            </select>
          </li>
        </ul>
      </article>

      <p v-if="sections.length === 0" class="rounded-2xl border border-dashed border-gray-200 px-4 py-8 text-center text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
        {{ t('admin.channelMonitor.displayGroups.emptyState') }}
      </p>
    </div>

    <!-- 监控表单也是 50。这里必须更高，否则确认被表单挡住。 -->
    <ConfirmDialog
      :show="showDiscard"
      :title="t('admin.channelMonitor.displayGroups.discardTitle')"
      :message="t('admin.channelMonitor.displayGroups.discardMessage')"
      :confirm-text="t('admin.channelMonitor.displayGroups.discardAction')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      :z-index="60"
      @confirm="confirmDiscard"
      @cancel="cancelDiscard"
    />
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
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
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
  changed: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const draft = ref<DisplayDraft>({ groups: [], ungroupedIds: [], orphans: [] })
const monitors = ref(new Map<number, DisplayLayoutMonitor>())
const renameName = ref<Record<number, string>>({})
const newName = ref('')
const dirty = ref(false)
const writing = ref(false)
const reading = ref(false)
const loadFailed = ref(false)
const held = ref(false)
const showDelete = ref(false)
const showDiscard = ref(false)
const deleteTarget = ref<DisplayDraftGroup | null>(null)
const pendingAction = ref<PendingAction | null>(null)
let loadController: AbortController | null = null
let externalDecision: ((allowed: boolean) => void) | null = null

const layoutLocked = computed(() => (
  writing.value || reading.value || showDiscard.value || showDelete.value || held.value || loadFailed.value
))
const canSave = computed(() => dirty.value && orphanCount(draft.value) === 0 && !layoutLocked.value)

type PendingAction =
  | { kind: 'create' }
  | { kind: 'rename'; group: DisplayDraftGroup }
  | { kind: 'delete'; group: DisplayDraftGroup }
  | { kind: 'external' }

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

function isCanceled(err: unknown) {
  const e = err as { name?: string; code?: string }
  return e?.name === 'AbortError' || e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED'
}

async function load() {
  loadController?.abort()
  const ctrl = new AbortController()
  loadController = ctrl
  reading.value = true
  loadFailed.value = false
  try {
    const view = await adminAPI.channelMonitor.listDisplayGroups({ signal: ctrl.signal })
    if (ctrl.signal.aborted || loadController !== ctrl) return
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
    if (ctrl.signal.aborted || loadController !== ctrl || isCanceled(err)) return
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displayGroups.loadFailed')))
  } finally {
    if (loadController === ctrl) {
      reading.value = false
      loadController = null
    }
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
  writing.value = true
  try {
    await work()
    appStore.showSuccess(t(successKey))
    emit('changed')
    await load()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.displayGroups.saveFailed')))
  } finally {
    writing.value = false
  }
}

function releaseExternal(allowed: boolean) {
  const decide = externalDecision
  externalDecision = null
  decide?.(allowed)
}

function confirmExternalChange(): Promise<boolean> {
  if (!dirty.value) return Promise.resolve(true)
  if (writing.value || reading.value || showDiscard.value || showDelete.value || held.value) {
    return Promise.resolve(false)
  }
  return new Promise((resolve) => {
    externalDecision = resolve
    pendingAction.value = { kind: 'external' }
    showDiscard.value = true
  })
}

function hold(value: boolean) {
  held.value = value
}

defineExpose({ confirmExternalChange, hold })

function askProceed(action: PendingAction) {
  if (layoutLocked.value) return
  if (dirty.value) {
    pendingAction.value = action
    showDiscard.value = true
    return
  }
  if (action.kind === 'delete') {
    openDelete(action.group)
    return
  }
  if (action.kind === 'external') return
  void proceed(action)
}

function cancelDiscard() {
  showDiscard.value = false
  pendingAction.value = null
  releaseExternal(false)
}

function confirmDiscard() {
  const action = pendingAction.value
  showDiscard.value = false
  pendingAction.value = null
  if (!action) return
  if (action.kind === 'external') {
    releaseExternal(true)
    return
  }
  if (action.kind === 'delete') {
    openDelete(action.group)
    return
  }
  void proceed(action)
}

function openDelete(group: DisplayDraftGroup) {
  deleteTarget.value = group
  showDelete.value = true
}

async function proceed(action: { kind: 'create' } | { kind: 'rename'; group: DisplayDraftGroup }) {
  if (action.kind === 'create') {
    const name = newName.value.trim()
    if (!name) return
    await runMutation(async () => {
      await adminAPI.channelMonitor.createDisplayGroup(name)
      newName.value = ''
    }, 'admin.channelMonitor.displayGroups.created')
    return
  }
  const name = (renameName.value[action.group.id] ?? '').trim()
  if (!name || name === action.group.name) return
  await runMutation(
    () => adminAPI.channelMonitor.renameDisplayGroup(action.group.id, name),
    'admin.channelMonitor.displayGroups.renamed',
  )
}

async function createGroup() {
  if (!newName.value.trim()) return
  askProceed({ kind: 'create' })
}

async function rename(group: DisplayDraftGroup) {
  const name = (renameName.value[group.id] ?? '').trim()
  if (!name || name === group.name) return
  askProceed({ kind: 'rename', group })
}

function askDelete(group: DisplayDraftGroup) {
  askProceed({ kind: 'delete', group })
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
onUnmounted(() => {
  loadController?.abort()
  releaseExternal(false)
})
</script>
