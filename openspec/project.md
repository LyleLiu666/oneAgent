# Project Context

## Purpose
oneAgent is a **local-first agent client** that helps users complete real work end-to-end on their own computer.

The core idea is: users should be able to **install once, choose a workspace folder, and start working immediately**—with an agent that can run for hours, produce industrial-grade deliverables, and leave traceable evidence.

This project is designed for users who:
- Have a personal “domain”: writing, planning, coding, research, email handling, etc.
- Want both **custom workflows** (their own know-how) and a **general all-purpose assistant**.
- Want the agent to **use their local files as working context** (manuals, guidelines, playbooks, specs).

We model reusable know-how as **Skills**: portable, composable, and governable assets that encapsulate workflow + prompt + references/scripts/templates.
We also provide **skill recall** for users who want “omniscient assistant” behavior, but in a controllable and auditable way.

## Tech Stack
- Backend: Go
- Frontend: TypeScript + Vue
- Runtime: local execution (workspace + tools), with optional remote LLM providers

## Project Conventions

### Code Style
- Prefer small, focused changes and stable interfaces.
- Keep behavior spec-driven via OpenSpec. Specs are truth; changes are proposals.
- Favor explicitness and evidence over “looks good” output.

### Architecture Patterns
- **Client-first** UX: a “desktop-client experience” even when delivered as a local web UI.
- **Agentic execution**: long-running tasks, resumable attempts, and clear completion criteria.
- **Subagent isolation**: delegate atomic operations to subagents to reduce main-thread context pollution.
- **Evidence & replay**: durable logs, receipts, and artifacts to enable audit, rollback, and learning.

### Testing Strategy
- TDD mindset: every change should have tests and an end-to-end verification path.
- Prefer integration tests for critical workflows (API + filesystem + artifacts).
- CI should keep “daily green”: backend tests + frontend tests + OpenSpec validation.

### Git Workflow
- Keep commits small and coherent.
- Avoid mixing unrelated changes.
- Prefer “spec → implement → validate → archive change” as the working loop.

## Domain Context
### Skills (workflow + prompt as assets)
Skills are the primary way to store and reuse know-how:
- Skills can include templates, references, and scripts when needed.
- Skills must be **governable**: dedupe, merge/pin canonical, archive/drop obsolete.
- Skills should represent **non-trivial know-how** (not obvious prompts). A practical heuristic:
  “If this can be replaced by a simple question with similar results, it’s probably not worth storing.”

### Skill recall (general assistant mode)
For users who want an “all-purpose assistant”, the system recalls skills based on context.
Recall must be observable and controllable: users should understand why a skill was used and be able to change outcomes through governance.

### Subagents (context pollution control)
Subagents handle atomic tasks to keep the main agent’s reasoning stable.
This reduces—but does not eliminate—context drift, so we also need context management.

### Context management (automatic compression)
When the conversation context grows beyond a threshold (e.g. ~80k tokens, chosen to support cost-effective models),
the system should automatically compress history into:
- **work ledger** (流水账 / timeline)
- **findings** (key decisions, constraints, results, evidence)

Compression must preserve the ability to resume work safely and produce evidence.

### Learning (SOP / know-how extraction)
The system SHOULD be able to extract repeated “receipt patterns” from real execution history into candidate SOPs/Skills.
Principles:
- Must be a separate pipeline (no impact on core execution stability).
- Must be evidence-driven (store only when it improves end-to-end delivery, not “interesting” output).
- Must be human-in-the-loop (governance workbench: dedupe/merge, drop obsolete, promote canonical).
- Prefer high-signal assets: “If a simple question yields similar results, it’s probably not worth storing.”

### Observability & reliability (must-have)
LLMs are inherently non-deterministic, so oneAgent treats the following as first-class:
- Observable process: traces/logs/receipts
- Replayable / inspectable artifacts
- Safe recovery: resume after failure; rollback when needed
- Clear evidence for “done”: tests, reports, and verifiable outputs

## Important Constraints
- Must be “open the app and do work” (choose workspace, run tasks).
- Must support multiple LLM providers and model selection.
- Must optimize prompt-caching / KV-cache usage wherever possible.
- Long-running operation is a core scenario (hours), not an edge case.
- Local safety matters: tool permissions, guardrails, and evidence-based execution.

## External Dependencies
- LLM provider APIs (multiple vendors / custom endpoints)
- Optional future tool ecosystems (e.g., MCP servers, HTTP function-call tools)
