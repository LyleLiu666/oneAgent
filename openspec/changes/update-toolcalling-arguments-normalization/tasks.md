## 1. Implementation
- [x] 1.1 Add spec deltas for tool arguments normalization/repair behavior
- [ ] 1.2 Backend: extend `normalizeToolArguments` to handle common dirty patterns (fences, prefix/suffix text, quoted JSON, multi-JSON)
- [ ] 1.3 Backend tests: unit tests for `normalizeToolArguments` covering real-world patterns
- [x] 1.4 Run `openspec validate update-toolcalling-arguments-normalization --strict --no-interactive`
- [ ] 1.5 Run `cd backend && go test ./...`
