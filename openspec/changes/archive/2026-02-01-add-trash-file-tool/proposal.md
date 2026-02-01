# Change: Add soft-delete (trash) file tool

## Why
当前系统对文件删除的能力基本等价于“不可用”：`bash/run_command` 的命令白名单不包含 `rm`，且缺少专用 `delete/trash` 工具。
这会导致 agent 无法完成常见的“清理/移除文件”任务（例如删除生成物、移除误生成文件等）。

## What Changes
- 新增一个文件工具 `trash_file`：对 workspace 内的文件/目录执行“软删除”（move-to-trash），将其移动到 workspace 内的系统回收站目录（`.oneagent/trash/`）。
- 系统自动清理回收站：回收站条目保留 7 天，超过保留期将被永久删除（best-effort）。

## Impact
- Affected specs:
  - `system-file-tools`（新增 `trash_file` 与回收站自动清理的需求）
- Affected code (expected):
  - `backend/internal/tool/*`（新增工具 + registry + safety metadata）
  - `backend/internal/handler/chat.go`（可选：按 workspaceRoot 启动后台清理循环）

## Open Questions
- “每七天自动清空一次”是否等价于“回收站条目保留 7 天，超过 7 天即清理（按年龄删）”？还是要“每 7 天全量清空（不看年龄）”？
- `trash_file` 需要支持目录（递归）吗？还是仅文件？

