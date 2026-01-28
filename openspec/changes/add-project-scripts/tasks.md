## 1. Implementation
- [x] 1.1 Add spec deltas for `.oneagent/project.json` and attempt script lifecycle
- [ ] 1.2 Backend: discover/parse `.oneagent/project.json` for a workspace (validate schema, actionable errors)
- [ ] 1.3 Backend: run `setup_script/test_script/cleanup_script` with tool permissions enforced; persist logs as artifacts
- [ ] 1.4 Backend tests: script execution success/failure cases and artifact paths in receipt/attempt
- [ ] 1.5 Frontend: surface project scripts presence + latest logs (minimal view is enough)
- [ ] 1.6 Frontend unit tests
- [x] 1.7 Run `openspec validate add-project-scripts --strict --no-interactive` and keep tests green
