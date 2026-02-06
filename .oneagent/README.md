# `.oneagent/` 目录说明（oneAgent 本地状态目录）

> 这个目录通常是 **oneAgent 的本地运行时状态**（internal state）。默认它位于 `ONEAGENT_HOME/.oneagent/`。
>
> 在本仓库里，当你把 `ONEAGENT_HOME` 指向当前 workspace（例如项目根目录）时，状态数据会落在 `./.oneagent/`，因此你会在 IDE 里看到它。

## 安全与提交（非常重要）

- **不要把这个目录里的数据提交到 Git**：里面包含 token、LLM 调用日志（可能含 prompt/response）、会话内容、任务痕迹等敏感信息。
- 仅建议把本文件 `./.oneagent/README.md` 跟踪到仓库（`.gitignore` 已做了例外放行）；其它内容默认应保持 ignore。

## ID/目录名速记

- `<session_id>`：一次 Chat 会话 ID（UUID）。
- `<task_id>`：长任务 Task 的 ID（UUID）。
- `<attempt_id>`：Task 的一次尝试/重试（UUID）。
- `<run_id>`：子 agent（subagent）的一次运行（UUID）。
- `YYYY-MM-DD`：按日期分桶的日志目录名。

## 目录结构（常见）

```
.oneagent/
  config/
    auth_token                 # AUTH_MODE=token 登录令牌（明文；不要泄露）
    config.yaml                # （可选）运行时配置文件

  settings.db                  # SQLite：Providers/Models/Settings/Tool approvals/skill usage 等
  settings.db-wal|settings.db-shm
  memory.db                    # SQLite：Secretary Memory（记忆条目）
  memory.db-wal|memory.db-shm

  data/
    sessions/<session_id>/      # 会话落盘
      session.json              # 会话元数据（含 next_message_id）
      messages.jsonl            # 消息流（JSON Lines，追加写）

    tasks/
      governance.json           # Task Queue 全局/工作区策略 + 定时调度
      <task_id>/
        task.json               # 任务定义（prompt、workspace、limits、attempts...）
        events.jsonl            # 事件流（状态流转、错误、产物路径等）
        attempts/<attempt_id>/  # 具体一次 attempt 的产物与回滚边界
          artifact_manifest.v1.json
          checkpoint/workspace.tgz     # attempt 开始时的 workspace 快照（用于 rollback）
          worktree/                   # （可选）worktree mode 的执行根
          review/
            changed_files.txt          # 变更文件列表（尽量稳定）
            diff.patch                # （可选）git diff（可能因环境/过大而缺失）
            review_comments.jsonl      # 机器 review 备注（JSONL）

    ledger/                     # Work Ledger（流水账/回执/学习）
      receipts/<receipt_id>/{receipt.json,receipt.md}
      digests/<principal_id>/{YYYY-MM-DD.md,YYYY-MM-DD.json}
      learning_jobs/<principal_id>/YYYY-MM-DD.json
      sop_suggestions/<suggestion_id>/suggestion.json

    workflows/<workspace_key>/   # Workflow 定义与运行（按 workspace 分桶）
      workflows/<workflow_id>/
        workflow.json
        versions/<version_id>.json
        runs/<run_id>/run.json

  logs/
    server.out                  # 服务端 stdout/stderr（best-effort）
    llm/YYYY-MM-DD/<session_id>/<call_id>.json   # 每次 LLM 调用的完整 request/response
    trace/
      mcp_calls.jsonl           # MCP 调用 trace（best-effort）
      channel_relay.jsonl       # channel relay trace（best-effort）
    subagent/YYYY-MM-DD/<parent_session_id>/<run_id>/
      trace.jsonl               # 子 agent 逐步 trace（JSONL）
      FINDINGS.md               # 子 agent 产出摘要（含流水账/Findings/变更文件）

  skills/<skill_id>/SKILL.md     # oneAgent-managed personal skills（以及 workspace 私有 skills）

  tmp/                           # 临时输出（可清理）
    live_llm_regression_*.{json,md}    # 在线回归报告（示例）
    benchmarks/<run_id>/report.{json,md}

  workspaces/ws-*/               # Secretary 自动创建的 workspace 池（可通过 env 覆盖根目录）
```

## 常见文件：你在 IDE 里看到它们通常意味着什么

- `config/auth_token`：本地登录 token（UI 登录页粘贴的那个）。**不要贴到聊天/issue 里**。
- `settings.db`：你配置的 provider/model、一些用户设置、工具审批记录等（SQLite）。
- `data/tasks/<task_id>/task.json`：某个长任务的定义与当前状态快照。
- `data/tasks/<task_id>/events.jsonl`：任务的事件时间线（调度/运行/产物/错误）。
- `logs/subagent/.../FINDINGS.md`：子 agent 的交付摘要（便于人工验收）。
- `logs/llm/.../*.json`：一次 LLM 调用的全量记录（排查 prompt/响应/错误很有用，但也最敏感）。

## 清理与重置（谨慎）

如果你只是想“清缓存/清日志”，一般只清这些就够了：

- `rm -rf .oneagent/tmp .oneagent/logs`

如果你想“彻底重置本地状态”（会丢失会话、任务历史、ledger 等）：

- 停止 oneAgent 后删除整个目录：`rm -rf .oneagent`

下次启动会自动重建所需目录/DB 文件。

## 相关实现（想对照代码时）

- 目录布局：`backend/internal/runtime/layout.go`
- Sessions 落盘：`backend/internal/sessionstore/`
- Tasks 落盘：`backend/internal/taskqueue/`
- LLM 调用日志：`backend/internal/llmlog/`
- Subagent 运行产物：`backend/internal/subagent/`
- Work Ledger：`backend/internal/workledger/`
- Workflow 落盘：`backend/internal/workflow/`

