import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelMonitorView from '@/views/admin/ChannelMonitorView.vue'

const { listMonitors } = vi.hoisted(() => ({ listMonitors: vi.fn() }))

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV1Mode: () => false,
  isChannelMonitorV2Mode: () => true,
  getChannelMonitorMode: () => 'v2' as const,
}))

vi.mock('@/features/channel-monitor-v2/MonitorSettingsPanel.vue', () => ({
  default: { name: 'MonitorSettingsPanel', template: '<div />' },
}))

vi.mock('@/components/admin/monitor/ChannelMonitorModePanel.vue', () => ({
  default: defineComponent({
    name: 'ChannelMonitorModePanel',
    emits: ['update:saved'],
    template: '<button class="save-mode" @click="$emit(\'update:saved\', \'custom\')">save</button>',
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { channelMonitor: { list: listMonitors } },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

beforeEach(() => {
  listMonitors.mockReset()
  listMonitors.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
})

describe('ChannelMonitorView mode reload', () => {
  it('reloads monitor stats after the saved mode changes', async () => {
    const wrapper = mount(ChannelMonitorView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          TablePageLayout: true,
          DataTable: true,
          MonitorFiltersBar: true,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          HelpTooltip: true,
          Toggle: true,
          Icon: true,
          MonitorFormDialog: true,
          MonitorTemplateManagerDialog: true,
          MonitorRunResultDialog: true,
          MonitorPrimaryModelCell: true,
          MonitorActionsCell: true,
        },
      },
    })
    await flushPromises()
    expect(listMonitors).not.toHaveBeenCalled()

    await wrapper.get('button.save-mode').trigger('click')
    await flushPromises()
    expect(listMonitors).toHaveBeenCalledTimes(1)
  })
})
