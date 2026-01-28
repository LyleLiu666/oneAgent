# Design: Cost/token governance

## Budget fields (proposal)
- `limits.max_total_tokens` (int, optional)
- `limits.max_cost_usd` (float, optional)

## Normalized usage record (proposal)
Per LLM call:
- provider_id / model_id
- input_tokens / output_tokens
- cached_input_tokens (optional)
- cost_usd (optional; computed if provider supplies or we have pricing config)

Per attempt aggregate:
- total_input_tokens / total_output_tokens / total_tokens
- total_cost_usd

## Enforcement semantics
- When budget exceeded: stop further execution, mark attempt terminal with reason `limit_exceeded` (surfaced in summary)
- Resume: user can increase limits or split task; previous attempt usage remains visible

