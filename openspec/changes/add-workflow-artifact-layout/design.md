# Design: Workflow artifact layout (path-only handoffs)

## Goals
- **Only paths**：节点间只传交付物路径（不传内容），下游节点自行按路径读取。
- **Durable**：交付物不依赖临时执行目录（例如 worktree），run 结束/重启后仍可追溯。
- **Deterministic**：基于 workspace/workflow/run/node 的稳定目录结构，便于 UI/排障/清理。
- **Traceable**：每个节点都有流水账、findings（含交付清单）、交付文件，以及事件/日志指针。
- **User-configurable (best-effort)**：允许用户决定“交付物落在哪里、保留多久、是否同步到 workspace”。

## Non-goals
- 不做跨机器的远程 artifact 存储（S3/OSS）与分发。
- 不在节点间传输文件内容（即使是小文件也不走 prompt/tool payload）。

## Key idea
把 **node_run 的交付物目录（artifact root）**当成“节点的唯一写入口”，并将其置于**持久化工作流存储**中；节点执行可以发生在 workspace / worktree / sandbox，但交付物必须写到这个 durable 目录里，并在 `artifact manifest` 中只返回路径。

## Canonical layout
术语：
- `WorkflowStoreRoot`：工作流持久化根目录（当前实现为 `ONEAGENT_HOME/.oneagent/data/workflows`）。
- `RunRoot`：某次 workflow_run 的目录。
- `NodeRoot`：某个 node_run 的目录。

建议目录结构（示意）：

```
WorkflowStoreRoot/
  <workspace_key>/
    workflows/<workflow_id>/
      workflow.json
      versions/<version_id>.json
      runs/<run_id>/
        run.json
        run.events.jsonl                # 可选：run 级事件流（append-only）
        nodes/
          <node_id>/
            inputs.json                 # 仅包含“上游交付物路径列表/指针”，不嵌入内容
            LEDGER.md                   # 流水账（人类可读）
            FINDINGS.md                 # Findings（必须包含交付清单摘要）
            artifacts.json              # 机器可读 manifest（paths + metadata）
            trace.jsonl                 # 节点内部执行 trace（append-only）
            deliverables/               # 节点交付文件（自由结构）
              ...
            gates/
              hard_gate.json            # 可选
              soft_gate.json            # 可选
```

### Path conventions
- `node_run.artifacts.artifacts[].path`：**绝对路径**（OS path），指向 `NodeRoot` 内的文件/目录。
  - 这样即便节点执行根目录是 worktree（会被清理），交付物仍可用。
  - UI 可以直接 copy path；后续如果补充“点击打开”，也可以用该路径作为指针。
- `inputs.json`：只包含路径（string 数组 / 结构体），下游节点读取文件内容时必须通过 file tools（或等价机制）按路径读取。

## What each node MUST deliver
每个节点的最小交付包（用于“信任 + 可追溯”）：
- `LEDGER.md`：工作流水账（按时间/步骤记录做了什么）。
- `FINDINGS.md`：关键结论 + **交付文件清单（manifest 摘要）**。
- `deliverables/`：交付文件本体（文章、数据、图片、补丁等）。
- `artifacts.json`：机器可读清单（供 UI 展示/下游节点消费）。

其中 `artifacts.json` 建议结构：

```json
{
  "schema_version": 1,
  "node_id": "writer",
  "run_id": "…",
  "artifacts": [
    {"kind": "ledger", "path": "/abs/.../LEDGER.md"},
    {"kind": "findings", "path": "/abs/.../FINDINGS.md"},
    {"kind": "deliverable", "path": "/abs/.../deliverables/article.md"}
  ]
}
```

> `workflow.NodeRun.Artifacts` 可以直接复用该 `artifacts` 数组（避免前端额外读文件）。

## Path-only handoff mechanics
节点 A → 节点 B 的“交接输入”只包含路径：
- `inputs.json`（位于 B 的 NodeRoot）写入：
  - 上游 node_id
  - 上游 `artifacts.json` 路径
  - 上游 `deliverables/*` 具体文件路径（可选：展开或只给 manifest）
- B 的 agent 读取这些路径，决定如何加工，然后在自己的 `deliverables/` 写出新版本。

关键点：**下游节点不修改上游交付目录**（避免破坏证据链）。若需要“润色同一篇文章”，推荐输出新文件并在 `FINDINGS.md` 中注明输入输出对应关系。

## User configuration (best-effort)
为了满足不同团队/项目习惯，建议提供三层配置（由近到远覆盖）：
1) Workflow-level（随 workflow_version 固化，适合可复用工作流）
2) Workspace-level（`<workspace>/.oneagent/project.json`，适合项目偏好）
3) Global（`ONEAGENT_HOME/.oneagent/config/config.yaml`，适合个人默认）

建议可配置项：
- `workflow_artifacts_root`：artifact 根目录（默认 `ONEAGENT_HOME/.oneagent/data/workflows`）
- `workflow_artifacts_retention_days`：保留天数（默认 30；0 表示不自动清理）
- `workflow_publish_targets`（可选）：把某些 deliverables 同步/复制到 workspace 的目标路径（例如 editor 节点把 `deliverables/article.md` 复制到 `docs/published/article.md`）

> publish/export 由系统执行（copy），避免让节点在写入权限上“既要能写 workspace，又要能写 artifacts root”导致 scope 复杂化。

## Retention & cleanup
- `WorkflowStoreRoot` 下的 run/node artifacts 属于“证据链”，默认建议保留 30 天（或跟随现有 `log_retention_days`）。
- 清理策略建议按 `FinishedAt`/目录 mtime best-effort 执行：
  - 删除超过保留天数的 `runs/<run_id>/` 目录
  - 仅清理终态 run（running 不清理）

## Open questions (need your decision)
1) 交付物路径：你更希望 **绝对路径**（最稳）还是 **workspace 相对路径**（更可移植）？我倾向先绝对路径 + UI 里额外展示相对路径。
2) “责任编辑润色”场景：是否要求 **每个节点输出不可变**（copy-on-write）？还是允许就地修改上游交付文件但必须生成 diff 证据？
3) publish/export：你希望默认把最终交付同步到 workspace 的某个固定目录（比如 `<workspace>/deliverables/`），还是完全由 workflow 配置决定？

