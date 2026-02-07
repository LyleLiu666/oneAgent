// @vitest-environment jsdom

import { beforeEach, expect, it } from 'vitest'

import type { ChatMessage } from '@/stores/chat'
import {
  appendSecretaryLocalMessage,
  clearSecretaryLocalMessages,
  loadSecretaryLocalMessages,
  mergeSecretaryLocalMessages,
  nextSecretaryLocalMessageID,
} from '@/lib/secretaryLocalMessages'

beforeEach(() => {
  localStorage.removeItem('oneagent-secretary-local-messages-v1')
  localStorage.removeItem('oneagent-secretary-local-message-next-id-v1')
})

it('persists and loads secretary local messages', () => {
  const sessionId = 'secretary-session-1'
  const createdAt = new Date('2026-02-07T00:00:00.000Z')

  const saved = appendSecretaryLocalMessage(sessionId, {
    role: 'assistant',
    type: 'text',
    content: '交付已更新。',
    createdAt,
  })

  expect(saved).not.toBeNull()
  expect(saved!.id).toBeLessThan(0)

  const loaded = loadSecretaryLocalMessages(sessionId)
  expect(loaded.length).toBe(1)
  expect(loaded[0].content).toBe('交付已更新。')
  expect(loaded[0].createdAt.getTime()).toBe(createdAt.getTime())
})

it('generates unique negative ids', () => {
  const a = nextSecretaryLocalMessageID()
  const b = nextSecretaryLocalMessageID()
  expect(a).toBeLessThan(0)
  expect(b).toBeLessThan(0)
  expect(b).toBe(a - 1)
})

it('merges and sorts by createdAt, keeping system prompt first', () => {
  const sessionId = 'secretary-session-2'

  appendSecretaryLocalMessage(sessionId, {
    role: 'assistant',
    type: 'text',
    content: 'A',
    createdAt: new Date('2026-02-07T00:00:15.000Z'),
  })

  const base: ChatMessage[] = [
    {
      id: -1,
      role: 'system',
      type: 'text',
      content: 'SYS',
      createdAt: new Date('2026-02-07T00:00:30.000Z'),
      isStreaming: false,
    },
    {
      id: 1,
      role: 'user',
      type: 'text',
      content: 'U',
      createdAt: new Date('2026-02-07T00:00:20.000Z'),
      isStreaming: false,
    },
  ]

  const merged = mergeSecretaryLocalMessages(sessionId, base)
  expect(merged.map((m) => m.content)).toEqual(['SYS', 'A', 'U'])
})

it('clears messages per session', () => {
  appendSecretaryLocalMessage('s1', {
    role: 'assistant',
    type: 'text',
    content: 'one',
    createdAt: new Date(),
  })
  appendSecretaryLocalMessage('s2', {
    role: 'assistant',
    type: 'text',
    content: 'two',
    createdAt: new Date(),
  })

  clearSecretaryLocalMessages('s1')
  expect(loadSecretaryLocalMessages('s1').length).toBe(0)
  expect(loadSecretaryLocalMessages('s2').length).toBe(1)
})
