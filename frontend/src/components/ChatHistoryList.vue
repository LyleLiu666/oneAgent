<script setup lang="ts">
import { computed } from 'vue'
import { useChatStore } from '@/stores/chat'
import { Plus, MessageSquare, Loader2 } from 'lucide-vue-next'

defineProps<{
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'select', sessionId: string): void
  (e: 'new'): void
}>()

const chatStore = useChatStore()

const sessions = computed(() => chatStore.sortedSessions)

const formatTime = (date: Date) => {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  if (days === 0) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (days < 7) {
    return date.toLocaleDateString([], { weekday: 'short' })
  } else {
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-surface-900/40 w-64 flex-shrink-0 history-panel">
    <!-- Header -->
    <div class="p-3 history-header">
      <button
        @click="emit('new')"
        class="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-primary-600 hover:bg-primary-500 text-white transition-all shadow-lg shadow-primary-900/20 font-medium text-sm group"
      >
        <Plus class="w-4 h-4 transition-transform group-hover:rotate-90" />
        New Chat
      </button>
    </div>

    <!-- Scrollable List -->
    <div class="flex-1 overflow-y-auto custom-scrollbar p-2 space-y-1">
      <div v-if="loading" class="flex items-center justify-center py-8 text-surface-500">
        <Loader2 class="w-5 h-5 animate-spin mr-2" />
        <span class="text-sm">Loading...</span>
      </div>

      <div
        v-else-if="sessions.length === 0"
        class="text-center py-8 px-4 text-surface-500 text-sm"
      >
        <div class="w-12 h-12 rounded-full bg-surface-800/50 flex items-center justify-center mx-auto mb-3">
          <MessageSquare class="w-6 h-6 opacity-40" />
        </div>
        <p>No history yet</p>
      </div>

      <button
        v-for="session in sessions"
        :key="session.id"
        @click="emit('select', session.id)"
        :class="[
          'w-full text-left px-3 py-3 rounded-lg transition-all duration-200 group relative overflow-hidden',
          chatStore.currentSessionId === session.id
            ? 'bg-surface-800 text-surface-100 shadow-sm ring-1 ring-surface-700'
            : 'text-surface-400 hover:bg-surface-800/50 hover:text-surface-200',
        ]"
      >
        <div class="flex flex-col gap-0.5 relative z-10">
          <span class="text-sm font-medium truncate pr-2">
            {{ session.title || 'New Chat' }}
          </span>
          <span class="text-[10px] opacity-60 font-medium tracking-wide uppercase">
            {{ formatTime(new Date(session.updatedAt || session.createdAt)) }}
          </span>
        </div>
        
        <!-- Active Indicator -->
        <div 
          v-if="chatStore.currentSessionId === session.id"
          class="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-8 bg-primary-500 rounded-r-full"
        ></div>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Use gradient shadow instead of hard border line */
.history-panel {
  position: relative;
}

.history-panel::after {
  content: '';
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 1px;
  background: linear-gradient(
    to bottom,
    transparent 0%,
    rgba(99, 102, 241, 0.15) 20%,
    rgba(99, 102, 241, 0.08) 50%,
    rgba(99, 102, 241, 0.15) 80%,
    transparent 100%
  );
}

.history-header {
  position: relative;
}

.history-header::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(
    to right,
    transparent 0%,
    rgba(99, 102, 241, 0.2) 30%,
    rgba(99, 102, 241, 0.2) 70%,
    transparent 100%
  );
}

.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background-color: var(--color-surface-800);
  border-radius: 9999px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
</style>
