<script setup lang="ts">
import { Bot } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'

const { user } = useAuth()
</script>

<template>
  <div class="h-full flex flex-col items-center justify-center p-8">
    <div class="max-w-3xl w-full space-y-12 text-center">
      
      <!-- Hero Section -->
      <div class="space-y-6 animate-fade-in-up">
        <div class="relative inline-block">
          <div class="absolute inset-0 bg-primary-500/20 blur-3xl rounded-full"></div>
          <div class="relative bg-surface-900 ring-1 ring-surface-800 p-6 rounded-3xl shadow-2xl">
            <Bot class="w-16 h-16 text-primary-400" />
          </div>
        </div>
        
        <div class="space-y-3">
          <h1 class="text-4xl md:text-5xl font-bold tracking-tight text-[var(--color-text-main)]">
            <span class="text-[var(--color-text-muted)]">Hello, </span>
            <span class="text-primary-600 dark:text-primary-400">
              {{ user?.name || user?.username || 'User' }}
            </span>
          </h1>
          <p class="text-lg text-surface-400 max-w-lg mx-auto leading-relaxed">
            I'm your advanced AI assistant. Ready to help you with coding, writing, analysis, and more.
          </p>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.animate-fade-in-up {
  animation: fade-in-up 0.6s ease-out forwards;
  opacity: 0;
  transform: translateY(20px);
}

.delay-200 {
  animation-delay: 200ms;
}

@keyframes fade-in-up {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
