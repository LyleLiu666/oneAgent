## Context
系统目前缺少安全、可回退的“删除文件”能力。直接开放 `rm`（通过命令工具）会引入更大的不可逆风险与权限治理复杂度。

## Goals / Non-Goals
- Goals:
  - 允许 agent 执行“删除”语义，同时保持可恢复（软删除）。
  - 不依赖 `bash/run_command` 来完成删除，避免扩大命令权限面。
  - 回收站条目自动清理，避免无限增长。
- Non-Goals:
  - 与操作系统原生回收站/Trash 深度集成。
  - 提供类似 git 的多版本时间线能力（本变更仅提供 move-to-trash + retention）。

## Decisions
- 回收站位置：`<workspace>/.oneagent/trash/`（workspace 内、系统管理目录）。
- 条目结构：每次 trash 生成一个独立条目目录 `<id>/`，包含 `manifest.json` 与 payload（文件或目录）。
- 权限与安全：
  - 对用户输入的 `filePath` 进行 workspace-aware 解析，拒绝越界路径与 symlink 逃逸。
  - `file_scope` 等权限约束用于限制“被删除的目标路径”；回收站目录为系统内部路径，允许写入以完成软删除（仍限制在 workspace 内）。
- 清理策略：按条目年龄清理（retention=7 days）。清理触发为 best-effort：启动时与定期循环执行（并可在 `trash_file` 调用中顺带触发）。

## Risks / Trade-offs
- 若回收站目录写入不受 `file_scope` 约束，可能与“严格最小权限”冲突；但回收站仅在 workspace 内且用于回滚能力，属于可接受的系统内部写入。
- 软删除可能改变一些工具/流程对文件存在性的假设；需要明确错误与可恢复路径。

## Migration Plan
- 新增工具不影响现有行为；仅当 agent 调用 `trash_file` 时生效。

## Open Questions
- 是否需要同时提供 `restore_file`/`list_trash` 工具，以便 agent 在误删时自助恢复（或先仅支持人工恢复）？

