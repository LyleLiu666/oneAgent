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
    getTaskQueueGovernance: vi.fn(async () => ({ global: { max_running_workspaces: 0 }, workspaces: {}, schedules: [] })),
    getTaskQueueGovernanceSnapshot: vi.fn(async () => ({
        ts: new Date().toISOString(),
        global: { max_running_workspaces: 0 },
        running_workspaces: [],
        deferred_workspaces: 0,
        paused_workspaces: 0,
        workspaces: {},
    })),
    updateTaskQueueWorkspacePolicy: vi.fn(async () => ({ global: { max_running_workspaces: 0 }, workspaces: {}, schedules: [] })),
    createTaskQueueSchedule: vi.fn(async () => ({ global: { max_running_workspaces: 0 }, workspaces: {}, schedules: [] })),
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
                observer: {
                    pass: true,
                    reason: 'ok',
                    evidence: ['FINDINGS.md'],
                    next_steps: '',
                    questions_for_user: [],
                },
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
    expect(wrapper.text()).toContain('观察者')
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

    const textarea = wrapper.get('[data-testid="newtask-prompt"]')
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

it('shows newest events first', async () => {
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
            attempts: [{ id: 'a1', status: 'running', created_at: new Date().toISOString() }],
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
        attempts: [{ id: 'a1', status: 'running', created_at: new Date().toISOString() }],
    })
    ;(apiClient.getTaskEvents as any).mockResolvedValueOnce([
        { ts: '2020-01-01T00:00:00.000Z', task_id: 'task-1', type: 'task.created', message: 'old event' },
        { ts: '2020-01-01T00:00:01.000Z', task_id: 'task-1', type: 'attempt.running', message: 'new event' },
    ])

    const wrapper = shallowMount(TaskQueuePanel, {
        props: {
            workspace: '/tmp/ws',
            modelId: 'model-1',
        },
    })

    await flushPromises()

    await wrapper.get('[data-testid="tasks-toggle"]').trigger('click')
    await flushPromises()

    const taskButton = wrapper.get('[data-testid="task-item"][data-task-id="task-1"]')
    await taskButton.trigger('click')
    await flushPromises()
    await flushPromises()

    const viewer = wrapper.findComponent({ name: 'EventLogViewer' })
    expect(viewer.exists()).toBe(true)
    const evs = viewer.props('events') as any[]
    expect(evs).toHaveLength(2)
    expect(evs[0].message).toBe('new event')
    expect(evs[1].message).toBe('old event')

    wrapper.unmount()
})

it('shows updates when a task finishes after baseline', async () => {
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
            attempts: [{ id: 'a1', status: 'running', created_at: new Date().toISOString() }],
        },
    ])

    const wrapper = shallowMount(TaskQueuePanel, {
        props: {
            workspace: '/tmp/ws',
            modelId: 'model-1',
        },
    })

    await flushPromises()

    await wrapper.get('[data-testid="tasks-toggle"]').trigger('click')
    await flushPromises()

    ;(apiClient.listTasks as any).mockResolvedValueOnce([
        {
            id: 'task-1',
            user_id: 'local',
            workspace: '/tmp/ws',
            title: 'T1',
            prompt: 'do it',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            attempts: [
                { id: 'a1', status: 'succeeded', created_at: new Date().toISOString(), finished_at: new Date().toISOString() },
            ],
        },
    ])

    await wrapper.get('[data-testid="tasks-refresh"]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="task-updates"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('更新')
    expect(wrapper.text()).toContain('succeeded')

    wrapper.unmount()
})

it('updates workspace governance policy via API', async () => {
    const { default: TaskQueuePanel } = await import('@/components/TaskQueuePanel.vue')

    ;(apiClient.listTasks as any).mockResolvedValueOnce([])
    ;(apiClient.getTaskQueueGovernance as any).mockResolvedValueOnce({
        global: { max_running_workspaces: 0 },
        workspaces: { '/tmp/ws': { paused: false, priority: 1 } },
        schedules: [],
    })

    const wrapper = shallowMount(TaskQueuePanel, {
        props: {
            workspace: '/tmp/ws',
            modelId: 'model-1',
        },
    })
    await flushPromises()

    await wrapper.get('[data-testid="tasks-toggle"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="governance-paused"]').setValue(true)
    await wrapper.get('[data-testid="governance-priority"]').setValue('3')

    await wrapper.get('[data-testid="governance-save"]').trigger('click')
    await flushPromises()

    expect(apiClient.updateTaskQueueWorkspacePolicy).toHaveBeenCalledWith(
        expect.objectContaining({
            workspace: '/tmp/ws',
            paused: true,
            priority: 3,
        }),
    )

    wrapper.unmount()
})
