<script setup lang="ts">
import { computed, ref } from 'vue'
import { Download, FolderOpen } from 'lucide-vue-next'

import { chooseWorkspaceDir, exportDocument, type DocumentExportFormat } from '@/api/client'

const workspace = ref(String(globalThis?.localStorage?.getItem?.('oneagent-workspace') || '').trim())
const inputPath = ref('report.md')
const format = ref<DocumentExportFormat>('docx')
const outputPath = ref('')
const templatePath = ref('')

const running = ref(false)
const choosing = ref(false)
const error = ref('')
const result = ref<any>(null)

const canRun = computed(() => workspace.value.trim() && inputPath.value.trim() && format.value)

const chooseWorkspace = async () => {
  error.value = ''
  choosing.value = true
  try {
    const res: any = await chooseWorkspaceDir()
    const p = String(res?.path || '').trim()
    if (p) {
      workspace.value = p
      localStorage.setItem('oneagent-workspace', p)
    }
  } catch (e: any) {
    error.value = String(e?.data?.error || e?.message || 'Failed to choose workspace')
  } finally {
    choosing.value = false
  }
}

const run = async () => {
  error.value = ''
  result.value = null
  const ws = workspace.value.trim()
  const inPath = inputPath.value.trim()
  if (!ws || !inPath) return

  running.value = true
  try {
    const payload = {
      workspace: ws,
      input_path: inPath,
      format: format.value,
      output_path: outputPath.value.trim() || undefined,
      template_path: templatePath.value.trim() || undefined,
    }
    const res = await exportDocument(payload)
    result.value = res
  } catch (e: any) {
    error.value = String(e?.data?.error || e?.message || 'Export failed')
  } finally {
    running.value = false
  }
}
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-4xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">文档导出</h1>
          <p class="text-sm text-surface-500">使用 pandoc 将工作区内的 Markdown 导出为 Office 格式。</p>
        </div>
      </div>

      <div v-if="error" class="rounded-2xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
        {{ error }}
      </div>

      <div class="glass rounded-2xl overflow-hidden">
        <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
          <Download class="w-5 h-5 text-primary-400" />
          <div class="min-w-0">
            <p class="text-sm font-semibold text-surface-100">导出设置</p>
            <p class="text-xs text-surface-500">输出路径必须位于工作区内。</p>
          </div>
        </div>

        <div class="p-5 space-y-4">
          <div class="space-y-2">
            <label class="text-xs text-surface-500">工作区</label>
            <div class="flex gap-2">
              <input
                data-testid="doc-export-workspace"
                v-model="workspace"
                class="flex-1 px-3 py-2 rounded-xl bg-surface-900/40 border border-surface-700/40 text-surface-200 placeholder:text-surface-600"
                placeholder="/path/to/workspace"
              />
              <button
                class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                :disabled="choosing"
                @click="chooseWorkspace"
              >
                <FolderOpen class="w-4 h-4" />
                选择
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-xs text-surface-500">Markdown 文件（相对路径）</label>
              <input
                data-testid="doc-export-input"
                v-model="inputPath"
                class="w-full px-3 py-2 rounded-xl bg-surface-900/40 border border-surface-700/40 text-surface-200 placeholder:text-surface-600"
                placeholder="report.md"
              />
            </div>

            <div class="space-y-2">
              <label class="text-xs text-surface-500">格式</label>
              <select
                data-testid="doc-export-format"
                v-model="format"
                class="w-full px-3 py-2 rounded-xl bg-surface-900/40 border border-surface-700/40 text-surface-200"
              >
                <option value="docx">docx</option>
                <option value="pptx">pptx</option>
              </select>
            </div>

            <div class="space-y-2">
              <label class="text-xs text-surface-500">输出路径（可选，相对路径）</label>
              <input
                data-testid="doc-export-output"
                v-model="outputPath"
                class="w-full px-3 py-2 rounded-xl bg-surface-900/40 border border-surface-700/40 text-surface-200 placeholder:text-surface-600"
                placeholder="report.docx"
              />
            </div>

            <div class="space-y-2">
              <label class="text-xs text-surface-500">模板路径（可选，相对路径）</label>
              <input
                data-testid="doc-export-template"
                v-model="templatePath"
                class="w-full px-3 py-2 rounded-xl bg-surface-900/40 border border-surface-700/40 text-surface-200 placeholder:text-surface-600"
                placeholder="templates/reference.docx"
              />
            </div>
          </div>

          <div class="flex justify-end">
            <button
              data-testid="doc-export-run"
              class="px-4 py-2 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-500 disabled:opacity-50"
              :disabled="!canRun || running"
              @click="run"
            >
              {{ running ? '正在导出…' : '导出' }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="result" class="glass rounded-2xl overflow-hidden">
        <div class="px-5 py-4 border-b border-surface-700/50">
          <p class="text-sm font-semibold text-surface-100">结果</p>
        </div>
        <pre class="p-5 text-xs text-surface-200 overflow-x-auto">{{ JSON.stringify(result, null, 2) }}</pre>
      </div>
    </div>
  </div>
</template>
