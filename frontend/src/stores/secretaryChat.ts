import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

import type { ChatMessage, ChatSession } from './chat'

export const useSecretaryChatStore = defineStore(
  'secretaryChat',
  () => {
    const sessions = ref<ChatSession[]>([])
    const currentSessionId = ref<string>('')
    const currentModelId = ref<string>('')
    const currentToolIds = ref<string[]>([])
    const currentToolProtocol = ref<string>('xml')
    const messages = ref<ChatMessage[]>([])
    const isLoading = ref(false)
    const streamingContent = ref('')
    const lastResponseTokens = ref<number | undefined>(undefined)

    const currentSession = computed(() =>
      sessions.value.find((s) => s.id === currentSessionId.value)
    )

    const sortedSessions = computed(() =>
      [...sessions.value].sort(
        (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      )
    )

    function setSessions(newSessions: ChatSession[]) {
      sessions.value = newSessions
    }

    function setCurrentSession(sessionId: string) {
      currentSessionId.value = sessionId
    }

    function setCurrentModel(modelId: string) {
      currentModelId.value = modelId
    }

    function setCurrentTools(toolIds: string[]) {
      currentToolIds.value = [...toolIds]
    }

    function setCurrentToolProtocol(protocol: string) {
      currentToolProtocol.value = protocol
    }

    function setMessages(newMessages: ChatMessage[]) {
      messages.value = newMessages
    }

    function addMessage(message: ChatMessage) {
      messages.value.push(message)
    }

    function updateLastMessage(content: string, isStreaming: boolean = true, responseTokens?: number) {
      const lastMessage = messages.value[messages.value.length - 1]
      if (lastMessage && lastMessage.role === 'assistant') {
        lastMessage.content = content
        lastMessage.isStreaming = isStreaming
        if (responseTokens !== undefined) {
          lastMessage.responseTokens = responseTokens
        }
      }
    }

    function setLoading(loading: boolean) {
      isLoading.value = loading
    }

    function setLastResponseTokens(tokens: number | undefined) {
      lastResponseTokens.value = tokens
    }

    function setStreamingContent(content: string) {
      streamingContent.value = content
    }

    function appendStreamingContent(chunk: string) {
      streamingContent.value += chunk
    }

    function clearStreamingContent() {
      streamingContent.value = ''
    }

    function removeLastMessage() {
      if (messages.value.length > 0) {
        messages.value.pop()
      }
    }

    function clearMessages() {
      messages.value = []
      currentSessionId.value = ''
    }

    return {
      sessions,
      currentSessionId,
      currentModelId,
      currentToolIds,
      currentToolProtocol,
      messages,
      isLoading,
      streamingContent,
      lastResponseTokens,
      currentSession,
      sortedSessions,
      setSessions,
      setCurrentSession,
      setCurrentModel,
      setCurrentTools,
      setCurrentToolProtocol,
      setMessages,
      addMessage,
      updateLastMessage,
      setLoading,
      setLastResponseTokens,
      setStreamingContent,
      appendStreamingContent,
      clearStreamingContent,
      removeLastMessage,
      clearMessages,
    }
  },
  {
    // @ts-ignore
    persist: {
      key: 'secretary-chat',
      storage: localStorage,
      paths: ['currentSessionId', 'currentModelId', 'currentToolIds', 'currentToolProtocol'],
    },
  }
)

