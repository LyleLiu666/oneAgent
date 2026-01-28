## 1. Implementation
- [ ] 1.1 Add spec deltas for test report artifact + evidence UX
- [ ] 1.2 Extend receipt/artifact schema to include `test_report_path`
- [ ] 1.3 Implement best-effort test report generation per workspace type (go/node/python) (configurable)
- [ ] 1.4 Update Outcome Observer prompt to request/expect test report artifacts when relevant
- [ ] 1.5 Add tests: receipt includes test_report_path; observer reads report only (no command execution)
- [ ] 1.6 Run `openspec validate add-task-evidence-policy --strict --no-interactive` and keep tests green

