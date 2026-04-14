<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  KeyRound,
  RefreshCw,
  Save,
  Shield,
  Trash2,
  ChevronDown,
  ChevronRight,
} from "lucide-vue-next";

import ErrorBanner from "@/components/ErrorBanner.vue";
import {
  createAuthToken,
  getToolPolicy,
  getTools,
  listAuthTokens,
  revokeAuthToken,
  setToolPolicy,
  type AuthTokenResponse,
  type ToolPolicyResponse,
  type ToolPolicy,
} from "@/api/client";
import { useSimpleToolPermissions } from "@/composables/useSimpleToolPermissions";
import { parseApiError, type ParsedApiError } from "@/lib/apiError";
import { simpleToolPermissionLabel } from "@/lib/toolPermissions";

type ToolInfo = {
  id: string;
  name: string;
  description?: string;
};

const adminLoading = ref(false);
const adminError = ref<ParsedApiError | null>(null);
const tools = ref<ToolInfo[]>([]);
const tokens = ref<AuthTokenResponse[]>([]);
const principalID = ref("local");
const policyResponse = ref<ToolPolicyResponse | null>(null);
const policyJSON = ref("");
const policyDirty = ref(false);
const newTokenPrincipal = ref("");
const createdToken = ref<AuthTokenResponse | null>(null);
const showAdvanced = ref(false);

const {
  state: simpleState,
  loading: simpleLoading,
  saving: simpleSaving,
  error: simpleError,
  successMessage: simpleSuccessMessage,
  clearSuccessMessage,
  refresh: refreshSimple,
  applyMode,
  updateApprovalMode,
} = useSimpleToolPermissions();

const pageError = computed(() => simpleError.value || adminError.value);
const busy = computed(
  () => adminLoading.value || simpleLoading.value || simpleSaving.value,
);
const snapshot = computed(() => simpleState.value?.snapshot || policyResponse.value?.snapshot);
const canManageAdvanced = computed(
  () => simpleState.value?.advanced_settings_available === true,
);
const currentModeLabel = computed(() =>
  simpleToolPermissionLabel(simpleState.value?.current_mode),
);

const shortHash = (hash?: string) =>
  hash && hash.length >= 8 ? hash.slice(0, 8) : hash || "";

const syncPrincipalFromSimple = () => {
  const principal = String(simpleState.value?.principal_id || "").trim();
  if (principal) principalID.value = principal;
};

const refreshAdmin = async () => {
  if (!canManageAdvanced.value) {
    tools.value = [];
    tokens.value = [];
    policyResponse.value = null;
    policyJSON.value = "";
    policyDirty.value = false;
    return;
  }

  adminError.value = null;
  adminLoading.value = true;
  createdToken.value = null;
  try {
    const [rawTools, rawTokens] = await Promise.all([getTools(), listAuthTokens()]);
    tools.value = (Array.isArray(rawTools) ? rawTools : []).map((t: any) => ({
      id: String(t?.id || ""),
      name: String(t?.name || t?.id || ""),
      description: t?.description ? String(t.description) : undefined,
    }));
    tokens.value = Array.isArray(rawTokens) ? rawTokens : [];
  } catch (e: any) {
    adminError.value = parseApiError(e, "加载高级工具权限失败");
    tools.value = [];
    tokens.value = [];
  } finally {
    adminLoading.value = false;
  }
};

const loadPolicy = async () => {
  if (!canManageAdvanced.value) return;

  adminError.value = null;
  const id = String(principalID.value || "").trim();
  if (!id) return;
  adminLoading.value = true;
  try {
    const res = await getToolPolicy(id);
    policyResponse.value = res;
    const src = res.exists ? res.policy : res.snapshot?.policy;
    policyJSON.value = JSON.stringify(src || {}, null, 2);
    policyDirty.value = false;
  } catch (e: any) {
    adminError.value = parseApiError(e, "加载高级策略失败");
  } finally {
    adminLoading.value = false;
  }
};

const savePolicy = async () => {
  adminError.value = null;
  const id = String(principalID.value || "").trim();
  if (!id) return;
  let parsed: ToolPolicy;
  try {
    parsed = JSON.parse(policyJSON.value || "{}");
  } catch (e: any) {
    adminError.value = { message: `Invalid policy JSON: ${String(e?.message || e)}` };
    return;
  }

  adminLoading.value = true;
  try {
    const res = await setToolPolicy(id, parsed);
    policyResponse.value = res;
    policyJSON.value = JSON.stringify(res.policy || {}, null, 2);
    policyDirty.value = false;
    await refreshSimple();
  } catch (e: any) {
    adminError.value = parseApiError(e, "保存高级策略失败");
  } finally {
    adminLoading.value = false;
  }
};

