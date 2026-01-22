import { ofetch } from 'ofetch'
import { useAuthStore } from '@/stores/auth'

const API_BASE = import.meta.env.VITE_API_BASE || ''

// Create base fetch instance
export const api = ofetch.create({
    baseURL: API_BASE,
    onRequest({ options }) {
        const authStore = useAuthStore()
        if (authStore.token) {
            options.headers = new Headers(options.headers)
            options.headers.set('Authorization', `Bearer ${authStore.token}`)
        }
    },
    onResponseError({ response }) {
        if (response.status === 401) {
            const authStore = useAuthStore()
            authStore.clearAuth()
            window.location.href = '/login'
        }
    },
})

// Stream event interface
export interface StreamEvent {
    type: 'session' | 'content' | 'trace' | 'done' | 'error' | 'usage'
    data: string
}

function concatBytes(a: Uint8Array, b: Uint8Array): Uint8Array {
    if (a.length === 0) return b
    if (b.length === 0) return a
    const out = new Uint8Array(a.length + b.length)
    out.set(a, 0)
    out.set(b, a.length)
    return out
}

function utf8ExpectedLength(firstByte: number): number {
    if ((firstByte & 0x80) === 0) return 1
    if ((firstByte & 0xe0) === 0xc0) return 2
    if ((firstByte & 0xf0) === 0xe0) return 3
    if ((firstByte & 0xf8) === 0xf0) return 4
    return 0
}

function splitIncompleteUtf8Tail(bytes: Uint8Array): { complete: Uint8Array; remainder: Uint8Array } {
    if (bytes.length === 0) {
        return { complete: bytes, remainder: bytes }
    }

    const last = bytes[bytes.length - 1]
    if (last < 0x80) {
        return { complete: bytes, remainder: new Uint8Array() }
    }

    const lookback = Math.min(4, bytes.length)
    for (let i = 1; i <= lookback; i++) {
        const start = bytes.length - i
        const b = bytes[start]

        // Continuation bytes are 10xxxxxx; skip them.
        if ((b & 0xc0) === 0x80) continue

        const expected = utf8ExpectedLength(b)
        if (expected === 0 || expected === 1) {
            return { complete: bytes, remainder: new Uint8Array() }
        }

        const available = bytes.length - start
        if (available >= expected) {
            return { complete: bytes, remainder: new Uint8Array() }
        }

        return {
            complete: bytes.slice(0, start),
            remainder: bytes.slice(start),
        }
    }

    return { complete: bytes, remainder: new Uint8Array() }
}

/**
 * Stream chat messages from the API
 */
