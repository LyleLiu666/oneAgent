## 1. Implementation
- [x] 1.1 Add spec deltas for enabling tools on OpenAI Responses provider
- [x] 1.2 Backend: implement `ChatCompletionWithTools` for Responses provider (MVP: non-stream)
- [x] 1.3 Backend: map tool result messages into Responses input (tool_call_id aware; best-effort)
- [x] 1.4 Backend tests: e2e tool loop on mocked `/responses` (tool_call -> tool -> completion)
- [x] 1.5 Backend: implement `ChatCompletionStreamWithTools` for Responses provider (optional; best-effort)
- [x] 1.6 Run `openspec validate update-openai-responses-toolcalling --strict --no-interactive`
- [x] 1.7 Run `cd backend && go test ./...`
