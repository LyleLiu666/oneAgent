<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Send, RotateCcw, Loader2, ChevronDown, Copy, Check, Sparkles, Cpu, Folder } from 'lucide-vue-next'
import { marked } from 'marked'
import { useChatStore, type ChatMessage } from '@/stores/chat'
import { streamChat, getSessions, getSession, truncateSession, getModels, getTools, chooseWorkspaceDir, getConfig } from '@/api/client'
import { resolveWorkspaceChoice } from '@/lib/workspaceOnboarding'
import Welcome from './Welcome.vue'
import ChatHistoryList from './ChatHistoryList.vue'
import TraceLog from './TraceLog.vue'
import ThinkingProcess from './ThinkingProcess.vue'
import ToolMessage from './ToolMessage.vue'
import TaskQueuePanel from './TaskQueuePanel.vue'

const chatStore = useChatStore()

// Local state
const inputMessage = ref('')
const inputEl = ref<HTMLTextAreaElement | null>(null)
const messagesContainer = ref<HTMLElement | null>(null)
const showScrollButton = ref(false)
const copiedId = ref<number | null>(null)
const sessionsLoading = ref(false)
const loadingHistory = ref(false)
const showHistory = ref(true) // Control visibility of history panel
const modelsLoading = ref(false)
const models = ref<ModelOption[]>([])
const toolsLoading = ref(false)
const tools = ref<ToolOption[]>([])
const lastWorkspace = ref(localStorage.getItem('oneagent-workspace') || '')
const workspacePath = ref(lastWorkspace.value)
const workspaceChoosing = ref(false)
const workspaceChooseError = ref('')
const workspaceOnboardingDismissed = ref(false)
const sessionWorkspace = ref('')
const sessionPolicyID = ref('')
const sessionPolicyHash = ref('')
const sessionPolicyResolvedAt = ref('')

const runtimeConfigLoading = ref(false)
const runtimeConfigError = ref('')
const runtimeWarnings = ref<string[]>([])
const serverDefaultWorkspace = ref('')
const serverBaseURL = ref('')

const toolPickerOpen = ref(false)
const toolPickerEl = ref<HTMLElement | null>(null)

interface ModelOption {
  id: string
  name: string
  model: string
  isDefault: boolean
  enableKVCache: boolean
  provider?: {
    id: string
    name: string
    provider_type: string
    base_url: string
  }
}

interface ToolOption {
  id: string
  name: string
  description?: string
}

type ToolCallPayload = {
  protocol?: string
  content?: string
  llm_content?: string
  tool_calls?: any[]
}

type ToolResultPayload = {
  protocol?: string
  tool_call_id?: string
  name?: string
  arguments?: string
  content?: string
  results?: Array<{
    tool_name?: string
    tool_call_id?: string
    arguments?: string
    ok?: boolean
    output?: string
    error?: string
  }>
}

const safeJsonParse = <T,>(raw: any): T | undefined => {
  if (typeof raw !== 'string') return undefined
  const trimmed = raw.trim()
  if (!trimmed) return undefined
  try {
    return JSON.parse(trimmed) as T
  } catch {
    return undefined
  }
}

const normalizeTrace = (raw: any): string | undefined => {
  if (raw == null) return undefined
  if (typeof raw === 'string') {
    const trimmed = raw.trim()
    return trimmed ? raw : undefined
  }
  if (typeof raw === 'object') {
    const entries = (raw as any)?.entries
    if (Array.isArray(entries) && entries.length > 0) {
      return JSON.stringify(raw, null, 2)
    }
    return undefined
  }
  return String(raw)
}

// Computed
const workspaceOnboardingBlocking = computed(() => {
  const isNewSession = !chatStore.currentSessionId
  const isEmptyState = !loadingHistory.value && chatStore.messages.length === 0
  return (
    isNewSession &&
    isEmptyState &&
    !String(workspacePath.value || '').trim() &&
    !workspaceOnboardingDismissed.value
  )
})

const shortSessionPolicyHash = computed(() => {
  const h = String(sessionPolicyHash.value || '').trim()
  return h.length >= 8 ? h.slice(0, 8) : h
})

const canSend = computed(
  () =>
    inputMessage.value.trim() &&
    !chatStore.isLoading &&
    !loadingHistory.value &&
    !workspaceOnboardingBlocking.value
)

const currentSessionTitle = computed(() => chatStore.currentSession?.title || 'New chat')
const selectedModelId = computed({
  get: () => chatStore.currentModelId,
  set: (value: string) => chatStore.setCurrentModel(value),
})
const selectedToolIds = computed({
  get: () => chatStore.currentToolIds,
  set: (value: string[]) => chatStore.setCurrentTools(value),
})
const selectedToolProtocol = computed({
  get: () => chatStore.currentToolProtocol,
  set: (value: string) => chatStore.setCurrentToolProtocol(value),
})

const workspaceSourceLabel = computed(() => {
  const current = String(workspacePath.value || '').trim()
  if (!current) return ''
  if (String(sessionWorkspace.value || '').trim() === current) return 'session'
  if (String(lastWorkspace.value || '').trim() === current) return 'last used'
  if (String(serverDefaultWorkspace.value || '').trim() === current) return 'server default'
  return 'custom'
})

const workspacePromptError = computed(() => {
  const err = String(workspaceChooseError.value || runtimeConfigError.value || '').trim()
  return err
})

const toolSummary = computed(() => {
  const total = tools.value.length
  const selected = selectedToolIds.value.length
  if (total === 0) return 'Tools'
  if (selected === 0) return `Tools (0/${total})`
  if (selected === total) return `Tools (all)`
  return `Tools (${selected}/${total})`
})

