<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Archive, RefreshCw, Wrench, Save, Copy } from 'lucide-vue-next'

import {
  archiveSkill,
  getSkill,
  listSkillDuplicates,
  listSkills,
  updateSkill,
  type SkillDuplicateGroup,
  type SkillInfo,
} from '@/api/client'

const loading = ref(false)
const error = ref('')
const skills = ref<SkillInfo[]>([])

const duplicatesLoading = ref(false)
const duplicatesError = ref('')
const duplicates = ref<SkillDuplicateGroup[]>([])

const selectedID = ref('')
const selectedLoading = ref(false)
const selectedError = ref('')
const selected = ref<(SkillInfo & { sha256?: string; skill_md?: string }) | null>(null)
const editMD = ref('')
const editSHA = ref('')

const refresh = async () => {
  error.value = ''
  duplicatesError.value = ''
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
    const dup = await listSkillDuplicates()
    duplicates.value = Array.isArray(dup) ? dup : []
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

onMounted(async () => {
  await refresh()
})
</script>

<template>
  <div class="min-h-screen p-6 lg:p-10">
    <div class="max-w-6xl mx-auto space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-surface-100">Skill Governance</h1>
          <p class="text-sm text-surface-500">List skills and archive personal ones (remove from recall)</p>
        </div>
        <button
          data-testid="skill-governance-refresh"
          class="px-4 py-2 rounded-xl text-sm font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
          :disabled="loading"
          @click="refresh"
        >
          <RefreshCw class="w-4 h-4" />
          Refresh
        </button>
      </div>

      <div v-if="error" class="rounded-2xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
        {{ error }}
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="lg:col-span-2 space-y-4">
          <div class="glass rounded-2xl overflow-hidden">
            <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
              <Wrench class="w-5 h-5 text-primary-400" />
              <div>
                <p class="text-sm font-semibold text-surface-100">Skills</p>
                <p class="text-xs text-surface-500">{{ sorted.length }} discovered</p>
              </div>
            </div>

            <div class="p-5">
              <div v-if="loading" class="text-sm text-surface-500">Loading…</div>
              <div v-else-if="sorted.length === 0" class="text-sm text-surface-500">No skills found.</div>
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
                    <p class="text-[11px] text-surface-500 mt-2 truncate">
                      id={{ s.skill_id }} · source={{ s.source }} · {{ s.path }}
                    </p>
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
                      Archive
                    </button>
                    <span v-else class="text-xs text-surface-600">—</span>
                  </div>
                </button>
              </div>
            </div>
          </div>
        </div>

          <div class="glass rounded-2xl overflow-hidden">
            <div class="px-5 py-4 border-b border-surface-700/50 flex items-center gap-3">
              <Copy class="w-5 h-5 text-primary-400" />
              <div>
                <p class="text-sm font-semibold text-surface-100">Duplicates</p>
                <p class="text-xs text-surface-500">{{ duplicates.length }} groups</p>
              </div>
            </div>

            <div class="p-5">
              <div v-if="duplicatesError" class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
                {{ duplicatesError }}
              </div>
              <div v-else-if="duplicatesLoading" class="text-sm text-surface-500">Loading…</div>
              <div v-else-if="duplicates.length === 0" class="text-sm text-surface-500">No duplicates found.</div>
              <div v-else class="space-y-3">
                <div v-for="g in duplicates" :key="g.skill_id" class="glass-card p-4 space-y-3">
                  <div class="flex items-center justify-between gap-4">
                    <div class="min-w-0">
                      <p class="text-sm text-surface-100 font-semibold truncate">id={{ g.skill_id }}</p>
                      <p class="text-xs text-surface-500">{{ g.candidates.length }} candidates · first effective=true</p>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <div
                      v-for="c in g.candidates"
                      :key="`${c.skill_id}:${c.source}:${c.path}`"
                      class="rounded-xl border border-surface-800/70 bg-surface-950/30 p-3 flex items-start justify-between gap-4"
                    >
                      <div class="min-w-0">
                        <p class="text-xs text-surface-200 truncate">
                          <span class="font-semibold">{{ c.name }}</span>
                          <span v-if="c.effective" class="ml-2 text-[11px] text-primary-300">effective</span>
                        </p>
                        <p class="text-[11px] text-surface-500 truncate">
                          source={{ c.source }} · rank={{ c.precedence_rank }} · {{ c.path }}
                        </p>
                      </div>
                      <div class="shrink-0">
                        <button
                          v-if="c.archivable"
                          data-testid="duplicate-archive"
                          class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                          :disabled="loading"
                          @click="doArchive(c)"
                        >
                          <Archive class="w-4 h-4" />
                          Archive
                        </button>
                        <span v-else class="text-xs text-surface-600">—</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

        <div class="glass rounded-2xl overflow-hidden lg:col-span-1">
          <div class="px-5 py-4 border-b border-surface-700/50 flex items-center justify-between gap-2">
            <p class="text-sm font-semibold text-surface-100">Editor</p>
            <button
              data-testid="skill-save"
              class="px-3 py-2 rounded-xl text-xs font-medium bg-primary-600 text-white hover:bg-primary-500 inline-flex items-center gap-2 disabled:opacity-50"
              :disabled="!canEditSelected || selectedLoading"
              @click="saveSelected"
            >
              <Save class="w-4 h-4" />
              Save
            </button>
          </div>

          <div v-if="selectedError" class="m-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-sm text-rose-200">
            {{ selectedError }}
          </div>

          <div v-if="!selected" class="p-6 text-sm text-surface-500">Select a skill to view/edit.</div>
          <div v-else class="p-4 space-y-3">
            <div class="text-xs text-surface-500">
              <div>id: <span class="text-surface-200 font-mono">{{ selected.skill_id }}</span></div>
              <div>source: <span class="text-surface-200 font-mono">{{ selected.source }}</span></div>
              <div v-if="selected.sha256">sha: <span class="text-surface-200 font-mono">{{ selected.sha256.slice(0, 12) }}</span></div>
              <div v-if="!selected.archivable" class="mt-2 text-surface-400">Not editable (only personal skills are editable).</div>
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
      </div>
    </div>
  </div>
</template>
