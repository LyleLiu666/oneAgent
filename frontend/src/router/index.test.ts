// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/composables/useAuth', () => ({
  ensureAuthModeLoaded: vi.fn(async () => {}),
  initAuth: vi.fn(async () => true),
  useAuth: () => ({
    authMode: { value: 'token' },
  }),
}))

vi.mock('@/views/Settings.vue', () => ({
  default: { name: 'SettingsView', template: '<div>settings</div>' },
}))

it('switches to full mode before entering full-mode routes', async () => {
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

  const router = (await import('@/router')).default
  await router.push('/settings')
  await router.isReady()

  expect(ui.mode).toBe('full')
})
