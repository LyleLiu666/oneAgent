## ADDED Requirements

### Requirement: Receipt MUST reference diff artifacts when available
系统必须 (MUST) 在 attempt 产生“变更证据”（diff/变更摘要）时，将其引用写入 Receipt（例如 `diff_patch_path` / `changed_files_path`），以便用户在 Work Ledger/Digest 中直接打开审查证据。

#### Scenario: Receipt includes diff artifact pointers
- **GIVEN** 某次 attempt artifacts 中包含 `diff_patch_path`（或等价字段）
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含对应 diff 指针字段

#### Scenario: Receipt remains valid without diff artifacts
- **GIVEN** 某次 attempt 未产生 diff artifacts（例如无文件改动）
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt 仍然是有效证据条目（包含 summary/findings/trace 等其他字段）

### Requirement: Review comments MUST be preserved in the evidence chain
系统必须 (MUST) 支持用户对某次 attempt/receipt 提交 review comments，并将其作为 append-only 的证据写入 Work Ledger（例如 `review_comments.jsonl` 或等价结构），以保证：
- 复盘时可看到“用户审查意见如何影响后续交付”
- follow-up attempt 可引用并注入这些 comments

#### Scenario: User submits a review comment and it is persisted
- **GIVEN** 用户打开某条 receipt 的 review 页面
- **WHEN** 用户提交一条 review comment（文本即可）
- **THEN** 系统持久化该 comment 并关联到对应 receipt_id/attempt_id
- **AND** comment 可在后续查询 receipt 详情时被读取（best-effort）

