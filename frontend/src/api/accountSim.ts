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

// 只切这一条仪表盘接口。账号统计的模型明细会读它，其余仪表盘仍走真实接口。
const ACCOUNT_SIM_EXACT_PATHS = new Set(['/admin/dashboard/user-breakdown'])

// 只在一次同步调度期间存在。异步间隙里不持有，避免把其他页面的请求也改走模拟接口。
let forced: boolean | undefined

/** 固定这一次调用的请求来源。只覆盖 fn 同步执行的部分，请求必须在这次调用内发出。 */
export function withAccountSimRequest<T>(sim: boolean, fn: () => T): T {
  const previous = forced
  forced = sim
  try {
    return fn()
  } finally {
    forced = previous
  }
}

/**
 * 把对象上的函数调用固定到来源。非函数原样返回。
 * 组件卸载后尚未发出的调用不再打接口。
 */
export function bindAccountSimCalls<T extends object>(target: T, sim: boolean, alive: () => boolean): T {
  return new Proxy(target, {
    get(obj, prop, receiver) {
      const value = Reflect.get(obj, prop, receiver)
      if (typeof value !== 'function') return value
      return (...args: unknown[]) => {
        if (!alive()) return new Promise(() => {})
        return withAccountSimRequest(sim, () => Reflect.apply(value, obj, args))
      }
    },
  })
}

function accountSimActive(pathname: string): boolean {
  return forced !== undefined ? forced : isAccountSimPage(pathname)
}

/** 在账号管理 #2 页面内，或这次调用已固定的来源里，把账号接口改写到模拟接口。 */
export function toAccountSimPath(path: string, pathname: string = currentPathname()): string {
  if (!accountSimActive(pathname)) return path
  const [route] = path.split(/[?#]/, 1)
  if (ACCOUNT_SIM_DIRECT_PATHS.has(route)) return path
  if (ACCOUNT_SIM_EXACT_PATHS.has(route)) {
    return ACCOUNT_SIM_API_PREFIX + path.slice('/admin'.length)
  }
  const switched = ACCOUNT_SIM_SWITCHED_PREFIXES.some(
    (prefix) => route === prefix || route.startsWith(`${prefix}/`)
  )
  return switched ? ACCOUNT_SIM_API_PREFIX + path.slice('/admin'.length) : path
}
