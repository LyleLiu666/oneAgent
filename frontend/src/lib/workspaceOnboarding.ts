export type WorkspaceSource = 'session' | 'local' | 'server' | 'none'

export interface WorkspaceChoice {
    path: string
    source: WorkspaceSource
}

export function resolveWorkspaceChoice(input: {
    sessionWorkspace?: string
    localWorkspace?: string
    serverDefaultWorkspace?: string
}): WorkspaceChoice {
    const session = String(input.sessionWorkspace || '').trim()
    if (session) return { path: session, source: 'session' }

    const local = String(input.localWorkspace || '').trim()
    if (local) return { path: local, source: 'local' }

    const server = String(input.serverDefaultWorkspace || '').trim()
    if (server) return { path: server, source: 'server' }

    return { path: '', source: 'none' }
}

