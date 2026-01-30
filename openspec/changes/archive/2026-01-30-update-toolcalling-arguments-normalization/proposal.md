# Change: Improve tool arguments normalization (JSON repair)

## Why
在 `tool_protocol=json` 的链路里，tool call 的 `arguments` 字段经常出现“几乎是 JSON，但不是合法 JSON”的常见形态，例如：
- ```json ... ``` 围栏
- 前后夹杂解释文本（`Here is the JSON: {...}`）
- 整体被引号包住（JSON-string）
- 多段 JSON 拼接或截断片段（best-effort）

当前实现 `backend/internal/llm/llm.go:normalizeToolArguments` 的修复逻辑非常有限，几乎无法覆盖上述真实场景，导致：
- 工具执行前的 `json.Unmarshal` 失败率高
- 自愈循环步数增加（用户体感“不顺滑”）

## What Changes
- 扩展 `normalizeToolArguments` 的修复能力（best-effort），优先保证：
  - 产出合法 JSON object/array 字符串
  - 不改变语义（尽量保持无损）
- 增加单元测试覆盖典型脏输入（围栏/夹杂文本/quoted JSON/多段 JSON）

## Impact
- Affected specs: `system-toolcalling-reliability`
- Affected code (expected): `backend/internal/llm/llm.go`
- Tests (expected): `backend/internal/llm/*_test.go`（新增）

