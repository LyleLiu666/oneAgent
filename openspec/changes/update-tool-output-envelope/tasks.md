## 1. Implementation
- [x] 1.1 Add spec deltas for JSON envelope tool outputs/errors
- [ ] 1.2 Backend: wrap tool outputs/errors into a stable JSON envelope (still bounded as untrusted)
- [ ] 1.3 Backend: add actionable `error_code/retryable/hint` for common tool loop failures (best-effort)
- [ ] 1.4 Tests: add regression tests asserting envelope JSON validity + key fields
- [x] 1.5 Run `openspec validate update-tool-output-envelope --strict --no-interactive`
- [ ] 1.6 Run `cd backend && go test ./...`

