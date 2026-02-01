<script setup lang="ts">
import { ref, computed } from 'vue'
import { Cpu, ChevronDown, ChevronRight, Loader2 } from 'lucide-vue-next'
import type { ChatMessage } from '@/stores/chat'
import { approveToolApproval, denyToolApproval, getSubagentRunArtifact } from '@/api/client'

const props = defineProps<{
  message: ChatMessage
  sessionId?: string
  pending?: boolean
  progressTokens?: number
  showTrace?: boolean
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

const isPending = computed(() => props.pending === true)
const showTrace = computed(() => props.showTrace !== false)

type ApprovalInfo =
  | { kind: 'required'; approvalId: string }
  | { kind: 'denied'; approvalId: string; reason?: string }

const tryParseJsonObject = (raw: any): any | undefined => {
  if (typeof raw !== 'string') return undefined
  const trimmed = raw.trim()
  if (!trimmed) return undefined
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object') return parsed
    return undefined
  } catch {
    return undefined
  }
}

const approvalInfo = computed<ApprovalInfo | null>(() => {
  if (!isToolResult.value) return null
  const raw = String(props.message.tool?.output || props.message.content || '')
  const parsed = tryParseJsonObject(raw)
  if (!parsed) return null

  const approvalId = String((parsed as any).approval_id || '').trim()
  if (!approvalId) return null

  if ((parsed as any).approval_required === true || String((parsed as any).error || '') === 'approval_required') {
    return { kind: 'required', approvalId }
  }
  if ((parsed as any).approval_denied === true || String((parsed as any).error || '') === 'approval_denied') {
    const reason = String((parsed as any).reason || '').trim()
    return { kind: 'denied', approvalId, reason: reason || undefined }
  }
  return null
})

const approvalBusy = ref(false)

const onApprove = async () => {
  const info = approvalInfo.value
  if (!info || info.kind !== 'required') return
  if (approvalBusy.value) return
  approvalBusy.value = true
  try {
    const reason = window.prompt('审批原因（可选）', '') ?? ''
    await approveToolApproval(info.approvalId, reason)
    alert('已批准。请让 Agent 重试该操作。')
  } catch (e) {
    alert(String(e))
  } finally {
    approvalBusy.value = false
  }
}

