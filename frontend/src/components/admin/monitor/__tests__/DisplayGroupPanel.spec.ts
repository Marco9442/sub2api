import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import DisplayGroupPanel from '../DisplayGroupPanel.vue'

const api = vi.hoisted(() => ({
  listDisplayGroups: vi.fn(),
  createDisplayGroup: vi.fn(),
  renameDisplayGroup: vi.fn(),
  deleteDisplayGroup: vi.fn(),
  saveDisplayLayout: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { channelMonitor: api },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

enableAutoUnmount(afterEach)

const view = {
  groups: [{ id: 1, name: 'OpenAI', sort_order: 0, monitor_ids: [10, 11] }],
  ungrouped_ids: [] as number[],
  monitors: [
    { id: 10, name: 'alpha', provider: 'openai', enabled: true, group_name: 'OpenAI', display_sort: 0 },
    { id: 11, name: 'beta', provider: 'openai', enabled: true, group_name: 'OpenAI', display_sort: 1 },
  ],
}

function mountPanel() {
  return mount(DisplayGroupPanel, {
    global: {
      stubs: {
        Icon: true,
        ConfirmDialog: {
          props: ['show', 'title', 'message', 'confirmText', 'zIndex'],
          template: `
            <div v-if="show" class="confirm" :data-title="title" :data-z="zIndex">
              <p>{{ message }}</p>
              <button class="confirm-yes" @click="$emit('confirm')">{{ confirmText }}</button>
              <button class="confirm-no" @click="$emit('cancel')">cancel</button>
            </div>
          `,
        },
      },
    },
  })
}

function discardDialog(wrapper: ReturnType<typeof mountPanel>) {
  return wrapper.findAll('.confirm').find((node) => node.attributes('data-title') === 'admin.channelMonitor.displayGroups.discardTitle')
}

function deleteDialog(wrapper: ReturnType<typeof mountPanel>) {
  return wrapper.findAll('.confirm').find((node) => node.attributes('data-title') === 'common.delete')
}

async function makeDirty(wrapper: ReturnType<typeof mountPanel>) {
  const moveDown = wrapper.findAll('button').find((button) => (
    button.attributes('aria-label') === 'admin.channelMonitor.displayGroups.moveDown' && button.attributes('disabled') === undefined
  ))
  expect(moveDown).toBeTruthy()
  await moveDown!.trigger('click')
}

async function submitCreate(wrapper: ReturnType<typeof mountPanel>, name = '备用') {
  await wrapper.get('form input').setValue(name)
  await wrapper.get('form').trigger('submit')
}

describe('DisplayGroupPanel draft protection', () => {
  beforeEach(() => {
    api.listDisplayGroups.mockReset()
    api.listDisplayGroups.mockResolvedValue(view)
    api.createDisplayGroup.mockReset()
    api.createDisplayGroup.mockResolvedValue({ id: 2, name: '备用', sort_order: 1 })
    api.renameDisplayGroup.mockReset()
    api.renameDisplayGroup.mockResolvedValue(undefined)
    api.deleteDisplayGroup.mockReset()
    api.deleteDisplayGroup.mockResolvedValue(undefined)
    api.saveDisplayLayout.mockReset()
    api.saveDisplayLayout.mockResolvedValue(undefined)
  })

  it('asks before a group change discards a dirty arrangement', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)
    await submitCreate(wrapper)

    expect(api.createDisplayGroup).not.toHaveBeenCalled()
    expect(discardDialog(wrapper)?.text()).toContain('admin.channelMonitor.displayGroups.discardMessage')
    expect(wrapper.text()).toContain('beta')
  })

  it('keeps the draft when the discard prompt is cancelled', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)
    await submitCreate(wrapper)
    await discardDialog(wrapper)!.get('.confirm-no').trigger('click')

    expect(api.createDisplayGroup).not.toHaveBeenCalled()
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeUndefined()
    expect(wrapper.text().indexOf('beta')).toBeLessThan(wrapper.text().indexOf('alpha'))
  })

  it('continues the group change only after the draft is explicitly discarded', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)
    await submitCreate(wrapper)
    await discardDialog(wrapper)!.get('.confirm-yes').trigger('click')
    await flushPromises()

    expect(api.createDisplayGroup).toHaveBeenCalledWith('备用')
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect(api.listDisplayGroups).toHaveBeenCalledTimes(2)
  })

  it('does not ask when the arrangement is already saved', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await submitCreate(wrapper)
    await flushPromises()

    expect(discardDialog(wrapper)).toBeUndefined()
    expect(api.createDisplayGroup).toHaveBeenCalledTimes(1)
  })

  it('confirms discard before the existing delete confirmation', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)
    await wrapper.get('button.text-red-600').trigger('click')

    expect(deleteDialog(wrapper)).toBeUndefined()
    expect(api.deleteDisplayGroup).not.toHaveBeenCalled()
    await discardDialog(wrapper)!.get('.confirm-yes').trigger('click')
    expect(api.deleteDisplayGroup).not.toHaveBeenCalled()
    await deleteDialog(wrapper)!.get('.confirm-yes').trigger('click')
    await flushPromises()

    expect(api.deleteDisplayGroup).toHaveBeenCalledWith(1)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('saves a dirty arrangement without asking to discard it', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(discardDialog(wrapper)).toBeUndefined()
    expect(api.saveDisplayLayout).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('does not refresh the monitor table on the initial read', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()

    await wrapper.setProps({ reloadToken: 1 })
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(api.listDisplayGroups).toHaveBeenCalledTimes(2)
  })

  it('keeps member controls locked until the follow-up read finishes', async () => {
    let releaseCreate: (value: { id: number; name: string; sort_order: number }) => void = () => {}
    api.createDisplayGroup.mockImplementation(() => new Promise((resolve) => { releaseCreate = resolve }))
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('form input').setValue('备用')
    const pending = wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('select').attributes('disabled')).toBeDefined()

    let releaseRead: (value: typeof view) => void = () => {}
    api.listDisplayGroups.mockImplementationOnce(() => new Promise((resolve) => { releaseRead = resolve }))
    releaseCreate({ id: 2, name: '备用', sort_order: 1 })
    await flushPromises()
    expect(wrapper.get('select').attributes('disabled')).toBeDefined()

    releaseRead(view)
    await pending
    await flushPromises()
    const enabledMove = wrapper.findAll('button').find((button) => (
      button.attributes('aria-label') === 'admin.channelMonitor.displayGroups.moveDown' && button.attributes('disabled') === undefined
    ))
    expect(enabledMove).toBeTruthy()
    expect(wrapper.get('select').attributes('disabled')).toBeUndefined()
  })

  it('does not refresh the monitor table when the write fails', async () => {
    api.createDisplayGroup.mockRejectedValue(new Error('no'))
    const wrapper = mountPanel()
    await flushPromises()
    await submitCreate(wrapper)
    await flushPromises()

    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(api.listDisplayGroups).toHaveBeenCalledTimes(1)
  })

  it('lets a monitor change continue without asking when nothing is unsaved', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await expect(panelApi(wrapper).confirmExternalChange()).resolves.toBe(true)
    expect(discardDialog(wrapper)).toBeUndefined()
  })

  it('asks before a monitor change discards a dirty arrangement and keeps it on cancel', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)

    const pending = panelApi(wrapper).confirmExternalChange()
    await flushPromises()
    expect(discardDialog(wrapper)?.text()).toContain('admin.channelMonitor.displayGroups.discardMessage')
    expect(discardDialog(wrapper)?.attributes('data-z')).toBe('60')
    await discardDialog(wrapper)!.get('.confirm-no').trigger('click')

    await expect(pending).resolves.toBe(false)
    expect(api.createDisplayGroup).not.toHaveBeenCalled()
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeUndefined()
    expect(wrapper.text().indexOf('beta')).toBeLessThan(wrapper.text().indexOf('alpha'))
  })

  it('allows the monitor change after the draft is explicitly discarded', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await makeDirty(wrapper)

    const pending = panelApi(wrapper).confirmExternalChange()
    await flushPromises()
    await discardDialog(wrapper)!.get('.confirm-yes').trigger('click')

    await expect(pending).resolves.toBe(true)
    expect(api.createDisplayGroup).not.toHaveBeenCalled()
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeUndefined()
  })

  it('keeps the old arrangement locked when the follow-up read fails', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    api.listDisplayGroups.mockRejectedValueOnce(new Error('read failed'))
    await submitCreate(wrapper)
    await flushPromises()

    expect(wrapper.get('select').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.channelMonitor.displayGroups.loadRetry')

    api.listDisplayGroups.mockResolvedValueOnce({
      ...view,
      groups: [{ id: 1, name: 'OpenAI', sort_order: 0, monitor_ids: [11, 10] }],
    })
    await wrapper.get('.display-group-retry').trigger('click')
    await flushPromises()

    expect(wrapper.text().indexOf('beta')).toBeLessThan(wrapper.text().indexOf('alpha'))
    expect(wrapper.get('select').attributes('disabled')).toBeUndefined()
  })

  it('ignores an older read that arrives after a newer one', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    let releaseOld: (value: typeof view) => void = () => {}
    api.listDisplayGroups.mockImplementationOnce(() => new Promise((resolve) => { releaseOld = resolve }))
    await wrapper.setProps({ reloadToken: 1 })
    await flushPromises()

    api.listDisplayGroups.mockResolvedValueOnce({
      ...view,
      groups: [{ id: 1, name: 'OpenAI', sort_order: 0, monitor_ids: [11, 10] }],
    })
    await wrapper.setProps({ reloadToken: 2 })
    await flushPromises()
    expect(wrapper.text().indexOf('beta')).toBeLessThan(wrapper.text().indexOf('alpha'))

    releaseOld(view)
    await flushPromises()
    expect(wrapper.text().indexOf('beta')).toBeLessThan(wrapper.text().indexOf('alpha'))
  })
})

function panelApi(wrapper: ReturnType<typeof mountPanel>) {
  return wrapper.vm as unknown as { confirmExternalChange: () => Promise<boolean> }
}
