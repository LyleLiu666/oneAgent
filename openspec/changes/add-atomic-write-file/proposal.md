# Change: Atomic overwrite for file-writing tools (`write_file` + SBE)

## Why
oneAgent 内部状态落盘（tasks/sessions/ledger）已经大量采用 “write temp + rename” 的原子写策略，但用户侧文件工具仍存在直接 `os.WriteFile` 覆盖写的路径，导致在崩溃/中断/磁盘抖动时更容易出现半写文件（silent corruption）。

对 “长任务 + 工业级交付” 来说，这类低概率错误的代价极高：一次错误覆盖会污染后续所有步骤与证据。

## What Changes
- 将 `write_file` 的 overwrite 模式升级为“同目录 temp + rename”的原子替换策略（append 保持非原子语义）。
- 将 SBE（edit actuator）的写回同样升级为原子替换策略。
- 增加测试覆盖：确保失败时不污染目标文件，确保临时文件可清理。

## Impact
- Affected specs:
  - `system-file-tools`（新增/修改写文件工具的 crash-consistency 要求）
- Affected code (expected):
  - `backend/internal/tool/write_file.go`
  - `backend/internal/sbe/actuator.go`
  - `backend/internal/fsutil/atomic_write.go`（新通用工具）
  - tests

