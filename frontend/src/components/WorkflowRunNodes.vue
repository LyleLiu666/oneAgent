<script setup lang="ts">
import { computed } from "vue";
import { Copy } from "lucide-vue-next";

import type { WorkflowRun } from "@/api/client";

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
              class="flex items-center justify-between gap-2"
            >
              <div class="min-w-0 truncate font-mono text-xs text-surface-200">
                {{ a.path }}
              </div>
              <button
                class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-surface-400 hover:bg-surface-800 hover:text-surface-200"
                @click="copyText(a.path)"
              >
                <Copy class="h-3.5 w-3.5" />
                Copy
              </button>
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

