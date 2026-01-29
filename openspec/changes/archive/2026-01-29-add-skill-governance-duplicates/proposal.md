# Change: Add skill governance duplicates workbench

## Why
当前系统按 `skill name`（normalized id）做去重并按优先级只保留“最终生效版本”。这对运行时召回是正确的，但对治理是盲区：
- 同名冲突/覆盖发生时，用户看不到“哪些版本被 shadowed”
- 用户无法快速定位“应该 archive 哪一个版本”来解除污染或切换生效版本

因此需要一个最小可交付的“duplicates 工作台”：把所有候选版本都展示出来，并标记哪一个是最终生效版本。

## What Changes
- 新增 API：列出所有同名冲突的 skill candidates，并标记 `effective`
- Skill Governance 页面新增 “Duplicates” 区域，用于查看/治理冲突（复用现有 archive 能力）

## Impact
- Affected specs: `system-skill-management`
- Affected code: `backend/internal/skill/*`, `backend/internal/handler/*`, `backend/internal/server/server.go`, `frontend/src/views/*`, `frontend/src/api/*`

