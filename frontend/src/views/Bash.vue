<script setup lang="ts">
import { computed, ref } from 'vue'
import { Terminal, Play, RefreshCcw, Clock } from 'lucide-vue-next'
import { runBashCommand, type BashRunResponse } from '@/api/client'

const command = ref('ls -la')
const timeoutMs = ref(10000)
const isRunning = ref(false)
const result = ref<BashRunResponse | null>(null)
const error = ref('')

const examples = [
  { label: 'List files', value: 'ls -la' },
  { label: 'Working dir', value: 'pwd' },
  { label: 'System info', value: 'uname -a' },
  { label: 'Disk usage', value: 'df -h' },
]

const formattedDuration = computed(() => {
  if (!result.value) return ''
  const ms = result.value.duration_ms
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${ms}ms`
})

const statusLabel = computed(() => {
  if (!result.value) return ''
  if (result.value.timed_out) return 'Timed out'
  if (result.value.exit_code === 0) return 'Success'
  return `Exit ${result.value.exit_code}`
})

const runCommand = async () => {
  error.value = ''
  result.value = null

  const trimmed = command.value.trim()
  if (!trimmed) {
    error.value = 'Command is required.'
    return
  }

  const safeTimeout = Number.isFinite(timeoutMs.value)
    ? Math.min(Math.max(Math.floor(timeoutMs.value), 0), 30000)
    : 0
  if (Number.isFinite(timeoutMs.value)) {
    timeoutMs.value = safeTimeout
  }

  isRunning.value = true
  try {
    const payload = await runBashCommand(trimmed, safeTimeout || undefined)
    result.value = payload
  } catch (err: any) {
    const message =
      err?.data?.error ||
      err?.message ||
      'Failed to run command.'
    error.value = message
  } finally {
    isRunning.value = false
  }
}

const clearResult = () => {
  error.value = ''
  result.value = null
}
</script>

<template>
  <div class="min-h-screen bg-surface-950 p-6 lg:p-8">
    <div class="max-w-5xl mx-auto space-y-6">
      <header class="space-y-2">
        <div class="flex items-center gap-3">
          <Terminal class="w-8 h-8 text-primary-400" />
          <h1 class="text-2xl font-bold text-surface-100">Bash Playground</h1>
        </div>
        <p class="text-surface-400">
          Run bash commands on the server and inspect stdout/stderr in one place.
        </p>
      </header>

      <div class="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
        <section class="glass rounded-2xl p-6 space-y-5">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold text-surface-100">Command</h2>
            <span class="text-xs text-surface-500">Linux/macOS only</span>
          </div>

          <textarea
            v-model="command"
            class="input-base font-mono min-h-[160px] resize-none"
            placeholder="Enter a bash command..."
          />

          <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <label class="space-y-1 text-sm text-surface-400">
              <span class="flex items-center gap-2">
                <Clock class="w-4 h-4" />
                Timeout (ms)
              </span>
              <input
                v-model.number="timeoutMs"
                type="number"
                min="0"
                max="30000"
                step="500"
                class="input-base w-40"
              />
            </label>

            <div class="flex items-center gap-2">
              <button
                class="btn-primary px-4 py-2"
                :disabled="isRunning"
                @click="runCommand"
              >
                <Play class="w-4 h-4" />
                {{ isRunning ? 'Running...' : 'Run' }}
              </button>
              <button
                class="btn-ghost px-4 py-2"
                :disabled="isRunning"
                @click="clearResult"
              >
                <RefreshCcw class="w-4 h-4" />
                Clear
              </button>
            </div>
          </div>

          <div class="space-y-2">
            <p class="text-xs uppercase tracking-widest text-surface-500">Examples</p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="example in examples"
                :key="example.label"
                class="px-3 py-1.5 text-xs rounded-full bg-surface-800/70 text-surface-300 hover:text-white hover:bg-primary-600/70 transition"
                @click="command = example.value"
              >
                {{ example.label }}
              </button>
            </div>
          </div>
        </section>

        <section class="glass rounded-2xl p-6 space-y-4">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold text-surface-100">Result</h2>
            <span
              v-if="result"
              class="text-xs px-2 py-1 rounded-full"
              :class="[
                result.timed_out
                  ? 'bg-error/20 text-error'
                  : result.exit_code === 0
                    ? 'bg-success/20 text-success'
                    : 'bg-warning/20 text-warning',
              ]"
            >
              {{ statusLabel }}
            </span>
          </div>

          <p v-if="!result && !error" class="text-sm text-surface-500">
            Output will appear here after you run a command.
          </p>

          <div v-if="error" class="rounded-xl border border-error/30 bg-error/10 p-3 text-sm text-error">
            {{ error }}
          </div>

          <div v-if="result" class="space-y-3">
            <div class="flex flex-wrap gap-2 text-xs text-surface-400">
              <span class="px-2 py-1 rounded bg-surface-800/60">
                Duration: {{ formattedDuration }}
              </span>
              <span class="px-2 py-1 rounded bg-surface-800/60">
                Exit code: {{ result.exit_code }}
              </span>
              <span v-if="result.timed_out" class="px-2 py-1 rounded bg-error/20 text-error">
                Timeout reached
              </span>
            </div>

            <p class="text-xs text-surface-500 break-all">
              Shell: {{ result.shell }}
            </p>

            <div class="glass-card p-4">
              <div class="flex items-center justify-between">
                <h3 class="text-sm font-semibold text-surface-200">Stdout</h3>
                <span v-if="result.stdout_truncated" class="text-xs text-warning">Truncated</span>
              </div>
              <pre class="mt-2 text-xs text-surface-200 font-mono whitespace-pre-wrap break-words min-h-[80px] max-h-[220px] overflow-auto">{{
                result.stdout || 'No output.'
              }}</pre>
            </div>

            <div class="glass-card p-4">
              <div class="flex items-center justify-between">
                <h3 class="text-sm font-semibold text-surface-200">Stderr</h3>
                <span v-if="result.stderr_truncated" class="text-xs text-warning">Truncated</span>
              </div>
              <pre class="mt-2 text-xs text-surface-200 font-mono whitespace-pre-wrap break-words min-h-[80px] max-h-[220px] overflow-auto">{{
                result.stderr || 'No output.'
              }}</pre>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
