<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import {
  Folder,
  ListTodo,
  RefreshCw,
  Loader2,
  RotateCcw,
  X,
  Plus,
  ChevronDown,
  MousePointerClick,
} from "lucide-vue-next";

import ErrorBanner from "@/components/ErrorBanner.vue";
import EventLogViewer from "@/components/EventLogViewer.vue";

import {
  cancelTask,
  chooseWorkspaceDir,
  createTask,
  getTask,
  getTaskAttemptArtifact,
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
import { parseApiError, type ParsedApiError } from "@/lib/apiError";

type WorkspaceSummary = {
  workspace: string;
  total: number;
  queued: number;
  running: number;
  failed: number;
};

const STORAGE_KEY = "oneagent-workspaces";
const TASK_SNAPSHOT_KEY = "oneagent-task-snapshots";

const safeGetLocalStorageItem = (key: string) => {
  try {
    return String(globalThis?.localStorage?.getItem?.(key) || "");
  } catch {
    return "";
  }
};

const tasksLoading = ref(false);
const tasksError = ref<ParsedApiError | null>(null);
const tasks = ref<Task[]>([]);
let tasksRequestSeq = 0;

const notifyInitialized = ref(false);
const taskSnapshots = ref<Record<string, TaskSnapshot>>({});
const taskUpdates = ref<TaskUpdate[]>([]);

const workspacesManual = ref<string[]>([]);
const workspaceNew = ref("");
const workspaceSelected = ref("");
const workspaceChoosing = ref(false);
const workspaceChooseError = ref<ParsedApiError | null>(null);

const selectedTaskId = ref("");
const selectedTask = ref<Task | null>(null);
const selectedEvents = ref<TaskEvent[]>([]);
const selectedLoading = ref(false);
const selectedError = ref<ParsedApiError | null>(null);
let selectedRequestSeq = 0;

const title = ref("");
const prompt = ref("");
const promptEl = ref<HTMLTextAreaElement | null>(null);
const budgetTokens = ref("");
const budgetCost = ref("");
const submitting = ref(false);
const composerAdvancedOpen = ref(false);

const reviewLoading = ref(false);
const reviewError = ref<ParsedApiError | null>(null);
const diffPatch = ref<TaskAttemptArtifactContent | null>(null);
const changedFiles = ref<TaskAttemptArtifactContent | null>(null);
const reviewComments = ref<TaskAttemptReviewComment[]>([]);
const reviewCommentDraft = ref("");
const followUpNotes = ref("");
const submittingReviewComment = ref(false);
const submittingFollowUp = ref(false);

type AttemptArtifactSummary = {
  kind: string;
  label: string;
  path: string;
};

const advancedArtifacts = computed<AttemptArtifactSummary[]>(() => {
  const a = latestAttempt.value;
  if (!a) return [];
  const items: AttemptArtifactSummary[] = [
    {
      kind: "findings",
      label: "findings",
      path: String(a.findings_path || ""),
    },
    {
      kind: "artifact_manifest",
      label: "manifest",
      path: String(a.artifact_manifest_path || ""),
    },
    { kind: "trace", label: "trace", path: String(a.trace_log_path || "") },
    {
      kind: "test_report",
      label: "测试报告",
      path: String(a.test_report_path || ""),
    },
    {
      kind: "changed_files",
      label: "changed_files",
      path: String(a.changed_files_path || ""),
    },
    {
      kind: "diff_patch",
      label: "diff_patch",
      path: String(a.diff_patch_path || ""),
    },
    {
      kind: "review_comments",
      label: "review_comments",
      path: String(a.review_comments_path || ""),
    },
    {
      kind: "project_config",
      label: "project.json",
      path: String(a.project_config_path || ""),
    },
    {
      kind: "copy_files_log",
      label: "copy_files",
      path: String(a.copy_files_log_path || ""),
    },
    {
      kind: "setup_script_log",
      label: "setup_script",
      path: String(a.setup_script_log_path || ""),
    },
    {
      kind: "test_script_log",
      label: "test_script",
      path: String(a.test_script_log_path || ""),
    },
    {
      kind: "cleanup_script_log",
      label: "cleanup_script",
      path: String(a.cleanup_script_log_path || ""),
    },
  ];
  return items.filter((i) => i.path.trim());
});

const artifactModalOpen = ref(false);
const artifactModalLoading = ref(false);
const artifactModalError = ref<ParsedApiError | null>(null);
const artifactModalLabel = ref("");
const artifactModalKind = ref("");
const artifactModalPath = ref("");
const artifactModalContent = ref<TaskAttemptArtifactContent | null>(null);

let artifactRequestSeq = 0;
const openArtifactModal = async (artifact: AttemptArtifactSummary) => {
  const t = selectedTask.value;
  const a = latestAttempt.value;
  if (!t || !a) return;

  artifactModalOpen.value = true;
  artifactModalLoading.value = true;
  artifactModalError.value = null;
  artifactModalLabel.value = artifact.label;
  artifactModalKind.value = artifact.kind;
  artifactModalPath.value = artifact.path;
  artifactModalContent.value = null;

  const seq = ++artifactRequestSeq;
  try {
    const res = await getTaskAttemptArtifact(t.id, a.id, artifact.kind);
    if (seq !== artifactRequestSeq) return;
    artifactModalPath.value = String(res?.path || artifact.path);
    artifactModalContent.value = res;
  } catch (e: any) {
    if (seq !== artifactRequestSeq) return;
    artifactModalError.value = parseApiError(e, "加载产物失败");
  } finally {
    if (seq === artifactRequestSeq) artifactModalLoading.value = false;
  }
};

const closeArtifactModal = () => {
  artifactModalOpen.value = false;
};

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
  activeDetailTab.value = "overview";
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
  [
    "succeeded",
    "failed",
    "canceled",
    "limit_exceeded",
    "timed_out",
    "interrupted",
  ].includes(latestStatus.value),
);

