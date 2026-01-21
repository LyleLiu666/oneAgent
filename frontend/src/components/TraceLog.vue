<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  content?: string
}>()

const hasContent = computed(() => !!props.content && props.content.trim().length > 0)
</script>

<template>
  <details
    v-if="hasContent"
    class="text-xs text-surface-500 mt-2 bg-surface-900/50 rounded-lg border border-surface-800"
  >
    <summary class="cursor-pointer hover:text-surface-300 px-3 py-2 font-medium select-none flex items-center gap-2">
      <div class="i-lucide-activity w-3 h-3 opacity-70"></div>
      Show trace
    </summary>
    <div class="px-3 pb-3 pt-0">
      <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-all text-xs font-mono text-surface-400 max-h-[300px] overflow-y-auto custom-scrollbar shadow-inner">{{ content }}</pre>
    </div>
  </details>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
  height: 4px;
}

.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background: var(--color-surface-700);
  border-radius: 2px;
}

.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: var(--color-surface-600);
}
</style>
