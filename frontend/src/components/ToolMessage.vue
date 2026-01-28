<script setup lang="ts">
import { ref, computed } from 'vue'
import { Cpu, ChevronDown, ChevronRight } from 'lucide-vue-next'
import type { ChatMessage } from '@/stores/chat'

const props = defineProps<{
  message: ChatMessage
}>()

const isExpanded = ref(false)
const isToolCall = computed(() => props.message.type === 'tool_call')
const isToolResult = computed(() => props.message.type === 'tool_result')
const toolCalls = computed(() =>
  Array.isArray(props.message.tool?.toolCalls) ? props.message.tool?.toolCalls : []
)

const title = computed(() => {
  if (isToolCall.value) {
    if (toolCalls.value.length === 1) {
      return toolCalls.value[0]?.function?.name || 'tool_call'
    }
    if (toolCalls.value.length > 1) {
      const name = toolCalls.value[0]?.function?.name || 'tool_call'
      return `${name} +${toolCalls.value.length - 1}`
    }
    return 'tool_call'
  }

  if (props.message.tool?.name) return props.message.tool.name
  const firstResultName = props.message.tool?.results?.[0]?.tool_name
  return firstResultName || 'tool_result'
})

const subtitle = computed(() => {
  const role = props.message.rawRole || props.message.role
  const type = props.message.rawType || props.message.type
  return `${role} · ${type}`
})

const checkPermissionDenied = (err?: string) => {
  const raw = String(err || '').trim()
  if (!raw) return false
  return raw.toLowerCase().includes('permission denied')
}

const isPermissionDenied = computed(() => {
  return checkPermissionDenied(props.message.tool?.error)
})

const toggle = () => {
  isExpanded.value = !isExpanded.value
}


const formatMaybeJson = (raw: string): string => {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) return ''
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return raw
  }
}
</script>

<template>
  <div class="max-w-3xl flex gap-3">
    <div
      class="w-8 h-8 rounded-lg bg-surface-800 flex items-center justify-center flex-shrink-0 border border-surface-700/40"
    >
      <Cpu class="w-4 h-4 text-surface-300" />
    </div>
    <div class="flex-1 space-y-2 min-w-0">
      <div class="glass rounded-2xl rounded-tl-md overflow-hidden">
        <!-- Header / Toggle -->
        <button
          @click="toggle"
          class="w-full flex items-center justify-between px-4 py-3 bg-surface-800/30 hover:bg-surface-800/50 transition-colors text-left"
        >
          <div class="flex items-center gap-2 text-xs text-surface-300">
            <span class="font-medium">{{ title }}</span>
            <span class="text-[11px] text-surface-500">{{ subtitle }}</span>
            <span
              v-if="message.tool?.toolCallId"
              class="text-surface-500 font-mono text-[11px] truncate max-w-[150px]"
              :title="message.tool.toolCallId"
            >
              {{ message.tool.toolCallId }}
            </span>
          </div>
          <component :is="isExpanded ? ChevronDown : ChevronRight" class="w-4 h-4 text-surface-500" />
        </button>

        <div
          v-if="!isExpanded && isToolCall && message.content && message.content.trim()"
          class="px-4 pb-3 text-xs text-surface-400 whitespace-pre-wrap break-words line-clamp-3"
        >
          {{ message.content }}
        </div>

        <!-- Content -->
        <div v-show="isExpanded" class="px-4 py-3 border-t border-surface-700/30">
          <template v-if="isToolCall">
            <div v-if="message.content && message.content.trim()" class="space-y-1 mb-3">
              <div class="text-[11px] text-surface-500">assistant_visible</div>
              <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ message.content }}</pre>
            </div>

            <div
              v-if="toolCalls.length"
              class="space-y-4"
            >
              <div
                v-for="(call, idx) in toolCalls"
                :key="idx"
                class="space-y-2"
              >
                <div class="flex items-center justify-between gap-2 text-[11px] text-surface-500">
                  <span class="font-medium text-surface-300 truncate" :title="call?.function?.name || ''">
                    {{ call?.function?.name || 'tool' }}
                  </span>
                  <span v-if="call?.id" class="font-mono truncate" :title="call.id">
                    {{ call.id }}
                  </span>
                </div>
                <div v-if="call?.function?.arguments" class="space-y-1">
                  <div class="text-[11px] text-surface-500">arguments</div>
                  <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(String(call.function.arguments)) }}</pre>
                </div>
              </div>
            </div>
            <div v-else class="text-xs text-surface-500">No tool calls</div>

            <div
              v-if="message.tool?.llmContent && message.tool.llmContent.trim() && message.tool.llmContent !== message.tool.content"
              class="space-y-1 mt-3"
            >
              <div class="text-[11px] text-surface-500">llm_content</div>
              <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ message.tool.llmContent }}</pre>
            </div>
          </template>

          <template v-else-if="isToolResult">
            <div v-if="message.tool?.arguments" class="space-y-1 mb-3">
              <div class="text-[11px] text-surface-500">arguments</div>
              <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(message.tool.arguments) }}</pre>
            </div>

            <div
              v-if="message.tool?.results && message.tool.results.length"
              class="space-y-4"
            >
              <div
                v-for="(r, idx) in message.tool.results"
                :key="idx"
                class="space-y-2"
              >
                <div class="flex items-center justify-between gap-2 text-[11px] text-surface-500">
                  <span class="font-medium text-surface-300 truncate" :title="r.tool_name || ''">
                    {{ r.tool_name || message.tool?.name || 'tool' }}
                  </span>
                  <span v-if="r.tool_call_id" class="font-mono truncate" :title="r.tool_call_id">
                    {{ r.tool_call_id }}
                  </span>
                </div>
                <div v-if="r.arguments" class="space-y-1">
                  <div class="text-[11px] text-surface-500">arguments</div>
                  <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(String(r.arguments)) }}</pre>
                </div>
                <div v-if="r.output" class="space-y-1">
                  <div class="text-[11px] text-surface-500">output</div>
                  <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(String(r.output)) }}</pre>
                </div>
                <div v-if="r.error" class="text-xs text-red-400">{{ r.error }}</div>
                <div v-if="r.error && checkPermissionDenied(r.error)" class="text-xs text-surface-400 mt-2">
                  Blocked by tool permissions.
                  <a href="/governance/tools" class="underline text-primary-400 hover:text-primary-300">Tool Permissions</a>
                </div>
              </div>
            </div>
            <div v-else class="space-y-2">
              <div v-if="message.tool?.output" class="space-y-1">
                <div class="text-[11px] text-surface-500">output</div>
                <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(message.tool.output) }}</pre>
              </div>
              <div v-else-if="message.content" class="space-y-1">
                <div class="text-[11px] text-surface-500">output</div>
                <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[260px] overflow-y-auto custom-scrollbar shadow-inner">{{ formatMaybeJson(message.content) }}</pre>
              </div>
              <div v-if="message.tool?.error" class="text-xs text-red-400">{{ message.tool.error }}</div>
              <div v-if="message.tool?.error && isPermissionDenied" class="text-xs text-surface-400">
                Blocked by tool permissions.
                <a href="/governance/tools" class="underline text-primary-400 hover:text-primary-300">Tool Permissions</a>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="text-xs text-surface-500">Unsupported message type</div>
          </template>
        </div>
      </div>

      <!-- Trace Log Component -->
      <TraceLog :content="message.trace" />
    </div>
  </div>
</template>

<style scoped>
.glass {
  background: rgba(var(--color-surface-900), 0.6);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(var(--color-surface-700), 0.4);
}

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
