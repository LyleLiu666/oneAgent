## MODIFIED Requirements

### Requirement: System MUST auto-compress long chat sessions
系统必须 (MUST) 在 chat 会话上下文过长时自动执行“会话压缩”，以维持长会话可继续、可追溯、可缓存（best-effort）。

系统必须 (MUST) 使用固定的压缩触发阈值 `80k`（以 tokens 或等价的 prompt 长度估算为准，best-effort）。当系统估算本轮 prompt 上下文长度超过该阈值时，必须在继续下一轮对话前执行压缩（best-effort）。

压缩至少包含：
- 生成一条可继续对话的摘要（包含“流水账 + Findings”，best-effort）
- 保留最近若干轮对话作为 tail（best-effort）
- 将摘要作为一条稳定消息注入后续 prompt（best-effort）

#### Scenario: Compression triggers when context exceeds threshold
- **GIVEN** 某会话的 prompt 上下文长度超过 `80k` 阈值（best-effort）
- **WHEN** 系统准备继续下一轮对话
- **THEN** 系统执行会话压缩并生成一条摘要消息（best-effort）
- **AND** 后续 prompt 包含该摘要消息（best-effort）

#### Scenario: No compression when context is below threshold
- **GIVEN** 某会话的 prompt 上下文长度低于 `80k` 阈值（best-effort）
- **WHEN** 系统准备继续下一轮对话
- **THEN** 系统不执行压缩（best-effort）

#### Scenario: Secretary SU/SW channels are compressed independently (best-effort)
- **GIVEN** 同一 `principal_id` 下存在 SU 与 SW 两个通道（best-effort）
- **AND** SU 的 prompt 上下文长度低于 `80k`（best-effort）
- **AND** SW 的 prompt 上下文长度超过 `80k`（best-effort）
- **WHEN** 系统准备为 SU 继续下一轮对话
- **THEN** 系统不应因为 SW 超阈值而压缩 SU（best-effort）
- **WHEN** 系统准备为 SW 继续下一轮对话
- **THEN** 系统执行 SW 通道的会话压缩（best-effort）

