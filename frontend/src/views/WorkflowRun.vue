<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute } from "vue-router";
import { RefreshCw, Play, X } from "lucide-vue-next";

import ErrorBanner from "@/components/ErrorBanner.vue";
import WorkflowRunNodes from "@/components/WorkflowRunNodes.vue";

import {
  cancelWorkflowRun,
  executeWorkflowRun,
  getWorkflowRun,
  type WorkflowRun,
} from "@/api/client";
import { parseApiError, type ParsedApiError } from "@/lib/apiError";

const route = useRoute();

const workflowId = computed(() => String(route.params.id || "").trim());
const runId = computed(() => String(route.params.runId || "").trim());

const run = ref<WorkflowRun | null>(null);
const loading = ref(false);
const error = ref<ParsedApiError | null>(null);

const workspace = ref("");
const resolveWorkspace = () => {
  const fromQuery = String(route.query.workspace || "").trim();
  if (fromQuery) return fromQuery;
  try {
    return String(localStorage.getItem("oneagent-workspace") || "").trim();
  } catch {
    return "";
  }
};

const refresh = async () => {
  error.value = null;
  const wf = workflowId.value;
  const rid = runId.value;
  const ws = String(workspace.value || "").trim();
  if (!wf || !rid || !ws) return;
  loading.value = true;
  try {
    run.value = await getWorkflowRun(wf, rid, ws);
  } catch (e: any) {
    error.value = parseApiError(e, "加载 run 失败");
  } finally {
    loading.value = false;
  }
};

const running = ref(false);
const execute = async () => {
  error.value = null;
  const wf = workflowId.value;
  const rid = runId.value;
  const ws = String(workspace.value || "").trim();
  if (!wf || !rid || !ws) return;
  running.value = true;
  try {
    run.value = await executeWorkflowRun(wf, rid, { workspace_root: ws, concurrency: 2 });
  } catch (e: any) {
    error.value = parseApiError(e, "执行失败");
  } finally {
    running.value = false;
  }
};

const canceling = ref(false);
const cancel = async () => {
  error.value = null;
  const wf = workflowId.value;
  const rid = runId.value;
  const ws = String(workspace.value || "").trim();
  if (!wf || !rid || !ws) return;
  canceling.value = true;
  try {
    run.value = await cancelWorkflowRun(wf, rid, { workspace_root: ws, reason: "user_cancel" });
  } catch (e: any) {
    error.value = parseApiError(e, "取消失败");
  } finally {
    canceling.value = false;
  }
};

let pollTimer: number | undefined;
const startPolling = () => {
  if (pollTimer != null) return;
  pollTimer = window.setInterval(() => {
    const st = String(run.value?.status || "");
    if (st === "running" || st === "queued") {
      void refresh();
    }
  }, 2000);
};

onMounted(() => {
  workspace.value = resolveWorkspace();
  void refresh();
  startPolling();
});

onUnmounted(() => {
  if (pollTimer != null) {
    window.clearInterval(pollTimer);
    pollTimer = undefined;
  }
});
</script>

<template>
  <div class="min-h-screen bg-surface-950 p-6 text-surface-200">
    <div class="mx-auto max-w-5xl space-y-6">
      <header class="space-y-1">
        <h1 class="text-xl font-bold">Workflow Run</h1>
        <div class="text-xs text-surface-500">
          <span class="font-mono">{{ workflowId }}</span>
          <span class="mx-2">/</span>
          <span class="font-mono">{{ runId }}</span>
        </div>
      </header>

      <section class="rounded-2xl border border-surface-800 bg-surface-900/40 p-4">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="text-sm text-surface-400">
            status:
            <span class="ml-2 font-mono text-surface-200">{{ run?.status || "-" }}</span>
            <span v-if="run?.error" class="ml-3 text-red-400">{{ run.error }}</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              class="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700 disabled:opacity-60"
              :disabled="loading"
              @click="refresh"
            >
              <RefreshCw class="h-4 w-4" />
              Refresh
            </button>
            <button
              class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-2 text-sm font-medium text-primary-300 hover:bg-primary-500/20 disabled:opacity-60"
              :disabled="running"
              @click="execute"
            >
              <Play class="h-4 w-4" />
              Execute
            </button>
            <button
              class="inline-flex items-center gap-2 rounded-lg bg-red-500/10 px-3 py-2 text-sm font-medium text-red-400 hover:bg-red-500/15 disabled:opacity-60"
              :disabled="canceling"
              @click="cancel"
            >
              <X class="h-4 w-4" />
              Cancel
            </button>
          </div>
        </div>

        <ErrorBanner v-if="error" class="mt-3" :error="error" />
        <div v-if="!workspace" class="mt-3 text-sm text-surface-500">
          Missing workspace. Go back to Workflows and select a workspace first.
        </div>
      </section>

      <section v-if="run" class="space-y-3">
        <WorkflowRunNodes :run="run" />
      </section>
    </div>
  </div>
</template>

