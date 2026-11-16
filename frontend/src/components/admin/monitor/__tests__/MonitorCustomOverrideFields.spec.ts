import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { ChannelMonitorCustomOverride } from '@/api/admin/channelMonitor'
import MonitorCustomOverrideFields from '../MonitorCustomOverrideFields.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function mountFields(initial: ChannelMonitorCustomOverride = {}) {
  const wrapper = mount(MonitorCustomOverrideFields, {
    props: {
      modelValue: initial,
      'onUpdate:modelValue': (v: ChannelMonitorCustomOverride) => wrapper.setProps({ modelValue: v }),
    },
  })
  return wrapper
}

const value = (wrapper: ReturnType<typeof mountFields>) => wrapper.props('modelValue') as ChannelMonitorCustomOverride

describe('MonitorCustomOverrideFields', () => {
  it('switches TTFT processing between inherit, on and off', async () => {
    const wrapper = mountFields()
    const group = () => wrapper.find('[data-section="ttft"]')

    expect(group().find('[data-choice="inherit"]').attributes('aria-checked')).toBe('true')

    await group().find('[data-choice="on"]').trigger('click')
    expect(value(wrapper).ttft_adjust_enabled).toBe(true)

    await group().find('[data-choice="off"]').trigger('click')
    expect(value(wrapper).ttft_adjust_enabled).toBe(false)

    await group().find('[data-choice="inherit"]').trigger('click')
    expect(value(wrapper).ttft_adjust_enabled).toBeNull()
  })

  it('stores numbers and maps an empty field back to inherit', async () => {
    const wrapper = mountFields({ tps_max: 80 })
    const input = wrapper.findAll('[data-section="tps"] input')[1]

    expect((input.element as HTMLInputElement).value).toBe('80')
    await input.setValue('120.5')
    expect(value(wrapper).tps_max).toBe(120.5)

    await input.setValue('')
    expect(value(wrapper).tps_max).toBeNull()
  })

  it('offers a reset only when something is overridden', async () => {
    const wrapper = mountFields()
    expect(wrapper.find('[data-test="reset-override"]').exists()).toBe(false)

    await wrapper.setProps({ modelValue: { ttft_multiplier: 1.2, tps_enabled: false } })
    await wrapper.find('[data-test="reset-override"]').trigger('click')
    expect(value(wrapper)).toEqual({})
  })
})
