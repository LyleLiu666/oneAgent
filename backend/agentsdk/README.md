# agentsdk

This folder is intended to be extracted into a standalone Go module later.

Current scope:
- `agentsdk`: minimal SDK-owned interfaces/types (LLM client, messages, tool execution, observability events).
- `agentsdk/xmlprotocol`: an XML `<tool_data>` toolcalling loop (`RunLoop`) with streaming visibility filtering and best-effort protocol repair.

## Quickstart (XML toolcalling loop)

1) Implement `agentsdk.Client` for your LLM provider.
2) Register tools with `agentsdk.ToolRegistry` (or provide a custom `agentsdk.ToolExecutor`).
3) Run the loop via `xmlprotocol.RunLoop`.

```go
reg := agentsdk.NewToolRegistry()
_ = reg.Register("bash", func(ctx context.Context, call agentsdk.ToolCall) (any, error) {
  // call.Fields contains all top-level tags from <call> (unknown tags are preserved).
  return map[string]any{"stdout_delta": "hi\n"}, nil
})

sink := agentsdk.EventSinkFunc(func(ctx context.Context, ev agentsdk.Event) {
  // Persist ev for full observability/replay.
})

out, err := xmlprotocol.RunLoop(ctx, xmlprotocol.RunLoopInput{
  Client: client,
  Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
  Executor: reg,
  Callbacks: xmlprotocol.Callbacks{
    EventSink: sink,
  },
})
_ = out
_ = err
```

## Observability hook

Use `agentsdk.EventSink` to capture an append-only timeline:
- `llm_request` / `llm_response` (raw + visible)
- `tool_call` / `tool_result`
- `trace`
- `error` (terminal failures + tool-protocol anomalies)

The SDK does not implement storage or redaction. Consumers own persistence.

## Tests

- Fast path: `cd backend && go test ./agentsdk/...`
- Full backend: `cd backend && go test ./...`

See:
- `backend/agentsdk/TEST_MATRIX.md`
- `backend/agentsdk/TEST_DESIGN.md`

