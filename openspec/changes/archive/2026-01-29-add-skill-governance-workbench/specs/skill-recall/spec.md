## ADDED Requirements

### Requirement: Archived skills MUST NOT be recalled
系统必须 (MUST) 支持将技能归档（archived），归档后的技能不得参与 recall / 搜索结果，以避免过时资产污染召回质量。

#### Scenario: archive 后不再出现在 recall
- **GIVEN** 用户将某个 oneAgent personal skill 归档
- **WHEN** 系统执行 skill recall/search
- **THEN** 归档技能不应出现在候选列表中