const formatPolicyJSON = () => {
  adminError.value = null;
  try {
    const parsed = JSON.parse(policyJSON.value || "{}");
    policyJSON.value = JSON.stringify(parsed || {}, null, 2);
    policyDirty.value = true;
  } catch (e: any) {
    adminError.value = { message: `Invalid policy JSON: ${String(e?.message || e)}` };
  }
};

const createToken = async () => {
  adminError.value = null;
  const p =
    String(newTokenPrincipal.value || "").trim() ||
    String(principalID.value || "").trim();
  if (!p) return;
  adminLoading.value = true;
  try {
    createdToken.value = await createAuthToken(p);
    await refreshAdmin();
  } catch (e: any) {
    adminError.value = parseApiError(e, "创建 token 失败");
  } finally {
    adminLoading.value = false;
  }
};

const revokeToken = async (token: string) => {
  adminError.value = null;
  const t = String(token || "").trim();
  if (!t) return;
  adminLoading.value = true;
  try {
    await revokeAuthToken(t);
    await refreshAdmin();
  } catch (e: any) {
    adminError.value = parseApiError(e, "撤销 token 失败");
  } finally {
    adminLoading.value = false;
  }
};

const refreshPage = async () => {
  clearSuccessMessage();
  await refreshSimple();
  syncPrincipalFromSimple();
  await refreshAdmin();
  await loadPolicy();
};

const onApplyMode = async (mode: "readonly" | "sandbox_coding" | "host_full") => {
  if (mode === "host_full") {
    const ok = window.confirm(
      "“本机执行”会直接在当前机器上运行命令，风险最高。要继续吗？",
    );
    if (!ok) return;
  }
  await applyMode(mode, "tool_permissions_page");
  syncPrincipalFromSimple();
  if (canManageAdvanced.value) {
    await loadPolicy();
  }
};

const onUpdateApprovalMode = async (mode: "auto" | "manual") => {
  await updateApprovalMode(mode, "tool_permissions_page");
};