const onDeny = async () => {
  const info = approvalInfo.value
  if (!info || info.kind !== 'required') return
  if (approvalBusy.value) return
  approvalBusy.value = true
  try {
    const reason = window.prompt('拒绝原因（可选）', '') ?? ''
    await denyToolApproval(info.approvalId, reason)
    alert('已拒绝。')
  } catch (e) {
    alert(String(e))
  } finally {
    approvalBusy.value = false
  }
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

type SubagentRunInfo = {
  runId: string
  findingsPath?: string
  traceLogPath?: string
}

const subagentRunInfo = computed<SubagentRunInfo | null>(() => {
  if (!isToolResult.value) return null
  const toolName = String(props.message.tool?.name || '').trim()
  if (toolName !== 'subagent') return null

  const raw = String(props.message.tool?.output || props.message.content || '').trim()
  const parsed = tryParseJsonObject(raw)
  if (!parsed) return null

  const runId = String((parsed as any).run_id || '').trim()
  if (!runId) return null

  const findingsPath = String((parsed as any).findings_path || '').trim()
  const traceLogPath = String((parsed as any).trace_log_path || '').trim()
  return {
    runId,
    findingsPath: findingsPath || undefined,
    traceLogPath: traceLogPath || undefined,
  }
})

const subagentArtifactsBusy = ref(false)
const subagentArtifactsError = ref('')
const subagentTrace = ref('')
const subagentFindings = ref('')

const loadSubagentArtifact = async (kind: 'trace' | 'findings') => {
  const info = subagentRunInfo.value
  if (!info) return
  const sessionId = String(props.sessionId || '').trim()
  if (!sessionId) {
    subagentArtifactsError.value = '缺少 session_id，无法加载子任务产物。'
    return
  }
  if (subagentArtifactsBusy.value) return
  subagentArtifactsBusy.value = true
  subagentArtifactsError.value = ''
  try {
    const res = await getSubagentRunArtifact(sessionId, info.runId, kind, { tail: kind === 'trace' })
    if (kind === 'trace') {
      subagentTrace.value = String((res as any)?.content || '')
    } else {
      subagentFindings.value = String((res as any)?.content || '')
    }
  } catch (e: any) {
    subagentArtifactsError.value = String(e?.message || e)
  } finally {
    subagentArtifactsBusy.value = false
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
	              v-if="approvalInfo?.kind === 'required'"
	              class="px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-300 text-[10px]"
	            >
	              需要审批
	            </span>
	            <span
	              v-else-if="approvalInfo?.kind === 'denied'"
	              class="px-2 py-0.5 rounded-full bg-red-500/20 text-red-300 text-[10px]"
	            >
	              审批已拒绝
	            </span>
	            <span
	              v-if="isPending"
	              data-testid="tool-pending-badge"
	              class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 bg-surface-900/60 border border-surface-700/40 text-[11px] text-surface-200"
            >
              <Loader2 class="w-3 h-3 animate-spin text-primary-300" />
              <span>执行中</span>
              <span
                v-if="typeof progressTokens === 'number'"
                data-testid="tool-progress-tokens"
                class="text-surface-400"
              >
                {{ progressTokens }} tokens
              </span>
            </span>
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

	        <div v-if="approvalInfo" class="px-4 pt-3">
	          <div class="p-3 rounded-md border border-surface-700/40 bg-surface-900/40">
	            <div class="flex items-start justify-between gap-3">
	              <div class="min-w-0">
	                <div class="text-xs text-surface-300">
	                  <span v-if="approvalInfo.kind === 'required'">需要审批</span>
	                  <span v-else>审批已拒绝</span>
	                </div>
	                <div class="mt-1 text-[11px] text-surface-500 font-mono break-all">
	                  {{ approvalInfo.approvalId }}
	                </div>
	              </div>
	              <div v-if="approvalInfo.kind === 'required'" class="flex gap-2 flex-shrink-0">
	                <button
	                  data-testid="tool-approval-approve"
	                  class="px-3 py-1 rounded-md bg-primary-600 hover:bg-primary-500 text-white text-xs disabled:opacity-50"
	                  :disabled="approvalBusy"
	                  @click="onApprove"
	                >
	                  批准
	                </button>
	                <button
	                  data-testid="tool-approval-deny"
	                  class="px-3 py-1 rounded-md bg-surface-800 hover:bg-surface-700 text-surface-200 text-xs disabled:opacity-50"
	                  :disabled="approvalBusy"
	                  @click="onDeny"
	                >
	                  拒绝
	                </button>
	              </div>
	            </div>
	            <div v-if="approvalInfo.kind === 'denied' && approvalInfo.reason" class="mt-2 text-xs text-red-400">
	              拒绝原因：{{ approvalInfo.reason }}
	            </div>
	          </div>
	        </div>

	        <div
	          v-if="!isExpanded && isToolCall && message.content && message.content.trim()"
	          class="px-4 pb-3 text-xs text-surface-400 whitespace-pre-wrap break-words line-clamp-3"
	        >
          {{ message.content }}
	        </div>

	        <!-- Content -->
	        <div v-if="isExpanded" class="px-4 py-3 border-t border-surface-700/30">
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
                  被工具权限拦截。
                  <a href="/governance/tools" class="underline text-primary-400 hover:text-primary-300">工具权限</a>
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
                被工具权限拦截。
                <a href="/governance/tools" class="underline text-primary-400 hover:text-primary-300">工具权限</a>
              </div>
            </div>

            <div v-if="subagentRunInfo" class="mt-4 space-y-2">
              <div class="text-[11px] text-surface-500">subagent artifacts</div>
              <div class="flex flex-wrap gap-2 items-center">
                <button
                  type="button"
                  class="inline-flex items-center gap-2 rounded-md border border-surface-700 bg-surface-950 px-3 py-1.5 text-xs text-surface-200 hover:bg-surface-900 disabled:opacity-60"
                  :disabled="subagentArtifactsBusy"
                  @click="loadSubagentArtifact('findings')"
                >
                  <Loader2 v-if="subagentArtifactsBusy" class="w-3.5 h-3.5 animate-spin" />
                  加载 findings
                </button>
                <button
                  type="button"
                  class="inline-flex items-center gap-2 rounded-md border border-surface-700 bg-surface-950 px-3 py-1.5 text-xs text-surface-200 hover:bg-surface-900 disabled:opacity-60"
                  :disabled="subagentArtifactsBusy"
                  @click="loadSubagentArtifact('trace')"
                >
                  <Loader2 v-if="subagentArtifactsBusy" class="w-3.5 h-3.5 animate-spin" />
                  加载 trace（tail）
                </button>
                <span class="text-[11px] text-surface-500 font-mono">run_id={{ subagentRunInfo.runId }}</span>
              </div>
              <div v-if="subagentRunInfo.findingsPath || subagentRunInfo.traceLogPath" class="text-[11px] text-surface-500 space-y-0.5">
                <div v-if="subagentRunInfo.findingsPath" class="truncate" :title="subagentRunInfo.findingsPath">
                  findings_path: <span class="font-mono">{{ subagentRunInfo.findingsPath }}</span>
                </div>
                <div v-if="subagentRunInfo.traceLogPath" class="truncate" :title="subagentRunInfo.traceLogPath">
                  trace_log_path: <span class="font-mono">{{ subagentRunInfo.traceLogPath }}</span>
                </div>
              </div>
              <div v-if="subagentArtifactsError" class="text-xs text-red-400">{{ subagentArtifactsError }}</div>

              <details
                v-if="subagentFindings && subagentFindings.trim()"
                class="rounded-xl bg-surface-900/60 border border-surface-700/50"
              >
                <summary class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200">查看 Findings</summary>
                <div class="px-4 pb-4">
                  <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[320px] overflow-y-auto custom-scrollbar shadow-inner">{{ subagentFindings }}</pre>
                </div>
              </details>

              <details
                v-if="subagentTrace && subagentTrace.trim()"
                class="rounded-xl bg-surface-900/60 border border-surface-700/50"
              >
                <summary class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200">查看 Subagent Trace（tail）</summary>
                <div class="px-4 pb-4">
                  <pre class="p-3 bg-surface-950 rounded-md border border-surface-800/50 whitespace-pre-wrap break-words text-xs font-mono text-surface-300 max-h-[320px] overflow-y-auto custom-scrollbar shadow-inner">{{ subagentTrace }}</pre>
                </div>
              </details>
            </div>
          </template>
          <template v-else>
            <div class="text-xs text-surface-500">不支持的消息类型</div>
          </template>
        </div>
      </div>

      <!-- Trace Log Component -->
      <TraceLog v-if="showTrace" :content="message.trace" />
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
