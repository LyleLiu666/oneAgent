## run_command 工具使用说明（简版）
- 异步执行命令：`action=start|poll|cancel`；`start` 返回 `job_id`，`poll` 用 offset 分段取输出。
- 适用：长任务/大输出/需要持续轮询；短命令可用 `bash`。
- 禁止：在 command 里用 heredoc/重定向写文件；写文件用 `write_file`，改文件用 `edit/edit_v2`。
