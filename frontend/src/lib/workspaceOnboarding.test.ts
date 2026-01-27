import { describe, expect, it } from 'vitest'

import { resolveWorkspaceChoice } from './workspaceOnboarding'

describe('resolveWorkspaceChoice', () => {
    it('prefers session workspace', () => {
        const got = resolveWorkspaceChoice({
            sessionWorkspace: '/ws/session',
            localWorkspace: '/ws/local',
            serverDefaultWorkspace: '/ws/server',
        })
        expect(got).toEqual({ path: '/ws/session', source: 'session' })
    })

    it('prefers local workspace over server default', () => {
        const got = resolveWorkspaceChoice({
            sessionWorkspace: '',
            localWorkspace: '/ws/local',
            serverDefaultWorkspace: '/ws/server',
        })
        expect(got).toEqual({ path: '/ws/local', source: 'local' })
    })

    it('falls back to server default workspace', () => {
        const got = resolveWorkspaceChoice({
            sessionWorkspace: '',
            localWorkspace: '',
            serverDefaultWorkspace: '/ws/server',
        })
        expect(got).toEqual({ path: '/ws/server', source: 'server' })
    })

    it('returns none when no workspace is available', () => {
        const got = resolveWorkspaceChoice({
            sessionWorkspace: '',
            localWorkspace: '',
            serverDefaultWorkspace: '',
        })
        expect(got).toEqual({ path: '', source: 'none' })
    })
})

