// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import * as apiClient from '@/api/client'

vi.mock('@/api/client', () => ({
    streamChat: vi.fn(),
    attachChatStream: vi.fn(),
    attachSecretarySessionStream: vi.fn(),
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
    getSimpleToolPermissions: vi.fn(async () => ({
        principal_id: 'local',
        current_mode: 'readonly',
        command_approval_mode: 'auto',
        available_modes: [
            {
                mode: 'readonly',
                label: '只读查看',
                description: '可读、可查、不可改。',
                risk_level: 'low',
                available: true,
                current: true,
                recommended: false,
            },
        ],
        effective_scope: {
            summary: '仅影响后续执行',
            affects: [],
            does_not_affect: [],
            future_executions_only: true,
            running_attempts_unchanged: true,
            secretary_remains_read_only: true,
        },
        snapshot: { principal_id: 'local', policy: { id: 'default' }, policy_hash: 'abc12345', resolved_at: 't' },
        advanced_settings_available: true,
    })),
    updateSimpleToolPermissions: vi.fn(async () => ({
        principal_id: 'local',
        current_mode: 'sandbox_coding',
        command_approval_mode: 'auto',
        available_modes: [],
        effective_scope: {
            summary: '仅影响后续执行',
            affects: [],
            does_not_affect: [],
            future_executions_only: true,
            running_attempts_unchanged: true,
            secretary_remains_read_only: true,
        },
        snapshot: { principal_id: 'local', policy: { id: 'simple_sandbox_coding' }, policy_hash: 'def67890', resolved_at: 't' },
        advanced_settings_available: true,
    })),
    chooseWorkspaceDir: vi.fn(async () => ({ path: '/tmp/workspace' })),
    browseWorkspaceDir: vi.fn(),
    createWorkspaceDir: vi.fn(),
    approveToolApproval: vi.fn(),
    denyToolApproval: vi.fn(),
    // Task queue (used by TaskQueuePanel).
    createTask: vi.fn(),
    secretaryHandoff: vi.fn(),
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

it('disables browse when the server environment does not support the native picker', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.getConfig as any).mockResolvedValueOnce({
        default_workspace: '',
        base_url: '',
        warnings: [],
        workspace_chooser_supported: false,
        workspace_chooser_reason: '服务端不支持原生选择器，请手动填写路径。',
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
    await flushPromises()

    const browseButton = wrapper.get('[data-testid="chat-workspace-choose"]')
    expect(browseButton.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="chat-workspace-chooser-hint"]').text()).toContain('服务端不支持原生选择器')
})

it('keeps browse disabled when runtime config cannot be loaded', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.getConfig as any).mockRejectedValueOnce(new Error('boom'))

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
    await flushPromises()

    const browseButton = wrapper.get('[data-testid="chat-workspace-choose"]')
    expect(browseButton.attributes('disabled')).toBeDefined()
  expect(wrapper.get('[data-testid="chat-workspace-chooser-hint"]').text()).toContain('暂时无法确认服务端是否支持')
})

it('opens the web workspace browser when native chooser is unavailable but browser mode is supported', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.getConfig as any).mockResolvedValueOnce({
        default_workspace: '',
        base_url: '',
        warnings: [],
        workspace_chooser_supported: false,
        workspace_browser_supported: true,
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
    await flushPromises()

    const browseButton = wrapper.get('[data-testid="chat-workspace-choose"]')
    expect(browseButton.attributes('disabled')).toBeUndefined()

    await browseButton.trigger('click')
    await flushPromises()

    expect(apiClient.chooseWorkspaceDir).not.toHaveBeenCalled()
    expect(wrapper.find('workspace-browser-modal-stub').exists()).toBe(true)

    const modal = wrapper.getComponent({ name: 'WorkspaceBrowserModal' }) as any
    modal.vm.$emit('select', '/data/project')
    await flushPromises()

    const input = wrapper.get('[data-testid="chat-workspace-path"]')
    expect((input.element as HTMLInputElement).value).toBe('/data/project')
})

it('keeps tools selected by default when a saved session has no tool_ids metadata', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.getTools as any).mockResolvedValueOnce([
        { id: 'bash', name: 'Bash' },
        { id: 'edit', name: 'Edit' },
    ])
    ;(apiClient.getSessions as any).mockResolvedValueOnce([
        {
            id: 'session-1',
            title: 'Session 1',
            created_at: '2026-04-15T00:00:00Z',
            updated_at: '2026-04-15T00:00:00Z',
        },
    ])
    ;(apiClient.getSession as any).mockResolvedValueOnce({
        id: 'session-1',
        messages: [],
        metadata: {},
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chatStore = useChatStore()
    chatStore.setCurrentSession('session-1')
    chatStore.setCurrentTools(['bash'])

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    expect(chatStore.currentToolIds).toEqual(['bash', 'edit'])
    expect(wrapper.text()).not.toContain('工具 (0/2)')
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

it('shows the current execution permission chip in the header', async () => {
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
    await flushPromises()

    expect(wrapper.get('[data-testid="chat-permission-chip"]').text()).toContain('只读查看')
})

it('does not show the secretary permission suggestion card in full mode', async () => {
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
    await flushPromises()

    await wrapper.get('textarea').setValue('请帮我修改代码并运行测试')
    await flushPromises()

    expect(wrapper.find('[data-testid="chat-permission-suggestion"]').exists()).toBe(false)
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

it('shows setup guidance when no model is configured in full mode', async () => {
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
    await flushPromises()

    expect(wrapper.find('[data-testid="chat-model-setup-banner"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="chat-open-settings"]').attributes('href')).toBe('/settings')
})

it('shows actionable assistant feedback when full-mode send fails before any stream message arrives', async () => {
    const store = new Map<string, string>()
    store.set('oneagent-workspace', '/tmp/workspace')
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.streamChat as any).mockImplementation(
        async (
            _message: string,
            _sessionId: string,
            _modelId: string,
            _toolIds: string[],
            _toolProtocol: string,
            _workspace: string,
            _onEvent: unknown,
            onError: (error: Error) => void
        ) => {
            onError({
                message: '还没有配置可用的大模型',
                data: {
                    error: '还没有配置可用的大模型',
                    hint: '请前往“设置”添加 Provider，并至少设置一个默认 Model',
                },
                response: {
                    status: 400,
                    headers: { get: vi.fn() },
                },
            } as any)
        }
    )

    const pinia = createPinia()
    setActivePinia(pinia)

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()
    chat.setMessages([])

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'full' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    await wrapper.get('textarea').setValue('你好')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()
    await flushPromises()

    const assistantMessage = chat.messages.find((m: any) => m.role === 'assistant')
    expect(assistantMessage).toBeTruthy()
    expect(String(assistantMessage?.content || '')).toContain('还没有配置可用的大模型')
    expect(String(assistantMessage?.content || '')).toContain('请前往“设置”添加 Provider')
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

    ;(apiClient.secretaryHandoff as any).mockResolvedValueOnce({
        session_id: 's1',
        task_id: 't1',
        user_message_id: 101,
        assistant_message_id: 102,
        receipt_text: '已交给后台处理，交付物会出现在交付区。',
    })

    const { useChatStore } = await import('@/stores/chat')
    const chat = useChatStore()

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
    expect(apiClient.secretaryHandoff).toHaveBeenCalledTimes(1)
    expect(apiClient.secretaryHandoff).toHaveBeenCalledWith({
        workspace: '/tmp/workspace',
        prompt: 'do the thing',
        model_id: undefined,
    })
    expect(apiClient.createTask).not.toHaveBeenCalled()
    expect(apiClient.streamChat).not.toHaveBeenCalled()

    expect(chat.messages.some((m: any) => m.role === 'user' && m.content === 'do the thing')).toBe(true)
    expect(chat.messages.some((m: any) => m.role === 'assistant' && String(m.content).includes('交付区'))).toBe(true)
    expect(chat.messages.some((m: any) => m.role === 'system')).toBe(false)

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

    expect(apiClient.secretaryHandoff).not.toHaveBeenCalled()
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
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    let onEvent: ((event: any) => void) | undefined
    ;(apiClient.attachSecretarySessionStream as any).mockImplementation(async (cb: any) => {
        onEvent = cb
    })

    ;(apiClient.getSecretaryState as any)
        .mockResolvedValueOnce({ session_id: 's1', cursor_message_id: 0, triage_runs: [] })
        .mockResolvedValueOnce({
            session_id: 's1',
            cursor_message_id: 1,
            triage_runs: [
                {
                    from_cursor: 0,
                    to_message_id: 1,
                    questions: ['用哪个目录来做？'],
                },
            ],
        })

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
    await flushPromises()

    expect(apiClient.attachSecretarySessionStream).toHaveBeenCalledTimes(1)

    await wrapper.get('textarea').setValue('帮我修一下测试')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(1)
    expect(apiClient.secretaryTriage).not.toHaveBeenCalled()

    onEvent?.({
        type: 'msg',
        data: JSON.stringify({
            op: 'insert',
            id: '10',
            role: 'assistant',
            msg_type: 'text',
            delta: '我这边卡在一个点，需要你确认。',
        }),
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="secretary-pending-questions"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(false)

    await wrapper.get('[data-testid="secretary-pending-questions"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('用哪个目录来做？')

    await wrapper.get('[data-testid="secretary-pending-questions-modal-close"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="secretary-pending-questions-modal"]').exists()).toBe(false)

    wrapper.unmount()
})

it('binds a browser-selected root path in chat secretary mode even when the path is only one segment deep', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.appendSecretaryInboxMessage as any).mockClear()
    ;(apiClient.getConfig as any).mockResolvedValueOnce({
        default_workspace: '',
        base_url: '',
        warnings: [],
        workspace_chooser_supported: false,
        workspace_browser_supported: true,
    })
    ;(apiClient.getSecretaryState as any).mockResolvedValueOnce({
        session_id: 's1',
        cursor_message_id: 0,
        triage_runs: [
            {
                from_cursor: 0,
                to_message_id: 1,
                questions: ['要继续推进，我需要你发我项目目录（仓库根目录）的路径。'],
            },
        ],
    })
    ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
        session_id: 's1',
        message_id: 1,
        ack_message_id: 0,
        ack_text: '',
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="chat-workspace-choose"]').exists()).toBe(false)

    const browseButton = wrapper.get('[data-testid="secretary-browse-workspace-reply"]')
    await browseButton.trigger('click')
    await flushPromises()

    const modal = wrapper.find('workspace-browser-modal-stub')
    expect(modal.exists()).toBe(true)
    expect(modal.attributes('open')).toBe('true')

    const modalVm = wrapper.getComponent({ name: 'WorkspaceBrowserModal' }) as any
    modalVm.vm.$emit('select', '/repo')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledWith({
        content: '/repo',
        workspace: '/repo',
    })

    wrapper.unmount()
})

it('binds a one-segment absolute root path when the user types it in chat secretary mode', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.getSecretaryState as any).mockResolvedValueOnce({
        session_id: 's1',
        cursor_message_id: 0,
        triage_runs: [
            {
                from_cursor: 0,
                to_message_id: 1,
                questions: ['要继续推进，我需要你发我项目目录（仓库根目录）的路径。'],
            },
        ],
    })
    ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
        session_id: 's1',
        message_id: 1,
        ack_message_id: 0,
        ack_text: '',
    })

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    await wrapper.get('textarea').setValue('/repo')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    const args = (apiClient.appendSecretaryInboxMessage as any).mock.calls[0]?.[0]
    expect(args).toBeTruthy()
    expect(args.workspace).toBe('/repo')

    wrapper.unmount()
})

