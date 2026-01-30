## plan 工具使用说明（简版）
- 维护工作计划文件：`$WORKSPACE_ROOT/.oneagent/PLAN.md`。
- 入参：`{ action: init|get|mark_done, task_id?, template?, overwrite? }`。
- `mark_done` 会做只读验收，通过才标记 done；失败会返回原因并不写回。
