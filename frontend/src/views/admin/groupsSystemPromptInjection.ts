import type { GroupPlatform, GroupSystemPromptInjection } from '@/types'
import { getPlatformSpec } from '@/constants/platformCatalog'

export const supportsSystemPromptInjection = (platform: GroupPlatform): boolean =>
  platform === 'composite' || !!getPlatformSpec(platform)

/** 创建独立的表单/提交快照，覆盖登记平台及组合分组。 */
export function normalizeSystemPromptInjection(
  platform: GroupPlatform,
  config?: GroupSystemPromptInjection
): GroupSystemPromptInjection {
  return {
    enabled: supportsSystemPromptInjection(platform) && (config?.enabled ?? false),
    mode: config?.mode === 'prepend' || config?.mode === 'replace' ? config.mode : 'append',
    content: config?.content ?? ''
  }
}
