## 1. Implementation
- [x] 1.1 Add spec deltas for worktree execution mode + lifecycle management
- [ ] 1.2 Backend: detect git workspace + resolve base ref/SHA
- [ ] 1.3 Backend: create/manage git worktrees per attempt (create/cleanup/orphan cleanup)
- [ ] 1.4 Backend: run attempt tools/scripts rooted at worktree path (and support copy_files)
- [ ] 1.5 Backend tests: worktree create/cleanup + artifact pointers in receipts
- [ ] 1.6 Frontend: surface worktree path + open/view diff entrypoints (minimal)
- [ ] 1.7 Frontend unit tests
- [x] 1.8 Run `openspec validate add-worktree-attempt-isolation --strict --no-interactive` and keep tests green
