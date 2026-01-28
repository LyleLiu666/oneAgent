# Change: Add workspace project scripts

## Why
在真实项目里，Agent 要“开干”往往卡在环境与工作流的隐性前置上：安装依赖、启动 dev server、复制 `.env`、跑测试、清理临时文件等。

如果这些步骤每次都让 Agent 自己摸索，会导致：
- 前期时间大量浪费在重复探索（影响“进来就能干活”）
- 工作流不稳定，产物难以复现
- 证据链缺失：用户难以判断“到底跑了什么/为什么失败”

因此需要把项目级的 workflow 显性化为可配置资产，并把执行与日志纳入证据链。

## What Changes
- 在 workspace 内支持可选的 project config 文件：`.oneagent/project.json`
  - 支持声明 `setup_script` / `dev_server_script` / `cleanup_script` / `test_script`
  - 支持声明 `copy_files`（用于 attempt/worktree 场景复制 `.env` 等文件）
- Task attempt 启动前 best-effort 执行 `setup_script`，并将 stdout/stderr 记录为可追溯产物
- Task attempt 执行结束时 best-effort 执行 `test_script` 与 `cleanup_script`，并将日志纳入 receipt/artifacts（失败也要留痕）

## Impact
- Affected specs: `workspace`, `system-task-queue`
- Affected code (expected): `backend/internal/taskqueue/*`, `backend/internal/runtime/*`, `frontend/src/views/*`

