import { expect, it, vi } from 'vitest'

vi.mock('@/stores/auth', () => ({
    useAuthStore: () => ({
        token: 'test-token',
        clearAuth: vi.fn(),
    }),
}))

import { streamChat, type StreamEvent } from './client'

function makeStream(chunks: Uint8Array[]) {
    let index = 0
    return new ReadableStream<Uint8Array>({
        pull(controller) {
            if (index >= chunks.length) {
                controller.close()
                return
            }
            controller.enqueue(chunks[index++])
        },
    })
}

it('decodes UTF-8 correctly when a multibyte rune is split across chunks', async () => {
    const encoder = new TextEncoder()

    const prefix = encoder.encode('data: {"type":"content","data":"')
    const rune = encoder.encode('个') // 3 bytes in UTF-8
    const suffix = encoder.encode('"}\n\n')

    const stream = makeStream([
        new Uint8Array([...prefix, rune[0]]),
        new Uint8Array([rune[1]]),
        new Uint8Array([rune[2], ...suffix]),
    ])

    const fetchMock = vi.fn(async () => new Response(stream, { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const events: StreamEvent[] = []
    await streamChat(
        'hi',
        '',
        '',
        [],
        'json',
        (event) => events.push(event),
        (err) => {
            throw err
        }
    )

    expect(events).toHaveLength(1)
    expect(events[0].type).toBe('content')
    expect(events[0].data).toBe('个')
    expect(events[0].data).not.toContain('\uFFFD')
})
