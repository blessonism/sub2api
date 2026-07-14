import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  remove: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: remove }
}))

import accountCollectionsAPI from '@/api/admin/accountCollections'

describe('admin account collections api', () => {
  beforeEach(() => {
    get.mockReset(); post.mockReset(); put.mockReset(); remove.mockReset()
  })

  it('lists and manages account collections', async () => {
    get.mockResolvedValue({ data: [] })
    post.mockResolvedValue({ data: { id: 1, name: 'Primary' } })
    put.mockResolvedValue({ data: { id: 1, name: 'Renamed' } })
    remove.mockResolvedValue({ data: {} })

    await accountCollectionsAPI.list()
    await accountCollectionsAPI.create('Primary')
    await accountCollectionsAPI.update(1, 'Renamed')
    await accountCollectionsAPI.remove(1)

    expect(get).toHaveBeenCalledWith('/admin/account-collections')
    expect(post).toHaveBeenCalledWith('/admin/account-collections', { name: 'Primary' })
    expect(put).toHaveBeenCalledWith('/admin/account-collections/1', { name: 'Renamed' })
    expect(remove).toHaveBeenCalledWith('/admin/account-collections/1')
  })

  it('submits incremental membership operations', async () => {
    post.mockResolvedValue({ data: {} })
    await accountCollectionsAPI.batchMembers([1, 2], [7, 8], 'remove')
    expect(post).toHaveBeenCalledWith('/admin/account-collections/members/batch', {
      account_ids: [1, 2],
      account_collection_ids: [7, 8],
      operation: 'remove'
    })
  })
})
