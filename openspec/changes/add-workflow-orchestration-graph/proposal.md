# Change: Add workflow orchestration graph (nodes=work-style agents, artifacts=file sets)

## Why
当前 oneAgent 主要以“单任务/多 attempt”的方式交付。长期愿景（见 `openspec/project.md` / `docs/ux-vision.md`）需要走向**工业级 agent 平台**：像 FastGPT / n8n 一样可编排工作流，但每个节点不是函数/插件，而是类似 Codex/Claude Code 的“工作型 agent”，节点间的交付物可能是**多个文件**。

这类能力一旦缺少“显式图”的表达，就会出现：
- 依赖关系隐式、不可回放 → 难以规模化复用
- 无法并行推进多条工作线 → “秘书”难以承接多线程委托
- 交付/验收边界不清 → 容易卡死在“看起来差不多”的主观判断里

## What Changes
- 新增 OpenSpec capability：`system-workflow-orchestration`
  - workflow graph：node/edge/versioning/run snapshot
  - node execution：节点=工作型 agent；交付物=文件集（artifact manifest）
  - completion：Hard Gate（客观可校验）+ Soft Gate（按 rubric 的模型评分）
  - observability：每个 node/run 的事件与日志可追溯、可续跑

## Impact
- Affected specs: `system-workflow-orchestration` (new)
- Affected code (expected): `backend/internal/workflow/*`, `backend/internal/handler/*`, `frontend/src/views/*`
- Related docs: `openspec/project.md`, `docs/ux-vision.md`, `docs/roadmap-longterm.md`, `docs/opensource/README.md`

