## ADDED Requirements
### Requirement: Work Ledger 数据落盘（本地模式）
系统必须 (MUST) 在本地模式下将 Work Ledger 相关数据持久化到 `ONEAGENT_HOME/.oneagent/data/` 下，并建议采用按类型分层的目录结构（示意）：
- `.../data/ledger/receipts/<receipt_id>/receipt.json`
- `.../data/ledger/receipts/<receipt_id>/receipt.md`
- `.../data/ledger/digests/<principal_id>/YYYY-MM-DD.md`
- `.../data/ledger/sop_suggestions/<suggestion_id>/suggestion.json`

系统必须 (MUST) 确保 receipts 在服务重启后仍可查询到（不得依赖仅内存索引）。

#### Scenario: 重启后 receipts 仍可查询
- **GIVEN** 系统已生成并落盘至少一条 receipt
- **WHEN** oneAgent 重启
- **THEN** 用户仍可通过 ledger 查询到该 receipt
