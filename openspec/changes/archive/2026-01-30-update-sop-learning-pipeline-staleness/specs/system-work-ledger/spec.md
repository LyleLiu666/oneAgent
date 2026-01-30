## ADDED Requirements

### Requirement: Learning jobs MUST produce governance hints for dedupe/merge (best-effort)
系统必须 (MUST) 在学习管线（learning job）中为每条 SOP suggestion 产出可用于治理的线索（best-effort），以降低人工去重成本：
- 至少包含一个 `similar_suggestion_ids[]` 或等价结构（best-effort）
- 当系统能找到明显的 merge 目标时，输出 `recommended_merge_target_id`（best-effort）
- hints 不得替代证据：suggestion 仍必须包含 `evidence_receipt_ids[]`（best-effort）

#### Scenario: Suggestion includes similar hints and preserves evidence
- **GIVEN** 系统生成了一条 SOP suggestion（best-effort）
- **WHEN** 客户端获取该 suggestion 详情
- **THEN** 返回包含 `evidence_receipt_ids[]`（best-effort）
- **AND** 返回包含 `similar_*` 治理线索字段（best-effort）

