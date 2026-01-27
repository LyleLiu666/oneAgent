<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { FileText, ListChecks, ScrollText, RefreshCcw, Search, Wand2 } from 'lucide-vue-next'
import {
  getTodayDigest,
  listReceipts,
  type Receipt,
  listSopSuggestions,
  generateSopSuggestions,
  loadMoreSopSuggestions,
  updateSopSuggestionStatus,
  updateSopSuggestion,
  getSimilarSopSuggestions,
  type Suggestion,
  type SuggestionStatus,
  type SimilarSuggestion,
} from '@/api/client'

const activeTab = ref<'receipts' | 'digest' | 'sop'>('receipts')

// Receipts state
const receiptsLoading = ref(false)
const receiptsError = ref('')
const receipts = ref<Receipt[]>([])
const receiptsQuery = ref('')
const receiptsStatus = ref<string>('')
const receiptsWorkspace = ref(localStorage.getItem('oneagent-workspace') || '')
const selectedReceiptID = ref<string>('')

const selectedReceipt = computed(() => receipts.value.find((r) => r.receipt_id === selectedReceiptID.value))

const loadReceipts = async () => {
  receiptsLoading.value = true
  receiptsError.value = ''
  try {
    const list = await listReceipts({
      q: receiptsQuery.value.trim() || undefined,
      status: (receiptsStatus.value.trim() as any) || undefined,
      workspace: receiptsWorkspace.value.trim() || undefined,
      limit: 50,
    })
    receipts.value = Array.isArray(list) ? list : []
    if (selectedReceiptID.value && !receipts.value.some((r) => r.receipt_id === selectedReceiptID.value)) {
      selectedReceiptID.value = ''
    }
  } catch (e: any) {
    receiptsError.value = e?.message || 'Failed to load receipts'
    receipts.value = []
    selectedReceiptID.value = ''
  } finally {
    receiptsLoading.value = false
  }
}

// Digest state
const digestLoading = ref(false)
const digestError = ref('')
const digestMarkdown = ref('')
const digestDayKey = ref('')

const loadDigest = async (refresh: boolean = false) => {
  digestLoading.value = true
  digestError.value = ''
  try {
    const d: any = await getTodayDigest(refresh)
    digestMarkdown.value = String(d?.markdown || '')
    digestDayKey.value = String(d?.day_key || '')
  } catch (e: any) {
    digestError.value = e?.message || 'Failed to load digest'
    digestMarkdown.value = ''
    digestDayKey.value = ''
  } finally {
    digestLoading.value = false
  }
}

// SOP state
const sopLoading = ref(false)
const sopError = ref('')
const sopIncludeParked = ref(false)
const sopStatusFilter = ref<SuggestionStatus>('proposed')
const sopItems = ref<Suggestion[]>([])

const editOpen = ref<Record<string, boolean>>({})
const editDraft = ref<Record<string, { title: string; description: string; risk_notes: string; draft_skill: string }>>({})

const similarLoading = ref<Record<string, boolean>>({})
const similarByID = ref<Record<string, SimilarSuggestion[]>>({})

const loadSopSuggestionsList = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    const list = await listSopSuggestions({
      status: sopStatusFilter.value,
      include_parked: sopIncludeParked.value,
      limit: 50,
    })
    sopItems.value = Array.isArray(list) ? list : []
  } catch (e: any) {
    sopError.value = e?.message || 'Failed to load SOP suggestions'
    sopItems.value = []
  } finally {
    sopLoading.value = false
  }
}

const ensureEditDraft = (s: Suggestion) => {
  if (!editDraft.value[s.suggestion_id]) {
    editDraft.value[s.suggestion_id] = {
      title: String(s.title || ''),
      description: String(s.description || ''),
      risk_notes: String(s.risk_notes || ''),
      draft_skill: String(s.draft_skill || ''),
    }
  }
}

const toggleEdit = (s: Suggestion) => {
  ensureEditDraft(s)
  editOpen.value[s.suggestion_id] = !editOpen.value[s.suggestion_id]
}

