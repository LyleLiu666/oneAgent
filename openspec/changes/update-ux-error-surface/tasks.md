## 1. Implementation
- [x] 1.1 Backend: add `request_id` middleware (header + JSON field)
- [x] 1.2 Backend: implement centralized error responder (code/message/hint + redaction)
- [x] 1.3 Backend: replace direct `err.Error()` JSON responses in `backend/internal/handler/*`
- [x] 1.4 Frontend: add shared error parsing + `ErrorBanner` component with optional details + copy `request_id`
- [ ] 1.5 Frontend: apply unified error UX to `/tasks`, `/governance/*`, `/documents/export`, `/ledger`
- [ ] 1.6 Frontend: implement page-scoped UX fixes per spec (empty states, density, editor usability)

## 2. Tests
- [x] 2.1 Backend: unit tests for error redaction (paths/policy/internal strings)
- [ ] 2.2 Frontend: view tests for error banner + key empty states (do not couple to UI text)

## 3. Validation
- [ ] 3.1 Run `openspec validate update-ux-error-surface --strict --no-interactive`
- [ ] 3.2 Run `go test ./...`
- [ ] 3.3 Run frontend tests (e.g. `pnpm test`)
