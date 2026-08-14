import type { CustomMenuItem, CustomMenuOpenMode } from '@/types'
import { sanitizeUrl } from '@/utils/url'

export function normalizeCustomMenuOpenMode(mode?: string | null): CustomMenuOpenMode {
  return mode === 'external' ? 'external' : 'embed'
}

export function isExternalCustomMenuItem(
  item: Pick<CustomMenuItem, 'open_mode' | 'url'> | { open_mode?: string | null; url?: string | null },
): boolean {
  if (normalizeCustomMenuOpenMode(item.open_mode) !== 'external') return false
  const url = item.url?.trim() ?? ''
  if (!url || url.startsWith('md:')) return false
  return sanitizeUrl(url) !== ''
}
