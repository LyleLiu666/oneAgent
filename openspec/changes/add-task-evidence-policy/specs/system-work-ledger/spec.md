## ADDED Requirements

### Requirement: Receipts MUST reference test reports when available
系统必须 (MUST) 在存在测试报告时，将其引用写入 Receipt（例如 `test_report_path`），以便用户在 digest/ledger 中直接打开证据。

#### Scenario: Receipt includes test_report_path
- **GIVEN** 某次 attempt 产生了测试报告文件
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含 `test_report_path`
