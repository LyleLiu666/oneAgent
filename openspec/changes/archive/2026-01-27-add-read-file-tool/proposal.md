# Change: Add `read_file` tool (paged, size-capped, workspace-aware)

## Why
目前 oneAgent 缺少稳定的“文件读取”工具：只能通过 `rg` 命中行或 `bash/run_command` 拼装读取，易受输出截断与上下文不完整影响，进而放大 `edit` 的失败率与误编辑风险。

对标 openagentic-sdk，`read_file`（分页/限流/offset）是稳定性基石：它让 agent 能确定性地分块读取大文件，并在每次编辑前拿到足够上下文与锚点证据。

## What Changes
- 新增 `read_file` 工具：
  - 支持按行分页（`offset_lines`/`limit_lines`）与按字节上限（`max_bytes`）的双重限流；
  - 返回结构化结果（行号范围、是否截断、总行数/总字节等可选元信息）；
  - 路径解析与权限：
    - workspace 启用时：相对路径必须在 workspace 内；绝对路径允许读取（只读）；
    - workspace 未启用时：仅允许读取绝对路径（避免相对路径语义不清）。

## Impact
- Affected specs:
  - `workspace`（补齐“读取能力”的工具级实现约定）
  - **New**: `system-file-tools`（文件读写工具的能力与稳定性约定）
- Affected code (expected):
  - `backend/internal/tool/registry.go`（新增 ToolID）
  - `backend/internal/tool/read_file.go`（新工具实现）
  - `backend/internal/tool/*_test.go`（单测）
  - （可选）`docs/tool-call-failures.md`（读文件最佳实践）

