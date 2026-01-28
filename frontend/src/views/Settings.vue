<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Settings as SettingsIcon,
  Server,
  Plus,
  Trash2,
  Cpu,
  Search,
  FileText,
  ListChecks,
} from 'lucide-vue-next'
import {
  getProviders,
  createProvider,
  deleteProvider,
  createModel,
  deleteModel,
  updateModel,
  getBochaSettings,
  updateBochaSettings,
  bochaSearch,
  getTodayDigest,
  listSopSuggestions,
  generateSopSuggestions,
  updateSopSuggestionStatus,
  loadMoreSopSuggestions,
  type Suggestion,
  type SuggestionStatus,
} from '@/api/client'

interface LLMModel {
  id: string
  provider_id: string
  name: string
  model: string
  is_default: boolean
  enable_kv_cache: boolean
}

interface LLMProvider {
  id: string
  name: string
  provider_type: string
  base_url: string
  has_api_key: boolean
  models: LLMModel[]
}

interface ModelFormState {
  name: string
  model: string
  isDefault: boolean
  enableKVCache: boolean
}

const providerTypeOptions = [
  { label: 'OpenAI（Chat Completions）', value: 'openai' },
  { label: 'OpenAI（Responses）', value: 'openai_response' },
  { label: 'Claude（Anthropic）', value: 'claude' },
  { label: 'OpenRouter', value: 'openrouter' },
  { label: 'AWS Bedrock', value: 'bedrock' },
  { label: 'DeepSeek', value: 'deepseek' },
  { label: 'ZhipuAI', value: 'zhipuai' },
  { label: 'MiniMax', value: 'minimax' },
  { label: 'Antigravity', value: 'antigravity' },
  { label: 'Codex', value: 'codex' },
]

// Tab state
const activeTab = ref<'providers' | 'search' | 'digest' | 'sop'>('providers')

// LLM Providers state
const providers = ref<LLMProvider[]>([])
const providersLoading = ref(false)
const providerError = ref('')
const modelErrors = ref<Record<string, string>>({})

const providerForm = ref({
  name: '',
  provider_type: 'openai',
  base_url: '',
  api_key: '',
})
const modelForms = ref<Record<string, ModelFormState>>({})

// Search services state
const hasBochaAPIKey = ref(false)
const bochaAPIKey = ref('')
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const settingsSuccess = ref('')
const settingsError = ref('')

// Test search state
const testQuery = ref('今天天气怎么样')
const testLoading = ref(false)
const testResult = ref<string | null>(null)
const showTestDialog = ref(false)

// Digest state
const digestLoading = ref(false)
const digestError = ref('')
const digestMarkdown = ref('')
const digestDayKey = ref('')

const loadDigest = async (refresh: boolean = false) => {
  digestLoading.value = true
  digestError.value = ''
  try {
    const d: any = await getTodayDigest(refresh)
    digestMarkdown.value = String(d?.markdown || '')
    digestDayKey.value = String(d?.day_key || '')
  } catch (e: any) {
    digestError.value = e?.message || '加载日报失败'
    digestMarkdown.value = ''
    digestDayKey.value = ''
  } finally {
    digestLoading.value = false
  }
}

// SOP Suggestions state
const sopLoading = ref(false)
const sopError = ref('')
const sopIncludeParked = ref(false)
const sopItems = ref<Suggestion[]>([])

const loadSopSuggestionsList = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    const list = await listSopSuggestions({
      status: 'proposed',
      include_parked: sopIncludeParked.value,
      limit: 50,
    })
    sopItems.value = Array.isArray(list) ? list : []
  } catch (e: any) {
    sopError.value = e?.message || '加载 SOP 建议失败'
    sopItems.value = []
  } finally {
    sopLoading.value = false
  }
}

const generateOneSuggestion = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await generateSopSuggestions({ count: 1, lookback_days: 7 })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || '生成建议失败'
  } finally {
    sopLoading.value = false
  }
}

