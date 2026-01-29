<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  FileText,
  ListChecks,
  ScrollText,
  RefreshCcw,
  Search,
  Wand2,
} from "lucide-vue-next";

import ErrorBanner from "@/components/ErrorBanner.vue";
import {
  getLedgerStatusToday,
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
} from "@/api/client";
import { parseApiError, type ParsedApiError } from "@/lib/apiError";

const activeTab = ref<"receipts" | "digest" | "sop">("receipts");

// Ledger status badges
const statusLoading = ref(false);
const statusError = ref<ParsedApiError | null>(null);
const ledgerStatus = ref<{
  day_key: string;
  digest_exists: boolean;
  learning_job_status: string;
  sop_proposed_count: number;
} | null>(null);

const loadLedgerStatus = async () => {
  statusLoading.value = true;
  statusError.value = null;
  try {
    const st = await getLedgerStatusToday();
    ledgerStatus.value = st;
  } catch (e: any) {
    statusError.value = parseApiError(e, "加载 ledger 状态失败");
    ledgerStatus.value = null;
  } finally {
    statusLoading.value = false;
  }
};

const hasDigestBadge = computed(() => !!ledgerStatus.value?.digest_exists);
const sopProposedCount = computed(() =>
  Number(ledgerStatus.value?.sop_proposed_count || 0),
);
const learningStatus = computed(() =>
  String(ledgerStatus.value?.learning_job_status || "none"),
);
const hasLearningBadge = computed(() =>
  ["queued", "running", "failed"].includes(learningStatus.value),
);

// Receipts state
const receiptsLoading = ref(false);
const receiptsError = ref<ParsedApiError | null>(null);
const receipts = ref<Receipt[]>([]);
const receiptsQuery = ref("");
const receiptsStatus = ref<string>("");
const receiptsWorkspace = ref(localStorage.getItem("oneagent-workspace") || "");
const selectedReceiptID = ref<string>("");

const selectedReceipt = computed(() =>
  receipts.value.find((r) => r.receipt_id === selectedReceiptID.value),
);

const loadReceipts = async () => {
  receiptsLoading.value = true;
  receiptsError.value = null;
  try {
    const list = await listReceipts({
      q: receiptsQuery.value.trim() || undefined,
      status: (receiptsStatus.value.trim() as any) || undefined,
      workspace: receiptsWorkspace.value.trim() || undefined,
      limit: 50,
    });
    receipts.value = Array.isArray(list) ? list : [];
    if (
      selectedReceiptID.value &&
      !receipts.value.some((r) => r.receipt_id === selectedReceiptID.value)
    ) {
      selectedReceiptID.value = "";
    }
  } catch (e: any) {
    receiptsError.value = parseApiError(e, "加载 receipts 失败");
    receipts.value = [];
    selectedReceiptID.value = "";
  } finally {
    receiptsLoading.value = false;
  }
};

// Digest state
const digestLoading = ref(false);
const digestError = ref<ParsedApiError | null>(null);
const digestMarkdown = ref("");
const digestDayKey = ref("");

const loadDigest = async (refresh: boolean = false) => {
  digestLoading.value = true;
  digestError.value = null;
  try {
    const d: any = await getTodayDigest(refresh);
    digestMarkdown.value = String(d?.markdown || "");
    digestDayKey.value = String(d?.day_key || "");
  } catch (e: any) {
    digestError.value = parseApiError(e, "加载 digest 失败");
    digestMarkdown.value = "";
    digestDayKey.value = "";
  } finally {
    digestLoading.value = false;
  }
};

// SOP state
const sopLoading = ref(false);
const sopError = ref<ParsedApiError | null>(null);
const sopIncludeParked = ref(false);
const sopStatusFilter = ref<SuggestionStatus>("proposed");
const sopItems = ref<Suggestion[]>([]);

const editOpen = ref<Record<string, boolean>>({});
const editDraft = ref<
  Record<
    string,
    {
      title: string;
      description: string;
      risk_notes: string;
      draft_skill: string;
    }
  >
>({});

const similarLoading = ref<Record<string, boolean>>({});
const similarByID = ref<Record<string, SimilarSuggestion[]>>({});

