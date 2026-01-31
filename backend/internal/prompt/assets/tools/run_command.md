## run_command 工具使用说明（简版）
- 异步执行命令：`action=start|poll|cancel`；`start` 返回 `job_id`，`poll` 用 offset 分段取输出。
- 适用：长任务/大输出/需要持续轮询；短命令可用 `bash`。
- 默认在 **workspace 根目录**执行（无需 `cd`），优先用相对路径。
- 禁止：在 command 里用 heredoc/重定向写文件；写文件用 `write_file`，改文件用 `edit/edit_v2`。
- stdout/stderr 会分别分段返回，不需要 `2>&1`（允许但通常没必要）。
