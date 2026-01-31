## plan 工具使用说明（简版）
- 维护工作计划文件：`$WORKSPACE_ROOT/.oneagent/PLAN.md`。
- 入参（常用）：`{ action: init|get|upsert_task|delete_task|mark_done, task_id?, title?, status?, scope?, acceptance?, template?, overwrite? }`。
- `upsert_task`：新增/更新任务（不填 task_id 则从 title 生成）；建议同时补充 scope/acceptance。
- `mark_done` 会做只读验收，通过才标记 done；失败会返回原因并不写回。
