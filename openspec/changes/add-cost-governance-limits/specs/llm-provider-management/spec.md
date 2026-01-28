## ADDED Requirements

### Requirement: Provider/model configuration MUST support usage/cost normalization
系统必须 (MUST) 提供一套方式将不同 provider 的 usage 字段归一为统一结构，并在可得时计算或记录 `cost`，以支持预算治理与可观测性。

备注：当 provider 未返回 usage/cost 时，系统可以使用稳定的 heuristic 做 best-effort 估算（用于展示/治理），并在 UI 上明确这是估算值。

#### Scenario: Usage is recorded per call and aggregated per attempt
- **WHEN** 系统执行一次 LLM 调用
- **THEN** 系统记录该次调用的 normalized usage
- **AND** task attempt 可查询到累计 usage（tokens/cost）
