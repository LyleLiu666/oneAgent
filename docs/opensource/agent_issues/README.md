# Agent Competitor Issues Snapshot

本目录专注于 **agent / coding-agent 类产品**（如 Codex、OpenCode、Continue、Aider、OpenHands 等）的 issue 快照，目的是快速收集“工业级落地细节”的高频坑位与用户诉求。

## How to refresh

```bash
cd /Users/liu_y/code/goProject/oneAgent
chmod +x docs/opensource/agent_issues/fetch_agent_issues.sh
docs/opensource/agent_issues/fetch_agent_issues.sh
```

## What we fetch (default)

- GitHub Search API：每个仓库抓取 **Top N open issues**（按 `reactions` 降序）
- 输出：
  - `*.json`：原始数据（便于后续二次统计/聚类）
  - `*.md`：可读摘要（标题/链接/反应数/评论数/标签/更新时间 + body 摘要）

