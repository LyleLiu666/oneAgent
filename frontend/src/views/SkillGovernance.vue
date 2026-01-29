<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Archive, RefreshCw, Wrench, Save, Copy, Pin, Layers, ChevronDown, ChevronRight } from 'lucide-vue-next'

import {
  archiveShadowedPersonalDuplicates,
  archiveSkill,
  getSkill,
  listSkillDuplicates,
  listSkills,
  pinSkillCandidate,
  updateSkill,
  type SkillDuplicateGroup,
  type SkillCandidateInfo,
  type SkillInfo,
} from '@/api/client'

const loading = ref(false)
const error = ref('')
const skills = ref<SkillInfo[]>([])

const duplicatesLoading = ref(false)
const duplicatesError = ref('')
const duplicates = ref<SkillDuplicateGroup[]>([])
const duplicatesActionLoading = ref(false)
const duplicatesActionError = ref('')
const archiveShadowedBySkillID = ref<Record<string, boolean>>({})
const advancedOpen = ref(false)

const selectedID = ref('')
const selectedLoading = ref(false)
const selectedError = ref('')
const selected = ref<(SkillInfo & { sha256?: string; skill_md?: string }) | null>(null)
const editMD = ref('')
const editSHA = ref('')

const workspaceRoot = computed(() => {
  try {
    return String(globalThis?.localStorage?.getItem?.('oneagent-workspace') || '').trim()
  } catch {
    return ''
  }
})

const refresh = async () => {
  error.value = ''
  duplicatesError.value = ''
  duplicatesActionError.value = ''
  loading.value = true
  try {
    const list = await listSkills()
    skills.value = Array.isArray(list) ? list : []
  } catch (e: any) {
    skills.value = []
    error.value = String(e?.data?.error || e?.message || 'Failed to load skills')
  } finally {
    loading.value = false
  }

  duplicatesLoading.value = true
  try {
    const dup = await listSkillDuplicates(workspaceRoot.value ? { workspace: workspaceRoot.value } : undefined)
    duplicates.value = Array.isArray(dup) ? dup : []
    for (const g of duplicates.value) {
      if (archiveShadowedBySkillID.value[g.skill_id] === undefined) {
        archiveShadowedBySkillID.value[g.skill_id] = true
      }
    }
  } catch (e: any) {
    duplicates.value = []
    duplicatesError.value = String(e?.data?.error || e?.message || 'Failed to load duplicates')
  } finally {
    duplicatesLoading.value = false
  }
}

const loadSelected = async () => {
  selectedError.value = ''
  const id = String(selectedID.value || '').trim()
  if (!id) {
    selected.value = null
    editMD.value = ''
    editSHA.value = ''
    return
  }

  selectedLoading.value = true
  try {
    const s = await getSkill(id)
    selected.value = s
    editMD.value = String(s.skill_md || '')
    editSHA.value = String(s.sha256 || '')
  } catch (e: any) {
    selected.value = null
    editMD.value = ''
    editSHA.value = ''
    selectedError.value = String(e?.data?.error || e?.message || 'Failed to load skill')
  } finally {
    selectedLoading.value = false
  }
}

const sorted = computed(() => {
  const list = [...skills.value]
  list.sort((a, b) => String(a.skill_id).localeCompare(String(b.skill_id)))
  return list
})

const canEditSelected = computed(() => {
  return Boolean(selected.value && selected.value.archivable)
})

const saveSelected = async () => {
  if (!selected.value) return
  if (!canEditSelected.value) return
  selectedError.value = ''
  selectedLoading.value = true
  try {
    const res = await updateSkill({
      skill_id: selected.value.skill_id,
      skill_md: editMD.value,
      expected_sha256: editSHA.value || undefined,
    })
    selected.value = res
    editMD.value = String(res.skill_md || '')
    editSHA.value = String(res.sha256 || '')
    await refresh()
  } catch (e: any) {
    selectedError.value = String(e?.data?.error || e?.message || 'Failed to save skill')
  } finally {
    selectedLoading.value = false
  }
}

const doArchive = async (s: SkillInfo) => {
  if (!s.archivable) return
  error.value = ''
  loading.value = true
  try {
    await archiveSkill(s.skill_id)
    await refresh()
  } catch (e: any) {
    error.value = String(e?.data?.error || e?.message || 'Failed to archive skill')
  } finally {
    loading.value = false
  }
}

