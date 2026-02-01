// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

const makeLocalStorage = () => {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  }
}

const mocks = vi.hoisted(() => {
  return {
    routerPush: vi.fn(),
    getLedgerStatusToday: vi.fn(async () => ({
      day_key: '2026-02-01',
      digest_exists: false,
      learning_job_status: 'none',
      sop_proposed_count: 0,
    })),
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}))

vi.mock('@/api/client', () => ({
  getLedgerStatusToday: mocks.getLedgerStatusToday,
}))

it('renders SOP hint when proposed count > 0', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  mocks.getLedgerStatusToday.mockResolvedValueOnce({
    day_key: '2026-02-01',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 3,
  })

  const { default: SecretaryStatusHints } = await import('@/components/SecretaryStatusHints.vue')
  const wrapper = mount(SecretaryStatusHints, {
    global: { plugins: [pinia] },
  })

  await flushPromises()

  const hint = wrapper.find('[data-testid="secretary-sop-hint"]')
  expect(hint.exists()).toBe(true)
  expect(wrapper.get('[data-testid="secretary-sop-hint-count"]').text()).toBe('3')

  wrapper.unmount()
})

it('does not render SOP hint when proposed count is 0', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  mocks.getLedgerStatusToday.mockResolvedValueOnce({
    day_key: '2026-02-01',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 0,
  })

  const { default: SecretaryStatusHints } = await import('@/components/SecretaryStatusHints.vue')
  const wrapper = mount(SecretaryStatusHints, {
    global: { plugins: [pinia] },
  })

  await flushPromises()

  expect(wrapper.find('[data-testid="secretary-sop-hint"]').exists()).toBe(false)

  wrapper.unmount()
})

it('switches to full mode and navigates when clicking the hint', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()
  ui.setMode('secretary')

  mocks.getLedgerStatusToday.mockResolvedValueOnce({
    day_key: '2026-02-01',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 1,
  })

  const { default: SecretaryStatusHints } = await import('@/components/SecretaryStatusHints.vue')
  const wrapper = mount(SecretaryStatusHints, {
    global: { plugins: [pinia] },
  })

  await flushPromises()

  await wrapper.get('[data-testid="secretary-sop-hint"]').trigger('click')
  await flushPromises()

  expect(ui.mode).toBe('full')
  expect(mocks.routerPush).toHaveBeenCalledWith('/governance/sop')

  wrapper.unmount()
})

