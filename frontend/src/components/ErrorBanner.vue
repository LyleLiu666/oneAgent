<script setup lang="ts">
import { computed, ref } from "vue";
import { ClipboardCopy, ChevronDown, ChevronUp } from "lucide-vue-next";

import type { ParsedApiError } from "@/lib/apiError";

const props = defineProps<{
  error?: ParsedApiError | string | null;
  title?: string;
}>();

const expanded = ref(false);
const copied = ref(false);

const normalized = computed<ParsedApiError | null>(() => {
  const e = props.error;
  if (!e) return null;
  if (typeof e === "string") return { message: e };
  if (typeof e === "object" && typeof e.message === "string") return e;
  return null;
});

const hasDetails = computed(() => {
  const e = normalized.value;
  if (!e) return false;
  return Boolean(String(e.code || "").trim() || String(e.requestId || "").trim());
});

async function copyText(text: string): Promise<boolean> {
  const t = String(text || "").trim();
  if (!t) return false;

  try {
    await navigator.clipboard.writeText(t);
    return true;
  } catch {
    // Fallback for environments where clipboard API isn't available.
  }

  try {
    const el = document.createElement("textarea");
    el.value = t;
    el.setAttribute("readonly", "");
    el.style.position = "fixed";
    el.style.top = "-1000px";
    el.style.left = "-1000px";
    document.body.appendChild(el);
    el.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(el);
    return ok;
  } catch {
    return false;
  }
}

const onCopyRequestID = async () => {
  const id = String(normalized.value?.requestId || "").trim();
  if (!id) return;

  const ok = await copyText(id);
  if (!ok) return;

  copied.value = true;
  window.setTimeout(() => {
    copied.value = false;
  }, 1200);
};
</script>

<template>
  <div
    v-if="normalized"
    data-testid="error-banner"
    class="rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-red-100"
  >
    <div class="flex items-start justify-between gap-4">
      <div class="min-w-0">
        <div class="text-sm font-semibold text-red-200">
          {{ title || "出错了" }}
        </div>
        <div data-testid="error-banner-message" class="mt-1 text-sm break-words">
          {{ normalized.message }}
        </div>
        <div v-if="normalized.hint" data-testid="error-banner-hint" class="mt-1 text-xs text-red-200/90">
          {{ normalized.hint }}
        </div>
      </div>

      <button
        v-if="hasDetails"
        type="button"
        data-testid="error-banner-toggle"
        class="inline-flex shrink-0 items-center gap-1 rounded-lg border border-red-400/20 bg-red-500/10 px-2 py-1 text-xs text-red-200 hover:bg-red-500/15"
        @click="expanded = !expanded"
      >
        <span>{{ expanded ? "收起" : "详情" }}</span>
        <ChevronUp v-if="expanded" class="h-3.5 w-3.5" />
        <ChevronDown v-else class="h-3.5 w-3.5" />
      </button>
    </div>

    <div v-if="expanded && hasDetails" data-testid="error-banner-details" class="mt-3 rounded-lg bg-surface-950/50 p-3">
      <div v-if="normalized.code" class="text-xs">
        <span class="text-red-200/70">code</span>
        <span class="mx-1 text-red-200/40">•</span>
        <code class="rounded bg-surface-900/70 px-1.5 py-0.5 text-red-100">{{ normalized.code }}</code>
      </div>

      <div v-if="normalized.requestId" class="mt-2 flex flex-wrap items-center gap-2 text-xs">
        <span class="text-red-200/70">request_id</span>
        <span class="mx-1 text-red-200/40">•</span>
        <code
          data-testid="error-banner-request-id"
          class="rounded bg-surface-900/70 px-1.5 py-0.5 text-red-100"
          >{{ normalized.requestId }}</code
        >
        <button
          type="button"
          data-testid="error-banner-copy-request-id"
          class="inline-flex items-center gap-1 rounded-md border border-red-400/20 bg-red-500/10 px-2 py-1 text-xs text-red-200 hover:bg-red-500/15"
          @click="onCopyRequestID"
        >
          <ClipboardCopy class="h-3.5 w-3.5" />
          <span>{{ copied ? "已复制" : "复制" }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