const doPin = async (skillID: string, c: SkillCandidateInfo) => {
  duplicatesActionError.value = ''
  duplicatesActionLoading.value = true
  try {
    await pinSkillCandidate(skillID, {
      source: c.source,
      path: c.path,
      workspace_root: workspaceRoot.value || undefined,
      archive_shadowed_personal: Boolean(archiveShadowedBySkillID.value[skillID]),
    })
    await refresh()
  } catch (e: any) {
    duplicatesActionError.value = String(e?.data?.error || e?.message || 'Failed to pin skill')
  } finally {
    duplicatesActionLoading.value = false
  }
}

const doArchiveShadowed = async (skillID: string) => {
  duplicatesActionError.value = ''
  duplicatesActionLoading.value = true
  try {
    await archiveShadowedPersonalDuplicates(skillID)
    await refresh()
  } catch (e: any) {
    duplicatesActionError.value = String(e?.data?.error || e?.message || 'Failed to archive shadowed duplicates')
  } finally {
    duplicatesActionLoading.value = false
  }
}

const effectiveCandidate = (g: SkillDuplicateGroup) => {
  return g.candidates.find((c) => c.effective) || g.candidates[0] || null
}

const duplicateGroupTitle = (g: SkillDuplicateGroup) => {
  const c = effectiveCandidate(g)
  return (c?.name || '').trim() || g.skill_id
}

