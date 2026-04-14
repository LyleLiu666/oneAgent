// @vitest-environment jsdom

import { beforeEach, expect, it, vi } from "vitest";
import { shallowMount } from "@vue/test-utils";

import * as apiClient from "@/api/client";

vi.mock("@/api/client", () => ({
  listTasks: vi.fn(async () => []),
  createTask: vi.fn(async () => ({})),
  getTask: vi.fn(async () => ({})),
  getTaskAttemptArtifact: vi.fn(async () => ({})),
  getTaskAttemptDiffPatch: vi.fn(async () => ({})),
  getTaskAttemptChangedFiles: vi.fn(async () => ({})),
  listTaskAttemptFiles: vi.fn(async () => ({ files: [], omitted: 0 })),
  readTaskAttemptFileSnapshot: vi.fn(async () => ({ path: "", content: "", truncated: false })),
  getTaskQueueGovernanceSnapshot: vi.fn(async () => ({
    ts: new Date().toISOString(),
    global: { max_running_workspaces: 0 },
    running_workspaces: [],
    deferred_workspaces: 0,
    paused_workspaces: 0,
    workspaces: {},
  })),
  getTaskEvents: vi.fn(async () => []),
  cancelTask: vi.fn(async () => ({})),
  resumeTask: vi.fn(async () => ({})),
  listTaskAttemptReviewComments: vi.fn(async () => []),
  postTaskAttemptReviewComment: vi.fn(async () => ({})),
  chooseWorkspaceDir: vi.fn(async () => ({ path: "/tmp/ws" })),
  browseWorkspaceDir: vi.fn(),
  createWorkspaceDir: vi.fn(),
}));

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
});

it("loads tasks and groups by workspace", async () => {
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "running", created_at: new Date().toISOString() },
      ],
    },
    {
      id: "t2",
      user_id: "local",
      workspace: "/tmp/wsB",
      title: "B1",
      prompt: "do B",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "queued", created_at: new Date().toISOString() },
      ],
    },
  ]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  expect(apiClient.listTasks).toHaveBeenCalled();

  const wsItems = wrapper.findAll('[data-testid="workspace-item"]');
  expect(wsItems.length).toBeGreaterThanOrEqual(2);

  wrapper.unmount();
});

it("uses a wider container to reduce side whitespace", async () => {
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  expect(wrapper.find(".max-w-screen-2xl").exists()).toBe(true);

  wrapper.unmount();
});

it("shows worktree evidence in advanced tab when present", async () => {
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "succeeded", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      {
        id: "a1",
        status: "succeeded",
        created_at: new Date().toISOString(),
        worktree_root: "/tmp/wsA/.oneagent-worktree/a1",
        base_ref: "main",
        base_commit_sha: "abc123",
      },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  await wrapper
    .get('[data-testid="workbench-details-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  expect(
    wrapper.find('[data-testid="workbench-worktree-evidence"]').exists(),
  ).toBe(true);
  expect(wrapper.get('[data-testid="workbench-worktree-root"]').text()).toBe(
    "/tmp/wsA/.oneagent-worktree/a1",
  );
  expect(wrapper.get('[data-testid="workbench-base-ref"]').text()).toBe("main");
  expect(wrapper.get('[data-testid="workbench-base-commit-sha"]').text()).toBe(
    "abc123",
  );

  wrapper.unmount();
});

it("does not reload review artifacts on background refresh when attempt unchanged", async () => {
  vi.useFakeTimers();
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValue([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "running", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockImplementation(async () => ({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "running", created_at: new Date().toISOString() },
    ],
  }));
  (apiClient.getTaskEvents as any).mockResolvedValue([]);
  (apiClient.getTaskAttemptDiffPatch as any).mockResolvedValue({
    path: "/tmp/diff.patch",
    content: "diff --git a/a b/a\n",
    truncated: false,
  });
  (apiClient.getTaskAttemptChangedFiles as any).mockResolvedValue({
    path: "/tmp/changed_files.txt",
    content: "a\n",
    truncated: false,
  });
  (apiClient.listTaskAttemptReviewComments as any).mockResolvedValue([]);

  const wrapper = shallowMount(TaskWorkbench);

  const tick = async () => {
    await Promise.resolve();
    await wrapper.vm.$nextTick();
  };

  await tick();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await tick();

  await wrapper.get('[data-testid="workbench-review-toggle"]').trigger("click");
  await tick();

  expect(apiClient.getTaskAttemptDiffPatch).toHaveBeenCalledTimes(1);
  expect(apiClient.getTaskAttemptChangedFiles).toHaveBeenCalledTimes(1);

  vi.advanceTimersByTime(2000);
  await tick();
  await tick();

  expect(apiClient.getTask).toHaveBeenCalledTimes(2);
  expect(apiClient.getTaskAttemptDiffPatch).toHaveBeenCalledTimes(1);
  expect(apiClient.getTaskAttemptChangedFiles).toHaveBeenCalledTimes(1);

  wrapper.unmount();
  vi.useRealTimers();
});

