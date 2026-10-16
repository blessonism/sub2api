import type { GroupPlatform, GroupSystemPromptInjection } from '@/types'

export const supportsSystemPromptInjection = (platform: GroupPlatform): boolean =>
  platform === 'openai' || platform === 'anthropic'

/** 创建独立的表单/提交快照，同时禁止在其他平台开启。 */
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
