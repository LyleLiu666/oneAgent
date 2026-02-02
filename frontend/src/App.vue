<script setup lang="ts">
import { computed } from 'vue'
import { RouterView } from 'vue-router'
import { useRoute } from 'vue-router'
import Sidebar from '@/components/Sidebar.vue'
import { initTheme } from '@/composables/useTheme'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'

initTheme()

const authStore = useAuthStore()
const uiStore = useUIStore()
const route = useRoute()

const shouldShowSidebar = computed(() => authStore.isAuthenticated && uiStore.isFullMode)

const shouldShowFullModeGate = computed(() => {
  if (!authStore.isAuthenticated) return false
  if (!uiStore.isSecretaryMode) return false
  return Boolean((route.meta as any)?.requiresFullMode)
})

const enterFullMode = () => {
  uiStore.setMode('full')
}
</script>

<template>
  <div class="flex min-h-screen bg-background transition-colors duration-300">
    <!-- Sidebar Navigation -->
    <Sidebar v-if="shouldShowSidebar" />
    
    <!-- Main Content -->
    <main 
      class="flex-1 transition-all duration-300 relative z-10"
      :class="authStore.isAuthenticated ? 'ml-0' : ''"
    >
      <div
        v-if="shouldShowFullModeGate"
        data-testid="full-mode-gate"
        class="sticky top-0 z-40 border-b border-surface-800 bg-surface-950/80 backdrop-blur px-4 py-2 text-xs text-surface-200"
      >
        <div class="mx-auto flex max-w-5xl items-center justify-between gap-3">
          <span class="truncate">此页面属于完全模式</span>
          <button
            type="button"
            data-testid="full-mode-gate-enter"
            class="shrink-0 rounded-lg border border-surface-700 bg-surface-900 px-3 py-1.5 text-xs text-surface-100 hover:bg-surface-800"
            @click="enterFullMode"
          >
            进入完全模式
          </button>
        </div>
      </div>

      <RouterView />
    </main>
  </div>
</template>

<style scoped>
/* App-level styles if needed */
</style>