const normalizeWorkspace = (ws: string) => String(ws || "").trim();

const workspacePathForDisplay = (ws: string) =>
  normalizeWorkspace(ws).replace(/[\\/]+$/, "");

const workspaceBasename = (ws: string) => {
  const p = workspacePathForDisplay(ws);
  if (!p) return "";
  const parts = p.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] || p;
};

const persistDefaultWorkspace = (ws: string) => {
  const v = normalizeWorkspace(ws);
  if (!v) return;
  try {
    localStorage.setItem("oneagent-workspace", v);
  } catch {
    // ignore
  }
};

const focusComposer = () => {
  const el = promptEl.value;
  if (!el) return;
  el.focus();
  el.scrollIntoView?.({ behavior: "smooth", block: "center" });
};

const onPromptKeydown = (e: KeyboardEvent) => {
  if (e.key !== "Enter") return;
  if (!e.metaKey && !e.ctrlKey) return;
  e.preventDefault();
  void queueTask();
};

const sortTaskEventsNewestFirst = (events: TaskEvent[]) => {
  const safeEvents = Array.isArray(events) ? events : [];
  return [...safeEvents].sort((a, b) => {
    const at = Date.parse(String(a?.ts || ""));
    const bt = Date.parse(String(b?.ts || ""));
    if (!Number.isNaN(at) && !Number.isNaN(bt)) return bt - at;
    if (!Number.isNaN(bt)) return 1;
    if (!Number.isNaN(at)) return -1;
    return String(b?.ts || "").localeCompare(String(a?.ts || ""));
  });
};

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
  const seq = ++tasksRequestSeq;
  tasksError.value = null;
  tasksLoading.value = true;
  try {
    const list = await listTasks();
    if (seq !== tasksRequestSeq) return;
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
    if (seq !== tasksRequestSeq) return;
    tasksError.value = parseApiError(e, "加载任务失败");
    tasks.value = [];
  } finally {
    if (seq === tasksRequestSeq) tasksLoading.value = false;
  }
};

const clearTaskUpdates = () => {
  taskUpdates.value = [];
};

const refreshSelected = async () => {
  const seq = ++selectedRequestSeq;
  selectedError.value = null;
  const id = normalizeWorkspace(selectedTaskId.value);
  if (!id) {
    selectedTask.value = null;
    selectedEvents.value = [];
    return;
  }

  selectedLoading.value = true;
  try {
    const [t, evs] = await Promise.all([getTask(id), getTaskEvents(id)]);
    if (seq !== selectedRequestSeq) return;
    selectedTask.value = t;
    selectedEvents.value = Array.isArray(evs)
      ? sortTaskEventsNewestFirst(evs)
      : [];
  } catch (e: any) {
    if (seq !== selectedRequestSeq) return;
    selectedError.value = parseApiError(e, "加载任务失败");
    selectedTask.value = null;
    selectedEvents.value = [];
  } finally {
    if (seq === selectedRequestSeq) selectedLoading.value = false;
  }
};

const loadReviewArtifacts = async () => {
  reviewError.value = null;
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
    reviewError.value = parseApiError(e, "加载审查信息失败");
  } finally {
    reviewLoading.value = false;
  }
};

const submitReviewComment = async () => {
  reviewError.value = null;
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
    reviewError.value = parseApiError(e, "提交评论失败");
  } finally {
    submittingReviewComment.value = false;
  }
};

const toggleReviewTab = async () => {
  if (!selectedTask.value || !latestAttempt.value) return;
  activeDetailTab.value =
    activeDetailTab.value === "review" ? "overview" : "review";
};

const createFollowUpAttempt = async () => {
  reviewError.value = null;
  const t = selectedTask.value;
  if (!t) return;
  const notes = String(followUpNotes.value || "").trim();
  if (!notes) return;

  submittingFollowUp.value = true;
  try {
    await resumeTask(t.id, { review_notes: notes });
    await refreshSelected();
    await refreshTasks();
    activeDetailTab.value = "overview";
  } catch (e: any) {
    reviewError.value = parseApiError(e, "创建 follow-up 失败");
  } finally {
    submittingFollowUp.value = false;
  }
};

watch(
  [
    activeDetailTab,
    () => selectedTask.value?.id,
    () => latestAttempt.value?.id,
    () => latestAttempt.value?.status,
    () => latestAttempt.value?.diff_patch_path,
    () => latestAttempt.value?.changed_files_path,
    () => latestAttempt.value?.review_comments_path,
  ],
  async ([tab]) => {
    if (tab !== "review") return;
    await loadReviewArtifacts();
  },
);

