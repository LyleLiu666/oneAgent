// @vitest-environment jsdom

import { expect, it } from "vitest";
import { mount } from "@vue/test-utils";

it("renders node status and artifacts", async () => {
  const { default: WorkflowRunNodes } = await import(
    "@/components/WorkflowRunNodes.vue"
  );

  const wrapper = mount(WorkflowRunNodes, {
    props: {
      run: {
        run_id: "r1",
        workflow_id: "w1",
        version_id: "v1",
        workspace_root: "/tmp/ws",
        graph_snapshot: {
          nodes: [{ node_id: "A", title: "Node A", prompt: "" }],
          edges: [],
        },
        node_runs: {
          A: {
            node_id: "A",
            status: "succeeded",
            artifacts: { artifacts: [{ path: "out/result.txt", kind: "file" }] },
          },
        },
        status: "running",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    },
  });

  expect(wrapper.text()).toContain("Node A");
  expect(wrapper.text()).toContain("succeeded");
  expect(wrapper.text()).toContain("out/result.txt");
});