onMounted(async () => {
  await refresh()
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">技能治理</h1>
          <p class="text-sm text-surface-500">查看技能并归档个人技能（从召回中移除）</p>
        </div>
        <button
          data-testid="skill-governance-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="loading"
          @click="refresh"
        >
          <RefreshCw class="w-4 h-4" />
          刷新
        </button>
      </div>

      <div v-if="error" class="rounded-2xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
        {{ error }}
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="glass rounded-2xl overflow-hidden lg:col-span-2">
            <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
              <Wrench class="w-5 h-5 text-primary-400" />
              <div>
                <p class="text-sm font-semibold text-surface-100">技能</p>
                <p class="text-xs text-surface-500">已发现 {{ sorted.length }} 个</p>
              </div>
            </div>

            <div class="p-5">
              <div v-if="loading" class="text-sm text-surface-500">加载中…</div>
              <div v-else-if="sorted.length === 0" class="text-sm text-surface-500">未找到技能。</div>
              <div v-else class="space-y-3">
                <button
                  v-for="s in sorted"
                  :key="s.skill_id"
                  data-testid="skill-item"
                  class="glass-card p-4 flex items-start justify-between gap-4 text-left w-full hover:bg-surface-900/40"
                  :class="selectedID === s.skill_id ? 'ring-1 ring-primary-500/30' : ''"
                  @click="selectedID = s.skill_id; loadSelected()"
                >
                  <div class="min-w-0">
                    <p class="text-sm text-surface-100 font-semibold truncate">{{ s.name }}</p>
                    <p class="text-xs text-surface-500 truncate">{{ s.description }}</p>
                  </div>
                  <div class="shrink-0">
                    <button
                      v-if="s.archivable"
                      data-testid="skill-archive"
                      class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                      :disabled="loading"
                      @click.stop="doArchive(s)"
                    >
                      <Archive class="w-4 h-4" />
                      归档
                    </button>
                    <span v-else class="text-xs text-surface-600">—</span>
                  </div>
                </button>
              </div>
            </div>
          </div>
        <div class="space-y-4 lg:col-span-1">
          <div class="glass rounded-2xl overflow-hidden">
            <div class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-2">
              <p class="text-sm font-semibold text-surface-100">编辑器</p>
              <button
                data-testid="skill-save"
                class="px-3 py-2 rounded-xl text-xs font-medium bg-primary-600 text-white hover:bg-primary-500 inline-flex items-center gap-2 disabled:opacity-50"
                :disabled="!canEditSelected || selectedLoading"
                @click="saveSelected"
              >
                <Save class="w-4 h-4" />
                保存
              </button>
            </div>

            <div v-if="selectedError" class="m-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
              {{ selectedError }}
            </div>

            <div v-if="!selected" class="p-6 text-sm text-surface-500">选择技能查看/编辑。</div>
            <div v-else class="p-4 space-y-3">
              <div class="text-xs text-surface-500">
                <div>id: <span class="text-surface-200 font-mono">{{ selected.skill_id }}</span></div>
                <div>source: <span class="text-surface-200 font-mono">{{ selected.source }}</span></div>
                <div v-if="selected.sha256">sha: <span class="text-surface-200 font-mono">{{ selected.sha256.slice(0, 12) }}</span></div>
                <div v-if="!selected.archivable" class="mt-2 text-surface-400">不可编辑（仅个人技能可编辑）。</div>
              </div>
              <textarea
                v-model="editMD"
                rows="18"
                class="w-full rounded-xl bg-surface-950/60 border border-surface-800 text-surface-100 text-xs font-mono px-3 py-2"
                :disabled="!canEditSelected || selectedLoading"
                placeholder="SKILL.md"
              />
            </div>
          </div>

          <div class="glass rounded-2xl overflow-hidden">
            <button
              type="button"
              data-testid="skill-governance-advanced-toggle"
              class="w-full px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-3 text-left hover:bg-surface-900/30"
              @click="advancedOpen = !advancedOpen"
            >
              <div class="flex items-center gap-3">
                <Copy class="w-5 h-5 text-primary-400" />
                <div>
                  <p class="text-sm font-semibold text-surface-100">高级：重复项治理</p>
                  <p class="text-xs text-surface-500">{{ duplicates.length }} 组</p>
                </div>
              </div>
              <ChevronDown v-if="advancedOpen" class="w-4 h-4 text-surface-400" />
              <ChevronRight v-else class="w-4 h-4 text-surface-400" />
            </button>

            <div v-if="advancedOpen" data-testid="skill-governance-advanced" class="p-5">
              <div
                v-if="duplicatesActionError"
                class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200"
              >
                {{ duplicatesActionError }}
              </div>
              <div v-if="duplicatesError" class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
                {{ duplicatesError }}
              </div>
              <div v-else-if="duplicatesLoading" class="text-sm text-surface-500">加载中…</div>
              <div v-else-if="duplicates.length === 0" class="text-sm text-surface-500">未发现重复项。</div>
              <div v-else class="space-y-3">
                <div v-for="g in duplicates" :key="g.skill_id" class="glass-card p-4">
                  <div class="space-y-3">
                    <div class="min-w-0">
                      <p class="text-sm text-surface-100 font-semibold truncate">{{ duplicateGroupTitle(g) }}</p>
                      <p class="text-xs text-surface-500 truncate">id={{ g.skill_id }} · {{ g.candidates.length }} 个候选</p>
                    </div>

                    <div class="flex items-center justify-between gap-2">
                      <label
                        class="text-[11px] text-surface-500 inline-flex items-center gap-2 whitespace-nowrap"
                        title="归档被遮蔽的个人技能"
                      >
                        <input
                          type="checkbox"
                          class="accent-primary-500"
                          v-model="archiveShadowedBySkillID[g.skill_id]"
                          :disabled="duplicatesActionLoading"
                        />
                        归档遮蔽个人
                      </label>
                      <button
                        data-testid="duplicate-archive-shadowed"
                        class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                        :disabled="duplicatesActionLoading"
                        @click="doArchiveShadowed(g.skill_id)"
                      >
                        <Layers class="w-4 h-4" />
                        归档遮蔽项
                      </button>
                    </div>

                    <div class="rounded-xl border border-surface-800/70 bg-surface-950/20 overflow-hidden divide-y divide-surface-800/60">
                      <div
                        v-for="c in g.candidates"
                        :key="`${c.skill_id}:${c.source}:${c.path}`"
                        class="px-3 py-2 flex items-start justify-between gap-3"
                      >
                        <div class="min-w-0">
                          <div class="flex items-center gap-2">
                            <p class="text-xs text-surface-200 font-semibold truncate">{{ c.name }}</p>
                            <span
                              v-if="c.effective"
                              class="text-[11px] px-2 py-0.5 rounded-full bg-primary-500/15 text-primary-200 border border-primary-500/20 shrink-0"
                            >
                              生效
                            </span>
                          </div>
                          <p class="text-[11px] text-surface-500 truncate mt-1">
                            source={{ c.source }} · rank={{ c.precedence_rank }} · {{ c.path }}
                          </p>
                        </div>

                        <div class="shrink-0 flex flex-col items-end gap-2">
                          <button
                            v-if="c.source !== '.oneagent'"
                            data-testid="duplicate-pin"
                            class="px-2 py-1 rounded-lg text-[11px] font-medium bg-primary-500/15 text-primary-200 hover:bg-primary-500/20 inline-flex items-center gap-2 disabled:opacity-50 whitespace-nowrap"
                            :disabled="duplicatesActionLoading"
                            @click="doPin(g.skill_id, c)"
                          >
                            <Pin class="w-4 h-4" />
                            固定
                          </button>
                          <button
                            v-if="c.archivable"
                            data-testid="duplicate-archive"
                            class="px-2 py-1 rounded-lg text-[11px] font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2 disabled:opacity-50 whitespace-nowrap"
                            :disabled="loading"
                            @click="doArchive(c)"
                          >
                            <Archive class="w-4 h-4" />
                            归档
                          </button>
                          <span v-else class="text-xs text-surface-600">—</span>
                        </div>
                      </div>
                    </div>
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