onMounted(async () => {
  await refreshPage();
});
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">工具权限</h1>
          <p class="text-sm text-surface-500">
            常见场景直接切预设；只有需要自定义策略时，才展开高级设置。
          </p>
        </div>
        <button
          data-testid="tool-permissions-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="busy"
          @click="refreshPage"
        >
          <RefreshCw class="w-4 h-4" />
          刷新
        </button>
      </div>

      <ErrorBanner v-if="pageError" :error="pageError" title="操作失败" />

      <div class="glass rounded-2xl overflow-hidden">
        <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
          <Shield class="w-5 h-5 text-primary-400" />
          <div class="min-w-0">
            <p class="text-sm font-semibold text-surface-100">简单预设</p>
            <p class="text-xs text-surface-500 truncate">
              当前账户：{{ simpleState?.principal_id || "加载中…" }} · 执行权限：{{ currentModeLabel }}
            </p>
          </div>
        </div>

        <div class="p-5 space-y-4">
          <div
            v-if="simpleSuccessMessage"
            class="rounded-xl border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100"
          >
            {{ simpleSuccessMessage }}
          </div>

          <div v-if="snapshot" class="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
            <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
              <div class="text-surface-500">当前模式</div>
              <div class="text-surface-200 mt-1">{{ currentModeLabel }}</div>
            </div>
            <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
              <div class="text-surface-500">高风险命令审批</div>
              <div class="text-surface-200 mt-1">
                {{ simpleState?.command_approval_mode === "manual" ? "手动审批" : "自动审批" }}
              </div>
            </div>
            <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
              <div class="text-surface-500">策略快照</div>
              <div class="text-surface-200 mt-1 font-mono">
                {{ snapshot.policy?.id || "default" }} · {{ shortHash(snapshot.policy_hash) }}
              </div>
            </div>
          </div>

          <div v-if="simpleLoading" class="text-sm text-surface-500">加载中…</div>
          <div v-else class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <button
              v-for="option in simpleState?.available_modes || []"
              :key="option.mode"
              type="button"
              class="rounded-2xl border p-4 text-left transition-colors"
              :class="
                option.current
                  ? 'border-primary-500/50 bg-primary-500/10'
                  : 'border-surface-700/40 bg-surface-950/40 hover:bg-surface-900/50'
              "
              :disabled="busy || !option.available"
              @click="onApplyMode(option.mode)"
            >
              <div class="flex items-center justify-between gap-3">
                <div class="text-sm font-semibold text-surface-100">
                  {{ option.label }}
                </div>
                <span
                  v-if="option.current"
                  class="px-2 py-0.5 rounded-full bg-primary-500/20 text-primary-200 text-[10px] uppercase tracking-wide"
                >
                  当前
                </span>
                <span
                  v-else-if="option.recommended"
                  class="px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-200 text-[10px] uppercase tracking-wide"
                >
                  推荐
                </span>
              </div>
              <p class="mt-2 text-sm text-surface-400">
                {{ option.description }}
              </p>
              <p v-if="!option.available && option.unavailable_reason" class="mt-3 text-xs text-red-400">
                {{ option.unavailable_reason }}
              </p>
            </button>
          </div>

          <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4 space-y-3">
            <div class="flex items-center justify-between gap-4">
              <div>
                <div class="text-sm font-medium text-surface-100">高风险命令审批</div>
                <div class="text-xs text-surface-500">
                  自动审批更流畅；手动审批更稳，会在高风险命令前要求你确认。
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="px-3 py-1.5 rounded-lg text-xs border"
                  :class="
                    simpleState?.command_approval_mode === 'auto'
                      ? 'border-primary-500/50 bg-primary-500/10 text-primary-200'
                      : 'border-surface-700/50 bg-surface-900/60 text-surface-300'
                  "
                  :disabled="busy"
                  @click="onUpdateApprovalMode('auto')"
                >
                  自动
                </button>
                <button
                  type="button"
                  class="px-3 py-1.5 rounded-lg text-xs border"
                  :class="
                    simpleState?.command_approval_mode === 'manual'
                      ? 'border-primary-500/50 bg-primary-500/10 text-primary-200'
                      : 'border-surface-700/50 bg-surface-900/60 text-surface-300'
                  "
                  :disabled="busy"
                  @click="onUpdateApprovalMode('manual')"
                >
                  手动
                </button>
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4 text-sm text-surface-400">
            {{ simpleState?.effective_scope?.summary }}
          </div>
        </div>
      </div>

      <div class="glass rounded-2xl overflow-hidden">
        <button
          type="button"
          class="w-full px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-3 text-left"
          @click="showAdvanced = !showAdvanced"
        >
          <div class="flex items-center gap-3 min-w-0">
            <Save class="w-5 h-5 text-primary-400" />
            <div class="min-w-0">
              <p class="text-sm font-semibold text-surface-100">高级 / 自定义策略</p>
              <p class="text-xs text-surface-500 truncate">
                {{ canManageAdvanced ? "仅本机管理员可用，用于 JSON 自定义和跨 principal 治理。" : "当前账户不能使用管理员级高级设置。" }}
              </p>
            </div>
          </div>
          <component :is="showAdvanced ? ChevronDown : ChevronRight" class="w-4 h-4 text-surface-500" />
        </button>

        <div v-if="showAdvanced" class="p-5 space-y-4">
          <div
            v-if="!canManageAdvanced"
            class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-4 text-sm text-surface-400"
          >
            当前账户不是 `local`，所以这里只保留简单预设；不会再把你送进一个必然失败的管理员页面。
          </div>

          <template v-else>
            <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <div class="lg:col-span-2 space-y-4">
                <div class="glass rounded-2xl overflow-hidden">
                  <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
                    <Shield class="w-5 h-5 text-primary-400" />
                    <div class="min-w-0">
                      <p class="text-sm font-semibold text-surface-100">生效策略快照</p>
                      <p v-if="snapshot" class="text-xs text-surface-500 truncate">
                        principal={{ snapshot.principal_id }} · policy={{ snapshot.policy?.id }} · hash={{ shortHash(snapshot.policy_hash) }}
                      </p>
                      <p v-else class="text-xs text-surface-500">加载中…</p>
                    </div>
                  </div>

                  <div class="p-5 space-y-3">
                    <div class="flex items-center gap-3">
                      <label class="text-xs text-surface-500 w-28">principal_id</label>
                      <input
                        v-model="principalID"
                        class="bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 w-full"
                        placeholder="local"
                        :disabled="busy"
                      />
                      <button
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                        :disabled="busy"
                        @click="loadPolicy"
                      >
                        <RefreshCw class="w-4 h-4" />
                        加载
                      </button>
                    </div>

                    <div v-if="snapshot" class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                      <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                        <div class="text-surface-500">default_effect</div>
                        <div class="text-surface-200 font-mono mt-1">{{ snapshot.policy?.default_effect || "allow" }}</div>
                      </div>
                      <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                        <div class="text-surface-500">default_command_profile</div>
                        <div class="text-surface-200 font-mono mt-1">{{ snapshot.policy?.default_command_profile || "dev" }}</div>
                      </div>
                      <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                        <div class="text-surface-500">resolved_at</div>
                        <div class="text-surface-200 font-mono mt-1">{{ snapshot.resolved_at }}</div>
                      </div>
                      <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                        <div class="text-surface-500">allowed_tools</div>
                        <div class="text-surface-200 font-mono mt-1">{{ tools.length }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="glass rounded-2xl overflow-hidden">
                  <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
                    <Shield class="w-5 h-5 text-primary-400" />
                    <div>
                      <p class="text-sm font-semibold text-surface-100">允许的工具</p>
                      <p class="text-xs text-surface-500">{{ tools.length }} 个工具</p>
                    </div>
                  </div>

                  <div class="p-5">
                    <div v-if="adminLoading && tools.length === 0" class="text-sm text-surface-500">加载中…</div>
                    <div v-else-if="tools.length === 0" class="text-sm text-surface-500">暂无可用工具。</div>
                    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      <div
                        v-for="t in tools"
                        :key="t.id"
                        class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3 min-w-0"
                      >
                        <div class="text-sm text-surface-100 font-semibold truncate">{{ t.name }}</div>
                        <div class="text-[11px] text-surface-500 truncate">id={{ t.id }}</div>
                        <div v-if="t.description" class="text-xs text-surface-400 mt-2 line-clamp-2">{{ t.description }}</div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class="space-y-4">
                <div class="glass rounded-2xl overflow-hidden">
                  <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
                    <Save class="w-5 h-5 text-primary-400" />
                    <div class="min-w-0">
                      <p class="text-sm font-semibold text-surface-100">策略编辑器</p>
                      <p class="text-xs text-surface-500 truncate">
                        {{ policyResponse?.exists ? "已存储的策略" : "当前展示的是生效（默认）策略" }}
                      </p>
                    </div>
                  </div>

                  <div class="p-5 space-y-3">
                    <textarea
                      v-model="policyJSON"
                      class="w-full min-h-[260px] bg-surface-950 text-surface-200 text-xs font-mono rounded-xl border border-surface-800/60 p-3 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                      placeholder="{ ... }"
                      :disabled="busy"
                      @input="policyDirty = true"
                    />
                    <div class="flex justify-end gap-2">
                      <button
                        type="button"
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                        :disabled="busy"
                        @click="formatPolicyJSON"
                      >
                        格式化
                      </button>
                      <button
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                        :disabled="busy || !policyDirty"
                        @click="loadPolicy"
                      >
                        <RefreshCw class="w-4 h-4" />
                        重置
                      </button>
                      <button
                        data-testid="tool-permissions-save"
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-primary-500/20 text-primary-200 hover:bg-primary-500/30 inline-flex items-center gap-2 disabled:opacity-50"
                        :disabled="busy || !policyDirty"
                        @click="savePolicy"
                      >
                        <Save class="w-4 h-4" />
                        保存
                      </button>
                    </div>
                  </div>
                </div>

                <div class="glass rounded-2xl overflow-hidden">
                  <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
                    <KeyRound class="w-5 h-5 text-primary-400" />
                    <div>
                      <p class="text-sm font-semibold text-surface-100">访问令牌</p>
                      <p class="text-xs text-surface-500">{{ tokens.length }} 个令牌</p>
                    </div>
                  </div>

                  <div class="p-5 space-y-3">
                    <div class="flex items-center gap-2">
                      <input
                        v-model="newTokenPrincipal"
                        class="bg-surface-900 text-surface-200 text-xs rounded-lg px-2 py-1.5 border border-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500/40 w-full"
                        placeholder="principal_id（默认：当前）"
                        :disabled="busy"
                      />
                      <button
                        data-testid="tool-permissions-create-token"
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                        :disabled="busy"
                        @click="createToken"
                      >
                        <KeyRound class="w-4 h-4" />
                        创建
                      </button>
                    </div>

                    <div
                      v-if="createdToken"
                      class="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3 text-xs text-emerald-100"
                    >
                      已创建 token（principal_id=<span class="font-mono">{{ createdToken.principal_id }}</span>）：
                      <div class="mt-2 font-mono break-all">{{ createdToken.token }}</div>
                    </div>

                    <div v-if="tokens.length === 0" class="text-sm text-surface-500">暂无令牌。</div>
                    <div v-else class="space-y-2">
                      <div
                        v-for="t in tokens"
                        :key="t.token"
                        class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3"
                      >
                        <div class="flex items-start justify-between gap-3">
                          <div class="min-w-0">
                            <div class="text-xs text-surface-200 font-mono break-all">{{ t.token }}</div>
                            <div class="text-[11px] text-surface-500 mt-1">
                              principal={{ t.principal_id }} · created={{ t.created_at }}
                              <span v-if="t.revoked_at"> · 撤销于 {{ t.revoked_at }}</span>
                            </div>
                          </div>
                          <button
                            v-if="!t.revoked_at"
                            class="px-3 py-2 rounded-xl text-xs font-medium bg-rose-500/10 text-rose-200 hover:bg-rose-500/15 inline-flex items-center gap-2"
                            :disabled="busy"
                            @click="revokeToken(t.token)"
                          >
                            <Trash2 class="w-4 h-4" />
                            撤销
                          </button>
                          <span v-else class="text-xs text-surface-600">已撤销</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.glass {
  background: rgba(var(--color-surface-900), 0.6);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(var(--color-surface-700), 0.4);
}
</style>
