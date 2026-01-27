import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { initAuth } from "@/composables/useAuth";

const routes = [
  {
    path: "/",
    name: "Home",
    component: () => import("@/views/Home.vue"),
    meta: { requiresAuth: true },
    alias: "/chat",
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
    meta: { requiresAuth: true },
  },
  {
    path: "/ledger",
    name: "Ledger",
    component: () => import("@/views/Ledger.vue"),
    meta: { requiresAuth: true },
  },
  {
    path: "/tasks",
    name: "Tasks",
    component: () => import("@/views/TaskWorkbench.vue"),
    meta: { requiresAuth: true },
  },
  {
    path: "/governance/sop",
    name: "SOP Governance",
    component: () => import("@/views/SopGovernance.vue"),
    meta: { requiresAuth: true },
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
});

export default router;
