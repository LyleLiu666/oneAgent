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
- 不尝试把“跨机/外部 agent 执行”的安全与回溯留在当前 oneAgent 服务里：若节点在外部系统执行，则应由外部系统负责其权限边界与证据链存储；oneAgent 仅保留最小化的指针/摘要（例如 run_id / request_id / 外部报告链接）用于索引与跳转（best-effort）。

## Key idea
把 **node_run 的交付物目录（artifact root）**当成“节点交付的唯一真相来源”，并将其置于**可持久化的工作流存储**中。

但落地时必须同时满足现有系统约束：文件工具默认只允许写入 workspace 内文件（否则会触发 `path is outside workspace`）。因此需要明确：
- 节点 agent 的 `WorkspaceRoot` 取什么（决定 file tools 的写边界）
- artifacts 目录是由 agent 直接写，还是由系统在节点结束后“收集/复制”写入

我建议把这件事拆成两层：**执行根目录（ExecutionRoot）** vs **交付根目录（NodeRoot）**，并让 NodeRoot 永远可追溯。

## Recommended execution model (v1): execute in project workspace/worktree, then export
你选择的是“一开始就支持在项目 workspace/worktree 内执行并导出交付物”，因此 v1 的推荐落地方式是：

1) **执行**：node agent 在项目 `ExecutionRoot` 内运行（workspace 或 worktree）
   - file tools 写边界仍然是项目代码目录（符合人类习惯；review 后改代码也合理）
2) **声明交付**：node agent 在结束时输出“交付清单（paths only）”
   - 交付清单只包含路径（不嵌入内容）
   - 路径可以是绝对路径或相对 `ExecutionRoot` 的路径
3) **导出（系统执行）**：系统将交付清单中的文件复制到 `NodeRoot/deliverables/`，并生成 `LEDGER.md`/`FINDINGS.md`/`artifacts.json`
   - 这样即使 ExecutionRoot 是会被清理的 worktree，交付物仍然可追溯
4) **交接**：下游节点只拿到上游 `NodeRoot` 下的路径（含 `artifacts.json` 路径），不拿内容

这个模型的核心收益是：**代码修改能力** 与 **可追溯交付证据链** 两者都保留，并且不需要放宽 file tools 的写权限边界。

## Notes: “责任编辑润色/就地修改”与证据链
你明确允许“就地修改上游交付文件，但必须有 diff 证据”，且也指出这在代码项目里很难严格限制。

因此我们采取的策略是：
- **不做强限制**（不要求 copy-on-write），避免和真实工作流冲突
- **尽量留痕**：每个 node_run 生成 best-effort 的 `diff_from_inputs` 证据（例如把输入 artifacts 导出的文件与本节点导出的文件做 diff；或记录 git patch）
- **推荐但不强制**：文档类交付建议“加法/新文件”优先；代码类交付允许修改并依赖 review/diff 证据链

## Scope note: local-first
本 change 的落地假设是 **workspace 与执行环境在同一台机器（local-first）**，因此：
- artifacts 以 OS 路径指针为主（绝对路径 + UI 展示相对路径）
- “可追溯过程”主要落在本机的文件与日志中（符合 current oneAgent 的 local runtime 画像）

当未来引入“跨机/外部 agent 节点”时，本 change 不尝试让 oneAgent 承担其安全与回溯存储；建议单独拆 capability：
- 外部执行平台负责：权限隔离、日志/交付物存储、可审计证据链
- oneAgent 仅保存：最小索引（外部 run_id/request_id）与可跳转链接（best-effort）

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
- `node_run.artifacts.artifacts[].path`：**绝对路径**（OS path），指向 `NodeRoot` 内的文件/目录（你选择：绝对路径为主）。
  - UI 额外展示相对路径（若能计算，例如相对 workspace root 或相对 `ExecutionRoot`）。
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

> 你选择“publish/export 完全由 agent 决定”：系统不预设 publish target；agent 可以直接在项目 workspace 内写入最终发布路径（例如 `docs/published/article.md`）。系统仅负责把 agent 声明的交付文件复制到 `NodeRoot` 形成证据链（export）。

## Retention & cleanup
- `WorkflowStoreRoot` 下的 run/node artifacts 属于“证据链”，默认建议保留 30 天（或跟随现有 `log_retention_days`）。
- 清理策略建议按 `FinishedAt`/目录 mtime best-effort 执行：
  - 删除超过保留天数的 `runs/<run_id>/` 目录
  - 仅清理终态 run（running 不清理）

## Open questions (need your decision)
## Decisions captured
- 交付物路径：先绝对路径 + UI 额外展示相对路径
- “责任编辑润色/就地修改”：允许，但必须生成 diff 证据（best-effort；不做强限制）
- publish/export：完全由 agent 决定（系统不预设 publish target；系统只做证据链 export）
- 执行模型：从一开始就支持“在项目 workspace/worktree 内执行并导出交付物”
- 用户配置：每个节点可配置 `skills/model/principal`
