## 1. Spec & Design (planning first)
- [x] 1.1 Define `sandbox_mode=native` semantics in OpenSpec deltas
- [x] 1.2 Define acceptance tests/scenarios (rm in-root ok, out-of-root denied)
- [x] 1.3 Decide Linux/Windows fallback behavior (fail vs readonly)

## 2. Implementation (after proposal approval)
- [x] 2.1 Introduce a unified sandbox policy abstraction for command tools (none/docker/native)
- [x] 2.2 Implement macOS native backend (Seatbelt via `/usr/bin/sandbox-exec`)
- [x] 2.3 Implement Linux native backend (Landlock; best-effort)
- [ ] 2.4 Implement Windows native backend (Restricted Token + ACL; best-effort)
- [x] 2.5 Allow safe deletion commands (e.g., `rm`) when sandbox is hard-boundary (native/docker)
- [x] 2.6 Add doctor diagnostics for native sandbox availability

## 3. Tests & Validation
- [x] 3.1 Unit tests: sandbox policy selection, env normalization, path boundaries
- [x] 3.2 Integration tests per OS (skip when unavailable): in-root delete ok; out-of-root denied
- [x] 3.3 `go test ./...` (backend)
- [x] 3.4 `openspec validate add-native-command-sandbox --strict --no-interactive`
