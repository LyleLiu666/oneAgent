<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { KeyRound, RefreshCw, Save, Shield, Trash2 } from 'lucide-vue-next'

import ErrorBanner from '@/components/ErrorBanner.vue'
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
} from '@/api/client'
import { parseApiError, type ParsedApiError } from '@/lib/apiError'

type ToolInfo = {
  id: string
  name: string
  description?: string
}

const loading = ref(false)
const error = ref<ParsedApiError | null>(null)

const tools = ref<ToolInfo[]>([])
const tokens = ref<AuthTokenResponse[]>([])

const principalID = ref('local')
const policyResponse = ref<ToolPolicyResponse | null>(null)
const policyJSON = ref('')
const policyDirty = ref(false)

const newTokenPrincipal = ref('')
const createdToken = ref<AuthTokenResponse | null>(null)

const snapshot = computed(() => policyResponse.value?.snapshot)

const shortHash = (hash?: string) => (hash && hash.length >= 8 ? hash.slice(0, 8) : hash || '')

const refresh = async () => {
  error.value = null
  loading.value = true
  createdToken.value = null
  try {
    const [rawTools, rawTokens] = await Promise.all([getTools(), listAuthTokens()])
    tools.value = (Array.isArray(rawTools) ? rawTools : []).map((t: any) => ({
      id: String(t?.id || ''),
      name: String(t?.name || t?.id || ''),
      description: t?.description ? String(t.description) : undefined,
    }))
    tokens.value = Array.isArray(rawTokens) ? rawTokens : []
  } catch (e: any) {
    error.value = parseApiError(e, '加载工具权限失败')
    tools.value = []
    tokens.value = []
  } finally {
    loading.value = false
  }
}

const loadPolicy = async () => {
  error.value = null
  const id = String(principalID.value || '').trim()
  if (!id) return
  loading.value = true
  try {
    const res = await getToolPolicy(id)
    policyResponse.value = res
    const src = res.exists ? res.policy : res.snapshot?.policy
    policyJSON.value = JSON.stringify(src || {}, null, 2)
    policyDirty.value = false
  } catch (e: any) {
    error.value = parseApiError(e, '加载策略失败')
  } finally {
    loading.value = false
  }
}

const savePolicy = async () => {
  error.value = null
  const id = String(principalID.value || '').trim()
  if (!id) return
  let parsed: ToolPolicy
  try {
    parsed = JSON.parse(policyJSON.value || '{}')
  } catch (e: any) {
    error.value = { message: `Invalid policy JSON: ${String(e?.message || e)}` }
    return
  }

  loading.value = true
  try {
    const res = await setToolPolicy(id, parsed)
    policyResponse.value = res
    policyJSON.value = JSON.stringify(res.policy || {}, null, 2)
    policyDirty.value = false
  } catch (e: any) {
    error.value = parseApiError(e, '保存策略失败')
  } finally {
    loading.value = false
  }
}

const createToken = async () => {
  error.value = null
  const p = String(newTokenPrincipal.value || '').trim() || String(principalID.value || '').trim()
  if (!p) return
  loading.value = true
  try {
    const res = await createAuthToken(p)
    createdToken.value = res
    await refresh()
  } catch (e: any) {
    error.value = parseApiError(e, '创建 token 失败')
  } finally {
    loading.value = false
  }
}

const revokeToken = async (token: string) => {
  error.value = null
  const t = String(token || '').trim()
  if (!t) return
  loading.value = true
  try {
    await revokeAuthToken(t)
    await refresh()
  } catch (e: any) {
    error.value = parseApiError(e, '撤销 token 失败')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await refresh()
  await loadPolicy()
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">工具权限</h1>
          <p class="text-sm text-surface-500">查看生效策略快照，并管理 token/策略（仅本机）。</p>
        </div>
        <button
          data-testid="tool-permissions-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="loading"
          @click="refresh(); loadPolicy()"
        >
          <RefreshCw class="w-4 h-4" />
          刷新
        </button>
      </div>

      <ErrorBanner v-if="error" :error="error" title="操作失败" />

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
                  :disabled="loading"
                />
                <button
                  class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                  :disabled="loading"
                  @click="loadPolicy"
                >
                  <RefreshCw class="w-4 h-4" />
                  加载
                </button>
              </div>

              <div v-if="snapshot" class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                  <div class="text-surface-500">default_effect</div>
                  <div class="text-surface-200 font-mono mt-1">{{ snapshot.policy?.default_effect || 'allow' }}</div>
                </div>
                <div class="rounded-xl border border-surface-700/40 bg-surface-950/40 p-3">
                  <div class="text-surface-500">default_command_profile</div>
                  <div class="text-surface-200 font-mono mt-1">{{ snapshot.policy?.default_command_profile || 'dev' }}</div>
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
              <div v-if="loading && tools.length === 0" class="text-sm text-surface-500">加载中…</div>
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
                  {{ policyResponse?.exists ? '已存储的策略' : '当前展示的是生效（默认）策略' }}
                </p>
              </div>
            </div>

            <div class="p-5 space-y-3">
              <textarea
                v-model="policyJSON"
                class="w-full min-h-[260px] bg-surface-950 text-surface-200 text-xs font-mono rounded-xl border border-surface-800/60 p-3 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                placeholder="{ ... }"
                :disabled="loading"
                @input="policyDirty = true"
              />
              <div class="flex justify-end gap-2">
                <button
                  class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                  :disabled="loading || !policyDirty"
                  @click="loadPolicy"
                >
                  <RefreshCw class="w-4 h-4" />
                  重置
                </button>
                <button
                  data-testid="tool-permissions-save"
                  class="px-3 py-2 rounded-xl text-xs font-medium bg-primary-500/20 text-primary-200 hover:bg-primary-500/30 inline-flex items-center gap-2 disabled:opacity-50"
                  :disabled="loading || !policyDirty"
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
                  :disabled="loading"
                />
                <button
                  data-testid="tool-permissions-create-token"
                  class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                  :disabled="loading"
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
                      :disabled="loading"
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
