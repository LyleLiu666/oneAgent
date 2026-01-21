<template>
  <div class="h-full flex flex-col p-4">
    <h1 class="text-2xl font-bold mb-4 text-gray-800 dark:text-gray-100">
      Smart Block Edit Debugger
    </h1>

    <div class="flex-1 flex flex-col gap-4">
      <div class="flex flex-col gap-2 flex-1">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300"
          >Command Input (Heredoc)</label
        >
        <textarea
          v-model="inputCommand"
          class="flex-1 w-full p-4 font-mono text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none resize-none"
          placeholder="apply_smart_edit <<'EOF'
file: /path/to/file.go
<<<< SEARCH
func foo() {
  return
}
==== REPLACE
func foo() {
  println('hello')
  return
}
>>>>
EOF"
        ></textarea>
      </div>

      <div class="flex justify-end">
        <button
          @click="applySmartEdit"
          :disabled="loading"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
        >
          <span v-if="loading" class="animate-spin">⌛</span>
          Apply Edit
        </button>
      </div>

      <div class="h-1/3 flex flex-col gap-2">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300"
          >Output</label
        >
        <div
          class="flex-1 w-full p-4 font-mono text-sm bg-gray-900 text-green-400 rounded-lg overflow-auto whitespace-pre-wrap"
        >
          {{ output || "Waiting for input..." }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";

const inputCommand = ref("");
const output = ref("");
const loading = ref(false);

const applySmartEdit = async () => {
  if (!inputCommand.value.trim()) return;

  loading.value = true;
  output.value = "Executing...";

  try {
    const response = await fetch("/api/sbe/apply", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ command: inputCommand.value }),
    });

    const data = await response.json();

    if (response.ok) {
      output.value = `SUCCESS:\n${JSON.stringify(data, null, 2)}`;
    } else {
      output.value = `ERROR (${response.status}):\n${JSON.stringify(data, null, 2)}`;
    }
  } catch (err: any) {
    output.value = `NETWORK ERROR:\n${err.message}`;
  } finally {
    loading.value = false;
  }
};
</script>
