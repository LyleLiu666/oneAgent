<script setup lang="ts">
import { Brain } from 'lucide-vue-next'
import { marked } from 'marked'

defineProps<{
  content: string
  isStreaming?: boolean
}>()

const renderMarkdown = (text: string) => {
  if (!text) return ''
  // Escape HTML to prevent XSS when rendering with v-html
  const escaped = text.replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return marked(escaped, { breaks: true, gfm: true })
}
</script>

<template>
  <div class="rounded-2xl px-4 py-3 bg-surface-900/40 backdrop-blur border border-surface-700/30">
    <div class="flex items-center gap-2 text-[11px] text-surface-500 mb-2 select-none">
      <Brain class="w-3.5 h-3.5" />
      <span class="font-medium">思考</span>
      <span v-if="isStreaming" class="flex gap-0.5 ml-1">
        <span class="w-1 h-1 rounded-full bg-surface-500 animate-bounce [animation-delay:-0.3s]"></span>
        <span class="w-1 h-1 rounded-full bg-surface-500 animate-bounce [animation-delay:-0.15s]"></span>
        <span class="w-1 h-1 rounded-full bg-surface-500 animate-bounce"></span>
      </span>
    </div>
    <div
      class="thinking-prose prose prose-invert prose-sm max-w-none break-words text-xs leading-relaxed opacity-90"
      v-html="renderMarkdown(content)"
    />
  </div>
</template>

<style scoped>
.thinking-prose {
  --tw-prose-body: rgb(var(--color-surface-400));
  --tw-prose-headings: rgb(var(--color-surface-300));
  --tw-prose-links: rgb(var(--color-surface-300));
  --tw-prose-bold: rgb(var(--color-surface-300));
  --tw-prose-code: rgb(var(--color-surface-300));
  --tw-prose-pre-code: rgb(var(--color-surface-300));
  --tw-prose-bullets: rgb(var(--color-surface-500));
  --tw-prose-counters: rgb(var(--color-surface-500));
}
</style>
