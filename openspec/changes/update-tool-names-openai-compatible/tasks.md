## 1. Implementation
- [x] 1.1 Add spec deltas: canonical underscore tool names + legacy dotted aliases
- [ ] 1.2 Backend: rename tool function names to underscore form and add alias mapping for legacy dotted names
- [ ] 1.3 Prompt/tool manuals: standardize filenames and examples to underscore names
- [ ] 1.4 Frontend/UI: update any hardcoded tool name references (best-effort)
- [ ] 1.5 Tests: verify both canonical+alias names work; ensure provider tool registration is stable
- [x] 1.6 Run `openspec validate update-tool-names-openai-compatible --strict --no-interactive`
- [ ] 1.7 Run `cd backend && go test ./...` (and frontend tests if touched)