const allWorkspaces = computed(() => {
  const fromTasks = tasks.value
    .map((t) => normalizeWorkspace(t.workspace))
    .filter(Boolean);
  const defaultWS =
    normalizeWorkspace(workspaceSelected.value) ||
    normalizeWorkspace(safeGetLocalStorageItem("oneagent-workspace"));

  const merged = [...workspacesManual.value, ...fromTasks]
    .map(normalizeWorkspace)
    .filter(Boolean);

  const rest = Array.from(new Set(merged))
    .filter((ws) => ws !== defaultWS)
    .sort((a, b) => a.localeCompare(b));

  return defaultWS ? [defaultWS, ...rest] : rest;
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

const workspaceSummaryByPath = computed(() => {
  const m = new Map<string, WorkspaceSummary>();
  for (const s of workspaceSummaries.value) m.set(s.workspace, s);
  return m;
});

const filteredTasks = computed(() => {
  const ws = normalizeWorkspace(workspaceSelected.value);
  if (!ws) return tasks.value;
  return tasks.value.filter((t) => normalizeWorkspace(t.workspace) === ws);
});

const selectWorkspace = (ws: string) => {
  workspaceSelected.value = ws;
  persistDefaultWorkspace(ws);
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
  if (!workspaceSelected.value) {
    workspaceSelected.value = ws;
    persistDefaultWorkspace(ws);
  }
};

const chooseWorkspace = async () => {
  workspaceChooseError.value = null;
  workspaceChoosing.value = true;
  try {
    const res: any = await chooseWorkspaceDir();
    const ws = normalizeWorkspace(res?.path);
    if (!ws) return;
    if (!workspacesManual.value.includes(ws)) {
      workspacesManual.value = [...workspacesManual.value, ws];
      saveManualWorkspaces();
    }
    workspaceSelected.value = ws;
    persistDefaultWorkspace(ws);
  } catch (e: any) {
    workspaceChooseError.value = parseApiError(e, "选择文件夹失败");
  } finally {
    workspaceChoosing.value = false;
  }
};

const queueTask = async () => {
  tasksError.value = null;
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
    tasksError.value = parseApiError(e, "创建任务失败");
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
  selectedError.value = null;
  try {
    await cancelTask(id);
    await refreshTasks();
    await refreshSelected();
  } catch (e: any) {
    selectedError.value = parseApiError(e, "取消任务失败");
  }
};

const doResume = async () => {
  const id = normalizeWorkspace(selectedTaskId.value);
  if (!id) return;
  selectedError.value = null;
  try {
    await resumeTask(id);
    await refreshTasks();
    await refreshSelected();
  } catch (e: any) {
    selectedError.value = parseApiError(e, "恢复任务失败");
  }
};

onMounted(async () => {
  loadManualWorkspaces();
  const persisted = normalizeWorkspace(
    safeGetLocalStorageItem("oneagent-workspace"),
  );
  if (persisted) {
    workspaceSelected.value = persisted;
  }
  await refreshTasks();
  if (!workspaceSelected.value) {
    workspaceSelected.value = allWorkspaces.value[0] || "";
  }
  persistDefaultWorkspace(workspaceSelected.value);
});

let timer: number | undefined;
onMounted(() => {
  timer = window.setInterval(() => {
    if (!tasksLoading.value) void refreshTasks();
    if (selectedTaskId.value && !selectedLoading.value) {
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
    <div class="max-w-screen-2xl mx-auto">
      <div class="flex items-start justify-between gap-4 mb-8">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">任务工作台</h1>
          <p class="text-sm text-surface-500 mt-1">多工作区队列、进度与控制</p>
        </div>
        <button
          data-testid="task-workbench-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:opacity-50"
          :disabled="tasksLoading"
          title="刷新任务列表（页面会自动轮询刷新）"
          @click="refreshTasks"
        >
          <Loader2 v-if="tasksLoading" class="w-4 h-4 animate-spin" />
          <RefreshCw v-else class="w-4 h-4" />
          {{ tasksLoading ? "刷新中…" : "刷新" }}
        </button>
      </div>

      <div
        v-if="taskUpdates.length"
        data-testid="task-updates"
        class="mb-6 glass rounded-2xl p-4"
      >
        <div class="flex items-center justify-between gap-3 mb-3">
          <div class="flex items-center gap-2">
            <div
              class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"
            ></div>
            <div class="text-sm font-semibold text-surface-100">更新</div>
          </div>
          <button
            class="text-xs text-surface-400 hover:text-surface-200 px-2 py-1 rounded-lg hover:bg-surface-900/40 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
            @click="clearTaskUpdates"
          >
            清空
          </button>
        </div>
        <div class="space-y-2">
          <div
            v-for="u in taskUpdates"
            :key="`${u.taskId}:${u.attemptId}:${u.status}`"
            class="flex items-center gap-2 text-sm p-2 rounded-lg hover:bg-surface-900/30 transition-colors"
          >
            <span
              class="px-2 py-0.5 rounded-md text-xs font-medium"
              :class="{
                'bg-emerald-500/20 text-emerald-300': u.status === 'succeeded',
                'bg-rose-500/20 text-rose-300': ['failed', 'canceled'].includes(
                  u.status,
                ),
                'bg-amber-500/20 text-amber-300': u.status === 'running',
                'bg-surface-500/20 text-surface-400': [
                  'queued',
                  'pending',
                ].includes(u.status),
              }"
              >{{ u.status }}</span
            >
            <span class="text-surface-100 font-medium truncate max-w-[200px]">{{
              u.title
            }}</span>
            <span class="text-surface-600">·</span>
            <span class="text-surface-500 text-xs truncate">{{
              u.workspace
            }}</span>
          </div>
        </div>
      </div>

      <ErrorBanner
        v-if="tasksError"
        class="mb-6"
        :error="tasksError"
        title="任务失败"
      />

      <div class="grid grid-cols-1 lg:grid-cols-12 gap-5">
        <!-- Left column: Workspaces + Task list -->
        <div class="space-y-4 lg:col-span-5">
          <!-- Workspaces -->
          <div class="glass rounded-2xl overflow-hidden">
            <div
              class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3"
            >
              <Folder class="w-5 h-5 text-primary-400" />
              <div class="min-w-0">
                <p class="text-sm font-semibold text-surface-100">工作区</p>
                <p class="text-xs text-surface-500 mt-0.5">
                  每个工作区按 FIFO（L0）串行执行任务
                </p>
              </div>
            </div>

            <div class="p-5 space-y-4">
              <div class="space-y-2">
                <label class="text-xs text-surface-500" for="workspace-path">
                  工作区路径（服务端）
                </label>
                <div class="flex gap-2">
                  <input
                    id="workspace-path"
                    data-testid="workspace-add-input"
                    v-model="workspaceNew"
                    class="flex-1 px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:border-primary-500/30 transition-all"
                    placeholder="/path/to/workspace"
                    @keydown.enter.prevent="addWorkspace"
                  />
                  <button
                    data-testid="workspace-browse"
                    type="button"
                    class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2 disabled:opacity-50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                    :disabled="workspaceChoosing"
                    @click="chooseWorkspace"
                  >
                    <Folder class="w-4 h-4" />
                    选择文件夹
                  </button>
                  <button
                    data-testid="workspace-add"
                    class="px-3 py-2 rounded-xl text-sm font-medium bg-primary-500/15 text-primary-200 hover:bg-primary-500/20 inline-flex items-center gap-2 disabled:opacity-50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                    :disabled="!workspaceNew.trim()"
                    title="添加工作区"
                    aria-label="添加工作区"
                    @click="addWorkspace"
                  >
                    <Plus class="w-4 h-4" />
                    <span class="hidden sm:inline">添加</span>
                  </button>
                </div>
                <ErrorBanner
                  v-if="workspaceChooseError"
                  class="mt-3"
                  :error="workspaceChooseError"
                  title="选择失败"
                />
              </div>

              <div class="max-h-[30vh] overflow-y-auto -mx-1 px-1">
                <div class="space-y-2">
                  <button
                    v-for="ws in allWorkspaces"
                    :key="ws"
                    data-testid="workspace-item"
                    class="glass-card p-4 flex items-start gap-4 text-left w-full hover:bg-surface-900/40 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                    :class="
                      workspaceSelected === ws
                        ? 'ring-1 ring-primary-500/30'
                        : ''
                    "
                    @click="selectWorkspace(ws)"
                  >
                    <div
                      class="w-9 h-9 rounded-xl bg-surface-900/40 flex items-center justify-center flex-shrink-0"
                    >
                      <Folder class="w-4 h-4 text-surface-400" />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p
                            class="text-sm text-surface-100 font-semibold truncate"
                          >
                            {{ workspaceBasename(ws) || ws }}
                          </p>
                          <p class="text-xs text-surface-500 truncate">
                            {{ workspacePathForDisplay(ws) }}
                          </p>
                        </div>
                        <span
                          v-if="workspaceSelected === ws"
                          class="shrink-0 text-[11px] text-primary-200 bg-primary-500/15 px-2 py-1 rounded-lg"
                        >
                          当前
                        </span>
                      </div>

                      <div
                        class="mt-2 flex flex-wrap items-center gap-2 text-xs"
                      >
                        <template v-if="workspaceSummaryByPath.get(ws)">
                          <span
                            class="px-2 py-1 rounded-lg bg-surface-900/40 text-surface-300"
                          >
                            {{ workspaceSummaryByPath.get(ws)?.total }} 任务
                          </span>
                          <span
                            v-if="workspaceSummaryByPath.get(ws)?.running"
                            class="px-2 py-1 rounded-lg bg-amber-500/10 text-amber-300"
                          >
                            运行 {{ workspaceSummaryByPath.get(ws)?.running }}
                          </span>
                          <span
                            v-if="workspaceSummaryByPath.get(ws)?.queued"
                            class="px-2 py-1 rounded-lg bg-surface-900/40 text-surface-400"
                          >
                            排队 {{ workspaceSummaryByPath.get(ws)?.queued }}
                          </span>
                          <span
                            v-if="workspaceSummaryByPath.get(ws)?.failed"
                            class="px-2 py-1 rounded-lg bg-rose-500/10 text-rose-300"
                          >
                            失败 {{ workspaceSummaryByPath.get(ws)?.failed }}
                          </span>
                        </template>
                        <span v-else class="text-xs text-surface-500"
                          >暂无任务</span
                        >
                      </div>
                    </div>
                  </button>
                </div>

                <div
                  v-if="allWorkspaces.length === 0"
                  class="p-6 text-sm text-surface-500 text-center"
                >
                  暂无工作区。
                </div>
              </div>
            </div>
          </div>

          <!-- Tasks list -->
          <div class="glass rounded-2xl overflow-hidden">
            <div
              class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-3"
            >
              <div class="flex items-center gap-3">
                <ListTodo class="w-5 h-5 text-primary-400" />
                <div>
                  <p class="text-sm font-semibold text-surface-100">任务</p>
                  <p class="text-xs text-surface-500">
                    当前工作区 {{ filteredTasks.length }} 个
                  </p>
                </div>
              </div>
              <div
                v-if="tasksLoading"
                class="inline-flex items-center gap-2 text-xs text-surface-500"
              >
                <Loader2 class="w-3.5 h-3.5 animate-spin" />
                刷新中…
              </div>
            </div>

            <div class="max-h-[45vh] overflow-y-auto p-5 pt-4">
              <div
                v-if="filteredTasks.length === 0"
                class="py-10 text-sm text-surface-500 text-center"
              >
                该工作区暂无任务。
              </div>
              <div v-else class="space-y-2">
                <button
                  v-for="t in filteredTasks"
                  :key="t.id"
                  data-testid="workbench-task-item"
                  class="glass-card p-4 text-left w-full hover:bg-surface-900/40 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                  :class="
                    selectedTaskId === t.id ? 'ring-1 ring-primary-500/30' : ''
                  "
                  @click="
                    selectedTaskId = t.id;
                    refreshSelected();
                  "
                >
                  <div class="flex items-start justify-between gap-4">
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-2">
                        <div
                          class="w-2.5 h-2.5 rounded-full flex-shrink-0"
                          :class="{
                            'bg-emerald-400':
                              (t.attempts?.length
                                ? t.attempts[t.attempts.length - 1].status
                                : 'queued') === 'succeeded',
                            'bg-rose-400': [
                              'failed',
                              'canceled',
                              'limit_exceeded',
                              'timed_out',
                              'interrupted',
                            ].includes(
                              t.attempts?.length
                                ? t.attempts[t.attempts.length - 1].status
                                : 'queued',
                            ),
                            'bg-amber-400':
                              (t.attempts?.length
                                ? t.attempts[t.attempts.length - 1].status
                                : 'queued') === 'running',
                            'bg-surface-400':
                              (t.attempts?.length
                                ? t.attempts[t.attempts.length - 1].status
                                : 'queued') === 'queued',
                          }"
                        ></div>
                        <p
                          class="text-sm text-surface-100 truncate font-semibold"
                        >
                          {{ t.title || t.id }}
                        </p>
                      </div>
                      <p class="text-xs text-surface-500 mt-1.5 truncate pl-5">
                        {{ t.prompt }}
                      </p>
                    </div>
                    <div
                      class="text-[11px] text-surface-400 flex-shrink-0 mt-0.5"
                    >
                      <span
                        class="inline-flex items-center rounded-lg px-2 py-1 bg-surface-900/40 text-surface-400"
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
              </div>
            </div>
          </div>
        </div>

        <!-- Right column: Composer + Task details -->
        <div class="space-y-4 lg:col-span-7">
          <!-- Composer -->
          <div class="glass rounded-2xl overflow-hidden">
            <div
              class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3"
            >
              <Plus class="w-5 h-5 text-primary-400" />
              <div class="min-w-0">
                <p class="text-sm font-semibold text-surface-100">新建任务</p>
                <p class="text-xs text-surface-500 mt-0.5 truncate">
                  在
                  <span class="text-surface-300 font-mono">{{
                    workspaceBasename(workspaceSelected) || "未选择工作区"
                  }}</span>
                  创建新任务
                </p>
              </div>
            </div>

            <div class="p-5">
              <div class="grid grid-cols-1 gap-3">
                <div class="flex items-end justify-between gap-3">
                  <label
                    class="text-xs text-surface-500"
                    for="workbench-prompt"
                  >
                    需求描述
                  </label>
                  <div class="text-[11px] text-surface-500">
                    ⌘/Ctrl + Enter 入队
                  </div>
                </div>
                <textarea
                  id="workbench-prompt"
                  data-testid="workbench-prompt"
                  ref="promptEl"
                  v-model="prompt"
                  rows="4"
                  class="px-4 py-3 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:border-primary-500/30 transition-all resize-none"
                  placeholder="描述你希望完成的内容..."
                  @keydown="onPromptKeydown"
                />
                <button
                  data-testid="workbench-queue"
                  class="px-4 py-2.5 rounded-xl text-sm font-medium bg-primary-600 text-white hover:bg-primary-500 disabled:opacity-50 transition-colors inline-flex items-center justify-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                  :disabled="submitting || !workspaceSelected || !prompt.trim()"
                  @click="queueTask"
                >
                  <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
                  {{ submitting ? "入队中…" : "入队任务" }}
                </button>
                <button
                  type="button"
                  data-testid="workbench-composer-advanced-toggle"
                  class="text-xs text-surface-400 hover:text-surface-200 inline-flex items-center gap-1.5 text-left py-1 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 rounded-lg px-2 -mx-2"
                  @click="composerAdvancedOpen = !composerAdvancedOpen"
                >
                  <ChevronDown
                    class="w-4 h-4 transition-transform"
                    :class="composerAdvancedOpen ? 'rotate-180' : ''"
                  />
                  {{
                    composerAdvancedOpen
                      ? "收起高级设置"
                      : "展开高级设置（标题/预算）"
                  }}
                </button>
                <div
                  v-if="composerAdvancedOpen"
                  data-testid="workbench-composer-advanced"
                  class="grid grid-cols-1 gap-3 pt-1"
                >
                  <div class="space-y-1.5">
                    <label class="text-xs text-surface-500" for="task-title">
                      标题（可选）
                    </label>
                    <input
                      id="task-title"
                      v-model="title"
                      class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:border-primary-500/30 transition-all"
                      placeholder="例如：修复登录流程"
                    />
                  </div>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div class="space-y-1.5">
                      <label
                        class="text-xs text-surface-500"
                        for="budget-tokens"
                      >
                        Token 预算（可选）
                      </label>
                      <input
                        id="budget-tokens"
                        v-model="budgetTokens"
                        inputmode="numeric"
                        class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:border-primary-500/30 transition-all"
                        placeholder="例如：20000"
                      />
                    </div>
                    <div class="space-y-1.5">
                      <label class="text-xs text-surface-500" for="budget-cost">
                        成本预算 USD（可选）
                      </label>
                      <input
                        id="budget-cost"
                        v-model="budgetCost"
                        inputmode="decimal"
                        class="px-3 py-2 rounded-xl bg-surface-950/60 border border-surface-800 text-surface-200 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:border-primary-500/30 transition-all"
                        placeholder="例如：0.30"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Task details -->
          <div class="glass rounded-2xl overflow-hidden">
            <!-- Header: Title + Actions -->
            <div
              class="px-5 py-4 flex items-center justify-between gap-4 border-b border-surface-700/50"
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
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                  :disabled="!canCancel || selectedLoading"
                  @click="doCancel"
                >
                  <X class="w-4 h-4" />
                  取消
                </button>
                <button
                  data-testid="workbench-resume"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                  :disabled="!canResume || selectedLoading"
                  @click="doResume"
                >
                  <RotateCcw class="w-4 h-4" />
                  继续
                </button>
                <button
                  type="button"
                  data-testid="workbench-review-toggle"
                  class="px-3 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 disabled:opacity-50 inline-flex items-center gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                  :disabled="!latestAttempt"
                  :class="
                    activeDetailTab === 'review'
                      ? 'bg-indigo-500/20 text-indigo-300'
                      : ''
                  "
                  @click="toggleReviewTab"
                >
                  <ListTodo class="w-4 h-4" />
                  审查
                </button>
              </div>
            </div>

            <!-- Tab Navigation -->
            <div
              v-if="selectedTask"
              class="px-5 py-3 flex items-center gap-1 border-b border-surface-700/50 overflow-x-auto"
            >
              <button
                v-for="tab in detailTabs"
                :key="tab.key"
                :data-testid="
                  tab.key === 'advanced'
                    ? 'workbench-details-advanced-toggle'
                    : undefined
                "
                class="px-4 py-2 rounded-xl text-xs font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                :class="
                  activeDetailTab === tab.key
                    ? 'bg-surface-700/60 text-surface-100 shadow-sm'
                    : 'text-surface-500 hover:text-surface-300 hover:bg-surface-900/30'
                "
                @click="activeDetailTab = tab.key"
              >
                {{ tab.label }}
              </button>
            </div>

            <ErrorBanner
              v-if="selectedError"
              class="m-4"
              :error="selectedError"
              title="加载失败"
            />

            <div v-if="!selectedTask" class="p-6">
              <div class="glass-card p-8 text-center">
                <MousePointerClick class="w-6 h-6 text-surface-400 mx-auto" />
                <div class="mt-3 text-sm font-semibold text-surface-200">
                  还没有选择任务
                </div>
                <div class="mt-1 text-xs text-surface-500">
                  从左侧选择一个任务查看详情，或直接创建一个新任务。
                </div>
                <div
                  class="mt-5 flex flex-wrap items-center justify-center gap-2"
                >
                  <button
                    type="button"
                    data-testid="workbench-empty-focus"
                    class="px-3 py-2 rounded-xl text-xs font-medium bg-primary-600 text-white hover:bg-primary-500 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                    @click="focusComposer"
                  >
                    聚焦到新建任务
                  </button>
                  <button
                    type="button"
                    class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-200 hover:bg-surface-800/60 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                    @click="refreshTasks"
                  >
                    刷新列表
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="p-4 pt-2">
              <!-- Overview Tab -->
              <div v-if="activeDetailTab === 'overview'" class="space-y-4">
                <!-- Two-column layout: Status info | Details -->
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  <!-- Left: Task Summary -->
                  <div class="space-y-3">
                    <div class="glass-card p-4">
                      <div class="flex items-center gap-3 mb-4">
                        <div
                          class="w-3 h-3 rounded-full"
                          :class="{
                            'bg-emerald-400': latestStatus === 'succeeded',
                            'bg-rose-400': [
                              'failed',
                              'canceled',
                              'limit_exceeded',
                              'timed_out',
                              'interrupted',
                            ].includes(latestStatus),
                            'bg-amber-400': latestStatus === 'running',
                            'bg-surface-400': latestStatus === 'queued',
                          }"
                        ></div>
                        <span
                          class="text-sm font-medium text-surface-100 capitalize"
                          >{{ latestStatus || "queued" }}</span
                        >
                      </div>
                      <div
                        v-if="formatUsage(latestAttempt)"
                        class="flex items-center gap-3"
                      >
                        <div
                          class="px-3 py-1.5 rounded-lg bg-surface-900/60 text-xs text-surface-300"
                        >
                          {{ formatUsage(latestAttempt) }}
                        </div>
                      </div>
                    </div>

                    <div v-if="latestAttempt?.observer" class="glass-card p-4">
                      <div class="flex items-center gap-2 mb-3">
                        <div
                          class="w-2.5 h-2.5 rounded-full"
                          :class="
                            latestAttempt.observer.pass
                              ? 'bg-emerald-400'
                              : 'bg-rose-400'
                          "
                        ></div>
                        <span class="text-xs font-medium text-surface-200"
                          >观察者：{{
                            latestAttempt.observer.pass ? "通过" : "失败"
                          }}</span
                        >
                      </div>
                      <p class="text-xs text-surface-400 mb-3">
                        {{ latestAttempt.observer.reason }}
                      </p>
                      <div
                        v-if="
                          !latestAttempt.observer.pass &&
                          latestAttempt.observer.next_steps
                        "
                        class="p-3 rounded-xl bg-surface-900/60"
                      >
                        <p class="text-[11px] text-surface-500 mb-1">
                          下一步：
                        </p>
                        <p class="text-xs text-surface-300 whitespace-pre-wrap">
                          {{ latestAttempt.observer.next_steps }}
                        </p>
                      </div>
                      <div
                        v-if="
                          !latestAttempt.observer.pass &&
                          latestAttempt.observer.questions_for_user?.length
                        "
                        class="mt-3 p-3 rounded-xl bg-amber-500/10 border border-amber-500/20"
                      >
                        <p class="text-[11px] text-amber-400 mb-1">
                          需要你确认：
                        </p>
                        <p class="text-xs text-surface-300 whitespace-pre-wrap">
                          {{
                            latestAttempt.observer.questions_for_user.join("\n")
                          }}
                        </p>
                      </div>
                    </div>
                  </div>

                  <!-- Right: Summary Text -->
                  <div class="glass-card p-5 h-fit">
                    <div class="flex items-center justify-between mb-4">
                      <span class="text-sm font-semibold text-surface-100"
                        >执行摘要</span
                      >
                      <span class="text-xs text-surface-500 font-mono">{{
                        latestAttempt?.id?.slice(0, 8)
                      }}</span>
                    </div>
                    <div
                      v-if="latestAttempt?.summary"
                      class="text-sm text-surface-300 whitespace-pre-wrap leading-relaxed"
                    >
                      {{ latestAttempt.summary }}
                    </div>
                    <div
                      v-else
                      class="text-sm text-surface-500 italic py-8 text-center"
                    >
                      暂无执行摘要
                    </div>
                  </div>
                </div>
              </div>

              <!-- Review Tab -->
              <div v-else-if="activeDetailTab === 'review'" class="space-y-4">
                <div
                  v-if="!latestAttempt"
                  class="text-sm text-surface-500 text-center py-8"
                >
                  暂无尝试记录
                </div>
                <div v-else class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <!-- Left: File changes -->
                  <div class="space-y-4">
                    <div class="glass-card p-4">
                      <div class="flex items-center justify-between mb-3">
                        <span class="text-sm font-semibold text-surface-100"
                          >变更文件</span
                        >
                        <span
                          v-if="changedFiles?.content"
                          class="text-xs text-surface-500"
                          >{{
                            changedFiles.content
                              .split("\n")
                              .filter((l) => l.trim()).length
                          }}
                          文件</span
                        >
                      </div>
                      <pre
                        v-if="changedFiles?.content"
                        class="max-h-[40vh] overflow-auto rounded-xl bg-surface-950/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                        >{{ changedFiles.content }}</pre
                      >
                      <div
                        v-else
                        class="py-8 text-sm text-surface-500 text-center italic"
                      >
                        暂无变更文件列表
                      </div>
                    </div>
                  </div>

                  <!-- Right: Diff + Comments -->
                  <div class="space-y-4">
                    <div class="glass-card p-4">
                      <div class="text-sm font-semibold text-surface-100 mb-3">
                        Diff Patch
                      </div>
                      <pre
                        v-if="diffPatch?.content"
                        class="max-h-[30vh] overflow-auto rounded-xl bg-surface-950/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                        >{{ diffPatch.content }}</pre
                      >
                      <div
                        v-else
                        class="py-8 text-sm text-surface-500 text-center italic"
                      >
                        暂无 diff patch
                      </div>
                    </div>

                    <!-- Review Comments -->
                    <div class="glass-card p-4">
                      <div class="text-sm font-semibold text-surface-100 mb-3">
                        Review Comments
                      </div>
                      <div v-if="reviewComments.length" class="space-y-2 mb-4">
                        <div
                          v-for="(cmt, idx) in reviewComments"
                          :key="idx"
                          class="rounded-xl bg-surface-900/50 p-3 text-sm text-surface-200"
                        >
                          <div class="text-[11px] text-surface-500 mb-1">
                            {{ cmt.ts }} · {{ cmt.principal_id }}
                          </div>
                          <div class="whitespace-pre-wrap">
                            {{ cmt.comment }}
                          </div>
                        </div>
                      </div>

                      <textarea
                        data-testid="review-comment-input"
                        v-model="reviewCommentDraft"
                        class="w-full rounded-xl bg-surface-900/50 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-indigo-500/30 transition-all"
                        rows="2"
                        placeholder="写一条 review comment..."
                      />
                      <div class="mt-2 flex justify-end gap-2">
                        <button
                          data-testid="review-comment-submit"
                          class="px-3 py-2 rounded-xl text-sm font-medium bg-indigo-500/80 text-white hover:bg-indigo-500 disabled:opacity-50 transition-colors"
                          :disabled="
                            submittingReviewComment ||
                            !reviewCommentDraft.trim()
                          "
                          @click="submitReviewComment"
                        >
                          提交评论
                        </button>
                      </div>
                    </div>

                    <!-- Follow-up -->
                    <div class="glass-card p-4">
                      <div class="text-sm font-semibold text-surface-100 mb-3">
                        Follow-up Attempt
                      </div>
                      <textarea
                        data-testid="follow-up-notes-input"
                        v-model="followUpNotes"
                        class="w-full rounded-xl bg-surface-900/50 border border-surface-700/40 p-3 text-surface-100 text-sm outline-none focus:ring-2 focus:ring-emerald-500/30 transition-all"
                        rows="2"
                        placeholder="输入跟进指令..."
                      />
                      <div class="mt-2 flex justify-end">
                        <button
                          data-testid="follow-up-submit"
                          class="px-3 py-2 rounded-xl text-sm font-medium bg-emerald-500/80 text-white hover:bg-emerald-500 disabled:opacity-50 transition-colors"
                          :disabled="
                            submittingFollowUp || !followUpNotes.trim()
                          "
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
              <div v-else-if="activeDetailTab === 'events'" class="space-y-4">
                <EventLogViewer
                  :events="selectedEvents"
                  :loading="selectedLoading && selectedEvents.length === 0"
                  :refreshing="selectedLoading && selectedEvents.length > 0"
                />
              </div>

              <!-- Advanced Tab -->
              <div v-else-if="activeDetailTab === 'advanced'" class="space-y-4">
                <div data-testid="workbench-details-advanced">
                  <div
                    v-if="!latestAttempt"
                    class="text-sm text-surface-500 text-center py-8"
                  >
                    暂无尝试记录
                  </div>
                  <div v-else class="space-y-3">
                    <div
                      v-if="
                        latestAttempt.worktree_root ||
                        latestAttempt.base_commit_sha ||
                        latestAttempt.base_ref
                      "
                      class="glass-card p-4"
                      data-testid="workbench-worktree-evidence"
                    >
                      <div class="text-xs font-semibold text-surface-200">
                        Worktree
                      </div>
                      <div class="mt-2 space-y-2">
                        <div
                          v-if="latestAttempt.worktree_root"
                          class="space-y-1"
                        >
                          <div class="text-[11px] text-surface-500">
                            worktree_root
                          </div>
                          <div
                            class="text-[11px] text-surface-200 font-mono break-all"
                            data-testid="workbench-worktree-root"
                          >
                            {{ latestAttempt.worktree_root }}
                          </div>
                        </div>

                        <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
                          <div v-if="latestAttempt.base_ref" class="space-y-1">
                            <div class="text-[11px] text-surface-500">
                              base_ref
                            </div>
                            <div
                              class="text-[11px] text-surface-200 font-mono break-all"
                              data-testid="workbench-base-ref"
                            >
                              {{ latestAttempt.base_ref }}
                            </div>
                          </div>
                          <div
                            v-if="latestAttempt.base_commit_sha"
                            class="space-y-1"
                          >
                            <div class="text-[11px] text-surface-500">
                              base_commit_sha
                            </div>
                            <div
                              class="text-[11px] text-surface-200 font-mono break-all"
                              data-testid="workbench-base-commit-sha"
                            >
                              {{ latestAttempt.base_commit_sha }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    <div
                      v-if="advancedArtifacts.length"
                      class="grid grid-cols-1 lg:grid-cols-2 gap-3"
                    >
                      <button
                        v-for="item in advancedArtifacts"
                        :key="item.kind"
                        type="button"
                        class="glass-card p-4 text-left hover:bg-surface-900/40 transition-all group"
                        :data-testid="`artifact-card-${item.kind}`"
                        @click="openArtifactModal(item)"
                      >
                        <div
                          class="flex items-center justify-between gap-2 mb-2"
                        >
                          <div
                            class="text-xs font-medium text-surface-300 uppercase tracking-wide"
                          >
                            {{ item.label }}
                          </div>
                          <div
                            class="text-[11px] text-surface-500 group-hover:text-surface-300 transition-colors"
                          >
                            查看 →
                          </div>
                        </div>
                        <div
                          class="text-[11px] text-surface-500 font-mono truncate"
                        >
                          {{ item.path }}
                        </div>
                      </button>
                    </div>
                    <div
                      v-else
                      class="text-sm text-surface-500 text-center py-8"
                    >
                      暂无可预览的文件
                    </div>
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
                <div class="text-xs text-surface-500">
                  {{ latestAttempt?.id }}
                </div>
              </div>

              <ErrorBanner
                v-if="reviewError"
                :error="reviewError"
                title="审查失败"
              />

              <div v-if="reviewLoading" class="text-xs text-surface-500">
                加载中…
              </div>

              <div v-else class="space-y-3">
                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">变更文件</div>
                  <pre
                    v-if="changedFiles?.content"
                    class="mt-2 max-h-[20vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                    >{{ changedFiles?.content }}</pre
                  >
                  <div v-else class="mt-2 text-surface-500">
                    暂无变更文件列表（可能无改动或未生成）。
                  </div>
                </div>

                <div class="text-xs text-surface-400">
                  <div class="font-semibold text-surface-200">Diff Patch</div>
                  <pre
                    v-if="diffPatch?.content"
                    class="mt-2 max-h-[30vh] overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap"
                    >{{ diffPatch?.content }}</pre
                  >
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
                      :disabled="
                        submittingReviewComment || !reviewCommentDraft.trim()
                      "
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
    <div
      v-if="artifactModalOpen"
      data-testid="artifact-modal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4"
    >
      <div
        class="absolute inset-0 bg-black/70"
        @click="closeArtifactModal"
      ></div>
      <div
        class="relative w-full max-w-5xl rounded-3xl bg-surface-900 shadow-2xl overflow-hidden"
      >
        <!-- Header -->
        <div
          class="px-5 py-4 bg-surface-800/50 flex items-start justify-between gap-3"
        >
          <div class="min-w-0">
            <div class="text-sm font-semibold text-surface-100 truncate">
              {{ artifactModalLabel }}
            </div>
            <div class="text-xs text-surface-500 font-mono break-all mt-0.5">
              {{ artifactModalPath }}
            </div>
          </div>
          <button
            type="button"
            class="px-3 py-1.5 rounded-lg text-sm font-medium bg-surface-700/50 text-surface-300 hover:bg-surface-600/50 transition-colors"
            @click="closeArtifactModal"
          >
            关闭
          </button>
        </div>

        <!-- Content -->
        <div class="p-5">
          <div
            v-if="artifactModalLoading"
            class="text-sm text-surface-500 py-8 text-center"
          >
            加载中…
          </div>
          <ErrorBanner
            v-else-if="artifactModalError"
            :error="artifactModalError"
            title="加载失败"
          />
          <div v-else class="space-y-3">
            <div
              v-if="artifactModalContent?.truncated"
              class="text-xs text-amber-400 px-1"
            >
              内容已截断（仅展示前 512KB）
            </div>
            <pre
              class="max-h-[65vh] overflow-auto rounded-2xl bg-surface-950/60 p-4 text-[12px] text-surface-200 whitespace-pre font-mono leading-relaxed"
              >{{ artifactModalContent?.content }}</pre
            >
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
