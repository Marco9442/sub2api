import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelMonitorSettingsMode from '../ChannelMonitorSettingsMode.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function mountMode(mode: 'v1' | 'v2' | 'custom') {
  return mount(ChannelMonitorSettingsMode, { props: { mode } })
}

describe('ChannelMonitorSettingsMode', () => {
  it('shows Custom as the current mode without a control that selects it', () => {
    const wrapper = mountMode('custom')

    const current = wrapper.get('[data-mode="custom"]')
    expect(current.element.tagName).not.toBe('BUTTON')
    expect(current.attributes('aria-current')).toBe('true')
    expect(current.text()).toContain('channelMonitorCustom.modeCustom')
    expect(wrapper.findAll('button')).toHaveLength(2)
    expect(wrapper.find('[data-mode="custom"]').attributes('data-selected')).toBe('true')
    expect(wrapper.get('[data-mode="v1"]').attributes('data-selected')).toBe('false')
    expect(wrapper.get('[data-mode="v2"]').attributes('data-selected')).toBe('false')
  })

  it('hides the Custom state outside Custom and still switches to V1 or V2', async () => {
    const wrapper = mountMode('v1')

    expect(wrapper.find('[data-mode="custom"]').exists()).toBe(false)
    expect(wrapper.get('[data-mode="v1"]').attributes('data-selected')).toBe('true')

    await wrapper.get('[data-mode="v2"]').trigger('click')
    expect(wrapper.emitted('update:mode')).toEqual([['v2']])

    await wrapper.setProps({ mode: 'v2' })
    await wrapper.get('[data-mode="v1"]').trigger('click')
    expect(wrapper.emitted('update:mode')).toEqual([['v2'], ['v1']])
  })
})
