import { describe, expect, it } from 'vitest'
import router from '@/router'

describe('已退役的对话历史页面', () => {
  it.each(['/admin/conversations', '/admin/conversations/123'])('旧地址 %s 进入现有 404', (path) => {
    expect(router.resolve(path).name).toBe('NotFound')
  })

  it('其他管理页面仍可解析', () => {
    expect(router.resolve('/admin/groups').name).toBe('AdminGroups')
    expect(router.resolve('/admin/usage').name).toBe('AdminUsage')
  })
})
