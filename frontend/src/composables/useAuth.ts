import { ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

// Reactive state
const isInitialized = ref(false)
const isProcessingCallback = ref(false)

// Keycloak configuration (fetched from backend)
let keycloakConfig = {
    url: import.meta.env.VITE_KEYCLOAK_URL || 'http://localhost:8180',
    realm: import.meta.env.VITE_KEYCLOAK_REALM || 'base-realm',
    clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'base-app',
}

/**
 * Fetch Keycloak config from backend
 */
async function fetchConfig(): Promise<void> {
    try {
        const res = await fetch('/api/auth/config')
        if (res.ok) {
            const remoteConfig = await res.json()
            if (remoteConfig.url) {
                keycloakConfig = {
                    ...keycloakConfig,
                    url: remoteConfig.url,
                    realm: remoteConfig.realm || keycloakConfig.realm,
                    clientId: remoteConfig.clientId || keycloakConfig.clientId,
                }
            }
        }
    } catch (e) {
        console.warn('Failed to fetch auth config, using defaults', e)
    }
}

/**
 * Check if current URL has OAuth callback parameters
 */
function hasCallbackParams(): boolean {
    const params = new URLSearchParams(window.location.search)
    return params.has('code') && params.has('state')
}

/**
 * Get callback parameters from URL
 */
function getCallbackParams(): { code: string; state: string } | null {
    const params = new URLSearchParams(window.location.search)
    const code = params.get('code')
    const state = params.get('state')
    if (code && state) {
        return { code, state }
    }
    return null
}

/**
 * Clear OAuth callback parameters from URL
 */
function clearCallbackParams(): void {
    const params = new URLSearchParams(window.location.search)
    params.delete('code')
    params.delete('state')
    params.delete('session_state')
    params.delete('iss')

    const newUrl = window.location.pathname +
        (params.toString() ? `?${params.toString()}` : '') +
        window.location.hash
    window.history.replaceState({}, document.title, newUrl)
}

/**
 * Exchange authorization code for backend JWT
 */
async function exchangeCodeForToken(code: string): Promise<boolean> {
    try {
        const redirectUri = window.location.origin + window.location.pathname

        const res = await fetch('/api/auth/callback', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                code,
                redirect_uri: redirectUri,
            }),
        })

        if (!res.ok) {
            const error = await res.json()
            console.error('Token exchange failed:', error)
            return false
        }

        const data = await res.json()

        // Store the backend-issued token
        const authStore = useAuthStore()
        authStore.setToken(data.access_token)
        authStore.setUser({
            id: data.user.id,
            username: data.user.username,
            email: data.user.email,
            name: data.user.name,
        })

        return true
    } catch (error) {
        console.error('Token exchange error:', error)
        return false
    }
}

/**
 * Initialize auth - check for existing session or OAuth callback
 */
export async function initKeycloak(): Promise<boolean> {
    if (isInitialized.value) {
        const authStore = useAuthStore()
        return authStore.isAuthenticated
    }

    await fetchConfig()

    const authStore = useAuthStore()

    // Check if we have a valid stored token
    if (authStore.isAuthenticated && authStore.token) {
        isInitialized.value = true
        return true
    }

    // Check if this is an OAuth callback
    if (hasCallbackParams() && !isProcessingCallback.value) {
        isProcessingCallback.value = true
        const callbackParams = getCallbackParams()

        if (callbackParams) {
            const success = await exchangeCodeForToken(callbackParams.code)
            clearCallbackParams()
            isProcessingCallback.value = false
            isInitialized.value = true
            return success
        }

        isProcessingCallback.value = false
    }

    isInitialized.value = true
    return authStore.isAuthenticated
}

/**
 * Redirect to Keycloak login
 */
export function login(redirectPath?: string) {
    // Save the intended redirect path
    const redirectUri = redirectPath
        ? new URL(redirectPath, window.location.origin).toString()
        : window.location.origin + '/'

    // Build Keycloak authorization URL
    const authUrl = new URL(`${keycloakConfig.url}/realms/${keycloakConfig.realm}/protocol/openid-connect/auth`)
    authUrl.searchParams.set('client_id', keycloakConfig.clientId)
    authUrl.searchParams.set('redirect_uri', redirectUri)
    authUrl.searchParams.set('response_type', 'code')
    authUrl.searchParams.set('scope', 'openid profile email')
    authUrl.searchParams.set('state', crypto.randomUUID())

    // Redirect to Keycloak
    window.location.href = authUrl.toString()
}

/**
 * Logout - clear local state and redirect to Keycloak logout
 */
export function logout() {
    const authStore = useAuthStore()
    authStore.clearAuth()

    const logoutUrl = new URL(`${keycloakConfig.url}/realms/${keycloakConfig.realm}/protocol/openid-connect/logout`)
    logoutUrl.searchParams.set('client_id', keycloakConfig.clientId)
    logoutUrl.searchParams.set('post_logout_redirect_uri', window.location.origin + '/')

    window.location.href = logoutUrl.toString()
}

/**
 * Get current access token
 */
export function getToken(): string | undefined {
    const authStore = useAuthStore()
    return authStore.token || undefined
}

/**
 * Composable for using auth in components
 */
export function useAuth() {
    const authStore = useAuthStore()

    return {
        isInitialized: computed(() => isInitialized.value),
        isAuthenticated: computed(() => authStore.isAuthenticated),
        user: computed(() => authStore.user),
        token: computed(() => authStore.token),
        login,
        logout,
        getToken,
    }
}

export default useAuth

