// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  listSopSuggestions: vi.fn(async () => []),
  loadMoreSopSuggestions: vi.fn(async () => []),
  updateSopSuggestionStatus: vi.fn(async () => ({})),
  updateSopSuggestion: vi.fn(async () => ({})),
  getSimilarSopSuggestions: vi.fn(async () => []),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads and renders suggestions sorted by total_score', async () => {
  const { default: SopGovernance } = await import('@/views/SopGovernance.vue')

  ;(apiClient.listSopSuggestions as any).mockResolvedValueOnce([
    {
      suggestion_id: 's1',
      principal_id: 'local',
      title: 'Low',
      status: 'proposed',
      evidence_receipt_ids: [],
      evidence_count: 0,
      draft_skill: 'a',
      scores: { scarcity_score: 0.1, depth_score: 0.1, evidence_score: 0, total_score: 0.1 },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      suggestion_id: 's2',
      principal_id: 'local',
      title: 'High',
      status: 'proposed',
      evidence_receipt_ids: [],
      evidence_count: 0,
      draft_skill: 'b',
      scores: { scarcity_score: 0.9, depth_score: 0.9, evidence_score: 0.9, total_score: 0.9 },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ])

  const wrapper = shallowMount(SopGovernance)
  await flushPromises()

  const cards = wrapper.findAll('.glass-card')
  expect(cards.length).toBe(2)
  expect(cards[0].text()).toContain('High')
  expect(cards[1].text()).toContain('Low')

  wrapper.unmount()
})

it('shows governance hints and can merge into recommended target', async () => {
  const { default: SopGovernance } = await import('@/views/SopGovernance.vue')

  ;(apiClient.listSopSuggestions as any).mockResolvedValueOnce([
    {
      suggestion_id: 's1',
      principal_id: 'local',
      title: 'Needs merge',
      status: 'proposed',
      evidence_receipt_ids: [],
      evidence_count: 0,
      draft_skill: 'a',
      scores: { scarcity_score: 0.1, depth_score: 0.1, evidence_score: 0, total_score: 0.1 },
      meta: {
        recommended_merge_target_id: 'target-12345678',
        similar_suggestion_ids: ['target-12345678', 'other-abcdef01'],
      },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ])

  const wrapper = shallowMount(SopGovernance)
  await flushPromises()

  expect(wrapper.find('[data-testid="sop-governance-hints"]').exists()).toBe(true)

  await wrapper.get('[data-testid="sop-merge-recommended"]').trigger('click')
  await flushPromises()

  expect((apiClient as any).updateSopSuggestionStatus).toHaveBeenCalledWith(
    's1',
    expect.objectContaining({ status: 'merged', merged_into_suggestion_id: 'target-12345678' }),
  )

  wrapper.unmount()
})