const loadSopSuggestionsList = async () => {
  sopLoading.value = true;
  sopError.value = null;
  try {
    const list = await listSopSuggestions({
      status: sopStatusFilter.value,
      include_parked: sopIncludeParked.value,
      limit: 50,
    });
    sopItems.value = Array.isArray(list) ? list : [];
  } catch (e: any) {
    sopError.value = parseApiError(e, "加载 SOP 建议失败");
    sopItems.value = [];
  } finally {
    sopLoading.value = false;
  }
};

const ensureEditDraft = (s: Suggestion) => {
  if (!editDraft.value[s.suggestion_id]) {
    editDraft.value[s.suggestion_id] = {
      title: String(s.title || ""),
      description: String(s.description || ""),
      risk_notes: String(s.risk_notes || ""),
      draft_skill: String(s.draft_skill || ""),
    };
  }
};

const toggleEdit = (s: Suggestion) => {
  ensureEditDraft(s);
  editOpen.value[s.suggestion_id] = !editOpen.value[s.suggestion_id];
};

const saveEdit = async (s: Suggestion) => {
  const d = editDraft.value[s.suggestion_id];
  if (!d) return;
  sopLoading.value = true;
  sopError.value = null;
  try {
    await updateSopSuggestion(s.suggestion_id, {
      title: d.title,
      description: d.description,
      risk_notes: d.risk_notes,
      draft_skill: d.draft_skill,
    });
    editOpen.value[s.suggestion_id] = false;
    await loadSopSuggestionsList();
    await loadLedgerStatus();
  } catch (e: any) {
    sopError.value = parseApiError(e, "保存失败");
  } finally {
    sopLoading.value = false;
  }
};

const setSuggestionStatus = async (
  suggestionId: string,
  status: SuggestionStatus,
  mergedInto?: string,
) => {
  sopLoading.value = true;
  sopError.value = null;
  try {
    await updateSopSuggestionStatus(suggestionId, {
      status,
      merged_into_suggestion_id: mergedInto,
    });
    await loadSopSuggestionsList();
    await loadLedgerStatus();
  } catch (e: any) {
    sopError.value = parseApiError(e, "更新状态失败");
  } finally {
    sopLoading.value = false;
  }
};

const generateOneSuggestion = async () => {
  sopLoading.value = true;
  sopError.value = null;
  try {
    await generateSopSuggestions({ count: 1, lookback_days: 7 });
    await loadSopSuggestionsList();
    await loadLedgerStatus();
  } catch (e: any) {
    sopError.value = parseApiError(e, "生成失败");
  } finally {
    sopLoading.value = false;
  }
};

const loadMoreParkedSuggestions = async () => {
  sopLoading.value = true;
  sopError.value = null;
  try {
    await loadMoreSopSuggestions({ count: 3 });
    await loadSopSuggestionsList();
    await loadLedgerStatus();
  } catch (e: any) {
    sopError.value = parseApiError(e, "加载更多失败");
  } finally {
    sopLoading.value = false;
  }
};

const loadSimilar = async (s: Suggestion) => {
  similarLoading.value[s.suggestion_id] = true;
  try {
    const list = await getSimilarSopSuggestions(s.suggestion_id, 8);
    similarByID.value[s.suggestion_id] = Array.isArray(list) ? list : [];
  } catch {
    similarByID.value[s.suggestion_id] = [];
  } finally {
    similarLoading.value[s.suggestion_id] = false;
  }
};

const formatTime = (iso: string) => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("zh-CN");
};

