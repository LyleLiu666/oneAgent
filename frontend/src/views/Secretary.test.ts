// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useChatStore } from '@/stores/chat'
import { useSecretaryChatStore } from '@/stores/secretaryChat'
import * as apiClient from '@/api/client'

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('@/api/client', () => ({
  streamChat: vi.fn(),
  attachSecretarySessionStream: vi.fn(),
  appendSecretaryInboxMessage: vi.fn(),
  secretaryTriage: vi.fn(),
  getSecretaryState: vi.fn(async () => ({ session_id: 's1', cursor_message_id: 0, triage_runs: [] })),
  setSecretaryRecoveryFocus: vi.fn(),
  getSecretarySession: vi.fn(async () => ({ id: 's1', messages: [], metadata: {} })),
  resetSecretarySession: vi.fn(async () => ({ session_id: 's1' })),
  getConfig: vi.fn(async () => ({ default_workspace: '', base_url: '', warnings: [] })),
  getSessions: vi.fn(async () => []),
  getSession: vi.fn(async () => ({ messages: [], metadata: {} })),
  truncateSession: vi.fn(),
  getModels: vi.fn(async () => []),
  getTools: vi.fn(async () => []),
  chooseWorkspaceDir: vi.fn(async () => ({ path: '/tmp/workspace' })),
  browseWorkspaceDir: vi.fn(),
  createWorkspaceDir: vi.fn(),
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
  routerPush.mockReset()
})

const stubLocalStorage = () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })
  // jsdom doesn't implement scrollTo on HTMLElement; ChatBox uses it for auto-scrolling.
  if (typeof (HTMLElement.prototype as any).scrollTo !== 'function') {
    ;(HTMLElement.prototype as any).scrollTo = () => {}
  }
  return store
}

it('renders SecretaryChatBox in secretary mode for /secretary', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: Secretary } = await import('@/views/Secretary.vue')
  const wrapper = shallowMount(Secretary, {
    global: {
      plugins: [pinia],
    },
  })

  const secretaryChatBox = wrapper.findComponent({ name: 'SecretaryChatBox' })
  expect(secretaryChatBox.exists()).toBe(true)
  expect((secretaryChatBox.props() as any).initialMode).toBe('secretary')

  wrapper.unmount()
})

it('hides low-frequency UI in secretary mode and keeps full mode discoverable', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)
  routerPush.mockReset()

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()

  expect(wrapper.find('chat-history-list-stub').exists()).toBe(false)
  expect(wrapper.find('task-queue-panel-stub').exists()).toBe(false)
  expect(wrapper.find('[data-testid="chat-workspace-choose"]').exists()).toBe(false)

  const taskPanel = wrapper.get('[data-testid="secretary-task-panel"]')
  expect(String(taskPanel.attributes('style') || '')).toContain('display: none')
  const taskPanelToggle = wrapper.get('[data-testid="secretary-toggle-task-panel"]')
  await taskPanelToggle.trigger('click')
  await flushPromises()
  expect(String(wrapper.get('[data-testid="secretary-task-panel"]').attributes('style') || '')).not.toContain('display: none')

  const toggle = wrapper.get('[data-testid="chat-toggle-mode"]')
  expect(toggle.text()).toContain('进入完整模式')

  await toggle.trigger('click')
  await flushPromises()

  const { useUIStore } = await import('@/stores/ui')
  const ui = useUIStore()
  expect(ui.mode).toBe('full')
  expect(routerPush).toHaveBeenCalledWith('/chat')

  wrapper.unmount()
})

