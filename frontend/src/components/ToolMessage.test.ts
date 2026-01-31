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

it('shows approval actions for approval_required tool_result even when collapsed', async () => {
  const { default: ToolMessage } = await import('@/components/ToolMessage.vue')

  const payload = JSON.stringify({
    error: 'approval_required',
    approval_required: true,
    approval_id: 'a_1',
    tool_id: 'bash',
    scope_id: 's_1',
    arguments: '{"command":"rm -rf a"}',
  })

  const wrapper = shallowMount(ToolMessage, {
    props: {
      message: {
        id: 2,
        role: 'tool',
        type: 'tool_result',
        content: payload,
        createdAt: new Date(),
        tool: {
          name: 'bash',
          output: payload,
        },
      },
    },
  })

  expect(wrapper.find('[data-testid="tool-approval-approve"]').exists()).toBe(true)
  expect(wrapper.find('[data-testid="tool-approval-deny"]').exists()).toBe(true)

  wrapper.unmount()
})
