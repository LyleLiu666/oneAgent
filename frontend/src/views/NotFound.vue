<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Home, ArrowLeft } from 'lucide-vue-next'

const router = useRouter()

const goHome = () => {
  router.push('/')
}

const goBack = () => {
  router.back()
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface-950 p-4">
    <div class="text-center">
      <!-- 404 graphic -->
      <div class="mb-8">
        <span class="text-8xl font-bold text-gradient">404</span>
      </div>

      <h1 class="text-2xl font-bold text-surface-100 mb-2">Page Not Found</h1>
      <p class="text-surface-400 mb-8 max-w-md mx-auto">
        The page you're looking for doesn't exist or has been moved.
      </p>

      <!-- Actions -->
      <div class="flex items-center justify-center gap-4">
        <button
          @click="goBack"
          class="flex items-center gap-2 px-5 py-2.5 bg-surface-800 hover:bg-surface-700 text-surface-200 rounded-xl transition-colors"
        >
          <ArrowLeft class="w-4 h-4" />
          Go Back
        </button>
        <button
          @click="goHome"
          class="flex items-center gap-2 px-5 py-2.5 bg-primary-600 hover:bg-primary-500 text-white rounded-xl transition-colors"
        >
          <Home class="w-4 h-4" />
          Home
        </button>
      </div>
    </div>
  </div>
</template>
