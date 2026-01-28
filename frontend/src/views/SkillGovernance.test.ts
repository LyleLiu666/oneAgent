// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
  listSkills: vi.fn(async () => []),
  archiveSkill: vi.fn(async () => ({})),
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
  expect(wrapper.text()).toContain('A')
  expect(wrapper.find('[data-testid="skill-archive"]').exists()).toBe(true)

  wrapper.unmount()
})

