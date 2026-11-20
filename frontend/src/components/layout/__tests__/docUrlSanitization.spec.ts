import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')
const settingsSource = readFileSync(resolve(dir, '../../../views/admin/SettingsView.vue'), 'utf8')

describe('doc_url sanitization', () => {
  it('AppHeader imports sanitizeUrl', () => {
    expect(headerSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('AppHeader applies sanitizeUrl to docUrl', () => {
    expect(headerSource).toContain('sanitizeUrl(appStore.docUrl, { allowRelative: true })')
  })

  it('HomeView imports sanitizeUrl', () => {
    expect(homeViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('HomeView applies sanitizeUrl to docUrl', () => {
    expect(homeViewSource).toContain("sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '', { allowRelative: true })")
  })

  it('KeyUsageView imports sanitizeUrl', () => {
    expect(keyUsageViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('KeyUsageView applies sanitizeUrl to docUrl', () => {
    expect(keyUsageViewSource).toContain("sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '', { allowRelative: true })")
  })

  it('SettingsView accepts a same-origin docs path and still clears an invalid value', () => {
    expect(settingsSource).toContain('import { isSameOriginPath } from "@/utils/url"')
    expect(settingsSource).toContain('return isSameOriginPath(url) || isValidHttpUrl(url)')
    expect(settingsSource).toContain('if (!isValidDocUrl(form.doc_url)) form.doc_url = ""')
  })
})
