<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  Home,
  MessageSquare,
  ScrollText,
  Settings,
  LogOut,
  ChevronLeft,
  ChevronRight,
  Menu,
  User,
  Sun,
  Moon,
  Bot,
  ListTodo,
  ListChecks,
} from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import { useTheme } from '@/composables/useTheme'
import { getLedgerStatusToday } from '@/api/client'

const router = useRouter()
const route = useRoute()
const { user, logout } = useAuth()
const { isDark, toggleTheme } = useTheme()

// Sidebar state
const isCollapsed = ref(false)
const isMobileOpen = ref(false)

// Navigation items
const navItems = [
  { name: 'Home', path: '/', icon: Home },
  { name: 'Chat', path: '/chat', icon: MessageSquare },
  { name: 'Tasks', path: '/tasks', icon: ListTodo },
  { name: 'SOP Governance', path: '/governance/sop', icon: ListChecks },
  { name: 'Skill Governance', path: '/governance/skills', icon: ListChecks },
  { name: 'Ledger', path: '/ledger', icon: ScrollText },
  { name: 'Settings', path: '/settings', icon: Settings },

]

const isActive = (path: string) => route.path === path

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
  localStorage.setItem('sidebar-collapsed', String(isCollapsed.value))
}

const toggleMobile = () => {
  isMobileOpen.value = !isMobileOpen.value
}

const handleLogout = () => {
  logout()
}

const navigateTo = (path: string) => {
  router.push(path)
  isMobileOpen.value = false
}

// Load saved state
const savedState = localStorage.getItem('sidebar-collapsed')
if (savedState) {
  isCollapsed.value = savedState === 'true'
}

const sidebarWidth = computed(() => (isCollapsed.value ? 'w-16' : 'w-64'))

const sopBadgeCount = ref(0)

const refreshLedgerStatus = async () => {
  try {
    const st = await getLedgerStatusToday()
    sopBadgeCount.value = Number(st?.sop_proposed_count || 0)
  } catch {
    // keep last known
  }
}

let statusTimer: number | undefined
onMounted(() => {
  void refreshLedgerStatus()
  statusTimer = window.setInterval(() => {
    void refreshLedgerStatus()
  }, 30_000)
})
onUnmounted(() => {
  if (statusTimer != null) {
    window.clearInterval(statusTimer)
    statusTimer = undefined
  }
})
</script>

<template>
  <!-- Mobile menu button -->
  <button
    class="fixed top-4 left-4 z-50 p-2 rounded-lg bg-surface-800 text-surface-200 lg:hidden"
    @click="toggleMobile"
  >
    <Menu class="w-5 h-5" />
  </button>

  <!-- Overlay for mobile -->
  <div
    v-if="isMobileOpen"
    class="fixed inset-0 bg-black/50 z-40 lg:hidden"
    @click="toggleMobile"
  />

  <!-- Sidebar -->
  <aside
    :class="[
      'fixed left-0 top-0 h-full z-40 transition-all duration-300 ease-in-out',
      'glass border-r-0',
      sidebarWidth,
      isMobileOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
    ]"
  >
    <div class="flex flex-col h-full bg-surface-900/50 backdrop-blur-sm">
      <!-- Header -->
      <div
        class="flex items-center justify-between h-16 px-4"
      >
        <div
          v-if="!isCollapsed"
          class="flex items-center gap-2 overflow-hidden"
        >
          <div
            class="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-600 flex items-center justify-center shadow-lg shadow-primary-500/20"
          >
            <Bot class="w-5 h-5 text-white" />
          </div>
          <span class="font-bold text-lg text-surface-900 dark:text-white whitespace-nowrap">
            OneAgent
          </span>
        </div>
        <div class="flex items-center gap-1">
          <button
              @click="toggleTheme"
              class="p-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-primary-500 transition-all duration-200"
              title="Toggle Theme"
          >
            <Sun v-if="!isDark" class="w-5 h-5" />
            <Moon v-else class="w-5 h-5" />
          </button>
          <button
            @click="toggleSidebar"
            class="p-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 hidden lg:flex"
          >
            <ChevronLeft v-if="!isCollapsed" class="w-5 h-5" />
            <ChevronRight v-else class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 px-3 py-4 overflow-y-auto">
        <ul class="space-y-1">
          <li v-for="item in navItems" :key="item.name">
            <button
              @click="navigateTo(item.path)"
              :class="[
                'w-full flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-300 group relative overflow-hidden',
                isActive(item.path)
                  ? 'bg-primary-500/10 text-primary-400 shadow-[0_0_20px_rgba(99,102,241,0.1)]'
                  : 'text-surface-400 hover:bg-surface-800/50 hover:text-surface-200',
              ]"
            >
              <div v-if="isActive(item.path)" class="absolute inset-y-0 left-0 w-1 bg-primary-500 rounded-r-full"></div>
              <component
                :is="item.icon"
                :class="[
                  'w-5 h-5 flex-shrink-0 transition-transform duration-300',
                  isActive(item.path) ? 'scale-110' : 'group-hover:scale-110'
                ]"
              />
              <span
                v-if="!isCollapsed"
                class="whitespace-nowrap overflow-hidden font-medium flex items-center gap-2"
              >
                {{ item.name }}
                <span
                  v-if="item.path === '/governance/sop' && sopBadgeCount > 0"
                  data-testid="sidebar-sop-badge"
                  class="inline-flex items-center justify-center rounded-full bg-primary-500/15 text-primary-300 border border-primary-500/20 px-2 py-0.5 text-[11px]"
                >
                  {{ sopBadgeCount }}
                </span>
              </span>
            </button>
          </li>
        </ul>

        <!-- Recent Sessions -->

      </nav>

      <!-- User section -->
      <div class="p-3 mx-2 mb-2">
        <div
          :class="[
            'flex items-center gap-3 px-3 py-2 rounded-xl transition-colors hover:bg-surface-800/50',
            isCollapsed ? 'justify-center' : '',
          ]"
        >
          <div
            class="w-9 h-9 rounded-full bg-gradient-to-br from-surface-700 to-surface-800 border border-surface-600 flex items-center justify-center flex-shrink-0 shadow-lg"
          >
            <User class="w-4 h-4 text-surface-300" />
          </div>
          <div v-if="!isCollapsed" class="flex-1 min-w-0">
            <p class="text-sm font-semibold text-surface-200 truncate">
              {{ user?.name || user?.username || 'User' }}
            </p>
            <p class="text-xs text-surface-500 truncate">
              {{ user?.email || '' }}
            </p>
          </div>
          <button
            v-if="!isCollapsed"
            @click="handleLogout"
            class="p-2 rounded-lg hover:bg-red-500/10 text-surface-400 hover:text-red-400 transition-colors"
            title="Logout"
          >
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </aside>

  <!-- Spacer for main content -->
  <div
    :class="[
      'hidden lg:block flex-shrink-0 transition-all duration-300',
      sidebarWidth,
    ]"
  />
</template>

<style scoped>
/* Custom scrollbar for nav */
nav::-webkit-scrollbar {
  width: 4px;
}

nav::-webkit-scrollbar-thumb {
  background-color: var(--color-surface-700);
  border-radius: 9999px;
}
</style>
