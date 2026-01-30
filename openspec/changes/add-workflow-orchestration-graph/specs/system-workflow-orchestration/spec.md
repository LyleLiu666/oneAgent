## ADDED Requirements

### Requirement: The system MUST support workflow definitions as explicit graphs
系统必须 (MUST) 支持将一个“可执行委托”表达为显式 workflow graph，并作为**workspace 级资源**持久化：
- workflow 具备稳定 ID（可被引用/复用）
- workflow 有版本（workflow_version），run 必须绑定到一个版本快照（best-effort）

#### Scenario: A workflow can be created and listed per workspace
- **GIVEN** 用户选择了一个 workspace
- **WHEN** 用户创建一个 workflow 并保存
- **THEN** 该 workflow 可在该 workspace 的 workflow 列表中被找到（best-effort）

### Requirement: A workflow graph MUST have deterministic dependency semantics
系统必须 (MUST) 将 workflow graph 定义为有向图（DAG best-effort），并提供确定的依赖语义：
- node 只能在其上游依赖 node 达到“完成”后才会被调度（best-effort）
- 允许无依赖的 node 并行执行（受并发/预算限制）（best-effort）

#### Scenario: Downstream node runs only after upstream completion
- **GIVEN** 一个 workflow 含 A→B 的依赖边
- **WHEN** workflow_run 执行
- **THEN** B 的 node_run 不会在 A 完成前启动（best-effort）

### Requirement: Each workflow node MUST execute as a work-style agent and hand off file-set artifacts
系统必须 (MUST) 将每个 workflow node 视为一个“工作型 agent”执行单元（而非函数调用），并满足：
- node 运行产生可交付产物：**文件集 artifacts**（artifact manifest / pointers）与证据（trace/summary/findings）（best-effort）
- node 的 outputs 可作为下游 node 的输入（best-effort）

#### Scenario: A node produces a multi-file artifact manifest
- **GIVEN** 一个 node 的目标是“生成一组文件”（best-effort）
- **WHEN** node_run 完成
- **THEN** 系统保存该 node_run 的 artifact manifest（包含多个文件指针）（best-effort）

### Requirement: Node completion MUST be defined by Hard Gate + Soft Gate
系统必须 (MUST) 为 workflow node 提供双层完成判定（与 `openspec/project.md` 一致）：
1) Hard Gate：可代码/规则校验的客观事实（例如文件存在、测试通过、schema valid）（best-effort）
2) Soft Gate：按预设 rubric 的主观评分（LLM 评审；best-effort）

#### Scenario: Node is marked done only when both gates pass
- **GIVEN** 一个 node 配置了 Hard Gate 与 Soft Gate（best-effort）
- **WHEN** node_run 产出交付物并触发验收
- **THEN** Hard Gate 与 Soft Gate 都通过时，node 才进入 done 状态（best-effort）
- **AND** 任一 gate 未通过时，系统留下结构化报告以支持继续迭代（best-effort）

### Requirement: Workflow runs MUST be observable and resumable
系统必须 (MUST) 让 workflow_run 可被观察与续跑：
- 每个 node_run 有明确状态（pending/running/done/failed/canceled best-effort）
- 每个 node_run 的事件/日志可查看（waterfall best-effort）
- workflow_run 可 resume，并复用已完成 node_run 的结果（best-effort）

#### Scenario: A workflow run can be resumed without rerunning completed nodes
- **GIVEN** 一个 workflow_run 中 A 已 done，B 尚未开始（best-effort）
- **WHEN** 用户对该 workflow_run 执行 resume
- **THEN** 系统不会重跑 A，而是继续调度 B（best-effort）

