import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountsSimView from '../AccountsSimView.vue'

const getConfig = vi.fn()
const putConfig = vi.fn()

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: (...args: unknown[]) => getConfig(...args),
    put: (...args: unknown[]) => putConfig(...args)
  }
}))

vi.mock('../AccountsView.vue', async () => {
  const { defineComponent, inject, onMounted } = await import('vue')
  const { openAccountSimConfigKey } = await import('@/composables/accountSimPage')
  return {
    default: defineComponent({
      name: 'AccountsView',
      setup() {
        const open = inject(openAccountSimConfigKey, null)
        onMounted(() => open?.())
      },
      template: '<div data-test="accounts-view" />'
    })
  }
})

const config = {
  enabled: true,
  daily_new_min: 30,
  daily_new_max: 120,
  lifetime_days_min: 3,
  lifetime_days_max: 15,
  linger_days_min: 1,
  linger_days_max: 3,
  usage_level: 1
}

describe('账号管理 #2 配置', () => {
  beforeEach(() => {
    getConfig.mockReset()
    putConfig.mockReset()
  })

  it('加载失败时保存保持禁用，并不提交', async () => {
    getConfig.mockRejectedValue(new Error('down'))
    const wrapper = mount(AccountsSimView)
    await flushPromises()

    const save = wrapper.find('button[type="submit"]')
    expect(save.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('accountSim.saveFailed')

    await wrapper.find('form').trigger('submit')
    expect(putConfig).not.toHaveBeenCalled()
  })

  it('加载成功后才允许保存', async () => {
    getConfig.mockResolvedValue({ data: config })
    const wrapper = mount(AccountsSimView)
    await flushPromises()

    const save = wrapper.find('button[type="submit"]')
    expect(save.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('accountSim.saveFailed')
  })
})
