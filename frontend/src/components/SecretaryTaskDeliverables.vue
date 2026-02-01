<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import {
  getTaskAttemptArtifact,
  listTasks,
  resumeTask,
  type Task,
  type TaskAttempt,
  type TaskAttemptArtifactContent,
} from '@/api/client'
import ErrorBanner from '@/components/ErrorBanner.vue'
import { useUIStore } from '@/stores/ui'

type DeliverableArtifact = {
  kind: string
  label: string
}

type DeliverableCard = {
  task: Task
  attempt: TaskAttempt
  artifacts: DeliverableArtifact[]
}

type RecoveryCard = {
  task: Task
  attempt: TaskAttempt
}

const props = defineProps<{
  workspace?: string
  pollIntervalMs?: number
  maxCards?: number
  maxRecoveryCards?: number
}>()

type TaskCompletedEvent = {
  taskId: string
  attemptId: string
  title: string
  status: string
}

const emit = defineEmits<{
  (e: 'task-completed', payload: TaskCompletedEvent): void
}>()

const ui = useUIStore()
const router = useRouter()

const pollIntervalMs = computed(() => (typeof props.pollIntervalMs === 'number' ? props.pollIntervalMs : 10_000))
const maxCards = computed(() => (typeof props.maxCards === 'number' ? props.maxCards : 3))
const maxRecoveryCards = computed(() => (typeof props.maxRecoveryCards === 'number' ? props.maxRecoveryCards : 2))

const loading = ref(false)
const error = ref<string>('')
const tasks = ref<Task[]>([])
const hasTaskBaseline = ref(false)
const lastLatestStatusByTaskID = ref<Record<string, string>>({})

let pollTimer: number | undefined

const normalizeWorkspace = (ws: any) => String(ws || '').trim()

const isTerminalAttempt = (a: TaskAttempt) => !['queued', 'running'].includes(String(a?.status || ''))
const isSucceededAttempt = (a: TaskAttempt) => String(a?.status || '') === 'succeeded'
const isTerminalStatus = (status: string) => status !== '' && !['queued', 'running'].includes(status)

const isNeedsAttentionAttempt = (a: TaskAttempt) => {
  const s = String(a?.status || '')
  return s === 'failed' || s === 'limit_exceeded' || s === 'timed_out' || s === 'interrupted'
}

const getLatestAttempt = (t: Task): TaskAttempt | null => {
  if (!t || !Array.isArray(t.attempts) || t.attempts.length === 0) return null
  return t.attempts[t.attempts.length - 1] || null
}

const cardArtifactsForAttempt = (a: TaskAttempt): DeliverableArtifact[] => {
  const items: DeliverableArtifact[] = []

  const findingsPath = String(a.findings_path || '').trim()
  if (findingsPath) items.push({ kind: 'findings', label: 'findings' })

  // Diff is often present even if the attempt does not explicitly store the path (server may have a default).
  if (isTerminalAttempt(a)) items.push({ kind: 'diff_patch', label: 'diff' })

  const testReportPath = String(a.test_report_path || '').trim()
  if (testReportPath) items.push({ kind: 'test_report', label: '测试报告' })

  const tracePath = String(a.trace_log_path || '').trim()
  if (tracePath) items.push({ kind: 'trace', label: 'trace' })

  return items
}

const deliverableCards = computed<DeliverableCard[]>(() => {
  const normalized = normalizeWorkspace(props.workspace)
  const filtered = tasks.value.filter((t) => {
    if (!normalized) return true
    return normalizeWorkspace((t as any)?.workspace) === normalized
  })

  const mapped: DeliverableCard[] = []
  for (const t of filtered) {
    const a = getLatestAttempt(t)
    if (!a || !isTerminalAttempt(a)) continue
    const artifacts = cardArtifactsForAttempt(a)
    if (artifacts.length === 0) continue
    mapped.push({ task: t, attempt: a, artifacts })
  }

  mapped.sort((a, b) => {
    const ta = Date.parse(a.attempt.finished_at || a.task.updated_at || a.task.created_at || '') || 0
    const tb = Date.parse(b.attempt.finished_at || b.task.updated_at || b.task.created_at || '') || 0
    return tb - ta
  })

  return mapped.slice(0, Math.max(0, maxCards.value))
})

const recoveryCards = computed<RecoveryCard[]>(() => {
  const normalized = normalizeWorkspace(props.workspace)
  const filtered = tasks.value.filter((t) => {
    if (!normalized) return true
    return normalizeWorkspace((t as any)?.workspace) === normalized
  })

  const mapped: RecoveryCard[] = []
  for (const t of filtered) {
    const a = getLatestAttempt(t)
    if (!a || !isTerminalAttempt(a)) continue
    if (isSucceededAttempt(a)) continue
    if (!isNeedsAttentionAttempt(a)) continue
    mapped.push({ task: t, attempt: a })
  }

  mapped.sort((a, b) => {
    const ta = Date.parse(a.attempt.finished_at || a.task.updated_at || a.task.created_at || '') || 0
    const tb = Date.parse(b.attempt.finished_at || b.task.updated_at || b.task.created_at || '') || 0
    return tb - ta
  })

  return mapped.slice(0, Math.max(0, maxRecoveryCards.value))
})

