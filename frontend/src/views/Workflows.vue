<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  Folder,
  Plus,
  RefreshCw,
  Trash2,
  Pencil,
  Upload,
  Play,
  ExternalLink,
} from "lucide-vue-next";
import { useRouter } from "vue-router";

import ErrorBanner from "@/components/ErrorBanner.vue";
import WorkflowGraphEditor from "@/components/WorkflowGraphEditor.vue";

import {
  chooseWorkspaceDir,
  createWorkflow,
  createWorkflowRun,
  deleteWorkflow,
  executeWorkflowRun,
  getConfig,
  listWorkflows,
  publishWorkflowVersion,
  renameWorkflow,
  type Workflow,
  type WorkflowGraph,
  type WorkflowRun,
  type WorkflowVersion,
} from "@/api/client";
import { parseApiError, type ParsedApiError } from "@/lib/apiError";
import { resolveWorkspaceChooserSupport, unknownWorkspaceChooserSupport } from "@/lib/workspaceChooser";

const router = useRouter();

const STORAGE_KEY = "oneagent-workspaces";

const normalizeWorkspace = (ws: string) => String(ws || "").trim();

const workspacesManual = ref<string[]>([]);
const workspaceNew = ref("");
const workspaceSelected = ref("");
const workspaceChoosing = ref(false);
const workspaceChooseError = ref<ParsedApiError | null>(null);
const workspaceChooserSupported = ref(true);
const workspaceChooserHint = ref("");

const workflowsLoading = ref(false);
const workflowsError = ref<ParsedApiError | null>(null);
const workflows = ref<Workflow[]>([]);

const workflowNewName = ref("");
const creatingWorkflow = ref(false);
const createError = ref<ParsedApiError | null>(null);

const selectedWorkflowId = ref("");
const selectedWorkflow = computed<Workflow | null>(() => {
  const id = String(selectedWorkflowId.value || "");
  return workflows.value.find((w) => w.workflow_id === id) || null;
});

const graphDraft = ref<WorkflowGraph>({ nodes: [], edges: [] });
const latestVersion = ref<WorkflowVersion | null>(null);
const runDraft = ref<WorkflowRun | null>(null);

