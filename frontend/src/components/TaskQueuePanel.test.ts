// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
    listTasks: vi.fn(async () => []),
    createTask: vi.fn(),
    getTask: vi.fn(),
    getTaskEvents: vi.fn(async () => []),
    cancelTask: vi.fn(),
    resumeTask: vi.fn(),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

it('loads tasks for workspace and shows details after selection', async () => {
    const { default: TaskQueuePanel } = await import('@/components/TaskQueuePanel.vue')

    ;(apiClient.listTasks as any).mockResolvedValueOnce([
        {
            id: 'task-1',
            user_id: 'local',
            workspace: '/tmp/ws',
            title: 'T1',
            prompt: 'do it',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            attempts: [{ id: 'a1', status: 'queued', created_at: new Date().toISOString() }],
        },
    ])
    ;(apiClient.getTask as any).mockResolvedValueOnce({
        id: 'task-1',
        user_id: 'local',
        workspace: '/tmp/ws',
        title: 'T1',
        prompt: 'do it',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        attempts: [
            {
                id: 'a1',
                status: 'succeeded',
                created_at: new Date().toISOString(),
                summary: 'done',
                findings_path: '/tmp/findings',
                trace_log_path: '/tmp/trace',
                observer: { pass: true, reason: 'ok', evidence: ['FINDINGS.md'] },
            },
        ],
    })
    ;(apiClient.getTaskEvents as any).mockResolvedValueOnce([
        { ts: new Date().toISOString(), task_id: 'task-1', type: 'task.created', message: 'Task created' },
    ])

    const wrapper = shallowMount(TaskQueuePanel, {
        props: {
            workspace: '/tmp/ws',
            modelId: 'model-1',
        },
    })

    await flushPromises()
    expect(apiClient.listTasks).toHaveBeenCalled()

    await wrapper.get('[data-testid="tasks-toggle"]').trigger('click')
    await flushPromises()

    const taskButton = wrapper.get('[data-testid="task-item"][data-task-id="task-1"]')
    await taskButton.trigger('click')
    await flushPromises()

    expect(apiClient.getTask).toHaveBeenCalledWith('task-1')
    expect(apiClient.getTaskEvents).toHaveBeenCalledWith('task-1')
    expect(wrapper.text()).toContain('Observer')
    expect(wrapper.text()).toContain('succeeded')

    wrapper.unmount()
})

it('queues a new task using current workspace and model', async () => {
    const { default: TaskQueuePanel } = await import('@/components/TaskQueuePanel.vue')

    ;(apiClient.listTasks as any).mockResolvedValueOnce([])
    ;(apiClient.createTask as any).mockResolvedValueOnce({
        id: 'task-1',
        user_id: 'local',
        workspace: '/tmp/ws',
        title: 'T1',
        prompt: 'do it',
        model_id: 'model-1',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        attempts: [{ id: 'a1', status: 'queued', created_at: new Date().toISOString() }],
    })
    ;(apiClient.listTasks as any).mockResolvedValueOnce([
        {
            id: 'task-1',
            user_id: 'local',
            workspace: '/tmp/ws',
            title: 'T1',
            prompt: 'do it',
            model_id: 'model-1',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            attempts: [{ id: 'a1', status: 'queued', created_at: new Date().toISOString() }],
        },
    ])
    ;(apiClient.getTask as any).mockResolvedValueOnce({
        id: 'task-1',
        user_id: 'local',
        workspace: '/tmp/ws',
        title: 'T1',
        prompt: 'do it',
        model_id: 'model-1',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        attempts: [{ id: 'a1', status: 'queued', created_at: new Date().toISOString() }],
    })
    ;(apiClient.getTaskEvents as any).mockResolvedValueOnce([])

    const wrapper = shallowMount(TaskQueuePanel, {
        props: {
            workspace: '/tmp/ws',
            modelId: 'model-1',
        },
    })
    await flushPromises()

    await wrapper.get('[data-testid="tasks-toggle"]').trigger('click')
    await flushPromises()

    const textarea = wrapper.get('textarea')
    await textarea.setValue('do it')

    const queueButton = wrapper.get('[data-testid="tasks-queue"]')
    await queueButton.trigger('click')
    await flushPromises()

    expect(apiClient.createTask).toHaveBeenCalledWith({
        workspace: '/tmp/ws',
        title: undefined,
        prompt: 'do it',
        model_id: 'model-1',
    })

    wrapper.unmount()
})
