<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import {
  Folder,
  ListTodo,
  RefreshCw,
  RotateCcw,
  X,
  Plus,
} from "lucide-vue-next";

import {
  cancelTask,
  createTask,
  getTask,
  getTaskAttemptChangedFiles,
  getTaskAttemptDiffPatch,
  getTaskEvents,
  listTasks,
  listTaskAttemptReviewComments,
  postTaskAttemptReviewComment,
  resumeTask,
  type Task,
  type TaskAttempt,
  type TaskAttemptArtifactContent,
  type TaskAttemptReviewComment,
  type TaskEvent,
} from "@/api/client";
import {
  diffTaskUpdates,
  saveTaskSnapshotsToStorage,
  type TaskSnapshot,
  type TaskUpdate,
} from "@/lib/taskUpdates";

type WorkspaceSummary = {
  workspace: string;
  total: number;
  queued: number;
  running: number;
  failed: number;
};

const STORAGE_KEY = "oneagent-workspaces";
const TASK_SNAPSHOT_KEY = "oneagent-task-snapshots";

const tasksLoading = ref(false);
const tasksError = ref("");
const tasks = ref<Task[]>([]);

const notifyInitialized = ref(false);
const taskSnapshots = ref<Record<string, TaskSnapshot>>({});
const taskUpdates = ref<TaskUpdate[]>([]);

const workspacesManual = ref<string[]>([]);
const workspaceNew = ref("");
const workspaceSelected = ref("");

const selectedTaskId = ref("");
const selectedTask = ref<Task | null>(null);
const selectedEvents = ref<TaskEvent[]>([]);
const selectedLoading = ref(false);
const selectedError = ref("");

const title = ref("");
const prompt = ref("");
const budgetTokens = ref("");
const budgetCost = ref("");
const submitting = ref(false);
const composerAdvancedOpen = ref(false);
const detailsAdvancedOpen = ref(false);
const reviewOpen = ref(false);

const reviewLoading = ref(false);
const reviewError = ref("");
const diffPatch = ref<TaskAttemptArtifactContent | null>(null);
const changedFiles = ref<TaskAttemptArtifactContent | null>(null);
const reviewComments = ref<TaskAttemptReviewComment[]>([]);
const reviewCommentDraft = ref("");
const followUpNotes = ref("");
const submittingReviewComment = ref(false);
const submittingFollowUp = ref(false);

// Tab navigation for task details
const activeDetailTab = ref("overview");
const detailTabs = [
  { key: "overview", label: "概览" },
  { key: "review", label: "审查" },
  { key: "events", label: "事件" },
  { key: "advanced", label: "高级" },
];

// Reset tab when task changes
watch(selectedTaskId, () => {
  activeDetailTab.value = 'overview';
});

const latestAttempt = computed<TaskAttempt | null>(() => {
  const t = selectedTask.value;
  if (!t || !Array.isArray(t.attempts) || t.attempts.length === 0) return null;
  return t.attempts[t.attempts.length - 1] || null;
});

const latestStatus = computed(() => latestAttempt.value?.status || "");

const canCancel = computed(
  () => latestStatus.value === "queued" || latestStatus.value === "running",
);
const canResume = computed(() =>
  ["succeeded", "failed", "canceled", "limit_exceeded", "timed_out", "interrupted"].includes(
    latestStatus.value,
  ),
);

const normalizeWorkspace = (ws: string) => String(ws || "").trim();
const shortHash = (hash?: string) =>
  hash && hash.length >= 8 ? hash.slice(0, 8) : hash || "";

const loadManualWorkspaces = () => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const arr = raw ? JSON.parse(raw) : [];
    workspacesManual.value = Array.isArray(arr)
      ? arr.map(normalizeWorkspace).filter(Boolean)
      : [];
  } catch {
    workspacesManual.value = [];
  }
};

const saveManualWorkspaces = () => {
  const uniq = Array.from(
    new Set(workspacesManual.value.map(normalizeWorkspace).filter(Boolean)),
  );
  workspacesManual.value = uniq;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(uniq));
};