const loadMoreParkedSuggestions = async () => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await loadMoreSopSuggestions({ count: 3 })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || '加载更多建议失败'
  } finally {
    sopLoading.value = false
  }
}

const setSuggestionStatus = async (suggestionId: string, status: SuggestionStatus) => {
  sopLoading.value = true
  sopError.value = ''
  try {
    await updateSopSuggestionStatus(suggestionId, { status })
    await loadSopSuggestionsList()
  } catch (e: any) {
    sopError.value = e?.message || '更新状态失败'
  } finally {
    sopLoading.value = false
  }
}

const providerTypeLabel = (value: string) => {
  const match = providerTypeOptions.find((option) => option.value === value)
  return match ? match.label : value
}

const ensureModelForm = (providerId: string) => {
  if (!modelForms.value[providerId]) {
    modelForms.value[providerId] = {
      name: '',
      model: '',
      isDefault: false,
      enableKVCache: true,
    }
  }
}

const loadProviders = async () => {
  providersLoading.value = true
  try {
    const raw = await getProviders()
    const mapped = (Array.isArray(raw) ? raw : []).map((p: any) => ({
      id: String(p.id),
      name: String(p.name || ''),
      provider_type: String(p.provider_type || ''),
      base_url: String(p.base_url || ''),
      has_api_key: Boolean(p.has_api_key),
      models: (Array.isArray(p.models) ? p.models : []).map((m: any) => ({
        id: String(m.id),
        provider_id: String(m.provider_id),
        name: String(m.name || ''),
        model: String(m.model || ''),
        is_default: Boolean(m.is_default),
        enable_kv_cache: Boolean(m.enable_kv_cache),
      })),
    }))
    providers.value = mapped
    mapped.forEach((provider: LLMProvider) => ensureModelForm(provider.id))
  } catch (error) {
    console.error('Failed to load providers:', error)
    providers.value = []
  } finally {
    providersLoading.value = false
  }
}

const submitProvider = async () => {
  providerError.value = ''
  if (!providerForm.value.name || !providerForm.value.base_url || !providerForm.value.api_key) {
    providerError.value = '请填写 Provider 名称、Base URL 和 API Key。'
    return
  }

  try {
    await createProvider({
      name: providerForm.value.name.trim(),
      provider_type: providerForm.value.provider_type,
      base_url: providerForm.value.base_url.trim(),
      api_key: providerForm.value.api_key.trim(),
    })
    providerForm.value = {
      name: '',
      provider_type: providerForm.value.provider_type,
      base_url: '',
      api_key: '',
    }
    await loadProviders()
  } catch (error) {
    console.error('Failed to create provider:', error)
    providerError.value = '创建 Provider 失败。'
  }
}

const removeProvider = async (providerId: string) => {
  if (!confirm('确定删除此 Provider 及其所有模型吗？')) return
  try {
    await deleteProvider(providerId)
    await loadProviders()
  } catch (error) {
    console.error('Failed to delete provider:', error)
  }
}

const submitModel = async (providerId: string) => {
  const form = modelForms.value[providerId]
  if (!form) return
  modelErrors.value[providerId] = ''
  if (!form.name || !form.model) {
    modelErrors.value[providerId] = '请填写模型名称与模型标识符。'
    return
  }

  try {
    await createModel({
      provider_id: providerId,
      name: form.name.trim(),
      model: form.model.trim(),
      is_default: form.isDefault,
      enable_kv_cache: form.enableKVCache,
    })
    modelForms.value[providerId] = {
      name: '',
      model: '',
      isDefault: false,
      enableKVCache: true,
    }
    await loadProviders()
  } catch (error) {
    console.error('Failed to create model:', error)
    modelErrors.value[providerId] = '创建模型失败。'
  }
}

const removeModel = async (modelId: string) => {
  if (!confirm('确定删除此模型吗？')) return
  try {
    await deleteModel(modelId)
    await loadProviders()
  } catch (error) {
    console.error('Failed to delete model:', error)
  }
}

