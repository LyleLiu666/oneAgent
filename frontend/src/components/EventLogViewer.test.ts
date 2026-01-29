// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";

import type { TaskEvent } from "@/api/client";

it("filters by search and expands event details", async () => {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn(async () => {}) },
    configurable: true,
  });

  const { default: EventLogViewer } = await import(
    "@/components/EventLogViewer.vue"
  );

  const events: TaskEvent[] = [
    {
      ts: "2026-01-29T00:00:01.000Z",
      task_id: "t1",
      attempt_id: "a1",
      type: "attempt.running",
      message: "running",
      data: { step: 1 },
    },
    {
      ts: "2026-01-29T00:00:00.000Z",
      task_id: "t1",
      attempt_id: "a1",
      type: "task.created",
      message: "created",
    },
  ];

  const wrapper = mount(EventLogViewer, {
    props: {
      events,
      loading: false,
      refreshing: true,
    },
  });

  expect(wrapper.get('[data-testid="event-viewer-refreshing"]').exists()).toBe(
    true,
  );

  await wrapper.get('[data-testid="event-viewer-search"]').setValue("running");

  const items = wrapper.findAll('[data-testid="event-viewer-item"]');
  expect(items).toHaveLength(1);
  expect(wrapper.text()).toContain("attempt.running");

  await wrapper.get('[data-testid="event-viewer-item-toggle"]').trigger("click");
  expect(
    wrapper.find('[data-testid="event-viewer-item-details"]').exists(),
  ).toBe(true);
  expect(wrapper.text()).toContain("attempt_id");
  expect(wrapper.text()).toContain("a1");
  expect(wrapper.text()).toContain('"step": 1');

  wrapper.unmount();
});

it("copies filtered events", async () => {
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });

  const { default: EventLogViewer } = await import(
    "@/components/EventLogViewer.vue"
  );

  const events: TaskEvent[] = [
    {
      ts: "2026-01-29T00:00:01.000Z",
      task_id: "t1",
      attempt_id: "a1",
      type: "attempt.running",
      message: "running",
      data: { step: 1 },
    },
  ];

  const wrapper = mount(EventLogViewer, {
    props: { events },
  });

  await wrapper.get('[data-testid="event-viewer-copy"]').trigger("click");
  expect(writeText).toHaveBeenCalledTimes(1);
  expect(String(writeText.mock.calls[0][0])).toContain("attempt.running");

  wrapper.unmount();
});

