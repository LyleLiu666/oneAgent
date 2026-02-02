## 1. Specs
- [ ] 1.1 Add `system-workflow-orchestration` delta requirements for artifact layout + path-only handoffs
- [ ] 1.2 Run `openspec validate add-workflow-artifact-layout --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Add path helpers: compute `RunRoot`/`NodeRoot` under workflow store (durable)
- [ ] 2.2 Extend graph schema: node-level `principal_id`/`model`/`skills[]` config (published + run snapshot)
- [ ] 2.3 Implement node executor: run in workspace/worktree `ExecutionRoot`, then export declared deliverables to `NodeRoot/deliverables/`
- [ ] 2.4 Persist `ArtifactManifest` + gate report paths + best-effort `diff_from_inputs` evidence into `run.json`
- [ ] 2.5 Write `inputs.json` for each node_run using upstream artifact *paths only*
- [ ] 2.5 Add retention cleanup for finished workflow runs (configurable)
- [ ] 2.6 Tests: path determinism, no-content handoff, retention cleanup

## 3. Frontend
- [ ] 3.1 Render artifacts with “open/copy” actions (clickable, per spec)
- [ ] 3.2 Editor: configure node `skills/model/principal` (persisted in published version)
- [ ] 3.3 Surface `LEDGER.md`/`FINDINGS.md` as first-class artifacts (not just generic list)
- [ ] 3.4 Tests: editor saves node config + run view renders artifacts