const saveEdit = async (s: Suggestion) => {
  const d = editDraft.value[s.suggestion_id]
  if (!d) return
  sopLoading.value = true
  sopError.value = ''
  try {
    await updateSopSuggestion(s.suggestion_id, {
      title: d.title,
      description: d.description,
      risk_notes: d.risk_notes,
      draft_skill: d.draft_skill,
    })
    editOpen.value[s.suggestion_id] = false
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || 'Failed to save edit'
  } finally {
    sopLoading.value = false
  }
}

const setSuggestionStatus = async (suggestionId: string, status: SuggestionStatus, mergedInto?: string) => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await updateSopSuggestionStatus(suggestionId, { status, merged_into_suggestion_id: mergedInto })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || 'Failed to update status'
  } finally {
    sopLoading.value = false
  }
}

const generateOneSuggestion = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await generateSopSuggestions({ count: 1, lookback_days: 7 })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || 'Failed to generate suggestion'
  } finally {
    sopLoading.value = false
  }
}

const loadMoreParkedSuggestions = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await loadMoreSopSuggestions({ count: 3 })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || 'Failed to load more suggestions'
  } finally {
    sopLoading.value = false
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

const formatTime = (iso: string) => {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

onMounted(async () => {
  await loadReceipts()
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto">
      <div class="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">Work Ledger</h1>
          <p class="text-sm text-surface-500">Receipts, daily digest, and SOP suggestions</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 mb-6">
        <button
          data-testid="ledger-tab-receipts"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="activeTab === 'receipts' ? 'bg-primary-500/15 text-primary-300' : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'"
          @click="activeTab = 'receipts'"
        >
          <div class="inline-flex items-center gap-2">
            <ScrollText class="w-4 h-4" />
            Receipts
          </div>
        </button>
        <button
          data-testid="ledger-tab-digest"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="activeTab === 'digest' ? 'bg-primary-500/15 text-primary-300' : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'"
          @click="activeTab = 'digest'; if (!digestMarkdown) loadDigest(false)"
        >
          <div class="inline-flex items-center gap-2">
            <FileText class="w-4 h-4" />
            Digest
          </div>
        </button>
        <button
          data-testid="ledger-tab-sop"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="activeTab === 'sop' ? 'bg-primary-500/15 text-primary-300' : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'"
          @click="activeTab = 'sop'; if (sopItems.length === 0) loadSopSuggestionsList()"
        >
          <div class="inline-flex items-center gap-2">
            <ListChecks class="w-4 h-4" />
            SOP
          </div>
        </button>
      </div>

      <!-- Receipts -->
      <div v-show="activeTab === 'receipts'" class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50">
            <div class="flex items-center justify-between gap-2">
              <p class="text-sm font-semibold text-surface-100">Receipts</p>
              <button
                data-testid="receipts-refresh"
                class="p-2 rounded-lg bg-surface-900/60 border border-surface-700/50 text-surface-200 hover:bg-surface-800/60"
                :disabled="receiptsLoading"
                @click="loadReceipts"
              >
                <RefreshCcw class="w-4 h-4" />
              </button>
            </div>
            <div class="mt-3 space-y-2">
              <div class="relative">
                <Search class="w-4 h-4 text-surface-500 absolute left-3 top-2.5" />
                <input
                  v-model="receiptsQuery"
                  data-testid="receipts-q"
                  type="text"
                  placeholder="Search receipts..."
                  class="w-full pl-9 pr-3 py-2 rounded-lg bg-surface-900/60 border border-surface-700/50 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                  @keyup.enter="loadReceipts"
                />
              </div>
              <div class="grid grid-cols-2 gap-2">
                <select
                  v-model="receiptsStatus"
                  data-testid="receipts-status"
                  class="rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm"
                  @change="loadReceipts"
                >
                  <option value="">all statuses</option>
                  <option value="succeeded">succeeded</option>
                  <option value="failed">failed</option>
                  <option value="timed_out">timed_out</option>
                  <option value="interrupted">interrupted</option>
                  <option value="canceled">canceled</option>
                </select>
                <input
                  v-model="receiptsWorkspace"
                  data-testid="receipts-workspace"
                  type="text"
                  placeholder="workspace filter"
                  class="rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                  @keyup.enter="loadReceipts"
                />
              </div>
            </div>
          </div>

          <div class="p-4">
            <div v-if="receiptsLoading" class="text-sm text-surface-500">Loading…</div>
            <div v-else-if="receiptsError" class="text-sm text-red-400">{{ receiptsError }}</div>
            <div v-else-if="receipts.length === 0" class="text-sm text-surface-500">No receipts.</div>
            <div v-else class="space-y-2">
              <button
                v-for="r in receipts"
                :key="r.receipt_id"
                data-testid="receipt-item"
                class="w-full text-left glass-card p-3 rounded-xl border border-transparent hover:border-surface-700/60"
                :class="selectedReceiptID === r.receipt_id ? 'border-primary-500/40' : ''"
                @click="selectedReceiptID = r.receipt_id"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="text-sm text-surface-100 truncate">{{ r.summary }}</p>
                    <p class="mt-1 text-[11px] text-surface-500 truncate">{{ formatTime(r.finished_at) }}</p>
                  </div>
                  <span
                    class="px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wide"
                    :class="
                      r.status === 'succeeded'
                        ? 'bg-emerald-500/15 text-emerald-200'
                        : r.status === 'failed'
                          ? 'bg-red-500/15 text-red-200'
                          : 'bg-surface-800/70 text-surface-300'
                    "
                  >
                    {{ r.status }}
                  </span>
                </div>
              </button>
            </div>
          </div>
        </div>

        <div class="glass rounded-2xl overflow-hidden lg:col-span-2">
          <div class="px-5 py-4 border-b border-surface-700/50">
            <p class="text-sm font-semibold text-surface-100">Receipt detail</p>
          </div>
          <div class="p-5">
            <div v-if="!selectedReceipt" class="text-sm text-surface-500">Select a receipt to view details.</div>
            <div v-else class="space-y-4">
              <div class="glass-card p-4 rounded-xl">
                <p class="text-sm font-semibold text-surface-100">{{ selectedReceipt.summary }}</p>
                <div class="mt-2 flex flex-wrap gap-2 text-[11px] text-surface-500">
                  <span class="px-2 py-0.5 rounded-full bg-surface-800/70">kind: {{ selectedReceipt.kind }}</span>
                  <span class="px-2 py-0.5 rounded-full bg-surface-800/70">status: {{ selectedReceipt.status }}</span>
                  <span v-if="selectedReceipt.workspace_root" class="px-2 py-0.5 rounded-full bg-surface-800/70">
                    ws: {{ selectedReceipt.workspace_root }}
                  </span>
                </div>
              </div>

              <details class="rounded-xl bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200">Artifacts</summary>
                <div class="px-4 pb-4 text-sm text-surface-200 space-y-1">
                  <p v-if="selectedReceipt.artifacts?.findings_path"><span class="text-surface-500">findings:</span> {{ selectedReceipt.artifacts.findings_path }}</p>
                  <p v-if="selectedReceipt.artifacts?.trace_log_path"><span class="text-surface-500">trace:</span> {{ selectedReceipt.artifacts.trace_log_path }}</p>
                  <p v-if="selectedReceipt.artifacts?.test_report_path"><span class="text-surface-500">test report:</span> {{ selectedReceipt.artifacts.test_report_path }}</p>
                  <p v-if="selectedReceipt.artifacts?.diff_ref"><span class="text-surface-500">diff:</span> {{ selectedReceipt.artifacts.diff_ref }}</p>
                  <p v-if="!selectedReceipt.artifacts || Object.keys(selectedReceipt.artifacts || {}).length === 0" class="text-surface-500">
                    (none)
                  </p>
                </div>
              </details>

              <details class="rounded-xl bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200">Raw JSON</summary>
                <pre class="whitespace-pre-wrap text-xs text-surface-100 px-4 pb-4">{{ JSON.stringify(selectedReceipt, null, 2) }}</pre>
              </details>
            </div>
          </div>
        </div>
      </div>

      <!-- Digest -->
      <div v-show="activeTab === 'digest'" class="glass rounded-2xl overflow-hidden">
        <div class="px-6 py-4 border-b border-surface-700/50">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3">
              <FileText class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">Daily Digest</h2>
                <p class="text-sm text-surface-500">A quick summary of today's receipts</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="digestDayKey" class="text-xs text-surface-500">{{ digestDayKey }}</span>
              <button
                data-testid="digest-refresh"
                class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                :disabled="digestLoading"
                @click="loadDigest(true)"
              >
                {{ digestLoading ? 'Loading…' : 'Refresh' }}
              </button>
            </div>
          </div>
        </div>

        <div class="p-6 space-y-3">
          <div v-if="digestLoading" class="text-sm text-surface-500">Loading…</div>
          <div v-else-if="digestError" class="text-sm text-red-400">{{ digestError }}</div>
          <div v-else class="prose prose-invert max-w-none">
            <pre class="whitespace-pre-wrap text-sm bg-surface-900/60 border border-surface-700/50 rounded-xl p-4">{{ digestMarkdown || '(empty)' }}</pre>
          </div>
        </div>
      </div>

      <!-- SOP -->
      <div v-show="activeTab === 'sop'" class="glass rounded-2xl overflow-hidden">
        <div class="px-6 py-4 border-b border-surface-700/50">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3">
              <ListChecks class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">SOP Suggestions</h2>
                <p class="text-sm text-surface-500">Manual review → approve to materialize as personal skills</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button
                data-testid="sop-generate"
                class="px-3 py-2 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-500"
                :disabled="sopLoading"
                @click="generateOneSuggestion"
              >
                <span class="inline-flex items-center gap-2">
                  <Wand2 class="w-4 h-4" />
                  {{ sopLoading ? 'Working…' : 'Generate' }}
                </span>
              </button>
              <button
                class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                :disabled="sopLoading"
                @click="loadMoreParkedSuggestions"
              >
                Load more
              </button>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-2">
              <select
                v-model="sopStatusFilter"
                data-testid="sop-status"
                class="rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-xs"
                @change="loadSopSuggestionsList"
              >
                <option value="proposed">proposed</option>
                <option value="parked">parked</option>
                <option value="approved">approved</option>
                <option value="rejected">rejected</option>
                <option value="archived">archived</option>
              </select>
              <label class="flex items-center gap-2 text-xs text-surface-400">
                <input v-model="sopIncludeParked" type="checkbox" class="accent-primary-500" @change="loadSopSuggestionsList" />
                Include parked
              </label>
            </div>
            <button
              class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-900/60 border border-surface-700/50 text-surface-200 hover:bg-surface-800/60"
              :disabled="sopLoading"
              @click="loadSopSuggestionsList"
            >
              Refresh
            </button>
          </div>
        </div>

        <div class="p-6 space-y-4">
          <div v-if="sopLoading" class="text-sm text-surface-500">Loading…</div>
          <div v-else-if="sopError" class="text-sm text-red-400">{{ sopError }}</div>
          <div v-else-if="sopItems.length === 0" class="text-sm text-surface-500">
            No suggestions. Click Generate to propose one from recent receipts.
          </div>
          <div v-else class="space-y-4">
            <div v-for="s in sopItems" :key="s.suggestion_id" class="glass-card p-4 space-y-3">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-surface-100 truncate">{{ s.title }}</p>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-surface-500">
                    <span class="px-2 py-0.5 rounded-full bg-surface-800/70">{{ s.status }}</span>
                    <span class="px-2 py-0.5 rounded-full bg-surface-800/70">evidence: {{ s.evidence_count }}</span>
                    <span v-if="s.scores" class="px-2 py-0.5 rounded-full bg-surface-800/70">score: {{ s.scores.total_score.toFixed(2) }}</span>
                    <span v-if="s.meta?.compression_verdict" class="px-2 py-0.5 rounded-full bg-surface-800/70">depth: {{ s.meta.compression_verdict }}</span>
                  </div>
                </div>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    v-if="s.status !== 'approved' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-emerald-500/15 text-emerald-200 hover:bg-emerald-500/25"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'approved')"
                  >
                    Approve
                  </button>
                  <button
                    v-if="s.status !== 'rejected' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-red-500/10 text-red-200 hover:bg-red-500/20"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'rejected')"
                  >
                    Reject
                  </button>
                  <button
                    v-if="s.status === 'proposed'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'parked')"
                  >
                    Park
                  </button>
                  <button
                    v-if="s.status !== 'archived' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'archived')"
                  >
                    Archive
                  </button>
                  <button
                    v-if="s.status === 'proposed' || s.status === 'parked'"
                    data-testid="sop-edit"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-primary-500/10 text-primary-200 hover:bg-primary-500/20"
                    :disabled="sopLoading"
                    @click="toggleEdit(s)"
                  >
                    {{ editOpen[s.suggestion_id] ? 'Close' : 'Edit' }}
                  </button>
                  <button
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading || similarLoading[s.suggestion_id]"
                    @click="loadSimilar(s)"
                  >
                    {{ similarLoading[s.suggestion_id] ? '…' : 'Similar' }}
                  </button>
                </div>
              </div>

              <div v-if="editOpen[s.suggestion_id]" class="space-y-2 rounded-xl bg-surface-900/60 border border-surface-700/50 p-3">
                <input
                  v-model="editDraft[s.suggestion_id].title"
                  data-testid="sop-edit-title"
                  type="text"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm"
                  placeholder="Title"
                />
                <textarea
                  v-model="editDraft[s.suggestion_id].draft_skill"
                  data-testid="sop-edit-draft"
                  rows="8"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-xs font-mono"
                  placeholder="Draft skill (SKILL.md body)"
                />
                <div class="flex justify-end gap-2">
                  <button
                    data-testid="sop-edit-save"
                    class="px-3 py-2 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-500"
                    :disabled="sopLoading"
                    @click="saveEdit(s)"
                  >
                    Save
                  </button>
                </div>
              </div>

              <div v-if="Array.isArray(similarByID[s.suggestion_id]) && similarByID[s.suggestion_id].length > 0" class="rounded-xl bg-surface-900/60 border border-surface-700/50 p-3">
                <p class="text-xs text-surface-400 mb-2">Similar suggestions</p>
                <div class="space-y-2">
                  <div v-for="sim in similarByID[s.suggestion_id]" :key="sim.suggestion_id" class="flex items-center justify-between gap-2">
                    <div class="min-w-0">
                      <p class="text-xs text-surface-200 truncate">{{ sim.title }}</p>
                      <p class="text-[11px] text-surface-500">sim={{ sim.similarity.toFixed(2) }} · {{ sim.status }} · id={{ sim.suggestion_id.slice(0, 8) }}</p>
                    </div>
                    <button
                      v-if="(s.status === 'proposed' || s.status === 'parked') && sim.status !== 'merged'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                      :disabled="sopLoading"
                      @click="setSuggestionStatus(s.suggestion_id, 'merged', sim.suggestion_id)"
                    >
                      Merge into
                    </button>
                  </div>
                </div>
              </div>

              <details class="rounded-lg bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300">Draft</summary>
                <pre class="whitespace-pre-wrap text-xs text-surface-100 px-3 pb-3">{{ s.draft_skill }}</pre>
              </details>
              <details v-if="s.status === 'approved' && s.meta?.materialized_skill_path" class="rounded-lg bg-surface-900/60 border border-surface-700/50">
                <summary class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300">Materialized skill</summary>
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