// Methods
const handleDocumentClick = (event: MouseEvent) => {
  if (!toolPickerOpen.value) return
  const el = toolPickerEl.value
  if (!el) {
    toolPickerOpen.value = false
    return
  }
  if (event.target instanceof Node && el.contains(event.target)) return
  toolPickerOpen.value = false
}

const scrollToBottom = (smooth = true) => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTo({
        top: messagesContainer.value.scrollHeight,
        behavior: smooth ? 'smooth' : 'auto',
      })
    }
  })
}

const loadSessions = async () => {
  sessionsLoading.value = true
  try {
    const raw = await getSessions()
    const mapped = (Array.isArray(raw) ? raw : []).map((s: any) => ({
      id: String(s.id),
      title: String(s.title || ''),
      createdAt: new Date(s.created_at ?? s.createdAt ?? Date.now()),
      updatedAt: new Date(s.updated_at ?? s.updatedAt ?? Date.now()),
    }))
    chatStore.setSessions(mapped)
  } catch (error) {
    console.error('Failed to load sessions:', error)
    chatStore.setSessions([])
  } finally {
    sessionsLoading.value = false
  }
}

const loadModels = async () => {
  modelsLoading.value = true
  try {
    const raw = await getModels()
    const mapped = (Array.isArray(raw) ? raw : []).map((m: any) => ({
      id: String(m.id),
      name: String(m.name || ''),
      model: String(m.model || ''),
      isDefault: Boolean(m.is_default),
      enableKVCache: Boolean(m.enable_kv_cache),
      provider: m.provider
        ? {
            id: String(m.provider.id),
            name: String(m.provider.name || ''),
            provider_type: String(m.provider.provider_type || ''),
            base_url: String(m.provider.base_url || ''),
          }
        : undefined,
    }))
    models.value = mapped

    if (!selectedModelId.value) {
      const fallback = mapped.find((m: ModelOption) => m.isDefault) || mapped[0]
      if (fallback) {
        selectedModelId.value = fallback.id
      }
    } else if (mapped.length > 0) {
      const exists = mapped.some((m: ModelOption) => m.id === selectedModelId.value)
      if (!exists) {
        const fallback = mapped.find((m: ModelOption) => m.isDefault) || mapped[0]
        selectedModelId.value = fallback ? fallback.id : ''
      }
    }
  } catch (error) {
    console.error('Failed to load models:', error)
    models.value = []
  } finally {
    modelsLoading.value = false
  }
}

const loadTools = async () => {
  toolsLoading.value = true
  try {
    const raw = await getTools()
    const mapped = (Array.isArray(raw) ? raw : []).map((t: any) => ({
      id: String(t.id || ''),
      name: String(t.name || t.id || ''),
      description: t.description ? String(t.description) : undefined,
    }))
    tools.value = mapped

    // If no tools are currently selected (first time user), select all tools by default
    if (selectedToolIds.value.length === 0 && mapped.length > 0) {
      selectedToolIds.value = mapped.map((t: ToolOption) => t.id)
    } else if (selectedToolIds.value.length > 0) {
      // Filter out any tools that no longer exist
      const known = new Set(mapped.map((t: ToolOption) => t.id))
      const filtered = selectedToolIds.value.filter((id: string) => known.has(id))
      if (filtered.length !== selectedToolIds.value.length) {
        selectedToolIds.value = filtered
      }
    }
  } catch (error) {
    console.error('Failed to load tools:', error)
    tools.value = []
  } finally {
    toolsLoading.value = false
  }
}

const selectAllTools = () => {
  selectedToolIds.value = tools.value.map((t: ToolOption) => t.id)
}

const clearAllTools = () => {
  selectedToolIds.value = []
}

const applyWorkspaceDefaultsForNewSession = () => {
  if (chatStore.currentSessionId) return
  if (chatStore.messages.length > 0) return

  const resolved = resolveWorkspaceChoice({
    sessionWorkspace: '',
    localWorkspace: lastWorkspace.value,
    serverDefaultWorkspace: serverDefaultWorkspace.value,
  })

  if (!String(workspacePath.value || '').trim() && resolved.path) {
    workspacePath.value = resolved.path
  }
}

const loadRuntimeConfig = async () => {
  runtimeConfigLoading.value = true
  runtimeConfigError.value = ''
  runtimeWarnings.value = []
  try {
    const raw: any = await getConfig()
    serverDefaultWorkspace.value =
      typeof raw?.default_workspace === 'string' ? String(raw.default_workspace) : ''
    serverBaseURL.value = typeof raw?.base_url === 'string' ? String(raw.base_url) : ''
    runtimeWarnings.value = Array.isArray(raw?.warnings)
      ? raw.warnings.map((w: any) => String(w)).filter((w: string) => Boolean(w.trim()))
      : []
  } catch (error) {
    const msg = (error as any)?.data?.error || (error as any)?.message || 'Failed to load runtime config.'
    runtimeConfigError.value = String(msg)
    runtimeWarnings.value = []
    console.error('Failed to load runtime config:', error)
  } finally {
    runtimeConfigLoading.value = false
  }

  applyWorkspaceDefaultsForNewSession()
}

