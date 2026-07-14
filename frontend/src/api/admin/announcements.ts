/**
 * Admin Announcements API endpoints
 */

import { apiClient } from '../client'
import type {
  Announcement,
  AnnouncementEmailBroadcast,
  AnnouncementEmailBroadcastOverview,
  AnnouncementEmailDelivery,
  AnnouncementEmailDeliveryStatus,
  AnnouncementUserReadStatus,
  BasePaginationResponse,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest
} from '@/types'

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: string
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<BasePaginationResponse<Announcement>> {
  const { data } = await apiClient.get<BasePaginationResponse<Announcement>>('/admin/announcements', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}

export async function getEmailBroadcast(id: number): Promise<AnnouncementEmailBroadcastOverview> {
  const { data } = await apiClient.get<AnnouncementEmailBroadcastOverview>(`/admin/announcements/${id}/email-broadcast`)
  return data
}

export async function createEmailBroadcast(id: number): Promise<AnnouncementEmailBroadcast> {
  const { data } = await apiClient.post<AnnouncementEmailBroadcast>(`/admin/announcements/${id}/email-broadcast`)
  return data
}

export async function listEmailDeliveries(
  id: number,
  page: number = 1,
  pageSize: number = 20,
  filters?: { status?: AnnouncementEmailDeliveryStatus | ''; search?: string }
): Promise<BasePaginationResponse<AnnouncementEmailDelivery>> {
  const { data } = await apiClient.get<BasePaginationResponse<AnnouncementEmailDelivery>>(
    `/admin/announcements/${id}/email-broadcast/deliveries`,
    { params: { page, page_size: pageSize, ...filters } }
  )
  return data
}

export async function retryFailedEmailDeliveries(id: number): Promise<AnnouncementEmailBroadcast> {
  const { data } = await apiClient.post<AnnouncementEmailBroadcast>(`/admin/announcements/${id}/email-broadcast/retry-failed`)
  return data
}

export async function getById(id: number): Promise<Announcement> {
  const { data } = await apiClient.get<Announcement>(`/admin/announcements/${id}`)
  return data
}

export async function create(request: CreateAnnouncementRequest): Promise<Announcement> {
  const { data } = await apiClient.post<Announcement>('/admin/announcements', request)
  return data
}

export async function update(id: number, request: UpdateAnnouncementRequest): Promise<Announcement> {
  const { data } = await apiClient.put<Announcement>(`/admin/announcements/${id}`, request)
  return data
}

export async function deleteAnnouncement(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/announcements/${id}`)
  return data
}

export async function getReadStatus(
  id: number,
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<BasePaginationResponse<AnnouncementUserReadStatus>> {
  const { data } = await apiClient.get<BasePaginationResponse<AnnouncementUserReadStatus>>(
    `/admin/announcements/${id}/read-status`,
    {
      params: { page, page_size: pageSize, ...filters },
      signal: options?.signal
    }
  )
  return data
}

const announcementsAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteAnnouncement,
  getReadStatus,
  getEmailBroadcast,
  createEmailBroadcast,
  listEmailDeliveries,
  retryFailedEmailDeliveries
}

export default announcementsAPI
