// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
    streamChat: vi.fn(),
    attachChatStream: vi.fn(),
    stopSessionStream: vi.fn(),
    appendSecretaryInboxMessage: vi.fn(),
    secretaryTriage: vi.fn(),
    getSecretaryState: vi.fn(async () => ({ cursor_message_id: 0, triage_runs: [] })),
    setSecretaryRecoveryFocus: vi.fn(),
    getSecretarySession: vi.fn(async () => ({ id: 's1', messages: [], metadata: {} })),
    getConfig: vi.fn(async () => ({ default_workspace: '', base_url: '', warnings: [] })),
    getLedgerStatusToday: vi.fn(async () => ({
        day_key: '2026-02-01',
        digest_exists: false,
        learning_job_status: 'none',
        sop_proposed_count: 0,
    })),
    getSessions: vi.fn(async () => []),
    getSession: vi.fn(async () => ({ messages: [], metadata: {} })),
    truncateSession: vi.fn(),
    getModels: vi.fn(async () => []),
    getTools: vi.fn(async () => []),
    chooseWorkspaceDir: vi.fn(async () => ({ path: '/tmp/workspace' })),
    approveToolApproval: vi.fn(),
    denyToolApproval: vi.fn(),
    // Task queue (used by TaskQueuePanel).
    createTask: vi.fn(),
    listTasks: vi.fn(async () => []),
    getTask: vi.fn(),
    getTaskEvents: vi.fn(async () => []),
    cancelTask: vi.fn(),
    resumeTask: vi.fn(),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
    vi.clearAllMocks()
})

it('sets workspace path after clicking Browse', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    const browseButton = wrapper.get('[data-testid="chat-workspace-choose"]')
    await browseButton.trigger('click')
    await flushPromises()

    expect(apiClient.chooseWorkspaceDir).toHaveBeenCalledTimes(1)

    const input = wrapper.get('[data-testid="chat-workspace-path"]')
    expect((input.element as HTMLInputElement).value).toBe('/tmp/workspace')
})

it('keeps session header above messages (for tool popover)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    const header = wrapper.get('.session-header')
    expect(header.classes()).toContain('relative')
    expect(header.classes()).toContain('z-30')
})

it('allows messages pane to scroll (min-h-0)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    const messages = wrapper.get({ ref: 'messagesContainer' })
    expect(messages.classes()).toContain('min-h-0')
})

it('does not render tool_result output as a giant text bubble in secretary mode', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setMessages([
        {
            id: 1,
            role: 'tool',
            type: 'tool_result',
            content: 'HUGE_TOOL_OUTPUT_SHOULD_NOT_RENDER',
            createdAt: new Date(),
            tool: { name: 'bash', output: 'HUGE_TOOL_OUTPUT_SHOULD_NOT_RENDER' },
        },
    ])

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('HUGE_TOOL_OUTPUT_SHOULD_NOT_RENDER')
})

it('renders streaming token count while waiting for content', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setLastResponseTokens(7)
    chat.setMessages([
        {
            id: 1,
            role: 'assistant',
            type: 'text',
            content: '',
            createdAt: new Date(),
            isStreaming: true,
        },
    ])

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('思考中')
    expect(wrapper.text()).toContain('7 tokens')
})

it('does not write recovery prompts into chat message list (secretary mode)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setMessages([])

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: { plugins: [pinia] },
    })

    await flushPromises()

    const deliverables = wrapper.findComponent({ name: 'SecretaryTaskDeliverables' })
    expect(deliverables.exists()).toBe(true)

    deliverables.vm.$emit('recovery-snapshot', [
        {
            taskId: 't1',
            attemptId: 'a1',
            title: 'task1',
            status: 'failed',
            summary: 'subagent finished',
            error: 'invalid observer output (expected JSON)',
        },
    ])

    await flushPromises()

    expect(chat.messages.length).toBe(0)
})

it('shows a stop button while streaming and calls stop endpoint', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setCurrentSession('s1')

    ;(apiClient.getSessions as any).mockResolvedValueOnce([
        { id: 's1', title: 't', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
    ])
    ;(apiClient.getSession as any).mockResolvedValueOnce({ metadata: {}, messages: [] })

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    chat.setLoading(true)
    chat.setMessages([
        {
            id: 1,
            role: 'assistant',
            type: 'text',
            content: 'partial',
            createdAt: new Date(),
            isStreaming: true,
        },
    ])
    await flushPromises()

    const stopButton = wrapper.get('[data-testid="chat-stop"]')
    await stopButton.trigger('click')

    expect(apiClient.stopSessionStream).toHaveBeenCalledTimes(1)
    expect(apiClient.stopSessionStream).toHaveBeenCalledWith('s1')
    expect(chat.isLoading).toBe(false)
    expect(chat.messages.some((m: any) => m.isStreaming)).toBe(false)
})

