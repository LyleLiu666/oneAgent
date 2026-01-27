// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  getTodayDigest: vi.fn(),
  listReceipts: vi.fn(async () => []),
  listSopSuggestions: vi.fn(async () => []),
  generateSopSuggestions: vi.fn(async () => []),
  loadMoreSopSuggestions: vi.fn(async () => []),
  updateSopSuggestionStatus: vi.fn(async () => ({})),
  updateSopSuggestion: vi.fn(async () => ({})),
  getSimilarSopSuggestions: vi.fn(async () => []),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads receipts and shows details after selection', async () => {
  const store = new Map<string, string>([['oneagent-workspace', '/tmp/ws']])
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const { default: Ledger } = await import('@/views/Ledger.vue')

  ;(apiClient.listReceipts as any).mockResolvedValueOnce([
    {
      receipt_id: 'r1',
      principal_id: 'local',
      workspace_root: '/tmp/ws',
      kind: 'subagent_run',
      status: 'succeeded',
      started_at: new Date().toISOString(),
      finished_at: new Date().toISOString(),
      summary: 'did work',
      artifacts: { findings_path: 'findings.md', trace_log_path: 'trace.jsonl' },
    },
  ])

  const wrapper = shallowMount(Ledger)
  await flushPromises()

  expect(apiClient.listReceipts).toHaveBeenCalled()

  const items = wrapper.findAll('[data-testid="receipt-item"]')
  expect(items.length).toBe(1)

  await items[0].trigger('click')
  await flushPromises()

  expect(wrapper.text()).toContain('did work')

  wrapper.unmount()
})

it('edits a proposed SOP suggestion and saves via API', async () => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  })

  const { default: Ledger } = await import('@/views/Ledger.vue')

  ;(apiClient.listReceipts as any).mockResolvedValueOnce([])
  ;(apiClient.listSopSuggestions as any).mockResolvedValue([
    {
      suggestion_id: 's1',
      principal_id: 'local',
      title: 'SOP: old',
      description: '',
      risk_notes: '',
      status: 'proposed',
      evidence_receipt_ids: ['r1', 'r2'],
      evidence_count: 2,
      draft_skill: '# Skill\n\n1. Old',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ])

  const wrapper = shallowMount(Ledger)
  await flushPromises()

  await wrapper.get('[data-testid="ledger-tab-sop"]').trigger('click')
  await flushPromises()

  await wrapper.get('[data-testid="sop-edit"]').trigger('click')
  await flushPromises()

  const titleInput = wrapper.get('[data-testid="sop-edit-title"]')
  await titleInput.setValue('SOP: new')

  const draft = wrapper.get('[data-testid="sop-edit-draft"]')
  await draft.setValue('# Skill\n\n1. New')

  await wrapper.get('[data-testid="sop-edit-save"]').trigger('click')
  await flushPromises()

  expect(apiClient.updateSopSuggestion).toHaveBeenCalledWith('s1', expect.objectContaining({ title: 'SOP: new' }))

  wrapper.unmount()
})

