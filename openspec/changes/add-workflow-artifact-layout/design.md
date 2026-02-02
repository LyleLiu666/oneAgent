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
把 **node_run 的交付物目录（artifact root）**当成“节点交付的唯一真相来源”，并将其置于**可持久化的工作流存储**中。

但落地时必须同时满足现有系统约束：文件工具默认只允许写入 workspace 内文件（否则会触发 `path is outside workspace`）。因此需要明确：
- 节点 agent 的 `WorkspaceRoot` 取什么（决定 file tools 的写边界）
- artifacts 目录是由 agent 直接写，还是由系统在节点结束后“收集/复制”写入

我建议把这件事拆成两层：**执行根目录（ExecutionRoot）** vs **交付根目录（NodeRoot）**，并让 NodeRoot 永远可追溯。

## Recommended execution model (v1)
**优先推荐 v1 使用“RunRoot 作为 workspaceRoot”**：
- 为每次 workflow_run 创建一个 `RunRoot`（durable），并把 node agent 的 `WorkspaceRoot` 设置为 `RunRoot`
- 对每个 node 设置 `WriteScope=["nodes/<node_id>/**"]`，让该 node 只能写自己的交付目录
- 上游节点交付物位于同一个 `RunRoot` 下，下游节点可用相对路径读取（仍然是 path-only）

优点：
- 不需要放宽“写 workspace 外”权限，也不需要新增写入工具
- 节点间交付天然只传路径（同根目录可直接引用）
- 证据链稳定（不依赖 worktree/临时目录）

限制：
- 节点若需要修改项目代码（workspace repo），需要额外设计“挂载/复制/导出”机制（可作为 v2 扩展）

## Alternative (v2): execute in project workspace/worktree, then export
当工作流节点需要读写项目代码时，可以采用：
- node agent 仍在项目 `ExecutionRoot`（workspace 或 worktree）内执行（file tools 作用域不变）
- 节点结束后，系统将“声明的交付文件（paths only）”复制到 `NodeRoot/deliverables/`
- `artifact manifest` 记录 `NodeRoot` 下的最终路径，供下游节点使用

这能把“写代码/跑命令”和“交付物证据链”解耦，但需要补充：
- 节点如何声明交付清单（例如输出一个机器可读的 `deliverables.json`）
- 上游交付物如何注入到下游的 ExecutionRoot（copy/symlink，仍只传路径不传内容）

## Canonical layout
术语：
- `WorkflowStoreRoot`：工作流持久化根目录（当前实现为 `ONEAGENT_HOME/.oneagent/data/workflows`；也可通过配置迁移到 workspace 内的 `.oneagent/`）。
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
4) 执行模型：你希望 workflow 节点 **v1 先只在 RunRoot 内产出交付物**（不碰项目代码），还是需要从一开始就支持“在项目 workspace/worktree 内执行并导出交付物”？
5) “用户配置”具体指什么：仅 artifacts 存储策略，还是也要支持 **每个节点配置 skills/model/principal**（例如 reporter/writer/editor 各自 skills 列表）？如果需要，我建议单独开一个 change 做 graph schema 扩展。
