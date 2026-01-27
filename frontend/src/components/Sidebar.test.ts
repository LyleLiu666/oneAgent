// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/chat' }),
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({ user: { name: 'u' }, logout: vi.fn() }),
}))

vi.mock('@/composables/useTheme', () => ({
  useTheme: () => ({ isDark: false, toggleTheme: vi.fn() }),
}))

vi.mock('@/api/client', () => ({
  getLedgerStatusToday: vi.fn(async () => ({
    day_key: '2026-01-27',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 3,
  })),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('shows SOP badge on Governance nav item', async () => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  })

  const { default: Sidebar } = await import('@/components/Sidebar.vue')
  const wrapper = shallowMount(Sidebar)
  await flushPromises()

  const badge = wrapper.find('[data-testid="sidebar-sop-badge"]')
  expect(badge.exists()).toBe(true)
  expect(badge.text()).toContain('3')

  wrapper.unmount()
})

