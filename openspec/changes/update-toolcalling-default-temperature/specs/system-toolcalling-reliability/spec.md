## ADDED Requirements

### Requirement: Tool-enabled chat MUST default to temperature=0 (unless explicitly set)
当本轮启用了工具调用（例如 `tool_ids` 非空且使用原生 tools 协议）时，系统必须 (MUST) 使用低温度以提升结构化输出稳定性。若未显式配置温度，系统必须 (MUST) 默认将 `temperature` 设为 `0`（best-effort；若 provider 不支持 0，可采用最接近的等价低温度）。

#### Scenario: Tool-enabled request sends temperature=0
- **GIVEN** 本轮启用了 tools（tool_ids 非空）
- **WHEN** 系统向 provider 发起一次 LLM 调用
- **THEN** 请求体包含 `temperature=0`（best-effort）