const refreshTasks = async () => {
  tasksError.value = "";
  tasksLoading.value = true;
  try {
    const list = await listTasks();
    const nextTasks = Array.isArray(list) ? list : [];

    if (!notifyInitialized.value) {
      // Establish baseline without emitting notifications.
      const { next } = diffTaskUpdates({}, nextTasks);
      taskSnapshots.value = next;
      saveTaskSnapshotsToStorage(TASK_SNAPSHOT_KEY, taskSnapshots.value);
      notifyInitialized.value = true;
    } else {
      const { next, updates } = diffTaskUpdates(taskSnapshots.value, nextTasks);
      taskSnapshots.value = next;
      saveTaskSnapshotsToStorage(TASK_SNAPSHOT_KEY, next);
      if (updates.length) {
        const seen = new Set(
          taskUpdates.value.map(
            (u) => `${u.taskId}:${u.attemptId}:${u.status}`,
          ),
        );
        const merged = [
          ...updates.filter(
            (u) => !seen.has(`${u.taskId}:${u.attemptId}:${u.status}`),
          ),
          ...taskUpdates.value,
        ];
        taskUpdates.value = merged.slice(0, 10);
      }
    }

    tasks.value = nextTasks;
  } catch (e: any) {
    tasksError.value = String(
      e?.data?.error || e?.message || "Failed to load tasks",
    );
    tasks.value = [];
  } finally {
    tasksLoading.value = false;
  }
};

const clearTaskUpdates = () => {
  taskUpdates.value = [];
};

const refreshSelected = async () => {
  selectedError.value = "";
  const id = normalizeWorkspace(selectedTaskId.value);
  if (!id) {
    selectedTask.value = null;
    selectedEvents.value = [];
    return;
  }

  selectedLoading.value = true;
  try {
    const [t, evs] = await Promise.all([getTask(id), getTaskEvents(id)]);
    selectedTask.value = t;
    selectedEvents.value = Array.isArray(evs) ? evs : [];
  } catch (e: any) {
    selectedError.value = String(
      e?.data?.error || e?.message || "Failed to load task",
    );
    selectedTask.value = null;
    selectedEvents.value = [];
  } finally {
    selectedLoading.value = false;
  }
};

const loadReviewArtifacts = async () => {
  reviewError.value = "";
  diffPatch.value = null;
  changedFiles.value = null;
  reviewComments.value = [];

  const t = selectedTask.value;
  const a = latestAttempt.value;
  if (!t || !a) return;

  reviewLoading.value = true;
  try {
    const [diff, changed, comments] = await Promise.all([
      getTaskAttemptDiffPatch(t.id, a.id).catch(() => null),
      getTaskAttemptChangedFiles(t.id, a.id).catch(() => null),
      listTaskAttemptReviewComments(t.id, a.id).catch(() => []),
    ]);
    diffPatch.value = diff;
    changedFiles.value = changed;
    reviewComments.value = Array.isArray(comments) ? comments : [];
  } catch (e: any) {
    reviewError.value = String(
      e?.data?.error || e?.message || "Failed to load review artifacts",
    );
  } finally {
    reviewLoading.value = false;
  }
};

const submitReviewComment = async () => {
  reviewError.value = "";
  const t = selectedTask.value;
  const a = latestAttempt.value;
  const text = String(reviewCommentDraft.value || "").trim();
  if (!t || !a || !text) return;

  submittingReviewComment.value = true;
  try {
    const created = await postTaskAttemptReviewComment(t.id, a.id, {
      comment: text,
    });
    reviewCommentDraft.value = "";
    reviewComments.value = [created, ...reviewComments.value].slice(0, 10);
  } catch (e: any) {
    reviewError.value = String(
      e?.data?.error || e?.message || "Failed to submit review comment",
    );
  } finally {
    submittingReviewComment.value = false;
  }
};

const createFollowUpAttempt = async () => {
  reviewError.value = "";
  const t = selectedTask.value;
  if (!t) return;
  const notes = String(followUpNotes.value || "").trim();
  if (!notes) return;

  submittingFollowUp.value = true;
  try {
    await resumeTask(t.id, { review_notes: notes });
    await refreshSelected();
    await refreshTasks();
    reviewOpen.value = false;
  } catch (e: any) {
    reviewError.value = String(
      e?.data?.error || e?.message || "Failed to create follow-up attempt",
    );
  } finally {
    submittingFollowUp.value = false;
  }
};

watch(
  () => [reviewOpen.value, latestAttempt.value?.id, selectedTask.value?.id],
  async ([open]) => {
    if (!open) return;
    await loadReviewArtifacts();
  },
);

const allWorkspaces = computed(() => {
  const fromTasks = tasks.value
    .map((t) => normalizeWorkspace(t.workspace))
    .filter(Boolean);
  const defaultWS = normalizeWorkspace(
    localStorage.getItem("oneagent-workspace") || "",
  );
  const merged = [...workspacesManual.value, ...fromTasks];
  if (defaultWS) merged.unshift(defaultWS);
  return Array.from(
    new Set(merged.map(normalizeWorkspace).filter(Boolean)),
  ).sort();
});

