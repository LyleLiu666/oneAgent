import { describe, expect, it } from 'vitest'

import {
  WORKSPACE_CHOOSER_UNKNOWN_HINT,
  WORKSPACE_CHOOSER_UNSUPPORTED_HINT,
  resolveWorkspaceChooserSupport,
  unknownWorkspaceChooserSupport,
} from '@/lib/workspaceChooser'

describe('workspace chooser support', () => {
  it('prefers the server-provided reason when chooser is unsupported', () => {
    const resolved = resolveWorkspaceChooserSupport({
      workspace_chooser_supported: false,
      workspace_chooser_reason: '服务端不支持原生选择器，请手动填写路径。',
    })

    expect(resolved).toEqual({
      supported: false,
      strategy: 'unsupported',
      hint: '服务端不支持原生选择器，请手动填写路径。',
    })
  })

  it('falls back to the default unsupported hint when no reason is provided', () => {
    const resolved = resolveWorkspaceChooserSupport({
      workspace_chooser_supported: false,
    })

    expect(resolved).toEqual({
      supported: false,
      strategy: 'unsupported',
      hint: WORKSPACE_CHOOSER_UNSUPPORTED_HINT,
    })
  })

  it('returns a conservative unknown-state hint when config loading fails', () => {
    expect(unknownWorkspaceChooserSupport()).toEqual({
      supported: false,
      strategy: 'unsupported',
      hint: WORKSPACE_CHOOSER_UNKNOWN_HINT,
    })
  })

  it('prefers the web browser strategy when native chooser is unavailable but web browse is supported', () => {
    const resolved = resolveWorkspaceChooserSupport({
      workspace_chooser_supported: false,
      workspace_browser_supported: true,
    })

    expect(resolved).toEqual({
      supported: true,
      strategy: 'browser',
      hint: '',
    })
  })
})
