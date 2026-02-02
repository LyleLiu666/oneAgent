// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

const makeLocalStorage = () => {
  const store = new Map<string, string>();
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => void store.clear(),
  };
};

vi.mock("vue-router", () => ({
  RouterView: { template: "<div />" },
  useRoute: () => ({ meta: {} }),
}));

beforeEach(() => {
  document.documentElement.classList.remove("dark");
  vi.clearAllMocks();
});

it("initializes theme even when Sidebar is hidden (secretary mode)", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());
  localStorage.setItem("theme", "dark");

  const pinia = createPinia();
  setActivePinia(pinia);

  const { useAuthStore } = await import("@/stores/auth");
  const auth = useAuthStore();
  auth.setToken("t");
  auth.setUser({ id: "u1", username: "u", email: "e", name: "n" });

  const { useUIStore } = await import("@/stores/ui");
  const ui = useUIStore();
  ui.setMode("secretary");

  const { default: App } = await import("@/App.vue");
  const wrapper = mount(App, { global: { plugins: [pinia] } });

  await flushPromises();

  expect(document.documentElement.classList.contains("dark")).toBe(true);

  wrapper.unmount();
});

