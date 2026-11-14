import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelMonitorModePanel from '../ChannelMonitorModePanel.vue'

const mocks = vi.hoisted(() => ({
  getMode: vi.fn(),
  updateMode: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/channelMonitor', () => ({
  channelMonitorAPI: {
    getMode: mocks.getMode,
    updateMode: mocks.updateMode,
  },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: mocks.fetchPublicSettings,
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
  }),
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const savedView = {
  mode: 'v2',
  custom: {
    prompt: 'ping',
    yellow_seconds: 2,
    red_seconds: 8,
    ttft_adjust_enabled: false,
    ttft_multiplier: 1,
    ttft_min_ms: 0,
    ttft_max_ms: 0,
    tps_enabled: false,
    tps_min: 0,
    tps_max: 0,
  },
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getMode.mockResolvedValue({ ...savedView, mode: 'custom' })
  mocks.updateMode.mockResolvedValue(savedView)
  mocks.fetchPublicSettings.mockResolvedValue(null)
})

describe('ChannelMonitorModePanel', () => {
  it('forces a public settings refresh after saving the mode', async () => {
    const wrapper = mount(ChannelMonitorModePanel, {
      props: { preview: 'custom', saved: 'custom' },
    })
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mocks.updateMode).toHaveBeenCalled()
    expect(mocks.fetchPublicSettings).toHaveBeenCalledWith(true)
  })
})
