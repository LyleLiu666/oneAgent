import { computed, onMounted, onUnmounted, ref } from 'vue'
import { listTasks, type Task, type AttemptStatus } from '@/api/client'

export type TaskQueueStatusPollingOptions = {
  enabled?: boolean
  pollIntervalMs?: number
  workspace?: string
}

const isActiveAttemptStatus = (status: AttemptStatus | undefined): boolean =>
  status === 'queued' || status === 'running'

const latestAttemptStatus = (task: Task): AttemptStatus | undefined => {
  const attempts = Array.isArray(task.attempts) ? task.attempts : []
  return attempts.length > 0 ? attempts[attempts.length - 1]?.status : undefined
}

export function useTaskQueueStatus(options: TaskQueueStatusPollingOptions = {}) {
  const tasks = ref<Task[]>([])
  const loading = ref(false)

  const refresh = async () => {
    if (loading.value) return
    loading.value = true
    try {
      tasks.value = await listTasks(options.workspace)
    } catch {
      // keep last known
    } finally {
      loading.value = false
    }
  }

  const activeTaskCount = computed(() => tasks.value.filter((t) => isActiveAttemptStatus(latestAttemptStatus(t))).length)

  const enabled = options.enabled ?? true
  const pollIntervalMs = options.pollIntervalMs ?? 10_000

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

  return { tasks, activeTaskCount, loading, refresh }
}

