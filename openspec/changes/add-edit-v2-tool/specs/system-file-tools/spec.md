## ADDED Requirements

### Requirement: System MUST provide `edit_v2` for deterministic edits
系统必须 (MUST) 提供一个 `edit_v2` 工具，用于对文件内容执行可证明的替换，并在存在歧义/低置信度时明确失败（不得 silent success）。

`edit_v2` 必须支持至少以下约束能力：
- `occurrence`：指定替换第 N 次匹配
- `before/after` anchors：用于消歧
- `expected_replacements`：期望替换次数（不满足则失败）
- `expected_sha256`：基于文件内容的条件写入（OCC）

#### Scenario: 多处匹配时通过 occurrence 精确替换
- **GIVEN** 文件中 `oldcontent` 出现 2 次
- **WHEN** agent 调用 `edit_v2(..., occurrence=2, expected_replacements=1)`
- **THEN** 系统仅替换第 2 处匹配
- **AND** 返回 `matched_range` 指向被替换的行号范围

#### Scenario: 不可消歧时必须失败并返回诊断
- **GIVEN** 文件中 `oldcontent` 出现多处且未提供 anchors/occurrence
- **WHEN** agent 调用 `edit_v2(...)`
- **THEN** 系统拒绝执行并返回明确错误（包含诊断信息与建议下一步，例如先 `read_file`）

#### Scenario: OCC precondition mismatch fails safely
- **GIVEN** agent 读取了某文件并得到一个 `expected_sha256`
- **AND** 该文件在此后被其他进程修改（sha256 已变化）
- **WHEN** agent 调用 `edit_v2(..., expected_sha256=<old>)`
- **THEN** 系统拒绝写入并返回 `precondition_failed=true`

#### Scenario: diff_preview MUST be capped
- **GIVEN** 一次替换会产生很大的 diff
- **WHEN** agent 调用 `edit_v2(...)`
- **THEN** 系统返回的 `diff_preview` 仍然是可读的摘要（被 size cap 截断）
