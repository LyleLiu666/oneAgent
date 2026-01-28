## 1. Implementation
- [ ] 1.1 Add spec deltas for cost/token limits + usage accounting
- [ ] 1.2 Extend task limits schema (API + persistence) to include cost/token budgets
- [ ] 1.3 Implement per-attempt usage aggregation across providers (normalized model)
- [ ] 1.4 Enforce budgets in runner (stop attempt when exceeded; record reason)
- [ ] 1.5 Surface usage in UI + receipts (human readable)
- [ ] 1.6 Run `openspec validate add-cost-governance-limits --strict --no-interactive` and keep tests green

