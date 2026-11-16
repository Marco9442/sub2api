import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { bindAccountSimCalls, isAccountSimPage, toAccountSimPath, withAccountSimRequest } from '@/api/accountSim'

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
    expect(toAccountSimPath('/admin/dashboard/user-breakdown?start_date=2026-11-01', page)).toBe(
      '/admin/account-sim/dashboard/user-breakdown?start_date=2026-11-01'
    )
  })

  it('不改写其他页面、其他接口和前缀相似的路径', () => {
    expect(toAccountSimPath('/admin/accounts', '/admin/accounts')).toBe('/admin/accounts')
    const page = '/admin/accounts-2'
    expect(toAccountSimPath('/admin/settings', page)).toBe('/admin/settings')
    expect(toAccountSimPath('/admin/payment/config', page)).toBe('/admin/payment/config')
    expect(toAccountSimPath('/admin/dashboard/snapshot-v2', page)).toBe('/admin/dashboard/snapshot-v2')
    expect(toAccountSimPath('/admin/accountsx', page)).toBe('/admin/accountsx')
    expect(toAccountSimPath('/auth/me', page)).toBe('/auth/me')
    expect(toAccountSimPath('/admin/account-sim/config', page)).toBe('/admin/account-sim/config')
    expect(toAccountSimPath('/admin/accounts/antigravity/default-model-mapping', page)).toBe(
      '/admin/accounts/antigravity/default-model-mapping'
    )
    expect(toAccountSimPath('/admin/accounts/antigravity/default-model-mapping?x=1', page)).toBe(
      '/admin/accounts/antigravity/default-model-mapping?x=1'
    )
  })

  it('单次标记只覆盖这次同步调用，离开后其他请求不切换', () => {
    window.history.replaceState({}, '', '/admin/accounts')
    withAccountSimRequest(true, () => {
      expect(toAccountSimPath('/admin/accounts/data')).toBe('/admin/account-sim/accounts/data')
      expect(toAccountSimPath('/admin/accounts/antigravity/default-model-mapping')).toBe(
        '/admin/accounts/antigravity/default-model-mapping'
      )
    })
    expect(toAccountSimPath('/admin/accounts/data')).toBe('/admin/accounts/data')
    window.history.replaceState({}, '', '/admin/accounts-2')
    withAccountSimRequest(false, () => {
      expect(toAccountSimPath('/admin/accounts/data')).toBe('/admin/accounts/data')
    })
  })

  it('绑定调用在存活时固定来源，卸载后不再打出', async () => {
    window.history.replaceState({}, '', '/admin/accounts')
    const calls: string[] = []
    const api = {
      id: 1,
      save: () => {
        calls.push(toAccountSimPath('/admin/accounts/1'))
        return Promise.resolve('ok')
      },
    }
    let alive = true
    const bound = bindAccountSimCalls(api, true, () => alive)
    expect(bound.id).toBe(1)
    await bound.save()
    alive = false
    const pending = bound.save()
    await expect(Promise.race([pending, Promise.resolve('hung')])).resolves.toBe('hung')
    expect(calls).toEqual(['/admin/account-sim/accounts/1'])
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

  it('单次标记的请求在离开页面后仍走模拟接口，其他请求不切换', async () => {
    window.history.replaceState({}, '', '/admin/accounts')
    const { apiClient } = await import('@/api/client')
    const { withAccountSimRequest: mark } = await import('@/api/accountSim')
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: {} },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    apiClient.defaults.adapter = adapter

    await mark(true, () => apiClient.post('/admin/accounts/data'))
    await apiClient.post('/admin/accounts/data')
    await mark(true, () => apiClient.get('/admin/accounts/antigravity/default-model-mapping'))
    expect(adapter.mock.calls[0][0].url).toBe('/admin/account-sim/accounts/data')
    expect(adapter.mock.calls[1][0].url).toBe('/admin/accounts/data')
    expect(adapter.mock.calls[2][0].url).toBe('/admin/accounts/antigravity/default-model-mapping')
  })

  it('buildApiUrl（测试连接的流式请求）同样切换', async () => {
    window.history.replaceState({}, '', '/admin/accounts-2')
    const { buildApiUrl } = await import('@/api/client')
    expect(buildApiUrl('/admin/accounts/5/test')).toBe('/api/v1/admin/account-sim/accounts/5/test')
    expect(buildApiUrl('/api/v1/admin/accounts/5/test')).toBe('/api/v1/admin/account-sim/accounts/5/test')
    expect(buildApiUrl('/payment/orders/1')).toBe('/api/v1/payment/orders/1')
  })
})
