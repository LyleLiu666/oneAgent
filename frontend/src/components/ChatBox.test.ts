// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
    streamChat: vi.fn(),
    attachChatStream: vi.fn(),
    stopSessionStream: vi.fn(),
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
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    const header = wrapper.get('.session-header')
    expect(header.classes()).toContain('relative')
    expect(header.classes()).toContain('z-30')
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
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('思考中')
    expect(wrapper.text()).toContain('7 tokens')
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
