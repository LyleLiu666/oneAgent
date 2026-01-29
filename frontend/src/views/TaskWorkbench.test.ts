// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { shallowMount } from "@vue/test-utils";

import * as apiClient from "@/api/client";

vi.mock("@/api/client", () => ({
  listTasks: vi.fn(async () => []),
  createTask: vi.fn(async () => ({})),
  getTask: vi.fn(async () => ({})),
  getTaskAttemptDiffPatch: vi.fn(async () => ({})),
  getTaskAttemptChangedFiles: vi.fn(async () => ({})),
  getTaskEvents: vi.fn(async () => []),
  cancelTask: vi.fn(async () => ({})),
  resumeTask: vi.fn(async () => ({})),
  listTaskAttemptReviewComments: vi.fn(async () => []),
  postTaskAttemptReviewComment: vi.fn(async () => ({})),
}));

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

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
