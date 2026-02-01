import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export type UIMode = 'secretary' | 'full'

const UI_MODE_STORAGE_KEY = 'oneagent-ui-mode'
const LEGACY_CHAT_UI_MODE_KEY = 'oneagent-chat-ui-mode'

const normalizeMode = (raw: unknown): UIMode | undefined => {
  const v = String(raw ?? '').trim().toLowerCase()
  if (v === 'secretary' || v === 'full') return v
  return undefined
}

const readPersistedMode = (): UIMode | undefined => {
  const raw = localStorage.getItem(UI_MODE_STORAGE_KEY)
  if (raw == null) return undefined

  // Back-compat: some versions may store the raw string.
  const direct = normalizeMode(raw)
  if (direct) return direct

  // pinia-plugin-persistedstate stores JSON by default.
  try {
    const parsed = JSON.parse(raw) as any
    const nested = normalizeMode(parsed?.mode)
    if (nested) return nested
  } catch {
    // ignore
  }

  return undefined
}

export const useUIStore = defineStore(
  'ui',
  () => {
    const mode = ref<UIMode>('secretary')

    const isSecretaryMode = computed(() => mode.value === 'secretary')
    const isFullMode = computed(() => mode.value === 'full')

    const setMode = (next: UIMode) => {
      mode.value = next
    }

    const toggleMode = () => {
      mode.value = mode.value === 'secretary' ? 'full' : 'secretary'
    }

    // Best-effort init + migration from legacy key used by ChatBox before ui_mode became global.
    try {
      const current = readPersistedMode()
      if (current) {
        mode.value = current
      } else {
        const legacy = normalizeMode(localStorage.getItem(LEGACY_CHAT_UI_MODE_KEY))
        if (legacy) {
          mode.value = legacy
          // Ensure persistedstate plugin can pick it up.
          localStorage.setItem(UI_MODE_STORAGE_KEY, JSON.stringify({ mode: legacy }))
        }
      }
    } catch {
      // ignore storage failures
    }

    return { mode, isSecretaryMode, isFullMode, setMode, toggleMode }
  },
  {
    // @ts-ignore
    persist: {
      key: UI_MODE_STORAGE_KEY,
      storage: localStorage,
      paths: ['mode'],
    },
  }
)
