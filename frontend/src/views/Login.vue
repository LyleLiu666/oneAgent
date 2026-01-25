<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { LogIn, Shield } from 'lucide-vue-next'
import { loginWithToken, useAuth } from '@/composables/useAuth'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const { authMode } = useAuth()

const tokenInput = ref('')
const submitting = ref(false)
const errorMessage = ref('')

const handleLogin = async () => {
  errorMessage.value = ''
  submitting.value = true

  try {
    const redirect = router.currentRoute.value.query.redirect
    const safeRedirect = (() => {
      if (typeof redirect !== 'string' || !redirect.startsWith('/')) return '/'
      try {
        const url = new URL(redirect, window.location.origin)
        return url.pathname + url.search + url.hash
      } catch {
        return '/'
      }
    })()

    // AUTH_MODE=none: token is not required.
    if (authMode.value === 'none') {
      await router.push(safeRedirect)
      return
    }

    const ok = await loginWithToken(tokenInput.value)
    if (!ok) {
      errorMessage.value = 'Token 无效或鉴权失败，请检查后重试。'
      return
    }
    await router.push(safeRedirect)
  } finally {
    submitting.value = false
  }
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
          <div class="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-200">
            <p class="font-semibold mb-1">安全提示</p>
            <p>仅建议在可信局域网内使用。将服务暴露到公网风险极大。</p>
          </div>

          <div v-if="authMode === 'none'" class="rounded-xl border border-surface-700 bg-surface-900/40 p-4 text-sm text-surface-200">
            <p class="font-semibold mb-1">当前实例未启用认证（AUTH_MODE=none）</p>
            <p>仅建议用于开发/离线极简场景。</p>
          </div>

          <div v-else class="space-y-3">
            <p class="text-surface-300">
              请输入本地访问令牌（token）。可运行 <code class="text-surface-100">oneagent doctor</code> 查看 token 文件路径并读取其内容。
            </p>
            <input
              v-model="tokenInput"
              type="password"
              autocomplete="current-password"
              placeholder="Paste token here..."
              class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
            />
            <p v-if="errorMessage" class="text-sm text-red-300">{{ errorMessage }}</p>
          </div>

          <button
            @click="handleLogin"
            :disabled="submitting || (authMode !== 'none' && !tokenInput.trim())"
            class="w-full flex items-center justify-center gap-3 px-6 py-3 bg-primary-600 hover:bg-primary-500 text-white font-medium rounded-xl transition-all duration-200 shadow-lg shadow-primary-500/20 hover:shadow-primary-500/30 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <LogIn class="w-5 h-5" />
            {{ authMode === 'none' ? 'Continue' : 'Sign in with Token' }}
          </button>
        </div>

        <div class="mt-6 pt-6 border-t border-surface-700" />
      </div>

      <!-- Footer -->
      <p class="mt-8 text-center text-sm text-surface-500">
        &copy; {{ new Date().getFullYear() }} OneAgent
      </p>
    </div>
  </div>
</template>
