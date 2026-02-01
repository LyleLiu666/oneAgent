// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('vue-router', () => ({
  RouterView: { name: 'RouterView', template: '<div data-testid="router-view" />' },
  useRoute: () => ({ meta: { requiresFullMode: true } }),
}))

vi.mock('@/components/Sidebar.vue', () => ({
  default: { name: 'Sidebar', template: '<aside />' },
}))

it('hides global Sidebar in secretary mode', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useAuthStore } = await import('@/stores/auth')
  const auth = useAuthStore()
  auth.setToken('t')
  auth.setUser({ id: 'u1', username: 'u', email: 'u@example.com', name: 'U' })

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()
  ui.setMode('secretary')

  const { default: App } = await import('@/App.vue')
  const wrapper = shallowMount(App, {
    global: { plugins: [pinia] },
  })

  expect(wrapper.findComponent({ name: 'Sidebar' }).exists()).toBe(false)
})

it('shows global Sidebar in full mode', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useAuthStore } = await import('@/stores/auth')
  const auth = useAuthStore()
  auth.setToken('t')
  auth.setUser({ id: 'u1', username: 'u', email: 'u@example.com', name: 'U' })

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()
  ui.setMode('full')

  const { default: App } = await import('@/App.vue')
  const wrapper = shallowMount(App, {
    global: { plugins: [pinia] },
  })

  expect(wrapper.findComponent({ name: 'Sidebar' }).exists()).toBe(true)
})

it('shows a low-noise full-mode gate on full pages in secretary mode', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useAuthStore } = await import('@/stores/auth')
  const auth = useAuthStore()
  auth.setToken('t')
  auth.setUser({ id: 'u1', username: 'u', email: 'u@example.com', name: 'U' })

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()
  ui.setMode('secretary')

  const { default: App } = await import('@/App.vue')
  const wrapper = shallowMount(App, {
    global: { plugins: [pinia] },
  })

  const gate = wrapper.find('[data-testid="full-mode-gate"]')
  expect(gate.exists()).toBe(true)

  await wrapper.get('[data-testid="full-mode-gate-enter"]').trigger('click')

  expect(ui.mode).toBe('full')
  expect(wrapper.findComponent({ name: 'Sidebar' }).exists()).toBe(true)
})
