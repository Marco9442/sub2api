import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { isAccountSimPage, toAccountSimPath } from '@/api/accountSim'

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

describe('账号管理 #2 请求切换', () => {
  it('只在账号管理 #2 页面内切换', () => {
    expect(isAccountSimPage('/admin/accounts-2')).toBe(true)
    expect(isAccountSimPage('/admin/accounts-2/')).toBe(true)
    expect(isAccountSimPage('/admin/accounts')).toBe(false)
    expect(isAccountSimPage('/admin/accounts-20')).toBe(false)
  })

  it('把账号页用到的接口前缀改写到模拟接口，保留查询参数', () => {
    const page = '/admin/accounts-2'
    expect(toAccountSimPath('/admin/accounts', page)).toBe('/admin/account-sim/accounts')
    expect(toAccountSimPath('/admin/accounts/12/usage?source=passive', page)).toBe(
      '/admin/account-sim/accounts/12/usage?source=passive'
    )
    expect(toAccountSimPath('/admin/groups/all', page)).toBe('/admin/account-sim/groups/all')
    expect(toAccountSimPath('/admin/proxies/all', page)).toBe('/admin/account-sim/proxies/all')
    expect(toAccountSimPath('/admin/grok/accounts/3/quota', page)).toBe('/admin/account-sim/grok/accounts/3/quota')
  })

  it('不改写其他页面、其他接口和前缀相似的路径', () => {
    expect(toAccountSimPath('/admin/accounts', '/admin/accounts')).toBe('/admin/accounts')
    const page = '/admin/accounts-2'
    expect(toAccountSimPath('/admin/settings', page)).toBe('/admin/settings')
    expect(toAccountSimPath('/admin/payment/config', page)).toBe('/admin/payment/config')
    expect(toAccountSimPath('/admin/accountsx', page)).toBe('/admin/accountsx')
    expect(toAccountSimPath('/auth/me', page)).toBe('/auth/me')
    expect(toAccountSimPath('/admin/account-sim/config', page)).toBe('/admin/account-sim/config')
  })
})

describe('统一请求入口接入切换', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetModules()
  })

  afterEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('apiClient 在账号管理 #2 页面请求模拟接口', async () => {
    window.history.replaceState({}, '', '/admin/accounts-2')
    const { apiClient } = await import('@/api/client')
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: {} },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    apiClient.defaults.adapter = adapter

    await apiClient.get('/admin/accounts', { params: { page: 1 } })
    await apiClient.get('/admin/settings')

    expect(adapter.mock.calls[0][0].url).toBe('/admin/account-sim/accounts')
    expect(adapter.mock.calls[1][0].url).toBe('/admin/settings')
  })

  it('apiClient 在真实账号管理页面不切换', async () => {
    window.history.replaceState({}, '', '/admin/accounts')
    const { apiClient } = await import('@/api/client')
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: {} },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    apiClient.defaults.adapter = adapter

    await apiClient.get('/admin/accounts')

    expect(adapter.mock.calls[0][0].url).toBe('/admin/accounts')
  })

  it('buildApiUrl（测试连接的流式请求）同样切换', async () => {
    window.history.replaceState({}, '', '/admin/accounts-2')
    const { buildApiUrl } = await import('@/api/client')
    expect(buildApiUrl('/admin/accounts/5/test')).toBe('/api/v1/admin/account-sim/accounts/5/test')
    expect(buildApiUrl('/api/v1/admin/accounts/5/test')).toBe('/api/v1/admin/account-sim/accounts/5/test')
    expect(buildApiUrl('/payment/orders/1')).toBe('/api/v1/payment/orders/1')
  })
})
