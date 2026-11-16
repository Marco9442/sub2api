/**
 * 账号管理 #2：在该页面内把账号相关请求切到模拟接口。
 * 真实组件不感知切换，只有统一请求入口（apiClient 拦截器与 buildApiUrl）调用这里。
 */

export const ACCOUNT_SIM_PAGE_PATH = '/admin/accounts-2'
export const ACCOUNT_SIM_API_PREFIX = '/admin/account-sim'

/**
 * 账号管理页及其弹窗会请求的接口前缀。
 * 账号页新增其他前缀的请求时，accountSim.coverage.spec.ts 会失败，提示在这里补上。
 */
export const ACCOUNT_SIM_SWITCHED_PREFIXES = [
  '/admin/accounts',
  '/admin/groups',
  '/admin/proxies',
  '/admin/openai',
  '/admin/gemini',
  '/admin/antigravity',
  '/admin/grok',
  '/admin/cn-providers',
  '/admin/scheduled-test-plans',
  '/admin/error-passthrough-rules',
  '/admin/tls-fingerprint-profiles',
  '/admin/settings/web-search-emulation',
] as const

function currentPathname(): string {
  return typeof window === 'undefined' ? '' : window.location.pathname
}

export function isAccountSimPage(pathname: string = currentPathname()): boolean {
  return pathname === ACCOUNT_SIM_PAGE_PATH || pathname.startsWith(`${ACCOUNT_SIM_PAGE_PATH}/`)
}

// 静态表，不读号池。改写后会得到空对象，并被前端缓存成“没有映射”。
const ACCOUNT_SIM_DIRECT_PATHS = new Set(['/admin/accounts/antigravity/default-model-mapping'])

// 按下导入时记住走模拟接口。只覆盖这一次操作，离开页面要取消还没发出的请求。
let pinnedActions = 0

export function pinAccountSimAction(): () => void {
  if (!isAccountSimPage()) return () => {}
  pinnedActions += 1
  let released = false
  return () => {
    if (released) return
    released = true
    pinnedActions -= 1
  }
}

function accountSimActive(pathname: string): boolean {
  return isAccountSimPage(pathname) || pinnedActions > 0
}

/** 在账号管理 #2 页面内，或已记住的操作里，把账号接口改写到模拟接口。 */
export function toAccountSimPath(path: string, pathname: string = currentPathname()): string {
  if (!accountSimActive(pathname)) return path
  const [route] = path.split(/[?#]/, 1)
  if (ACCOUNT_SIM_DIRECT_PATHS.has(route)) return path
  const switched = ACCOUNT_SIM_SWITCHED_PREFIXES.some(
    (prefix) => route === prefix || route.startsWith(`${prefix}/`)
  )
  return switched ? ACCOUNT_SIM_API_PREFIX + path.slice('/admin'.length) : path
}