const loadSessionMessages = async (
  sessionId: string,
  showLoading = true,
  fallbackAssistantTrace?: string
) => {
  if (!sessionId) return
  if (showLoading) loadingHistory.value = true
  try {
    const raw: any = await getSession(sessionId)
    console.log('[ChatBox] loaded session raw:', raw)
    const sessionModelId = raw?.metadata?.model_id
    if (sessionModelId) {
      selectedModelId.value = String(sessionModelId)
    }
    const sessionToolIds = raw?.metadata?.tool_ids
    if (Array.isArray(sessionToolIds)) {
      selectedToolIds.value = sessionToolIds.map((id: any) => String(id))
    } else {
      selectedToolIds.value = []
    }
    const sessionToolProtocol = raw?.metadata?.tool_protocol
    if (sessionToolProtocol) {
      selectedToolProtocol.value = String(sessionToolProtocol).toLowerCase()
    } else {
      selectedToolProtocol.value = 'json'
    }
    const sessionSystemPrompt = raw?.metadata?.system_prompt
    const policyID = raw?.metadata?.policy_id
    const policyHash = raw?.metadata?.policy_hash
    const policyResolvedAt = raw?.metadata?.policy_resolved_at
    const sessionWorkspaceRaw = raw?.metadata?.workspace
    if (typeof sessionWorkspaceRaw === 'string' && sessionWorkspaceRaw.trim()) {
      const normalized = String(sessionWorkspaceRaw).trim()
      sessionWorkspace.value = normalized
      workspacePath.value = normalized
    } else {
      sessionWorkspace.value = ''
      workspacePath.value = ''
    }

    sessionPolicyID.value = typeof policyID === 'string' ? String(policyID) : ''
    sessionPolicyHash.value = typeof policyHash === 'string' ? String(policyHash) : ''
    sessionPolicyResolvedAt.value = typeof policyResolvedAt === 'string' ? String(policyResolvedAt) : ''
    const rawMessages = Array.isArray(raw?.messages) ? raw.messages : []
    const mapped: ChatMessage[] = rawMessages.map((m: any, idx: number) => {
        const msg = m ?? {}
        const serverId = Number(msg.id)
        const fallbackId = Date.now() + idx
        const rawType = String(msg.type || 'text')
        const type: ChatMessage['type'] =
          rawType === 'tool_call' ? 'tool_call' : rawType === 'tool_result' ? 'tool_result' : 'text'
        const rawContent = String(msg.content || '')

        const rawRole = String(msg.role || '')
        let role: ChatMessage['role'] =
          rawRole === 'user'
            ? 'user'
            : rawRole === 'assistant'
              ? 'assistant'
              : rawRole === 'tool'
                ? 'tool'
                : rawRole === 'system'
                  ? 'system'
                  : 'system'
        let content = rawContent
        let tool: ChatMessage['tool'] | undefined

        if (type === 'tool_call') {
          const payload = safeJsonParse<ToolCallPayload>(rawContent)
          if (payload) {
            content = String(payload.content || '')
            tool = {
              protocol: payload.protocol,
              llmContent: payload.llm_content,
              toolCalls: Array.isArray(payload.tool_calls) ? payload.tool_calls : undefined,
              content: payload.content,
            }
          }
        } else if (type === 'tool_result') {
          const payload = safeJsonParse<ToolResultPayload>(rawContent)
          if (payload) {
            const output = String(payload.content || '')
            let toolError: string | undefined
            const parsedOutput = safeJsonParse<any>(output)
            if (parsedOutput && typeof parsedOutput.error === 'string') {
              toolError = parsedOutput.error
            }
            tool = {
              protocol: payload.protocol,
              name: payload.name,
              toolCallId: payload.tool_call_id,
              arguments: payload.arguments,
              output,
              error: toolError,
              results: Array.isArray(payload.results) ? payload.results : undefined,
            }
            content = output
          }
        }

        return {
          id: Number.isFinite(serverId) && serverId > 0 ? serverId : fallbackId,
          serverId: Number.isFinite(serverId) && serverId > 0 ? serverId : undefined,
          role,
          type,
          rawRole,
          rawType,
          content,
          tool,
          createdAt: new Date(msg.created_at ?? msg.createdAt ?? Date.now()),
          trace: normalizeTrace(msg.trace),
          isStreaming: false,
        }
      })

    chatStore.setCurrentSession(sessionId)
    const withSystemPrompt: ChatMessage[] =
      typeof sessionSystemPrompt === 'string' && sessionSystemPrompt.trim()
        ? [
            {
              id: -1,
              role: 'system',
              type: 'text',
              rawRole: 'system',
              rawType: 'text',
              content: sessionSystemPrompt,
              createdAt: new Date(raw?.created_at ?? raw?.createdAt ?? Date.now()),
              isStreaming: false,
            },
            ...mapped,
          ]
        : mapped
    const withPlaceholders = withSystemPrompt
    const fallback = (fallbackAssistantTrace ?? '').trim()
    if (fallback) {
      for (let i = withPlaceholders.length - 1; i >= 0; i--) {
        if (withPlaceholders[i].role === 'assistant' && withPlaceholders[i].type === 'text') {
          if (!withPlaceholders[i].trace) {
            withPlaceholders[i].trace = fallbackAssistantTrace
          }
          break
        }
      }
    }
    chatStore.setMessages(withPlaceholders)
    scrollToBottom(false)
  } catch (error) {
    console.error('Failed to load session:', error)
    const status = (error as any)?.response?.status
    if (status === 404) {
      // Stale session id (e.g., DB reset / deleted session / switched account).
      chatStore.clearMessages()
      await loadSessions()
      return
    }
    chatStore.clearMessages()
  } finally {
    if (showLoading) loadingHistory.value = false
  }
}

const selectSession = async (sessionId: string) => {
  chatStore.setMessages([])
  await loadSessionMessages(sessionId)
}

const startNewSession = () => {
  inputMessage.value = ''
  chatStore.setLoading(false)
  chatStore.clearStreamingContent()
  chatStore.clearMessages()
  chatStore.setCurrentSession('')
  sessionWorkspace.value = ''
  sessionPolicyID.value = ''
  sessionPolicyHash.value = ''
  sessionPolicyResolvedAt.value = ''
  workspaceOnboardingDismissed.value = false
  applyWorkspaceDefaultsForNewSession()
}

