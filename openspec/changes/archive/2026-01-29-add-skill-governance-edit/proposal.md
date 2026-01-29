# Change: Add skill governance editing (with OCC)

## Why
归档只是治理的一部分；更常见的治理动作是“合并/去重/修订”。为了让用户能在产品内完成治理闭环，需要支持查看与编辑个人技能的 `SKILL.md`，并用 OCC 保护写入，避免长时任务/并发导致的漂移覆盖。

## What Changes
- Backend 增加 skills 详情与编辑 API（仅限 oneAgent personal skills）：
  - `GET /api/skills/:id` 返回 skill 元信息 + `skill_md` + `sha256`
  - `PUT /api/skills/:id` 更新 `SKILL.md`（支持 `expected_sha256` OCC）
- Skill Governance UI 支持：
  - 查看/编辑 personal skills
  - 保存时带 `expected_sha256`，冲突时提示重新刷新

## Impact
- Affected specs: `system-skill-management`, `skill-recall`
- Affected code: `backend/internal/handler/skills.go`, `frontend/src/views/SkillGovernance.vue`, `frontend/src/api/client.ts`

