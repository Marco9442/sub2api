import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorFormDialog from '@/components/admin/monitor/MonitorFormDialog.vue'

const { monitorUpdate } = vi.hoisted(() => ({ monitorUpdate: vi.fn() }))

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV1Mode: () => true,
  isChannelMonitorV2Mode: () => false,
  getChannelMonitorMode: () => 'v1' as const,
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: { create: vi.fn(), update: monitorUpdate },
    channelMonitorTemplate: { list: vi.fn().mockResolvedValue({ items: [] }) },
    accounts: { list: vi.fn(), getById: vi.fn() },
  },
}))

vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn() } }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getUserGroupRates: vi.fn() } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null, showError: vi.fn(), showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const monitor = {
  id: 42,
  name: 'primary',
  provider: 'openai',
  api_mode: 'chat_completions',
  endpoint: 'https://api.example.com',
  api_key_masked: 'sk-t***',
  primary_model: 'gpt-4o-mini',
  extra_models: [],
  group_name: '',
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
} as ChannelMonitor

describe('MonitorFormDialog beforeWrite', () => {
  beforeEach(() => {
    monitorUpdate.mockReset()
  })

  it('does not save an existing monitor when the unsaved arrangement is kept', async () => {
    const beforeWrite = vi.fn(async () => false)
    const wrapper = mount(MonitorFormDialog, {
      props: { show: true, monitor, beforeWrite },
      global: {
        stubs: {
          BaseDialog: defineComponent({
            props: { show: { type: Boolean, default: false } },
            template: '<div v-if="show"><slot /><slot name="footer" /></div>',
          }),
          Toggle: true,
          Select: true,
          ModelTagInput: true,
          MonitorKeyPickerDialog: true,
          MonitorAdvancedRequestConfig: true,
        },
      },
    })
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(beforeWrite).toHaveBeenCalledTimes(1)
    expect(monitorUpdate).not.toHaveBeenCalled()
    expect(wrapper.emitted('saved')).toBeUndefined()
    wrapper.unmount()
  })
})
