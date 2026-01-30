# OpenAI Codex — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:57:17Z
- Repo: https://github.com/openai/codex
- Query: `repo:openai/codex is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [Plan Mode](https://github.com/openai/codex/issues/2101) (#2101)
- Reactions: **499** | Comments: **50** | Updated: 2026-01-28T17:50:49Z
- Labels: `enhancement`

Please add a feature for researching & planning before executing any changes. We can sort of do it today via prompting in certain ways but it will often dive into making changes right away and lacks a good way to manage or edit a plan before executing on it.

---

### [Event Hooks](https://github.com/openai/codex/issues/2109) (#2109)
- Reactions: **452** | Comments: **32** | Updated: 2026-01-29T23:36:56Z
- Labels: `enhancement`, `agent`

Let us define event hooks with pattern matching, to trigger scripts/commands before/after codex behaviors.

---

### [Subagent Support](https://github.com/openai/codex/issues/2604) (#2604)
- Reactions: **351** | Comments: **70** | Updated: 2026-01-30T00:47:30Z
- Labels: `enhancement`, `agent`

### What feature would you like to see?

## Summary
  Request for official subagent functionality to be implemented in codex.

  ## Background
  This  codebase  `https://github.com/bluxolguin/codex.git` branch `bluxolguin/subagents` currently has experimental agent management features (see commit a14a89b1) that demonstrate the foundation for subagent functionality. This includes:
  - Agent storage system in `codex-rs/core/src/agents_store.rs`
  - TUI integration for agent creation
  - Basic prompt templating for agent definitions

  ## Motivation
  Subagents would provide several key benefits:…

---

### [A way to exclude sensitive files](https://github.com/openai/codex/issues/2847) (#2847)
- Reactions: **146** | Comments: **37** | Updated: 2026-01-25T13:20:27Z
- Labels: `enhancement`, `sandbox`

### What feature would you like to see?

- A mechanism to explicitly mark files/paths that the agent must not read or send to the model, at both repository and global levels (e.g., a repo-local .codexignore plus a global ignore file).
- Example: keep node_modules/ searchable for implementation checks, but never read or send .env, .env.*, *.pem, id_*, .aws/**, .ssh/**.
- The configuration should be deterministic and shareable across the team/repo, and also support user defaults, rather than relying on project documentation or conventions.

### Are you interested in implementing this feature?

-…

---

### [Login shells override inherited PATH in curated environments; need configurable default](https://github.com/openai/codex/issues/8922) (#8922)
- Reactions: **110** | Comments: **1** | Updated: 2026-01-21T16:37:59Z
- Labels: `bug`, `CLI`

### What version of Codex is running?

codex-cli 0.77.0

### What subscription do you have?

Using Codex via Oracle Enterprise‑hosted environment

### Which model were you using?

gpt-5-codex

### What platform is your computer?

Linux 5.15.0-315.193.2.el8uek.x86_64 x86_64 x86_64

### What issue are you seeing?

**Description**
In curated environments (e.g., ADE or dev shells), codex inherits a carefully constructed PATH via config, but the default shell execution launches a login shell (bash -lc). Login shells re-run startup scripts and can override the inherited PATH, which causes tools from…

---

### [Named sessions for --resume](https://github.com/openai/codex/issues/4163) (#4163)
- Reactions: **104** | Comments: **4** | Updated: 2026-01-08T12:19:16Z
- Labels: `enhancement`

### What feature would you like to see?

### Feature Request: Named Sessions for `--resume`

**Problem**
The current `--resume` feature shows multiple sessions without clear identifiers. For developers who work across several projects, it’s easy to get lost and accidentally pick the wrong session since there are no names or tags to distinguish them.

**Proposal**
Introduce optional **session names and tags**:
- `codex --name "checkout-fix"` to assign a human-friendly name.
- Support `--tags payments,urgent` for lightweight grouping.
- `/session name <newName>` and `/session tags +tag -tag` to …

---

### [Play a sound when Codex finishes a prompt / task](https://github.com/openai/codex/issues/3962) (#3962)
- Reactions: **86** | Comments: **26** | Updated: 2026-01-29T13:28:53Z
- Labels: `enhancement`, `Extension`, `CLI`

### Sound when Codex finishes

**Summary**
Please add an optional, clearly audible completion sound that plays when Codex finishes executing a prompt/task (useful when prompts run longer and the user switches focus).

**Why**
Helps when Codex works in background and I’m in another window/tab.
Terminal bell \a is too short/quiet and easy to miss.

**Use case / Motivation**
Most of my prompts take Codex quite a long time to execute — often more than a minute. It’s frustrating that I have to sit and stare at the screen just to know when it’s done. Since I work remotely, I could easily use that wa…

---

### [Control over color theme in TUI](https://github.com/openai/codex/issues/1618) (#1618)
- Reactions: **80** | Comments: **18** | Updated: 2026-01-29T08:53:27Z
- Labels: `enhancement`, `TUI`

The color theme of the Codex CLI is a MUST for customization.

---

### [IDE-integrated diff / approval](https://github.com/openai/codex/issues/2998) (#2998)
- Reactions: **78** | Comments: **29** | Updated: 2026-01-17T00:35:26Z
- Labels: `enhancement`, `Extension`

### What feature would you like to see?

Codex CLI already has a good approval flow: it can show red/green diffs in the terminal and ask the user to approve or reject changes. This works well, but currently it only happens in the terminal.

It would be great to also have this experience directly inside an IDE (e.g. VS Code or Cursor). For example:

Show the diff inline in the IDE’s editor

Let the user approve, approve for session, or reject from there

Then send the decision back so Codex can apply or skip the patch

This would feel much smoother, similar to how CloudCode shows changes inside…

---

### [Native SSE transport support for MCP servers](https://github.com/openai/codex/issues/2129) (#2129)
- Reactions: **61** | Comments: **24** | Updated: 2026-01-27T18:52:59Z
- Labels: `enhancement`, `mcp`

# Summary

  Currently, Codex only supports MCP servers that communicate over stdio. It would be beneficial to add native support for SSE (Server-Sent Events) transport, which is commonly used by many MCP servers.

# Current Situation

  - Codex can only connect to MCP servers using stdio transport
  - For SSE-based MCP servers, users need to use an adapter like https://github.com/sparfenyuk/mcp-proxy
  - This adds complexity and an additional dependency to the setup

# Proposed Solution

  Add native SSE transport support to the MCP client implementation in Codex, allowing direct connection t…

---

