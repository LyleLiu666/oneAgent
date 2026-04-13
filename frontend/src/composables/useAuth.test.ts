// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from 'vitest'
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

beforeEach(() => {
  vi.resetModules()
})

it('loads auth mode when ensureAuthModeLoaded is called', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())
  vi.stubGlobal('fetch', vi.fn(async () => {
    return new Response(JSON.stringify({ auth_mode: 'token' }), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
      },
    })
  }))

  const pinia = createPinia()
  setActivePinia(pinia)

  const { ensureAuthModeLoaded, useAuth } = await import('@/composables/useAuth')
  const auth = useAuth()

  await ensureAuthModeLoaded()

  await flushPromises()
  await flushPromises()

  expect(auth.authMode.value).toBe('token')
})