export async function streamChat(
    message: string,
    sessionId: string = '',
    modelId: string = '',
    toolIds: string[] = [],
    toolProtocol: string = 'json',
    onEvent: (event: StreamEvent) => void,
    onError: (error: Error) => void
): Promise<void> {
    const authStore = useAuthStore()

    try {
        const response = await fetch(`${API_BASE}/api/chat`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'text/event-stream',
                Authorization: `Bearer ${authStore.token}`,
            },
            body: JSON.stringify({
                message,
                session_id: sessionId,
                model_id: modelId,
                tool_ids: toolIds,
                tool_protocol: toolProtocol,
            }),
        })

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`)
        }

        const reader = response.body?.getReader()
        if (!reader) {
            throw new Error('No response body')
        }

        const decoder = new TextDecoder()
        let pendingBytes: Uint8Array<ArrayBufferLike> = new Uint8Array()
        let buffer = ''

        while (true) {
            const { done, value } = await reader.read()
            if (done) break

            const combined = concatBytes(pendingBytes, value)
            const { complete, remainder } = splitIncompleteUtf8Tail(combined)
            pendingBytes = remainder

            buffer += decoder.decode(complete)
            // Normalize CRLF -> LF so we can reliably split SSE events.
            buffer = buffer.replace(/\r\n/g, '\n')

            // Process complete events
            let delimiterIndex = buffer.indexOf('\n\n')
            while (delimiterIndex !== -1) {
                const rawEvent = buffer.slice(0, delimiterIndex)
                buffer = buffer.slice(delimiterIndex + 2)

                const dataLines = rawEvent
                    .split('\n')
                    .filter((l) => l.startsWith('data:'))
                    .map((l) => l.replace(/^data:\s?/, ''))

                if (dataLines.length > 0) {
                    const dataText = dataLines.join('\n')
                    try {
                        const data = JSON.parse(dataText)
                        onEvent(data as StreamEvent)
                    } catch (e) {
                        console.error('Failed to parse SSE event:', e)
                    }
                }

                delimiterIndex = buffer.indexOf('\n\n')
            }
        }

        if (pendingBytes.length > 0) {
            buffer += decoder.decode(pendingBytes)
        }

        // Process any remaining buffer
        buffer = buffer.replace(/\r\n/g, '\n')
        const delimiterIndex = buffer.indexOf('\n\n')
        const lastEvent = (delimiterIndex === -1 ? buffer : buffer.slice(0, delimiterIndex)).trim()
        if (lastEvent) {
            const dataLines = lastEvent
                .split('\n')
                .filter((l) => l.startsWith('data:'))
                .map((l) => l.replace(/^data:\s?/, ''))
            if (dataLines.length > 0) {
                try {
                    const data = JSON.parse(dataLines.join('\n'))
                    onEvent(data as StreamEvent)
                } catch {
                    // Ignore incomplete / non-JSON tail.
                }
            }
        }
    } catch (error) {
        onError(error as Error)
    }
}

/**
 * Get user info
 */
export async function getUserInfo() {
    return api('/api/me')
}

/**
 * Get chat sessions
 */
export async function getSessions() {
    return api('/api/sessions')
}

/**
 * Get a specific session with messages
 */
export async function getSession(sessionId: string) {
    return api(`/api/sessions/${sessionId}`)
}

/**
 * Delete a session
 */
export async function deleteSession(sessionId: string) {
    return api(`/api/sessions/${sessionId}`, { method: 'DELETE' })
}

/**
 * Truncate a session's messages from a given message id (inclusive).
 * Used for "retry/regenerate" flows.
 */
export async function truncateSession(sessionId: string, fromMessageId: number) {
    return api(`/api/sessions/${sessionId}/truncate`, {
        method: 'POST',
        body: { from_message_id: fromMessageId },
    })
}

export async function getProviders() {
    return api('/api/llm/providers')
}

export async function createProvider(payload: {
    name: string
    provider_type: string
    base_url: string
    api_key: string
}) {
    return api('/api/llm/providers', { method: 'POST', body: payload })
}

export async function updateProvider(
    providerId: string,
    payload: {
        name?: string
        provider_type?: string
        base_url?: string
        api_key?: string
    }
) {
    return api(`/api/llm/providers/${providerId}`, { method: 'PUT', body: payload })
}

export async function deleteProvider(providerId: string) {
    return api(`/api/llm/providers/${providerId}`, { method: 'DELETE' })
}

export async function getModels(providerId?: string) {
    const query = providerId ? `?provider_id=${encodeURIComponent(providerId)}` : ''
    return api(`/api/llm/models${query}`)
}

export async function getTools() {
    return api('/api/tools')
}

export async function createModel(payload: {
    provider_id: string
    name: string
    model: string
    is_default?: boolean
    enable_kv_cache?: boolean
}) {
    return api('/api/llm/models', { method: 'POST', body: payload })
}

export async function updateModel(
    modelId: string,
    payload: {
        name?: string
        model?: string
        is_default?: boolean
        enable_kv_cache?: boolean
    }
) {
    return api(`/api/llm/models/${modelId}`, { method: 'PUT', body: payload })
}

export async function deleteModel(modelId: string) {
    return api(`/api/llm/models/${modelId}`, { method: 'DELETE' })
}

// ============================================================================
// Bash Command Runner
// ============================================================================

export interface BashRunResponse {
    shell: string
    stdout: string
    stderr: string
    exit_code: number
    duration_ms: number
    timed_out: boolean
    stdout_truncated: boolean
    stderr_truncated: boolean
}

export async function runBashCommand(command: string, timeoutMs?: number): Promise<BashRunResponse> {
    const body: Record<string, any> = { command }
    if (typeof timeoutMs === 'number' && timeoutMs > 0) {
        body.timeout_ms = timeoutMs
    }
    return api('/api/bash', { method: 'POST', body })
}

// ============================================================================
// Bocha Search Services
// ============================================================================

export interface BochaSettings {
    has_bocha_api_key: boolean
}

export async function getBochaSettings(): Promise<BochaSettings> {
    return api('/api/bocha/settings')
}

export async function updateBochaSettings(payload: { bocha_api_key?: string }) {
    return api('/api/bocha/settings', { method: 'PUT', body: payload })
}

export interface BochaSearchOptions {
    summary?: boolean
    freshness?: 'noLimit' | 'oneDay' | 'oneWeek' | 'oneMonth' | 'oneYear'
    count?: number
}

export async function bochaSearch(query: string, options?: BochaSearchOptions) {
    return api('/api/bocha/search', {
        method: 'POST',
        body: { query, ...options },
    })
}
