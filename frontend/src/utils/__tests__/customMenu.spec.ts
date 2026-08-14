import { describe, expect, it } from 'vitest'

import { isExternalCustomMenuItem, normalizeCustomMenuOpenMode } from '../customMenu'

describe('normalizeCustomMenuOpenMode', () => {
  it('defaults missing and unknown values to embed', () => {
    expect(normalizeCustomMenuOpenMode(undefined)).toBe('embed')
    expect(normalizeCustomMenuOpenMode('')).toBe('embed')
    expect(normalizeCustomMenuOpenMode('popup')).toBe('embed')
    expect(normalizeCustomMenuOpenMode('embed')).toBe('embed')
    expect(normalizeCustomMenuOpenMode('external')).toBe('external')
  })
})

describe('isExternalCustomMenuItem', () => {
  it('requires external mode and an absolute http(s) URL', () => {
    expect(isExternalCustomMenuItem({
      open_mode: 'external',
      url: 'https://canvas.example/app',
    })).toBe(true)
    expect(isExternalCustomMenuItem({
      url: 'https://canvas.example/app',
    })).toBe(false)
    expect(isExternalCustomMenuItem({
      open_mode: 'embed',
      url: 'https://canvas.example/app',
    })).toBe(false)
    expect(isExternalCustomMenuItem({
      open_mode: 'external',
      url: 'md:help',
    })).toBe(false)
    expect(isExternalCustomMenuItem({
      open_mode: 'external',
      url: 'javascript:alert(1)',
    })).toBe(false)
    expect(isExternalCustomMenuItem({
      open_mode: 'external',
      url: 'HTTPS://canvas.example/app',
    })).toBe(true)
  })
})
