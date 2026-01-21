<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { Send, RotateCcw, Loader2, ChevronDown, Copy, Check, Sparkles, Cpu } from 'lucide-vue-next'
import { marked } from 'marked'
import { useChatStore, type ChatMessage } from '@/stores/chat'
import { streamChat, getSessions, getSession, truncateSession, getModels, getTools } from '@/api/client'
import Welcome from './Welcome.vue'
import ChatHistoryList from './ChatHistoryList.vue'
import TraceLog from './TraceLog.vue'

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

// Computed
const canSend = computed(
  () => inputMessage.value.trim() && !chatStore.isLoading && !loadingHistory.value
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

// Methods
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

    if (selectedToolIds.value.length > 0) {
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

const loadSessionMessages = async (sessionId: string, showLoading = true) => {
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
    const rawMessages = Array.isArray(raw?.messages) ? raw.messages : []
    const mapped: ChatMessage[] = rawMessages
      .filter((m: any) => m?.role === 'user' || m?.role === 'assistant')
      .map((m: any, idx: number) => {
        const serverId = Number(m.id)
        const fallbackId = Date.now() + idx
        return {
          id: Number.isFinite(serverId) && serverId > 0 ? serverId : fallbackId,
          serverId: Number.isFinite(serverId) && serverId > 0 ? serverId : undefined,
          role: m.role,
          content: String(m.content || ''),
          createdAt: new Date(m.created_at ?? m.createdAt ?? Date.now()),
          trace:
            m.trace == null
              ? undefined
              : typeof m.trace === 'string'
                ? m.trace
                : JSON.stringify(m.trace, null, 2),
          isStreaming: false,
        }
      })

    chatStore.setCurrentSession(sessionId)
    chatStore.setMessages(insertAssistantPlaceholders(mapped))
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

const insertAssistantPlaceholders = (messages: ChatMessage[]) => {
  if (messages.length === 0) return messages

  const out: ChatMessage[] = []
  const placeholderBase = Date.now() + 1000000
  let placeholderIndex = 0

  for (let i = 0; i < messages.length; i++) {
    const message = messages[i]
    out.push(message)

    if (message.role !== 'user') {
      continue
    }

    const next = messages[i + 1]
    if (next && next.role === 'assistant') {
      continue
    }

    out.push({
      id: placeholderBase + placeholderIndex,
      role: 'assistant',
      content: '',
      createdAt: message.createdAt,
      isStreaming: false,
    })
    placeholderIndex += 1
  }

  return out
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
}

const handleScroll = () => {
  if (messagesContainer.value) {
    const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value
    showScrollButton.value = scrollHeight - scrollTop - clientHeight > 100
  }
}

const sendChat = async (rawMessage: string) => {
  const message = rawMessage.trim()
  if (!message || chatStore.isLoading || loadingHistory.value) return

  // Add user message
  const userMessage: ChatMessage = {
    id: Date.now(),
    role: 'user',
    content: message,
    createdAt: new Date(),
  }
  chatStore.addMessage(userMessage)
  scrollToBottom()

  // Add placeholder for assistant
  const assistantMessage: ChatMessage = {
    id: Date.now() + 1,
    role: 'assistant',
    content: '',
    createdAt: new Date(),
    isStreaming: true,
  }
  chatStore.addMessage(assistantMessage)
  chatStore.setLoading(true)
  chatStore.clearStreamingContent()
  let streamHadError = false

  try {
    await streamChat(
      message,
      chatStore.currentSessionId,
      chatStore.currentModelId,
      chatStore.currentToolIds,
      (event) => {
        if (event.type === 'session') {
          chatStore.setCurrentSession(event.data)
        } else if (event.type === 'content') {
          chatStore.appendStreamingContent(event.data)
          chatStore.updateLastMessage(chatStore.streamingContent, true)
          scrollToBottom(false)
        } else if (event.type === 'trace') {
          const lastMsg = chatStore.messages[chatStore.messages.length - 1]
          if (lastMsg) {
            lastMsg.trace = (lastMsg.trace || '') + event.data + '\n'
          }
        } else if (event.type === 'error') {
          streamHadError = true
          const suffix = event.data ? `\n\n[Error] ${event.data}` : '\n\n[Error] Request failed.'
          chatStore.appendStreamingContent(suffix)
          chatStore.updateLastMessage(chatStore.streamingContent, false)
        } else if (event.type === 'done') {
          chatStore.updateLastMessage(chatStore.streamingContent, false)
          chatStore.clearStreamingContent()
          // Refresh sessions list to show new session or update time
          loadSessions()
          // Reload session messages to ensure trace and metadata are up to date
          if (chatStore.currentSessionId && !streamHadError) {
            loadSessionMessages(chatStore.currentSessionId, false)
          }
        }
      },
      (error) => {
        console.error('Stream error:', error)
        chatStore.updateLastMessage('An error occurred. Please try again.', false)
      }
    )
  } catch (error) {
    console.error('Chat error:', error)
    chatStore.updateLastMessage('Failed to send message. Please try again.', false)
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

  const userMessageIndex = assistantMessageIndex - 1
  if (userMessageIndex < 0) return

  const userMessage = chatStore.messages[userMessageIndex]
  if (!userMessage || userMessage.role !== 'user') return

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
  return marked(content, { breaks: true, gfm: true })
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
            <div v-if="tools.length > 0" class="flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-surface-400" />
              <div class="flex items-center gap-2">
                <label
                  v-for="tool in tools"
                  :key="tool.id"
                  class="flex items-center gap-1 text-[11px] text-surface-300"
                >
                  <input
                    v-model="selectedToolIds"
                    type="checkbox"
                    class="accent-primary-500"
                    :value="tool.id"
                    :disabled="toolsLoading"
                  />
                  <span class="max-w-[110px] truncate" :title="tool.description || tool.name">
                    {{ tool.name }}
                  </span>
                </label>
              </div>
            </div>
          </div>
        </div>
      </div>

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
          @select="handleWelcomeSelect"
        />

        <!-- Messages -->
        <div
          v-for="(message, index) in chatStore.messages"
          :key="message.id"
          :class="[
            'flex gap-4 animate-fade-in',
            message.role === 'user' ? 'justify-end' : 'justify-start',
          ]"
        >
          <!-- Assistant message -->
          <div
            v-if="message.role === 'assistant'"
            class="max-w-3xl flex gap-3"
          >
            <div
              class="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center flex-shrink-0"
            >
              <span class="text-white font-bold text-xs">AI</span>
            </div>
            <div class="flex-1 space-y-2 min-w-0">
              <div class="glass rounded-2xl rounded-tl-md px-4 py-3">
                <!-- Thinking indicator -->
                <div
                  v-if="message.isStreaming && !message.content"
                  class="flex items-center gap-3 px-4 py-3 rounded-2xl bg-surface-900/40 backdrop-blur border border-surface-700/30"
                >
                  <div class="relative">
                    <Sparkles class="w-4 h-4 text-primary-400 animate-pulse" />
                    <div class="absolute inset-0 bg-primary-400/20 blur-md rounded-full animate-pulse"></div>
                  </div>
                  <div class="flex items-center gap-1 text-sm font-medium text-surface-300">
                    <span>Thinking</span>
                    <span class="flex gap-0.5 ml-0.5">
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.3s]"></span>
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.15s]"></span>
                      <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce"></span>
                    </span>
                  </div>
                </div>
                <!-- Content -->
                <div
                  v-else
                  class="prose prose-invert prose-sm max-w-none break-words"
                  v-html="renderMarkdown(message.content)"
                />
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
              </div>
              
              <!-- Trace Log Component -->
              <TraceLog :content="message.trace" />

            </div>
          </div>

          <!-- User message -->
          <div
            v-else
            class="max-w-3xl"
          >
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
                placeholder="Type your message..."
                rows="1"
                class="w-full px-4 py-3 pr-12 text-base leading-6 overflow-y-auto bg-surface-800 rounded-xl text-surface-50 placeholder-surface-500 resize-y focus:outline-none focus:ring-2 focus:ring-primary-500/50 transition-all border border-surface-700"
                :disabled="chatStore.isLoading"
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
</style>
