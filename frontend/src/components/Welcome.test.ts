// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { shallowMount } from "@vue/test-utils";

vi.mock("@/composables/useAuth", () => ({
  useAuth: () => ({
    user: {
      name: "测试用户",
      username: "tester",
    },
  }),
}));

it("shows a manual-path hint and disables picker when chooser is unsupported", async () => {
  const { default: Welcome } = await import("@/components/Welcome.vue");

  const wrapper = shallowMount(Welcome, {
    props: {
      showWorkspacePrompt: true,
      workspaceChooserSupported: false,
      workspaceChooserHint:
        "当前服务端环境不支持原生文件夹选择，请手动填写服务端工作区路径。",
    },
  });

  expect(
    wrapper.get('[data-testid="welcome-workspace-chooser-hint"]').text(),
  ).toContain("手动填写服务端工作区路径");
  expect(
    wrapper
      .get('[data-testid="welcome-workspace-choose"]')
      .attributes("disabled"),
  ).toBeDefined();
});

it("keeps the chooser enabled when browser strategy is available", async () => {
  const { default: Welcome } = await import("@/components/Welcome.vue");

  const wrapper = shallowMount(Welcome, {
    props: {
      showWorkspacePrompt: true,
      workspaceChooserSupported: false,
      workspaceChooserHint:
        "当前服务端环境不支持原生文件夹选择，请手动填写服务端工作区路径。",
      workspaceChooserStrategy: "browser",
    },
  });

  expect(
    wrapper.find('[data-testid="welcome-workspace-chooser-hint"]').exists(),
  ).toBe(false);
  expect(
    wrapper
      .get('[data-testid="welcome-workspace-choose"]')
      .attributes("disabled"),
  ).toBeUndefined();

  await wrapper
    .get('[data-testid="welcome-workspace-choose"]')
    .trigger("click");
  expect(wrapper.emitted("choose-workspace")).toHaveLength(1);
});
