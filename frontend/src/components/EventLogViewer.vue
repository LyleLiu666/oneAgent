<script setup lang="ts">
import { computed, ref } from "vue";

import type { TaskEvent } from "@/api/client";

const props = defineProps<{
  events: TaskEvent[];
  loading?: boolean;
  refreshing?: boolean;
  compact?: boolean;
}>();

const viewMode = ref<"pretty" | "raw">("pretty");
const attemptFilter = ref<string>("all");
const typeFilter = ref<string>("all");
const search = ref<string>("");
const expandedKeys = ref<Set<string>>(new Set());
const copyStatus = ref<"" | "copied" | "failed">("");

const normalizedEvents = computed<TaskEvent[]>(() => {
  const list = Array.isArray(props.events) ? props.events : [];
  return [...list].sort((a, b) => {
    const at = Date.parse(String(a?.ts || ""));
    const bt = Date.parse(String(b?.ts || ""));
    if (!Number.isNaN(at) && !Number.isNaN(bt)) return bt - at;
    if (!Number.isNaN(bt)) return 1;
    if (!Number.isNaN(at)) return -1;
    return String(b?.ts || "").localeCompare(String(a?.ts || ""));
  });
});

const attemptOptions = computed<string[]>(() => {
  const uniq = new Set<string>();
  for (const e of normalizedEvents.value) {
    const id = String(e?.attempt_id || "").trim();
    if (id) uniq.add(id);
  }
  return ["all", ...Array.from(uniq).sort()];
});

const typeOptions = computed<string[]>(() => {
  const uniq = new Set<string>();
  for (const e of normalizedEvents.value) {
    const t = String(e?.type || "").trim();
    if (t) uniq.add(t);
  }
  return ["all", ...Array.from(uniq).sort()];
});

const filteredEvents = computed<TaskEvent[]>(() => {
  const attempt = String(attemptFilter.value || "").trim();
  const type = String(typeFilter.value || "").trim();
  const q = String(search.value || "").trim().toLowerCase();

  return normalizedEvents.value.filter((e) => {
    if (attempt !== "all" && String(e?.attempt_id || "") !== attempt) {
      return false;
    }
    if (type !== "all" && String(e?.type || "") !== type) {
      return false;
    }
    if (!q) return true;
    const hay = [
      String(e?.type || ""),
      String(e?.message || ""),
      String(e?.attempt_id || ""),
      safeJSONStringify(e?.data),
    ]
      .join("\n")
      .toLowerCase();
    return hay.includes(q);
  });
});

const countsLabel = computed(() => {
  const total = normalizedEvents.value.length;
  const filtered = filteredEvents.value.length;
  return `${filtered} / ${total}`;
});

