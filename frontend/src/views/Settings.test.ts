// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'
import { ref } from 'vue'

const mockAuthMode = ref<'token' | 'none' | 'unknown'>('token')

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({ authMode: mockAuthMode }),
}))

vi.mock('@/api/client', () => ({
  getProviders: vi.fn(async () => []),
  createProvider: vi.fn(async () => ({})),
  deleteProvider: vi.fn(async () => ({ ok: true })),
  createModel: vi.fn(async () => ({})),
  deleteModel: vi.fn(async () => ({ ok: true })),
  updateModel: vi.fn(async () => ({})),

  getCommandApprovalSettings: vi.fn(async () => ({ command_approval_mode: 'auto' })),
  updateCommandApprovalSettings: vi.fn(async () => ({ command_approval_mode: 'auto' })),

  getBochaSettings: vi.fn(async () => ({ has_bocha_api_key: false })),
  updateBochaSettings: vi.fn(async () => ({})),
  bochaSearch: vi.fn(async () => ({ code: 200, data: {} })),

  getTodayDigest: vi.fn(async () => ({ markdown: '', day_key: '' })),

  listSopSuggestions: vi.fn(async () => []),
  generateSopSuggestions: vi.fn(async () => []),
  updateSopSuggestionStatus: vi.fn(async () => ({})),
  loadMoreSopSuggestions: vi.fn(async () => []),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads command approval settings on mount', async () => {
  mockAuthMode.value = 'token'
  const { default: Settings } = await import('@/views/Settings.vue')

  ;(apiClient.getCommandApprovalSettings as any).mockResolvedValueOnce({ command_approval_mode: 'auto' })

  const wrapper = shallowMount(Settings)
  await flushPromises()

  expect(apiClient.getCommandApprovalSettings).toHaveBeenCalled()

  wrapper.unmount()
})

it('updates command approval mode when toggled', async () => {
  mockAuthMode.value = 'token'
  const { default: Settings } = await import('@/views/Settings.vue')

  ;(apiClient.getCommandApprovalSettings as any).mockResolvedValueOnce({ command_approval_mode: 'auto' })
  ;(apiClient.updateCommandApprovalSettings as any).mockResolvedValueOnce({ command_approval_mode: 'manual' })

  const wrapper = shallowMount(Settings)
  await flushPromises()

  await wrapper.get('[data-testid="settings-tab-security"]').trigger('click')
  await flushPromises()

  await wrapper.get('[data-testid="command-approval-manual"]').trigger('click')
  await flushPromises()

  expect(apiClient.updateCommandApprovalSettings).toHaveBeenCalledWith({ command_approval_mode: 'manual' })

  wrapper.unmount()
})

it('shows AUTH_MODE=none guidance when authentication is disabled', async () => {
  mockAuthMode.value = 'none'
  const { default: Settings } = await import('@/views/Settings.vue')
  const wrapper = shallowMount(Settings)
  await flushPromises()

  expect(wrapper.text()).toContain('当前实例未启用认证（AUTH_MODE=none）')
  expect(wrapper.text()).not.toContain('当前实例使用本地访问令牌保护（AUTH_MODE=token）')

  wrapper.unmount()
})