it("queues a task for selected workspace", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([]);
  (apiClient.createTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "T",
    prompt: "do it",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "queued", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.listTasks as any).mockResolvedValueOnce([]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "T",
    prompt: "do it",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "queued", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  const prompt = wrapper.get('[data-testid="workbench-prompt"]');
  await prompt.setValue("do it");

  await wrapper.get('[data-testid="workbench-queue"]').trigger("click");
  await flushPromises();

  expect(apiClient.createTask).toHaveBeenCalledWith(
    expect.objectContaining({ workspace: "/tmp/wsA", prompt: "do it" }),
  );

  wrapper.unmount();
});

it("selects persisted workspace on mount when available", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsB"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "running", created_at: new Date().toISOString() },
      ],
    },
    {
      id: "t2",
      user_id: "local",
      workspace: "/tmp/wsB",
      title: "B1",
      prompt: "do B",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "queued", created_at: new Date().toISOString() },
      ],
    },
  ]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  const wsItems = wrapper.findAll('[data-testid="workspace-item"]');
  const wsB = wsItems.find((w) => w.text().includes("/tmp/wsB"));
  expect(wsB, "expected /tmp/wsB workspace item").toBeTruthy();
  expect(wsB!.classes()).toContain("ring-1");

  wrapper.unmount();
});

it("queues a task on cmd/ctrl+enter in the prompt", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([]);
  (apiClient.createTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "T",
    prompt: "do it",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "queued", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.listTasks as any).mockResolvedValueOnce([]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "T",
    prompt: "do it",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "queued", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  const textarea = wrapper.get('[data-testid="workbench-prompt"]');
  await textarea.setValue("do it");

  await textarea.trigger("keydown", { key: "Enter", metaKey: true });
  await flushPromises();

  expect(apiClient.createTask).toHaveBeenCalledWith(
    expect.objectContaining({ workspace: "/tmp/wsA", prompt: "do it" }),
  );

  wrapper.unmount();
});

it("shows guided empty state when no task is selected", async () => {
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  expect(wrapper.find('[data-testid="workbench-empty-focus"]').exists()).toBe(
    true,
  );

  const textarea = wrapper.get('[data-testid="workbench-prompt"]');
  const focusSpy = vi.spyOn(textarea.element as HTMLTextAreaElement, "focus");
  await wrapper.get('[data-testid="workbench-empty-focus"]').trigger("click");

  expect(focusSpy).toHaveBeenCalled();

  wrapper.unmount();
});

it("shows updates when a task finishes after baseline", async () => {
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "running", created_at: new Date().toISOString() },
      ],
    },
  ]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  // Manual refresh returns succeeded; updates should appear.
  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        {
          id: "a1",
          status: "succeeded",
          created_at: new Date().toISOString(),
          finished_at: new Date().toISOString(),
        },
      ],
    },
  ]);

  await wrapper.get('[data-testid="task-workbench-refresh"]').trigger("click");
  await flushPromises();

  expect(wrapper.find('[data-testid=\"task-updates\"]').exists()).toBe(true);
  expect(wrapper.text()).toContain("更新");
  expect(wrapper.text()).toContain("succeeded");

  wrapper.unmount();
});

it("opens a modal to preview artifact content from advanced tab", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "failed", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      {
        id: "a1",
        status: "failed",
        created_at: new Date().toISOString(),
        findings_path: "/tmp/wsA/.oneagent/FINDINGS.md",
      },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);
  (apiClient.getTaskAttemptArtifact as any).mockResolvedValueOnce({
    path: "/tmp/wsA/.oneagent/FINDINGS.md",
    content: "# Findings\n\nHello\n",
    truncated: false,
  });

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  await wrapper
    .get('[data-testid="workbench-details-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  await wrapper.get('[data-testid="artifact-card-findings"]').trigger("click");
  await flushPromises();

  expect(apiClient.getTaskAttemptArtifact).toHaveBeenCalledWith(
    "t1",
    "a1",
    "findings",
  );
  expect(wrapper.find('[data-testid="artifact-modal"]').exists()).toBe(true);
  expect(wrapper.text()).toContain("# Findings");

  wrapper.unmount();
});

it("opens a modal to preview artifact manifest from advanced tab", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "failed", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      {
        id: "a1",
        status: "failed",
        created_at: new Date().toISOString(),
        artifact_manifest_path: "/tmp/wsA/.oneagent/artifact_manifest.v1.json",
      },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);
  (apiClient.getTaskAttemptArtifact as any).mockResolvedValueOnce({
    path: "/tmp/wsA/.oneagent/artifact_manifest.v1.json",
    content: "{\n  \"version\": \"v1\"\n}\n",
    truncated: false,
  });

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  await wrapper
    .get('[data-testid="workbench-details-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  await wrapper
    .get('[data-testid="artifact-card-artifact_manifest"]')
    .trigger("click");
  await flushPromises();

  expect(apiClient.getTaskAttemptArtifact).toHaveBeenCalledWith(
    "t1",
    "a1",
    "artifact_manifest",
  );
  expect(wrapper.find('[data-testid="artifact-modal"]').exists()).toBe(true);
  expect(wrapper.text()).toContain("\"version\": \"v1\"");

  wrapper.unmount();
});