it('shows a secretary-visible error when choosing a workspace reply fails in chat secretary mode', async () => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, String(value)),
        removeItem: (key: string) => void store.delete(key),
        clear: () => void store.clear(),
    })

    ;(apiClient.chooseWorkspaceDir as any).mockClear()
    ;(apiClient.getConfig as any).mockResolvedValueOnce({
        default_workspace: '',
        base_url: '',
        warnings: [],
        workspace_chooser_supported: true,
        workspace_browser_supported: false,
    })
    ;(apiClient.getSecretaryState as any).mockResolvedValueOnce({
        session_id: 's1',
        cursor_message_id: 0,
        triage_runs: [
            {
                from_cursor: 0,
                to_message_id: 1,
                questions: ['请告诉我项目目录（仓库根目录）的路径。'],
            },
        ],
    })
    ;(apiClient.chooseWorkspaceDir as any).mockRejectedValueOnce(
        new Error('选择目录失败'),
    )

    const pinia = createPinia()
    setActivePinia(pinia)

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()
    await flushPromises()

    await wrapper.get('[data-testid="secretary-browse-workspace-reply"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="secretary-workspace-choose-error"]').text()).toContain(
        '选择目录失败',
    )

    wrapper.unmount()
})

it('allows multiple sends in secretary mode without calling triage', async () => {
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

    const { default: ChatBox } = await import('@/components/ChatBox.vue')

    const wrapper = shallowMount(ChatBox, {
        props: { initialMode: 'secretary' },
        global: {
            plugins: [pinia],
        },
    })

    await flushPromises()

    await wrapper.get('textarea').setValue('m1')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    await wrapper.get('textarea').setValue('m2')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    await wrapper.get('textarea').setValue('m3')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    expect(apiClient.appendSecretaryInboxMessage).toHaveBeenCalledTimes(3)
    expect(apiClient.secretaryTriage).not.toHaveBeenCalled()

    wrapper.unmount()
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

it('deduplicates optimistic secretary user message when stream insert arrives before inbox response', async () => {
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

    let onEvent: ((event: any) => void) | undefined
    ;(apiClient.attachSecretarySessionStream as any).mockImplementation(async (cb: any) => {
        onEvent = cb
    })

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
    await flushPromises()

    await wrapper.get('textarea').setValue('还有没完成的任务吗?')
    await wrapper.get('[data-testid="chat-send"]').trigger('click')
    await flushPromises()

    onEvent?.({
        type: 'msg',
        data: JSON.stringify({
            op: 'insert',
            id: '101',
            role: 'user',
            msg_type: 'text',
            delta: '还有没完成的任务吗?',
        }),
    })
    await flushPromises()

    resolveAppend?.({
        session_id: 's1',
        message_id: 101,
        ack_message_id: 0,
        ack_text: '',
    })
    await flushPromises()

    const userMessages = chat.messages.filter(
        (m: any) => m.role === 'user' && String(m.content) === '还有没完成的任务吗?'
    )
    expect(userMessages.length).toBe(1)
    expect(Number(userMessages[0]?.serverId || userMessages[0]?.id)).toBe(101)

    wrapper.unmount()
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
