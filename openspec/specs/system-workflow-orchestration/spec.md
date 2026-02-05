# system-workflow-orchestration Specification

## Purpose
TBD - created by archiving change add-workflow-orchestration-graph. Update Purpose after archive.
## Requirements
### Requirement: The system MUST support workflow graphs with versioning (workspace-scoped)
系统必须 (MUST) 支持工作流（workflow）的显式图表达，并提供版本化能力（workspace-scoped）：
- workflow 包含 `workflow_id`、`workspace_root`、`name`（best-effort）
- workflow_version 包含 `version_id`、`workflow_id`、`graph`（nodes/edges/config）与 `published_at`（best-effort）
- 已发布（published）的 workflow_version 不得 (MUST NOT) 被修改（immutable snapshot）

#### Scenario: Publish creates an immutable workflow_version
- **GIVEN** 用户在某 workspace 创建 workflow 并编辑 graph（best-effort）
- **WHEN** 用户发布一个新版本（publish）
- **THEN** 系统创建一个新的 workflow_version（best-effort）
- **AND** 后续编辑不得影响已发布版本的 graph（best-effort）

### Requirement: The system MUST support workflow runs and node runs with durable state (best-effort)
系统必须 (MUST) 支持 workflow_run / node_run 的持久化状态（best-effort），用于可回放与可续跑：
- workflow_run 引用一个已发布的 workflow_version（best-effort）
- workflow_run 保存一份运行快照（graph snapshot + resolved inputs）（best-effort）
- node_run 至少包含 `node_id`、`status`、`started_at/finished_at` 与错误信息（best-effort）

#### Scenario: A workflow_run keeps a snapshot for replay
- **GIVEN** 某 workflow_version 已发布（best-effort）
- **WHEN** 用户启动一次 workflow_run
- **THEN** 该 run 引用该版本并保存 graph snapshot（best-effort）
- **AND** 后续 workflow 编辑不会影响该 run 的执行快照（best-effort）

### Requirement: The system MUST execute DAG nodes with dependency-aware scheduling (best-effort)
系统必须 (MUST) 以 DAG 调度方式执行节点（best-effort）：
- 节点仅在其依赖节点满足完成条件后才可执行（best-effort）
- 系统应支持并发限制（best-effort）
- 支持 cancel 与 resume（best-effort）

#### Scenario: Node scheduling respects dependencies
- **GIVEN** workflow graph 中 B 依赖 A（A → B）
- **WHEN** workflow_run 执行
- **THEN** B 不会在 A 未完成前进入 running（best-effort）

### Requirement: Node runs MUST produce artifact manifests for multi-file handoffs (best-effort)
节点运行必须 (MUST) 产出“交付物清单”（artifact manifest best-effort），用于节点交接与审查：
- artifact manifest 必须只包含“文件/目录的路径或指针”（path-only；不得嵌入文件内容）（best-effort）
- artifact manifest 至少包含：流水账（ledger）、findings（含交付清单摘要）、以及交付文件路径列表（best-effort）
- artifact manifest 必须可被前端展示并可点击打开（best-effort）

#### Scenario: A node_run exposes a multi-file artifact list
- **GIVEN** 某节点产出多个文件作为交付物（best-effort）
- **WHEN** node_run 进入 succeeded
- **THEN** 该 node_run 的 artifact manifest 记录这些文件的路径（best-effort）

#### Scenario: Downstream nodes receive only artifact paths
- **GIVEN** workflow graph 中 B 依赖 A（A → B）
- **AND** A 的 node_run 已产出 artifact manifest（best-effort）
- **WHEN** 系统准备执行 B
- **THEN** 系统只向 B 注入 A 的交付物路径（含 manifest 路径）（best-effort）
- **AND** 系统不得把 A 的文件内容注入到 B 的上下文中作为“交付物传递”（best-effort）

### Requirement: Node completion MUST use Hard/Soft Gate with stored reports (best-effort)
每个节点完成必须 (MUST) 有双层验收（best-effort）：
- Hard Gate：客观可校验（测试/文件存在/Schema 等）并生成报告（best-effort）
- Soft Gate：按预设 rubric 由模型打分，并生成评分报告（best-effort）
- 两类 gate 的结果必须作为 evidence 被持久化并可在 UI 查看（best-effort）

#### Scenario: A node_run includes gate reports as evidence
- **GIVEN** 某 node_run 需要验收（best-effort）
- **WHEN** node_run 进入终态
- **THEN** 系统写入 Hard Gate 与 Soft Gate 的报告（best-effort）
- **AND** UI 可展示该报告指针（best-effort）

### Requirement: Workflow run artifacts MUST be stored durably with a deterministic directory layout (best-effort)
系统必须 (MUST) 为每次 workflow_run 与 node_run 创建可持久化的 artifacts 目录（best-effort），并满足：
- 目录结构可预测（基于 workspace/workflow/run/node 的稳定 key）（best-effort）
- artifacts 不得依赖临时执行目录（例如 worktree）而在 run 结束后失效（best-effort）
- 系统应在 node_run 中记录关键 artifacts 的路径指针（best-effort）

#### Scenario: Artifacts remain accessible after restart
- **GIVEN** 一次 workflow_run 已结束并进入终态（best-effort）
- **WHEN** oneAgent 服务重启后用户查询该 run（best-effort）
- **THEN** run 中记录的 artifacts 路径仍可被打开/读取（best-effort）

### Requirement: Workflow artifact storage MUST be user-configurable (best-effort)
系统必须 (MUST) 提供用户可配置的 workflow artifact 存储策略（best-effort），至少包含：
- artifacts 根目录（best-effort）
- 保留策略（例如 retention days）（best-effort）

#### Scenario: User overrides artifact root
- **GIVEN** 用户配置了 workflow artifacts 根目录（best-effort）
- **WHEN** 用户执行一次 workflow_run（best-effort）
- **THEN** 该 run 的 artifacts 落在用户指定的根目录下（best-effort）

### Requirement: Workflow nodes MUST support per-node execution config (skills/model/principal) (best-effort)
系统必须 (MUST) 支持为 workflow graph 的每个 node 配置独立的执行配置（best-effort），至少包含：
- `principal_id`：该 node 的执行主体（用于 tool policy/审计归属）（best-effort）
- `model` 或 `model_id`：该 node 使用的模型（best-effort）
- `skills[]`：该 node 注入的技能集合（best-effort）

系统必须 (MUST) 将该配置随 workflow_version 一起发布并固化（immutable snapshot）（best-effort），并在 workflow_run 中保留一份 resolved snapshot（best-effort）。

#### Scenario: Node execution config is snapshotted in workflow_version and run
- **GIVEN** workflow graph 中节点 writer 配置了 `principal_id=writer` 且 `skills=[reporter-v1]` 且 model 不同于 editor（best-effort）
- **WHEN** 用户 publish 一个 workflow_version 并创建一次 workflow_run（best-effort）
- **THEN** workflow_version 中的 graph snapshot 包含这些 node-level 配置（best-effort）
- **AND** workflow_run 保存一份 resolved snapshot，运行过程中不得漂移（best-effort）

