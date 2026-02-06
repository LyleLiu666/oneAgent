# Change: Standardize task deliverable contract with schema-versioned artifacts

## Why
当前任务交付证据虽然丰富，但字段与可用性在不同场景下可能不一致，导致“可审查包”体验不稳定。要实现规模化交付，必须把 artifacts 契约标准化并版本化。

## What Changes
- 定义统一的 task attempt artifact manifest（版本化 schema）
- 强制关键交付字段的稳定暴露：
  - `summary`
  - `findings_path`
  - `trace_log_path`
  - `changed_files_path` / `diff_patch_path`
  - `test_report_path`（可得时）
- 将 receipt 与 attempt artifacts 的映射标准化，支持 UI 稳定消费
- 为缺失字段提供可解释原因，而非静默缺失

## Impact
- Affected specs:
  - `system-task-queue`
  - `system-work-ledger`
- Affected code:
  - attempt artifact writer
  - receipt materialization
  - API response schema and UI consumers
