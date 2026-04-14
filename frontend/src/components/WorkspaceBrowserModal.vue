<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ChevronLeft, Folder, HardDrive, Loader2, RefreshCw, X } from 'lucide-vue-next'

import { browseWorkspaceDir, type WorkspaceBrowseEntry } from '@/api/client'
import ErrorBanner from '@/components/ErrorBanner.vue'
import { parseApiError, type ParsedApiError } from '@/lib/apiError'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  initialPath?: string
}>(), {
  title: '选择工作区文件夹',
  initialPath: '',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'select', path: string): void
}>()

const loading = ref(false)
const error = ref<ParsedApiError | null>(null)
const currentPath = ref('')
const rootPath = ref('')
const parentPath = ref('')
const entries = ref<WorkspaceBrowseEntry[]>([])

let loadSeq = 0

const atRootList = computed(() => !String(currentPath.value || '').trim())

const loadPath = async (path?: string) => {
  const seq = ++loadSeq
  loading.value = true
  error.value = null
  try {
    const res = await browseWorkspaceDir(path)
    if (seq !== loadSeq) return
    currentPath.value = String(res?.current_path || '').trim()
    rootPath.value = String(res?.root_path || '').trim()
    parentPath.value = String(res?.parent_path || '').trim()
    entries.value = Array.isArray(res?.entries) ? res.entries : []
  } catch (e: any) {
    if (seq !== loadSeq) return
    error.value = parseApiError(e, '加载目录失败')
    if (!path) {
      currentPath.value = ''
      rootPath.value = ''
      parentPath.value = ''
      entries.value = []
    }
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

const close = () => {
  emit('close')
}

const openEntry = async (path: string) => {
  const nextPath = String(path || '').trim()
  if (!nextPath) return
  await loadPath(nextPath)
}

const showRoots = async () => {
  await loadPath()
}

const goUp = async () => {
  const nextPath = String(parentPath.value || '').trim()
  if (!nextPath) return
  await loadPath(nextPath)
}

const selectCurrent = () => {
  const path = String(currentPath.value || '').trim()
  if (!path) return
  emit('select', path)
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    void loadPath(String(props.initialPath || '').trim() || undefined)
  },
  { immediate: true },
)
</script>

<template>
  <div
    v-if="open"
    data-testid="workspace-browser-modal"
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
  >
    <div class="absolute inset-0 bg-black/70" @click="close"></div>
    <div class="relative w-full max-w-3xl rounded-3xl bg-surface-900 shadow-2xl overflow-hidden">
      <div class="px-5 py-4 bg-surface-800/50 flex items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="text-sm font-semibold text-surface-100">{{ title }}</div>
          <div class="mt-1 text-xs text-surface-400">
            这里只展示服务端允许浏览的目录。进入目录后，点“选择当前目录”即可。
          </div>
        </div>
        <button
          type="button"
          data-testid="workspace-browser-close"
          class="rounded-xl px-3 py-2 text-sm text-surface-300 hover:bg-surface-700/60"
          @click="close"
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="p-5 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            data-testid="workspace-browser-root-list"
            class="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium bg-surface-800/70 text-surface-200 hover:bg-surface-700/70"
            @click="showRoots"
          >
            <HardDrive class="h-4 w-4" />
            根目录
          </button>
          <button
            type="button"
            data-testid="workspace-browser-up"
            class="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium bg-surface-800/70 text-surface-200 hover:bg-surface-700/70 disabled:opacity-50"
            :disabled="!parentPath"
            @click="goUp"
          >
            <ChevronLeft class="h-4 w-4" />
            上一级
          </button>
          <button
            type="button"
            data-testid="workspace-browser-select-current"
            class="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium bg-primary-600 text-white hover:bg-primary-500 disabled:opacity-50"
            :disabled="!currentPath"
            @click="selectCurrent"
          >
            <Folder class="h-4 w-4" />
            选择当前目录
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium bg-surface-800/70 text-surface-200 hover:bg-surface-700/70"
            :disabled="loading"
            @click="loadPath(currentPath || undefined)"
          >
            <RefreshCw class="h-4 w-4" />
            刷新
          </button>
        </div>

        <div class="rounded-2xl border border-surface-800 bg-surface-950/40 px-4 py-3 text-xs text-surface-400">
          <div class="font-medium text-surface-300">当前位置</div>
          <div data-testid="workspace-browser-current-path" class="mt-1 break-all font-mono">
            {{ atRootList ? '允许浏览的根目录列表' : currentPath }}
          </div>
          <div v-if="rootPath" class="mt-2 text-surface-500">
            当前根目录：<span class="font-mono break-all">{{ rootPath }}</span>
          </div>
        </div>

        <div v-if="loading" class="py-10 text-center text-sm text-surface-500">
          <Loader2 class="mx-auto mb-2 h-5 w-5 animate-spin" />
          正在加载目录…
        </div>
        <ErrorBanner v-else-if="error" :error="error" title="加载失败" />
        <div v-else class="rounded-2xl border border-surface-800 bg-surface-950/30 overflow-hidden">
          <div
            v-if="entries.length === 0"
            class="px-4 py-10 text-center text-sm text-surface-500"
          >
            当前没有可进入的子目录。
          </div>
          <button
            v-for="entry in entries"
            :key="entry.path"
            type="button"
            data-testid="workspace-browser-entry"
            class="flex w-full items-center justify-between gap-3 border-b border-surface-800/80 px-4 py-3 text-left text-sm text-surface-200 transition-colors last:border-b-0 hover:bg-surface-800/40"
            @click="openEntry(entry.path)"
          >
            <div class="min-w-0">
              <div class="font-medium">{{ entry.name }}</div>
              <div class="mt-1 break-all font-mono text-xs text-surface-500">
                {{ entry.path }}
              </div>
            </div>
            <div class="shrink-0 text-xs text-surface-500">
              {{ atRootList ? '进入根目录' : '进入' }}
            </div>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
