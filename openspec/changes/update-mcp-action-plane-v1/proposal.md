# Change: Extend MCP from read-only resources to policy-governed action plane

## Why
当前 MCP 更偏只读观察，难以承担“外部系统触发执行”的核心集成场景。要提升平台竞争力，需要在不破坏安全边界前提下支持可控写操作。

## What Changes
- 在 MCP 中增加受策略约束的动作能力（action plane）：
  - create task
  - resume task
  - cancel task
- 复用现有 principal auth、tool permission 与审批体系
- 统一 MCP 动作的审计字段（调用者、参数摘要、审批状态、结果引用）

## Impact
- Affected specs:
  - `system-mcp-server`
  - `system-tool-permissions`
  - `system-task-queue`
- Affected code:
  - MCP method handlers
  - approval/policy enforcement path
  - trace and task event linkage
