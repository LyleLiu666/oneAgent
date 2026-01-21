import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface ChatMessage {
    id: number
    serverId?: number
    role: 'user' | 'assistant'
    content: string
    trace?: string
    createdAt: Date
    isStreaming?: boolean
    responseTokens?: number
}

export interface ChatSession {
    id: string
    title: string
    createdAt: Date
    updatedAt: Date
}

export const useChatStore = defineStore(
    'chat',
    () => {
        // State
        const sessions = ref<ChatSession[]>([])
        const currentSessionId = ref<string>('')
        const currentModelId = ref<string>('')
        const currentToolIds = ref<string[]>([])
        const messages = ref<ChatMessage[]>([])
        const isLoading = ref(false)
        const streamingContent = ref('')

        // Getters
        const currentSession = computed(() =>
            sessions.value.find((s) => s.id === currentSessionId.value)
        )

        const sortedSessions = computed(() =>
            [...sessions.value].sort(
                (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
            )
        )

        // Actions
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
            messages,
            isLoading,
            streamingContent,
            currentSession,
            sortedSessions,
            setSessions,
            setCurrentSession,
            setCurrentModel,
            setCurrentTools,
            setMessages,
            addMessage,
            updateLastMessage,
            setLoading,
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
            key: 'chat',
            storage: localStorage,
            paths: ['currentSessionId', 'currentModelId', 'currentToolIds'],
        },
    }
)
