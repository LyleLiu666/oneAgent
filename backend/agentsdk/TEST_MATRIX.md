# agentsdk "Authoritative" Test Matrix

This folder is intended to be extracted into a standalone `agentsdk` module later.
Tests here are treated as the contract: if the tests are strong, the SDK is strong.

## Sources (local)

- opencode: `/Users/liu_y/code/opensource/opencode`
  - Agent config/permissions: `packages/opencode/test/agent/agent.test.ts`
  - ACP interface conformance: `packages/opencode/test/acp/agent-interface.test.ts`
  - Event subscription isolation: `packages/opencode/test/acp/event-subscription.test.ts`
  - Retry/backoff semantics: `packages/opencode/test/session/retry.test.ts`
  - Context overflow/compaction: `packages/opencode/test/session/compaction.test.ts`
  - Tool output truncation + cleanup: `packages/opencode/test/tool/truncation.test.ts`
- OpenClaw (moltbot): `/Users/liu_y/code/opensource/moltbot`
  - Thinking-tag stripping is code-aware: `src/agents/pi-embedded-subscribe.code-span-awareness.test.ts`
  - Session write-lock robustness: `src/agents/session-write-lock.test.ts`
  - Conformance payload JSON-serializable: `src/agents/tool-policy.conformance.test.ts`
  - Streaming chunking correctness (fences): `src/agents/pi-embedded-subscribe.subscribe-embedded-pi-session.reopens-fenced-blocks-splitting-inside-them.test.ts`
- openagentic-sdk (lower priority): `/Users/liu_y/code/opensource/openagentic-sdk`
  - Event serialization roundtrip: `tests/test_user_message_event.py`

## What We Test (high priority)

- XML protocol parsing
  - Tolerate common malformed XML from LLMs (tag typos, missing open tags, stray CDATA markers).
  - Multiple `<call>` blocks in one `<tool_data>`.
  - Stable field normalization for common aliases.
  - Existing: `backend/agentsdk/xmlprotocol/parser_test.go`.
- Streaming visibility filter
  - Never leak `<tool_data>...</tool_data>` into user-visible stream.
  - Never leak thinking blocks (`<thinking>`, `<think>`, etc) into user-visible stream.
  - Must work across chunk boundaries and must never panic on Unicode edge cases.
  - Existing: `backend/agentsdk/xmlprotocol/stream_filter_test.go`.
- Agent loop semantics
  - Correct step iteration: LLM -> parse tool_data -> execute tools -> append `<tool_result>` -> loop.
  - Protocol self-heal on invalid/truncated tool_data: continue with `tool_protocol` tool_result instead of terminating.
  - Max steps cap / deterministic tool_call_id generation.
  - Existing: `backend/agentsdk/xmlprotocol/engine_test.go`.
- Safety: "no accidental tool execution"
  - Tool protocol markers inside Markdown code spans/fences MUST NOT be treated as real tool calls.
  - Thinking tags inside code spans/fences MUST NOT be stripped.
  - Inspired by OpenClaw's code-span-awareness tests.

## What We Test (medium priority)

- Observability + replay hooks
  - Every step emits structured, append-only events (`llm_request`, `llm_response`, `tool_call`, `tool_result`, `error`).
  - Events must be JSON-serializable (or the SDK must provide a canonical serialization).
  - Existing (partial): `backend/agentsdk/xmlprotocol/engine_test.go`.
- Tool output size control
  - Prevent huge tool outputs from blowing context: truncate + store full output artifact + cleanup policy.
  - Inspired by opencode truncation tests.

## What We Test (future / optional)

- Retry/backoff policy helpers (if SDK takes ownership of retries)
  - Retry-After parsing (`retry-after-ms`, seconds, http-date), exponential fallback, sensible caps.
  - Inspired by opencode `session/retry` tests.
- Persistence safety for log sinks
  - File lock correctness, stale lock reclaim, cleanup on signals/exit, symlink canonicalization.
  - Inspired by OpenClaw `session-write-lock` tests.
- Conformance tests for any public interfaces
  - E.g. "SDK implements required interface methods" style tests (like opencode ACP conformance).

