<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Archive, RefreshCw, Wrench } from 'lucide-vue-next'

import { archiveSkill, listSkills, type SkillInfo } from '@/api/client'

const loading = ref(false)
const error = ref('')
const skills = ref<SkillInfo[]>([])

const refresh = async () => {
  error.value = ''
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
}

const sorted = computed(() => {
  const list = [...skills.value]
  list.sort((a, b) => String(a.skill_id).localeCompare(String(b.skill_id)))
  return list
})

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
            <div v-for="s in sorted" :key="s.skill_id" class="glass-card p-4 flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm text-surface-100 font-semibold truncate">{{ s.name }}</p>
                <p class="text-xs text-surface-500 truncate">{{ s.description }}</p>
                <p class="text-[11px] text-surface-500 mt-2 truncate">id={{ s.skill_id }} · source={{ s.source }} · {{ s.path }}</p>
              </div>
              <div class="shrink-0">
                <button
                  v-if="s.archivable"
                  data-testid="skill-archive"
                  class="px-3 py-2 rounded-xl text-xs font-medium bg-surface-900/60 text-surface-300 hover:bg-surface-800/60 inline-flex items-center gap-2"
                  :disabled="loading"
                  @click="doArchive(s)"
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
</template>

