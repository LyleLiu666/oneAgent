import type { ChatMessage } from '@/stores/chat'

const STORAGE_KEY = 'oneagent-secretary-local-messages-v1'
const NEXT_ID_KEY = 'oneagent-secretary-local-message-next-id-v1'
const MAX_MESSAGES_PER_SESSION = 200

type PersistedMessage = {
  id: number
  role: ChatMessage['role']
  type: ChatMessage['type']
  content: string
  createdAt: string
}

type StorageShape = Record<string, PersistedMessage[]>

const normalizeSessionID = (raw: any) => String(raw || '').trim()

const safeParseStorage = (raw: string | null): StorageShape => {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed as StorageShape
  } catch {
    return {}
  }
}

const safeDate = (raw: any): Date => {
  const d = raw instanceof Date ? raw : new Date(raw)
  if (Number.isFinite(d.getTime())) return d
  return new Date()
}

const isLegacyTaskCompletionNotification = (msg: PersistedMessage): boolean => {
  const role = String((msg as any)?.role || '').trim()
  const type = String((msg as any)?.type || '').trim()
  if (role !== 'assistant' || type !== 'text') return false
  const content = String((msg as any)?.content || '')
  const normalized = content.trimStart()
  if (
    !normalized.startsWith('后台任务已完成') &&
    !normalized.startsWith('后台任务已结束(')
  ) {
    return false
  }
  return normalized.includes('交付已更新')
}

export function nextSecretaryLocalMessageID(): number {
  try {
    const raw = localStorage.getItem(NEXT_ID_KEY)
    const parsed = Number(raw)
    // Reserve -1 for system prompt (used by session loader).
    const current = Number.isFinite(parsed) ? Math.trunc(parsed) : -2
    const normalized = current >= -1 ? -2 : current
    localStorage.setItem(NEXT_ID_KEY, String(normalized-1))
    return normalized
  } catch {
    // Best-effort: still ensure negative IDs.
    return -Math.max(2, Date.now())
  }
}

export function loadSecretaryLocalMessages(sessionId: string): ChatMessage[] {
  const sid = normalizeSessionID(sessionId)
  if (!sid) return []

  try {
    const storage = safeParseStorage(localStorage.getItem(STORAGE_KEY))
    const list = storage[sid]
    if (!Array.isArray(list) || list.length === 0) return []

    const filtered = list.filter((item) => !isLegacyTaskCompletionNotification(item))
    if (filtered.length !== list.length) {
      try {
        storage[sid] = filtered
        localStorage.setItem(STORAGE_KEY, JSON.stringify(storage))
      } catch {
        // ignore
      }
    }

    const out: ChatMessage[] = []
    for (const item of filtered) {
      const id = Number((item as any)?.id)
      if (!Number.isFinite(id)) continue
      const roleRaw = String((item as any)?.role || '')
      const typeRaw = String((item as any)?.type || '')
      const content = String((item as any)?.content || '')
      const createdAt = safeDate((item as any)?.createdAt)

      const role: ChatMessage['role'] =
        roleRaw === 'user'
          ? 'user'
          : roleRaw === 'assistant'
            ? 'assistant'
            : roleRaw === 'tool'
              ? 'tool'
              : 'system'
      const type: ChatMessage['type'] =
        typeRaw === 'tool_call' ? 'tool_call' : typeRaw === 'tool_result' ? 'tool_result' : 'text'

      out.push({
        id,
        role,
        type,
        content,
        createdAt,
        isStreaming: false,
      })
    }
    return out
  } catch {
    return []
  }
}

export function appendSecretaryLocalMessage(
  sessionId: string,
  payload: Pick<ChatMessage, 'role' | 'type' | 'content'> & { createdAt?: Date }
): ChatMessage | null {
  const sid = normalizeSessionID(sessionId)
  if (!sid) return null

  const createdAt = safeDate(payload.createdAt ?? new Date())
  const msg: ChatMessage = {
    id: nextSecretaryLocalMessageID(),
    role: payload.role,
    type: payload.type,
    content: String(payload.content || ''),
    createdAt,
    isStreaming: false,
  }

  try {
    const storage = safeParseStorage(localStorage.getItem(STORAGE_KEY))
    const existing = Array.isArray(storage[sid]) ? storage[sid] : []

    existing.push({
      id: msg.id,
      role: msg.role,
      type: msg.type,
      content: msg.content,
      createdAt: createdAt.toISOString(),
    })

    storage[sid] = existing.slice(-MAX_MESSAGES_PER_SESSION)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(storage))
  } catch {
    // ignore
  }

  return msg
}

export function clearSecretaryLocalMessages(sessionId: string): void {
  const sid = normalizeSessionID(sessionId)
  if (!sid) return
  try {
    const storage = safeParseStorage(localStorage.getItem(STORAGE_KEY))
    if (!(sid in storage)) return
    delete storage[sid]
    localStorage.setItem(STORAGE_KEY, JSON.stringify(storage))
  } catch {
    // ignore
  }
}

const safeTime = (d: any): number => {
  const t = d instanceof Date ? d.getTime() : new Date(d).getTime()
  return Number.isFinite(t) ? t : 0
}

export function mergeSecretaryLocalMessages(sessionId: string, base: ChatMessage[]): ChatMessage[] {
  const sid = normalizeSessionID(sessionId)
  if (!sid) return base

  const local = loadSecretaryLocalMessages(sid)
  if (local.length === 0) return base

  const seen = new Set<number>()
  const combined: ChatMessage[] = []
  for (const m of base) {
    if (!Number.isFinite(m?.id)) continue
    if (seen.has(m.id)) continue
    seen.add(m.id)
    combined.push(m)
  }
  for (const m of local) {
    if (!Number.isFinite(m?.id)) continue
    if (seen.has(m.id)) continue
    seen.add(m.id)
    combined.push(m)
  }

  return combined
    .map((msg, idx) => ({
      msg,
      idx,
      ts: safeTime((msg as any)?.createdAt),
      isSystemPrompt: msg?.id === -1 && msg?.role === 'system',
    }))
    .sort((a, b) => {
      if (a.isSystemPrompt !== b.isSystemPrompt) return a.isSystemPrompt ? -1 : 1
      if (a.ts !== b.ts) return a.ts - b.ts
      return a.idx - b.idx
    })
    .map((x) => x.msg)
}
