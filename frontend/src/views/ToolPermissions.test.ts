// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  getTools: vi.fn(async () => []),
  listAuthTokens: vi.fn(async () => []),
  getToolPolicy: vi.fn(async () => ({})),
  setToolPolicy: vi.fn(async () => ({})),
  createAuthToken: vi.fn(async () => ({})),
  revokeAuthToken: vi.fn(async () => ({ ok: true })),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads tools, tokens, and policy on mount', async () => {
  const { default: ToolPermissions } = await import('@/views/ToolPermissions.vue')

  ;(apiClient.getTools as any).mockResolvedValueOnce([{ id: 'bash', name: 'bash', description: 'bash tool' }])
  ;(apiClient.listAuthTokens as any).mockResolvedValueOnce([])
  ;(apiClient.getToolPolicy as any).mockResolvedValueOnce({
    principal_id: 'local',
    exists: false,
    policy: { id: '' },
    snapshot: { principal_id: 'local', policy: { id: 'default' }, policy_hash: 'abc12345', resolved_at: 't' },
  })

  const wrapper = shallowMount(ToolPermissions)
  await flushPromises()

  expect(apiClient.getTools).toHaveBeenCalled()
  expect(apiClient.listAuthTokens).toHaveBeenCalled()
  expect(apiClient.getToolPolicy).toHaveBeenCalled()
  expect(wrapper.text()).toContain('工具权限')

  wrapper.unmount()
})

it('saves policy JSON via setToolPolicy', async () => {
  const { default: ToolPermissions } = await import('@/views/ToolPermissions.vue')

  ;(apiClient.getTools as any).mockResolvedValueOnce([])
  ;(apiClient.listAuthTokens as any).mockResolvedValueOnce([])
  ;(apiClient.getToolPolicy as any).mockResolvedValueOnce({
    principal_id: 'local',
    exists: true,
    policy: { id: 'p1', default_effect: 'allow', rules: [] },
    snapshot: { principal_id: 'local', policy: { id: 'p1' }, policy_hash: 'abc12345', resolved_at: 't' },
  })
  ;(apiClient.setToolPolicy as any).mockResolvedValueOnce({
    principal_id: 'local',
    exists: true,
    policy: { id: 'p1', default_effect: 'allow', rules: [] },
    snapshot: { principal_id: 'local', policy: { id: 'p1' }, policy_hash: 'abc12345', resolved_at: 't' },
  })

  const wrapper = shallowMount(ToolPermissions)
  await flushPromises()

  const textarea = wrapper.get('textarea')
  await textarea.setValue(JSON.stringify({ id: 'p1', default_effect: 'allow', rules: [] }, null, 2))
  await textarea.trigger('input')

  await wrapper.get('[data-testid="tool-permissions-save"]').trigger('click')
  await flushPromises()

  expect(apiClient.setToolPolicy).toHaveBeenCalled()

  wrapper.unmount()
})
