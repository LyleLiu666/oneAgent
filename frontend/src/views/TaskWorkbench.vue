<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Folder, ListTodo, RefreshCw, RotateCcw, X, Plus } from 'lucide-vue-next'

import {
  cancelTask,
  createTask,
  getTask,
  getTaskEvents,
  listTasks,
  resumeTask,
  type Task,
  type TaskAttempt,
  type TaskEvent,
} from '@/api/client'
import {
  diffTaskUpdates,
  saveTaskSnapshotsToStorage,
  type TaskSnapshot,
  type TaskUpdate,
} from '@/lib/taskUpdates'

type WorkspaceSummary = {
  workspace: string
  total: number
  queued: number
  running: number
  failed: number
}

const STORAGE_KEY = 'oneagent-workspaces'
const TASK_SNAPSHOT_KEY = 'oneagent-task-snapshots'

const tasksLoading = ref(false)
const tasksError = ref('')
const tasks = ref<Task[]>([])

const notifyInitialized = ref(false)
const taskSnapshots = ref<Record<string, TaskSnapshot>>({})
const taskUpdates = ref<TaskUpdate[]>([])

const workspacesManual = ref<string[]>([])
const workspaceNew = ref('')
const workspaceSelected = ref('')

const selectedTaskId = ref('')
const selectedTask = ref<Task | null>(null)
const selectedEvents = ref<TaskEvent[]>([])
const selectedLoading = ref(false)
const selectedError = ref('')

const title = ref('')
const prompt = ref('')
const submitting = ref(false)

const latestAttempt = computed<TaskAttempt | null>(() => {
  const t = selectedTask.value
  if (!t || !Array.isArray(t.attempts) || t.attempts.length === 0) return null
  return t.attempts[t.attempts.length - 1] || null
})

const latestStatus = computed(() => latestAttempt.value?.status || '')

const canCancel = computed(() => latestStatus.value === 'queued' || latestStatus.value === 'running')
const canResume = computed(() => ['failed', 'timed_out', 'interrupted'].includes(latestStatus.value))

const normalizeWorkspace = (ws: string) => String(ws || '').trim()
const shortHash = (hash?: string) => (hash && hash.length >= 8 ? hash.slice(0, 8) : hash || '')

const loadManualWorkspaces = () => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const arr = raw ? JSON.parse(raw) : []
    workspacesManual.value = Array.isArray(arr) ? arr.map(normalizeWorkspace).filter(Boolean) : []
  } catch {
    workspacesManual.value = []
  }
}

const saveManualWorkspaces = () => {
  const uniq = Array.from(new Set(workspacesManual.value.map(normalizeWorkspace).filter(Boolean)))
  workspacesManual.value = uniq
  localStorage.setItem(STORAGE_KEY, JSON.stringify(uniq))
}

const refreshTasks = async () => {
  tasksError.value = ''
  tasksLoading.value = true
  try {
    const list = await listTasks()
    const nextTasks = Array.isArray(list) ? list : []

    if (!notifyInitialized.value) {
      // Establish baseline without emitting notifications.
      const { next } = diffTaskUpdates({}, nextTasks)
      taskSnapshots.value = next
      saveTaskSnapshotsToStorage(TASK_SNAPSHOT_KEY, taskSnapshots.value)
      notifyInitialized.value = true
    } else {
      const { next, updates } = diffTaskUpdates(taskSnapshots.value, nextTasks)
      taskSnapshots.value = next
      saveTaskSnapshotsToStorage(TASK_SNAPSHOT_KEY, next)
      if (updates.length) {
        const seen = new Set(taskUpdates.value.map((u) => `${u.taskId}:${u.attemptId}:${u.status}`))
        const merged = [...updates.filter((u) => !seen.has(`${u.taskId}:${u.attemptId}:${u.status}`)), ...taskUpdates.value]
        taskUpdates.value = merged.slice(0, 10)
      }
    }

    tasks.value = nextTasks
  } catch (e: any) {
    tasksError.value = String(e?.data?.error || e?.message || 'Failed to load tasks')
    tasks.value = []
  } finally {
    tasksLoading.value = false
  }
}

const clearTaskUpdates = () => {
  taskUpdates.value = []
}