it('attaches to in-flight stream on reload when last message is user', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setCurrentSession('s1')

    ;(apiClient.getSessions as any).mockResolvedValueOnce([
        { id: 's1', title: 't', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
    ])
    ;(apiClient.getSession as any).mockResolvedValueOnce({
        metadata: {},
        messages: [
            { id: 1, role: 'user', type: 'text', content: 'hello', created_at: new Date().toISOString() },
        ],
    })
    ;(apiClient.attachChatStream as any).mockImplementation(async () => {})

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    expect(apiClient.attachChatStream).toHaveBeenCalledTimes(1)
    expect((apiClient.attachChatStream as any).mock.calls[0][0]).toBe('s1')
    expect(wrapper.exists()).toBe(true)
})

it('hands off input to task queue in secretary mode', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    ;(apiClient.createTask as any).mockResolvedValueOnce({
        id: 't1',
        user_id: 'local',
        workspace: '/tmp/workspace',
        title: 'T1',
        prompt: 'do the thing',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        attempts: [],
    })

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    await wrapper.get('textarea').setValue('do the thing')
    await wrapper.get('[data-testid="chat-handoff-task"]').trigger('click')
    await flushPromises()

    expect(apiClient.chooseWorkspaceDir).toHaveBeenCalledTimes(1)
    expect(apiClient.createTask).toHaveBeenCalledTimes(1)
    expect(apiClient.createTask).toHaveBeenCalledWith({
        workspace: '/tmp/workspace',
        prompt: 'do the thing',
        model_id: undefined,
    })
    expect(apiClient.streamChat).not.toHaveBeenCalled()

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
})

it('does not handoff to task queue when workspace selection is canceled', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    ;(apiClient.chooseWorkspaceDir as any).mockResolvedValueOnce({ canceled: true })

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    await wrapper.get('textarea').setValue('do the thing')
    await wrapper.get('[data-testid="chat-handoff-task"]').trigger('click')
    await flushPromises()

    expect(apiClient.createTask).not.toHaveBeenCalled()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('do the thing')
})

it('sends messages via secretary inbox API in secretary mode', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
        session_id: 's1',
        message_id: 1,
        ack_message_id: 0,
        ack_text: '',
    })

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    await wrapper.get('textarea').setValue('帮我跑一下测试并修复失败用例')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)
    expect(apiClient.streamChat).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="chat-handoff-suggest"]').exists()).toBe(false)

    wrapper.unmount()
})

it('surfaces pending triage questions in a modal (secretary mode)', async () => {
    vi.useFakeTimers()
    try {
        const flush = async () => {
            const p = flushPromises()
            vi.advanceTimersByTime(0)
            await p
        }

        const store = new Map<string, string>()
        vi.stubGlobal('localStorage', {
            getItem: (key: string) => store.get(key) ?? null,
            setItem: (key: string, value: string) => void store.set(key, String(value)),
            removeItem: (key: string) => void store.delete(key),
            clear: () => void store.clear(),
        })

        const pinia = createPinia()
        setActivePinia(pinia)

        ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
            session_id: 's1',
            message_id: 1,
            ack_message_id: 0,
            ack_text: '',
        })

        ;(apiClient.secretaryTriage as any).mockResolvedValueOnce({
            session_id: 's1',
            summary_message: '我这边卡在一个点，需要你确认。',
            summary_message_id: 10,
            cursor_message_id: 1,
            created_task_ids: [],
            questions: ['用哪个目录来做？'],
            workspaces_created: [],
        })

        const { default: ChatBox } = await import('@/components/ChatBox.vue')

        const wrapper = shallowMount(ChatBox, {
            props: { initialMode: 'secretary' },
            global: {
                plugins: [pinia],
            },
        })

        await flush()
        await flush()

        await wrapper.get('textarea').setValue('帮我修一下测试')
        await wrapper.get('[data-testid="chat-send"]').trigger('click')
        await flush()
        await flush()

        expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)

        await vi.advanceTimersByTimeAsync(900)
        await flush()

        expect(apiClient.secretaryTriage).toHaveBeenCalledTimes(1)
        expect(wrapper.find('[data-testid="secretary-pending-questions"]').exists()).toBe(true)
        expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(false)

        await wrapper.get('[data-testid="secretary-pending-questions"]').trigger('click')
        await flush()
        expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(true)
        expect(wrapper.text()).toContain('用哪个目录来做？')

        await wrapper.get('[data-testid="secretary-pending-questions-modal-close"]').trigger('click')
        await flush()
        expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(false)

        wrapper.unmount()
    } finally {
        vi.useRealTimers()
    }
})

