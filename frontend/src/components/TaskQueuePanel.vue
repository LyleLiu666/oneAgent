<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { Loader2, RefreshCw, X, RotateCcw, ListTodo } from "lucide-vue-next";

import EventLogViewer from "@/components/EventLogViewer.vue";

import {
  cancelTask,
  createTask,
  getTask,
  getTaskEvents,
  listTasks,
  resumeTask,
  type Task,
  type TaskAttempt,
  type TaskEvent,
} from "@/api/client";
import {
  diffTaskUpdates,
  saveTaskSnapshotsToStorage,
  type TaskSnapshot,
  type TaskUpdate,
} from "@/lib/taskUpdates";

const props = defineProps<{
  workspace: string;
  modelId: string;
}>();

const open = ref(false);

const title = ref("");
const prompt = ref("");
const submitting = ref(false);

const tasks = ref<Task[]>([]);
const tasksLoading = ref(false);
const tasksError = ref("");

const TASK_SNAPSHOT_KEY = "oneagent-task-snapshots";
const notifyInitialized = ref(false);
const taskSnapshots = ref<Record<string, TaskSnapshot>>({});
const taskUpdates = ref<TaskUpdate[]>([]);

const selectedTaskId = ref<string>("");
const selectedTask = ref<Task | null>(null);
const selectedEvents = ref<TaskEvent[]>([]);
const selectedLoading = ref(false);
const selectedError = ref("");

const effectiveWorkspace = computed(() => String(props.workspace || "").trim());

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

const latestAttempt = computed<TaskAttempt | null>(() => {
  const t = selectedTask.value;
  if (!t || !Array.isArray(t.attempts) || t.attempts.length === 0) return null;
  return t.attempts[t.attempts.length - 1] || null;
});

const latestStatus = computed(() => latestAttempt.value?.status || "");

const canCancel = computed(() => {
  const s = latestStatus.value;
  return s === "queued" || s === "running";
});

const canResume = computed(() => {
  const s = latestStatus.value;
  return (
    s === "failed" ||
    s === "limit_exceeded" ||
    s === "timed_out" ||
    s === "interrupted"
  );
});

