## 1. Specs
- [ ] 1.1 Add `system-workflow-orchestration` delta requirements (graph model, node execution, artifacts, gates, resume)
- [ ] 1.2 Run `openspec validate add-workflow-orchestration-graph --strict --no-interactive`

## 2. Backend (MVP)
- [ ] 2.1 Data model: workflow / workflow_version / workflow_run / node_run
- [ ] 2.2 Storage: persist definitions + run snapshots (workspace-scoped)
- [ ] 2.3 Execution: DAG scheduling (deps, concurrency limit, cancel/resume)
- [ ] 2.4 Artifacts: node_run produces artifact manifest (multi-file pointers)
- [ ] 2.5 Gates: run hard/soft evaluation and attach report to node_run evidence
- [ ] 2.6 Backend tests: persistence + scheduler + resume invariants

## 3. Frontend (MVP)
- [ ] 3.1 Workflow list/create/rename/delete (workspace-scoped)
- [ ] 3.2 Workflow editor (table-first): nodes/edges/config + version publish
- [ ] 3.3 Workflow run view: per-node status + events/log waterfall + artifact pointers
- [ ] 3.4 Frontend tests: editor basics + run status rendering

## 4. Validation
- [ ] 4.1 `cd backend && go test ./...`
- [ ] 4.2 `cd frontend && npm test -- --run`

