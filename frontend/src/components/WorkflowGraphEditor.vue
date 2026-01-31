<script setup lang="ts">
import { computed, ref } from "vue";
import { Plus, Trash2 } from "lucide-vue-next";

import type { WorkflowGraph, WorkflowGraphEdge, WorkflowGraphNode } from "@/api/client";

const props = defineProps<{
  modelValue: WorkflowGraph;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: WorkflowGraph): void;
}>();

const graph = computed<WorkflowGraph>(() => {
  const g = props.modelValue;
  return {
    nodes: Array.isArray(g?.nodes) ? g.nodes : [],
    edges: Array.isArray(g?.edges) ? g.edges : [],
  };
});

const updateGraph = (next: WorkflowGraph) => {
  emit("update:modelValue", {
    nodes: Array.isArray(next?.nodes) ? next.nodes : [],
    edges: Array.isArray(next?.edges) ? next.edges : [],
  });
};

const generateNodeId = () => {
  const existing = new Set(graph.value.nodes.map((n) => String(n.node_id || "")));
  for (let i = 0; i < 10; i++) {
    const id = `node-${Math.random().toString(36).slice(2, 8)}`;
    if (!existing.has(id)) return id;
  }
  return `node-${Date.now()}`;
};

const addNode = () => {
  const node: WorkflowGraphNode = { node_id: generateNodeId(), title: "", prompt: "" };
  updateGraph({ nodes: [...graph.value.nodes, node], edges: graph.value.edges });
};

const updateNode = (nodeId: string, patch: Partial<WorkflowGraphNode>) => {
  const nextNodes = graph.value.nodes.map((n) =>
    n.node_id === nodeId ? { ...n, ...patch } : n,
  );
  updateGraph({ nodes: nextNodes, edges: graph.value.edges });
};

const deleteNode = (nodeId: string) => {
  const nextNodes = graph.value.nodes.filter((n) => n.node_id !== nodeId);
  const nextEdges = graph.value.edges.filter((e) => e.from !== nodeId && e.to !== nodeId);
  updateGraph({ nodes: nextNodes, edges: nextEdges });
};

const edgeFrom = ref("");
const edgeTo = ref("");

const addEdge = () => {
  const from = String(edgeFrom.value || "").trim();
  const to = String(edgeTo.value || "").trim();
  if (!from || !to) return;
  const edge: WorkflowGraphEdge = { from, to };
  updateGraph({ nodes: graph.value.nodes, edges: [...graph.value.edges, edge] });
  edgeFrom.value = "";
  edgeTo.value = "";
};

const deleteEdge = (idx: number) => {
  const nextEdges = graph.value.edges.filter((_, i) => i !== idx);
  updateGraph({ nodes: graph.value.nodes, edges: nextEdges });
};
</script>

<template>
  <div class="space-y-6">
    <section class="rounded-xl border border-surface-800 bg-surface-900/40 p-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold text-surface-200">Nodes</h3>
        <button
          data-testid="workflow-add-node"
          class="inline-flex items-center gap-2 rounded-lg bg-primary-500/15 px-3 py-1.5 text-sm font-medium text-primary-300 hover:bg-primary-500/20"
          @click="addNode"
        >
          <Plus class="h-4 w-4" />
          Add Node
        </button>
      </div>

      <div class="mt-4 space-y-3">
        <div
          v-for="node in graph.nodes"
          :key="node.node_id"
          class="rounded-lg border border-surface-800 bg-surface-950/30 p-3"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="text-xs text-surface-500">node_id</div>
              <div class="truncate font-mono text-sm text-surface-200">
                {{ node.node_id }}
              </div>
            </div>
            <button
              class="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm text-red-400 hover:bg-red-500/10"
              @click="deleteNode(node.node_id)"
            >
              <Trash2 class="h-4 w-4" />
              Delete
            </button>
          </div>

          <div class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
            <label class="block">
              <div class="text-xs text-surface-500">title</div>
              <input
                class="mt-1 w-full rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200 placeholder:text-surface-600"
                :value="node.title || ''"
                placeholder="Optional title"
                @input="updateNode(node.node_id, { title: ($event.target as HTMLInputElement).value })"
              />
            </label>
            <label class="block">
              <div class="text-xs text-surface-500">prompt</div>
              <textarea
                class="mt-1 min-h-[72px] w-full rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200 placeholder:text-surface-600"
                :value="node.prompt || ''"
                placeholder="Optional prompt"
                @input="updateNode(node.node_id, { prompt: ($event.target as HTMLTextAreaElement).value })"
              />
            </label>
          </div>
        </div>

        <div v-if="graph.nodes.length === 0" class="text-sm text-surface-500">
          No nodes yet.
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-surface-800 bg-surface-900/40 p-4">
      <h3 class="text-sm font-semibold text-surface-200">Edges</h3>

      <div class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-3">
        <label class="block">
          <div class="text-xs text-surface-500">from</div>
          <select
            class="mt-1 w-full rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200"
            v-model="edgeFrom"
          >
            <option value="">Select node</option>
            <option v-for="n in graph.nodes" :key="n.node_id" :value="n.node_id">
              {{ n.node_id }}
            </option>
          </select>
        </label>
        <label class="block">
          <div class="text-xs text-surface-500">to</div>
          <select
            class="mt-1 w-full rounded-lg border border-surface-800 bg-surface-950/40 px-3 py-2 text-sm text-surface-200"
            v-model="edgeTo"
          >
            <option value="">Select node</option>
            <option v-for="n in graph.nodes" :key="n.node_id" :value="n.node_id">
              {{ n.node_id }}
            </option>
          </select>
        </label>
        <div class="flex items-end">
          <button
            class="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm font-medium text-surface-200 hover:bg-surface-700"
            @click="addEdge"
          >
            <Plus class="h-4 w-4" />
            Add Edge
          </button>
        </div>
      </div>

      <div class="mt-4 space-y-2">
        <div
          v-for="(edge, idx) in graph.edges"
          :key="idx"
          class="flex items-center justify-between gap-2 rounded-lg border border-surface-800 bg-surface-950/30 px-3 py-2"
        >
          <div class="min-w-0 truncate font-mono text-sm text-surface-200">
            {{ edge.from }} → {{ edge.to }}
          </div>
          <button
            class="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm text-red-400 hover:bg-red-500/10"
            @click="deleteEdge(idx)"
          >
            <Trash2 class="h-4 w-4" />
            Delete
          </button>
        </div>

        <div v-if="graph.edges.length === 0" class="text-sm text-surface-500">
          No edges yet.
        </div>
      </div>
    </section>
  </div>
</template>

