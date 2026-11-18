import { describe, expect, it } from 'vitest'

import { isSameOriginPath, sanitizeUrl } from '../url'

describe('isSameOriginPath', () => {
  it('accepts a same-origin path', () => {
    expect(isSameOriginPath('/help/')).toBe(true)
    expect(isSameOriginPath('/model-plaza')).toBe(true)
  })

  it('rejects protocol-relative, slash, colon, and whitespace paths', () => {
    expect(isSameOriginPath('//evil.example')).toBe(false)
    expect(isSameOriginPath('/help/\\admin')).toBe(false)
    expect(isSameOriginPath('/javascript:alert(1)')).toBe(false)
    expect(isSameOriginPath('/help/ page')).toBe(false)
    expect(isSameOriginPath('javascript:alert(1)')).toBe(false)
    expect(isSameOriginPath('https://baimaapi.top/help/')).toBe(false)
  })
})

describe('sanitizeUrl relative docs links', () => {
  it('keeps a same-origin path when relative links are allowed', () => {
    expect(sanitizeUrl('/help/', { allowRelative: true })).toBe('/help/')
  })

  it('drops a same-origin path unless relative links are allowed', () => {
    expect(sanitizeUrl('/help/')).toBe('')
  })

  it('drops an unsafe relative path even when relative links are allowed', () => {
    expect(sanitizeUrl('//evil.example', { allowRelative: true })).toBe('')
    expect(sanitizeUrl('/help/\\admin', { allowRelative: true })).toBe('')
    expect(sanitizeUrl('/javascript:alert(1)', { allowRelative: true })).toBe('')
  })
})