const refreshSelected = async () => {
  selectedError.value = ''
  const id = normalizeWorkspace(selectedTaskId.value)
  if (!id) {
    selectedTask.value = null
    selectedEvents.value = []
    return
  }

  selectedLoading.value = true
  try {
    const [t, evs] = await Promise.all([getTask(id), getTaskEvents(id)])
    selectedTask.value = t
    selectedEvents.value = Array.isArray(evs) ? evs : []
  } catch (e: any) {
    selectedError.value = String(e?.data?.error || e?.message || 'Failed to load task')
    selectedTask.value = null
    selectedEvents.value = []
  } finally {
    selectedLoading.value = false
  }
}

const allWorkspaces = computed(() => {
  const fromTasks = tasks.value.map((t) => normalizeWorkspace(t.workspace)).filter(Boolean)
  const defaultWS = normalizeWorkspace(localStorage.getItem('oneagent-workspace') || '')
  const merged = [...workspacesManual.value, ...fromTasks]
  if (defaultWS) merged.unshift(defaultWS)
  return Array.from(new Set(merged.map(normalizeWorkspace).filter(Boolean))).sort()
})

const workspaceSummaries = computed<WorkspaceSummary[]>(() => {
  const byWS = new Map<string, WorkspaceSummary>()
  for (const t of tasks.value) {
    const ws = normalizeWorkspace(t.workspace)
    if (!ws) continue
    if (!byWS.has(ws)) {
      byWS.set(ws, { workspace: ws, total: 0, queued: 0, running: 0, failed: 0 })
    }
    const s = byWS.get(ws)!
    s.total++
    const a = Array.isArray(t.attempts) && t.attempts.length ? t.attempts[t.attempts.length - 1] : null
    const st = String(a?.status || '')
    if (st === 'queued') s.queued++
    else if (st === 'running') s.running++
    else if (['failed', 'timed_out', 'interrupted'].includes(st)) s.failed++
  }
  return Array.from(byWS.values()).sort((a, b) => a.workspace.localeCompare(b.workspace))
})

const filteredTasks = computed(() => {
  const ws = normalizeWorkspace(workspaceSelected.value)
  if (!ws) return tasks.value
  return tasks.value.filter((t) => normalizeWorkspace(t.workspace) === ws)
})

const selectWorkspace = (ws: string) => {
  workspaceSelected.value = ws
  if (selectedTask.value && normalizeWorkspace(selectedTask.value.workspace) !== normalizeWorkspace(ws)) {
    selectedTaskId.value = ''
    selectedTask.value = null
    selectedEvents.value = []
  }
}

const addWorkspace = () => {
  const ws = normalizeWorkspace(workspaceNew.value)
  if (!ws) return
  if (!workspacesManual.value.includes(ws)) {
    workspacesManual.value = [...workspacesManual.value, ws]
    saveManualWorkspaces()
  }
  workspaceNew.value = ''
  if (!workspaceSelected.value) workspaceSelected.value = ws
}

const queueTask = async () => {
  tasksError.value = ''
  const ws = normalizeWorkspace(workspaceSelected.value)
  const p = normalizeWorkspace(prompt.value)
  if (!ws || !p) return

  submitting.value = true
  try {
    const created = await createTask({
      workspace: ws,
      title: normalizeWorkspace(title.value) || undefined,
      prompt: p,
    })
    prompt.value = ''
    title.value = ''
    selectedTaskId.value = created.id
    await refreshTasks()
    await refreshSelected()
  } catch (e: any) {
    tasksError.value = String(e?.data?.error || e?.message || 'Failed to create task')
  } finally {
    submitting.value = false
  }
}

const doCancel = async () => {
  const id = normalizeWorkspace(selectedTaskId.value)
  if (!id) return
  selectedError.value = ''
  try {
    await cancelTask(id)
    await refreshTasks()
    await refreshSelected()
  } catch (e: any) {
    selectedError.value = String(e?.data?.error || e?.message || 'Failed to cancel task')
  }
}

const doResume = async () => {
  const id = normalizeWorkspace(selectedTaskId.value)
  if (!id) return
  selectedError.value = ''
  try {
    await resumeTask(id)
    await refreshTasks()
    await refreshSelected()
  } catch (e: any) {
    selectedError.value = String(e?.data?.error || e?.message || 'Failed to resume task')
  }
}

onMounted(async () => {
  loadManualWorkspaces()
  await refreshTasks()
  if (!workspaceSelected.value) {
    workspaceSelected.value = allWorkspaces.value[0] || ''
  }
})