const workspaceSummaries = computed<WorkspaceSummary[]>(() => {
  const byWS = new Map<string, WorkspaceSummary>();
  for (const t of tasks.value) {
    const ws = normalizeWorkspace(t.workspace);
    if (!ws) continue;
    if (!byWS.has(ws)) {
      byWS.set(ws, {
        workspace: ws,
        total: 0,
        queued: 0,
        running: 0,
        failed: 0,
      });
    }
    const s = byWS.get(ws)!;
    s.total++;
    const a =
      Array.isArray(t.attempts) && t.attempts.length
        ? t.attempts[t.attempts.length - 1]
        : null;
    const st = String(a?.status || "");
    if (st === "queued") s.queued++;
    else if (st === "running") s.running++;
    else if (
      ["failed", "limit_exceeded", "timed_out", "interrupted"].includes(st)
    )
      s.failed++;
  }
  return Array.from(byWS.values()).sort((a, b) =>
    a.workspace.localeCompare(b.workspace),
  );
});

const filteredTasks = computed(() => {
  const ws = normalizeWorkspace(workspaceSelected.value);
  if (!ws) return tasks.value;
  return tasks.value.filter((t) => normalizeWorkspace(t.workspace) === ws);
});

const selectWorkspace = (ws: string) => {
  workspaceSelected.value = ws;
  if (
    selectedTask.value &&
    normalizeWorkspace(selectedTask.value.workspace) !== normalizeWorkspace(ws)
  ) {
    selectedTaskId.value = "";
    selectedTask.value = null;
    selectedEvents.value = [];
  }
};

const addWorkspace = () => {
  const ws = normalizeWorkspace(workspaceNew.value);
  if (!ws) return;
  if (!workspacesManual.value.includes(ws)) {
    workspacesManual.value = [...workspacesManual.value, ws];
    saveManualWorkspaces();
  }
  workspaceNew.value = "";
  if (!workspaceSelected.value) workspaceSelected.value = ws;
};

const queueTask = async () => {
  tasksError.value = "";
  const ws = normalizeWorkspace(workspaceSelected.value);
  const p = normalizeWorkspace(prompt.value);
  if (!ws || !p) return;

  submitting.value = true;
  try {
    const limits: any = {};
    const tokenBudget = Number.parseInt(
      String(budgetTokens.value || "").trim(),
      10,
    );
    if (Number.isFinite(tokenBudget) && tokenBudget > 0)
      limits.max_total_tokens = tokenBudget;
    const costBudget = Number.parseFloat(String(budgetCost.value || "").trim());
    if (Number.isFinite(costBudget) && costBudget > 0)
      limits.max_cost_usd = costBudget;

    const created = await createTask({
      workspace: ws,
      title: normalizeWorkspace(title.value) || undefined,
      prompt: p,
      limits: Object.keys(limits).length ? limits : undefined,
    });
    prompt.value = "";
    title.value = "";
    budgetTokens.value = "";
    budgetCost.value = "";
    selectedTaskId.value = created.id;
    await refreshTasks();
    await refreshSelected();
  } catch (e: any) {
    tasksError.value = String(
      e?.data?.error || e?.message || "Failed to create task",
    );
  } finally {
    submitting.value = false;
  }
};

const formatUsage = (a: TaskAttempt | null) => {
  const u = a?.usage;
  if (!u) return "";
  const calls =
    typeof u.calls === "number" && u.calls > 0 ? `${u.calls} calls` : "";
  const tokens =
    typeof u.total_tokens === "number" && u.total_tokens > 0
      ? `${u.total_tokens} tokens`
      : "";
  const cost =
    typeof u.cost_usd === "number" && u.cost_usd > 0
      ? `$${u.cost_usd.toFixed(4)}`
      : "";
  return [tokens, calls, cost].filter(Boolean).join(" · ");
};

const doCancel = async () => {
  const id = normalizeWorkspace(selectedTaskId.value);
  if (!id) return;
  selectedError.value = "";
  try {
    await cancelTask(id);
    await refreshTasks();
    await refreshSelected();
  } catch (e: any) {
    selectedError.value = String(
      e?.data?.error || e?.message || "Failed to cancel task",
    );
  }
};

