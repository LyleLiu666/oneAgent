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

    // Best-effort migration from legacy key used by ChatBox before ui_mode became global.
    try {
      const current = normalizeMode(localStorage.getItem(UI_MODE_STORAGE_KEY))
      if (!current) {
        const legacy = normalizeMode(localStorage.getItem(LEGACY_CHAT_UI_MODE_KEY))
        if (legacy) {
          mode.value = legacy
          localStorage.setItem(UI_MODE_STORAGE_KEY, legacy)
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

