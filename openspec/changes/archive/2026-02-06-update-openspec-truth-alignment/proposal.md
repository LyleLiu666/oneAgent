# Change: Align OpenSpec source-of-truth with executable status

## Why
当前 OpenSpec 存在“文档状态与真实状态漂移”的风险：部分 capability `Purpose` 仍为 `TBD`，roadmap 快照也可能与 `openspec list` 输出不一致。这个问题会直接降低后续 change 拆解和执行质量。

## What Changes
- 为 OpenSpec 增加“真相对齐”治理约束：
  - capability spec 不允许保留 `Purpose: TBD`（新增校验）
  - roadmap 快照必须可由命令输出回放/校验（新增校验）
- 在项目工具链中加入 OpenSpec 结构化校验入口（本地 + CI）
- 补齐本轮涉及 capability 的 `Purpose` 与状态来源说明（避免人工口径漂移）

## Impact
- Affected specs:
  - `project-tooling`
- Affected docs:
  - `openspec/roadmap-longterm.md`
  - `openspec/roadmap-secretary-first.md`
- Affected code/scripts:
  - `scripts/` 下新增或扩展 OpenSpec 校验脚本
  - CI workflow（如需）增加 OpenSpec truth check 步骤