onMounted(async () => {
  await loadReceipts();
  await loadLedgerStatus();
});
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto">
      <div class="flex items-start justify-between gap-4 mb-6">
        <div>
          <div class="flex items-center gap-3">
            <h1 class="text-2xl font-bold text-surface-100">流水账</h1>
            <span
              v-if="hasLearningBadge"
              data-testid="ledger-badge-learning"
              class="px-2 py-1 rounded-full text-xs font-medium bg-amber-500/15 text-amber-300"
            >
              学习：{{ learningStatus }}
            </span>
          </div>
          <p class="text-sm text-surface-500">回执、日报与 SOP 建议</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 mb-6">
        <button
          data-testid="ledger-tab-receipts"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="
            activeTab === 'receipts'
              ? 'bg-primary-500/15 text-primary-300'
              : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'
          "
          @click="activeTab = 'receipts'"
        >
          <div class="inline-flex items-center gap-2">
            <ScrollText class="w-4 h-4" />
            回执
          </div>
        </button>
        <button
          data-testid="ledger-tab-digest"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="
            activeTab === 'digest'
              ? 'bg-primary-500/15 text-primary-300'
              : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'
          "
          @click="
            activeTab = 'digest';
            if (!digestMarkdown) loadDigest(false);
          "
        >
          <div class="inline-flex items-center gap-2">
            <FileText class="w-4 h-4" />
            日报
            <span
              v-if="hasDigestBadge"
              data-testid="ledger-badge-digest"
              class="w-2 h-2 rounded-full bg-emerald-400"
            ></span>
          </div>
        </button>
        <button
          data-testid="ledger-tab-sop"
          class="px-4 py-2 rounded-xl text-sm font-medium"
          :class="
            activeTab === 'sop'
              ? 'bg-primary-500/15 text-primary-300'
              : 'bg-surface-900/60 text-surface-300 hover:bg-surface-800/60'
          "
          @click="
            activeTab = 'sop';
            if (sopItems.length === 0) loadSopSuggestionsList();
          "
        >
          <div class="inline-flex items-center gap-2">
            <ListChecks class="w-4 h-4" />
            SOP
            <span
              v-if="sopProposedCount > 0"
              data-testid="ledger-badge-sop-count"
              class="px-2 py-0.5 rounded-full text-xs font-semibold bg-primary-500/15 text-primary-300"
            >
              {{ sopProposedCount }}
            </span>
          </div>
        </button>
      </div>

      <!-- Receipts -->
      <div
        v-show="activeTab === 'receipts'"
        class="grid grid-cols-1 lg:grid-cols-3 gap-4"
      >
        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50">
            <div class="flex items-center justify-between gap-2">
              <p class="text-sm font-semibold text-surface-100">回执</p>
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
                <Search
                  class="w-4 h-4 text-surface-500 absolute left-3 top-2.5"
                />
                <input
                  v-model="receiptsQuery"
                  data-testid="receipts-q"
                  type="text"
                  placeholder="搜索回执..."
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
                  <option value="">全部状态</option>
                  <option value="succeeded">成功</option>
                  <option value="failed">失败</option>
                  <option value="timed_out">超时</option>
                  <option value="interrupted">中断</option>
                  <option value="canceled">已取消</option>
                </select>
                <input
                  v-model="receiptsWorkspace"
                  data-testid="receipts-workspace"
                  type="text"
                  placeholder="工作区过滤"
                  class="rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                  @keyup.enter="loadReceipts"
                />
              </div>
            </div>
          </div>

          <div class="p-4">
            <div v-if="receiptsLoading" class="text-sm text-surface-500">
              加载中…
            </div>
            <ErrorBanner v-else-if="receiptsError" :error="receiptsError" title="加载失败" />
            <div
              v-else-if="receipts.length === 0"
              class="text-sm text-surface-500"
            >
              暂无回执。
            </div>
            <div v-else class="space-y-2">
              <button
                v-for="r in receipts"
                :key="r.receipt_id"
                data-testid="receipt-item"
                class="w-full text-left glass-card p-3 rounded-xl border border-transparent hover:border-surface-700/60"
                :class="
                  selectedReceiptID === r.receipt_id
                    ? 'border-primary-500/40'
                    : ''
                "
                @click="selectedReceiptID = r.receipt_id"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="text-sm text-surface-100 truncate">
                      {{ r.summary }}
                    </p>
                    <p class="mt-1 text-[11px] text-surface-500 truncate">
                      {{ formatTime(r.finished_at) }}
                    </p>
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
            <p class="text-sm font-semibold text-surface-100">回执详情</p>
          </div>
          <div class="p-5">
            <div v-if="!selectedReceipt" class="text-sm text-surface-500">
              选择回执查看详情。
            </div>
            <div v-else class="space-y-4">
              <div class="glass-card p-4 rounded-xl">
                <p class="text-sm font-semibold text-surface-100">
                  {{ selectedReceipt.summary }}
                </p>
                <div
                  class="mt-2 flex flex-wrap gap-2 text-[11px] text-surface-500"
                >
                  <span class="px-2 py-0.5 rounded-full bg-surface-800/70"
                    >kind: {{ selectedReceipt.kind }}</span
                  >
                  <span class="px-2 py-0.5 rounded-full bg-surface-800/70"
                    >status: {{ selectedReceipt.status }}</span
                  >
                  <span
                    v-if="selectedReceipt.workspace_root"
                    class="px-2 py-0.5 rounded-full bg-surface-800/70"
                  >
                    ws: {{ selectedReceipt.workspace_root }}
                  </span>
                </div>
              </div>

              <details
                class="rounded-xl bg-surface-900/60 border border-surface-700/50"
              >
                <summary
                  class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200"
                >
                  产物
                </summary>
                <div class="px-4 pb-4 text-sm text-surface-200 space-y-1">
                  <p v-if="selectedReceipt.artifacts?.findings_path">
                    <span class="text-surface-500">findings：</span>
                    {{ selectedReceipt.artifacts.findings_path }}
                  </p>
                  <p v-if="selectedReceipt.artifacts?.trace_log_path">
                    <span class="text-surface-500">trace：</span>
                    {{ selectedReceipt.artifacts.trace_log_path }}
                  </p>
                  <p v-if="selectedReceipt.artifacts?.test_report_path">
                    <span class="text-surface-500">测试报告：</span>
                    {{ selectedReceipt.artifacts.test_report_path }}
                  </p>
                  <p v-if="selectedReceipt.artifacts?.diff_ref">
                    <span class="text-surface-500">diff：</span>
                    {{ selectedReceipt.artifacts.diff_ref }}
                  </p>
                  <p
                    v-if="
                      !selectedReceipt.artifacts ||
                      Object.keys(selectedReceipt.artifacts || {}).length === 0
                    "
                    class="text-surface-500"
                  >
                    （无）
                  </p>
                </div>
              </details>

              <details
                class="rounded-xl bg-surface-900/60 border border-surface-700/50"
              >
                <summary
                  class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200"
                >
                  指标
                </summary>
                <div class="px-4 pb-4 text-sm text-surface-200 space-y-1">
                  <p v-if="selectedReceipt.signals?.duration_ms">
                    <span class="text-surface-500">duration_ms:</span>
                    {{ selectedReceipt.signals.duration_ms }}
                  </p>
                  <p v-if="selectedReceipt.signals?.calls">
                    <span class="text-surface-500">calls:</span>
                    {{ selectedReceipt.signals.calls }}
                  </p>
                  <p v-if="selectedReceipt.signals?.total_tokens">
                    <span class="text-surface-500">total_tokens:</span>
                    {{ selectedReceipt.signals.total_tokens }}
                  </p>
                  <p v-if="selectedReceipt.signals?.prompt_tokens">
                    <span class="text-surface-500">prompt_tokens:</span>
                    {{ selectedReceipt.signals.prompt_tokens }}
                  </p>
                  <p v-if="selectedReceipt.signals?.completion_tokens">
                    <span class="text-surface-500">completion_tokens:</span>
                    {{ selectedReceipt.signals.completion_tokens }}
                  </p>
                  <p v-if="selectedReceipt.signals?.cost_usd">
                    <span class="text-surface-500">cost_usd:</span>
                    {{ selectedReceipt.signals.cost_usd }}
                  </p>
                  <p
                    v-if="
                      !selectedReceipt.signals ||
                      Object.keys(selectedReceipt.signals || {}).length === 0
                    "
                    class="text-surface-500"
                  >
                    （无）
                  </p>
                </div>
              </details>

              <details
                class="rounded-xl bg-surface-900/60 border border-surface-700/50"
              >
                <summary
                  class="cursor-pointer select-none px-4 py-3 text-sm text-surface-200"
                >
                  原始 JSON
                </summary>
                <pre
                  class="whitespace-pre-wrap text-xs text-surface-100 px-4 pb-4"
                  >{{ JSON.stringify(selectedReceipt, null, 2) }}</pre
                >
              </details>
            </div>
          </div>
        </div>
      </div>

      <!-- Digest -->
      <div
        v-show="activeTab === 'digest'"
        class="glass rounded-2xl overflow-hidden"
      >
        <div class="px-6 py-4 border-b border-surface-700/50">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3">
              <FileText class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">今日日报</h2>
                <p class="text-sm text-surface-500">今日回执的快速摘要</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="digestDayKey" class="text-xs text-surface-500">{{
                digestDayKey
              }}</span>
              <button
                data-testid="digest-refresh"
                class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                :disabled="digestLoading"
                @click="loadDigest(true)"
              >
                {{ digestLoading ? "加载中…" : "刷新" }}
              </button>
            </div>
          </div>
        </div>

        <div class="p-6 space-y-3">
          <div v-if="digestLoading" class="text-sm text-surface-500">
            加载中…
          </div>
          <ErrorBanner v-else-if="digestError" :error="digestError" title="加载失败" />
          <div v-else class="prose prose-invert max-w-none">
            <pre
              class="whitespace-pre-wrap text-sm bg-surface-900/60 border border-surface-700/50 rounded-xl p-4"
              >{{ digestMarkdown || "（空）" }}</pre
            >
          </div>
        </div>
      </div>

      <!-- SOP -->
      <div
        v-show="activeTab === 'sop'"
        class="glass rounded-2xl overflow-hidden"
      >
        <div class="px-6 py-4 border-b border-surface-700/50">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3">
              <ListChecks class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">SOP 建议</h2>
                <p class="text-sm text-surface-500">
                  人工审核 → 通过后可落地为个人技能
                </p>
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
                  {{ sopLoading ? "处理中…" : "生成" }}
                </span>
              </button>
              <button
                class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                :disabled="sopLoading"
                @click="loadMoreParkedSuggestions"
              >
                加载更多
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
                <option value="proposed">待处理</option>
                <option value="parked">搁置</option>
                <option value="approved">已通过</option>
                <option value="rejected">已拒绝</option>
                <option value="archived">已归档</option>
              </select>
              <label class="flex items-center gap-2 text-xs text-surface-400">
                <input
                  v-model="sopIncludeParked"
                  type="checkbox"
                  class="accent-primary-500"
                  @change="loadSopSuggestionsList"
                />
                包含搁置
              </label>
            </div>
            <button
              class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-900/60 border border-surface-700/50 text-surface-200 hover:bg-surface-800/60"
              :disabled="sopLoading"
              @click="loadSopSuggestionsList"
            >
              刷新
            </button>
          </div>
        </div>

        <div class="p-6 space-y-4">
          <div v-if="sopLoading" class="text-sm text-surface-500">加载中…</div>
          <ErrorBanner v-else-if="sopError" :error="sopError" title="操作失败" />
          <div
            v-else-if="sopItems.length === 0"
            class="text-sm text-surface-500"
          >
            暂无建议。点击“生成”从近期回执中提取一条。
          </div>
          <div v-else class="space-y-4">
            <div
              v-for="s in sopItems"
              :key="s.suggestion_id"
              class="glass-card p-4 space-y-3"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-surface-100 truncate">
                    {{ s.title }}
                  </p>
                  <div
                    class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-surface-500"
                  >
                    <span class="px-2 py-0.5 rounded-full bg-surface-800/70">{{
                      s.status
                    }}</span>
                    <span class="px-2 py-0.5 rounded-full bg-surface-800/70"
                      >证据：{{ s.evidence_count }}</span
                    >
                    <span
                      v-if="s.scores"
                      class="px-2 py-0.5 rounded-full bg-surface-800/70"
                      >得分：{{ s.scores.total_score.toFixed(2) }}</span
                    >
                    <span
                      v-if="s.meta?.compression_verdict"
                      class="px-2 py-0.5 rounded-full bg-surface-800/70"
                      >深度：{{ s.meta.compression_verdict }}</span
                    >
                  </div>
                </div>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    v-if="s.status !== 'approved' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-emerald-500/15 text-emerald-200 hover:bg-emerald-500/25"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'approved')"
                  >
                    通过
                  </button>
                  <button
                    v-if="s.status !== 'rejected' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-red-500/10 text-red-200 hover:bg-red-500/20"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'rejected')"
                  >
                    拒绝
                  </button>
                  <button
                    v-if="s.status === 'proposed'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'parked')"
                  >
                    搁置
                  </button>
                  <button
                    v-if="s.status !== 'archived' && s.status !== 'merged'"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'archived')"
                  >
                    归档
                  </button>
                  <button
                    v-if="s.status === 'proposed' || s.status === 'parked'"
                    data-testid="sop-edit"
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-primary-500/10 text-primary-200 hover:bg-primary-500/20"
                    :disabled="sopLoading"
                    @click="toggleEdit(s)"
                  >
                    {{ editOpen[s.suggestion_id] ? "关闭" : "编辑" }}
                  </button>
                  <button
                    class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading || similarLoading[s.suggestion_id]"
                    @click="loadSimilar(s)"
                  >
                    {{ similarLoading[s.suggestion_id] ? "…" : "相似" }}
                  </button>
                </div>
              </div>

              <div
                v-if="editOpen[s.suggestion_id]"
                class="space-y-2 rounded-xl bg-surface-900/60 border border-surface-700/50 p-3"
              >
                <input
                  v-model="editDraft[s.suggestion_id].title"
                  data-testid="sop-edit-title"
                  type="text"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-sm"
                  placeholder="标题"
                />
                <textarea
                  v-model="editDraft[s.suggestion_id].draft_skill"
                  data-testid="sop-edit-draft"
                  rows="8"
                  class="w-full rounded-lg bg-surface-900/60 border border-surface-700/50 px-3 py-2 text-surface-100 text-xs font-mono"
                  placeholder="草稿技能（SKILL.md 内容）"
                />
                <div class="flex justify-end gap-2">
                  <button
                    data-testid="sop-edit-save"
                    class="px-3 py-2 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-500"
                    :disabled="sopLoading"
                    @click="saveEdit(s)"
                  >
                    保存
                  </button>
                </div>
              </div>

              <div
                v-if="
                  Array.isArray(similarByID[s.suggestion_id]) &&
                  similarByID[s.suggestion_id].length > 0
                "
                class="rounded-xl bg-surface-900/60 border border-surface-700/50 p-3"
              >
                <p class="text-xs text-surface-400 mb-2">相似建议</p>
                <div class="space-y-2">
                  <div
                    v-for="sim in similarByID[s.suggestion_id]"
                    :key="sim.suggestion_id"
                    class="flex items-center justify-between gap-2"
                  >
                    <div class="min-w-0">
                      <p class="text-xs text-surface-200 truncate">
                        {{ sim.title }}
                      </p>
                      <p class="text-[11px] text-surface-500">
                        sim={{ sim.similarity.toFixed(2) }} · {{ sim.status }} ·
                        id={{ sim.suggestion_id.slice(0, 8) }}
                      </p>
                    </div>
                    <button
                      v-if="
                        (s.status === 'proposed' || s.status === 'parked') &&
                        sim.status !== 'merged'
                      "
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                      :disabled="sopLoading"
                      @click="
                        setSuggestionStatus(
                          s.suggestion_id,
                          'merged',
                          sim.suggestion_id,
                        )
                      "
                    >
                      合并到
                    </button>
                  </div>
                </div>
              </div>

              <details
                class="rounded-lg bg-surface-900/60 border border-surface-700/50"
              >
                <summary
                  class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300"
                >
                  草稿
                </summary>
                <pre
                  class="whitespace-pre-wrap text-xs text-surface-100 px-3 pb-3"
                  >{{ s.draft_skill }}</pre
                >
              </details>
              <details
                v-if="
                  s.status === 'approved' && s.meta?.materialized_skill_path
                "
                class="rounded-lg bg-surface-900/60 border border-surface-700/50"
              >
                <summary
                  class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300"
                >
                  已落地技能
                </summary>
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
