// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
    streamChat: vi.fn(),
    getConfig: vi.fn(async () => ({ default_workspace: '', base_url: '', warnings: [] })),
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
