import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelMonitorDisplaySettings from '../ChannelMonitorDisplaySettings.vue'

const mocks = vi.hoisted(() => ({
  getDisplaySettings: vi.fn(),
  saveDisplaySettings: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/channelMonitor', () => ({
  getDisplaySettings: mocks.getDisplaySettings,
  saveDisplaySettings: mocks.saveDisplaySettings,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: mocks.fetchPublicSettings,
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountPanel() {
  return mount(ChannelMonitorDisplaySettings)
}

describe('ChannelMonitorDisplaySettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.getDisplaySettings.mockResolvedValue({ default_interval_seconds: 300, timeline_points: 20 })
    mocks.saveDisplaySettings.mockImplementation(async (payload) => payload)
    mocks.fetchPublicSettings.mockResolvedValue(undefined)
  })

  it('shows the saved interval and record count', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect((wrapper.get('#monitor-default-interval').element as HTMLInputElement).value).toBe('300')
    expect((wrapper.get('#monitor-timeline-points').element as HTMLInputElement).value).toBe('20')
    expect(wrapper.get('[data-test="save-display-settings"]').attributes('disabled')).toBeDefined()
  })

  it('saves a new interval and record count, then refreshes public settings', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('#monitor-default-interval').setValue(120)
    await wrapper.get('#monitor-timeline-points').setValue(40)
    await wrapper.get('[data-test="save-display-settings"]').trigger('click')
    await flushPromises()
    expect(mocks.saveDisplaySettings).toHaveBeenCalledWith({ default_interval_seconds: 120, timeline_points: 40 })
    expect(mocks.fetchPublicSettings).toHaveBeenCalledWith(true)
    expect(mocks.showSuccess).toHaveBeenCalled()
  })

  it('does not save a value outside the allowed range', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('#monitor-timeline-points').setValue(0)
    expect(wrapper.get('[data-test="save-display-settings"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="save-display-settings"]').trigger('click')
    expect(mocks.saveDisplaySettings).not.toHaveBeenCalled()
  })

  it('keeps save locked when the first read fails', async () => {
    mocks.getDisplaySettings.mockRejectedValue(new Error('down'))
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.channelMonitor.displaySettings.loadFailed')
    await wrapper.get('#monitor-default-interval').setValue(300)
    expect(wrapper.get('[data-test="save-display-settings"]').attributes('disabled')).toBeDefined()
    expect(mocks.saveDisplaySettings).not.toHaveBeenCalled()
  })
})
