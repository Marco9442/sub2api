import { describe, it, expect } from 'vitest'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { toAccountSimPath, ACCOUNT_SIM_PAGE_PATH } from '@/api/accountSim'

/**
 * 守护账号管理 #2 的接口覆盖：从账号管理页出发，沿导入关系找出它会请求的全部管理端接口，
 * 要求每个接口都被切到模拟接口，或在 PASSTHROUGH 里写明为什么可以直连真实接口。
 * 上游给账号页新增其他前缀的接口时，这里会失败。
 */

// 可以在模拟页直连真实接口的请求：只读、与业务数据无关，且布局层也会发起。
const PASSTHROUGH: Record<string, string> = {
  '/admin/settings': '只读系统设置（配额提醒全局开关），布局的设置 store 也会读取',
  '/admin/accounts/antigravity/default-model-mapping': '静态模型映射，不读号池',
}

const SRC = resolve(process.cwd(), 'src')
const ENTRY = join(SRC, 'views/admin/AccountsView.vue')
// 布局、全局 store 与 i18n 是所有页面共用的，不属于账号页。接口模块单独解析。
const SKIPPED = ['components/layout/', 'stores/', 'i18n/', 'api/']

function resolveImport(from: string, spec: string): string | null {
  let base: string
  if (spec.startsWith('@/')) base = join(SRC, spec.slice(2))
  else if (spec.startsWith('.')) base = resolve(dirname(from), spec)
  else return null
  for (const candidate of [base, `${base}.ts`, `${base}.vue`, join(base, 'index.ts')]) {
    if (existsSync(candidate) && /\.(ts|vue)$/.test(candidate)) return candidate
  }
  return null
}

function pageFiles(): string[] {
  const seen = new Set<string>()
  const queue = [ENTRY]
  while (queue.length > 0) {
    const file = queue.pop()!
    if (seen.has(file)) continue
    seen.add(file)
    const source = readFileSync(file, 'utf8')
    for (const match of source.matchAll(/(?:from|import\()\s*['"]([^'"]+)['"]/g)) {
      const target = resolveImport(file, match[1])
      if (!target || target.includes('__tests__')) continue
      const relative = target.slice(SRC.length + 1)
      if (SKIPPED.some((prefix) => relative.startsWith(prefix))) continue
      queue.push(target)
    }
  }
  return [...seen]
}

function adminPathsIn(source: string): string[] {
  return [...source.matchAll(/['"`](\/admin\/[^'"`]*)['"`]/g)].map((match) =>
    match[1].replace(/\$\{[^}]+\}/g, ':param')
  )
}

// adminAPI.xxx 的模块名到接口文件的映射，来自 api/admin/index.ts。
function adminModules(): Record<string, string> {
  const index = readFileSync(join(SRC, 'api/admin/index.ts'), 'utf8')
  const files: Record<string, string> = {}
  for (const match of index.matchAll(/import (\w+) from '\.\/(\w+)'/g)) files[match[1]] = match[2]
  const modules: Record<string, string> = {}
  for (const match of index.matchAll(/^\s+(\w+): (\w+),?$/gm)) {
    if (files[match[2]]) modules[match[1]] = files[match[2]]
  }
  return modules
}

// 取接口文件里某个函数体内的路径。函数在导出对象里改名时，跟随到实际函数。
function functionPaths(file: string, name: string): string[] {
  const source = readFileSync(join(SRC, 'api/admin', `${file}.ts`), 'utf8')
  const alias = source.match(new RegExp(`\\b${name}: (\\w+)`))
  const target = alias ? alias[1] : name
  const start = source.search(new RegExp(`(?:function|const) ${target}\\b`))
  if (start < 0) return []
  const rest = source.slice(start + 1)
  const end = rest.search(/\n(?:export |async function |function |const )/)
  return adminPathsIn(end < 0 ? rest : rest.slice(0, end))
}

type Binding = { local: string; imported: string }

// 解析 import 子句里的默认导入和具名导入。跳过 type。
function importBindings(clause: string): Binding[] {
  const bindings: Binding[] = []
  const namedAt = clause.indexOf('{')
  const head = (namedAt < 0 ? clause : clause.slice(0, namedAt)).replace(/,/g, ' ').trim()
  if (head.startsWith('*')) {
    const alias = head.match(/\*\s+as\s+(\w+)/)
    if (alias) bindings.push({ local: alias[1], imported: '*' })
  } else if (head) {
    const name = head.match(/^(\w+)/)
    if (name) bindings.push({ local: name[1], imported: 'default' })
  }
  if (namedAt >= 0) {
    const named = clause.slice(namedAt + 1, clause.indexOf('}', namedAt))
    for (const part of named.split(',')) {
      const piece = part.trim()
      if (!piece || piece.startsWith('type ')) continue
      const match = piece.match(/^(\w+)(?:\s+as\s+(\w+))?$/)
      if (!match) continue
      bindings.push({ local: match[2] || match[1], imported: match[1] })
    }
  }
  return bindings
}

function addFunctionPaths(paths: Set<string>, file: string, name: string) {
  functionPaths(file, name).forEach((path) => paths.add(path))
}

function requestedPaths(): string[] {
  const modules = adminModules()
  const paths = new Set<string>()
  for (const file of pageFiles()) {
    const source = readFileSync(file, 'utf8')
    adminPathsIn(source).forEach((path) => paths.add(path))
    for (const match of source.matchAll(/adminAPI\.(\w+)\s*\.(\w+)/g)) {
      const module = modules[match[1]]
      if (module) addFunctionPaths(paths, module, match[2])
    }
    for (const match of source.matchAll(/import\s+(type\s+)?((?:(?!\bfrom\b)[\s\S])*?)\s+from\s+['"]@\/api\/admin\/(\w+)['"]/g)) {
      if (match[1]) continue
      for (const binding of importBindings(match[2])) {
        if (binding.imported === 'default' || binding.imported === '*') {
          for (const use of source.matchAll(new RegExp(`\\b${binding.local}\\.(\\w+)`, 'g'))) {
            addFunctionPaths(paths, match[3], use[1])
          }
          continue
        }
        const called = new RegExp(`\\b${binding.local}\\s*(?:\\?\\.\\s*)?\\(`).test(source)
        if (called) addFunctionPaths(paths, match[3], binding.imported)
        for (const use of source.matchAll(new RegExp(`\\b${binding.local}\\.(\\w+)`, 'g'))) {
          addFunctionPaths(paths, match[3], use[1])
        }
      }
    }
  }
  return [...paths].sort()
}

describe('账号管理 #2 接口覆盖', () => {
  const paths = requestedPaths()

  it('能找到账号页的主要接口（防止扫描失效后静默通过）', () => {
    expect(paths).toContain('/admin/accounts')
    expect(paths).toContain('/admin/groups/all')
    expect(paths).toContain('/admin/proxies/all')
    expect(paths).toContain('/admin/accounts/:param/test')
    expect(paths.length).toBeGreaterThan(60)
  })

  it('账号页的每个接口都切到模拟接口，或明确允许直连', () => {
    const uncovered = paths.filter((path) => {
      const route = path.split('?')[0]
      return toAccountSimPath(route, ACCOUNT_SIM_PAGE_PATH) === route && !(route in PASSTHROUGH)
    })
    expect(uncovered).toEqual([])
  })

  it('PASSTHROUGH 没有过期条目', () => {
    for (const path of Object.keys(PASSTHROUGH)) {
      expect(paths).toContain(path)
    }
  })
})
