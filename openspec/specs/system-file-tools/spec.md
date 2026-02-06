# system-file-tools Specification

## Purpose
Defines file tools for workspace-aware reading and mutation (`read_file`, `write_file`, `edit_v2`, `trash_file`), including atomic writes, policy enforcement, and size/ambiguity guardrails.

## Requirements
### Requirement: 系统必须提供 `read_file` 工具（分页读取 + 输出限流）
系统必须 (MUST) 提供一个 `read_file` 工具，使 agent 可在不依赖 shell 输出的前提下读取文件内容，并支持：
- 按行分页：`offset_lines` + `limit_lines`
- 按字节上限：`max_bytes`

工具输出必须 (MUST) 返回可解释的读取结果，至少包含：
- `file_path`（解析后的路径）
- `content`
- `start_line` / `end_line`
- `truncated`（是否因限流截断）

#### Scenario: 读取小文件返回完整内容
- **GIVEN** workspace 内存在一个小文件 `a.txt`
- **WHEN** agent 调用 `read_file(filePath="a.txt", offset_lines=0, limit_lines=200, max_bytes=65536)`
- **THEN** 系统返回 `content` 包含完整文件
- **THEN** `truncated=false`

#### Scenario: 读取大文件可分页拉取
- **GIVEN** workspace 内存在一个大文件 `big.log`（行数远大于 200）
- **WHEN** agent 调用 `read_file(filePath="big.log", offset_lines=0, limit_lines=200)`
- **THEN** 系统返回 `start_line=1` 且 `end_line=200`（或等价的 1-based 行号）
- **WHEN** agent 再次调用 `read_file(filePath="big.log", offset_lines=200, limit_lines=200)`
- **THEN** 系统返回下一段内容（行号区间不重叠）

### Requirement: `read_file` 路径解析必须 workspace-aware
系统必须 (MUST) 在 workspace 启用时将相对路径解析为 workspace 内的文件；相对路径若越界（`..` 或 symlink 等导致）必须被拒绝并返回清晰错误。

系统必须 (MUST) 允许读取 workspace 之外的绝对路径文件（只读），以满足排障与参考资料读取需求。

#### Scenario: workspace 启用时相对路径越界被拒绝
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `read_file(filePath="../secrets.txt")`
- **THEN** 系统拒绝并返回“path is outside workspace”的清晰错误

#### Scenario: workspace 启用时允许读取绝对路径
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `read_file(filePath="/tmp/notes.txt")`
- **THEN** 系统允许读取并返回内容（只读）

### Requirement: `write_file` overwrite 必须是原子替换（Crash Consistency）
系统必须 (MUST) 确保 `write_file` 在 overwrite 模式下采用原子替换策略（同目录写临时文件 + rename 覆盖目标文件），以避免进程崩溃/中断导致半写文件。

系统可以 (MAY) 在 append 模式保持现有语义（append 不要求原子替换），但应在输出中清晰标注写入模式与写入统计信息，便于验收与复核。

#### Scenario: overwrite 在崩溃中断时不污染目标文件
- **GIVEN** 目标文件 `a.txt` 已存在且内容为 `old`
- **WHEN** 系统执行 overwrite 写入，但在 rename 之前发生异常中断
- **THEN** `a.txt` 仍保持 `old`（不会出现半写内容）

### Requirement: SBE 写回必须复用原子替换
系统必须 (MUST) 确保 SBE（edit）对文件的写回同样使用原子替换策略，避免出现“edit 成功返回但文件损坏”的 silent corruption。

#### Scenario: edit 写回不产生半写文件
- **GIVEN** SBE 对某文件执行替换并准备写回
- **WHEN** 写回过程中发生异常中断
- **THEN** 原文件要么保持不变，要么被完整的新版本替换（不得出现半写状态）

### Requirement: File tools MUST enforce policy constraints
系统必须 (MUST) 在文件写/改/删类工具中执行权限策略约束（例如 `file_scope`、`read_outside_workspace` 等）。

#### Scenario: file_scope blocks writes outside allowed glob
- **GIVEN** 当前 policy 的 `file_scope=["backend/**"]`
- **WHEN** 工具尝试写入 `frontend/App.vue`
- **THEN** 系统拒绝并返回“file_scope violation”

#### Scenario: read_outside_workspace denied by policy
- **GIVEN** policy 设置 `read_outside_workspace=deny`
- **WHEN** 工具尝试读取 workspace 外绝对路径文件
- **THEN** 系统拒绝并返回明确错误

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

### Requirement: 系统必须提供 `trash_file` 工具（软删除 / move-to-trash）
系统必须 (MUST) 提供一个 `trash_file` 工具，用于对 workspace 内的文件/目录执行软删除：从原路径移除，并移动到 workspace 内的系统回收站目录（例如 `<workspace>/.oneagent/trash/`）。

`trash_file` 必须 (MUST)：
- 仅允许操作 workspace 内路径（拒绝 `..` 与 symlink 逃逸导致的越界）
- 在工具输出中返回 `trash_id`、`original_path` 与 `trashed_path`（解析后的真实路径），便于审计与人工恢复
- 在执行阶段强制执行 tool permissions 中的 `file_scope` 等约束，至少对 `filePath` 进行限制

#### Scenario: trash_file 将文件移动到回收站并从原位置消失
- **GIVEN** workspace 内存在文件 `tmp/a.txt`
- **WHEN** agent 调用 `trash_file(filePath="tmp/a.txt")`
- **THEN** `tmp/a.txt` 在原位置不再存在
- **AND** 系统返回 `trashed_path` 指向 `<workspace>/.oneagent/trash/...` 下的真实路径

#### Scenario: workspace 启用时越界路径被拒绝
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `trash_file(filePath="../secrets.txt")`
- **THEN** 系统拒绝并返回“path is outside workspace”的清晰错误

### Requirement: 系统必须自动清理回收站条目（7 天保留期）
系统必须 (MUST) 对回收站条目实施 7 天保留期：条目创建超过 7 天后必须被永久删除（best-effort），以防止回收站无限增长。

系统必须 (MUST) 自动触发清理（best-effort），无需用户手动调用专用清理工具。

#### Scenario: 超过 7 天的回收站条目在清理后被删除
- **GIVEN** 回收站中存在一个创建时间超过 7 天的条目
- **WHEN** 系统执行一次回收站清理
- **THEN** 该条目被永久删除（payload 与 metadata 不再存在）

### Requirement: `write_file` MUST NOT silently truncate content
系统必须 (MUST) 确保 `write_file` 不会在无错误信号的情况下写入“被截断的内容”并返回成功（silent partial write）。

当单次入参 `content` 超过系统允许的最大大小时，系统必须 (MUST)：
- 明确失败（tool result 中 `ok=false` 且返回可理解的错误信息）
- 不写入/不改动目标文件（避免产生半成品）
- 给出 best-effort 的下一步建议（例如使用 `append=true` 分段写入）

#### Scenario: Oversize content fails without modifying existing file
- **GIVEN** 文件 `a.txt` 已存在且内容为 `old`
- **WHEN** agent 调用 `write_file(filePath="a.txt", content=<oversize>)`
- **THEN** 工具返回 `ok=false`
- **AND** `a.txt` 内容仍为 `old`
