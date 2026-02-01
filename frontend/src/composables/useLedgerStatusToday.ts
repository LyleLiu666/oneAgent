import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getLedgerStatusToday, type LedgerStatusToday } from '@/api/client'

export type LedgerStatusTodayPollingOptions = {
  enabled?: boolean
  pollIntervalMs?: number
}

export function useLedgerStatusToday(options: LedgerStatusTodayPollingOptions = {}) {
  const status = ref<LedgerStatusToday | null>(null)
  const loading = ref(false)

  const refresh = async () => {
    if (loading.value) return
    loading.value = true
    try {
      status.value = await getLedgerStatusToday()
    } catch {
      // keep last known
    } finally {
      loading.value = false
    }
  }

  const sopProposedCount = computed(() => Number(status.value?.sop_proposed_count || 0))

  const enabled = options.enabled ?? true
  const pollIntervalMs = options.pollIntervalMs ?? 30_000

  let timer: number | undefined
  onMounted(() => {
    if (!enabled) return
    void refresh()
    if (pollIntervalMs > 0) {
      timer = window.setInterval(() => {
        void refresh()
      }, pollIntervalMs)
    }
  })

  onUnmounted(() => {
    if (timer != null) {
      window.clearInterval(timer)
      timer = undefined
    }
  })

  return {
    status,
    sopProposedCount,
    loading,
    refresh,
  }
}

