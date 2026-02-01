## ADDED Requirements

### Requirement: System MUST auto-compress long chat sessions
系统必须 (MUST) 在 chat 会话上下文过长时自动执行“会话压缩”，以维持长会话可继续、可追溯、可缓存（best-effort）。

压缩至少包含：
- 生成一条可继续对话的摘要（包含“流水账 + Findings”，best-effort）
- 保留最近若干轮对话作为 tail（best-effort）
- 将摘要作为一条稳定消息注入后续 prompt（best-effort）

#### Scenario: Compression triggers when context exceeds threshold
- **GIVEN** 某会话的 prompt 上下文长度超过压缩阈值（best-effort）
- **WHEN** 系统准备继续下一轮对话
- **THEN** 系统执行会话压缩并生成一条摘要消息（best-effort）
- **AND** 后续 prompt 包含该摘要消息（best-effort）

#### Scenario: No compression when context is below threshold
- **GIVEN** 某会话的 prompt 上下文长度低于压缩阈值（best-effort）
- **WHEN** 系统准备继续下一轮对话
- **THEN** 系统不执行压缩（best-effort）

### Requirement: Compression summary format MUST be stable and labeled
系统必须 (MUST) 将压缩摘要以一条 assistant text message 的形式持久化，并使用稳定前缀标识来源（例如 `【会话压缩】`），以便用户/开发者可确认其发生（best-effort）。

摘要内容必须 (MUST) 至少包含两个段落：
1) 流水账（timeline）
2) Findings（结论/决定/约束/待办）

#### Scenario: Summary message is labeled and structured
- **GIVEN** 系统执行了一次会话压缩
- **WHEN** 系统将摘要写入 session store
- **THEN** 摘要消息内容以 `【会话压缩】` 开头（best-effort）
- **AND** 摘要包含“流水账”与“Findings”两个部分（best-effort）

### Requirement: Compression MUST NOT delete the current user message
系统必须 (MUST) 确保在“本轮 user message 写入”前执行压缩时，不会误删当前 user message；压缩后仍应继续使用当前 user message 作为本轮输入（best-effort）。

#### Scenario: Current user message is preserved after compression
- **GIVEN** 系统在持久化本轮 user message 之前触发压缩（best-effort）
- **WHEN** 系统完成压缩并继续本轮对话
- **THEN** 本轮 user message 仍作为本轮输入存在于 prompt tail（best-effort）

### Requirement: Compression failures MUST degrade safely (best-effort)
当摘要生成失败时（例如 LLM 调用失败），系统必须 (MUST) 退化为：
- 不对 session store 做 destructive rewrite（best-effort）
- 仅保留最近若干轮对话并插入一条占位提示（best-effort）

#### Scenario: Summary generation failure triggers fallback
- **GIVEN** 会话需要压缩但摘要生成失败（best-effort）
- **WHEN** 系统继续本轮对话
- **THEN** 系统通过占位提示说明“摘要生成失败”并仅保留最近对话（best-effort）

### Requirement: Project MUST provide regression tests for compression
项目必须 (MUST) 提供自动化回归测试覆盖会话压缩，以避免未来改动导致压缩静默失效。

#### Scenario: CI catches compression logic regressions
- **GIVEN** 某次修改破坏了压缩逻辑（例如不再写入 `【会话压缩】`）
- **WHEN** 运行 backend 单元测试
- **THEN** 测试失败并指出不符合压缩契约（best-effort）
