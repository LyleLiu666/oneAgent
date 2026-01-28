# Change: Add skill governance workbench

## Why
SOP suggestions 只是“候选资产”。一旦用户 approve 并 materialize 为 `SKILL.md`，还需要一套治理能力来支撑长期使用：
- 资产列表可见（来源/路径/描述）
- 去重与合并（后续迭代）
- 过时 drop / archive（召回时不再污染）

本 change 先做最小可交付：**列出技能 + 支持归档（archive）**，为后续 merge/dedupe 打基础。

## What Changes
- 新增 Skill Governance Workbench（治理工作台）
  - 列出当前可发现的 skills（按 `skill.Discover` 的优先级）
  - 支持对 oneAgent personal skills 执行 archive（移动到 `skills-archived/`，不再被 discover/recall）

## Impact
- Affected specs: `skill-recall`, `system-skill-management`
- Affected code: `backend/internal/handler/*`, `backend/internal/server/server.go`, `backend/internal/skill/*`, `frontend/src/views/*`

