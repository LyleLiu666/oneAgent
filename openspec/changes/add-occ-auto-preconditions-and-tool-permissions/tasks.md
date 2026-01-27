## 1. Implementation
- [x] 1.1 Add `local-runtime` spec delta for OCC auto-preconditions and tool permissions
- [x] 1.2 Add OCC state + context wiring (chat + task runner)
- [x] 1.3 Record fingerprints in `read_file` (bounded by file size)
- [x] 1.4 Auto-inject `expected_sha256` for `write_file`/`edit` and update fingerprint after write
- [x] 1.5 Add unit tests for OCC auto-preconditions (drift detection + self-update)
- [x] 1.6 Add tool permission controls: disable-by-env + bash `rm` guard
- [x] 1.7 Add unit tests for tool permission controls
- [x] 1.8 Run `openspec validate add-occ-auto-preconditions-and-tool-permissions --strict --no-interactive` and keep tests green
