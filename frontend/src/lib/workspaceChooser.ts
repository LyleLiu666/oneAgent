import type { RuntimeConfig } from '@/api/client'

export const WORKSPACE_CHOOSER_UNSUPPORTED_HINT =
  '当前服务端环境不支持原生文件夹选择，请手动填写服务端工作区路径。'

export type WorkspaceChooserSupport = {
  supported: boolean
  hint: string
}

export const resolveWorkspaceChooserSupport = (
  raw?: Partial<RuntimeConfig> | null,
): WorkspaceChooserSupport => {
  const supported = raw?.workspace_chooser_supported !== false
  return {
    supported,
    hint: supported ? '' : WORKSPACE_CHOOSER_UNSUPPORTED_HINT,
  }
}