watch(
  workspacePath,
  (value) => {
    const trimmed = String(value || '').trim()
    if (!trimmed) return
    lastWorkspace.value = trimmed
    localStorage.setItem('oneagent-workspace', trimmed)
  },
  { immediate: true }
)

const handleScroll = () => {
  if (messagesContainer.value) {
    const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value
    showScrollButton.value = scrollHeight - scrollTop - clientHeight > 100
  }
}

const chooseWorkspace = async () => {
  if (workspaceChoosing.value) return
  workspaceChoosing.value = true
  workspaceChooseError.value = ''
  try {
    const res: any = await chooseWorkspaceDir()
    if (res && typeof res === 'object' && res.canceled) return
    const path = res?.path
    if (typeof path === 'string' && path.trim()) {
      workspacePath.value = path
      workspaceOnboardingDismissed.value = true
      await nextTick()
      inputEl.value?.focus()
    }
  } catch (error) {
    const msg = (error as any)?.data?.error || (error as any)?.message || 'Failed to choose workspace folder.'
    workspaceChooseError.value = String(msg)
    console.error('Failed to choose workspace:', error)
  } finally {
    workspaceChoosing.value = false
  }
}

const skipWorkspaceOnboarding = async () => {
  workspaceOnboardingDismissed.value = true
  workspacePath.value = ''
  await nextTick()
  inputEl.value?.focus()
}

const sendChat = async (rawMessage: string) => {
  const message = rawMessage.trim()
  if (!message || chatStore.isLoading || loadingHistory.value) return

  // Add user message
  const userMessage: ChatMessage = {
    id: Date.now(),
    role: 'user',
    type: 'text',
    content: message,
    createdAt: new Date(),
  }
  chatStore.addMessage(userMessage)
  scrollToBottom()

  chatStore.setLoading(true)

  try {
    let sawMsgEvents = false
    let nextLocalId = Date.now()
    const streamIndex = new Map<string, number>()

    const normalizeRole = (raw: string | undefined): ChatMessage['role'] => {
      const role = String(raw || '').toLowerCase()
      if (role === 'user') return 'user'
      if (role === 'assistant') return 'assistant'
      if (role === 'tool') return 'tool'
      if (role === 'system') return 'system'
      return 'system'
    }

    const normalizeType = (raw: string | undefined): ChatMessage['type'] => {
      const type = String(raw || '').toLowerCase()
      if (type === 'tool_call') return 'tool_call'
      if (type === 'tool_result') return 'tool_result'
      return 'text'
    }

    type StreamMsgEvent = {
      op: 'start' | 'delta' | 'final' | 'insert'
      id: string
      role?: string
      msg_type?: string
      delta?: string
      error?: string
      tool_call?: ToolCallPayload
      tool_result?: ToolResultPayload
    }

    const findStreamingIndex = () => {
      for (let i = chatStore.messages.length - 1; i >= 0; i--) {
        if (chatStore.messages[i]?.isStreaming) return i
      }
      return -1
    }

    const ensureStreamMessage = (streamId: string, roleRaw?: string, typeRaw?: string) => {
      if (streamIndex.has(streamId)) return streamIndex.get(streamId)!

      nextLocalId += 1
      const role = normalizeRole(roleRaw)
      const type = normalizeType(typeRaw)
      const msg: ChatMessage = {
        id: nextLocalId,
        streamId,
        role,
        type,
        rawRole: roleRaw,
        rawType: typeRaw,
        content: '',
        createdAt: new Date(),
        isStreaming: true,
      }
      chatStore.addMessage(msg)
      const idx = chatStore.messages.length - 1
      streamIndex.set(streamId, idx)
      return idx
    }

    const applyToolCall = (message: ChatMessage, payload?: ToolCallPayload) => {
      if (!payload) return
      message.tool = {
        protocol: payload.protocol,
        llmContent: payload.llm_content,
        toolCalls: Array.isArray(payload.tool_calls) ? payload.tool_calls : undefined,
        content: payload.content,
      }
      if (typeof payload.content === 'string') {
        message.content = payload.content
      }
    }

    const applyToolResult = (message: ChatMessage, payload?: ToolResultPayload) => {
      if (!payload) return
      const output = String(payload.content || '')
      let toolError: string | undefined
      const parsedOutput = safeJsonParse<any>(output)
      if (parsedOutput && typeof parsedOutput.error === 'string') {
        toolError = parsedOutput.error
      }
      message.tool = {
        protocol: payload.protocol,
        name: payload.name,
        toolCallId: payload.tool_call_id,
        arguments: payload.arguments,
        output,
        error: toolError,
        results: Array.isArray(payload.results) ? payload.results : undefined,
      }
      message.content = output
    }

    await streamChat(
      message,
      chatStore.currentSessionId,
      chatStore.currentModelId,
      chatStore.currentToolIds,
      chatStore.currentToolProtocol,
      workspacePath.value,
      (event) => {
        if (event.type === 'session') {
          chatStore.setCurrentSession(event.data)
          const current = String(workspacePath.value || '').trim()
          if (current && !String(sessionWorkspace.value || '').trim()) {
            sessionWorkspace.value = current
          }
        } else if (event.type === 'msg') {
          sawMsgEvents = true
          const payload = safeJsonParse<StreamMsgEvent>(event.data)
          if (!payload || !payload.id || !payload.op) return

          if (payload.op === 'start') {
            ensureStreamMessage(payload.id, payload.role, payload.msg_type)
            scrollToBottom(false)
            return
          }

          if (payload.op === 'delta') {
            const idx = ensureStreamMessage(payload.id, payload.role, payload.msg_type)
            const msg = chatStore.messages[idx]
            if (msg && typeof payload.delta === 'string' && payload.delta) {
              msg.content += payload.delta
              msg.isStreaming = true
              scrollToBottom(false)
            }
            return
          }

          if (payload.op === 'final') {
            const idx = ensureStreamMessage(payload.id, payload.role, payload.msg_type)
            const msg = chatStore.messages[idx]
            if (!msg) return

            msg.isStreaming = false
            const finalType = normalizeType(payload.msg_type)
            msg.type = finalType
            msg.rawType = payload.msg_type
            if (payload.role) {
              msg.rawRole = payload.role
              msg.role = normalizeRole(payload.role)
            }

            if (finalType === 'tool_call') {
              applyToolCall(msg, payload.tool_call)
            }

            if (payload.error) {
              msg.content = (msg.content || '') + `\n\n[Error] ${payload.error}`
            }
            return
          }

          if (payload.op === 'insert') {
            nextLocalId += 1
            const role = normalizeRole(payload.role)
            const type = normalizeType(payload.msg_type)
            const msg: ChatMessage = {
              id: nextLocalId,
              streamId: payload.id,
              role,
              type,
              rawRole: payload.role,
              rawType: payload.msg_type,
              content: '',
              createdAt: new Date(),
              isStreaming: false,
            }

            if (type === 'tool_result') {
              applyToolResult(msg, payload.tool_result)
            }
            chatStore.addMessage(msg)
            scrollToBottom(false)
            return
          }
        } else if (event.type === 'trace') {
          const idx = findStreamingIndex()
          const target = idx >= 0 ? chatStore.messages[idx] : chatStore.messages[chatStore.messages.length - 1]
          if (target) target.trace = (target.trace || '') + event.data + '\n'
        } else if (event.type === 'usage') {
          try {
             // Expecting {"response_tokens": 123}
             const data = JSON.parse(event.data)
             if (typeof data.response_tokens === 'number') {
               for (let i = chatStore.messages.length - 1; i >= 0; i--) {
                 const msg = chatStore.messages[i]
                 if (msg.role === 'assistant' && msg.isStreaming) {
                   msg.responseTokens = data.response_tokens
                   break
                 }
               }
             }
          } catch(e) {
             console.warn('Failed to parse usage event:', e)
          }
        } else if (event.type === 'error') {
          const suffix = event.data ? `\n\n[Error] ${event.data}` : '\n\n[Error] Request failed.'
          const idx = findStreamingIndex()
          if (idx >= 0) {
            const msg = chatStore.messages[idx]
            msg.content = (msg.content || '') + suffix
            msg.isStreaming = false
          } else if (!sawMsgEvents) {
            chatStore.addMessage({
              id: Date.now(),
              role: 'system',
              type: 'text',
              content: suffix,
              createdAt: new Date(),
              isStreaming: false,
            })
          }
        } else if (event.type === 'done') {
          // Refresh sessions list to show new session or update time
          loadSessions()
        }
      },
      (error) => {
        console.error('Stream error:', error)
        const idx = findStreamingIndex()
        if (idx >= 0) {
          const msg = chatStore.messages[idx]
          msg.content = (msg.content || '') + '\n\n[Error] Stream failed.'
          msg.isStreaming = false
        }
      }
    )
  } catch (error) {
    console.error('Chat error:', error)
    chatStore.addMessage({
      id: Date.now(),
      role: 'system',
      type: 'text',
      content: 'Failed to send message. Please try again.',
      createdAt: new Date(),
      isStreaming: false,
    })
  } finally {
    chatStore.setLoading(false)
    if (chatStore.currentSessionId) {
      // We don't necessarily need to reload all messages, but getting the fresh session data is good practice
      // await loadSessionMessages(chatStore.currentSessionId) 
    }
    scrollToBottom()
  }
}