let timer: number | undefined
onMounted(() => {
  timer = window.setInterval(() => {
    void refreshTasks()
    if (selectedTaskId.value) {
      void refreshSelected()
    }
  }, 2000)
})
onUnmounted(() => {
  if (timer != null) {
    window.clearInterval(timer)
    timer = undefined
  }
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto">
      <div class="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">Task Workbench</h1>
          <p class="text-sm text-surface-500">Multi-workspace queue, progress, and controls</p>
        </div>
        <button
          data-testid="task-workbench-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="tasksLoading"
          @click="refreshTasks()"
        >
          <RefreshCw class="w-4 h-4" />
          Refresh
        </button>
      </div>

      <div
        v-if="taskUpdates.length"
        data-testid="task-updates"
        class="mb-4 rounded-2xl border border-surface-700/40 bg-surface-950/40 p-4"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="text-sm font-semibold text-surface-100">Updates</div>
          <button class="text-xs text-surface-400 hover:text-surface-200" @click="clearTaskUpdates">Clear</button>
        </div>
        <div class="mt-2 space-y-2">
          <div v-for="u in taskUpdates" :key="`${u.taskId}:${u.attemptId}:${u.status}`" class="text-sm text-surface-200">
            <span class="font-mono text-surface-400">{{ u.status }}</span>
            <span class="mx-2 text-surface-600">·</span>
            <span class="text-surface-100">{{ u.title }}</span>
            <span class="mx-2 text-surface-600">·</span>
            <span class="text-surface-400 truncate">{{ u.workspace }}</span>
          </div>
        </div>
      </div>

      <div v-if="tasksError" class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
        {{ tasksError }}
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- Workspaces -->
        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50">
            <p class="text-sm font-semibold text-surface-100">Workspaces</p>
            <p class="text-xs text-surface-500 mt-1">Each workspace runs tasks FIFO (L0)</p>
          </div>

          <div class="p-4 border-b border-surface-700/50">
            <div class="flex gap-2">
              <input
                data-testid="workspace-add-input"
                v-model="workspaceNew"
                class="flex-1 px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                placeholder="/path/to/workspace"
              />
              <button
                data-testid="workspace-add"
                class="px-3 py-2 rounded-xl text-sm font-medium bg-primary-500/15 text-primary-300 hover:bg-primary-500/20 inline-flex items-center gap-2"
                @click="addWorkspace"
              >
                <Plus class="w-4 h-4" />
              </button>
            </div>
          </div>

          <div class="max-h-[60vh] overflow-y-auto">
            <button
              v-for="ws in allWorkspaces"
              :key="ws"
              data-testid="workspace-item"
              class="w-full text-left px-4 py-3 border-b border-surface-700/30 hover:bg-surface-900/40"
              :class="workspaceSelected === ws ? 'bg-primary-500/10' : ''"
              @click="selectWorkspace(ws)"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="flex items-center gap-2 text-sm text-surface-100">
                    <Folder class="w-4 h-4 text-surface-400" />
                    <span class="truncate">{{ ws }}</span>
                  </div>
                  <div class="text-xs text-surface-500 mt-1">
                    <span v-if="workspaceSummaries.find((s) => s.workspace === ws)">
                      {{
                        (() => {
                          const s = workspaceSummaries.find((x) => x.workspace === ws)
                          if (!s) return ''
                          return `total ${s.total} · queued ${s.queued} · running ${s.running} · failed ${s.failed}`
                        })()
                      }}
                    </span>
                    <span v-else>no tasks yet</span>
                  </div>
                </div>
              </div>
            </button>
            <div v-if="allWorkspaces.length === 0" class="p-6 text-sm text-surface-500">No workspaces yet.</div>
          </div>
        </div>

        <!-- Tasks list + composer -->
        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between">
            <p class="text-sm font-semibold text-surface-100">Tasks</p>
            <div class="inline-flex items-center gap-2 text-xs text-surface-500">
              <ListTodo class="w-4 h-4" />
              {{ filteredTasks.length }}
            </div>
          </div>

          <div class="p-4 border-b border-surface-700/50">
            <div class="grid grid-cols-1 gap-2">
              <input
                v-model="title"
                class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                placeholder="Optional title"
              />
              <textarea
                data-testid="workbench-prompt"
                v-model="prompt"
                rows="4"
                class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                placeholder="Describe what you want done..."
              />
              <button
                data-testid="workbench-queue"
                class="px-4 py-2 rounded-xl text-sm font-medium bg-primary-500/15 text-primary-300 hover:bg-primary-500/20 disabled:opacity-50"
                :disabled="submitting || !workspaceSelected || !prompt.trim()"
                @click="queueTask"
              >
                Queue task
              </button>
            </div>
          </div>

          <div class="max-h-[50vh] overflow-y-auto">
            <button
              v-for="t in filteredTasks"
              :key="t.id"
              data-testid="workbench-task-item"
              class="w-full text-left px-4 py-3 border-b border-surface-700/30 hover:bg-surface-900/40"
              :class="selectedTaskId === t.id ? 'bg-primary-500/10' : ''"
              @click="selectedTaskId = t.id; refreshSelected()"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="text-sm text-surface-100 truncate">{{ t.title || t.id }}</div>
                  <div class="text-xs text-surface-500 mt-1 truncate">{{ t.prompt }}</div>
                </div>
                <div class="text-xs text-surface-400">
                  <span class="inline-flex items-center rounded-full px-2 py-0.5 border border-surface-700/40 bg-surface-900/40">
                    {{ t.attempts?.length ? t.attempts[t.attempts.length - 1].status : 'queued' }}
                  </span>
                </div>
              </div>
            </button>
            <div v-if="filteredTasks.length === 0" class="p-6 text-sm text-surface-500">No tasks for this workspace.</div>
          </div>
        </div>

        <!-- Task details -->
        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-2">
            <p class="text-sm font-semibold text-surface-100">Details</p>
            <div class="inline-flex gap-2">
              <button
                data-testid="workbench-cancel"
                class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2"
                :disabled="!canCancel || selectedLoading"
                @click="doCancel"
              >
                <X class="w-4 h-4" />
                Cancel
              </button>
              <button
                data-testid="workbench-resume"
                class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2"
                :disabled="!canResume || selectedLoading"
                @click="doResume"
              >
                <RotateCcw class="w-4 h-4" />
                Resume
              </button>
            </div>
          </div>

          <div v-if="selectedError" class="m-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
            {{ selectedError }}
          </div>

          <div v-if="!selectedTask" class="p-6 text-sm text-surface-500">Select a task to view details.</div>

          <div v-else class="p-4 space-y-4">
            <div>
              <div class="text-sm text-surface-100 font-semibold">{{ selectedTask.title }}</div>
              <div class="text-xs text-surface-500 mt-1 break-words">{{ selectedTask.workspace }}</div>
            </div>

            <div v-if="latestAttempt" class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
              <div class="flex items-center justify-between gap-2">
                <div class="text-sm text-surface-100 font-semibold">Latest attempt</div>
                <div class="text-xs text-surface-400">{{ latestAttempt.status }}</div>
              </div>
              <div v-if="latestAttempt.summary" class="text-sm text-surface-200 mt-2 whitespace-pre-wrap">{{ latestAttempt.summary }}</div>
              <div v-if="latestAttempt.findings_path" class="text-xs text-surface-400 mt-2">findings: {{ latestAttempt.findings_path }}</div>
              <div v-if="latestAttempt.trace_log_path" class="text-xs text-surface-400 mt-1">trace: {{ latestAttempt.trace_log_path }}</div>
              <div v-if="latestAttempt.policy_snapshot" class="text-xs text-surface-400 mt-1">
                policy:
                <span class="font-mono text-surface-200">{{ latestAttempt.policy_snapshot.policy?.id }}</span>
                ·
                <span class="font-mono text-surface-200">{{ shortHash(latestAttempt.policy_snapshot.policy_hash) }}</span>
                · principal={{ latestAttempt.policy_snapshot.principal_id }}
              </div>
              <div v-if="latestAttempt.observer" class="text-xs text-surface-400 mt-2">
                Observer: {{ latestAttempt.observer.pass ? 'pass' : 'fail' }} · {{ latestAttempt.observer.reason }}
              </div>
            </div>

            <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
              <div class="text-sm text-surface-100 font-semibold">Events</div>
              <div v-if="selectedLoading" class="text-xs text-surface-500 mt-2">Loading…</div>
              <div v-else-if="selectedEvents.length === 0" class="text-xs text-surface-500 mt-2">No events.</div>
              <div v-else class="mt-2 space-y-2 max-h-[40vh] overflow-y-auto">
                <div v-for="(e, idx) in selectedEvents" :key="idx" class="text-xs text-surface-300">
                  <span class="text-surface-500">{{ e.ts }}</span>
                  <span class="mx-2 text-surface-600">·</span>
                  <span class="font-mono text-surface-200">{{ e.type }}</span>
                  <span v-if="e.message" class="ml-2">{{ e.message }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