const setDefaultModel = async (modelId: string, providerId: string) => {
  try {
    await updateModel(modelId, { is_default: true })
    await loadProviders()
    ensureModelForm(providerId)
  } catch (error) {
    console.error('Failed to set default model:', error)
  }
}

// Settings functions
const loadSettings = async () => {
  settingsLoading.value = true
  try {
    const settings = await getBochaSettings()
    hasBochaAPIKey.value = settings.has_bocha_api_key
  } catch (error) {
    console.error('Failed to load settings:', error)
  } finally {
    settingsLoading.value = false
  }
}

const saveBochaAPIKey = async () => {
  settingsError.value = ''
  settingsSuccess.value = ''
  
  if (!bochaAPIKey.value.trim()) {
    settingsError.value = '请填写 API Key。'
    return
  }

  settingsSaving.value = true
  try {
    await updateBochaSettings({ bocha_api_key: bochaAPIKey.value.trim() })
    bochaAPIKey.value = ''
    hasBochaAPIKey.value = true
    settingsSuccess.value = 'API Key 已保存。'
    setTimeout(() => {
      settingsSuccess.value = ''
    }, 3000)
  } catch (error) {
    console.error('Failed to save settings:', error)
    settingsError.value = '保存 API Key 失败。'
  } finally {
    settingsSaving.value = false
  }
}

const runTestSearch = async () => {
  if (!testQuery.value.trim()) return
  
  testLoading.value = true
  testResult.value = null
  
  try {
    const result = await bochaSearch(testQuery.value.trim(), {
      summary: true,
      count: 3,
    })
    
    // Format result for display
    let message = `状态码：${result.code}\n`
    if (result.data?.webPages?.value?.length) {
      message += `\n找到 ${result.data.webPages.value.length} 条结果：\n`
      result.data.webPages.value.slice(0, 3).forEach((page: any, i: number) => {
        message += `\n${i + 1}. ${page.name}\n   ${page.url}\n   ${page.snippet?.substring(0, 100)}...`
      })
    } else {
      message += '\n未找到结果。'
    }
    
    testResult.value = message
    showTestDialog.value = true
    alert(message)
  } catch (error: any) {
    testResult.value = `错误：${error.message || '搜索失败'}`
    alert(testResult.value)
  } finally {
    testLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadProviders(), loadSettings()])
})
</script>

