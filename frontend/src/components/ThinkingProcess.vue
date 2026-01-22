<script setup lang="ts">
import { ref } from 'vue'
import { Brain, ChevronDown, ChevronRight } from 'lucide-vue-next'
import { marked } from 'marked'

defineProps<{
  content: string
  isStreaming?: boolean
}>()

const isExpanded = ref(true)

const toggle = () => {
  isExpanded.value = !isExpanded.value
}

const renderMarkdown = (text: string) => {
  if (!text) return ''
  // Escape HTML to prevent XSS when rendering with v-html
  const escaped = text.replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return marked(escaped, { breaks: true, gfm: true })
}
</script>

<template>
  <div class="my-2 border border-surface-700/50 rounded-xl overflow-hidden bg-surface-900/30">
    <button
      @click="toggle"
      class="w-full flex items-center justify-between px-4 py-2 bg-surface-800/50 hover:bg-surface-800/70 transition-colors text-xs text-surface-400 select-none"
    >
      <div class="flex items-center gap-2">
        <Brain class="w-3.5 h-3.5" />
        <span class="font-medium">Thinking Process</span>
      </div>
      <div class="flex items-center gap-2">
        <span v-if="isStreaming" class="flex gap-0.5">
          <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.3s]"></span>
          <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce [animation-delay:-0.15s]"></span>
          <span class="w-1 h-1 rounded-full bg-surface-400 animate-bounce"></span>
        </span>
        <ChevronDown v-if="isExpanded" class="w-3.5 h-3.5" />
        <ChevronRight v-else class="w-3.5 h-3.5" />
      </div>
    </button>
    
    <div
      v-show="isExpanded"
      class="px-4 py-3 text-sm text-surface-300/90 border-t border-surface-700/30 bg-surface-950/20"
    >
      <div 
        class="prose prose-invert prose-sm max-w-none break-words text-surface-300 text-xs leading-relaxed opacity-90"
        v-html="renderMarkdown(content)"
      />
    </div>
  </div>
</template>
