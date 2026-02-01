<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useLedgerStatusToday } from '@/composables/useLedgerStatusToday'
import { useTaskQueueStatus } from '@/composables/useTaskQueueStatus'
import { useUIStore } from '@/stores/ui'

const ui = useUIStore()
const router = useRouter()

const { sopProposedCount } = useLedgerStatusToday()
const { activeTaskCount } = useTaskQueueStatus({ pollIntervalMs: 10_000 })

const showSopHint = computed(() => ui.mode === 'secretary' && sopProposedCount.value > 0)
const showTaskHint = computed(() => ui.mode === 'secretary' && activeTaskCount.value > 0)

const onClickSopHint = async () => {
  ui.setMode('full')
  await router.push('/governance/sop')
}

const onClickTaskHint = async () => {
  ui.setMode('full')
  await router.push('/tasks')
}
</script>

<template>
  <button
    v-if="showTaskHint"
    type="button"
    data-testid="secretary-task-hint"
    class="inline-flex items-center gap-2 rounded-full border border-surface-600/40 bg-surface-900/40 px-3 py-1 text-xs text-surface-200 hover:bg-surface-800/50 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
    title="有任务正在运行/排队"
    @click="onClickTaskHint"
  >
    <span class="tracking-wide">任务</span>
    <span
      data-testid="secretary-task-hint-count"
      class="inline-flex min-w-[18px] items-center justify-center rounded-full bg-surface-800/80 px-1.5 py-0.5 text-[11px] text-surface-100"
    >
      {{ activeTaskCount }}
    </span>
  </button>

  <button
    v-if="showSopHint"
    type="button"
    data-testid="secretary-sop-hint"
    class="inline-flex items-center gap-2 rounded-full border border-primary-500/20 bg-primary-500/10 px-3 py-1 text-xs text-primary-200 hover:bg-primary-500/15 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
    title="有新的 SOP 待治理"
    @click="onClickSopHint"
  >
    <span class="tracking-wide">SOP</span>
    <span
      data-testid="secretary-sop-hint-count"
      class="inline-flex min-w-[18px] items-center justify-center rounded-full bg-primary-500/20 px-1.5 py-0.5 text-[11px] text-primary-100"
    >
      {{ sopProposedCount }}
    </span>
  </button>
</template>
