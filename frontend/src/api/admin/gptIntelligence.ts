import { apiClient } from '@/api/client'
import type { GptIntelligencePromptTemplate } from '@/api/gptIntelligence'

export interface UpdateGptIntelligenceTemplatesResponse {
  templates: GptIntelligencePromptTemplate[]
}

export async function updateGptIntelligenceTemplates(
  templates: GptIntelligencePromptTemplate[],
): Promise<UpdateGptIntelligenceTemplatesResponse> {
  const { data } = await apiClient.put<UpdateGptIntelligenceTemplatesResponse>(
    '/admin/settings/gpt-intelligence/templates',
    { templates },
  )
  return data
}

export const gptIntelligenceAdminAPI = {
  updateGptIntelligenceTemplates,
}

export default gptIntelligenceAdminAPI
