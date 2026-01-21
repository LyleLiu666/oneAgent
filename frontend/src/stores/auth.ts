import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface User {
    id: string
    username: string
    email: string
    name: string
}

export const useAuthStore = defineStore(
    'auth',
    () => {
        // State
        const token = ref<string>('')
        const refreshToken = ref<string>('')
        const user = ref<User | null>(null)

        // Getters
        const isAuthenticated = computed(() => !!token.value && !!user.value)

        // Actions
        function setToken(newToken: string) {
            token.value = newToken
        }

        function setRefreshToken(newRefreshToken: string) {
            refreshToken.value = newRefreshToken
        }

        function setUser(newUser: User) {
            user.value = newUser
        }

        function clearAuth() {
            token.value = ''
            refreshToken.value = ''
            user.value = null
        }

        return {
            token,
            refreshToken,
            user,
            isAuthenticated,
            setToken,
            setRefreshToken,
            setUser,
            clearAuth,
        }
    },
    {
        // @ts-ignore
        persist: {
            key: 'auth',
            storage: localStorage,
            paths: ['token', 'refreshToken', 'user'],
        },
    }
)
