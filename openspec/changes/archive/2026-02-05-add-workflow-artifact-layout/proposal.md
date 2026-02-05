# Change: Add workflow artifact layout (path-only handoffs)

## Why
当前 workflow 的节点交接已经有 `artifact manifest` 的概念，但缺少一个**稳定、可预测、可持久化**的落盘位置与约定，导致：
- 节点间交付很难“只传路径”（容易退化为把内容塞进 prompt）
- 一旦引入 worktree/临时执行目录，交付文件路径会失效（影响续跑/回放）
- 用户难以回溯“每个 agent 做了什么、交付了什么、依据是什么”

## What Changes
- 定义 workflow_run / node_run 的 **统一文件布局（artifact layout）**：包含流水账、findings（含交付清单）、交付文件、trace/events、gate reports。
- 明确 **path-only handoff**：节点间仅传交付物路径（含 manifest 路径），不传文件内容。
- 增加 **用户可配置项（best-effort）**：
  - artifact 根目录、保留策略
  - 每个节点的 skills/model/principal（节点执行配置）
  - publish/export 由 agent 决定（系统仅做证据链 export）

## Impact
- Affected specs: `system-workflow-orchestration` (modified)
- Affected code (planned): `backend/internal/workflow/*`, `backend/internal/handler/*`, `frontend/src/components/WorkflowRunNodes.vue`
