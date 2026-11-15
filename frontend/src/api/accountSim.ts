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

/** 在账号管理 #2 页面内，把以 /admin/... 开头的接口路径改写到模拟接口；其他情况原样返回。 */
export function toAccountSimPath(path: string, pathname: string = currentPathname()): string {
  if (!isAccountSimPage(pathname)) return path
  const [route] = path.split(/[?#]/, 1)
  const switched = ACCOUNT_SIM_SWITCHED_PREFIXES.some(
    (prefix) => route === prefix || route.startsWith(`${prefix}/`)
  )
  return switched ? ACCOUNT_SIM_API_PREFIX + path.slice('/admin'.length) : path
}
