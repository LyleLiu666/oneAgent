## bash 工具使用说明（简版）
- bash 默认在 **workspace 根目录**执行（无需 `cd`），优先使用相对路径。
- 不允许 heredoc（`<<`）；不允许变量/命令替换（`$FOO` / `` `cmd` `` / `$(cmd)`）。
- 路径默认必须在 workspace 根目录内（`/dev/null` 仅作为重定向目标例外）；不要访问 `/dev/null/...` 这类路径。
- **不要用重定向写文件**：写文件用 `write_file`；改文件用 `edit`/`edit_v2`。
- stdout/stderr 会分别返回，不需要 `2>&1`（允许但通常没必要）。
- 命令允许范围由 `command_profile` 决定：`readonly/dev/coding/system_install/full`；其中系统安装命令（`brew/apt/...`）属于高风险，会触发审批。
- 读文件/日志不要 `cat` 整个文件：优先 `read_file`（分页 + 限流）或 `rg` 定位后再分段读取；bash 输出过长会被截断（`stdout_truncated/stderr_truncated`），需要长输出用 `run_command` 分段拉取。
