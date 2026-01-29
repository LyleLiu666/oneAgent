// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  exportDocument: vi.fn(async () => ({ ok: true, output_path: 'report.docx' })),
  chooseWorkspaceDir: vi.fn(async () => ({ path: '/tmp/ws' })),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('exports markdown to docx', async () => {
  const store = new Map<string, string>([['oneagent-workspace', '/tmp/ws']])
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) || null,
    setItem: (k: string, v: string) => {
      store.set(k, String(v))
    },
    removeItem: (k: string) => {
      store.delete(k)
    },
    clear: () => {
      store.clear()
    },
  })

  const { default: DocumentExport } = await import('@/views/DocumentExport.vue')
  const wrapper = shallowMount(DocumentExport)
  await flushPromises()

  await wrapper.get('[data-testid="doc-export-input"]').setValue('report.md')
  await wrapper.get('[data-testid="doc-export-format"]').setValue('docx')

  await wrapper.get('[data-testid="doc-export-run"]').trigger('click')
  await flushPromises()

  expect(apiClient.exportDocument).toHaveBeenCalledWith(
    expect.objectContaining({ workspace: '/tmp/ws', input_path: 'report.md', format: 'docx' })
  )
  expect(wrapper.text()).toContain('report.docx')

  wrapper.unmount()
})

it('shows ErrorBanner when export fails', async () => {
  const store = new Map<string, string>([['oneagent-workspace', '/tmp/ws']])
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) || null,
    setItem: (k: string, v: string) => {
      store.set(k, String(v))
    },
    removeItem: (k: string) => {
      store.delete(k)
    },
    clear: () => {
      store.clear()
    },
  })

  ;(apiClient.exportDocument as any).mockRejectedValueOnce({
    data: { error: 'Export failed', request_id: 'req_1', hint: 'Try again' },
  })

  const { default: DocumentExport } = await import('@/views/DocumentExport.vue')
  const wrapper = shallowMount(DocumentExport)
  await flushPromises()

  await wrapper.get('[data-testid="doc-export-run"]').trigger('click')
  await flushPromises()

  const banner = wrapper.findComponent({ name: 'ErrorBanner' })
  expect(banner.exists()).toBe(true)

  const error = banner.props('error') as any
  expect(error?.requestId).toBe('req_1')

  wrapper.unmount()
})
