# Competitor Issues Snapshot

This directory contains snapshots of top/recent issues from key competitor projects, fetched to identify common pain points, feature requests, and edge cases.

## Usage

Use the included script to refresh these snapshots:

```bash
chmod +x fetch_issues.sh
./fetch_issues.sh
```

## Projects Tracked

- **[n8n](n8n.md)**: Workflow automation, node execution, triggering.
- **[FastGPT](FastGPT.md)**: Knowledge base, easy-to-use LLM workflows.
- **[Dify](Dify.md)**: Application builder, agent orchestration.
- **[Continue](Continue.md)**: IDE integration, coding assistant UX.
- **[Aider](Aider.md)**: CLI-based coding agent, diff handling.

## Agent-focused snapshots

For agent / coding-agent 类产品的 issues 快照（Codex / OpenCode / Qwen Code / OpenHands / AutoGen 等），见：

- `docs/opensource/agent_issues/README.md`

## Purpose

We analyze these issues to:

1. **Avoid Pitfalls**: See what breaks in "industrial-grade" usage (e.g., n8n scheduling bugs, large payload issues).
2. **Steal Features**: Identify high-demand features (high reactions) that users are begging for.
3. **Benchmark**: Compare our stability against mature projects.
