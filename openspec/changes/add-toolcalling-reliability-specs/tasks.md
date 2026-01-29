## 1. Implementation
- [x] 1.1 Add spec deltas for `system-toolcalling-reliability` (import/refer expert docs, define protocol long-text tests)
- [x] 1.2 Backend tests (XML): long-text (>=3000 chars) CDATA parsing + args build is lossless
- [x] 1.3 Backend tests (JSON vs XML): long-text (>=3000 chars) args encoding/decoding + size comparison for code-like payloads
- [x] 1.4 Run `openspec validate add-toolcalling-reliability-specs --strict --no-interactive`
- [x] 1.5 Run `cd backend && go test ./...`
