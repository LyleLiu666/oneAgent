## subagent 工具使用说明（简版）
- 启动子 Agent 执行独立子任务；必填 `task`。
- 可选：`context_summary`、`tool_ids`（限制工具集）、`scope`（限制可写路径）、`max_steps/max_runtime_seconds`、`skill_ids/k_skills`。
- 输出：`summary` + `findings_path/trace_log_path`（主线程用于复查与继续）。
