# Change: Fold low-frequency UI sections in workbenches

## Why
当前「技能治理」与「任务工作台」页面的信息密度过高：大量低频/诊断信息与高频操作混在同一视图中，导致用户在日常使用时需要在噪声中寻找入口，体验拥挤且容易误触。

我们需要引入 **Progressive Disclosure（渐进式披露）**：
- 保留最重要的高频入口（列表、选择、核心操作）
- 将低频/高级场景（重复项治理、预算、证据路径、事件流等）折叠到可展开区域

## What Changes
- 技能治理页面：
  - 默认仅展示：技能列表、选择技能、保存/归档等高频入口
  - 将重复项治理（pin / archive shadowed）与路径/sha 等诊断信息折叠为“高级”
- 任务工作台页面：
  - 默认仅展示：workspace 选择、任务入队（prompt + 按钮）、任务列表、任务状态/摘要与取消/继续
  - 将可选预算/标题、证据路径、policy snapshot、事件列表折叠为“高级”

## Impact
- Affected specs: `system-skill-management`, `system-task-queue`
- Affected code (expected): `frontend/src/views/SkillGovernance.vue`, `frontend/src/views/TaskWorkbench.vue`

