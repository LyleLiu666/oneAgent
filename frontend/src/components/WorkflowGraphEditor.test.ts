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

it("edits node principal/model/skills", async () => {
  const { default: WorkflowGraphEditor } = await import(
    "@/components/WorkflowGraphEditor.vue"
  );

  const wrapper = mount(WorkflowGraphEditor, {
    props: {
      modelValue: {
        nodes: [{ node_id: "n1", title: "", prompt: "" }],
        edges: [],
      },
    },
  });

  const syncProp = async () => {
    const emitted = wrapper.emitted("update:modelValue");
    const latest = emitted?.[emitted.length - 1]?.[0] as any;
    await wrapper.setProps({ modelValue: latest });
  };

  await wrapper.find('[data-testid="workflow-node-n1-principal"]').setValue("alice");
  await syncProp();
  await wrapper.find('[data-testid="workflow-node-n1-model"]').setValue("model-1");
  await syncProp();
  await wrapper.find('[data-testid="workflow-node-n1-skills"]').setValue("s1, s2");
  await syncProp();

  const emitted = wrapper.emitted("update:modelValue");
  expect(emitted).toBeTruthy();

  const latest = emitted?.[emitted.length - 1]?.[0] as any;
  const node = latest.nodes.find((n: any) => n.node_id === "n1");
  expect(node.principal_id).toBe("alice");
  expect(node.model_id).toBe("model-1");
  expect(node.skills).toEqual(["s1", "s2"]);
});
