## ADDED Requirements

### Requirement: Receipts MUST reference artifact manifest version and completeness status
Receipt records MUST include a pointer to the attempt artifact manifest version and a completeness status, so review tools can quickly determine whether evidence is sufficient for acceptance.

#### Scenario: Receipt includes manifest reference
- **GIVEN** an attempt has generated artifact manifest v1
- **WHEN** the system materializes the receipt
- **THEN** the receipt references manifest version and path (or equivalent pointer)
- **AND** includes a completeness status that is queryable by ledger consumers (best-effort)

### Requirement: Evidence completeness MUST be queryable from ledger APIs (best-effort)
Ledger and digest readers MUST be able to query whether each receipt is evidence-complete, evidence-partial, or evidence-insufficient (best-effort), without opening full artifacts manually.

#### Scenario: Ledger list includes evidence completeness classification
- **GIVEN** a user queries receipt list
- **WHEN** the system returns receipt summaries
- **THEN** each summary includes an evidence completeness classification (best-effort)
