<script setup lang="ts">
import { RouterView } from 'vue-router'
import Sidebar from '@/components/Sidebar.vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
</script>

<template>
  <div class="flex min-h-screen bg-background transition-colors duration-300">
    <!-- Sidebar Navigation -->
    <Sidebar v-if="authStore.isAuthenticated" />
    
    <!-- Main Content -->
    <main 
      class="flex-1 transition-all duration-300 relative z-10"
      :class="authStore.isAuthenticated ? 'ml-0' : ''"
    >
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
/* App-level styles if needed */
</style>
