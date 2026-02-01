## 1. Spec
- [x] Add/modify spec deltas for `system-toolcalling-reliability` (configurable chat tool max steps)
- [x] Add/modify spec deltas for `system-file-tools` (write_file must not truncate; fail safely)
- [x] Run `openspec validate update-tool-loop-limits-and-write-file-no-truncate --strict --no-interactive`

## 2. Backend: Tool Loop Limits
- [x] Make chat tool loop max steps configurable (env) and raise default above 20 (JSON tools loop)
- [x] Make XML fallback loop max steps configurable (env) and raise default above 20
- [x] Add tests for maxSteps env override (JSON + XML)

## 3. Backend: write_file Non-truncating
- [x] Change `write_file` to reject oversize content with a clear error and NO partial write
- [x] Update `write_file` definition/prompt docs to remove “auto truncation” semantics and document the new behavior
- [x] Update/replace truncation test with “reject too large” test

## 4. Verification
- [x] Run `cd backend && go test ./...`