<template>
  <div class="min-h-screen bg-surface-950 p-6 lg:p-8">
    <div class="max-w-4xl mx-auto">
      <!-- Header -->
      <div class="mb-8">
        <div class="flex items-center gap-3 mb-2">
          <SettingsIcon class="w-8 h-8 text-primary-400" />
          <h1 class="text-2xl font-bold text-surface-100">设置</h1>
        </div>
        <p class="text-surface-400">管理账户与应用偏好</p>
      </div>

      <!-- Tab Navigation -->
      <div class="flex gap-1 mb-6 p-1 bg-surface-900/50 rounded-xl w-fit">
        <button
          @click="activeTab = 'providers'"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all',
            activeTab === 'providers'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/20'
              : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50'
          ]"
        >
          <Server class="w-4 h-4" />
          模型服务商
        </button>
        <button
          @click="activeTab = 'search'"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all',
            activeTab === 'search'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/20'
              : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50'
          ]"
        >
          <Search class="w-4 h-4" />
          搜索服务
        </button>
        <button
          @click="activeTab = 'digest'; if (!digestMarkdown) loadDigest(false)"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all',
            activeTab === 'digest'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/20'
              : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50'
          ]"
        >
          <FileText class="w-4 h-4" />
          日报
        </button>
        <button
          @click="activeTab = 'sop'; if (sopItems.length === 0) loadSopSuggestionsList()"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all',
            activeTab === 'sop'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/20'
              : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50'
          ]"
        >
          <ListChecks class="w-4 h-4" />
          SOP
        </button>
      </div>

      <!-- Tab Content -->
      <div class="space-y-6">
        <!-- LLM Providers Tab -->
        <div v-show="activeTab === 'providers'" class="glass rounded-2xl overflow-hidden">
          <div class="px-6 py-4 border-b border-surface-700/50">
            <div class="flex items-center gap-3">
              <Server class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">模型服务商</h2>
                <p class="text-sm text-surface-500">配置 API Key 并添加模型</p>
              </div>
            </div>
          </div>

          <div class="p-6 space-y-6">
            <form
              class="grid gap-4 md:grid-cols-2"
              @submit.prevent="submitProvider"
            >
              <div class="space-y-2">
                <label class="text-xs uppercase tracking-wide text-surface-500">Provider 名称</label>
                <input
                  v-model="providerForm.name"
                  type="text"
                  placeholder="My OpenAI"
                  class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                />
              </div>
              <div class="space-y-2">
                <label class="text-xs uppercase tracking-wide text-surface-500">Provider 类型</label>
                <select
                  v-model="providerForm.provider_type"
                  class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                >
                  <option v-for="option in providerTypeOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
              </div>
              <div class="space-y-2 md:col-span-2">
                <label class="text-xs uppercase tracking-wide text-surface-500">Base URL</label>
                <input
                  v-model="providerForm.base_url"
                  type="text"
                  placeholder="https://api.openai.com/v1"
                  class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                />
              </div>
              <div class="space-y-2 md:col-span-2">
                <label class="text-xs uppercase tracking-wide text-surface-500">API Key</label>
                <input
                  v-model="providerForm.api_key"
                  type="password"
                  placeholder="sk-..."
                  class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                />
              </div>
              <div class="md:col-span-2 flex items-center justify-between">
                <p v-if="providerError" class="text-sm text-red-400">{{ providerError }}</p>
                <button
                  type="submit"
                  class="ml-auto inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary-600 text-white text-sm hover:bg-primary-500 transition-colors"
                >
                  <Plus class="w-4 h-4" />
                  添加 Provider
                </button>
              </div>
            </form>

            <div class="space-y-4">
              <div v-if="providersLoading" class="text-sm text-surface-500">正在加载 Providers...</div>
              <div v-else-if="providers.length === 0" class="text-sm text-surface-500">
                暂无 Provider。先添加一个开始使用。
              </div>
              <div v-else class="space-y-4">
                <div
                  v-for="provider in providers"
                  :key="provider.id"
                  class="glass-card p-4 space-y-4"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <p class="text-base font-semibold text-surface-100">{{ provider.name }}</p>
                      <p class="text-xs text-surface-500 mt-1">{{ providerTypeLabel(provider.provider_type) }}</p>
                      <p class="text-xs text-surface-500">{{ provider.base_url }}</p>
                    </div>
                    <button
                      type="button"
                      class="p-2 rounded-lg bg-surface-900/70 text-surface-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
                      @click="removeProvider(provider.id)"
                    >
                      <Trash2 class="w-4 h-4" />
                    </button>
                  </div>

                  <div class="flex items-center gap-2 text-xs text-surface-500">
                    <span class="px-2 py-0.5 rounded-full bg-surface-800/70">
                      {{ provider.has_api_key ? '已保存 API Key' : '未设置 API Key' }}
                    </span>
                  </div>

                  <div>
                    <div class="flex items-center gap-2 text-xs uppercase tracking-wide text-surface-500">
                      <Cpu class="w-4 h-4" />
                      模型
                    </div>
                    <div v-if="provider.models.length === 0" class="mt-2 text-sm text-surface-500">
                      暂无模型。
                    </div>
                    <div v-else class="mt-2 space-y-2">
                      <div
                        v-for="model in provider.models"
                        :key="model.id"
                        class="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-surface-900/60 border border-surface-800 px-3 py-2"
                      >
                        <div class="min-w-0">
                          <p class="text-sm text-surface-200 truncate">{{ model.name }}</p>
                          <p class="text-xs text-surface-500 truncate">{{ model.model }}</p>
                        </div>
                        <div class="flex items-center gap-2 flex-wrap">
                          <span
                            v-if="model.is_default"
                            class="px-2 py-0.5 rounded-full bg-primary-500/20 text-primary-300 text-[10px] uppercase tracking-wide"
                          >
                            默认
                          </span>
                          <span
                            v-if="model.enable_kv_cache"
                            class="px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 text-[10px] uppercase tracking-wide"
                          >
                            KV Cache
                          </span>
                          <button
                            v-if="!model.is_default"
                            type="button"
                            class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                            @click="setDefaultModel(model.id, provider.id)"
                          >
                            设为默认
                          </button>
                          <button
                            type="button"
                            class="p-1.5 rounded-md bg-surface-800 text-surface-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
                            @click="removeModel(model.id)"
                          >
                            <Trash2 class="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>

                  <form
                    v-if="modelForms[provider.id]"
                    class="grid gap-3 md:grid-cols-2"
                    @submit.prevent="submitModel(provider.id)"
                  >
                    <div class="space-y-1">
                      <label class="text-xs text-surface-500">模型名称</label>
                      <input
                        v-model="modelForms[provider.id].name"
                        type="text"
                        placeholder="ChatGPT Turbo"
                        class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                      />
                    </div>
                    <div class="space-y-1">
                      <label class="text-xs text-surface-500">模型标识符</label>
                      <input
                        v-model="modelForms[provider.id].model"
                        type="text"
                        placeholder="gpt-4o-mini"
                        class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                      />
                    </div>
                    <div class="flex items-center gap-4 text-xs text-surface-500 md:col-span-2">
                      <label class="flex items-center gap-2">
                        <input
                          v-model="modelForms[provider.id].isDefault"
                          type="checkbox"
                          class="accent-primary-500"
                        />
                        默认模型
                      </label>
                      <label class="flex items-center gap-2">
                        <input
                          v-model="modelForms[provider.id].enableKVCache"
                          type="checkbox"
                          class="accent-emerald-400"
                        />
                        启用 KV cache
                      </label>
                    </div>
                    <p class="md:col-span-2 text-xs text-surface-500">
                      KV cache 与 Provider 有关：OpenAI-compatible 使用 session cache key；Claude 使用 prompt-caching 标记；
                      OpenRouter/Bedrock 使用 cache markers。
                    </p>
                    <div class="md:col-span-2 flex items-center justify-between">
                      <p v-if="modelErrors[provider.id]" class="text-xs text-red-400">
                        {{ modelErrors[provider.id] }}
                      </p>
                      <button
                        type="submit"
                        class="ml-auto inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface-800 text-surface-200 text-xs uppercase tracking-wide hover:bg-surface-700 transition-colors"
                      >
                        <Plus class="w-3.5 h-3.5" />
                        添加模型
                      </button>
                    </div>
                  </form>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Search Services Tab -->
        <div v-show="activeTab === 'search'" class="glass rounded-2xl overflow-hidden">
          <div class="px-6 py-4 border-b border-surface-700/50">
            <div class="flex items-center gap-3">
              <Search class="w-5 h-5 text-primary-400" />
              <div>
                <h2 class="font-semibold text-surface-100">搜索服务</h2>
                <p class="text-sm text-surface-500">配置搜索服务的 API Key</p>
              </div>
            </div>
          </div>

          <div class="p-6 space-y-6">
            <!-- Bocha Search -->
            <div class="glass-card p-4 space-y-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-base font-semibold text-surface-100">博查搜索 (Bocha Search)</p>
                  <p class="text-xs text-surface-500 mt-1">
                    从近百亿网页和生态内容源中搜索高质量世界知识
                  </p>
                  <a
                    href="https://open.bochaai.com"
                    target="_blank"
                    class="text-xs text-primary-400 hover:text-primary-300 mt-1 inline-block"
                  >
                    获取 API Key →
                  </a>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    :class="[
                      'px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wide',
                      hasBochaAPIKey
                        ? 'bg-emerald-500/15 text-emerald-300'
                        : 'bg-surface-800/70 text-surface-500'
                    ]"
                  >
                    {{ hasBochaAPIKey ? '已配置' : '未配置' }}
                  </span>
                </div>
              </div>

              <form @submit.prevent="saveBochaAPIKey" class="space-y-4">
                <div class="space-y-2">
                  <label class="text-xs uppercase tracking-wide text-surface-500">
                    {{ hasBochaAPIKey ? '更新 API Key' : 'API Key' }}
                  </label>
                  <input
                    v-model="bochaAPIKey"
                    type="password"
                    :placeholder="hasBochaAPIKey ? '输入新的 API Key 以更新...' : '输入你的 Bocha API Key...'"
                    class="w-full rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                  />
                </div>

                <div class="flex items-center justify-between">
                  <div>
                    <p v-if="settingsError" class="text-sm text-red-400">{{ settingsError }}</p>
                    <p v-if="settingsSuccess" class="text-sm text-emerald-400">{{ settingsSuccess }}</p>
                  </div>
                  <button
                    type="submit"
                    :disabled="settingsSaving || !bochaAPIKey.trim()"
                    class="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary-600 text-white text-sm hover:bg-primary-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {{ settingsSaving ? '保存中...' : '保存 API Key' }}
                  </button>
                </div>
              </form>

              <!-- Test Search -->
              <div v-if="hasBochaAPIKey" class="border-t border-surface-700/50 pt-4 mt-4">
                <p class="text-xs uppercase tracking-wide text-surface-500 mb-2">测试搜索</p>
                <div class="flex gap-2">
                  <input
                    v-model="testQuery"
                    type="text"
                    placeholder="输入测试查询..."
                    class="flex-1 rounded-lg bg-surface-900/70 border border-surface-700 px-3 py-2 text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500/40"
                  />
                  <button
                    type="button"
                    :disabled="testLoading || !testQuery.trim()"
                    class="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-800 text-surface-200 text-sm hover:bg-surface-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    @click="runTestSearch"
                  >
                    {{ testLoading ? '测试中...' : '测试' }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Placeholder for future search services -->
            <div class="text-center py-8 text-surface-500 text-sm">
              更多搜索服务即将支持...
            </div>
          </div>
        </div>

        <!-- Digest Tab -->
        <div v-show="activeTab === 'digest'" class="glass rounded-2xl overflow-hidden">
          <div class="px-6 py-4 border-b border-surface-700/50">
            <div class="flex items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <FileText class="w-5 h-5 text-primary-400" />
                <div>
                  <h2 class="font-semibold text-surface-100">今日日报</h2>
                  <p class="text-sm text-surface-500">今日回执的快速摘要</p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span v-if="digestDayKey" class="text-xs text-surface-500">{{ digestDayKey }}</span>
                <button
                  class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                  :disabled="digestLoading"
                  @click="loadDigest(true)"
                >
                  {{ digestLoading ? '加载中…' : '刷新' }}
                </button>
              </div>
            </div>
          </div>

          <div class="p-6 space-y-3">
            <div v-if="digestLoading" class="text-sm text-surface-500">加载中…</div>
            <div v-else-if="digestError" class="text-sm text-red-400">{{ digestError }}</div>
            <div v-else class="prose prose-invert max-w-none">
              <pre class="whitespace-pre-wrap text-sm bg-surface-900/60 border border-surface-700/50 rounded-xl p-4">{{ digestMarkdown || '（空）' }}</pre>
            </div>
          </div>
        </div>

        <!-- SOP Suggestions Tab -->
        <div v-show="activeTab === 'sop'" class="glass rounded-2xl overflow-hidden">
          <div class="px-6 py-4 border-b border-surface-700/50">
            <div class="flex items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <ListChecks class="w-5 h-5 text-primary-400" />
                <div>
                  <h2 class="font-semibold text-surface-100">SOP 建议</h2>
                  <p class="text-sm text-surface-500">待审核的 SOP 建议（需人工确认）</p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                  :disabled="sopLoading"
                  @click="generateOneSuggestion"
                >
                  {{ sopLoading ? '处理中…' : '生成' }}
                </button>
                <button
                  class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-800/70 text-surface-100 hover:bg-surface-700/70"
                  :disabled="sopLoading"
                  @click="loadMoreParkedSuggestions"
                >
                  加载更多
                </button>
              </div>
            </div>
            <div class="mt-3 flex items-center justify-between gap-3">
              <label class="flex items-center gap-2 text-xs text-surface-400">
                <input v-model="sopIncludeParked" type="checkbox" class="accent-primary-500" @change="loadSopSuggestionsList" />
                包含搁置
              </label>
              <button
                class="px-3 py-2 rounded-lg text-xs font-medium bg-surface-900/60 border border-surface-700/50 text-surface-200 hover:bg-surface-800/60"
                :disabled="sopLoading"
                @click="loadSopSuggestionsList"
              >
                刷新
              </button>
            </div>
          </div>

          <div class="p-6 space-y-4">
            <div v-if="sopLoading" class="text-sm text-surface-500">加载中…</div>
            <div v-else-if="sopError" class="text-sm text-red-400">{{ sopError }}</div>
            <div v-else-if="sopItems.length === 0" class="text-sm text-surface-500">
              暂无建议。点击“生成”从近期回执中提取一条。
            </div>
            <div v-else class="space-y-4">
              <div v-for="s in sopItems" :key="s.suggestion_id" class="glass-card p-4 space-y-3">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="text-sm font-semibold text-surface-100 truncate">{{ s.title }}</p>
                    <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-surface-500">
                      <span class="px-2 py-0.5 rounded-full bg-surface-800/70">{{ s.status }}</span>
                      <span class="px-2 py-0.5 rounded-full bg-surface-800/70">证据：{{ s.evidence_count }}</span>
                      <span v-if="s.scores" class="px-2 py-0.5 rounded-full bg-surface-800/70">得分：{{ s.scores.total_score.toFixed(2) }}</span>
                    </div>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      v-if="s.status !== 'approved'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-emerald-500/15 text-emerald-200 hover:bg-emerald-500/25"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'approved')"
                  >
                    通过
                  </button>
                    <button
                      v-if="s.status !== 'rejected'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-red-500/10 text-red-200 hover:bg-red-500/20"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'rejected')"
                  >
                    拒绝
                  </button>
                    <button
                      v-if="s.status === 'proposed'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'parked')"
                  >
                    搁置
                  </button>
                    <button
                      v-if="s.status !== 'archived'"
                      class="px-2 py-1 rounded-md text-[10px] uppercase tracking-wide bg-surface-800 text-surface-300 hover:bg-surface-700"
                    :disabled="sopLoading"
                    @click="setSuggestionStatus(s.suggestion_id, 'archived')"
                  >
                    归档
                  </button>
                  </div>
                </div>

                <details class="rounded-lg bg-surface-900/60 border border-surface-700/50">
                  <summary class="cursor-pointer select-none px-3 py-2 text-xs text-surface-300">草稿</summary>
                  <pre class="whitespace-pre-wrap text-xs text-surface-100 px-3 pb-3">{{ s.draft_skill }}</pre>
                </details>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer note -->
      <p class="mt-8 text-center text-sm text-surface-500">
        当前实例使用本地访问令牌保护（AUTH_MODE=token）。请妥善保管。
      </p>
    </div>
  </div>
</template>
