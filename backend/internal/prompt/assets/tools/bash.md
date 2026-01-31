## bash 工具使用说明（简版）
- 不允许 heredoc（`<<`）；不要使用重定向写文件。
- 路径必须在 `$BASH_ROOT_DIR` 内（`/dev/null` 例外）。
- 禁止大量系统命令（例如 `sudo/apt-get/pip/node/npm/find/touch` 等）。
- 写文件用 `write_file`；改文件用 `edit`/`edit_v2`；不要用 bash 写文件。
- 读文件/日志不要 `cat` 整个文件：优先 `read_file`（分页 + 限流）或 `rg` 定位后再分段读取；bash 输出过长会被截断（`stdout_truncated/stderr_truncated`），需要长输出用 `run_command` 分段拉取。
