// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import * as apiClient from '@/api/client'

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

const makeLocalStorage = () => {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  }
}

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/api/client', () => ({
  streamChat: vi.fn(),
  attachChatStream: vi.fn(),
  attachSecretarySessionStream: vi.fn(),
  stopSessionStream: vi.fn(),
  getSessions: vi.fn(async () => []),
  getSession: vi.fn(async () => ({ id: 's-full', messages: [], metadata: {} })),
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
      {
        mode: 'sandbox_coding',
        label: '沙箱开发',
        description: '允许改代码和跑测试，但尽量放在隔离环境里。',
        risk_level: 'medium',
        available: true,
        current: false,
        recommended: true,
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
  getConfig: vi.fn(async () => ({ default_workspace: '', base_url: '', warnings: [] })),
  createTask: vi.fn(),
  secretaryHandoff: vi.fn(),
  resumeTask: vi.fn(),
  appendSecretaryInboxMessage: vi.fn(),
  secretaryTriage: vi.fn(),
  getSecretaryState: vi.fn(async () => ({ session_id: 's1', cursor_message_id: 0, triage_runs: [] })),
  setSecretaryRecoveryFocus: vi.fn(),
  getSecretarySession: vi.fn(async () => ({ id: 's1', title: 'Secretary', messages: [], metadata: {} })),
  resetSecretarySession: vi.fn(async () => ({})),

  // Task queue (imported by child components; not mounted in this test).
  listTasks: vi.fn(async () => []),
  getTask: vi.fn(),
  getTaskEvents: vi.fn(async () => []),
  cancelTask: vi.fn(),
  getLedgerStatusToday: vi.fn(async () => ({
    day_key: '2026-02-01',
    digest_exists: false,
    learning_job_status: 'none',
    sop_proposed_count: 0,
  })),
  getTaskQueueGovernance: vi.fn(),
  getTaskQueueGovernanceSnapshot: vi.fn(),
  updateTaskQueueWorkspacePolicy: vi.fn(),
  createTaskQueueSchedule: vi.fn(),
  getTaskAttemptArtifact: vi.fn(),
  listTaskAttemptFiles: vi.fn(),
  readTaskAttemptFileSnapshot: vi.fn(),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

it('hands off input to task queue and appends a persisted receipt (SecretaryChatBox)', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  ;(apiClient.secretaryHandoff as any).mockResolvedValueOnce({
    session_id: 's1',
    task_id: 't1',
    user_message_id: 11,
    assistant_message_id: 12,
    receipt_text: '已交给后台处理，交付物会出现在交付区。',
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { useSecretaryChatStore } = await import('@/stores/secretaryChat')
  const chat = useSecretaryChatStore()
  chat.setMessages([])

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
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

  expect(chat.messages.some((m: any) => m.role === 'user' && m.content === 'do the thing')).toBe(true)
  expect(chat.messages.some((m: any) => m.role === 'assistant' && String(m.content).includes('交付区'))).toBe(true)
  expect(chat.messages.some((m: any) => m.role === 'system')).toBe(false)

  expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')

  wrapper.unmount()
})

it('shows a permission suggestion card when the request obviously needs write/run capability', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  await wrapper.get('textarea').setValue('请帮我修改代码并运行测试')
  await flushPromises()

  expect(wrapper.get('[data-testid="chat-permission-suggestion"]').text()).toContain('沙箱开发')
})

it('closes reset modal after confirming reset', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  await wrapper.get('[data-testid="secretary-reset-context"]').trigger('click')
  await flushPromises()

  expect(wrapper.find('[data-testid="secretary-reset-modal"]').exists()).toBe(true)

  await wrapper.get('[data-testid="secretary-reset-confirm"]').trigger('click')
  await flushPromises()
  await flushPromises()

  expect(wrapper.find('[data-testid="secretary-reset-modal"]').exists()).toBe(false)

  wrapper.unmount()
})

