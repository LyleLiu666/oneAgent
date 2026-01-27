## 1. Implementation
- [x] 1.1 Define task data model + file-based store (task.json + events.jsonl, attempts, artifacts)
- [x] 1.2 Implement Outcome Observer (read-only, expectation-based success/fail)
- [x] 1.3 Implement TaskRunner (per-workspace FIFO, cross-workspace parallel, resume + interrupted on restart)
- [x] 1.4 Add backend APIs: create/list/get/cancel/resume/events
- [x] 1.5 Add minimal UI: workspace task queue panel + task detail (events + artifacts links)
- [x] 1.6 Add backend e2e tests for queue ordering, persistence, cancel/resume, interrupted recovery, and “runs without UI”
- [x] 1.7 Add frontend unit tests for queue list + task state rendering