it('allows multiple sends in secretary mode and debounces triage', async () => {
    vi.useFakeTimers()
    try {
        const flush = async () => {
            const p = flushPromises()
            vi.advanceTimersByTime(0)
            await p
        }

        const store = new Map<string, string>()
        vi.stubGlobal('localStorage', {
            getItem: (key: string) => store.get(key) ?? null,
            setItem: (key: string, value: string) => void store.set(key, String(value)),
            removeItem: (key: string) => void store.delete(key),
            clear: () => void store.clear(),
        })

        const pinia = createPinia()
        setActivePinia(pinia)

        ;(apiClient.appendSecretaryInboxMessage as any)
            .mockResolvedValueOnce({ session_id: 's1', message_id: 1, ack_message_id: 0, ack_text: '' })
            .mockResolvedValueOnce({ session_id: 's1', message_id: 3, ack_message_id: 0, ack_text: '' })
            .mockResolvedValueOnce({ session_id: 's1', message_id: 5, ack_message_id: 0, ack_text: '' })

        let resolveFirstTriage: (value: any) => void
        const firstTriage = new Promise((resolve) => {
            resolveFirstTriage = resolve
        })

        ;(apiClient.secretaryTriage as any)
            .mockReturnValueOnce(firstTriage)
            .mockResolvedValueOnce({
                summary_message: 'sum2',
                summary_message_id: 20,
                cursor_message_id: 5,
                created_task_ids: [],
                questions: [],
                workspaces_created: [],
            })

        const { default: ChatBox } = await import('@/components/ChatBox.vue')

        const wrapper = shallowMount(ChatBox, {
            props: { initialMode: 'secretary' },
            global: {
                plugins: [pinia],
            },
        })

        await flush()

        await wrapper.get('textarea').setValue('m1')
        await wrapper.get('[data-testid="chat-send"]').trigger('click')
        await flush()

        await wrapper.get('textarea').setValue('m2')
        await wrapper.get('[data-testid="chat-send"]').trigger('click')
        await flush()

        expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(2)

        // Debounce: only one triage call after the burst.
        vi.advanceTimersByTime(900)
        await flush()
        expect(apiClient.secretaryTriage).toHaveBeenCalledTimes(1)

        // While triage is still in-flight, sending another message should still work.
        await wrapper.get('textarea').setValue('m3')
        await wrapper.get('[data-testid="chat-send"]').trigger('click')
        await flush()
        expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(3)

        // Let the queued debounce fire while triage is still in-flight.
        vi.advanceTimersByTime(900)
        await flush()
        expect(apiClient.secretaryTriage).toHaveBeenCalledTimes(1)

        // Finish first triage, then queued triage should run once more.
        resolveFirstTriage!({
            summary_message: 'sum1',
            summary_message_id: 10,
            cursor_message_id: 4,
            created_task_ids: [],
            questions: [],
            workspaces_created: [],
        })
        await flush()

        vi.advanceTimersByTime(900)
        await flush()
        expect(apiClient.secretaryTriage).toHaveBeenCalledTimes(2)

        wrapper.unmount()
    } finally {
        vi.useRealTimers()
    }
})

it('does not inject chat messages when receiving a task-completed event (secretary mode)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()

    const { default: ChatBox } = await import('@/components/ChatBox.vue')
    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
            stubs: {
                SecretaryTaskDeliverables: {
                    template: `<button data-testid="emit-task-completed" @click="$emit('task-completed', { taskId: 't1', attemptId: 'a1', title: 'task1', status: 'failed', summary: 'WEATHER_RESULT', continuing: true })"></button>`,
                },
            },
        },
    })

    await flushPromises()

    expect(chat.messages.filter((m: any) => m.role === 'assistant').length).toBe(0)

    await wrapper.get('[data-testid="emit-task-completed"]').trigger('click')
    await flushPromises()

    expect(chat.messages.filter((m: any) => m.role === 'assistant').length).toBe(0)
})

