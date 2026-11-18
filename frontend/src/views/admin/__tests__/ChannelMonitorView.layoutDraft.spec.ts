import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import ChannelMonitorView from '@/views/admin/ChannelMonitorView.vue'

const {
  listMonitors,
  updateMonitor,
  duplicateMonitor,
  confirmExternalChange,
  hold,
} = vi.hoisted(() => ({
  listMonitors: vi.fn(),
  updateMonitor: vi.fn(),
  duplicateMonitor: vi.fn(),
  confirmExternalChange: vi.fn(),
  hold: vi.fn(),
}))

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV1Mode: () => true,
  isChannelMonitorV2Mode: () => false,
  getChannelMonitorMode: () => 'v1' as const,
}))

vi.mock('@/features/channel-monitor-v2/MonitorSettingsPanel.vue', () => ({
  default: { name: 'MonitorSettingsPanel', template: '<div />' },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: {
      list: listMonitors,
      update: updateMonitor,
      duplicate: duplicateMonitor,
    },
  },
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

const monitor: ChannelMonitor = {
  id: 7,
  name: 'primary',
  provider: 'openai',
  api_mode: 'chat_completions',
  endpoint: 'https://api.example.com',
  api_key_masked: 'sk-t***',
  primary_model: 'gpt-4o-mini',
  extra_models: [],
  group_name: 'OpenAI',
  enabled: true,
  interval_seconds: 60,
  jitter_seconds: 0,
  last_checked_at: null,
  created_by: 1,
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  primary_status: '',
  primary_latency_ms: null,
  availability_7d: 0,
  extra_models_status: [],
  template_id: null,
  extra_headers: {},
  body_override_mode: 'off',
  body_override: null,
  check_mode: 'probe',
  account_id: null,
}

const DisplayGroupPanelStub = defineComponent({
  name: 'DisplayGroupPanel',
  emits: ['changed', 'loaded'],
  setup(_, { expose, emit }) {
    expose({ confirmExternalChange, hold })
    return { emitChanged: () => emit('changed') }
  },
  template: '<button class="panel-changed" @click="emitChanged">changed</button>',
})

function mountView() {
  return mount(ChannelMonitorView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>' },
        DataTable: {
          props: { data: { type: Array, default: () => [] } },
          template: '<div><div v-for="row in data" :key="row.id">{{ row.name }}<slot name="cell-enabled" :row="row" /></div></div>',
        },
        Toggle: {
          props: { modelValue: Boolean },
          emits: ['update:modelValue'],
          template: '<button class="toggle" @click="$emit(\'update:modelValue\', !modelValue)">toggle</button>',
        },
        Pagination: {
          emits: ['update:page'],
          template: '<button class="goto-page-2" @click="$emit(\'update:page\', 2)">2</button>',
        },
        MonitorFiltersBar: true,
        ConfirmDialog: true,
        EmptyState: true,
        HelpTooltip: true,
        Icon: true,
        MonitorFormDialog: true,
        DisplayGroupPanel: DisplayGroupPanelStub,
        MonitorTemplateManagerDialog: true,
        MonitorRunResultDialog: true,
        MonitorPrimaryModelCell: true,
        MonitorActionsCell: true,
      },
    },
  })
}

describe('ChannelMonitorView layout draft', () => {
  beforeEach(() => {
    localStorage.clear()
    listMonitors.mockReset()
    updateMonitor.mockReset()
    duplicateMonitor.mockReset()
    confirmExternalChange.mockReset()
    hold.mockReset()
    confirmExternalChange.mockResolvedValue(true)
    listMonitors.mockResolvedValue({ items: [{ ...monitor }], total: 1, page: 1, page_size: 20, pages: 1 })
    updateMonitor.mockResolvedValue({ ...monitor })
  })

  it('does not toggle a monitor when the unsaved arrangement is kept', async () => {
    confirmExternalChange.mockResolvedValue(false)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button.toggle').trigger('click')
    await flushPromises()

    expect(confirmExternalChange).toHaveBeenCalledTimes(1)
    expect(updateMonitor).not.toHaveBeenCalled()
    expect(hold).not.toHaveBeenCalled()
  })

  it('toggles only after the arrangement is discarded', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button.toggle').trigger('click')
    await flushPromises()

    expect(updateMonitor).toHaveBeenCalledWith(7, { enabled: false })
    expect(hold).toHaveBeenCalledWith(true)
    expect(hold).toHaveBeenLastCalledWith(false)
  })

  it('rereads the last valid page when a refresh lands past the end', async () => {
    const wrapper = mountView()
    await flushPromises()
    const pageSize = listMonitors.mock.calls[0][0].page_size as number
    listMonitors.mockResolvedValueOnce({ items: [monitor], total: pageSize + 1, page: 2, page_size: pageSize, pages: 2 })
    await wrapper.get('button.goto-page-2').trigger('click')
    await flushPromises()

    listMonitors
      .mockResolvedValueOnce({ items: [], total: 1, page: 2, page_size: pageSize, pages: 1 })
      .mockResolvedValueOnce({ items: [monitor], total: 1, page: 1, page_size: pageSize, pages: 1 })
    await wrapper.get('button.panel-changed').trigger('click')
    await flushPromises()

    const pages = listMonitors.mock.calls.map((call) => call[0].page)
    expect(pages.at(-2)).toBe(2)
    expect(pages.at(-1)).toBe(1)
    expect(wrapper.text()).toContain('primary')
  })
})
