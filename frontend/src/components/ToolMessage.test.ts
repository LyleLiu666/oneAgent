// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

vi.mock('@/api/client', () => ({
  approveToolApproval: vi.fn(),
  denyToolApproval: vi.fn(),
}))

it('renders a pending badge with response token count (best-effort)', async () => {
  const { default: ToolMessage } = await import('@/components/ToolMessage.vue')

  const wrapper = shallowMount(ToolMessage, {
    props: {
      message: {
        id: 1,
        role: 'assistant',
        type: 'tool_call',
        content: 'calling tool',
        createdAt: new Date(),
        tool: {
          toolCalls: [{ id: 'call_0', function: { name: 'bash', arguments: '{\"cmd\":\"sleep 1\"}' } }],
          content: 'calling tool',
        },
      },
      pending: true,
      progressTokens: 42,
    },
    global: {
      stubs: {
        TraceLog: true,
      },
    },
  })

  expect(wrapper.find('[data-testid="tool-pending-badge"]').exists()).toBe(true)
  expect(wrapper.find('[data-testid="tool-progress-tokens"]').text()).toContain('42')

  wrapper.unmount()
})