it('binds workspace when user replies with an absolute path for a pending workspace question', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

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

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  await wrapper.get('textarea').setValue('1. /tmp/workspace')
  await wrapper.get('[data-testid="chat-send"]').trigger('click')
  await flushPromises()

  const args = (apiClient.appendSecretaryInboxMessage as any).mock.calls[0]?.[0]
  expect(args).toBeTruthy()
  expect(args.workspace).toBe('/tmp/workspace')

  wrapper.unmount()
})

it('binds workspace when user replies with a one-segment absolute root path', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

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

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
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

it('shows setup guidance when no model is configured in secretary mode', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  expect(wrapper.find('[data-testid="secretary-model-setup-banner"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="secretary-open-settings"]').attributes('href')).toBe('/settings')

  wrapper.unmount()
})

it('binds a browser-selected root path even when the path is only one segment deep', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())
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

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: { initialMode: 'secretary' },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  expect(wrapper.find('[data-testid="chat-workspace-choose"]').exists()).toBe(false)

  const browseButton = wrapper.get('[data-testid="secretary-browse-workspace-reply"]')
  expect(browseButton.attributes('disabled')).toBeUndefined()
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

it('shows a secretary-visible error when choosing a workspace reply fails', async () => {
  vi.stubGlobal('localStorage', makeLocalStorage())
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

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
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

it('deduplicates optimistic user message when stream insert arrives before inbox response', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  let onEvent: ((event: any) => void) | undefined
  ;(apiClient.attachSecretarySessionStream as any).mockImplementation(async (cb: any) => {
    onEvent = cb
  })

  let resolveAppend: ((value: any) => void) | undefined
  const appendPromise = new Promise((resolve) => {
    resolveAppend = resolve
  })
  ;(apiClient.appendSecretaryInboxMessage as any).mockImplementation(() => appendPromise)

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { useSecretaryChatStore } = await import('@/stores/secretaryChat')
  const chat = useSecretaryChatStore()
  chat.setMessages([])

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
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

it('does not inject chat messages when receiving task-completed (SecretaryChatBox)', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { useSecretaryChatStore } = await import('@/stores/secretaryChat')
  const chat = useSecretaryChatStore()
  chat.setMessages([])

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: { initialMode: 'secretary' },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  const deliverables = wrapper.findComponent({ name: 'SecretaryTaskDeliverables' })
  expect(deliverables.exists()).toBe(true)
  const before = chat.messages.length
  deliverables.vm.$emit('task-completed', {
    taskId: 't1',
    attemptId: 'a1',
    title: 'task1',
    status: 'failed',
    summary: 'WEATHER_RESULT',
    continuing: true,
  })
  await flushPromises()

  expect(chat.messages.length).toBe(before)
  expect(chat.messages.some((m: any) => String(m.content || '').includes('WEATHER_RESULT'))).toBe(false)

  wrapper.unmount()
})

it('shows recovery only in task panel (no chat injection) (SecretaryChatBox)', async () => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  })

  const pinia = createPinia()
  setActivePinia(pinia)

  const { useUIStore } = await import('@/stores/ui')
  useUIStore().setMode('secretary')

  const { useSecretaryChatStore } = await import('@/stores/secretaryChat')
  const chat = useSecretaryChatStore()
  chat.setMessages([])

  const { default: SecretaryChatBox } = await import('@/components/SecretaryChatBox.vue')
  const wrapper = shallowMount(SecretaryChatBox, {
    props: { initialMode: 'secretary' },
    global: {
      plugins: [pinia],
    },
  })

  await flushPromises()
  await flushPromises()

  expect(String(wrapper.get('[data-testid="secretary-task-panel"]').attributes('style') || '')).toContain('display: none')

  const deliverables = wrapper.findComponent({ name: 'SecretaryTaskDeliverables' })
  expect(deliverables.exists()).toBe(true)
  deliverables.vm.$emit('recovery-snapshot', [
    {
      taskId: 't1',
      attemptId: 'a1',
      title: 'task1',
      status: 'failed',
      summary: 's1',
      observer: { next_steps: 'n1', questions_for_user: ['q1'] },
    },
  ])
  await flushPromises()

  expect(chat.messages.length).toBe(0)
  expect(String(wrapper.get('[data-testid="secretary-task-panel"]').attributes('style') || '')).not.toContain('display: none')

  wrapper.unmount()
})
