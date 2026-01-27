// @vitest-environment jsdom

import { expect, it } from 'vitest'
import type { Task } from '@/api/client'
import { diffTaskUpdates } from '@/lib/taskUpdates'

const now = () => new Date().toISOString()

it('does not notify on new tasks without previous snapshot', () => {
  const tasks: Task[] = [
    {
      id: 't1',
      user_id: 'local',
      workspace: '/tmp/ws',
      title: 'T1',
      prompt: 'p',
      created_at: now(),
      updated_at: now(),
      attempts: [{ id: 'a1', status: 'succeeded', created_at: now(), finished_at: now() }],
    },
  ]

  const { updates } = diffTaskUpdates({}, tasks)
  expect(updates.length).toBe(0)
})

it('notifies when queued/running becomes terminal', () => {
  const tasks: Task[] = [
    {
      id: 't1',
      user_id: 'local',
      workspace: '/tmp/ws',
      title: 'T1',
      prompt: 'p',
      created_at: now(),
      updated_at: now(),
      attempts: [{ id: 'a1', status: 'succeeded', created_at: now(), finished_at: now() }],
    },
  ]

  const prev = { t1: { attemptId: 'a1', status: 'running' as const } }
  const { updates } = diffTaskUpdates(prev, tasks)
  expect(updates.length).toBe(1)
  expect(updates[0].status).toBe('succeeded')
})

