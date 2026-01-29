<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ListChecks, RefreshCw, Wand2 } from 'lucide-vue-next'

import ErrorBanner from '@/components/ErrorBanner.vue'
import {
  getSimilarSopSuggestions,
  listSopSuggestions,
  loadMoreSopSuggestions,
  updateSopSuggestion,
  updateSopSuggestionStatus,
  type SimilarSuggestion,
  type Suggestion,
  type SuggestionStatus,
} from '@/api/client'
import { parseApiError, type ParsedApiError } from '@/lib/apiError'

const loading = ref(false)
const error = ref<ParsedApiError | null>(null)
const items = ref<Suggestion[]>([])

const statusFilter = ref<SuggestionStatus>('proposed')
const includeParked = ref(true)
const limit = ref(10)

const editOpen = ref<Record<string, boolean>>({})
const editDraft = ref<Record<string, { title: string; draft_skill: string }>>({})

const similarLoading = ref<Record<string, boolean>>({})
const similarByID = ref<Record<string, SimilarSuggestion[]>>({})

const loadList = async () => {
  error.value = null
  loading.value = true
  try {
    const list = await listSopSuggestions({
      status: statusFilter.value,
      include_parked: includeParked.value,
      limit: limit.value,
    })
    items.value = Array.isArray(list) ? list : []
  } catch (e: any) {
    items.value = []
    error.value = parseApiError(e, '加载 SOP 建议失败')
  } finally {
    loading.value = false
  }
}

const sortedItems = computed(() => {
  const list = [...items.value]
  list.sort((a, b) => {
    const ta = a?.scores?.total_score ?? 0
    const tb = b?.scores?.total_score ?? 0
    if (tb !== ta) return tb - ta
    return String(b.updated_at || '').localeCompare(String(a.updated_at || ''))
  })
  return list
})

const toggleEdit = (s: Suggestion) => {
  if (!editDraft.value[s.suggestion_id]) {
    editDraft.value[s.suggestion_id] = { title: s.title, draft_skill: s.draft_skill }
  }
  editOpen.value[s.suggestion_id] = !editOpen.value[s.suggestion_id]
}

const saveEdit = async (s: Suggestion) => {
  const d = editDraft.value[s.suggestion_id]
  if (!d) return
  error.value = null
  loading.value = true
  try {
    await updateSopSuggestion(s.suggestion_id, { title: d.title, draft_skill: d.draft_skill })
    editOpen.value[s.suggestion_id] = false
    await loadList()
  } catch (e: any) {
    error.value = parseApiError(e, '保存失败')
  } finally {
    loading.value = false
  }
}

const setStatus = async (id: string, status: SuggestionStatus, mergedInto?: string) => {
  error.value = null
  loading.value = true
  try {
    await updateSopSuggestionStatus(id, { status, merged_into_suggestion_id: mergedInto })
    await loadList()
  } catch (e: any) {
    error.value = parseApiError(e, '更新状态失败')
  } finally {
    loading.value = false
  }
}

const loadSimilar = async (s: Suggestion) => {
  similarLoading.value[s.suggestion_id] = true
  try {
    const list = await getSimilarSopSuggestions(s.suggestion_id, 8)
    similarByID.value[s.suggestion_id] = Array.isArray(list) ? list : []
  } catch {
    similarByID.value[s.suggestion_id] = []
  } finally {
    similarLoading.value[s.suggestion_id] = false
  }
}

