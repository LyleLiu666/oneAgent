// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import type { Task } from "@/api/client";

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

const mocks = vi.hoisted(() => {
  return {
    routerPush: vi.fn(),
    listTasks: vi.fn(async (): Promise<Task[]> => []),
    resumeTask: vi.fn(),
    getTaskAttemptArtifact: vi.fn(async () => ({
      path: "findings.md",
      content: "# Findings\n- ok\n",
      truncated: false,
    })),
  };
});

vi.mock("vue-router", () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}));

vi.mock("@/api/client", () => ({
  listTasks: mocks.listTasks,
  resumeTask: mocks.resumeTask,
  getTaskAttemptArtifact: mocks.getTaskAttemptArtifact,
}));

beforeEach(() => {
  vi.clearAllMocks();
});

it("renders deliverable cards for completed task artifacts", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "done",
          findings_path: "/tmp/ws/FINDINGS.md",
          test_report_path: "/tmp/ws/TEST_REPORT.txt",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  expect(wrapper.find('[data-testid="secretary-task-deliverables"]').exists()).toBe(
    true,
  );
  expect(
    wrapper.findAll('[data-testid="secretary-task-deliverable-card"]').length,
  ).toBe(1);

  wrapper.unmount();
});

it("opens artifact preview modal when clicking findings", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "done",
          findings_path: "/tmp/ws/FINDINGS.md",
        },
      ],
    },
  ]);

  mocks.getTaskAttemptArtifact.mockResolvedValueOnce({
    path: "/tmp/ws/FINDINGS.md",
    content: "# Findings\n- ok\n",
    truncated: false,
  });

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  const card = wrapper.get('[data-testid="secretary-task-deliverable-card"]');
  await card.get('[data-testid="deliverable-open-findings"]').trigger("click");
  await flushPromises();

  expect(mocks.getTaskAttemptArtifact).toHaveBeenCalledWith("t1", "a1", "findings");
  expect(
    wrapper.find('[data-testid="secretary-task-artifact-modal"]').exists(),
  ).toBe(true);
  expect(wrapper.text()).toContain("# Findings");

  wrapper.unmount();
});

it("surfaces failed tasks and allows resuming from secretary mode", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  const { useUIStore } = await import("@/stores/ui");
  const ui = useUIStore();
  ui.setMode("secretary");

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "failed",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "failed",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  expect(wrapper.find('[data-testid="secretary-task-recovery"]').exists()).toBe(true);
  await wrapper.get('[data-testid="secretary-task-recovery-resume"]').trigger("click");
  expect(mocks.resumeTask).toHaveBeenCalledWith("t1");

  wrapper.unmount();
});

it("enters full mode and navigates to tasks when troubleshooting from secretary mode", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  const { useUIStore } = await import("@/stores/ui");
  const ui = useUIStore();
  ui.setMode("secretary");

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "failed",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "failed",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  await wrapper.get('[data-testid="secretary-task-recovery-troubleshoot"]').trigger("click");
  expect(ui.mode).toBe("full");
  expect(mocks.routerPush).toHaveBeenCalledWith("/tasks");

  wrapper.unmount();
});

it("emits task-completed when a running task finishes (no history replay)", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "running",
          created_at: "2026-02-01T00:00:01Z",
          started_at: "2026-02-01T00:00:01Z",
        },
      ],
    },
  ]);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:03Z",
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:03Z",
          summary: "done",
          findings_path: "/tmp/ws/FINDINGS.md",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  expect(wrapper.emitted("task-completed")).toBeUndefined();

  // Trigger a second refresh via the workspace watch (keeping normalized path stable).
  await wrapper.setProps({ workspace: "/tmp/ws " });
  await flushPromises();

  const events = wrapper.emitted("task-completed");
  expect(events?.length).toBe(1);
  expect(events?.[0]?.[0]).toMatchObject({
    taskId: "t1",
    status: "succeeded",
  });

  wrapper.unmount();
});

it("uses readable warning accents in light mode", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "failed",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "failed",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  const recovery = wrapper.get('[data-testid="secretary-task-recovery"]');
  const heading = recovery.get("div.text-xs.font-semibold");
  expect(heading.classes()).toContain("text-amber-800");
  expect(heading.classes()).toContain("dark:text-amber-200");

  const resume = wrapper.get('[data-testid="secretary-task-recovery-resume"]');
  expect(resume.classes()).toContain("text-amber-900");
  expect(resume.classes()).toContain("dark:text-amber-100");

  wrapper.unmount();
});

it("can collapse and expand deliverables in secretary mode", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "done",
          findings_path: "/tmp/ws/FINDINGS.md",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();
  expect(wrapper.find('[data-testid="secretary-task-deliverable-card"]').exists()).toBe(
    true,
  );

  await wrapper.get('[data-testid="secretary-task-deliverables-toggle"]').trigger("click");
  await flushPromises();
  expect(wrapper.find('[data-testid="secretary-task-deliverable-card"]').exists()).toBe(
    false,
  );

  await wrapper.get('[data-testid="secretary-task-deliverables-toggle"]').trigger("click");
  await flushPromises();
  expect(wrapper.find('[data-testid="secretary-task-deliverable-card"]').exists()).toBe(
    true,
  );

  wrapper.unmount();
});

it("can dismiss deliverable cards and persist them in localStorage", async () => {
  const ls = makeLocalStorage();
  vi.stubGlobal("localStorage", ls);

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "done",
          findings_path: "/tmp/ws/FINDINGS.md",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();
  expect(wrapper.find('[data-testid="secretary-task-deliverable-card"]').exists()).toBe(
    true,
  );

  await wrapper.get('[data-testid="secretary-task-deliverable-dismiss"]').trigger("click");
  await flushPromises();
  expect(wrapper.find('[data-testid="secretary-task-deliverable-card"]').exists()).toBe(
    false,
  );

  const raw = ls.getItem("oneagent-secretary-dismissed-attempts-v1");
  expect(raw).toBeTruthy();
  const parsed = JSON.parse(String(raw));
  expect(parsed["/tmp/ws"]).toContain("t1:a1");

  wrapper.unmount();
});

it("does not duplicate needs-attention tasks in deliverables", async () => {
  vi.stubGlobal("localStorage", makeLocalStorage());

  const pinia = createPinia();
  setActivePinia(pinia);

  mocks.listTasks.mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "u1",
      workspace: "/tmp/ws",
      title: "task1",
      prompt: "p",
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-01T00:00:02Z",
      attempts: [
        {
          id: "a1",
          status: "failed",
          created_at: "2026-02-01T00:00:01Z",
          finished_at: "2026-02-01T00:00:02Z",
          summary: "failed",
          findings_path: "/tmp/ws/FINDINGS.md",
          trace_log_path: "/tmp/ws/trace.jsonl",
        },
      ],
    },
  ]);

  const { default: SecretaryTaskDeliverables } = await import(
    "@/components/SecretaryTaskDeliverables.vue"
  );
  const wrapper = mount(SecretaryTaskDeliverables, {
    props: { workspace: "/tmp/ws", pollIntervalMs: 0 },
    global: { plugins: [pinia] },
  });

  await flushPromises();

  expect(wrapper.find('[data-testid="secretary-task-recovery"]').exists()).toBe(true);
  expect(wrapper.find('[data-testid="secretary-task-deliverables"]').exists()).toBe(false);
  expect(wrapper.text()).toContain("diff");

  wrapper.unmount();
});
