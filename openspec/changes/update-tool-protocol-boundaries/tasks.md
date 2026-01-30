## 1. Implementation
- [x] 1.1 Add spec deltas for tool protocol boundaries + auto fallback
- [ ] 1.2 Backend: default to JSON tool calling when provider supports tools (best-effort)
- [ ] 1.3 Backend: auto fallback to XML when tools are requested but provider lacks tool support (best-effort)
- [ ] 1.4 Backend: gate XML tool set to only supported tools; reject unsupported tool calls early
- [ ] 1.5 Tests: add protocol selection + XML unsupported tool coverage
- [x] 1.6 Run `openspec validate update-tool-protocol-boundaries --strict --no-interactive`
- [ ] 1.7 Run `cd backend && go test ./...`

