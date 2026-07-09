type LocaleTree = Record<string, unknown>

const isLocaleTree = (value: unknown): value is LocaleTree =>
  value !== null && typeof value === 'object' && !Array.isArray(value)

export function mergeLocale<T extends LocaleTree>(base: T, overlay: LocaleTree): T {
  const result: LocaleTree = { ...base }

  for (const [key, value] of Object.entries(overlay)) {
    const existing = result[key]
    if (isLocaleTree(existing) && isLocaleTree(value)) {
      result[key] = mergeLocale(existing, value)
      continue
    }
    result[key] = value
  }

  return result as T
}