const toggleExpanded = (e: TaskEvent, idx: number) => {
  const key = makeEventKey(e, idx);
  const next = new Set(expandedKeys.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  expandedKeys.value = next;
};

const isExpanded = (e: TaskEvent, idx: number) => {
  return expandedKeys.value.has(makeEventKey(e, idx));
};

const formatTime = (ts: string) => {
  const raw = String(ts || "");
  if (raw.length >= 19) return raw.slice(11, 19);
  return raw;
};

const safeJSONStringify = (v: any) => {
  if (v == null) return "";
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
};

const prettyJSON = (v: any) => {
  if (v == null) return "";
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
};

const makeEventKey = (e: TaskEvent, idx: number) => {
  return `${e?.ts || ""}:${e?.type || ""}:${e?.attempt_id || ""}:${idx}`;
};

const copyFiltered = async () => {
  const text = filteredEvents.value.map((e) => safeJSONStringify(e)).join("\n");
  copyStatus.value = "";
  try {
    await navigator.clipboard.writeText(text);
    copyStatus.value = "copied";
  } catch {
    copyStatus.value = "failed";
  } finally {
    window.setTimeout(() => {
      if (copyStatus.value) copyStatus.value = "";
    }, 1200);
  }
};
</script>

<template>
  <div class="space-y-3" data-testid="event-log-viewer">
    <div
      v-if="!compact"
      class="flex flex-wrap items-center justify-between gap-2"
    >
      <div class="flex items-center gap-2">
        <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500">
          事件
        </div>
        <div
          class="text-xs text-surface-400 font-mono"
          data-testid="event-viewer-counts"
        >
          {{ countsLabel }}
        </div>
        <div
          v-if="refreshing"
          class="text-[11px] text-surface-500"
          data-testid="event-viewer-refreshing"
        >
          refreshing…
        </div>
      </div>

      <div class="flex items-center gap-2">
        <button
          type="button"
          class="text-[11px] px-2 py-1 rounded border border-surface-800 bg-surface-950/40 text-surface-200 hover:bg-surface-900/40"
          :class="viewMode === 'pretty' ? 'border-surface-700' : ''"
          data-testid="event-viewer-mode-pretty"
          @click="viewMode = 'pretty'"
        >
          Pretty
        </button>
        <button
          type="button"
          class="text-[11px] px-2 py-1 rounded border border-surface-800 bg-surface-950/40 text-surface-200 hover:bg-surface-900/40"
          :class="viewMode === 'raw' ? 'border-surface-700' : ''"
          data-testid="event-viewer-mode-raw"
          @click="viewMode = 'raw'"
        >
          Raw
        </button>
      </div>
    </div>

    <div v-if="!compact" class="grid grid-cols-1 lg:grid-cols-3 gap-2">
      <label class="space-y-1">
        <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500">
          attempt
        </div>
        <select
          v-model="attemptFilter"
          data-testid="event-viewer-filter-attempt"
          class="w-full bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
        >
          <option v-for="opt in attemptOptions" :key="opt" :value="opt">
            {{ opt }}
          </option>
        </select>
      </label>

      <label class="space-y-1">
        <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500">
          type
        </div>
        <select
          v-model="typeFilter"
          data-testid="event-viewer-filter-type"
          class="w-full bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
        >
          <option v-for="opt in typeOptions" :key="opt" :value="opt">
            {{ opt }}
          </option>
        </select>
      </label>

      <label class="space-y-1">
        <div class="text-[10px] uppercase tracking-[0.2em] text-surface-500">
          search
        </div>
        <input
          v-model="search"
          data-testid="event-viewer-search"
          class="w-full bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
          placeholder="type / message / data"
        />
      </label>
    </div>

    <div v-if="!compact" class="flex items-center justify-between gap-2">
      <div class="text-xs text-surface-500">
        <span v-if="copyStatus === 'copied'">已复制</span>
        <span v-else-if="copyStatus === 'failed'">复制失败</span>
      </div>
      <button
        type="button"
        data-testid="event-viewer-copy"
        class="text-xs text-surface-300 hover:text-surface-100"
        :disabled="filteredEvents.length === 0"
        @click="copyFiltered"
      >
        Copy filtered
      </button>
    </div>

    <div v-if="loading && normalizedEvents.length === 0" class="text-xs text-surface-500">
      加载中…
    </div>

    <div v-else-if="normalizedEvents.length === 0" class="text-xs text-surface-500">
      暂无事件。
    </div>

    <div v-else class="space-y-1" data-testid="event-viewer-list">
      <div
        v-for="(ev, idx) in filteredEvents"
        :key="makeEventKey(ev, idx)"
        class="rounded-xl border border-surface-800/60 bg-surface-950/40"
        data-testid="event-viewer-item"
      >
        <button
          type="button"
          class="w-full text-left px-3 py-2"
          data-testid="event-viewer-item-toggle"
          @click="toggleExpanded(ev, idx)"
        >
          <div v-if="viewMode === 'raw'" class="text-[11px] text-surface-300 font-mono break-words">
            {{ safeJSONStringify(ev) }}
          </div>
          <div v-else class="flex items-start gap-3">
            <span
              class="text-[11px] text-surface-500 font-mono flex-shrink-0 w-[52px] pt-0.5"
              :title="ev.ts"
            >{{ formatTime(ev.ts) }}</span>
            <span
              class="text-[11px] font-medium text-surface-300 px-2 py-1 rounded-lg bg-surface-700/50 flex-shrink-0"
            >{{ ev.type }}</span>
            <span
              class="text-[11px] text-surface-400 flex-1 break-words pt-0.5"
            >
              <span v-if="ev.message">{{ ev.message }}</span>
              <span v-else-if="ev.data">{{ safeJSONStringify(ev.data) }}</span>
            </span>
          </div>
        </button>

        <div
          v-if="isExpanded(ev, idx)"
          class="px-3 pb-3"
          data-testid="event-viewer-item-details"
        >
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div class="text-[11px] text-surface-500">
              <div class="text-[10px] uppercase tracking-[0.2em] text-surface-600">
                ts
              </div>
              <div class="font-mono break-all text-surface-300">
                {{ ev.ts }}
              </div>
            </div>
            <div class="text-[11px] text-surface-500">
              <div class="text-[10px] uppercase tracking-[0.2em] text-surface-600">
                attempt_id
              </div>
              <div class="font-mono break-all text-surface-300">
                {{ ev.attempt_id || "" }}
              </div>
            </div>
          </div>

          <div v-if="ev.data" class="mt-2">
            <div class="text-[10px] uppercase tracking-[0.2em] text-surface-600">
              data
            </div>
            <pre class="mt-2 max-h-60 overflow-auto rounded-xl bg-surface-900/50 p-3 text-[11px] text-surface-200 whitespace-pre-wrap">{{ prettyJSON(ev.data) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

