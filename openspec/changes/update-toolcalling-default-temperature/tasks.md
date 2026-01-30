## 1. Implementation
- [x] 1.1 Add spec deltas for tool-enabled default temperature=0
- [x] 1.2 Backend: set default `opts.Temperature=0` when tools are enabled (unless explicitly set)
- [x] 1.3 Backend tests: e2e asserts provider request includes `temperature: 0` when tool_ids non-empty
- [x] 1.4 Run `openspec validate update-toolcalling-default-temperature --strict --no-interactive`
- [x] 1.5 Run `cd backend && go test ./...`
