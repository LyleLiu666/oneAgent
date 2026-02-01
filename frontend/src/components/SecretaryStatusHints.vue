<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useLedgerStatusToday } from '@/composables/useLedgerStatusToday'
import { useUIStore } from '@/stores/ui'

const ui = useUIStore()
const router = useRouter()

const { sopProposedCount } = useLedgerStatusToday()

const showSopHint = computed(() => ui.mode === 'secretary' && sopProposedCount.value > 0)

const onClickSopHint = async () => {
  ui.setMode('full')
  await router.push('/governance/sop')
}
</script>

<template>
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

