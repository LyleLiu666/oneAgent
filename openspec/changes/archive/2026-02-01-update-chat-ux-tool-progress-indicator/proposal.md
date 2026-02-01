# Change: Show tool progress in secretary mode

## Why
Long-running tool calls can leave users unsure whether the LLM is still working, especially in secretary mode where tool cards are hidden by default.

## What Changes
- Add a compact, visible progress indicator for in-flight tool calls in secretary mode.
- Show tool name/count and current response token count while the model is streaming.
- Allow reusing existing tool cards in secretary mode when configured (best-effort).

## Impact
- Affected specs: chat-ux
- Affected code: frontend chat message rendering + tool message display