const loadMore = async () => {
  error.value = null
  loading.value = true
  try {
    await loadMoreSopSuggestions({ count: 10 })
    await loadList()
  } catch (e: any) {
    error.value = parseApiError(e, '加载更多失败')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadList()
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">SOP 治理</h1>
          <p class="text-sm text-surface-500">收件箱（上限 10）· 按稀缺性/know-how/证据排序</p>
        </div>
        <button
          data-testid="sop-governance-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="loading"
          @click="loadList"
        >
          <RefreshCw class="w-4 h-4" />
          刷新
        </button>
      </div>

      <div class="glass rounded-2xl overflow-hidden">
        <div class="px-5 py-4 border-b border-surface-700/50 flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <ListChecks class="w-5 h-5 text-primary-400" />
            <div>
              <p class="text-sm font-semibold text-surface-100">建议</p>
              <p class="text-xs text-surface-500">需要人工审核后才能落地为 SKILL.md</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <select
              v-model="statusFilter"
              class="rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-xs"
              @change="loadList"
            >
              <option value="proposed">待处理</option>
              <option value="parked">搁置</option>
              <option value="approved">已通过</option>
              <option value="rejected">已拒绝</option>
              <option value="archived">已归档</option>
            </select>
            <label class="flex items-center gap-2 text-xs text-surface-400">
              <input v-model="includeParked" type="checkbox" class="accent-primary-500" @change="loadList" />
              包含搁置
            </label>
            <button
              class="px-3 py-2 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-500 inline-flex items-center gap-2"
              :disabled="loading"
              @click="loadMore"
            >
              <Wand2 class="w-4 h-4" />
              加载更多
            </button>
          </div>
        </div>

        <div class="p-5 space-y-3">
          <ErrorBanner v-if="error" :error="error" title="操作失败" />
          <div v-if="loading" class="text-sm text-surface-500">加载中…</div>
          <div v-else-if="sortedItems.length === 0" class="text-sm text-surface-500">暂无建议。</div>

          <div v-else class="space-y-3">
            <div v-for="s in sortedItems" :key="s.suggestion_id" class="glass-card p-4 space-y-3">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm text-surface-100 font-semibold truncate">{{ s.title }}</p>
                  <p class="text-xs text-surface-500 truncate">
                    id={{ s.suggestion_id.slice(0, 8) }} · {{ s.status }} · 证据={{ s.evidence_count || 0 }}
                  </p>
                </div>
                <div class="text-xs text-surface-300 text-right">
                  <div>总分={{ (s.scores?.total_score ?? 0).toFixed(2) }}</div>
                  <div class="text-surface-500">
                    稀缺性={{ (s.scores?.scarcity_score ?? 0).toFixed(2) }} · 深度={{ (s.scores?.depth_score ?? 0).toFixed(2) }}
                  </div>
                </div>
              </div>

              <div v-if="s.meta?.compression_verdict" class="rounded-xl bg-surface-900/60 border border-surface-700/50 p-3">
                <p class="text-xs text-surface-400">
                  可压缩性：<span class="text-surface-200">{{ s.meta.compression_verdict }}</span>
                </p>
                <p v-if="s.meta.compression_prompt" class="text-xs text-surface-200 mt-2 whitespace-pre-wrap">{{ s.meta.compression_prompt }}</p>
                <p v-if="s.meta.compression_reason" class="text-[11px] text-surface-500 mt-2">{{ s.meta.compression_reason }}</p>
              </div>

              <div class="flex flex-wrap gap-2">
                <button
                  v-if="s.status === 'proposed' || s.status === 'parked'"
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-primary-500/10 text-primary-200 hover:bg-primary-500/20"
                  :disabled="loading"
                  @click="setStatus(s.suggestion_id, 'approved')"
                >
                  通过
                </button>
                <button
                  v-if="s.status !== 'rejected' && s.status !== 'merged'"
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                  :disabled="loading"
                  @click="setStatus(s.suggestion_id, 'rejected')"
                >
                  拒绝
                </button>
                <button
                  v-if="s.status !== 'parked' && s.status !== 'merged'"
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                  :disabled="loading"
                  @click="setStatus(s.suggestion_id, 'parked')"
                >
                  搁置
                </button>
                <button
                  v-if="s.status !== 'archived' && s.status !== 'merged'"
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                  :disabled="loading"
                  @click="setStatus(s.suggestion_id, 'archived')"
                >
                  归档
                </button>
                <button
                  v-if="s.status === 'proposed' || s.status === 'parked'"
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-primary-500/10 text-primary-200 hover:bg-primary-500/20"
                  :disabled="loading"
                  @click="toggleEdit(s)"
                >
                  {{ editOpen[s.suggestion_id] ? '关闭' : '编辑' }}
                </button>
                <button
                  class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                  :disabled="loading || similarLoading[s.suggestion_id]"
                  @click="loadSimilar(s)"
                >
                  {{ similarLoading[s.suggestion_id] ? '…' : '相似' }}
                </button>
              </div>

              <div v-if="editOpen[s.suggestion_id]" class="space-y-2 rounded-xl bg-surface-900/60 border border-surface-700/50 p-3">
                <input
                  v-model="editDraft[s.suggestion_id].title"
                  type="text"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm"
                  placeholder="标题"
                />
                <textarea
                  v-model="editDraft[s.suggestion_id].draft_skill"
                  rows="8"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-xs font-mono"
                  placeholder="草稿技能（SKILL.md 内容）"
                />
                <div class="flex justify-end gap-2">
                  <button
                    class="px-3 py-2 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-500"
                    :disabled="loading"
                    @click="saveEdit(s)"
                  >
                    保存
                  </button>
                </div>
              </div>

              <div
                v-if="Array.isArray(similarByID[s.suggestion_id]) && similarByID[s.suggestion_id].length > 0"
                class="rounded-xl bg-surface-900/60 border border-surface-700/50 p-3"
              >
                <p class="text-xs text-surface-400 mb-2">相似建议</p>
                <div class="space-y-2">
                  <div v-for="sim in similarByID[s.suggestion_id]" :key="sim.suggestion_id" class="flex items-center justify-between gap-2">
                    <div class="min-w-0">
                      <p class="text-xs text-surface-200 truncate">{{ sim.title }}</p>
                      <p class="text-[11px] text-surface-500">
                        相似度={{ sim.similarity.toFixed(2) }} · {{ sim.status }} · id={{ sim.suggestion_id.slice(0, 8) }}
                      </p>
                    </div>
                    <button
                      v-if="(s.status === 'proposed' || s.status === 'parked') && sim.status !== 'merged'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                      :disabled="loading"
                      @click="setStatus(s.suggestion_id, 'merged', sim.suggestion_id)"
                    >
                      合并到
                    </button>
                  </div>
                </div>
              </div>

              <details class="rounded-lg bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300">草稿</summary>
                <pre class="whitespace-pre-wrap text-xs text-surface-100 px-3 pb-3">{{ s.draft_skill }}</pre>
              </details>

              <details v-if="s.status === 'approved' && s.meta?.materialized_skill_path" class="rounded-lg bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300">已落地技能</summary>
                <div class="px-3 pb-3 text-xs text-surface-100">
                  <p>skill_id: {{ s.meta.materialized_skill_id }}</p>
                  <p>path: {{ s.meta.materialized_skill_path }}</p>
                </div>
              </details>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