const sendMessage = async () => {
  if (!canSend.value) return
  const message = inputMessage.value.trim()
  inputMessage.value = ''
  if (inputEl.value) inputEl.value.style.height = ''
  await sendChat(message)
}

const retryMessage = async (assistantMessageIndex: number) => {
  if (chatStore.isLoading || loadingHistory.value) return

  const assistantMessage = chatStore.messages[assistantMessageIndex]
  if (!assistantMessage || assistantMessage.role !== 'assistant' || assistantMessage.isStreaming) return

  let userMessageIndex = -1
  for (let i = assistantMessageIndex - 1; i >= 0; i--) {
    const candidate = chatStore.messages[i]
    if (!candidate) continue
    if (candidate.role !== 'user') continue
    if (candidate.type !== 'text') continue
    userMessageIndex = i
    break
  }
  if (userMessageIndex < 0) return

  const userMessage = chatStore.messages[userMessageIndex]
  if (!userMessage) return

  const messageToRetry = userMessage.content
  const fromMessageId = userMessage.serverId

  // Keep history before the retried user message.
  chatStore.setMessages(chatStore.messages.slice(0, userMessageIndex))

  if (chatStore.currentSessionId && fromMessageId) {
    try {
      await truncateSession(chatStore.currentSessionId, fromMessageId)
    } catch (error) {
      console.error('Failed to truncate session:', error)
    }
  }

  await sendChat(messageToRetry)
}

const copyMessage = async (message: ChatMessage) => {
  try {
    await navigator.clipboard.writeText(message.content)
    copiedId.value = message.id
    setTimeout(() => {
      copiedId.value = null
    }, 2000)
  } catch (error) {
    console.error('Failed to copy:', error)
  }
}

const renderMarkdown = (content: string) => {
  const escaped = content.replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return marked(escaped, { breaks: true, gfm: true })
}