const refreshTasks = async () => {
  tasksError.value = "";
  const ws = effectiveWorkspace.value;
  if (!ws) {
    tasks.value = [];
    return;
  }

  tasksLoading.value = true;
  try {
    const nextTasks = await listTasks(ws);

    if (!notifyInitialized.value) {
      // Establish baseline without emitting notifications.
      const { next } = diffTaskUpdates({}, nextTasks);
      taskSnapshots.value = next;
      saveTaskSnapshotsToStorage(TASK_SNAPSHOT_KEY, next);
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

const refreshSelected = async () => {
  selectedError.value = "";
  const id = String(selectedTaskId.value || "").trim();
  if (!id) {
    selectedTask.value = null;
    selectedEvents.value = [];
    return;
  }

  selectedLoading.value = true;
  try {
    const [t, evs] = await Promise.all([getTask(id), getTaskEvents(id)]);
    selectedTask.value = t;
    selectedEvents.value = Array.isArray(evs) ? sortTaskEventsNewestFirst(evs) : [];
  } catch (e: any) {
    selectedError.value = String(
      e?.data?.error || e?.message || "Failed to load task",
    );
  } finally {
    selectedLoading.value = false;
  }
};

const queueTask = async () => {
  tasksError.value = "";
  const ws = effectiveWorkspace.value;
  const p = String(prompt.value || "").trim();
  if (!ws || !p) return;

  submitting.value = true;
  try {
    const created = await createTask({
      workspace: ws,
      title: String(title.value || "").trim() || undefined,
      prompt: p,
      model_id: String(props.modelId || "").trim() || undefined,
    });
    prompt.value = "";
    title.value = "";
    selectedTaskId.value = created.id;
    await refreshTasks();
    await refreshSelected();
    open.value = true;
  } catch (e: any) {
    tasksError.value = String(
      e?.data?.error || e?.message || "Failed to create task",
    );
  } finally {
    submitting.value = false;
  }
};

const doCancel = async () => {
  const id = String(selectedTaskId.value || "").trim();
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
  const id = String(selectedTaskId.value || "").trim();
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

const clearTaskUpdates = () => {
  taskUpdates.value = [];
};

watch(
  () => effectiveWorkspace.value,
  async () => {
    selectedTaskId.value = "";
    selectedTask.value = null;
    selectedEvents.value = [];
    await refreshTasks();
  },
  { immediate: true },
);

watch(
  () => selectedTaskId.value,
  async () => {
    await refreshSelected();
  },
);

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
  <div
    class="shrink-0 border-b border-surface-900/80 bg-surface-950/70 backdrop-blur"
  >
    <div class="max-w-4xl mx-auto px-4 py-2">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2 min-w-0">
          <ListTodo class="w-4 h-4 text-surface-400" />
          <p class="text-[10px] uppercase tracking-[0.2em] text-surface-500">
            任务
          </p>
          <p
            v-if="effectiveWorkspace"
            class="text-xs text-surface-300 truncate"
          >
            工作区内 {{ tasks.length }} 个任务
          </p>
          <p v-else class="text-xs text-surface-500">
            选择工作区以使用任务队列
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            data-testid="tasks-refresh"
            class="text-xs text-surface-300 hover:text-surface-100"
            :disabled="tasksLoading"
            title="刷新"
            @click="refreshTasks"
          >
            <Loader2 v-if="tasksLoading" class="w-4 h-4 animate-spin" />
            <RefreshCw v-else class="w-4 h-4" />
          </button>
          <button
            type="button"
            class="text-xs text-surface-300 hover:text-surface-100"
            :disabled="!effectiveWorkspace"
            data-testid="tasks-toggle"
            @click="open = !open"
          >
            {{ open ? "收起" : "展开" }}
          </button>
        </div>
      </div>

      <div v-if="open" class="mt-3 space-y-3">
        <div v-if="tasksError" class="text-xs text-red-400">
          {{ tasksError }}
        </div>

        <div
          v-if="taskUpdates.length"
          data-testid="task-updates"
          class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs font-semibold text-surface-200">更新</div>
            <button
              class="text-[11px] text-surface-400 hover:text-surface-200"
              @click="clearTaskUpdates"
            >
              清空
            </button>
          </div>
          <div class="mt-2 space-y-1">
            <div
              v-for="u in taskUpdates"
              :key="`${u.taskId}:${u.attemptId}:${u.status}`"
              class="text-xs text-surface-200 truncate"
            >
              <span class="font-mono text-surface-400">{{ u.status }}</span>
              <span class="mx-2 text-surface-600">·</span>
              <span class="text-surface-100">{{ u.title }}</span>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <p
                class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
              >
                新任务
              </p>
            </div>

            <input
              v-model="title"
              class="w-full bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
              placeholder="可选标题"
              :disabled="!effectiveWorkspace || submitting"
            />
            <textarea
              v-model="prompt"
              rows="3"
              class="w-full bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 resize-none"
              placeholder="描述要交付的结果（后台运行）"
              :disabled="!effectiveWorkspace || submitting"
            />
            <div class="flex items-center justify-end gap-2">
              <button
                type="button"
                class="bg-primary-600 text-white text-xs rounded-lg px-3 py-1.5 hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
                data-testid="tasks-queue"
                :disabled="!effectiveWorkspace || submitting || !prompt.trim()"
                @click="queueTask"
              >
                <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
                <span v-else>入队</span>
              </button>
            </div>

            <div class="mt-3">
              <p
                class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
              >
                队列
              </p>
              <div class="mt-2 space-y-1 max-h-52 overflow-y-auto pr-1">
                <button
                  v-for="t in tasks"
                  :key="t.id"
                  type="button"
                  data-testid="task-item"
                  :data-task-id="t.id"
                  class="w-full text-left rounded-lg border px-2 py-2 hover:bg-surface-900/60"
                  :class="[
                    selectedTaskId === t.id
                      ? 'border-primary-500/60 bg-surface-900/70'
                      : 'border-surface-800 bg-surface-950/40',
                  ]"
                  @click="selectedTaskId = t.id"
                >
                  <div class="flex items-center justify-between gap-2">
                    <p class="text-xs text-surface-100 truncate">
                      {{ t.title || t.id }}
                    </p>
                    <span
                      class="text-[10px] uppercase tracking-[0.2em] text-surface-400"
                    >
                      {{
                        t.attempts?.[t.attempts.length - 1]?.status || "unknown"
                      }}
                    </span>
                  </div>
                  <p class="text-[11px] text-surface-400 truncate">
                    {{ t.prompt }}
                  </p>
                </button>
                <div
                  v-if="!tasksLoading && tasks.length === 0"
                  class="text-xs text-surface-500"
                >
                  暂无任务。
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <p
                class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
              >
                详情
              </p>
              <div class="flex items-center gap-2">
                <button
                  v-if="selectedTaskId"
                  type="button"
                  class="text-xs text-surface-300 hover:text-surface-100"
                  :disabled="selectedLoading"
                  title="刷新"
                  @click="refreshSelected"
                >
                  <Loader2
                    v-if="selectedLoading"
                    class="w-4 h-4 animate-spin"
                  />
                  <RefreshCw v-else class="w-4 h-4" />
                </button>
                <button
                  v-if="selectedTaskId"
                  type="button"
                  class="text-xs text-surface-300 hover:text-surface-100"
                  title="清除选择"
                  @click="
                    selectedTaskId = '';
                    selectedTask = null;
                    selectedEvents = [];
                  "
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>

            <div v-if="selectedError" class="text-xs text-red-400">
              {{ selectedError }}
            </div>

            <div v-if="!selectedTaskId" class="text-xs text-surface-500">
              选择任务查看详情。
            </div>

            <div
              v-else-if="selectedTask"
              class="rounded-xl border border-surface-800 bg-surface-950/40 p-3"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="min-w-0">
                  <p class="text-xs text-surface-100 truncate">
                    {{ selectedTask.title }}
                  </p>
                  <p class="text-[11px] text-surface-400 truncate">
                    {{ selectedTask.id }}
                  </p>
                </div>
                <span
                  class="text-[10px] uppercase tracking-[0.2em] text-surface-300"
                >
                  {{ latestAttempt?.status || "unknown" }}
                </span>
              </div>

              <div class="mt-3 space-y-2">
                <div
                  v-if="latestAttempt?.summary"
                  class="text-xs text-surface-200 whitespace-pre-wrap"
                >
                  {{ latestAttempt.summary }}
                </div>

                <div v-if="latestAttempt?.observer" class="text-xs">
                  <p
                    class="text-surface-200"
                    :class="
                      latestAttempt.observer.pass
                        ? 'text-green-400'
                        : 'text-red-400'
                    "
                  >
                    观察者：{{ latestAttempt.observer.pass ? "通过" : "失败" }}
                  </p>
                  <p
                    v-if="latestAttempt.observer.reason"
                    class="text-surface-300"
                  >
                    {{ latestAttempt.observer.reason }}
                  </p>
                  <p
                    v-if="
                      !latestAttempt.observer.pass &&
                      latestAttempt.observer.next_steps
                    "
                    class="text-surface-400 whitespace-pre-wrap"
                  >
                    下一步：{{ latestAttempt.observer.next_steps }}
                  </p>
                  <p
                    v-if="
                      !latestAttempt.observer.pass &&
                      latestAttempt.observer.questions_for_user &&
                      latestAttempt.observer.questions_for_user.length
                    "
                    class="text-surface-400 whitespace-pre-wrap"
                  >
                    需要你确认：{{
                      latestAttempt.observer.questions_for_user.join("\n")
                    }}
                  </p>
                </div>

                <div v-if="latestAttempt?.findings_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    findings_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.findings_path
                  }}</code>
                </div>
                <div v-if="latestAttempt?.trace_log_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    trace_log_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.trace_log_path
                  }}</code>
                </div>
                <div v-if="latestAttempt?.test_report_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    test_report_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.test_report_path
                  }}</code>
                </div>
                <div v-if="latestAttempt?.project_config_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    project_config_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.project_config_path
                  }}</code>
                </div>
                <div v-if="latestAttempt?.copy_files_log_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    copy_files_log_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.copy_files_log_path
                  }}</code>
                </div>
                <div
                  v-if="latestAttempt?.setup_script_log_path"
                  class="text-xs"
                >
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    setup_script_log_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.setup_script_log_path
                  }}</code>
                </div>
                <div v-if="latestAttempt?.test_script_log_path" class="text-xs">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    test_script_log_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.test_script_log_path
                  }}</code>
                </div>
                <div
                  v-if="latestAttempt?.cleanup_script_log_path"
                  class="text-xs"
                >
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    cleanup_script_log_path
                  </p>
                  <code class="text-[11px] text-surface-300 break-all">{{
                    latestAttempt.cleanup_script_log_path
                  }}</code>
                </div>

                <div class="flex items-center justify-end gap-2 pt-1">
                  <button
                    type="button"
                    class="text-xs text-surface-300 hover:text-surface-100 disabled:opacity-50 disabled:cursor-not-allowed"
                    :disabled="!canCancel"
                    @click="doCancel"
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    class="text-xs text-surface-300 hover:text-surface-100 disabled:opacity-50 disabled:cursor-not-allowed"
                    :disabled="!canResume"
                    @click="doResume"
                  >
                    <RotateCcw class="w-3 h-3 inline-block mr-1" />
                    继续
                  </button>
                </div>

                <div class="pt-2">
                  <p
                    class="text-[10px] uppercase tracking-[0.2em] text-surface-500"
                  >
                    事件
                  </p>
                  <div class="mt-2 max-h-40 overflow-y-auto pr-1">
                    <EventLogViewer
                      :events="selectedEvents"
                      :loading="selectedLoading && selectedEvents.length === 0"
                      :refreshing="selectedLoading && selectedEvents.length > 0"
                      compact
                    />
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="text-xs text-surface-500">加载中…</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
