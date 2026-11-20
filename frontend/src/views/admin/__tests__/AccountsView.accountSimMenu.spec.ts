import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import { openAccountSimConfigKey } from '@/composables/accountSimPage'

const { listAccounts, getUpstreamBillingProbeSettings, getAllProxies, getAllGroups } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      getById: vi.fn(),
      listWithEtag: vi.fn(),
      getBatchTodayStats: vi.fn().mockResolvedValue({ stats: {} }),
      getUpstreamBillingProbeSettings,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      refreshCredentials: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showWarning: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountView(openConfig?: () => void) {
  return mount(AccountsView, {
    attachTo: document.body,
    global: {
      provide: openConfig ? { [openAccountSimConfigKey as symbol]: openConfig } : {},
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: true,
        AccountTableActions: { template: '<div><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        UpstreamBillingRateCell: true,
        HelpTooltip: true,
        Icon: true
      }
    }
  })
}

async function openMoreActions(wrapper: ReturnType<typeof mountView>) {
  await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
  await flushPromises()
}

describe('admin AccountsView 账号管理 #2 配置入口', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
    listAccounts.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    getUpstreamBillingProbeSettings.mockReset().mockResolvedValue({ enabled: false })
    getAllProxies.mockReset().mockResolvedValue([])
    getAllGroups.mockReset().mockResolvedValue([])
  })

  it('真实账号页的更多操作没有配置项', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openMoreActions(wrapper)

    expect(document.body.textContent).toContain('admin.tlsFingerprintProfiles.title')
    expect(document.body.textContent).not.toContain('accountSim.open')
    wrapper.unmount()
  })

  it('提供配置动作时，配置项紧跟 TLS 指纹模板，点击后关闭菜单', async () => {
    const openConfig = vi.fn()
    const wrapper = mountView(openConfig)
    await flushPromises()
    await openMoreActions(wrapper)

    const text = document.body.textContent || ''
    const tlsAt = text.indexOf('admin.tlsFingerprintProfiles.title')
    const configAt = text.indexOf('accountSim.open')
    expect(tlsAt).toBeGreaterThan(-1)
    expect(configAt).toBeGreaterThan(tlsAt)

    const configButton = [...document.body.querySelectorAll('button')].find((button) =>
      button.textContent?.includes('accountSim.open')
    )
    configButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    expect(openConfig).toHaveBeenCalledOnce()
    expect(document.body.textContent).not.toContain('admin.tlsFingerprintProfiles.title')
    wrapper.unmount()
  })
})
