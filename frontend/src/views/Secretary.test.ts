// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useChatStore } from '@/stores/chat'

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

it('renders ChatBox in secretary mode for /secretary', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: Secretary } = await import('@/views/Secretary.vue')
  const wrapper = shallowMount(Secretary, {
    global: {
      plugins: [pinia],
    },
  })

  const chatBox = wrapper.findComponent({ name: 'ChatBox' })
  expect(chatBox.exists()).toBe(true)
  expect((chatBox.props() as any).initialMode).toBe('secretary')

  wrapper.unmount()
})

it('hides low-frequency UI in secretary mode and keeps full mode discoverable', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const { default: ChatBox } = await import('@/components/ChatBox.vue')
  const wrapper = shallowMount(ChatBox, {
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

  const toggle = wrapper.get('[data-testid="chat-toggle-mode"]')
  expect(toggle.text()).toContain('进入完整模式')

  await toggle.trigger('click')
  await flushPromises()

  expect(wrapper.get('[data-testid="chat-toggle-mode"]').text()).toContain('进入秘书模式')
  expect(wrapper.find('[data-testid="chat-workspace-choose"]').exists()).toBe(true)
  expect(wrapper.find('task-queue-panel-stub').exists()).toBe(true)

  wrapper.unmount()
})

it('shows in-flight tool call progress in secretary mode', async () => {
  stubLocalStorage()

  const pinia = createPinia()
  setActivePinia(pinia)

  const chatStore = useChatStore()
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

  const { default: ChatBox } = await import('@/components/ChatBox.vue')
  const wrapper = shallowMount(ChatBox, {
    props: {
      initialMode: 'secretary',
    },
    global: {
      plugins: [pinia],
    },
  })

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
