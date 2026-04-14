// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  browseWorkspaceDir: vi.fn(),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads allowed roots, enters a directory, and emits the selected current path', async () => {
  ;(apiClient.browseWorkspaceDir as any)
    .mockResolvedValueOnce({
      entries: [{ name: 'data', path: '/data' }],
    })
    .mockResolvedValueOnce({
      current_path: '/data',
      root_path: '/data',
      parent_path: '',
      entries: [{ name: 'project', path: '/data/project' }],
    })

  const { default: WorkspaceBrowserModal } = await import('@/components/WorkspaceBrowserModal.vue')
  const wrapper = mount(WorkspaceBrowserModal, {
    props: {
      open: true,
    },
  })

  await flushPromises()

  expect(apiClient.browseWorkspaceDir).toHaveBeenCalledWith(undefined)
  expect(wrapper.text()).toContain('data')

  await wrapper.get('[data-testid="workspace-browser-entry"]').trigger('click')
  await flushPromises()

  expect(apiClient.browseWorkspaceDir).toHaveBeenLastCalledWith('/data')
  expect(wrapper.get('[data-testid="workspace-browser-current-path"]').text()).toContain('/data')

  await wrapper.get('[data-testid="workspace-browser-select-current"]').trigger('click')

  expect(wrapper.emitted('select')).toEqual([['/data']])
})
