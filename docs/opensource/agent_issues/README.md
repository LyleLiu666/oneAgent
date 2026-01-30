# Agent Competitor Issues Snapshot

本目录专注于 **agent / coding-agent 类产品**（如 Codex、OpenCode、Continue、Aider、OpenHands 等）的 issue 快照，目的是快速收集“工业级落地细节”的高频坑位与用户诉求。

## Focus

当前我们优先关注（默认只拉这 4 个）：

- Codex（`openai/codex`）
- Kode CLI（`shareAI-lab/Kode-cli`）
- OpenCode（`anomalyco/opencode`）
- Qwen Code（`QwenLM/qwen-code`）

## How to refresh

```bash
cd /Users/liu_y/code/goProject/oneAgent
chmod +x docs/opensource/agent_issues/fetch_agent_issues.sh
docs/opensource/agent_issues/fetch_agent_issues.sh
```

常用参数：

- `TOP_N=20`：每个仓库抓取 Top 20
- `FOCUS_ONLY=1`：只抓 focus 4 项（默认）
- `EXTRA=1`：额外抓 Continue/Aider/OpenHands/AutoGen 等（会增加 API 请求数）
- `GITHUB_TOKEN=...`：可显著提高 Search API 的限流上限（建议在环境变量里配置）

## Scheduling (macOS)

如果遇到 GitHub Search API 限流，最简单策略是 **定时拉取**（例如每天一次），避免频繁手动刷新。

已提供 launchd 模板（不会自动安装）：

```bash
chmod +x docs/opensource/agent_issues/scheduler/install_launchd.sh
docs/opensource/agent_issues/scheduler/install_launchd.sh /Users/liu_y/code/goProject/oneAgent
```

默认每天 03:15 拉取一次，日志写入：
- `.oneagent/tmp/fetch_agent_issues.out`
- `.oneagent/tmp/fetch_agent_issues.err`

## What we fetch (default)

- GitHub Search API：每个仓库抓取 **Top N open issues**（按 `reactions` 降序）
- 输出：
  - `*.json`：原始数据（便于后续二次统计/聚类）
  - `*.md`：可读摘要（标题/链接/反应数/评论数/标签/更新时间 + body 摘要）