const persistDefaultWorkspace = (ws: string) => {
  const v = normalizeWorkspace(ws);
  if (!v) return;
  try {
    localStorage.setItem("oneagent-workspace", v);
  } catch {
    // ignore
  }
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
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(workspacesManual.value));
  } catch {
    // ignore
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
  if (!workspaceChooserSupported.value) return;
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

const loadWorkspaceChooserSupport = async () => {
  try {
    const chooser = resolveWorkspaceChooserSupport(await getConfig());
    workspaceChooserSupported.value = chooser.supported;
    workspaceChooserHint.value = chooser.hint;
  } catch {
    const chooser = unknownWorkspaceChooserSupport();
    workspaceChooserSupported.value = chooser.supported;
    workspaceChooserHint.value = chooser.hint;
  }
};

const selectWorkspace = (ws: string) => {
  workspaceSelected.value = normalizeWorkspace(ws);
  persistDefaultWorkspace(workspaceSelected.value);
};

const loadWorkflowsForWorkspace = async () => {
  workflowsError.value = null;
  const ws = normalizeWorkspace(workspaceSelected.value);
  if (!ws) {
    workflows.value = [];
    selectedWorkflowId.value = "";
    return;
  }
  workflowsLoading.value = true;
  try {
    workflows.value = await listWorkflows(ws);
    if (
      selectedWorkflowId.value &&
      !workflows.value.some((w) => w.workflow_id === selectedWorkflowId.value)
    ) {
      selectedWorkflowId.value = "";
    }
  } catch (e: any) {
    workflowsError.value = parseApiError(e, "加载 workflows 失败");
  } finally {
    workflowsLoading.value = false;
  }
};

watch(workspaceSelected, () => {
  selectedWorkflowId.value = "";
  latestVersion.value = null;
  runDraft.value = null;
  graphDraft.value = { nodes: [], edges: [] };
  void loadWorkflowsForWorkspace();
});

watch(selectedWorkflowId, () => {
  latestVersion.value = null;
  runDraft.value = null;
  graphDraft.value = { nodes: [], edges: [] };
});

const selectWorkflow = (wf: Workflow) => {
  selectedWorkflowId.value = wf.workflow_id;
};

const createNewWorkflow = async () => {
  createError.value = null;
  const ws = normalizeWorkspace(workspaceSelected.value);
  const name = String(workflowNewName.value || "").trim();
  if (!ws || !name) return;

  creatingWorkflow.value = true;
  try {
    const wf = await createWorkflow({ workspace_root: ws, name });
    workflowNewName.value = "";
    await loadWorkflowsForWorkspace();
    selectedWorkflowId.value = wf.workflow_id;
  } catch (e: any) {
    createError.value = parseApiError(e, "创建 workflow 失败");
  } finally {
    creatingWorkflow.value = false;
  }
};

const renameOneWorkflow = async (wf: Workflow) => {
  const ws = normalizeWorkspace(workspaceSelected.value);
  if (!ws) return;
  const nextName = String(window.prompt("Rename workflow:", wf.name) || "").trim();
  if (!nextName || nextName === wf.name) return;
  workflowsError.value = null;
  try {
    await renameWorkflow(wf.workflow_id, { workspace_root: ws, name: nextName });
    await loadWorkflowsForWorkspace();
  } catch (e: any) {
    workflowsError.value = parseApiError(e, "重命名失败");
  }
};

const deleteOneWorkflow = async (wf: Workflow) => {
  const ws = normalizeWorkspace(workspaceSelected.value);
  if (!ws) return;
  const ok = window.confirm(`Delete workflow "${wf.name}"?`);
  if (!ok) return;
  workflowsError.value = null;
  try {
    await deleteWorkflow(wf.workflow_id, ws);
    await loadWorkflowsForWorkspace();
    if (selectedWorkflowId.value === wf.workflow_id) {
      selectedWorkflowId.value = "";
      latestVersion.value = null;
      runDraft.value = null;
      graphDraft.value = { nodes: [], edges: [] };
    }
  } catch (e: any) {
    workflowsError.value = parseApiError(e, "删除失败");
  }
};

const publishing = ref(false);
const publishError = ref<ParsedApiError | null>(null);
const publishVersion = async () => {
  publishError.value = null;
  const ws = normalizeWorkspace(workspaceSelected.value);
  const wf = selectedWorkflow.value;
  if (!ws || !wf) return;
  publishing.value = true;
  try {
    latestVersion.value = await publishWorkflowVersion(wf.workflow_id, {
      workspace_root: ws,
      graph: graphDraft.value,
    });
  } catch (e: any) {
    publishError.value = parseApiError(e, "发布版本失败");
  } finally {
    publishing.value = false;
  }
};

const running = ref(false);
const runError = ref<ParsedApiError | null>(null);
const createRun = async () => {
  runError.value = null;
  const ws = normalizeWorkspace(workspaceSelected.value);
  const wf = selectedWorkflow.value;
  const v = latestVersion.value;
  if (!ws || !wf || !v) return;
  running.value = true;
  try {
    runDraft.value = await createWorkflowRun(wf.workflow_id, {
      workspace_root: ws,
      version_id: v.version_id,
    });
  } catch (e: any) {
    runError.value = parseApiError(e, "创建 run 失败");
  } finally {
    running.value = false;
  }
};

const executeRun = async () => {
  runError.value = null;
  const ws = normalizeWorkspace(workspaceSelected.value);
  const wf = selectedWorkflow.value;
  const run = runDraft.value;
  if (!ws || !wf || !run) return;
  running.value = true;
  try {
    runDraft.value = await executeWorkflowRun(wf.workflow_id, run.run_id, {
      workspace_root: ws,
      concurrency: 2,
    });
  } catch (e: any) {
    runError.value = parseApiError(e, "执行 run 失败");
  } finally {
    running.value = false;
  }
};

const openRun = async () => {
  const ws = normalizeWorkspace(workspaceSelected.value);
  const wf = selectedWorkflow.value;
  const run = runDraft.value;
  if (!ws || !wf || !run) return;
  await router.push({
    name: "Workflow Run",
    params: { id: wf.workflow_id, runId: run.run_id },
    query: { workspace: ws },
  });
};

onMounted(() => {
  void loadWorkspaceChooserSupport();
  loadManualWorkspaces();
  try {
    const last = normalizeWorkspace(localStorage.getItem("oneagent-workspace") || "");
    if (last) {
      workspaceSelected.value = last;
      if (!workspacesManual.value.includes(last)) {
        workspacesManual.value = [...workspacesManual.value, last];
        saveManualWorkspaces();
      }
    }
  } catch {
    // ignore
  }
  void loadWorkflowsForWorkspace();
});
</script>

<template>
  <div class="min-h-screen bg-surface-950 p-6 text-surface-200">
    <div class="mx-auto max-w-6xl space-y-6">
      <header class="flex flex-col gap-2">
        <h1 class="text-xl font-bold">Workflows</h1>
        <p class="text-sm text-surface-500">（MVP）工作流编排：graph → publish → run</p>
      </header>

      <section class="rounded-2xl border border-surface-800 bg-surface-900/40 p-4">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div class="flex-1">
            <div class="text-xs text-surface-500">workspace</div>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <select
                class="min-w-[280px] rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200"
                :value="workspaceSelected"
                @change="selectWorkspace(($event.target as HTMLSelectElement).value)"
              >
                <option value="">Select workspace…</option>
                <option v-for="ws in workspacesManual" :key="ws" :value="ws">
                  {{ ws }}
                </option>
              </select>

              <button
                class="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700"
                :disabled="workspaceChoosing || !workspaceChooserSupported"
                :title="workspaceChooserSupported ? '选择工作区文件夹' : workspaceChooserHint"
                @click="chooseWorkspace"
              >
                <Folder class="h-4 w-4" />
                选择文件夹
              </button>

              <button
                class="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700"
                :disabled="workflowsLoading"
                @click="loadWorkflowsForWorkspace"
              >
                <RefreshCw class="h-4 w-4" />
                刷新
              </button>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <input
                v-model="workspaceNew"
                class="min-w-[280px] rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200 placeholder:text-surface-600"
                placeholder="手动添加 workspace 路径…"
              />
              <button
                class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-2 text-sm font-medium text-primary-300 hover:bg-primary-500/20"
                @click="addWorkspace"
              >
                <Plus class="h-4 w-4" />
                添加
              </button>
            </div>
          </div>
        </div>

        <p v-if="!workspaceChooserSupported" class="mt-3 text-xs text-amber-400">
          {{ workspaceChooserHint }}
        </p>
        <ErrorBanner v-if="workspaceChooseError" class="mt-3" :error="workspaceChooseError" />
      </section>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <section class="rounded-2xl border border-surface-800 bg-surface-900/40 p-4 lg:col-span-1">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold text-surface-200">Workflow List</h2>
            <span v-if="workflowsLoading" class="text-xs text-surface-500">loading…</span>
          </div>

          <ErrorBanner v-if="workflowsError" class="mt-3" :error="workflowsError" />

          <div class="mt-4 flex gap-2">
            <input
              v-model="workflowNewName"
              class="flex-1 rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200 placeholder:text-surface-600"
              placeholder="New workflow name…"
            />
            <button
              class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-2 text-sm font-medium text-primary-300 hover:bg-primary-500/20 disabled:opacity-60"
              :disabled="creatingWorkflow"
              @click="createNewWorkflow"
            >
              <Plus class="h-4 w-4" />
              Create
            </button>
          </div>
          <ErrorBanner v-if="createError" class="mt-3" :error="createError" />

          <div class="mt-4 space-y-2">
            <button
              v-for="wf in workflows"
              :key="wf.workflow_id"
              class="w-full rounded-xl border px-3 py-2 text-left transition-colors"
              :class="
                wf.workflow_id === selectedWorkflowId
                  ? 'border-primary-500/40 bg-primary-500/10'
                  : 'border-surface-800 bg-surface-950/30 hover:bg-surface-900/60'
              "
              @click="selectWorkflow(wf)"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-surface-200">
                    {{ wf.name }}
                  </div>
                  <div class="truncate font-mono text-xs text-surface-500">
                    {{ wf.workflow_id }}
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-1">
                  <button
                    class="rounded-lg px-2 py-1 text-xs text-surface-400 hover:bg-surface-800 hover:text-surface-200"
                    title="Rename"
                    @click.stop="renameOneWorkflow(wf)"
                  >
                    <Pencil class="h-4 w-4" />
                  </button>
                  <button
                    class="rounded-lg px-2 py-1 text-xs text-red-400 hover:bg-red-500/10"
                    title="Delete"
                    @click.stop="deleteOneWorkflow(wf)"
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </div>
            </button>

            <div v-if="!workflowsLoading && workflows.length === 0" class="text-sm text-surface-500">
              No workflows yet.
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-surface-800 bg-surface-900/40 p-4 lg:col-span-2">
          <div v-if="!selectedWorkflow" class="text-sm text-surface-500">
            Select a workflow to edit and run.
          </div>

          <div v-else class="space-y-4">
            <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
              <div class="min-w-0">
                <div class="truncate text-lg font-bold text-surface-200">{{ selectedWorkflow.name }}</div>
                <div class="truncate font-mono text-xs text-surface-500">
                  {{ selectedWorkflow.workflow_id }}
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button
                  class="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700 disabled:opacity-60"
                  :disabled="publishing"
                  @click="publishVersion"
                >
                  <Upload class="h-4 w-4" />
                  Publish
                </button>
                <button
                  class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-2 text-sm font-medium text-primary-300 hover:bg-primary-500/20 disabled:opacity-60"
                  :disabled="running || !latestVersion"
                  @click="createRun"
                >
                  <Plus class="h-4 w-4" />
                  Create Run
                </button>
                <button
                  class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-2 text-sm font-medium text-primary-300 hover:bg-primary-500/20 disabled:opacity-60"
                  :disabled="running || !runDraft"
                  @click="executeRun"
                >
                  <Play class="h-4 w-4" />
                  Execute
                </button>
                <button
                  class="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700 disabled:opacity-60"
                  :disabled="!runDraft"
                  @click="openRun"
                >
                  <ExternalLink class="h-4 w-4" />
                  Open Run
                </button>
              </div>
            </div>

            <ErrorBanner v-if="publishError" :error="publishError" />
            <ErrorBanner v-if="runError" :error="runError" />

            <div class="rounded-xl border border-surface-800 bg-surface-950/30 p-3 text-sm text-surface-400">
              <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
                <div>
                  <span class="text-surface-500">latest_version:</span>
                  <span class="ml-2 font-mono text-surface-200">{{ latestVersion?.version_id || "-" }}</span>
                </div>
                <div>
                  <span class="text-surface-500">run:</span>
                  <span class="ml-2 font-mono text-surface-200">{{ runDraft?.run_id || "-" }}</span>
                  <span class="ml-2 text-surface-500">{{ runDraft?.status || "" }}</span>
                </div>
              </div>
            </div>

            <WorkflowGraphEditor v-model="graphDraft" />
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
