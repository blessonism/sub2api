import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    put,
  },
}))

import { updateGptIntelligenceTemplates } from '@/api/admin/gptIntelligence'

describe('admin gpt intelligence api', () => {
  beforeEach(() => {
    put.mockReset()
  })

  it('saves global intelligence check templates', async () => {
    const response = { templates: [{ id: 'logic', title: '全局题', description: '', prompt: 'p', expected: 'e', threshold: 't' }] }
    put.mockResolvedValue({ data: response })

    await expect(updateGptIntelligenceTemplates(response.templates)).resolves.toEqual(response)
    expect(put).toHaveBeenCalledWith('/admin/settings/gpt-intelligence/templates', { templates: response.templates })
  })
})
