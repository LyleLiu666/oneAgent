// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  listSkills: vi.fn(async () => []),
  listSkillDuplicates: vi.fn(async () => []),
  archiveSkill: vi.fn(async () => ({})),
  pinSkillCandidate: vi.fn(async () => ({})),
  archiveShadowedPersonalDuplicates: vi.fn(async () => ({})),
  getSkill: vi.fn(async () => ({})),
  updateSkill: vi.fn(async () => ({})),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads skills and renders list', async () => {
  const { default: SkillGovernance } = await import('@/views/SkillGovernance.vue')

  ;(apiClient.listSkills as any).mockResolvedValueOnce([
    {
      skill_id: 'a',
      name: 'A',
      description: 'd',
      source: '.oneagent',
      path: '/tmp/a/SKILL.md',
      archivable: true,
    },
  ])

  const wrapper = shallowMount(SkillGovernance)
  await flushPromises()

  expect(apiClient.listSkills).toHaveBeenCalled()
  expect((apiClient as any).listSkillDuplicates).toHaveBeenCalled()
  expect(wrapper.text()).toContain('A')
  expect(wrapper.find('[data-testid="skill-archive"]').exists()).toBe(true)

  wrapper.unmount()
})

it('loads skill details when selected and saves with OCC', async () => {
  const { default: SkillGovernance } = await import('@/views/SkillGovernance.vue')

  ;(apiClient.listSkills as any).mockResolvedValueOnce([
    { skill_id: 'a', name: 'A', description: 'd', source: '.oneagent', path: '/tmp/a/SKILL.md', archivable: true },
  ])

  ;(apiClient.getSkill as any).mockResolvedValueOnce({
    skill_id: 'a',
    name: 'A',
    description: 'd',
    source: '.oneagent',
    path: '/tmp/a/SKILL.md',
    archivable: true,
    sha256: 'abc',
    skill_md: 'old',
  })

  ;(apiClient.updateSkill as any).mockResolvedValueOnce({
    skill_id: 'a',
    name: 'A',
    description: 'd',
    source: '.oneagent',
    path: '/tmp/a/SKILL.md',
    archivable: true,
    sha256: 'def',
    skill_md: 'new',
  })

  const wrapper = shallowMount(SkillGovernance)
  await flushPromises()

  expect((apiClient as any).listSkillDuplicates).toHaveBeenCalled()

  await wrapper.get('[data-testid="skill-item"]').trigger('click')
  await flushPromises()

  const textarea = wrapper.get('textarea')
  await textarea.setValue('new')

  await wrapper.get('[data-testid="skill-save"]').trigger('click')
  await flushPromises()

  expect(apiClient.updateSkill).toHaveBeenCalledWith(expect.objectContaining({ skill_id: 'a', expected_sha256: 'abc' }))

  wrapper.unmount()
})

it('pins a candidate from duplicates list', async () => {
  const { default: SkillGovernance } = await import('@/views/SkillGovernance.vue')

  ;(apiClient.listSkills as any).mockResolvedValueOnce([])
  ;(apiClient.listSkillDuplicates as any).mockResolvedValueOnce([
    {
      skill_id: 'foo',
      candidates: [
        {
          skill_id: 'foo',
          name: 'foo',
          description: 'from claude',
          source: '.claude',
          path: '/tmp/.claude/skills/foo/SKILL.md',
          archivable: false,
          effective: false,
          precedence_rank: 2,
        },
        {
          skill_id: 'foo',
          name: 'foo',
          description: 'personal',
          source: '.oneagent',
          path: '/tmp/.oneagent/skills/foo/SKILL.md',
          archivable: true,
          effective: true,
          precedence_rank: 3,
        },
      ],
    },
  ])

  const wrapper = shallowMount(SkillGovernance)
  await flushPromises()

  expect(wrapper.find('[data-testid="skill-governance-advanced"]').exists()).toBe(false)

  await wrapper.get('[data-testid="skill-governance-advanced-toggle"]').trigger('click')
  await flushPromises()

  expect(wrapper.find('[data-testid="skill-governance-advanced"]').exists()).toBe(true)

  await wrapper.get('[data-testid="duplicate-pin"]').trigger('click')
  await flushPromises()

  expect((apiClient as any).pinSkillCandidate).toHaveBeenCalledWith(
    'foo',
    expect.objectContaining({ source: '.claude', path: '/tmp/.claude/skills/foo/SKILL.md' })
  )

  wrapper.unmount()
})
