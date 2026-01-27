# Change: Add OCC auto-preconditions and tool permissions

## Why
在长时运行/任务队列/未来多 workspace 并行的目标下，工具层需要具备“读到的版本 == 写回的版本”的一致性保证，以及更强的工具权限控制，避免破坏性命令带来不可逆损失。

## What Changes
- 在 `local-runtime` 增加 **L2 OCC 自动化闭环**：`read_file` 记录文件版本指纹，`write_file`/`edit` 在未显式提供 `preconditions` 时自动注入 `expected_sha256`，并在写入成功后更新指纹，避免“自我冲突”。
- 增加 **工具权限控制（最小可交付）**：
  - 通过环境变量禁用指定 tool（例如禁用 `bash`、`rg`）。
  - `bash` 默认拒绝执行包含 `rm` 的命令（可通过开关放开）。

## Impact
- Affected specs: `local-runtime`
- Affected code: `backend/internal/tool/*`, `backend/internal/handler/chat.go`, `backend/internal/server/taskqueue_runner.go`
- Compatibility: 默认启用 OCC 自动化；如遇问题可通过环境变量关闭/绕过。