it("shows newest events first", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "running", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "running", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([
    {
      ts: "2020-01-01T00:00:00.000Z",
      task_id: "t1",
      type: "task.created",
      message: "old event",
    },
    {
      ts: "2020-01-01T00:00:01.000Z",
      task_id: "t1",
      type: "attempt.running",
      message: "new event",
    },
  ]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  const eventsTab = wrapper
    .findAll("button")
    .find((b) => b.text().trim() === "事件");
  expect(eventsTab, "expected 事件 tab button").toBeTruthy();

  await eventsTab!.trigger("click");
  await flushPromises();

  const viewer = wrapper.findComponent({ name: "EventLogViewer" });
  expect(viewer.exists()).toBe(true);
  const evs = viewer.props("events") as any[];
  expect(evs).toHaveLength(2);
  expect(evs[0].message).toBe("new event");
  expect(evs[1].message).toBe("old event");

  wrapper.unmount();
});

it("keeps advanced composer fields collapsed by default", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  expect(
    wrapper.find('[data-testid="workbench-composer-advanced"]').exists(),
  ).toBe(false);

  await wrapper
    .get('[data-testid="workbench-composer-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  expect(
    wrapper.find('[data-testid="workbench-composer-advanced"]').exists(),
  ).toBe(true);

  wrapper.unmount();
});

it("shows project scripts info in advanced details", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "failed", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      {
        id: "a1",
        status: "failed",
        created_at: new Date().toISOString(),
        project_config_path: "/tmp/wsA/.oneagent/project.json",
        setup_script_log_path: "/tmp/logs/setup_script.log",
      },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  await wrapper
    .get('[data-testid="workbench-details-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  expect(
    wrapper.find('[data-testid="workbench-details-advanced"]').exists(),
  ).toBe(true);
  expect(wrapper.text()).toContain("/tmp/wsA/.oneagent/project.json");
  expect(wrapper.text()).toContain("/tmp/logs/setup_script.log");

  wrapper.unmount();
});

it("loads review artifacts and submits a review comment", async () => {
  const store = new Map<string, string>([["oneagent-workspace", "/tmp/wsA"]]);
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
  });

  const { default: TaskWorkbench } = await import("@/views/TaskWorkbench.vue");

  (apiClient.listTasks as any).mockResolvedValueOnce([
    {
      id: "t1",
      user_id: "local",
      workspace: "/tmp/wsA",
      title: "A1",
      prompt: "do A",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      attempts: [
        { id: "a1", status: "succeeded", created_at: new Date().toISOString() },
      ],
    },
  ]);
  (apiClient.getTask as any).mockResolvedValueOnce({
    id: "t1",
    user_id: "local",
    workspace: "/tmp/wsA",
    title: "A1",
    prompt: "do A",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    attempts: [
      { id: "a1", status: "succeeded", created_at: new Date().toISOString() },
    ],
  });
  (apiClient.getTaskEvents as any).mockResolvedValueOnce([]);
  (apiClient.getTaskAttemptDiffPatch as any).mockResolvedValueOnce({
    path: "/tmp/diff.patch",
    content: "diff --git a/a b/a\n",
    truncated: false,
  });
  (apiClient.getTaskAttemptChangedFiles as any).mockResolvedValueOnce({
    path: "/tmp/changed_files.txt",
    content: "a\n",
    truncated: false,
  });
  (apiClient.listTaskAttemptReviewComments as any).mockResolvedValueOnce([]);
  (apiClient.postTaskAttemptReviewComment as any).mockResolvedValueOnce({
    ts: new Date().toISOString(),
    principal_id: "local",
    task_id: "t1",
    attempt_id: "a1",
    comment: "LGTM",
  });

  const wrapper = shallowMount(TaskWorkbench);
  await flushPromises();

  await wrapper.get('[data-testid="workbench-task-item"]').trigger("click");
  await flushPromises();

  await wrapper.get('[data-testid="workbench-review-toggle"]').trigger("click");
  await flushPromises();

  expect(apiClient.getTaskAttemptDiffPatch).toHaveBeenCalledWith("t1", "a1");
  expect(apiClient.getTaskAttemptChangedFiles).toHaveBeenCalledWith("t1", "a1");
  expect(apiClient.listTaskAttemptFiles).toHaveBeenCalledWith("t1", "a1");

  await wrapper.get('[data-testid="review-comment-input"]').setValue("LGTM");
  await wrapper.get('[data-testid="review-comment-submit"]').trigger("click");
  await flushPromises();

  expect(apiClient.postTaskAttemptReviewComment).toHaveBeenCalledWith(
    "t1",
    "a1",
    expect.objectContaining({ comment: "LGTM" }),
  );

  wrapper.unmount();
});
