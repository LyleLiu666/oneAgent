## MODIFIED Requirements
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

## ADDED Requirements
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
