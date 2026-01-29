// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  streamChat: vi.fn(),
  getConfig: vi.fn(async () => ({ default_workspace: '', base_url: '', warnings: [] })),
  getSessions: vi.fn(async () => []),
  getSession: vi.fn(async () => ({ messages: [], metadata: {} })),
  truncateSession: vi.fn(),
  getModels: vi.fn(async () => []),
  getTools: vi.fn(async () => []),
  chooseWorkspaceDir: vi.fn(async () => ({ path: '/tmp/workspace' })),
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
