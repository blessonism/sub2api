import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

import { reportActivity } from '@/api/auth'

describe('auth activity api', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('posts foreground activity to the user activity endpoint', async () => {
    post.mockResolvedValue({ data: { reported: true } })

    await reportActivity()

    expect(post).toHaveBeenCalledWith('/user/activity')
  })
})
