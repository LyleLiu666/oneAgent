# Change: Add skill governance merge/pin workflow

## Why
目前已有 skill 列表/编辑/归档/duplicates 视图，但缺少“下一步治理动作”：
- 同名冲突（duplicates）只能看，缺少一键把某个版本变成 canonical 的能力
- 当冲突来自不可归档来源（例如 `~/.claude`）时，用户很难“解除污染”

需要一个最小可交付的 merge/pin 工作流：把选中的 candidate 物化为 oneAgent personal canonical（更高优先级），并可选归档旧的 personal 版本，从而让召回/推荐稳定收敛。

## What Changes
- 新增 skill governance 动作：
  - `pin`：将某 candidate 复制为 oneAgent personal canonical（shadow 其它来源）
  - `archive shadowed`：归档同名 personal 旧版本（可选）
- Skill Governance UI 增加 merge/pin 操作入口（基于 duplicates 视图）

## Impact
- Affected specs: `system-skill-management`
- Affected code: `backend/internal/skill/*`, `backend/internal/handler/*`, `frontend/src/views/SkillGovernance.vue`