it('shows pending triage questions in a modal (SecretaryChatBox)', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  let onEvent: ((event: any) => void) | undefined
  ;(apiClient.attachSecretarySessionStream as any).mockImplementation(async (cb: any) => {
    onEvent = cb
  })

  ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
    session_id: 's1',
    message_id: 1,
    ack_message_id: 0,
    ack_text: '',
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

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
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

  const appendArgs = (apiClient.appendSecretaryInboxMessage as any).mock.calls[0]?.[0]
  expect(appendArgs).toBeTruthy()
  expect(appendArgs).not.toHaveProperty('session_id')

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

it('shows in-flight tool call progress in secretary mode', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()

  const chatStore = useSecretaryChatStore()
  chatStore.setMessages([
    {
      id: 1,
      role: 'user',
      type: 'text',
      content: 'run something',
      createdAt: new Date(),
    },
    {
      id: 2,
      role: 'assistant',
      type: 'tool_call',
      content: 'calling tool',
      createdAt: new Date(),
      tool: {
        toolCalls: [{ id: 'call_0', function: { name: 'bash', arguments: '{\"cmd\":\"sleep 1\"}' } }],
        content: 'calling tool',
      },
    },
  ])

  await flushPromises()

  expect(wrapper.find('[data-testid=\"chat-secretary-tool-progress\"]').exists()).toBe(true)

  // Once a matching tool_result appears after the tool_call, the in-flight indicator should disappear.
  chatStore.setMessages([
    ...chatStore.messages,
    {
      id: 3,
      role: 'tool',
      type: 'tool_result',
      content: JSON.stringify({
        protocol: 'json',
        tool_call_id: 'call_0',
        name: 'bash',
        arguments: '{\"cmd\":\"sleep 1\"}',
        content: 'ok',
      }),
      createdAt: new Date(),
      tool: {
        toolCallId: 'call_0',
        name: 'bash',
        arguments: '{\"cmd\":\"sleep 1\"}',
        output: 'ok',
      },
    },
  ])

  await flushPromises()
  expect(wrapper.find('[data-testid=\"chat-secretary-tool-progress\"]').exists()).toBe(false)

  wrapper.unmount()
})

it('can reset secretary context via a confirmation modal', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()

  const chatStore = useSecretaryChatStore()
  chatStore.setMessages([
    {
      id: 1,
      role: 'user',
      type: 'text',
      content: 'hi',
      createdAt: new Date(),
      isStreaming: false,
    } as any,
  ])

  await flushPromises()
  expect(chatStore.messages.length).toBe(1)

  await wrapper.get('[data-testid="secretary-reset-context"]').trigger('click')
  await flushPromises()
  expect(wrapper.find('[data-testid="secretary-reset-modal"]').exists()).toBe(true)

  await wrapper.get('[data-testid="secretary-reset-confirm"]').trigger('click')
  await flushPromises()

  expect((apiClient as any).resetSecretarySession).toHaveBeenCalledTimes(1)
  expect(chatStore.messages.length).toBe(0)

  wrapper.unmount()
})

it('does not render chat retry controls in secretary mode (prevents /api/chat + /truncate calls)', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()

  const chatStore = useSecretaryChatStore()
  chatStore.setMessages([
    {
      id: 1,
      role: 'user',
      type: 'text',
      content: 'hi',
      createdAt: new Date(),
      isStreaming: false,
    },
    {
      id: 2,
      role: 'assistant',
      type: 'text',
      content: 'ok',
      createdAt: new Date(),
      isStreaming: false,
    },
  ])

  await flushPromises()

  expect(wrapper.find('button[title="重试"]').exists()).toBe(false)
  expect(apiClient.truncateSession).not.toHaveBeenCalled()
  expect(apiClient.streamChat).not.toHaveBeenCalled()

  wrapper.unmount()
})

it('does not override assistant chat session when chatting with secretary', async () => {
  vi.useFakeTimers()

  const flush = async () => {
    const p = flushPromises()
    vi.advanceTimersByTime(0)
    await p
  }

  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const assistantStore = useChatStore()
  assistantStore.setCurrentSession('a1')

  const secretaryStore = useSecretaryChatStore()
  secretaryStore.setCurrentSession('s1')

  ;(apiClient.appendSecretaryInboxMessage as any).mockResolvedValueOnce({
    session_id: 's1',
    message_id: 1,
    ack_message_id: 0,
    ack_text: '',
  })

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

  await flush()

  await wrapper.get('textarea').setValue('你好')
  await wrapper.get('[data-testid="chat-send"]').trigger('click')
  await flush()

  expect(assistantStore.currentSessionId).toBe('a1')
  expect(secretaryStore.currentSessionId).toBe('s1')

  wrapper.unmount()
  vi.useRealTimers()
})