const doResume = async () => {
  const id = normalizeWorkspace(selectedTaskId.value);
  if (!id) return;
  selectedError.value = "";
  try {
    await resumeTask(id);
    await refreshTasks();
    await refreshSelected();
  } catch (e: any) {
    selectedError.value = String(
      e?.data?.error || e?.message || "Failed to resume task",
    );
  }
};

onMounted(async () => {
  loadManualWorkspaces();
  await refreshTasks();
  if (!workspaceSelected.value) {
    workspaceSelected.value = allWorkspaces.value[0] || "";
  }
});

let timer: number | undefined;
onMounted(() => {
  timer = window.setInterval(() => {
    void refreshTasks();
    if (selectedTaskId.value) {
      void refreshSelected();
    }
  }, 2000);
});
onUnmounted(() => {
  if (timer != null) {
    window.clearInterval(timer);
    timer = undefined;
  }
});
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto">
      <div class="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">任务工作台</h1>
          <p class="text-sm text-surface-500">多工作区队列、进度与控制</p>
        </div>
        <button
          data-testid="task-workbench-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="tasksLoading"
          @click="refreshTasks()"
        >
          <RefreshCw class="w-4 h-4" />
          刷新
        </button>
      </div>

      <div
        v-if="taskUpdates.length"
        data-testid="task-updates"
        class="mb-4 rounded-2xl border border-surface-700/40 bg-surface-950/40 p-4"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="text-sm font-semibold text-surface-100">更新</div>
          <button
            class="text-xs text-surface-400 hover:text-surface-200"
            @click="clearTaskUpdates"
          >
            清空
          </button>
        </div>
        <div class="mt-2 space-y-2">
          <div
            v-for="u in taskUpdates"
            :key="`${u.taskId}:${u.attemptId}:${u.status}`"
            class="text-sm text-surface-200"
          >
            <span class="font-mono text-surface-400">{{ u.status }}</span>
            <span class="mx-2 text-surface-600">·</span>
            <span class="text-surface-100">{{ u.title }}</span>
            <span class="mx-2 text-surface-600">·</span>
            <span class="text-surface-400 truncate">{{ u.workspace }}</span>
          </div>
        </div>
      </div>

      <div
        v-if="tasksError"
        class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200"
      >
        {{ tasksError }}
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <!-- Left column: Workspaces + Task list -->
        <div class="space-y-4">
          <!-- Workspaces -->
          <div class="glass rounded-2xl overflow-hidden">
            <div class="px-5 py-4 border-b border-surface-700/50">
              <p class="text-sm font-semibold text-surface-100">工作区</p>
              <p class="text-xs text-surface-500 mt-1">
                每个工作区按 FIFO（L0）串行执行任务
              </p>
            </div>

            <div class="p-4 border-b border-surface-700/50">
              <div class="flex gap-2">
                <input
                  data-testid="workspace-add-input"
                  v-model="workspaceNew"
                  class="flex-1 px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                  placeholder="/path/to/workspace"
                />
                <button
                  data-testid="workspace-add"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-primary-500/15 text-primary-300 hover:bg-primary-500/20 inline-flex items-center gap-2"
                  @click="addWorkspace"
                >
                  <Plus class="w-4 h-4" />
                </button>
              </div>
            </div>

            <div class="max-h-[30vh] overflow-y-auto">
              <button
                v-for="ws in allWorkspaces"
                :key="ws"
                data-testid="workspace-item"
                class="w-full text-left px-4 py-3 border-b border-surface-700/30 hover:bg-surface-900/40"
                :class="workspaceSelected === ws ? 'bg-primary-500/10' : ''"
                @click="selectWorkspace(ws)"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2 text-sm text-surface-100">
                      <Folder class="w-4 h-4 text-surface-400" />
                      <span class="truncate">{{ ws }}</span>
                    </div>
                    <div class="text-xs text-surface-500 mt-1">
                      <span
                        v-if="workspaceSummaries.find((s) => s.workspace === ws)"
                      >
                        {{
                          (() => {
                            const s = workspaceSummaries.find(
                              (x) => x.workspace === ws,
                            );
                            if (!s) return "";
                            return `总计 ${s.total} · 排队 ${s.queued} · 运行中 ${s.running} · 失败 ${s.failed}`;
                          })()
                        }}
                      </span>
                      <span v-else>暂无任务</span>
                    </div>
                  </div>
                </div>
              </button>
              <div
                v-if="allWorkspaces.length === 0"
                class="p-6 text-sm text-surface-500"
              >
                暂无工作区。
              </div>
            </div>
          </div>

          <!-- Tasks list -->
          <div class="glass rounded-2xl overflow-hidden">
            <div
              class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between"
            >
              <p class="text-sm font-semibold text-surface-100">任务</p>
              <div
                class="inline-flex items-center gap-2 text-xs text-surface-500"
              >
                <ListTodo class="w-4 h-4" />
                {{ filteredTasks.length }}
              </div>
            </div>

            <div class="overflow-y-auto">
              <button
                v-for="t in filteredTasks"
                :key="t.id"
                data-testid="workbench-task-item"
                class="w-full text-left px-4 py-3 border-b border-surface-700/30 hover:bg-surface-900/40"
                :class="selectedTaskId === t.id ? 'bg-primary-500/10' : ''"
                @click="
                  selectedTaskId = t.id;
                  refreshSelected();
                "
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="text-sm text-surface-100 truncate">
                      {{ t.title || t.id }}
                    </div>
                    <div class="text-xs text-surface-500 mt-1 truncate">
                      {{ t.prompt }}
                    </div>
                  </div>
                  <div class="text-xs text-surface-400">
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 border border-surface-700/40 bg-surface-900/40"
                    >
                      {{
                        t.attempts?.length
                          ? t.attempts[t.attempts.length - 1].status
                          : "queued"
                      }}
                    </span>
                  </div>
                </div>
              </button>
              <div
                v-if="filteredTasks.length === 0"
                class="p-6 text-sm text-surface-500"
              >
                该工作区暂无任务。
              </div>
            </div>
          </div>
        </div>

        <!-- Right column: Composer + Task details -->
        <div class="space-y-4">
          <!-- Composer -->
          <div class="glass rounded-2xl overflow-hidden">
            <div class="px-5 py-4 border-b border-surface-700/50">
              <p class="text-sm font-semibold text-surface-100">新建任务</p>
              <p class="text-xs text-surface-500 mt-1">
                在当前选中的工作区创建新任务
              </p>
            </div>

            <div class="p-4">
              <div class="grid grid-cols-1 gap-2">
                <textarea
                  data-testid="workbench-prompt"
                  v-model="prompt"
                  rows="4"
                  class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                  placeholder="描述你希望完成的内容..."
                />
                <button
                  data-testid="workbench-queue"
                  class="px-4 py-2 rounded-xl text-sm font-medium bg-primary-500/15 text-primary-300 hover:bg-primary-500/20 disabled:opacity-50"
                  :disabled="submitting || !workspaceSelected || !prompt.trim()"
                  @click="queueTask"
                >
                  入队任务
                </button>
                <button
                  type="button"
                  data-testid="workbench-composer-advanced-toggle"
                  class="text-xs text-surface-400 hover:text-surface-200 text-left"
                  @click="composerAdvancedOpen = !composerAdvancedOpen"
                >
                  {{
                    composerAdvancedOpen
                      ? "收起高级设置"
                      : "展开高级设置（标题/预算）"
                  }}
                </button>
                <div
                  v-if="composerAdvancedOpen"
                  data-testid="workbench-composer-advanced"
                  class="grid grid-cols-1 gap-2"
                >
                  <input
                    v-model="title"
                    class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                    placeholder="可选标题"
                  />
                  <div class="grid grid-cols-2 gap-2">
                    <input
                      v-model="budgetTokens"
                      inputmode="numeric"
                      class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                      placeholder="Token 预算（可选）"
                    />
                    <input
                      v-model="budgetCost"
                      inputmode="decimal"
                      class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm"
                      placeholder="成本预算 USD（可选）"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Task details -->
          <div class="glass rounded-2xl overflow-hidden">
            <!-- Header: Title + Actions -->
            <div
              class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-4"
            >
              <div class="min-w-0">
                <p class="text-sm font-semibold text-surface-100 truncate">
                  {{ selectedTask?.title || "详情" }}
                </p>
                <p
                  v-if="selectedTask"
                  class="text-xs text-surface-500 mt-0.5 truncate"
                >
                  {{ selectedTask.workspace }}
                </p>
              </div>
              <div class="inline-flex gap-2 flex-shrink-0">
                <button
                  data-testid="workbench-cancel"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2"
                  :disabled="!canCancel || selectedLoading"
                  @click="doCancel"
                >
                  <X class="w-4 h-4" />
                  取消
                </button>
                <button
                  data-testid="workbench-resume"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2"
                  :disabled="!canResume || selectedLoading"
                  @click="doResume"
                >
                  <RotateCcw class="w-4 h-4" />
                  继续
                </button>
                <button
                  type="button"
                  data-testid="workbench-review-toggle"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2"
                  :disabled="!latestAttempt"
                  :class="reviewOpen ? 'bg-indigo-500/20 text-indigo-300' : ''"
                  @click="reviewOpen = !reviewOpen"
                >
                  <ListTodo class="w-4 h-4" />
                  审查
                </button>
              </div>
            </div>

            <!-- Tab Navigation -->
            <div
              v-if="selectedTask"
              class="px-5 py-2 border-b border-surface-700/30 flex items-center gap-1 bg-surface-900/20"
            >
              <button
                v-for="tab in detailTabs"
                :key="tab.key"
                class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                :class="
                  activeDetailTab === tab.key
                    ? 'bg-surface-700/50 text-surface-100'
                    : 'text-surface-500 hover:text-surface-300 hover:bg-surface-800/30'
                "
                @click="activeDetailTab = tab.key"
              >
                {{ tab.label }}
              </button>
            </div>

            <div
              v-if="selectedError"
              class="m-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200"
            >
              {{ selectedError }}
            </div>

            <div v-if="!selectedTask" class="p-6 text-sm text-surface-500">
              选择任务查看详情。
            </div>

            <div v-else class="p-4">
              <!-- Overview Tab -->
              <div v-if="activeDetailTab === 'overview'" class="space-y-4">
                <!-- Two-column layout: Status info | Details -->
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  <!-- Left: Task Summary -->
                  <div class="space-y-3">
                    <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                      <div class="flex items-center gap-2 mb-3">
                        <div class="w-2 h-2 rounded-full" :class="{
                          'bg-emerald-400': latestStatus === 'succeeded',
                          'bg-rose-400': ['failed', 'canceled', 'limit_exceeded', 'timed_out', 'interrupted'].includes(latestStatus),
                          'bg-amber-400': latestStatus === 'running',
                          'bg-surface-400': latestStatus === 'queued'
                        }"></div>
                        <span class="text-sm font-medium text-surface-200">{{ latestStatus || 'queued' }}</span>
                      </div>
                      <div v-if="formatUsage(latestAttempt)" class="text-xs text-surface-400 flex items-center gap-4">
                        <span>用量：{{ formatUsage(latestAttempt) }}</span>
                      </div>
                    </div>

                    <div v-if="latestAttempt?.observer" class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                      <div class="flex items-center gap-2 mb-2">
                        <div class="w-2 h-2 rounded-full" :class="latestAttempt.observer.pass ? 'bg-emerald-400' : 'bg-rose-400'"></div>
                        <span class="text-xs font-medium text-surface-200">观察者：{{ latestAttempt.observer.pass ? '通过' : '失败' }}</span>
                      </div>
                      <p class="text-xs text-surface-400">{{ latestAttempt.observer.reason }}</p>
                      <div v-if="!latestAttempt.observer.pass && latestAttempt.observer.next_steps" class="mt-2 p-2 rounded-lg bg-surface-900/60 border border-surface-700/30">
                        <p class="text-[11px] text-surface-500 mb-1">下一步：</p>
                        <p class="text-xs text-surface-300 whitespace-pre-wrap">{{ latestAttempt.observer.next_steps }}</p>
                      </div>
                      <div v-if="!latestAttempt.observer.pass && latestAttempt.observer.questions_for_user?.length" class="mt-2 p-2 rounded-lg bg-amber-500/10 border border-amber-500/20">
                        <p class="text-[11px] text-amber-400 mb-1">需要你确认：</p>
                        <p class="text-xs text-surface-300 whitespace-pre-wrap">{{ latestAttempt.observer.questions_for_user.join('\n') }}</p>
                      </div>
                    </div>
                  </div>

                  <!-- Right: Summary Text -->
                  <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                    <div class="flex items-center justify-between mb-3">
                      <span class="text-sm font-medium text-surface-200">执行摘要</span>
                      <span class="text-xs text-surface-500">{{ latestAttempt?.id?.slice(0, 8) }}</span>
                    </div>
                    <div v-if="latestAttempt?.summary" class="text-sm text-surface-300 whitespace-pre-wrap leading-relaxed">
                      {{ latestAttempt.summary }}
                    </div>
                    <div v-else class="text-sm text-surface-500 italic">暂无执行摘要</div>
                  </div>
                </div>
              </div>

              <!-- Review Tab -->
              <div v-else-if="activeDetailTab === 'review'" class="space-y-4">
                <div v-if="!latestAttempt" class="text-sm text-surface-500">暂无尝试记录</div>
                <div v-else class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <!-- Left: File changes -->
                  <div class="space-y-4">
                    <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                      <div class="text-sm font-medium text-surface-200 mb-3">变更文件</div>
                      <pre
                        v-if="changedFiles?.content"
                        class="max-h-[40vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                      >{{ changedFiles.content }}</pre>
                      <div v-else class="text-sm text-surface-500 italic">暂无变更文件列表</div>
                    </div>
                  </div>

                  <!-- Right: Diff + Comments -->
                  <div class="space-y-4">
                    <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                      <div class="text-sm font-medium text-surface-200 mb-3">Diff Patch</div>
                      <pre
                        v-if="diffPatch?.content"
                        class="max-h-[30vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                      >{{ diffPatch.content }}</pre>
                      <div v-else class="text-sm text-surface-500 italic">暂无 diff patch</div>
                    </div>

                    <!-- Review Comments -->
                    <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4">
                      <div class="text-sm font-medium text-surface-200 mb-3">Review Comments</div>
                      <div v-if="reviewComments.length" class="space-y-2 mb-4">
                        <div
                          v-for="(cmt, idx) in reviewComments"
                          :key="idx"
                          class="rounded-xl bg-surface-900/40 p-3 text-sm text-surface-200"
                        >
                          <div class="text-[11px] text-surface-500"
                          >{{ cmt.ts }} · {{ cmt.principal_id }}</div>
                          <div class="mt-1 whitespace-pre-wrap">{{ cmt.comment }}</div>
                        </div>
                      </div>

                      <textarea
                        data-testid="review-comment-input"
                        v-model="reviewCommentDraft"
                        class="w-full rounded-xl bg-surface-900/40 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-indigo-500/30"
                        rows="2"
                        placeholder="写一条 review comment..."
                      />
                      <div class="mt-2 flex justify-end gap-2">
                        <button
                          data-testid="review-comment-submit"
                          class="px-3 py-2 rounded-xl text-sm font-medium bg-indigo-500/80 text-white hover:bg-indigo-500 disabled:opacity-50"
                          :disabled="submittingReviewComment || !reviewCommentDraft.trim()"
                          @click="submitReviewComment"
                        >
                          提交评论
                        </button>
                      </div>
                    </div>

                    <!-- Follow-up -->
                    <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4"
                    >
                      <div class="text-sm font-medium text-surface-200 mb-3">Follow-up Attempt</div>
                      <textarea
                        data-testid="follow-up-notes-input"
                        v-model="followUpNotes"
                        class="w-full rounded-xl bg-surface-900/40 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-emerald-500/30"
                        rows="2"
                        placeholder="输入跟进指令..."
                      />
                      <div class="mt-2 flex justify-end"
                    >
                        <button
                          data-testid="follow-up-submit"
                          class="px-3 py-2 rounded-xl text-sm font-medium bg-emerald-500/80 text-white hover:bg-emerald-500 disabled:opacity-50"
                          :disabled="submittingFollowUp || !followUpNotes.trim()"
                          @click="createFollowUpAttempt"
                        >
                          创建跟进 attempt
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Events Tab -->
              <div v-else-if="activeDetailTab === 'events'" class="space-y-4"
              >
                <div v-if="selectedLoading" class="text-sm text-surface-500"
                >加载中…</div>
                <div v-else-if="selectedEvents.length === 0" class="text-sm text-surface-500"
                >暂无事件。</div>
                <div v-else class="space-y-2 max-h-[60vh] overflow-y-auto pr-2"
                >
                  <div
                    v-for="(e, idx) in selectedEvents"
                    :key="idx"
                    class="flex items-start gap-3 py-2 border-b border-surface-700/20 last:border-0"
                  >
                    <span class="text-xs text-surface-500 font-mono flex-shrink-0 w-[52px]"
                    >{{ e.ts.slice(11, 19) }}</span>
                    <span class="text-xs font-mono text-surface-300 px-1.5 py-0.5 rounded bg-surface-800/50"
                    >{{ e.type }}</span>
                    <span v-if="e.message" class="text-xs text-surface-400"
                    >{{ e.message }}</span>
                  </div>
                </div>
              </div>

              <!-- Advanced Tab -->
              <div v-else-if="activeDetailTab === 'advanced'" class="space-y-4"
              >
                <div v-if="!latestAttempt" class="text-sm text-surface-500"
                >暂无尝试记录</div>
                <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4"
                >
                  <div
                    v-for="item in [
                      ['findings', latestAttempt.findings_path],
                      ['trace', latestAttempt.trace_log_path],
                      ['测试报告', latestAttempt.test_report_path],
                      ['changed_files', latestAttempt.changed_files_path],
                      ['diff_patch', latestAttempt.diff_patch_path],
                      ['review_comments', latestAttempt.review_comments_path],
                      ['project.json', latestAttempt.project_config_path],
                      ['copy_files', latestAttempt.copy_files_log_path],
                      ['setup_script', latestAttempt.setup_script_log_path],
                      ['test_script', latestAttempt.test_script_log_path],
                      ['cleanup_script', latestAttempt.cleanup_script_log_path],
                    ].filter(([, v]) => v) as [string, string]"
                    :key="item[0]"
                    class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3"
                  >
                    <div class="text-xs text-surface-500 mb-1"
                    >{{ item[0] }}</div>
                    <div class="text-xs text-surface-300 font-mono break-all"
                    >{{ item[1] }}</div>
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="false"
              data-testid="workbench-review-panel"
              class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4 space-y-3"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="text-sm text-surface-100 font-semibold">审查</div>
                <div class="text-xs text-surface-500">{{ latestAttempt.id }}</div>
              </div>

              <div
                v-if="reviewError"
                class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-200"
              >
                {{ reviewError }}
              </div>

              <div v-if="reviewLoading" class="text-xs text-surface-500">
                加载中…
              </div>

              <div v-else class="space-y-3">
                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">变更文件</div>
                  <pre
                    v-if="changedFiles?.content"
                    class="mt-2 max-h-[20vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                  >{{ changedFiles.content }}</pre>
                  <div v-else class="mt-2 text-surface-500">
                    暂无变更文件列表（可能无改动或未生成）。
                  </div>
                </div>

                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">Diff Patch</div>
                  <pre
                    v-if="diffPatch?.content"
                    class="mt-2 max-h-[30vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                  >{{ diffPatch.content }}</pre>
                  <div v-else class="mt-2 text-surface-500">
                    暂无 diff patch（可能非 git 或 diff 太大）。
                  </div>
                </div>

                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">
                    Review Comments
                  </div>
                  <div v-if="reviewComments.length" class="mt-2 space-y-2">
                    <div
                      v-for="(cmt, idx) in reviewComments"
                      :key="idx"
                      class="rounded-xl bg-surface-900/40 p-3 text-surface-200"
                    >
                      <div class="text-[11px] text-surface-500">
                        {{ cmt.ts }} · {{ cmt.principal_id }}
                      </div>
                      <div class="mt-1 whitespace-pre-wrap">
                        {{ cmt.comment }}
                      </div>
                    </div>
                  </div>

                  <textarea
                    data-testid="review-comment-input"
                    v-model="reviewCommentDraft"
                    class="mt-2 w-full rounded-xl bg-surface-900/40 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-indigo-500/30"
                    rows="3"
                    placeholder="写一条 review comment（会作为证据 append-only 保存）"
                  />
                  <div class="mt-2 flex justify-end">
                    <button
                      data-testid="review-comment-submit"
                      class="px-3 py-2 rounded-xl text-sm font-medium bg-indigo-500/80 text-white hover:bg-indigo-500 disabled:opacity-50"
                      :disabled="submittingReviewComment || !reviewCommentDraft.trim()"
                      @click="submitReviewComment"
                    >
                      提交评论
                    </button>
                  </div>
                </div>

                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">
                    Follow-up Attempt
                  </div>
                  <textarea
                    data-testid="follow-up-notes-input"
                    v-model="followUpNotes"
                    class="mt-2 w-full rounded-xl bg-surface-900/40 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-indigo-500/30"
                    rows="3"
                    placeholder="把 review 结论写成可执行的跟进指令（将注入下一轮 attempt 上下文）"
                  />
                  <div class="mt-2 flex justify-end">
                    <button
                      data-testid="follow-up-submit"
                      class="px-3 py-2 rounded-xl text-sm font-medium bg-emerald-500/80 text-white hover:bg-emerald-500 disabled:opacity-50"
                      :disabled="submittingFollowUp || !followUpNotes.trim()"
                      @click="createFollowUpAttempt"
                    >
                      创建跟进 attempt
                    </button>
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>
      </div>
    </div>
  </div>
</template>
