<script setup lang="ts">
import { computed, ref } from "vue";
import { Copy } from "lucide-vue-next";

import { getWorkflowNodeArtifact, type TaskAttemptArtifactContent, type WorkflowRun } from "@/api/client";

const props = defineProps<{
  run: WorkflowRun;
}>();

const nodes = computed(() => props.run?.graph_snapshot?.nodes || []);
const nodeRuns = computed(() => props.run?.node_runs || {});

const copyText = async (text: string) => {
  const v = String(text || "").trim();
  if (!v) return;
  try {
    await navigator.clipboard.writeText(v);
  } catch {
    // ignore
  }
};

type ArtifactViewState = {
  open: boolean;
  busy: boolean;
  error: string;
  result?: TaskAttemptArtifactContent;
};

const artifactViews = ref<Record<string, ArtifactViewState>>({});

const viewKey = (nodeId: string, kind: string) => `${nodeId}:${kind}`;

const toggleView = async (nodeId: string, kind: string) => {
  const k = viewKey(nodeId, kind);
  const cur = artifactViews.value[k];
  if (cur?.open) {
    artifactViews.value[k] = { ...cur, open: false };
    return;
  }

  artifactViews.value[k] = {
    open: true,
    busy: true,
    error: "",
    result: cur?.result,
  };
  try {
    const res = await getWorkflowNodeArtifact(
      props.run.workflow_id,
      props.run.run_id,
      nodeId,
      kind,
      props.run.workspace_root,
    );
    artifactViews.value[k] = { open: true, busy: false, error: "", result: res };
  } catch (e: any) {
    artifactViews.value[k] = { open: true, busy: false, error: String(e?.message || e) };
  }
};
</script>

<template>
  <div class="space-y-3">
    <div
      v-for="node in nodes"
      :key="node.node_id"
      class="rounded-xl border border-surface-800 bg-surface-900/40 p-4"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="truncate text-sm font-semibold text-surface-200">
            {{ node.title || node.node_id }}
          </div>
          <div class="truncate font-mono text-xs text-surface-500">
            {{ node.node_id }}
          </div>
        </div>

        <div class="shrink-0 text-xs font-medium text-surface-400">
          {{ nodeRuns[node.node_id]?.status || "queued" }}
        </div>
      </div>

      <div v-if="nodeRuns[node.node_id]?.error" class="mt-2 text-sm text-red-400">
        {{ nodeRuns[node.node_id]?.error }}
      </div>

      <div class="mt-3 space-y-2">
        <div
          v-if="(nodeRuns[node.node_id]?.artifacts?.artifacts || []).length"
          class="rounded-lg border border-surface-800 bg-surface-950/30 p-3"
        >
          <div class="text-xs font-semibold text-surface-400">Artifacts</div>
          <ul class="mt-2 space-y-1">
            <li
              v-for="(a, idx) in nodeRuns[node.node_id]?.artifacts?.artifacts || []"
              :key="idx"
              class="rounded-md border border-surface-800/50 bg-surface-950/30 p-2"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="text-[11px] text-surface-500">
                    {{ a.kind || "artifact" }}
                  </div>
                  <div class="truncate font-mono text-xs text-surface-200">
                    {{ a.path }}
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-2">
                  <button
                    class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-surface-400 hover:bg-surface-800 hover:text-surface-200"
                    @click="copyText(a.path)"
                  >
                    <Copy class="h-3.5 w-3.5" />
                    Copy
                  </button>
                  <button
                    v-if="
                      a.kind &&
                      a.kind !== 'deliverable' &&
                      a.kind !== 'file' &&
                      a.kind !== 'dir'
                    "
                    class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-surface-400 hover:bg-surface-800 hover:text-surface-200"
                    @click="toggleView(node.node_id, a.kind)"
                  >
                    View
                  </button>
                </div>
              </div>

              <div
                v-if="artifactViews[viewKey(node.node_id, String(a.kind || ''))]?.open"
                class="mt-2 rounded-md border border-surface-800/50 bg-surface-900/40 p-2"
              >
                <div
                  v-if="artifactViews[viewKey(node.node_id, String(a.kind || ''))]?.busy"
                  class="text-xs text-surface-500"
                >
                  Loading…
                </div>
                <div
                  v-else-if="artifactViews[viewKey(node.node_id, String(a.kind || ''))]?.error"
                  class="text-xs text-red-400"
                >
                  {{ artifactViews[viewKey(node.node_id, String(a.kind || ''))]?.error }}
                </div>
                <pre
                  v-else
                  class="max-h-[320px] overflow-y-auto whitespace-pre-wrap break-words text-xs font-mono text-surface-200 custom-scrollbar"
                >{{ artifactViews[viewKey(node.node_id, String(a.kind || ''))]?.result?.content }}</pre>
              </div>
            </li>
          </ul>
        </div>

        <div class="grid grid-cols-1 gap-2 lg:grid-cols-2">
          <div
            v-if="nodeRuns[node.node_id]?.hard_gate_report_path"
            class="rounded-lg border border-surface-800 bg-surface-950/30 p-3"
          >
            <div class="text-xs font-semibold text-surface-400">Hard Gate</div>
            <div class="mt-1 truncate font-mono text-xs text-surface-200">
              {{ nodeRuns[node.node_id]?.hard_gate_report_path }}
            </div>
          </div>
          <div
            v-if="nodeRuns[node.node_id]?.soft_gate_report_path"
            class="rounded-lg border border-surface-800 bg-surface-950/30 p-3"
          >
            <div class="text-xs font-semibold text-surface-400">Soft Gate</div>
            <div class="mt-1 truncate font-mono text-xs text-surface-200">
              {{ nodeRuns[node.node_id]?.soft_gate_report_path }}
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-surface-800 bg-surface-950/30 p-3">
          <div class="text-xs font-semibold text-surface-400">Events / Logs</div>
          <div class="mt-1 text-xs text-surface-500">
            （MVP）暂未提供 node 级事件流接口；后续会补充 waterfall 视图。
          </div>
        </div>
      </div>
    </div>

    <div v-if="nodes.length === 0" class="text-sm text-surface-500">
      No nodes in this run.
    </div>
  </div>
</template>
