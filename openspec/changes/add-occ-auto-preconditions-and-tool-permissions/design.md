# Design: OCC auto-preconditions + tool permissions

## Goals
- **L2 OCC 自动化**：在工具调用链路中，尽量减少“需要 LLM 自己记得带 preconditions”的负担。
- **不破坏现有流程**：只对“已被追踪/记录指纹”的文件自动注入 preconditions；未追踪文件保持当前行为。
- **可控开关**：允许通过环境变量关闭 OCC 或放开 bash 的 `rm`。

## OCC tracking model (per request/session)
- 每个请求/会话维护一个 `OCCState`（`map[path]sha256`）。
- `read_file`：
  - 若文件大小在阈值内，计算 `sha256` 并写入 `OCCState`。
  - 若文件过大，跳过记录（避免 IO 过重）。
- `write_file` / `edit`：
  - 如果调用参数 **未提供** `preconditions` 且 `OCCState` 中存在该文件的 sha，则自动注入 `expected_sha256`。
  - 写入成功后，重新计算并更新 `OCCState[path]`，确保同一任务连续多次编辑不会触发“自我冲突”。

## Limitations / tradeoffs
- `read_file` 支持分页/截断输出；OCC 指纹按“文件全量版本”计算（独立于输出是否截断），用于检测 explore→write 漂移。
- 对超大文件默认不记录 sha，避免显著性能退化；需要时可通过先显式传入 `preconditions` 达到更强保证。

## Tool permissions (minimal MVP)
- 禁用 tool：`ONEAGENT_DISABLE_TOOL_<TOOL_ID_UPPER>=1`（例如 `ONEAGENT_DISABLE_TOOL_BASH=1`）。
- bash 破坏性命令：默认拒绝包含 `rm` 的命令；通过 `ONEAGENT_BASH_ALLOW_RM=1` 放开。

