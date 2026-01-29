## ADDED Requirements

### Requirement: Tools MUST declare mutability + reversibility for safety governance
系统必须 (MUST) 为每个可执行工具维护一份“安全元信息”（registry 或等价机制），至少包含：
- `effect=read_only|mutating`（是否会修改持久资产/外部状态）
- `reversibility=rollbackable|irreversible`（是否具备回退/补偿路径；不要求一定是“完美回滚”，但必须可操作）

当工具缺少上述元信息（例如用户自定义 tool/MCP tool 未声明）时，系统必须 (MUST) 采用保守默认值（例如视为 `mutating+irreversible`）并触发更严格的策略/审批要求，而不是 silent allow。

#### Scenario: Unknown tool is treated as high-risk by default
- **GIVEN** 系统挂载了一个未声明安全元信息的自定义 tool
- **WHEN** 系统为该 principal 生成可用工具列表
- **THEN** 系统默认将该 tool 视为高风险（例如 `mutating+irreversible`）
- **AND** 若策略未显式允许高风险工具，则该 tool 不应被暴露给 LLM

### Requirement: Policy MUST support approval gating for high-risk tool executions
系统必须 (MUST) 支持在 tool permissions policy 中为工具定义“审批约束”（例如 `approval=required|auto` 或等价约束），并在**执行阶段**强制执行。

当某次 tool call 需要审批时，系统必须 (MUST)：
- 在未获得批准前不执行该 tool call（fail-closed）
- 返回可操作的“待审批”响应（包含 tool_id、参数摘要、风险提示、以及回退/不可逆说明）
- 将审批决策（approve/deny）与理由写入审计证据链（trace/receipt）

#### Scenario: Tool call requires approval and is blocked until approved
- **GIVEN** policy 对某 mutating tool 设置 `approval=required`
- **WHEN** LLM 触发该 tool call
- **THEN** 系统不执行该 tool call
- **AND** 返回 “approval_required” 的可操作响应（包含风险与回退信息）
- **WHEN** 用户批准该次调用
- **THEN** 系统才执行该 tool call 并返回结果

### Requirement: Approval grants MUST be scoped, non-replayable, and auditable
系统必须 (MUST) 将一次审批授权绑定到明确的“调用指纹”（例如 `attempt_id + tool_id + canonical_args_hash`），并确保：
- 该授权不可被跨 attempt 复用（避免 replay）
- 该授权不可被参数漂移复用（相同 tool 不同参数必须重新审批）
- 审批记录可被审计与回放（包含批准者、时间、风险提示版本、以及执行结果引用）

#### Scenario: Approval cannot be replayed across attempts
- **GIVEN** attempt A 中用户批准了一次 tool call（tool_id 相同）
- **WHEN** attempt B 中 LLM 触发“相同 tool 但不同 attempt_id”的调用
- **THEN** 系统不得复用 attempt A 的审批结果
- **AND** 必须再次进入审批或按策略拒绝

