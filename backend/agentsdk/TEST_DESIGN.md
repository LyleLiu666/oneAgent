# agentsdk Test Design

This document explains what the `agentsdk` test suite is trying to guarantee, and why.
Treat tests as the contract: a change that breaks these tests is considered a breaking change in behavior.

## Goals

- Correctness: parse and execute XML tool calls deterministically.
- Safety: never execute tool calls that appear inside Markdown code spans or fenced code blocks.
- Robustness: tolerate malformed / streaming / truncated model output without panics.
- Observability: emit enough structured events to make runs fully inspectable and replayable.
- Portability: keep the SDK loop independent from oneAgent internals (extractable module later).

## Non-goals

- Sensitive data redaction: the SDK does not attempt to mask or filter content.
- Provider/network retries: backoff/retry logic is a future optional layer (see references).
- Storage choices: persistence is handled by SDK consumers via hooks (EventSink).

## Test Layers

- Unit: small pure functions (parser, stream filter) with minimal harness.
- Loop integration: scripted LLM + fake ToolExecutor (covers step semantics and hook emissions).
- Contract: public event payloads are JSON-serializable (log sinks should not break).

## Canonical Scenarios

Each scenario below should be covered by a focused test (or a small set of tests).

### XML Protocol

- `XML-1 Basic call parse`
  - Input: single `<tool_data>` with one tool call.
  - Expect: tool name + fields parsed.
  - Tests: `backend/agentsdk/xmlprotocol/parser_test.go`
- `XML-2 Multi-call parse`
  - Input: `<tool_data>` containing multiple `<call>` blocks.
  - Expect: stable order, stable field extraction.
  - Tests: `backend/agentsdk/xmlprotocol/parser_test.go`
- `XML-3 Malformed but recoverable XML`
  - Input: tag typos, missing open tags, stray CDATA markers.
  - Expect: parse still succeeds when possible.
  - Tests: `backend/agentsdk/xmlprotocol/parser_test.go`

### Streaming Filter

- `STREAM-1 No leakage of tool_data/thinking`
  - Input: streamed chunks containing `<thinking>` and `<tool_data>` blocks.
  - Expect: user-visible text never includes these blocks.
  - Tests: `backend/agentsdk/xmlprotocol/stream_filter_test.go`
- `STREAM-2 Unicode panic regression`
  - Input: edge-case Unicode sequences that previously caused ToLower/index slicing bugs.
  - Expect: never panic.
  - Tests: `backend/agentsdk/xmlprotocol/stream_filter_test.go`
- `STREAM-3 Code fences across chunk boundaries`
  - Input: Markdown fences split across streamed chunks (e.g. "``" + "`").
  - Expect: content inside fences is preserved (no accidental tool_data/thinking suppression).
  - Tests: `backend/agentsdk/xmlprotocol/stream_filter_test.go`

### Safety: Markdown Code Awareness (must-have)

- `SAFE-1 Thinking tags inside code are preserved`
  - Input: `` `<thinking>` `` or fenced block containing `<thinking>...</thinking>`.
  - Expect: stripping logic does not remove content inside code spans/fences.
  - Tests: `backend/agentsdk/xmlprotocol/code_awareness_test.go`
- `SAFE-2 tool_data inside code is ignored`
  - Input: inline/fenced code containing `<tool_data>...</tool_data>`.
  - Expect: extraction does not treat it as a real tool call.
  - Tests: `backend/agentsdk/xmlprotocol/code_awareness_test.go`
- `SAFE-3 RunLoop never executes tool_data inside code`
  - Input: model output where `<tool_data>` appears only inside fenced code.
  - Expect: ToolExecutor not invoked; run terminates normally.
  - Tests: `backend/agentsdk/xmlprotocol/code_awareness_test.go`

### Loop Semantics

- `LOOP-1 Happy path tool loop`
  - Input: LLM emits tool_data -> tool executes -> LLM returns final.
  - Expect: correct message history append order; combined visible output correct.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `LOOP-1b Multi-tool calls in a single tool_data`
  - Input: `<tool_data>` contains multiple `<call>` blocks.
  - Expect: tools execute in order; tool_call_id stable; tool_result contains all calls.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `LOOP-2 Self-heal on tool protocol parse errors`
  - Input: invalid `<tool_data>` (e.g., empty tool name).
  - Expect: loop continues using a `tool_protocol` tool_result, then terminates.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `LOOP-3 Self-heal on truncated tool_data`
  - Input: `<tool_data>` start tag present but missing end tag.
  - Expect: loop continues using a `tool_protocol` tool_result, then terminates.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `LOOP-4 Special-case truncated write_file append recovery`
  - Input: truncated tool_data where write_file append can be safely recovered.
  - Expect: write_file executed in append-only mode; warning emitted as tool_protocol result.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `LOOP-5 ToolExecutor error handling`
  - Input: tool execution returns an error.
  - Expect: tool_result `ok=false` with both a human error and a JSON payload; loop continues.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`

### Observability / Replay Hooks

- `OBS-1 EventSink emits a complete timeline`
  - Expect: `llm_request` then `llm_response`, plus `tool_call` and `tool_result` events when tools run.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `OBS-2 Trace is structured`
  - Expect: any trace message is also emitted as `EventKindTrace` so sinks can persist it.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `OBS-3 JSON serializable events`
  - Expect: `json.Marshal(agentsdk.Event)` never fails for events emitted by the loop.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`
- `OBS-4 Streaming callback aborts are observable`
  - Input: OnContent returns an error mid-stream.
  - Expect: RunLoop aborts and llm_response is logged with error text.
  - Tests: `backend/agentsdk/xmlprotocol/engine_test.go`

## Running Tests

- Fast path: `cd backend && go test ./agentsdk/...`
- Full backend: `cd backend && go test ./...`

## References (local repos)

See `backend/agentsdk/TEST_MATRIX.md` for the curated list of external test sources and why they matter.