const parseMessageContent = (content: string) => {
  if (!content) return []
  
  const segments: Array<{ type: 'text' | 'think', content: string, isClosed?: boolean }> = []
  
  // Regex to match thinking tags: <think>, <thinking>, <reason>, <reasoning>
  // We need to capture the tag name to match the closing tag correctly
  // This regex matches:
  // 1. Start tag: <(think|thinking|reason|reasoning)>
  // 2. Content: lazy match until end tag or end of string
  // 3. End tag (optional for streaming): <\/\1>
  const regex = /(<(think|thinking|reason|reasoning)>)([\s\S]*?)(<\/\2>|$)/gi
  
  let lastIndex = 0
  let match
  
  while ((match = regex.exec(content)) !== null) {
    // Add text before the match
    if (match.index > lastIndex) {
      segments.push({
        type: 'text',
        content: content.slice(lastIndex, match.index)
      })
    }
    
    // Add thinking content
    // match[1] is start tag, match[2] is tag name, match[3] is content, match[4] is end tag
    segments.push({
      type: 'think',
      content: match[3],
      isClosed: !!match[4]
    })
    
    lastIndex = regex.lastIndex
  }
  
  // Add remaining text
  if (lastIndex < content.length) {
    segments.push({
      type: 'text',
      content: content.slice(lastIndex)
    })
  }
  
  return segments
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    if (event.isComposing) return
    event.preventDefault()
    sendMessage()
  }
}

const adjustTextareaHeight = () => {
  if (!inputEl.value) return
  inputEl.value.style.height = ''
  const styles = window.getComputedStyle(inputEl.value)
  const lineHeight = Number.parseFloat(styles.lineHeight) || 24
  const paddingTop = Number.parseFloat(styles.paddingTop) || 0
  const paddingBottom = Number.parseFloat(styles.paddingBottom) || 0
  const maxHeight = lineHeight * 5 + paddingTop + paddingBottom
  const nextHeight = Math.min(inputEl.value.scrollHeight, maxHeight)
  inputEl.value.style.height = `${nextHeight}px`
}

// Watch for new messages and scroll
watch(
  () => chatStore.messages.length,
  () => {
    scrollToBottom()
  }
)

watch(
  () => inputMessage.value,
  () => nextTick(adjustTextareaHeight)
)


const handleWelcomeSelect = (prompt: string) => {
  inputMessage.value = prompt
  sendMessage()
}

onMounted(async () => {
  document.addEventListener('click', handleDocumentClick)

  await loadRuntimeConfig()
  await loadTools()
  await loadModels()
  await loadSessions()
  const persistedId = chatStore.currentSessionId
  if (persistedId) {
    const exists = chatStore.sessions.some((s) => s.id === persistedId)
    if (exists) {
      await loadSessionMessages(persistedId)
    } else {
      // Avoid requesting a non-existent session on boot.
      chatStore.clearMessages()
    }
  }

  applyWorkspaceDefaultsForNewSession()
})

onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
})
</script>

