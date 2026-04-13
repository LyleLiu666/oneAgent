## MODIFIED Requirements
### Requirement: The system MUST normalize/repair tool arguments (best-effort)
当 provider/模型输出的 tool call arguments 不是严格合法 JSON 时，系统必须 (MUST) 做 best-effort 的 normalization/repair，以尽量将其修复为可继续处理的 JSON 字符串，减少无意义的 tool loop 浪费。

当系统进入“外置共享运行内核”模式时，宿主层不得 (MUST NOT) 再维护另一套独立的 “strict JSON / best-effort JSON” 判定规则；系统必须 (MUST) 直接复用外部 `agentsdk` 的共享实现（best-effort），宿主层只决定当前 loop 使用 `best_effort` 还是 `strict`。

共享容错至少满足（best-effort）：
- 非法 JSON 不直接让整轮 loop 崩掉
- 原始文本保留在 `{"_raw":"..."}` 中
- 明确 strict 模式时，系统才允许把坏参数视为协议错误

#### Scenario: 坏 tool arguments 在 best-effort 模式下保留 `_raw`
- **GIVEN** 模型返回了坏掉的 tool arguments JSON（best-effort）
- **WHEN** 系统以 best-effort 模式处理参数（best-effort）
- **THEN** 参数会被包装成合法 JSON 且保留 `_raw`（best-effort）
- **AND** 系统不会因为这一次坏参数直接中断整轮 loop（best-effort）
