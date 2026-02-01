// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const makeLocalStorage = (initial: Record<string, string> = {}) => {
  const store = new Map<string, string>(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  }
}

it('loads ui_mode from JSON persisted state', async () => {
  vi.stubGlobal(
    'localStorage',
    makeLocalStorage({
      'oneagent-ui-mode': JSON.stringify({ mode: 'full' }),
    })
  )

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()

  expect(ui.mode).toBe('full')
})

it('migrates legacy chat ui key to ui_mode JSON', async () => {
  const localStorage = makeLocalStorage({
    'oneagent-chat-ui-mode': 'full',
  })
  vi.stubGlobal('localStorage', localStorage)

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()

  expect(ui.mode).toBe('full')
  expect(localStorage.getItem('oneagent-ui-mode')).toBe(JSON.stringify({ mode: 'full' }))
})

it('does not override existing ui_mode when legacy key differs', async () => {
  const localStorage = makeLocalStorage({
    'oneagent-ui-mode': JSON.stringify({ mode: 'secretary' }),
    'oneagent-chat-ui-mode': 'full',
  })
  vi.stubGlobal('localStorage', localStorage)

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()

  expect(ui.mode).toBe('secretary')
  expect(localStorage.getItem('oneagent-ui-mode')).toBe(JSON.stringify({ mode: 'secretary' }))
})

