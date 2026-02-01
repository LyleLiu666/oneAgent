# Change: Update tool loop limits + make write_file non-truncating

## Why
当前 chat 的工具 loop 仍然硬编码 `maxSteps=20`，在“长任务/多次工具调用/分段写文件”场景下很容易触发 `tool call limit reached`，导致任务中断且用户体感为“卡住/半成品”。

同时 `write_file` 目前会在 content 超过上限时**静默截断并返回 ok=true**（只在 payload 里标记 truncated），容易产生“无报错但文件不完整”的失败模式（B）。

## What Changes
- 提升 chat 工具 loop（JSON tools + XML fallback）的最大步数默认值，并提供环境变量可配置。
- `write_file` 不再截断写入：当 content 超过单次上限时，工具**直接失败（ok=false + error）且不写入/不改动文件**，让模型按分段策略重试。
- 更新工具手册/提示词中的相关说明，避免模型依赖“截断+继续 append”的旧行为。
- 增加回归测试覆盖：
  - maxSteps 可配置且生效（JSON + XML）。
  - `write_file` 超限时不写入且返回明确错误。

## Impact
- Affected specs:
  - `system-toolcalling-reliability`
  - `system-file-tools`
- Affected code:
  - `backend/internal/handler/chat.go`
  - `backend/internal/toolxml/engine.go`
  - `backend/internal/tool/write_file.go`
  - `backend/internal/tool/limits.go`
  - `backend/internal/prompt/assets/tools/write_file.md`
  - `backend/internal/toolxml/prompt.go`

