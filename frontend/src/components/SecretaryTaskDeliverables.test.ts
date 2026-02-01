// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

const mocks = vi.hoisted(() => {
  return {
    listTasks: vi.fn(async () => []),
    getTaskAttemptArtifact: vi.fn(async () => ({
      path: "findings.md",
      content: "# Findings\n- ok\n",
      truncated: false,
    })),
  };
});

vi.mock("@/api/client", () => ({
  listTasks: mocks.listTasks,
  getTaskAttemptArtifact: mocks.getTaskAttemptArtifact,
}));

it("renders deliverable cards for completed task artifacts", async () => {
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

