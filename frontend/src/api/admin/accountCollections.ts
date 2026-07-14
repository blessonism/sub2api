import { apiClient } from '../client'

export interface AccountCollection {
  id: number
  name: string
  sort_order: number
  created_at: string
  updated_at: string
}

export async function list(): Promise<AccountCollection[]> {
  const { data } = await apiClient.get<AccountCollection[]>('/admin/account-collections')
  return data
}

export async function create(name: string): Promise<AccountCollection> {
  const { data } = await apiClient.post<AccountCollection>('/admin/account-collections', { name })
  return data
}

export async function update(id: number, name: string): Promise<AccountCollection> {
  const { data } = await apiClient.put<AccountCollection>(`/admin/account-collections/${id}`, { name })
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/account-collections/${id}`)
}

export async function updateSort(ids: number[]): Promise<void> {
  await apiClient.put('/admin/account-collections/sort-order', { ids })
}

export async function listForAccount(accountId: number): Promise<AccountCollection[]> {
  const { data } = await apiClient.get<AccountCollection[]>(`/admin/account-collections/accounts/${accountId}`)
  return data
}

export async function batchMembers(
  accountIds: number[],
  accountCollectionIds: number[],
  operation: 'add' | 'remove'
): Promise<void> {
  await apiClient.post('/admin/account-collections/members/batch', {
    account_ids: accountIds,
    account_collection_ids: accountCollectionIds,
    operation
  })
}

export default { list, create, update, remove, updateSort, listForAccount, batchMembers }
