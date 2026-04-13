import { ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

const API_BASE = import.meta.env.VITE_API_BASE || ''

const isInitialized = ref(false)
const authMode = ref<'token' | 'none' | 'unknown'>('unknown')
let authModeLoadPromise: Promise<void> | null = null

async function fetchHealth(): Promise<void> {
    try {
        const res = await fetch(`${API_BASE}/health`)
        if (!res.ok) return
        const data = await res.json()
        const mode = String(data?.auth_mode || '').toLowerCase()
        if (mode === 'none') authMode.value = 'none'
        else if (mode === 'token') authMode.value = 'token'
    } catch {
        // ignore
    }
}

export async function ensureAuthModeLoaded(force: boolean = false): Promise<void> {
    if (!force && authMode.value !== 'unknown') {
        return
    }
    if (authModeLoadPromise) {
        return authModeLoadPromise
    }

    authModeLoadPromise = (async () => {
        await fetchHealth()
    })().finally(() => {
        authModeLoadPromise = null
    })

    return authModeLoadPromise
}

async function fetchMe(token?: string): Promise<{ user_id: string; username: string } | null> {
    try {
        const headers: Record<string, string> = {}
        if (token) headers.Authorization = `Bearer ${token}`

        const res = await fetch(`${API_BASE}/api/me`, { headers })
        if (!res.ok) return null
        return await res.json()
    } catch {
        return null
    }
}

export async function initAuth(): Promise<boolean> {
    if (isInitialized.value) {
        return useAuthStore().isAuthenticated
    }

    await ensureAuthModeLoaded()

    const authStore = useAuthStore()

    // AUTH_MODE=none: allow auto-login without token.
    if (authMode.value === 'none') {
        const me = await fetchMe()
        if (me) {
            authStore.setUser({
                id: me.user_id,
                username: me.username,
                email: '',
                name: '',
            })
            if (!authStore.token) {
                authStore.setToken('__AUTH_NONE__')
            }
            isInitialized.value = true
            return true
        }
    }

    // AUTH_MODE=token: validate stored token (if any).
    if (authStore.token) {
        const me = await fetchMe(authStore.token)
        if (me) {
            authStore.setUser({
                id: me.user_id,
                username: me.username,
                email: '',
                name: '',
            })
            isInitialized.value = true
            return true
        }
    }

    authStore.clearAuth()
    isInitialized.value = true
    return false
}

export async function loginWithToken(token: string): Promise<boolean> {
    const authStore = useAuthStore()
    const trimmed = token.trim()
    if (!trimmed) return false

    await ensureAuthModeLoaded()
    authStore.setToken(trimmed)
    const me = await fetchMe(trimmed)
    if (!me) {
        authStore.clearAuth()
        return false
    }

    authStore.setUser({
        id: me.user_id,
        username: me.username,
        email: '',
        name: '',
    })
    return true
}

export function logout() {
    const authStore = useAuthStore()
    authStore.clearAuth()
    window.location.href = '/login'
}

export function getToken(): string | undefined {
    const authStore = useAuthStore()
    return authStore.token || undefined
}

export function useAuth() {
    const authStore = useAuthStore()

    return {
        isInitialized: computed(() => isInitialized.value),
        isAuthenticated: computed(() => authStore.isAuthenticated),
        user: computed(() => authStore.user),
        token: computed(() => authStore.token),
        authMode: computed(() => authMode.value),
        initAuth,
        loginWithToken,
        logout,
        getToken,
    }
}

export default useAuth