const refresh = async () => {
  const ws = normalizeWorkspace(props.workspace)
  loading.value = true
  error.value = ''

  try {
    const res = await listTasks(ws || undefined)
    const nextTasks = Array.isArray(res) ? res : []
    const nextLatestStatusByTaskID: Record<string, string> = {}

    for (const t of nextTasks) {
      const taskID = String((t as any)?.id || '').trim()
      if (!taskID) continue
      const latest = getLatestAttempt(t)
      nextLatestStatusByTaskID[taskID] = String(latest?.status || '').trim()
    }

    if (hasTaskBaseline.value) {
      const prev = lastLatestStatusByTaskID.value

      for (const t of nextTasks) {
        const taskID = String((t as any)?.id || '').trim()
        if (!taskID) continue
        const latest = getLatestAttempt(t)
        if (!latest) continue

        const prevStatus = String(prev[taskID] || '').trim()
        const nextStatus = String(latest.status || '').trim()
        if (!prevStatus) continue
        if (!['queued', 'running'].includes(prevStatus)) continue
        if (!isTerminalStatus(nextStatus)) continue

        emit('task-completed', {
          taskId: taskID,
          attemptId: String(latest.id || '').trim(),
          title: String((t as any)?.title || '').trim(),
          status: nextStatus,
        })
      }
    } else {
      hasTaskBaseline.value = true
    }

    tasks.value = nextTasks
    lastLatestStatusByTaskID.value = nextLatestStatusByTaskID
  } catch (e: any) {
    const msg = e?.data?.error || e?.message || 'Failed to load tasks.'
    error.value = String(msg)
    tasks.value = []
  } finally {
    loading.value = false
  }
}

const recoverySubmittingTaskId = ref<string>('')
const recoveryError = ref<string>('')

const onResume = async (taskId: string) => {
  const id = String(taskId || '').trim()
  if (!id) return
  if (recoverySubmittingTaskId.value) return

  recoverySubmittingTaskId.value = id
  recoveryError.value = ''
  try {
    await resumeTask(id)
    await refresh()
  } catch (e: any) {
    const msg = e?.data?.error || e?.message || 'Failed to resume task.'
    recoveryError.value = String(msg)
  } finally {
    recoverySubmittingTaskId.value = ''
  }
}

const onTroubleshoot = async () => {
  ui.setMode('full')
  await router.push('/tasks')
}

const stopPolling = () => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

const startPolling = () => {
  stopPolling()
  const ms = pollIntervalMs.value
  if (ms <= 0) return
  pollTimer = window.setInterval(() => {
    refresh()
  }, ms)
}

// Artifact preview modal (best-effort).
const artifactModalOpen = ref(false)
const artifactModalLoading = ref(false)
const artifactModalError = ref<string>('')
const artifactModalLabel = ref('')
const artifactModalPath = ref('')
const artifactModalContent = ref<TaskAttemptArtifactContent | null>(null)

let artifactRequestSeq = 0

const closeArtifactModal = () => {
  artifactModalOpen.value = false
  artifactModalLoading.value = false
  artifactModalError.value = ''
  artifactModalLabel.value = ''
  artifactModalPath.value = ''
  artifactModalContent.value = null
}

const openArtifactModal = async (taskId: string, attemptId: string, artifact: DeliverableArtifact) => {
  const tid = String(taskId || '').trim()
  const aid = String(attemptId || '').trim()
  if (!tid || !aid) return

  artifactModalOpen.value = true
  artifactModalLoading.value = true
  artifactModalError.value = ''
  artifactModalLabel.value = artifact.label
  artifactModalPath.value = ''
  artifactModalContent.value = null

  const seq = ++artifactRequestSeq

  try {
    const res = await getTaskAttemptArtifact(tid, aid, artifact.kind)
    if (seq !== artifactRequestSeq) return
    artifactModalPath.value = String(res?.path || '')
    artifactModalContent.value = res
  } catch (e: any) {
    if (seq !== artifactRequestSeq) return
    const msg = e?.data?.error || e?.message || 'Failed to load artifact.'
    artifactModalError.value = String(msg)
  } finally {
    if (seq === artifactRequestSeq) artifactModalLoading.value = false
  }
}

watch(
  () => props.workspace,
  async () => {
    await refresh()
  }
)

