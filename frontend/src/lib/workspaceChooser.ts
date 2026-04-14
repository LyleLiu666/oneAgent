import type { RuntimeConfig } from '@/api/client'

export const WORKSPACE_CHOOSER_UNSUPPORTED_HINT =
  '当前服务端环境不支持原生文件夹选择，请手动填写服务端工作区路径。'

export const WORKSPACE_CHOOSER_UNKNOWN_HINT =
  '暂时无法确认服务端是否支持原生文件夹选择，请手动填写服务端工作区路径，或刷新后重试。'

export type WorkspaceChooserStrategy = 'native' | 'browser' | 'unsupported'

export type WorkspaceChooserSupport = {
  supported: boolean
  strategy: WorkspaceChooserStrategy
  hint: string
}

const normalizeChooserHint = (raw: unknown, fallback: string): string => {
  const hint = String(raw ?? '').trim()
  return hint || fallback
}

export const resolveWorkspaceChooserSupport = (
  raw?: Partial<RuntimeConfig> | null,
): WorkspaceChooserSupport => {
  const nativeSupported = raw?.workspace_chooser_supported !== false
  if (nativeSupported) {
    return {
      supported: true,
      strategy: 'native',
      hint: '',
    }
  }

  const browserSupported = raw?.workspace_browser_supported === true
  if (browserSupported) {
    return {
      supported: true,
      strategy: 'browser',
      hint: '',
    }
  }

  const fallbackHint = raw?.workspace_browser_reason || raw?.workspace_chooser_reason
  return {
    supported: false,
    strategy: 'unsupported',
    hint: normalizeChooserHint(fallbackHint, WORKSPACE_CHOOSER_UNSUPPORTED_HINT),
  }
}

export const unknownWorkspaceChooserSupport = (): WorkspaceChooserSupport => ({
  supported: false,
  strategy: 'unsupported',
  hint: WORKSPACE_CHOOSER_UNKNOWN_HINT,
})
