<script setup lang="ts">
import { computed } from 'vue'
import { Bot, Folder, Loader2 } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'

const { user } = useAuth()

const props = defineProps<{
  workspace?: string
  workspaceSource?: string
  showWorkspacePrompt?: boolean
  workspaceChoosing?: boolean
  workspaceChooseError?: string
}>()

const emit = defineEmits<{
  (e: 'choose-workspace'): void
  (e: 'skip-workspace'): void
}>()

const hasWorkspace = computed(() => Boolean(String(props.workspace || '').trim()))
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

      <div v-if="hasWorkspace || showWorkspacePrompt" class="space-y-3">
        <div class="mx-auto max-w-2xl rounded-2xl bg-surface-900/40 backdrop-blur border border-surface-800 px-5 py-4 text-left">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <div class="flex items-center gap-2 text-surface-200">
                <Folder class="w-4 h-4 text-surface-400" />
                <p class="text-sm font-semibold">
                  {{ hasWorkspace ? 'Workspace ready' : 'Choose a workspace' }}
                </p>
              </div>
              <p v-if="hasWorkspace" class="mt-2 text-xs text-surface-400 break-words">
                {{ workspace }}
                <span v-if="workspaceSource" class="text-surface-500"> · {{ workspaceSource }}</span>
              </p>
              <p v-else class="mt-2 text-xs text-surface-400">
                Pick a folder to enable file/search/command tools. Or skip for chat-only mode.
              </p>
              <p v-if="workspaceChooseError" class="mt-2 text-xs text-red-400">
                {{ workspaceChooseError }}
              </p>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                type="button"
                class="inline-flex items-center gap-2 rounded-lg bg-surface-900 text-surface-200 text-xs sm:text-sm px-3 py-2 border border-surface-800 hover:bg-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="workspaceChoosing"
                title="Choose workspace folder"
                @click="emit('choose-workspace')"
              >
                <Loader2 v-if="workspaceChoosing" class="w-4 h-4 animate-spin" />
                <span v-else>Browse</span>
              </button>
              <button
                v-if="!hasWorkspace"
                type="button"
                class="inline-flex items-center gap-2 rounded-lg bg-surface-900 text-surface-300 text-xs sm:text-sm px-3 py-2 border border-surface-800 hover:bg-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                title="Skip workspace (chat only)"
                @click="emit('skip-workspace')"
              >
                Skip
              </button>
            </div>
          </div>
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
