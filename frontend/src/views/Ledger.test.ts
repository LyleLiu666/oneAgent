// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  getLedgerStatusToday: vi.fn(async () => ({
    day_key: '1970-01-01',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 0,
  })),
  getTodayDigest: vi.fn(),
  getTodayStructuredDigest: vi.fn(async () => ({ items: [], clusters: [] })),
  createLedgerFollowUpTask: vi.fn(async () => ({ id: 't1' })),
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
  expect((apiClient as any).getLedgerStatusToday).toHaveBeenCalled()

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

it('shows digest and SOP badges from status endpoint', async () => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  })

  ;(apiClient as any).getLedgerStatusToday.mockResolvedValueOnce({
    day_key: '2099-01-01',
    digest_exists: true,
    learning_job_status: 'none',
    sop_proposed_count: 3,
  })

  const { default: Ledger } = await import('@/views/Ledger.vue')
  const wrapper = shallowMount(Ledger)
  await flushPromises()

  expect(wrapper.find('[data-testid="ledger-badge-digest"]').exists()).toBe(true)
  const sopBadge = wrapper.find('[data-testid="ledger-badge-sop-count"]')
  expect(sopBadge.exists()).toBe(true)
  expect(sopBadge.text()).toContain('3')

  wrapper.unmount()
})

it('creates a follow-up task from selected digest items', async () => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  })

  ;(apiClient.getTodayDigest as any).mockResolvedValueOnce({
    principal_id: 'local',
    day_key: '2099-01-01',
    markdown: '# Digest',
  })
  ;(apiClient.getTodayStructuredDigest as any).mockResolvedValueOnce({
    principal_id: 'local',
    day_key: '2099-01-01',
    items: [
      {
        receipt_id: 'r1',
        status: 'failed',
        workspace_root: '/tmp/ws',
        summary: 'fix tests',
      },
      {
        receipt_id: 'r2',
        status: 'failed',
        workspace_root: '/tmp/ws',
        summary: 'fix tests',
      },
    ],
    clusters: [{ key: 'fix tests', count: 2, receipt_ids: ['r1', 'r2'] }],
  })

  const { default: Ledger } = await import('@/views/Ledger.vue')
  const wrapper = shallowMount(Ledger)
  await flushPromises()

  await wrapper.get('[data-testid="ledger-tab-digest"]').trigger('click')
  await flushPromises()

  const items = wrapper.findAll('[data-testid="digest-item"] input[type="checkbox"]')
  expect(items.length).toBeGreaterThanOrEqual(2)

  await items[0].setValue(true)
  await flushPromises()

  await wrapper.get('[data-testid="digest-followup-instruction"]').setValue('please follow up')
  await wrapper.get('[data-testid="digest-followup"]').trigger('click')
  await flushPromises()

  expect(apiClient.createLedgerFollowUpTask).toHaveBeenCalledWith(
    expect.objectContaining({
      receipt_ids: expect.arrayContaining(['r1']),
      instruction: 'please follow up',
    }),
  )

  wrapper.unmount()
})
