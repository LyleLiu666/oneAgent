<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import {
  getTaskAttemptArtifact,
  listTaskAttemptFiles,
  readTaskAttemptFileSnapshot,
  listTasks,
  resumeTask,
  type Task,
  type TaskAttempt,
  type TaskAttemptArtifactContent,
  type TaskAttemptFileEntry,
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
  artifacts: DeliverableArtifact[]
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

type TaskNeedsAttentionEvent = {
  taskId: string
  attemptId: string
  title: string
  status: string
  summary?: string
  error?: string
  observer?: TaskAttempt['observer']
  findings_path?: string
  trace_log_path?: string
  diff_patch_path?: string
  test_report_path?: string
}

type RecoveryActionEvent = {
  action: 'resume' | 'dismiss'
  taskId: string
  attemptId: string
  title?: string
  status?: string
}

const emit = defineEmits<{
  (e: 'task-completed', payload: TaskCompletedEvent): void
  (e: 'task-needs-attention', payload: TaskNeedsAttentionEvent): void
  (e: 'recovery-snapshot', payload: TaskNeedsAttentionEvent[]): void
  (e: 'recovery-focus', payload: TaskNeedsAttentionEvent): void
  (e: 'recovery-action', payload: RecoveryActionEvent): void
}>()

const ui = useUIStore()
const router = useRouter()

const STORAGE_RECOVERY_COLLAPSED = 'oneagent-secretary-recovery-collapsed'
const STORAGE_DELIVERABLES_COLLAPSED = 'oneagent-secretary-deliverables-collapsed'
const STORAGE_RECOVERY_DETAILS = 'oneagent-secretary-recovery-details-v1'
const STORAGE_DELIVERABLE_DETAILS = 'oneagent-secretary-deliverables-details-v1'
const STORAGE_DISMISSED_ATTEMPTS = 'oneagent-secretary-dismissed-attempts-v1'

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

const loadBool = (key: string): boolean => {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

const saveBool = (key: string, value: boolean) => {
  try {
    localStorage.setItem(key, value ? '1' : '0')
  } catch {
    // ignore
  }
}

const normalizeWorkspaceStorageKey = (ws: string) => {
  const v = normalizeWorkspace(ws)
  return v ? v : 'no-workspace'
}

const makeDismissKey = (taskId: string, attemptId: string) => `${taskId}:${attemptId}`

const loadDismissedKeys = (ws: string): Set<string> => {
  try {
    const raw = localStorage.getItem(STORAGE_DISMISSED_ATTEMPTS)
    const parsed = raw ? JSON.parse(raw) : {}
    const arr = parsed?.[normalizeWorkspaceStorageKey(ws)]
    if (!Array.isArray(arr)) return new Set<string>()
    return new Set(arr.map((v: any) => String(v || '').trim()).filter(Boolean))
  } catch {
    return new Set<string>()
  }
}

const saveDismissedKeys = (ws: string, keys: Set<string>) => {
  try {
    const raw = localStorage.getItem(STORAGE_DISMISSED_ATTEMPTS)
    const parsed = raw ? JSON.parse(raw) : {}
    parsed[normalizeWorkspaceStorageKey(ws)] = Array.from(keys).slice(-200)
    localStorage.setItem(STORAGE_DISMISSED_ATTEMPTS, JSON.stringify(parsed))
  } catch {
    // ignore
  }
}

const recoveryCollapsed = ref(loadBool(STORAGE_RECOVERY_COLLAPSED))
const deliverablesCollapsed = ref(loadBool(STORAGE_DELIVERABLES_COLLAPSED))
const recoveryDetailsExpanded = ref(loadBool(STORAGE_RECOVERY_DETAILS))
const deliverablesDetailsExpanded = ref(loadBool(STORAGE_DELIVERABLE_DETAILS))
const dismissedKeys = ref<Set<string>>(loadDismissedKeys(props.workspace || ''))

watch(recoveryCollapsed, (v) => saveBool(STORAGE_RECOVERY_COLLAPSED, v))
watch(deliverablesCollapsed, (v) => saveBool(STORAGE_DELIVERABLES_COLLAPSED, v))
watch(recoveryDetailsExpanded, (v) => saveBool(STORAGE_RECOVERY_DETAILS, v))
watch(deliverablesDetailsExpanded, (v) => saveBool(STORAGE_DELIVERABLE_DETAILS, v))

const dismissAttempt = (taskId: string, attemptId: string) => {
  const tid = String(taskId || '').trim()
  const aid = String(attemptId || '').trim()
  if (!tid || !aid) return

  const ws = normalizeWorkspace(props.workspace)
  const next = new Set(dismissedKeys.value)
  next.add(makeDismissKey(tid, aid))
  dismissedKeys.value = next
  saveDismissedKeys(ws, next)
}

const isDismissedAttempt = (taskId: string, attemptId: string) => {
  const tid = String(taskId || '').trim()
  const aid = String(attemptId || '').trim()
  if (!tid || !aid) return false
  return dismissedKeys.value.has(makeDismissKey(tid, aid))
}

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

const toNeedsAttentionEvent = (task: Task, attempt: TaskAttempt): TaskNeedsAttentionEvent => ({
  taskId: String((task as any)?.id || '').trim(),
  attemptId: String(attempt?.id || '').trim(),
  title: String((task as any)?.title || '').trim(),
  status: String(attempt?.status || '').trim(),
  summary: typeof attempt?.summary === 'string' ? attempt.summary : undefined,
  error: typeof attempt?.error === 'string' ? attempt.error : undefined,
  observer: attempt?.observer,
  findings_path: typeof attempt?.findings_path === 'string' ? attempt.findings_path : undefined,
  trace_log_path: typeof attempt?.trace_log_path === 'string' ? attempt.trace_log_path : undefined,
  diff_patch_path: typeof attempt?.diff_patch_path === 'string' ? attempt.diff_patch_path : undefined,
  test_report_path: typeof attempt?.test_report_path === 'string' ? attempt.test_report_path : undefined,
})

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
    if (!isSucceededAttempt(a) && isNeedsAttentionAttempt(a)) continue
    if (isDismissedAttempt((t as any)?.id || '', a.id || '')) continue
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
    if (isDismissedAttempt((t as any)?.id || '', a.id || '')) continue
    mapped.push({ task: t, attempt: a, artifacts: cardArtifactsForAttempt(a) })
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
    const hadBaseline = hasTaskBaseline.value

    for (const t of nextTasks) {
      const taskID = String((t as any)?.id || '').trim()
      if (!taskID) continue
      const latest = getLatestAttempt(t)
      nextLatestStatusByTaskID[taskID] = String(latest?.status || '').trim()
    }

    if (hadBaseline) {
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

        if (isNeedsAttentionAttempt(latest) && !isDismissedAttempt(taskID, String(latest.id || '').trim())) {
          emit('task-needs-attention', toNeedsAttentionEvent(t, latest))
        }
      }
    } else {
      hasTaskBaseline.value = true
    }

    tasks.value = nextTasks
    lastLatestStatusByTaskID.value = nextLatestStatusByTaskID

    // Initial snapshot: surface existing needs-attention items once per mount (best-effort).
    if (!hadBaseline) {
      const items: TaskNeedsAttentionEvent[] = []
      for (const t of nextTasks) {
        const a = getLatestAttempt(t)
        if (!a || !isTerminalAttempt(a)) continue
        if (isSucceededAttempt(a)) continue
        if (!isNeedsAttentionAttempt(a)) continue
        const taskID = String((t as any)?.id || '').trim()
        if (!taskID) continue
        const attemptID = String(a.id || '').trim()
        if (!attemptID) continue
        if (isDismissedAttempt(taskID, attemptID)) continue
        items.push(toNeedsAttentionEvent(t, a))
      }
      if (items.length > 0) emit('recovery-snapshot', items)
    }
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

const onFocus = (card: RecoveryCard) => {
  if (!card) return
  emit('recovery-focus', toNeedsAttentionEvent(card.task, card.attempt))
}

const onResume = async (card: RecoveryCard) => {
  const id = String(card?.task?.id || '').trim()
  const attemptID = String(card?.attempt?.id || '').trim()
  if (!id || !attemptID) return
  if (recoverySubmittingTaskId.value) return

  recoverySubmittingTaskId.value = id
  recoveryError.value = ''
  try {
    await resumeTask(id, { source: 'secretary-recovery' })
    emit('recovery-action', {
      action: 'resume',
      taskId: id,
      attemptId: attemptID,
      title: String(card?.task?.title || '').trim() || undefined,
      status: String(card?.attempt?.status || '').trim() || undefined,
    })
    await refresh()
  } catch (e: any) {
    const msg = e?.data?.error || e?.message || 'Failed to resume task.'
    recoveryError.value = String(msg)
  } finally {
    recoverySubmittingTaskId.value = ''
  }
}

const onDismiss = (card: RecoveryCard) => {
  const id = String(card?.task?.id || '').trim()
  const attemptID = String(card?.attempt?.id || '').trim()
  if (!id || !attemptID) return
  dismissAttempt(id, attemptID)
  emit('recovery-action', {
    action: 'dismiss',
    taskId: id,
    attemptId: attemptID,
    title: String(card?.task?.title || '').trim() || undefined,
    status: String(card?.attempt?.status || '').trim() || undefined,
  })
}

const onTroubleshoot = async (card?: RecoveryCard) => {
  const tid = String(card?.task?.id || '').trim()
  const aid = String(card?.attempt?.id || '').trim()

  if (tid && aid && Array.isArray(card?.artifacts)) {
    const trace = card.artifacts.find((a) => String(a?.kind || '').trim() === 'trace')
    if (trace) {
      await openArtifactModal(tid, aid, trace)
      return
    }
  }

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

// Attempt file browser (snapshotted file contents).
const fileModalOpen = ref(false)
const fileModalLoading = ref(false)
const fileModalError = ref<string>('')
const fileModalTitle = ref('')
const fileModalWorkspace = ref('')
const fileModalTaskId = ref('')
const fileModalAttemptId = ref('')
const fileModalFiles = ref<TaskAttemptFileEntry[]>([])
const fileModalOmitted = ref(0)
const fileModalSelectedPath = ref('')
const fileModalSelectedAvailable = ref(false)
const fileModalContentLoading = ref(false)
const fileModalContentError = ref('')
const fileModalContent = ref<TaskAttemptArtifactContent | null>(null)

let fileModalRequestSeq = 0
let fileModalContentSeq = 0

const closeFileModal = () => {
  fileModalOpen.value = false
  fileModalLoading.value = false
  fileModalError.value = ''
  fileModalTitle.value = ''
  fileModalWorkspace.value = ''
  fileModalTaskId.value = ''
  fileModalAttemptId.value = ''
  fileModalFiles.value = []
  fileModalOmitted.value = 0
  fileModalSelectedPath.value = ''
  fileModalSelectedAvailable.value = false
  fileModalContentLoading.value = false
  fileModalContentError.value = ''
  fileModalContent.value = null
}

const selectFileInModal = async (path: string) => {
  const tid = String(fileModalTaskId.value || '').trim()
  const aid = String(fileModalAttemptId.value || '').trim()
  const rel = String(path || '').trim()
  if (!tid || !aid || !rel) return

  const entry = fileModalFiles.value.find((f) => String((f as any)?.path || '').trim() === rel)
  const available = Boolean((entry as any)?.available)
  fileModalSelectedPath.value = rel
  fileModalSelectedAvailable.value = available
  fileModalContentError.value = ''
  fileModalContent.value = null
  if (!available) return

  fileModalContentLoading.value = true
  const seq = ++fileModalContentSeq
  try {
    const res = await readTaskAttemptFileSnapshot(tid, aid, rel)
    if (seq !== fileModalContentSeq) return
    fileModalContent.value = res
  } catch (e: any) {
    if (seq !== fileModalContentSeq) return
    const msg = e?.data?.error || e?.message || 'Failed to load file.'
    fileModalContentError.value = String(msg)
  } finally {
    if (seq === fileModalContentSeq) fileModalContentLoading.value = false
  }
}

const openFileModal = async (card: { task: Task; attempt: TaskAttempt }) => {
  const tid = String((card as any)?.task?.id || '').trim()
  const aid = String((card as any)?.attempt?.id || '').trim()
  if (!tid || !aid) return

  fileModalOpen.value = true
  fileModalLoading.value = true
  fileModalError.value = ''
  fileModalTitle.value = String((card as any)?.task?.title || '').trim() || '文件'
  fileModalWorkspace.value = String((card as any)?.task?.workspace || '').trim()
  fileModalTaskId.value = tid
  fileModalAttemptId.value = aid
  fileModalFiles.value = []
  fileModalOmitted.value = 0
  fileModalSelectedPath.value = ''
  fileModalSelectedAvailable.value = false
  fileModalContentLoading.value = false
  fileModalContentError.value = ''
  fileModalContent.value = null

  const seq = ++fileModalRequestSeq
  try {
    const res = await listTaskAttemptFiles(tid, aid)
    if (seq !== fileModalRequestSeq) return
    const files = Array.isArray((res as any)?.files) ? ((res as any).files as TaskAttemptFileEntry[]) : []
    fileModalFiles.value = files
    fileModalOmitted.value = Number((res as any)?.omitted || 0) || 0
    const first = files.find((f) => Boolean((f as any)?.available)) || files[0]
    if (first && String((first as any)?.path || '').trim()) {
      await selectFileInModal(String((first as any).path))
    }
  } catch (e: any) {
    if (seq !== fileModalRequestSeq) return
    const msg = e?.data?.error || e?.message || 'Failed to load files.'
    fileModalError.value = String(msg)
  } finally {
    if (seq === fileModalRequestSeq) fileModalLoading.value = false
  }
}

watch(
  () => props.workspace,
  async () => {
    dismissedKeys.value = loadDismissedKeys(props.workspace || '')
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
        <div class="text-xs font-semibold text-amber-800 dark:text-amber-200 tracking-wide">需要处理</div>
        <div class="flex items-center gap-2">
          <div class="text-[11px] text-amber-700/80 dark:text-amber-200/70">最近 {{ recoveryCards.length }} 个任务</div>
          <button
            type="button"
            data-testid="secretary-task-recovery-more"
            class="rounded-full border border-surface-700/40 bg-surface-900/40 px-2 py-0.5 text-[11px] text-surface-200 hover:bg-surface-800/50"
            @click="recoveryDetailsExpanded = !recoveryDetailsExpanded"
          >
            {{ recoveryDetailsExpanded ? '简洁' : '更多' }}
          </button>
          <button
            type="button"
            data-testid="secretary-task-recovery-toggle"
            class="rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 text-[11px] text-amber-900 dark:text-amber-100 hover:bg-amber-500/15"
            @click="recoveryCollapsed = !recoveryCollapsed"
          >
            {{ recoveryCollapsed ? '展开' : '收起' }}
          </button>
        </div>
      </div>

      <div v-if="!recoveryCollapsed">
        <ErrorBanner v-if="recoveryError" :error="recoveryError" title="操作失败" class="mt-3" />

        <div class="mt-3 grid grid-cols-1 gap-3">
          <div
            v-for="card in recoveryCards"
            :key="card.task.id"
            class="rounded-2xl bg-surface-900/40 p-4 cursor-pointer hover:bg-surface-900/55"
            @click="onFocus(card)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="text-sm font-semibold text-surface-100 truncate">{{ card.task.title }}</div>
                <div class="mt-1 text-xs text-surface-500 truncate">
                  status={{ card.attempt.status }} · attempt={{ card.attempt.id.slice(0, 8) }}
                </div>
              </div>
            </div>

            <div v-if="recoveryDetailsExpanded" class="mt-3 space-y-2">
              <div
                v-if="card.attempt.error"
                class="text-xs text-surface-200 whitespace-pre-wrap"
              >
                {{ card.attempt.error }}
              </div>
              <div
                v-else-if="card.attempt.summary"
                class="text-xs text-surface-200 whitespace-pre-wrap"
              >
                {{ card.attempt.summary }}
              </div>
              <div
                v-if="card.attempt.observer?.next_steps"
                class="rounded-xl border border-surface-800/60 bg-surface-950/40 p-3 text-xs text-surface-300 whitespace-pre-wrap"
              >
                {{ card.attempt.observer.next_steps }}
              </div>
            </div>

            <div v-if="recoveryDetailsExpanded && card.artifacts.length" class="mt-3 flex flex-wrap gap-2">
              <button
                type="button"
                data-testid="recovery-open-files"
                class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
                title="浏览变更文件内容（快照）"
                @click.stop="openFileModal(card)"
              >
                文件
              </button>
              <button
                v-for="a in card.artifacts"
                :key="a.kind"
                type="button"
                class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
                :data-testid="
                  a.kind === 'findings'
                    ? 'recovery-open-findings'
                    : a.kind === 'trace'
                      ? 'recovery-open-trace'
                      : a.kind === 'diff_patch'
                        ? 'recovery-open-diff'
                        : undefined
                "
                @click.stop="openArtifactModal(card.task.id, card.attempt.id, a)"
              >
                {{ a.label }}
              </button>
            </div>

            <div v-if="recoveryDetailsExpanded" class="mt-3 flex flex-wrap gap-2">
              <button
                type="button"
                data-testid="secretary-task-recovery-resume"
                class="rounded-full border border-amber-500/30 bg-amber-500/10 px-3 py-1 text-xs text-amber-900 dark:text-amber-100 hover:bg-amber-500/15 disabled:opacity-60 disabled:cursor-not-allowed"
                :disabled="Boolean(recoverySubmittingTaskId)"
                @click.stop="onResume(card)"
              >
                继续
              </button>
              <button
                type="button"
                data-testid="secretary-task-recovery-troubleshoot"
                class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
                @click.stop="onTroubleshoot(card)"
              >
                排障
              </button>
              <button
                type="button"
                data-testid="secretary-task-recovery-dismiss"
                class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
                title="暂时隐藏（仍可在任务工作台查看）"
                @click.stop="onDismiss(card)"
              >
                稍后
              </button>
            </div>
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
        <div class="flex items-center gap-2">
          <div class="text-[11px] text-surface-500">最近 {{ deliverableCards.length }} 个任务</div>
          <button
            type="button"
            data-testid="secretary-task-deliverables-more"
            class="rounded-full border border-surface-700/40 bg-surface-900/40 px-2 py-0.5 text-[11px] text-surface-200 hover:bg-surface-800/50"
            @click="deliverablesDetailsExpanded = !deliverablesDetailsExpanded"
          >
            {{ deliverablesDetailsExpanded ? '简洁' : '更多' }}
          </button>
          <button
            type="button"
            data-testid="secretary-task-deliverables-toggle"
            class="rounded-full border border-surface-700/40 bg-surface-900/40 px-2 py-0.5 text-[11px] text-surface-200 hover:bg-surface-800/50"
            @click="deliverablesCollapsed = !deliverablesCollapsed"
          >
            {{ deliverablesCollapsed ? '展开' : '收起' }}
          </button>
        </div>
      </div>

      <div v-if="!deliverablesCollapsed" class="mt-3 grid grid-cols-1 gap-3">
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
            <button
              type="button"
              data-testid="secretary-task-deliverable-dismiss"
              class="shrink-0 rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
              title="隐藏该交付卡片（仍可在任务工作台查看）"
              @click="dismissAttempt(card.task.id, card.attempt.id)"
            >
              隐藏
            </button>
          </div>

          <div v-if="deliverablesDetailsExpanded && card.attempt.summary" class="mt-3 text-xs text-surface-300 whitespace-pre-wrap">
            {{ card.attempt.summary }}
          </div>

          <div v-if="deliverablesDetailsExpanded" class="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              data-testid="deliverable-open-files"
              class="rounded-full border border-surface-700/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50"
              title="浏览变更文件内容（快照）"
              @click="openFileModal(card)"
            >
              文件
            </button>
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

    <div
      v-if="fileModalOpen"
      data-testid="secretary-task-file-modal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4"
    >
      <div class="absolute inset-0 bg-black/70" @click="closeFileModal"></div>
      <div class="relative w-full max-w-6xl rounded-3xl bg-surface-900 shadow-2xl overflow-hidden">
        <div class="px-5 py-4 bg-surface-800/50 flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-surface-100 truncate">文件 · {{ fileModalTitle }}</div>
            <div v-if="fileModalWorkspace" class="text-xs text-surface-500 font-mono break-all mt-0.5">{{ fileModalWorkspace }}</div>
          </div>
          <button
            type="button"
            data-testid="secretary-task-file-modal-close"
            class="px-3 py-1.5 rounded-lg text-sm font-medium bg-surface-700/50 text-surface-300 hover:bg-surface-600/50 transition-colors"
            @click="closeFileModal"
          >
            关闭
          </button>
        </div>

        <div class="p-5">
          <div v-if="fileModalLoading" class="text-sm text-surface-500 py-8 text-center">
            加载中…
          </div>
          <ErrorBanner v-else-if="fileModalError" :error="fileModalError" title="加载失败" />
          <div v-else class="space-y-3">
            <div v-if="fileModalOmitted > 0" class="text-xs text-surface-500">
              仅展示前 {{ fileModalFiles.length }} 个文件，另有 {{ fileModalOmitted }} 个已省略。
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <div class="lg:col-span-1">
                <div v-if="fileModalFiles.length === 0" class="text-sm text-surface-500 italic py-10 text-center">
                  暂无可浏览文件（未检测到变更文件或未生成快照）
                </div>
                <div v-else class="max-h-[65vh] overflow-auto space-y-1 pr-1">
                  <button
                    v-for="f in fileModalFiles"
                    :key="f.path"
                    type="button"
                    class="w-full text-left rounded-xl border px-3 py-2 transition-colors"
                    :class="[
                      String(f.path) === fileModalSelectedPath
                        ? 'border-primary-500/40 bg-primary-500/10'
                        : 'border-surface-800/60 bg-surface-950/40 hover:bg-surface-900/55',
                    ]"
                    @click="selectFileInModal(f.path)"
                  >
                    <div class="text-xs text-surface-100 font-mono break-all">{{ f.path }}</div>
                    <div class="mt-0.5 text-[11px]" :class="f.available ? 'text-surface-500' : 'text-amber-400/90'">
                      {{ f.available ? (f.size_bytes ? `${f.size_bytes} bytes` : '可预览') : '无快照' }}
                    </div>
                  </button>
                </div>
              </div>

              <div class="lg:col-span-2">
                <div v-if="!fileModalSelectedPath" class="text-sm text-surface-500 italic py-10 text-center">
                  选择一个文件查看内容
                </div>
                <div v-else class="space-y-3">
                  <div class="text-xs text-surface-400 font-mono break-all">{{ fileModalSelectedPath }}</div>

                  <div v-if="!fileModalSelectedAvailable" class="text-sm text-surface-500 italic py-10 text-center">
                    该文件未生成快照（可能已被删除、是二进制文件，或超出限制）。
                  </div>
                  <div v-else>
                    <div v-if="fileModalContentLoading" class="text-sm text-surface-500 py-8 text-center">
                      加载中…
                    </div>
                    <ErrorBanner v-else-if="fileModalContentError" :error="fileModalContentError" title="加载失败" />
                    <div v-else>
                      <div v-if="fileModalContent?.truncated" class="text-xs text-amber-400 px-1 mb-2">
                        内容已截断（仅展示前 512KB）
                      </div>
                      <pre
                        class="max-h-[65vh] overflow-auto rounded-2xl bg-surface-950/60 p-4 text-[12px] text-surface-200 whitespace-pre font-mono leading-relaxed"
                      >{{ fileModalContent?.content }}</pre>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
