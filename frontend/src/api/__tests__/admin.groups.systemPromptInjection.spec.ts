import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GroupSystemPromptInjection } from '@/types'

const { post, put, get } = vi.hoisted(() => ({ post: vi.fn(), put: vi.fn(), get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post, put, get } }))
import { create, getById, update } from '@/api/admin/groups'

const config: GroupSystemPromptInjection = { enabled: true, mode: 'replace', content: 'Custom instructions' }

describe('admin group system prompt API contract', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const response = { data: { id: 42, name: 'Custom group', system_prompt_injection: config } }
    post.mockResolvedValue(response)
    put.mockResolvedValue(response)
    get.mockResolvedValue(response)
  })

  it('sends and returns the complete injection configuration through existing endpoints', async () => {
    const request = { name: 'Custom group', platform: 'openai' as const, system_prompt_injection: config }
    expect((await create(request)).system_prompt_injection).toEqual(config)
    expect(post).toHaveBeenCalledWith('/admin/groups', request)
    expect((await update(42, { system_prompt_injection: config })).system_prompt_injection).toEqual(config)
    expect(put).toHaveBeenCalledWith('/admin/groups/42', { system_prompt_injection: config })
    expect((await getById(42)).system_prompt_injection).toEqual(config)
    expect(get).toHaveBeenCalledWith('/admin/groups/42')
  })

  it('keeps omitted update configuration absent so the server can retain it', async () => {
    await update(42, { name: 'Renamed group' })
    expect(put).toHaveBeenCalledWith('/admin/groups/42', { name: 'Renamed group' })
    expect(put.mock.calls[0][1]).not.toHaveProperty('system_prompt_injection')
  })
})
