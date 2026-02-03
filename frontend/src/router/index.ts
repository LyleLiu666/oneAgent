import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { initAuth } from "@/composables/useAuth";
import { useUIStore } from "@/stores/ui";

const routes = [
  {
    path: "/",
    name: "Home",
    redirect: () => {
      const raw = localStorage.getItem("oneagent-ui-mode");
      const normalize = (value: any) => String(value ?? "").trim().toLowerCase();

      if (raw == null) return "/secretary";
      const direct = normalize(raw);
      if (direct === "full") return "/chat";
      if (direct === "secretary") return "/secretary";

      try {
        const parsed = JSON.parse(raw) as any;
        const nested = normalize(parsed?.mode);
        if (nested === "full") return "/chat";
        if (nested === "secretary") return "/secretary";
      } catch {
        // ignore
      }

      return "/secretary";
    },
    meta: { requiresAuth: true },
  },
  {
    path: "/chat",
    name: "Chat",
    component: () => import("@/views/Home.vue"),
    meta: { requiresAuth: true },
  },
  {
    path: "/secretary",
    name: "Secretary",
    component: () => import("@/views/Secretary.vue"),
    meta: { requiresAuth: true },
  },
  {
    path: "/login",
    name: "Login",
    component: () => import("@/views/Login.vue"),
    meta: { requiresAuth: false },
  },
  {
    path: "/settings",
    name: "Settings",
    component: () => import("@/views/Settings.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/ledger",
    name: "Ledger",
    component: () => import("@/views/Ledger.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/tasks",
    name: "Tasks",
    component: () => import("@/views/TaskWorkbench.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/workflows",
    name: "Workflows",
    component: () => import("@/views/Workflows.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/workflows/:id/runs/:runId",
    name: "Workflow Run",
    component: () => import("@/views/WorkflowRun.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/governance/sop",
    name: "SOP Governance",
    component: () => import("@/views/SopGovernance.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/governance/skills",
    name: "Skill Governance",
    component: () => import("@/views/SkillGovernance.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/governance/tools",
    name: "Tool Permissions",
    component: () => import("@/views/ToolPermissions.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },
  {
    path: "/documents/export",
    name: "Document Export",
    component: () => import("@/views/DocumentExport.vue"),
    meta: { requiresAuth: true, requiresFullMode: true },
  },

  {
    path: "/:pathMatch(.*)*",
    name: "NotFound",
    component: () => import("@/views/NotFound.vue"),
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Navigation guard for authentication
router.beforeEach(async (to) => {
  const authStore = useAuthStore();
  const uiStore = useUIStore();

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    await initAuth();
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    // Redirect to login if not authenticated
    return { name: "Login", query: { redirect: to.fullPath } };
  }

  if (to.name === "Login" && authStore.isAuthenticated) {
    // Redirect to home if already authenticated
    return { name: "Home" };
  }

  // Keep route <-> ui_mode consistent for chat surfaces.
  if (to.path === "/chat" && uiStore.mode !== "full") {
    uiStore.setMode("full");
  }
  if (to.path === "/secretary" && uiStore.mode !== "secretary") {
    uiStore.setMode("secretary");
  }
});

export default router;
