// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { shallowMount } from "@vue/test-utils";

import * as apiClient from "@/api/client";

vi.mock("@/api/client", () => ({
  listSkills: vi.fn(async () => []),
  listSkillDuplicates: vi.fn(async () => []),
  listStaleSkills: vi.fn(async () => []),
  archiveSkill: vi.fn(async () => ({})),
  pinSkillCandidate: vi.fn(async () => ({})),
  archiveShadowedPersonalDuplicates: vi.fn(async () => ({})),
  getSkill: vi.fn(async () => ({})),
  readSkillFile: vi.fn(async () => ({})),
  updateSkill: vi.fn(async () => ({})),
}));

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

it("loads skills and renders list", async () => {
  const { default: SkillGovernance } =
    await import("@/views/SkillGovernance.vue");

  (apiClient.listSkills as any).mockResolvedValueOnce([
    {
      skill_id: "a",
      name: "A",
      description: "d",
      source: ".oneagent",
      path: "/tmp/a/SKILL.md",
      archivable: true,
    },
  ]);

  const wrapper = shallowMount(SkillGovernance);
  await flushPromises();

  expect(apiClient.listSkills).toHaveBeenCalled();
  expect((apiClient as any).listSkillDuplicates).toHaveBeenCalled();
  expect((apiClient as any).listStaleSkills).toHaveBeenCalled();
  expect(wrapper.text()).toContain("A");
  expect(wrapper.find('[data-testid="skill-archive"]').exists()).toBe(true);

  wrapper.unmount();
});

it("loads skill details when selected and saves with OCC", async () => {
  const { default: SkillGovernance } =
    await import("@/views/SkillGovernance.vue");

  (apiClient.listSkills as any).mockResolvedValueOnce([
    {
      skill_id: "a",
      name: "A",
      description: "d",
      source: ".oneagent",
      path: "/tmp/a/SKILL.md",
      archivable: true,
    },
  ]);
  (apiClient.getSkill as any).mockResolvedValueOnce({
    skill_id: "a",
    name: "A",
    description: "d",
    source: ".oneagent",
    path: "/tmp/a/SKILL.md",
    archivable: true,
    sha256: "abc",
    skill_md: "old",
  });
  (apiClient.updateSkill as any).mockResolvedValueOnce({
    skill_id: "a",
    name: "A",
    description: "d",
    source: ".oneagent",
    path: "/tmp/a/SKILL.md",
    archivable: true,
    sha256: "def",
    skill_md: "new",
  });

  const wrapper = shallowMount(SkillGovernance);
  await flushPromises();

  expect((apiClient as any).listSkillDuplicates).toHaveBeenCalled();
  expect((apiClient as any).listStaleSkills).toHaveBeenCalled();

  await wrapper.get('[data-testid="skill-item"]').trigger("click");
  await flushPromises();

  const textarea = wrapper.get("textarea");
  await textarea.setValue("new");

  await wrapper.get('[data-testid="skill-save"]').trigger("click");
  await flushPromises();

  expect(apiClient.updateSkill).toHaveBeenCalledWith(
    expect.objectContaining({ skill_id: "a", expected_sha256: "abc" }),
  );

  wrapper.unmount();
});

it("pins a candidate from duplicates list", async () => {
  const { default: SkillGovernance } =
    await import("@/views/SkillGovernance.vue");

  (apiClient.listSkills as any).mockResolvedValueOnce([]);
  (apiClient.listSkillDuplicates as any).mockResolvedValueOnce([
    {
      skill_id: "foo",
      candidates: [
        {
          skill_id: "foo",
          name: "foo",
          description: "from claude",
          source: ".claude",
          path: "/tmp/.claude/skills/foo/SKILL.md",
          archivable: false,
          effective: false,
          precedence_rank: 2,
        },
        {
          skill_id: "foo",
          name: "foo",
          description: "personal",
          source: ".oneagent",
          path: "/tmp/.oneagent/skills/foo/SKILL.md",
          archivable: true,
          effective: true,
          precedence_rank: 3,
        },
      ],
    },
  ]);

  const wrapper = shallowMount(SkillGovernance);
  await flushPromises();

  expect(
    wrapper.find('[data-testid="skill-governance-advanced"]').exists(),
  ).toBe(false);

  await wrapper
    .get('[data-testid="skill-governance-advanced-toggle"]')
    .trigger("click");
  await flushPromises();

  expect(
    wrapper.find('[data-testid="skill-governance-advanced"]').exists(),
  ).toBe(true);

  await wrapper.get('[data-testid="duplicate-pin"]').trigger("click");
  await flushPromises();

  expect((apiClient as any).pinSkillCandidate).toHaveBeenCalledWith(
    "foo",
    expect.objectContaining({
      source: ".claude",
      path: "/tmp/.claude/skills/foo/SKILL.md",
    }),
  );

  wrapper.unmount();
});

it("allows viewing non-editable skill files", async () => {
  const { default: SkillGovernance } =
    await import("@/views/SkillGovernance.vue");

  (apiClient.listSkills as any).mockResolvedValueOnce([
    {
      skill_id: "x",
      name: "X",
      description: "d",
      source: ".claude",
      path: "/tmp/.claude/skills/x/SKILL.md",
      archivable: false,
    },
  ]);

  (apiClient.getSkill as any).mockResolvedValueOnce({
    skill_id: "x",
    name: "X",
    description: "d",
    source: ".claude",
    path: "/tmp/.claude/skills/x/SKILL.md",
    archivable: false,
    sha256: "abc",
    skill_md: "# Main",
    files: ["SKILL.md", "references/notes.md"],
  });

  (apiClient.readSkillFile as any).mockResolvedValueOnce({
    path: "references/notes.md",
    sha256: "def",
    content: "notes",
  });

  const wrapper = shallowMount(SkillGovernance);
  await flushPromises();

  await wrapper.get('[data-testid="skill-item"]').trigger("click");
  await flushPromises();

  expect(wrapper.text()).toContain("不可编辑");
  expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toContain(
    "# Main"
  );

  await wrapper
    .get('[data-testid="skill-file-select"]')
    .setValue("references/notes.md");
  await flushPromises();

  expect((apiClient as any).readSkillFile).toHaveBeenCalledWith(
    "x",
    "references/notes.md"
  );
  expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toContain(
    "notes"
  );

  wrapper.unmount();
});

it("shows stale skills and can archive with reason", async () => {
  const { default: SkillGovernance } =
    await import("@/views/SkillGovernance.vue");

  (apiClient.listSkills as any).mockResolvedValueOnce([]);
  (apiClient.listSkillDuplicates as any).mockResolvedValueOnce([]);
  (apiClient as any).listStaleSkills.mockResolvedValueOnce([
    {
      skill_id: "stale-1",
      name: "Stale 1",
      description: "d",
      source: ".oneagent",
      path: "/tmp/stale-1/SKILL.md",
      archivable: true,
      used_count: 0,
      last_activity_at: "2026-01-01T00:00:00Z",
      stale_reason: "inactive",
      stale_threshold_days: 30,
    },
  ]);

  const wrapper = shallowMount(SkillGovernance);
  await flushPromises();

  await wrapper.get('[data-testid="skill-governance-stale-toggle"]').trigger("click");
  await flushPromises();

  expect(wrapper.findAll('[data-testid="stale-skill-item"]').length).toBe(1);

  await wrapper.get('[data-testid="stale-skill-archive"]').trigger("click");
  await flushPromises();

  expect(apiClient.archiveSkill).toHaveBeenCalledWith(
    "stale-1",
    expect.objectContaining({ reason: expect.stringContaining("stale") }),
  );

  wrapper.unmount();
});
