<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { LogIn, Shield } from 'lucide-vue-next'
import { login } from '@/composables/useAuth'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const handleLogin = () => {
  const redirect = router.currentRoute.value.query.redirect

  const safeRedirect = (() => {
    if (typeof redirect !== 'string' || !redirect.startsWith('/')) return '/'
    try {
      const url = new URL(redirect, window.location.origin)
      const hashParams = new URLSearchParams(url.hash.startsWith('#') ? url.hash.slice(1) : url.hash)
      if (
        hashParams.has('code') ||
        hashParams.has('error') ||
        hashParams.has('access_token') ||
        hashParams.has('id_token')
      ) {
        return '/'
      }
      return url.pathname + url.search + url.hash
    } catch {
      return '/'
    }
  })()

  login(safeRedirect)
}

onMounted(() => {
  if (authStore.isAuthenticated) {
    router.push('/')
  }
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface-950 p-4">
    <div class="w-full max-w-md">
      <!-- Logo and title -->
      <div class="text-center mb-8">
        <div
          class="w-16 h-16 mx-auto rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center mb-4 shadow-lg shadow-primary-500/20"
        >
          <Shield class="w-8 h-8 text-white" />
        </div>
        <h1 class="text-3xl font-bold text-surface-100 mb-2">Welcome Back</h1>
        <p class="text-surface-400">Sign in to continue to OneAgent</p>
      </div>

      <!-- Login card -->
      <div class="glass rounded-2xl p-8">
        <div class="space-y-6">
          <p class="text-surface-300 text-center">
            Click below to sign in with your Keycloak account
          </p>

          <button
            @click="handleLogin"
            class="w-full flex items-center justify-center gap-3 px-6 py-3 bg-primary-600 hover:bg-primary-500 text-white font-medium rounded-xl transition-all duration-200 shadow-lg shadow-primary-500/20 hover:shadow-primary-500/30"
          >
            <LogIn class="w-5 h-5" />
            Sign in with Keycloak
          </button>
        </div>

        <div class="mt-6 pt-6 border-t border-surface-700">
          <p class="text-xs text-surface-500 text-center">
            By signing in, you agree to our Terms of Service and Privacy Policy
          </p>
        </div>
      </div>

      <!-- Footer -->
      <p class="mt-8 text-center text-sm text-surface-500">
        &copy; {{ new Date().getFullYear() }} OneAgent. All rights reserved.
      </p>
    </div>
  </div>
</template>
