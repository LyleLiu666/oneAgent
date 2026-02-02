## 1. Specs
- [ ] 1.1 Add `system-workflow-orchestration` delta requirements for artifact layout + path-only handoffs
- [ ] 1.2 Run `openspec validate add-workflow-artifact-layout --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Add path helpers: compute `RunRoot`/`NodeRoot` under workflow store (durable)
- [ ] 2.2 Implement node executor that writes `LEDGER.md`/`FINDINGS.md` and collects `deliverables/*`
- [ ] 2.3 Persist `ArtifactManifest` + gate report paths into `run.json`
- [ ] 2.4 Write `inputs.json` for each node_run using upstream artifact *paths only*
- [ ] 2.5 Add retention cleanup for finished workflow runs (configurable)
- [ ] 2.6 Tests: path determinism, no-content handoff, retention cleanup

## 3. Frontend
- [ ] 3.1 Render artifacts with “open/copy” actions (clickable, per spec)
- [ ] 3.2 Surface `LEDGER.md`/`FINDINGS.md` as first-class artifacts (not just generic list)
- [ ] 3.3 Tests: run view renders artifacts + copy works