<template>
  <div class="flex h-full bg-surface-950 overflow-hidden">
    
    <!-- History Sidebar -->
    <ChatHistoryList
      v-show="showHistory"
      :loading="sessionsLoading"
      @select="selectSession"
      @new="startNewSession"
    />

    <!-- Main Chat Area -->
    <div class="flex-1 flex flex-col h-full min-w-0 bg-surface-950 relative">
      
      <!-- Toggle History Button / Header -->
      <div v-if="!showHistory" class="absolute top-4 left-4 z-20">
         <!-- Optional: Add a button here to show history if hidden, 
              though currently I'm just keeping it always visible or using layout defaults.
              For now let's assume always visible on desktop or controlled by parent layout.
          -->
      </div>

      <div class="shrink-0 bg-surface-950/80 backdrop-blur session-header">
        <div class="max-w-4xl mx-auto px-4 py-3 flex items-center justify-between gap-3">
          <div class="min-w-0">
            <p class="text-[10px] uppercase tracking-[0.2em] text-surface-500">Session</p>
            <p class="text-sm text-surface-100 truncate">{{ currentSessionTitle }}</p>
            <p v-if="sessionPolicyHash" class="text-[11px] text-surface-500 truncate">
              policy={{ sessionPolicyID || 'unknown' }} · {{ shortSessionPolicyHash }}
              <a href="/governance/tools" class="ml-2 text-primary-400 hover:text-primary-300 underline">
                Tool Permissions
              </a>
              <span v-if="sessionPolicyResolvedAt" class="ml-2">· {{ sessionPolicyResolvedAt }}</span>
            </p>
          </div>
          <div class="flex items-center gap-2 flex-wrap justify-end">
            <Cpu class="w-4 h-4 text-surface-400" />
            <select
              v-model="selectedModelId"
              class="bg-surface-900 text-surface-200 text-xs sm:text-sm rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 max-w-[220px] truncate"
              :disabled="modelsLoading"
            >
              <option value="">{{ modelsLoading ? 'Loading...' : 'Default model' }}</option>
              <option v-for="model in models" :key="model.id" :value="model.id">
                {{ model.name || model.model }}{{ model.provider?.name ? ` · ${model.provider.name}` : '' }}
              </option>
            </select>
            <div class="flex items-center gap-2">
              <Folder class="w-4 h-4 text-surface-400" />
              <input
                v-model="workspacePath"
                class="bg-surface-900 text-surface-200 text-xs sm:text-sm rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 w-[220px] max-w-full"
                placeholder="Workspace path (server)"
                :disabled="Boolean(sessionWorkspace)"
                :title="
                  sessionWorkspace
                    ? 'Workspace is locked for this session. Start a new session to change it.'
                    : 'Workspace path on the server machine. File tools will be scoped to this directory.'
                "
              />
              <button
                type="button"
                :class="[
                  'bg-surface-900 text-surface-200 text-xs sm:text-sm rounded-lg px-2 py-1.5 border hover:bg-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 disabled:opacity-50 disabled:cursor-not-allowed',
                  workspaceChooseError ? 'border-red-500/60' : 'border-surface-800',
                ]"
                :disabled="workspaceChoosing || Boolean(sessionWorkspace)"
                :title="
                  sessionWorkspace
                    ? 'Workspace is locked for this session. Start a new session to change it.'
                    : workspaceChooseError || 'Choose workspace folder (server)'
                "
                @click="chooseWorkspace"
              >
                <Loader2 v-if="workspaceChoosing" class="w-4 h-4 animate-spin" />
                <span v-else>Browse</span>
              </button>
            </div>
            <div v-if="tools.length > 0" ref="toolPickerEl" class="relative flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-surface-400" />
              <button
                type="button"
                class="bg-surface-900 text-surface-200 text-xs sm:text-sm rounded-lg px-2 py-1.5 border border-surface-800 hover:bg-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                :disabled="toolsLoading"
                :title="toolSummary"
                @click.stop="toolPickerOpen = !toolPickerOpen"
              >
                {{ toolSummary }}
              </button>
              <div
                v-if="toolPickerOpen"
                class="absolute right-0 top-full mt-2 w-[320px] rounded-xl bg-surface-950 border border-surface-800 shadow-xl p-3 z-30"
                @click.stop
              >
                <div class="flex items-center justify-between gap-3">
                  <p class="text-[10px] uppercase tracking-[0.2em] text-surface-500">Protocol</p>
                  <select
                    v-model="selectedToolProtocol"
                    class="bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                    title="Tool protocol"
                  >
                    <option value="json">json tools</option>
                    <option value="xml">xml tools</option>
                  </select>
                </div>

                <div class="mt-3 max-h-60 overflow-y-auto space-y-2 pr-1">
                  <label
                    v-for="tool in tools"
                    :key="tool.id"
                    class="flex items-center gap-2 text-xs text-surface-300"
                  >
                    <input
                      v-model="selectedToolIds"
                      type="checkbox"
                      class="accent-primary-500"
                      :value="tool.id"
                      :disabled="toolsLoading"
                    />
                    <span class="truncate" :title="tool.description || tool.name">{{ tool.name }}</span>
                  </label>
                </div>

                <div class="mt-3 flex items-center justify-between gap-2">
                  <button
                    type="button"
                    class="text-xs text-surface-300 hover:text-surface-100"
                    @click="selectAllTools"
                  >
                    Select all
                  </button>
                  <button
                    type="button"
                    class="text-xs text-surface-300 hover:text-surface-100"
                    @click="clearAllTools"
                  >
                    Clear
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <TaskQueuePanel :workspace="workspacePath" :model-id="selectedModelId" />

      <!-- Messages area -->
      <div
        ref="messagesContainer"
        class="flex-1 overflow-y-auto px-4 py-6 space-y-6"
        @scroll="handleScroll"
      >
        <div v-if="loadingHistory" class="flex items-center justify-center py-6 text-surface-400">
          <Loader2 class="w-5 h-5 animate-spin" />
          <span class="ml-2 text-sm">Loading history...</span>
        </div>

        <!-- Empty state -->
        <Welcome
          v-if="!loadingHistory && chatStore.messages.length === 0"
          :workspace="workspacePath"
          :workspace-source="workspaceSourceLabel"
          :show-workspace-prompt="workspaceOnboardingBlocking"
          :workspace-choosing="workspaceChoosing"
          :workspace-choose-error="workspacePromptError"
          :runtime-warnings="runtimeWarnings"
          @choose-workspace="chooseWorkspace"
          @skip-workspace="skipWorkspaceOnboarding"
          @select="handleWelcomeSelect"
        />

        <!-- Messages -->
        <div
          v-for="(message, index) in chatStore.messages"
          :key="message.id"
          :class="[
            'flex gap-4 animate-fade-in',
            message.role === 'system'
              ? 'justify-center'
              : message.role === 'user' && message.type === 'text'
                ? 'justify-end'
                : 'justify-start',
          ]"
        >
          <!-- Tool message -->
          <div
            v-if="message.type === 'tool_call' || message.type === 'tool_result'"
            class="max-w-3xl"
          >
            <ToolMessage :message="message" />
          </div>

          <!-- System message -->
          <div v-else-if="message.role === 'system'" class="max-w-3xl w-full">
            <details class="rounded-xl bg-surface-900/30 backdrop-blur border border-surface-700/30">
              <summary class="cursor-pointer px-4 py-2 text-[11px] text-surface-500 select-none">
                system prompt
              </summary>
              <div class="px-4 pb-3">
                <div
                  class="prose prose-invert prose-sm max-w-none break-words text-surface-300"
                  v-html="renderMarkdown(message.content)"
                />
              </div>
            </details>
          </div>

          <!-- Assistant message -->
          <div
            v-else-if="message.role === 'assistant'"
            class="max-w-3xl flex gap-3"
          >
            <div
              class="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center flex-shrink-0"
            >
              <span class="text-white font-bold text-xs">AI</span>
            </div>
            <div class="flex-1 space-y-2 min-w-0">
              <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500 px-1">
                {{ message.rawRole || message.role }}
              </div>
              <div class="space-y-2">
                <!-- Streaming placeholder -->
                <div
                  v-if="message.isStreaming && (!message.content || !message.content.trim())"
                  class="flex items-center gap-3 px-4 py-3 rounded-2xl bg-surface-900/40 backdrop-blur border border-surface-700/30"
                >
                  <div class="relative">
                    <Sparkles class="w-4 h-4 text-primary-400 animate-pulse" />
                    <div class="absolute inset-0 bg-primary-400/20 blur-md rounded-full animate-pulse"></div>
                  </div>
                  <div class="flex items-center gap-1 text-sm font-medium text-surface-300">
                    <span>
                      Thinking
                      <span
                        v-if="message.responseTokens"
                        class="text-surface-400 font-normal text-xs"
                      >
                        ({{ message.responseTokens }} tokens)
                      </span>
                    </span>
                    <span class="flex gap-0.5 ml-0.5">
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.3s]"></span>
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.15s]"></span>
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce"></span>
                    </span>
                  </div>
                </div>

                <!-- Render segments as bubbles -->
                <template v-else>
                  <template v-for="(segment, sIdx) in parseMessageContent(message.content)" :key="sIdx">
                    <ThinkingProcess
                      v-if="segment.type === 'think'"
                      :content="segment.content"
                      :is-streaming="message.isStreaming && !segment.isClosed"
                    />
                    <div
                      v-else-if="segment.content && segment.content.trim()"
                      :class="['glass rounded-2xl px-4 py-3', sIdx === 0 ? 'rounded-tl-md' : '']"
                    >
                      <div
                        class="prose prose-invert prose-sm max-w-none break-words"
                        v-html="renderMarkdown(segment.content)"
                      />
                    </div>
                  </template>
                </template>

                <!-- Streaming cursor -->
                <span
                  v-if="message.isStreaming && message.content"
                  class="inline-block w-2 h-4 bg-primary-400 animate-pulse ml-1"
                />
              </div>
              <!-- Actions -->
              <div class="flex items-center gap-2 px-1">
                <button
                  @click="copyMessage(message)"
                  class="p-1.5 rounded-lg text-surface-500 hover:text-surface-300 hover:bg-surface-800 transition-colors"
                  title="Copy"
                >
                  <Check v-if="copiedId === message.id" class="w-4 h-4 text-green-400" />
                  <Copy v-else class="w-4 h-4" />
                </button>
                <button
                  v-if="!message.isStreaming"
                  @click="retryMessage(index)"
                  class="p-1.5 rounded-lg text-surface-500 hover:text-surface-300 hover:bg-surface-800 transition-colors"
                  title="Retry"
                >
                  <RotateCcw class="w-4 h-4" />
                </button>
                
                <!-- Helper for streaming stats if content is showing -->
                <div v-if="message.isStreaming && message.content && message.content.trim() && message.responseTokens" class="text-xs text-surface-500 ml-2">
                  {{ message.responseTokens }} tokens
                </div>
              </div>
              
              <!-- Trace Log Component -->
              <TraceLog :content="message.trace" />

            </div>
          </div>

          <div
            v-else
            class="max-w-3xl"
          >
            <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500 px-1 mb-1 text-right">
              {{ message.rawRole || message.role }}
            </div>
            <div
              class="bg-primary-600 text-white rounded-2xl rounded-tr-md px-4 py-3"
            >
              <p class="whitespace-pre-wrap break-words">{{ message.content }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Scroll to bottom button -->
      <transition name="fade">
        <button
          v-if="showScrollButton"
          @click="scrollToBottom()"
          class="absolute bottom-24 right-8 p-2 rounded-full bg-surface-800 text-surface-300 hover:bg-surface-700 shadow-lg transition-all z-10"
        >
          <ChevronDown class="w-5 h-5" />
        </button>
      </transition>

      <!-- Input area -->
      <div class="p-4 shrink-0">
        <div class="max-w-4xl mx-auto">
          <div class="relative flex items-end gap-3">
            <div class="flex-1 relative">
              <textarea
                ref="inputEl"
                v-model="inputMessage"
                @keydown="handleKeydown"
                :placeholder="
                  workspaceOnboardingBlocking
                    ? 'Choose a workspace (or skip) to start...'
                    : 'Type your message...'
                "
                rows="1"
                class="w-full px-4 py-3 pr-12 text-base leading-6 overflow-y-auto bg-surface-800 rounded-xl text-surface-50 placeholder-surface-500 resize-y focus:outline-none focus:ring-2 focus:ring-primary-500/50 transition-all border border-surface-700"
                :disabled="chatStore.isLoading || workspaceOnboardingBlocking"
              />
            </div>
            <button
              @click="sendMessage"
              :disabled="!canSend"
              :class="[
                'p-3 rounded-xl transition-all duration-200',
                canSend
                  ? 'bg-primary-600 text-white hover:bg-primary-500'
                  : 'bg-surface-800 text-surface-500 cursor-not-allowed',
              ]"
            >
              <Loader2
                v-if="chatStore.isLoading"
                class="w-5 h-5 animate-spin"
              />
              <Send v-else class="w-5 h-5" />
            </button>
          </div>
          <p class="text-xs text-surface-500 mt-2 text-center">
            Press Enter to send, Shift+Enter for new line
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
textarea {
  min-height: 48px;
  line-height: 1.5rem;
  max-height: calc(5 * 1.5rem + 1.5rem);
}

.prose :deep(pre) {
  background-color: var(--color-surface-900);
  border-radius: 0.5rem;
  padding: 0.75rem;
  overflow-x: auto;
}

.prose :deep(code) {
  background-color: var(--color-surface-800);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  color: #a5b4fc; /* primary-300 */
}

.prose :deep(pre code) {
  background-color: transparent;
  padding: 0;
}

/* Session header with gradient border instead of hard line */
.session-header {
  position: relative;
}

.session-header::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(
    to right,
    transparent 0%,
    rgba(99, 102, 241, 0.25) 15%,
    rgba(99, 102, 241, 0.12) 50%,
    rgba(99, 102, 241, 0.25) 85%,
    transparent 100%
  );
}

.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
  height: 4px;
}

.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background: var(--color-surface-700);
  border-radius: 2px;
}

.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: var(--color-surface-600);
}
</style>