onMounted(async () => {
  await refresh()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div v-if="recoveryCards.length || deliverableCards.length || error" class="max-w-4xl mx-auto px-4 pb-3">
    <div
      v-if="recoveryCards.length"
      data-testid="secretary-task-recovery"
      class="mb-3 rounded-2xl border border-amber-500/20 bg-amber-500/5 backdrop-blur px-4 py-3"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="text-xs font-semibold text-amber-200 tracking-wide">需要处理</div>
        <div class="text-[11px] text-amber-200/70">最近 {{ recoveryCards.length }} 个任务</div>
      </div>

      <ErrorBanner v-if="recoveryError" :error="recoveryError" title="操作失败" class="mt-3" />

      <div class="mt-3 grid grid-cols-1 gap-3">
        <div
          v-for="card in recoveryCards"
          :key="card.task.id"
          class="rounded-2xl bg-surface-900/40 p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-surface-100 truncate">{{ card.task.title }}</div>
              <div class="mt-1 text-xs text-surface-500 truncate">
                status={{ card.attempt.status }} · attempt={{ card.attempt.id.slice(0, 8) }}
              </div>
            </div>
          </div>

          <div class="mt-3 space-y-2">
            <div
              v-if="card.attempt.summary"
              class="text-xs text-surface-200 whitespace-pre-wrap"
            >
              {{ card.attempt.summary }}
            </div>
            <div
              v-else-if="card.attempt.error"
              class="text-xs text-surface-200 whitespace-pre-wrap"
            >
              {{ card.attempt.error }}
            </div>
            <div
              v-if="card.attempt.observer?.next_steps"
              class="rounded-xl border border-surface-800/60 bg-surface-950/40 p-3 text-xs text-surface-300 whitespace-pre-wrap"
            >
              {{ card.attempt.observer.next_steps }}
            </div>
          </div>

          <div class="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              data-testid="secretary-task-recovery-resume"
              class="rounded-full border border-amber-500/30 bg-amber-500/10 px-3 py-1 text-xs text-amber-100 hover:bg-amber-500/15 disabled:opacity-60 disabled:cursor-not-allowed"
              :disabled="Boolean(recoverySubmittingTaskId)"
              @click="onResume(card.task.id)"
            >
              继续
            </button>
            <button
              type="button"
              data-testid="secretary-task-recovery-troubleshoot"
              class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
              @click="onTroubleshoot"
            >
              排障
            </button>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="deliverableCards.length"
      data-testid="secretary-task-deliverables"
      class="rounded-2xl border border-surface-800/60 bg-surface-900/30 backdrop-blur px-4 py-3"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="text-xs font-semibold text-surface-200 tracking-wide">交付</div>
        <div class="text-[11px] text-surface-500">最近 {{ deliverableCards.length }} 个任务</div>
      </div>

      <div class="mt-3 grid grid-cols-1 gap-3">
        <div
          v-for="card in deliverableCards"
          :key="card.task.id"
          data-testid="secretary-task-deliverable-card"
          class="rounded-2xl bg-surface-800/30 p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-surface-100 truncate">{{ card.task.title }}</div>
              <div class="mt-1 text-xs text-surface-500 truncate">
                status={{ card.attempt.status }} · attempt={{ card.attempt.id.slice(0, 8) }}
              </div>
            </div>
          </div>

          <div v-if="card.attempt.summary" class="mt-3 text-xs text-surface-300 whitespace-pre-wrap">
            {{ card.attempt.summary }}
          </div>

          <div class="mt-3 flex flex-wrap gap-2">
            <button
              v-for="a in card.artifacts"
              :key="a.kind"
              type="button"
              class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
              :data-testid="a.kind === 'findings' ? 'deliverable-open-findings' : undefined"
              @click="openArtifactModal(card.task.id, card.attempt.id, a)"
            >
              {{ a.label }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <ErrorBanner v-else-if="error" :error="error" title="加载交付失败" />

    <div
      v-if="artifactModalOpen"
      data-testid="secretary-task-artifact-modal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4"
    >
      <div class="absolute inset-0 bg-black/70" @click="closeArtifactModal"></div>
      <div class="relative w-full max-w-5xl rounded-3xl bg-surface-900 shadow-2xl overflow-hidden">
        <div class="px-5 py-4 bg-surface-800/50 flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-surface-100 truncate">{{ artifactModalLabel }}</div>
            <div class="text-xs text-surface-500 font-mono break-all mt-0.5">{{ artifactModalPath }}</div>
          </div>
          <button
            type="button"
            data-testid="secretary-task-artifact-modal-close"
            class="px-3 py-1.5 rounded-lg text-sm font-medium bg-surface-700/50 text-surface-300 hover:bg-surface-600/50 transition-colors"
            @click="closeArtifactModal"
          >
            关闭
          </button>
        </div>

        <div class="p-5">
          <div v-if="artifactModalLoading" class="text-sm text-surface-500 py-8 text-center">
            加载中…
          </div>
          <ErrorBanner v-else-if="artifactModalError" :error="artifactModalError" title="加载失败" />
          <div v-else class="space-y-3">
            <div v-if="artifactModalContent?.truncated" class="text-xs text-amber-400 px-1">
              内容已截断（仅展示前 512KB）
            </div>
            <pre
              class="max-h-[65vh] overflow-auto rounded-2xl bg-surface-950/60 p-4 text-[12px] text-surface-200 whitespace-pre font-mono leading-relaxed"
            >{{ artifactModalContent?.content }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