it('optimistically renders secretary message and prevents resubmission while pending', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()

    let resolveAppend: ((value: any) => void) | undefined
    const appendPromise = new Promise((resolve) => {
        resolveAppend = resolve
    })
    ;(apiClient.appendSecretaryInboxMessage as any).mockImplementation(() => appendPromise)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    const textarea = wrapper.get('textarea')
    await textarea.setValue('你好。可以帮我写一篇文章吗？')

    const sendButton = wrapper.get('[data-testid="chat-send"]')
    void sendButton.trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)
    expect(chat.messages.some((m: any) => m.role === 'user' && String(m.content).includes('可以帮我写一篇文章'))).toBe(true)

    // While pending, input is disabled and a second click should not enqueue another request.
    expect(textarea.attributes('disabled')).toBeDefined()
    void sendButton.trigger('click')
    await flushPromises()
    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)

    resolveAppend?.({
        session_id: 's1',
        message_id: 101,
        ack_message_id: 0,
        ack_text: '',
    })
    await flushPromises()

    expect(chat.messages.some((m: any) => Number(m.id) === 101 && Number(m.serverId) === 101)).toBe(true)
    expect(chat.messages.some((m: any) => m.role === 'assistant')).toBe(false)
})

it('does not start a recovery conversation in chat (secretary mode)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()

    const { default: ChatBox } = await import('@/components/ChatBox.vue')
    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
            stubs: {
                SecretaryTaskDeliverables: {
                    template: `
                      <button data-testid="emit-recovery-snapshot" @click="$emit('recovery-snapshot', [
                        { taskId: 't1', attemptId: 'a1', title: 'task1', status: 'failed', summary: 's1', observer: { next_steps: 'n1', questions_for_user: ['q1'] } },
                        { taskId: 't2', attemptId: 'a2', title: 'task2', status: 'failed', summary: 's2', observer: { next_steps: 'n2' } }
                      ])"></button>
                    `,
                },
            },
        },
    })

    await flushPromises()
    expect(chat.messages.length).toBe(0)

    await wrapper.get('[data-testid="emit-recovery-snapshot"]').trigger('click')
    await flushPromises()

    expect(chat.messages.length).toBe(0)
})

it('does not hijack chat input to resume tasks (secretary mode)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()

    const { default: ChatBox } = await import('@/components/ChatBox.vue')
    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
            stubs: {
                SecretaryTaskDeliverables: {
                    template: `
                      <button data-testid="emit-task1" @click="$emit('task-needs-attention', { taskId: 't1', attemptId: 'a1', title: 'task1', status: 'failed', summary: 's1', observer: { next_steps: 'n1', questions_for_user: ['q1'] } })"></button>
                      <button data-testid="emit-task2" @click="$emit('task-needs-attention', { taskId: 't2', attemptId: 'a2', title: 'task2', status: 'failed', summary: 's2', observer: { next_steps: 'n2' } })"></button>
                      <button data-testid="focus-task2" @click="$emit('recovery-focus', { taskId: 't2', attemptId: 'a2', title: 'task2', status: 'failed', summary: 's2', observer: { next_steps: 'n2' } })"></button>
                    `,
                },
            },
        },
    })

    await flushPromises()
    expect(chat.messages.length).toBe(0)

    await wrapper.get('[data-testid="emit-task1"]').trigger('click')
    await wrapper.get('[data-testid="emit-task2"]').trigger('click')
    await flushPromises()

    // Switch focus to task2 before replying.
    await wrapper.get('[data-testid="focus-task2"]').trigger('click')
    await flushPromises()

    await wrapper.get('textarea').setValue('先处理第二个')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)
})

it('does not write recovery action receipts into chat message list (secretary mode)', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')
    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
            stubs: {
                SecretaryTaskDeliverables: {
                    template: `
                      <button data-testid="emit-recovery-snapshot" @click="$emit('recovery-snapshot', [
                        { taskId: 't1', attemptId: 'a1', title: 'task1', status: 'failed', summary: 's1' }
                      ])"></button>
                      <button data-testid="emit-recovery-dismiss" @click="$emit('recovery-action', { action: 'dismiss', taskId: 't1', attemptId: 'a1', title: 'task1' })"></button>
                    `,
                },
            },
        },
    })

    await flushPromises()

    await wrapper.get('[data-testid="emit-recovery-snapshot"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="emit-recovery-dismiss"]').trigger('click')
    await flushPromises()

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    expect(chat.messages.length).toBe(0)

    await wrapper.get('textarea').setValue('正常聊天')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)
})
