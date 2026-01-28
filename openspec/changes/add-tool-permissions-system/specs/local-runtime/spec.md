## MODIFIED Requirements

### Requirement: 工具权限控制（禁用与破坏性命令保护）
系统必须 (MUST) 提供 tool 权限控制能力，以便在本地/单机/多用户场景下限制风险；该能力必须 (MUST) 支持按 `principal_id`（用户）生效（见 `system-tool-permissions`）。

系统必须 (MUST) 保留 `ONEAGENT_DISABLE_TOOL_<TOOL_ID>=1`（或等价）作为 break-glass 的全局禁用开关，并保证其优先级最高。

系统不得 (MUST NOT) 继续支持 `ONEAGENT_BASH_ALLOW_RM` 这类“单点特例开关”作为权限模型的一部分；破坏性能力必须由统一的 tool permissions policy/profile 管理，并具备可解释拒绝原因与审计。

#### Scenario: 通过环境变量全局禁用工具
- **WHEN** 用户设置 `ONEAGENT_DISABLE_TOOL_BASH=1`（或等价）
- **THEN** 系统不得向 LLM 暴露该工具
- **AND** 若用户/系统显式请求该工具，应返回明确错误（包含 tool id 与禁用原因）

#### Scenario: `ONEAGENT_BASH_ALLOW_RM` 不再生效
- **GIVEN** 用户设置 `ONEAGENT_BASH_ALLOW_RM=1`
- **WHEN** 用户/LLM 通过 `bash` 尝试执行破坏性命令（例如 `rm`）
- **THEN** 系统仍应拒绝并返回明确错误
- **AND** 错误信息提示需要通过 tool permissions policy/profile 放开（而不是通过环境变量特例）

