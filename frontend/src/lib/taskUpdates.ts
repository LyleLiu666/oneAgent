import type { Task, TaskAttempt, AttemptStatus } from '@/api/client'

export type TaskSnapshot = {
  attemptId: string
  status: AttemptStatus
}

export type TaskUpdate = {
  taskId: string
  attemptId: string
  status: AttemptStatus
  title: string
  workspace: string
  finishedAt?: string
}

const TERMINAL_STATUSES = new Set<AttemptStatus>(['succeeded', 'failed', 'timed_out', 'interrupted', 'canceled'])

export function isTerminalStatus(status: AttemptStatus | string): status is AttemptStatus {
  return TERMINAL_STATUSES.has(status as AttemptStatus)
}

export function latestAttempt(task: Task): TaskAttempt | null {
  if (!task || !Array.isArray(task.attempts) || task.attempts.length === 0) return null
  return task.attempts[task.attempts.length - 1] || null
}

export function buildTaskSnapshots(tasks: Task[]): Record<string, TaskSnapshot> {
  const out: Record<string, TaskSnapshot> = {}
  for (const t of tasks || []) {
    const a = latestAttempt(t)
    if (!a) continue
    out[String(t.id)] = { attemptId: String(a.id), status: a.status }
  }
  return out
}

export function diffTaskUpdates(
  prev: Record<string, TaskSnapshot>,
  tasks: Task[]
): { next: Record<string, TaskSnapshot>; updates: TaskUpdate[] } {
  const next = buildTaskSnapshots(tasks)
  const updates: TaskUpdate[] = []

  for (const t of tasks || []) {
    const a = latestAttempt(t)
    if (!a) continue
    const taskId = String(t.id)
    const prevSnap = prev[taskId]
    if (!prevSnap) continue // new task: don't notify

    const changed = prevSnap.attemptId !== a.id || prevSnap.status !== a.status
    if (!changed) continue

    const becameTerminal = !isTerminalStatus(prevSnap.status) && isTerminalStatus(a.status)
    if (!becameTerminal) continue

    updates.push({
      taskId,
      attemptId: String(a.id),
      status: a.status,
      title: String(t.title || t.id),
      workspace: String(t.workspace || ''),
      finishedAt: a.finished_at,
    })
  }

  return { next, updates }
}

type StorageShape = Record<string, TaskSnapshot>

export function loadTaskSnapshotsFromStorage(storageKey: string): Record<string, TaskSnapshot> {
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as StorageShape
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed
  } catch {
    return {}
  }
}

export function saveTaskSnapshotsToStorage(storageKey: string, snapshots: Record<string, TaskSnapshot>) {
  try {
    localStorage.setItem(storageKey, JSON.stringify(snapshots || {}))
  } catch {
    // ignore
  }
}

