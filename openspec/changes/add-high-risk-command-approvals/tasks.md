## 1. Spec & Design (planning first)
- [x] 1.1 Define `approval=high_risk` semantics for command tools
- [x] 1.2 Define auto vs manual approval modes (default auto)
- [x] 1.3 Define high-risk command classification v1 list

## 2. Implementation (after proposal approval)
- [x] 2.1 Backend: implement high-risk detection for `bash` and `run_command(start)`
- [x] 2.2 Backend: add user setting `command_approval_mode` (auto|manual) + API
- [x] 2.3 Backend: enforce `approval=high_risk` and auto/manual behavior in approval gate
- [x] 2.4 Default policy: enable `approval=high_risk` for command tools (best-effort)
- [x] 2.5 Frontend: Settings UI toggle + wiring

## 3. Tests & Validation
- [x] 3.1 Unit tests: high-risk detection & approval gating (auto/manual)
- [x] 3.2 API tests: get/update approval settings
- [x] 3.3 Frontend tests: settings toggle persistence & API calls
- [x] 3.4 `go test ./...` (backend)
- [x] 3.5 `npm test -- --run` (frontend)
- [x] 3.6 `openspec validate add-high-risk-command-approvals --strict --no-interactive`
