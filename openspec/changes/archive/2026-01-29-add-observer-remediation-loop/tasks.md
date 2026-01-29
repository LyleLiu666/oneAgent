## 1. Implementation
- [x] 1.1 Add spec deltas for observer next_steps + auto follow-up attempt
- [x] 1.2 Backend: extend observer decision schema + prompt to output `next_steps`
- [x] 1.3 Backend: add `limits.max_auto_attempts` with defaults + env policy
- [x] 1.4 Backend: auto-create follow-up attempt on observer fail (bounded + auditable)
- [x] 1.5 Backend tests: observer fail triggers follow-up + cap stops auto loop
- [x] 1.6 Frontend: display observer next_steps (simple-by-default)
- [x] 1.7 Frontend unit tests
- [x] 1.8 Run `openspec validate add-observer-remediation-loop --strict --no-interactive` and keep tests green
