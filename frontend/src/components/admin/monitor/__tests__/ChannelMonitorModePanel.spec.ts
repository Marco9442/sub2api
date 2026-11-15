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

async function mountPanel() {
  const wrapper = mount(ChannelMonitorModePanel, {
    props: {
      preview: 'custom',
      saved: 'custom',
      'onUpdate:preview': (v: string) => wrapper.setProps({ preview: v }),
      'onUpdate:saved': (v: string) => wrapper.setProps({ saved: v }),
    },
  })
  await flushPromises()
  return wrapper
}

const saveButton = (wrapper: Awaited<ReturnType<typeof mountPanel>>) => wrapper.find('[data-test="save-mode"]')

describe('ChannelMonitorModePanel', () => {
  it('forces a public settings refresh after saving the mode', async () => {
    const wrapper = await mountPanel()
    await wrapper.find('[data-mode="v2"]').trigger('click')
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(mocks.updateMode).toHaveBeenCalledWith('v2', savedView.custom)
    expect(mocks.fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it('marks the saved mode and keeps save disabled until something changes', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.find('[data-mode="custom"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.find('[data-mode="custom"]').text()).toContain('channelMonitorCustom.savedBadge')
    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('channelMonitorCustom.previewDirty')

    await wrapper.find('[data-section="prompt"] input').setValue('hello')
    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('channelMonitorCustom.previewDirty')
  })

  it('groups custom settings and uses switches for optional metrics', async () => {
    const wrapper = await mountPanel()

    for (const section of ['prompt', 'thresholds', 'ttft', 'tps']) {
      expect(wrapper.find(`[data-section="${section}"]`).exists()).toBe(true)
    }
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(2)
    expect(wrapper.find('[data-section="ttft"] input').exists()).toBe(false)

    await wrapper.find('[data-section="ttft"] [role="switch"]').trigger('click')
    expect(wrapper.findAll('[data-section="ttft"] input')).toHaveLength(3)
  })

  it('shows inline errors and blocks saving invalid settings', async () => {
    const wrapper = await mountPanel()

    await wrapper.find('[data-section="thresholds"] input').setValue(9)
    expect(wrapper.find('[data-section="thresholds"]').text()).toContain('channelMonitorCustom.thresholdsInvalid')
    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()

    await wrapper.find('[data-section="thresholds"] input').setValue(2)
    await wrapper.find('[data-section="tps"] [role="switch"]').trigger('click')
    expect(wrapper.find('[data-section="tps"]').text()).toContain('channelMonitorCustom.tpsInvalid')
    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()

    await saveButton(wrapper).trigger('click')
    expect(mocks.updateMode).not.toHaveBeenCalled()
  })

  it('keeps the saved custom settings when saving another mode', async () => {
    const wrapper = await mountPanel()
    await wrapper.find('[data-section="thresholds"] input').setValue(9)
    await wrapper.find('[data-mode="v1"]').trigger('click')

    expect(wrapper.find('[data-section="thresholds"]').exists()).toBe(false)
    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined()

    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.updateMode).toHaveBeenCalledWith('v1', savedView.custom)
  })

  it('saves availability windows and blocks invalid ones', async () => {
    const wrapper = await mountPanel()
    const input = wrapper.find('[data-section="windows"] input')

    await input.setValue('31d')
    expect(wrapper.find('[data-section="windows"]').text()).toContain('channelMonitorCustom.windowsInvalid')
    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()

    await input.setValue('15m,1d')
    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined()

    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.updateMode).toHaveBeenCalledWith('custom', expect.objectContaining({
      availability_windows: [
        { amount: 15, unit: 'm' },
        { amount: 1, unit: 'd' },
      ],
    }))
  })
})
