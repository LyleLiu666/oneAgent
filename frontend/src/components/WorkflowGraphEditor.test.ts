// @vitest-environment jsdom

import { expect, it } from "vitest";
import { mount } from "@vue/test-utils";

it("adds a node and emits updated graph", async () => {
  const { default: WorkflowGraphEditor } = await import(
    "@/components/WorkflowGraphEditor.vue"
  );

  const wrapper = mount(WorkflowGraphEditor, {
    props: { modelValue: { nodes: [], edges: [] } },
  });

  await wrapper.find('[data-testid="workflow-add-node"]').trigger("click");

  const emitted = wrapper.emitted("update:modelValue");
  expect(emitted).toBeTruthy();
  const latest = emitted?.[0]?.[0] as any;
  expect(Array.isArray(latest?.nodes)).toBe(true);
  expect(latest.nodes).toHaveLength(1);
});

