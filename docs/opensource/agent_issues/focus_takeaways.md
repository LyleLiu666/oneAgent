# Focus Takeaways — Codex / Kode CLI / OpenCode / Qwen Code

目的：从这些 agent 类产品的高反应 issue 里快速抽取“别人踩过的坑”，把它们转成 oneAgent 的可执行需求/测试。

> 数据来源：`docs/opensource/agent_issues/snapshots/`（定期刷新）

## Codex（openai/codex）

- **Plan Mode / 先计划后执行**：用户希望有“研究/计划/可编辑计划/再执行”的明确模式（issue #2101）。
- **Event Hooks**：希望在关键事件前后触发脚本（before/after hooks，pattern matching）（#2109）。
- **Subagent**：官方级“子 agent”管理与复用（#2604）。
- **敏感文件排除（.codexignore）**：全局 + repo 级 deterministic ignore，避免把 `.env/.ssh/*.pem` 送上游（#2847）。
- **Shell/环境一致性**：login shell 覆盖 PATH，导致 curated env 下工具链失效（#8922）。
- **会话管理**：`--resume` 需要“命名/标签”降低误选风险（#4163）。
- **反馈与通知**：任务完成提示音（#3962）、主题可控（#1618）。
- **IDE diff/approval**：希望在 IDE 内完成 diff 审批，而不仅仅是终端（#2998）。
- **MCP SSE 传输**：MCP client 支持 SSE transport，降低“必须走 stdio”适配成本（#2129）。

## Kode CLI（shareAI-lab/Kode-cli）

- **Responses API / previous_response_id + 分支会话（rewind/fork）**：从任意历史轮回退继续时，内部形成分支（#139）。
- **ACP 集成诉求**：希望接入 Agent Client Protocol（#99）。
- **Local dev 体验/可复现性**：本地开发模式启动失败类问题（#141）。

## OpenCode（anomalyco/opencode）

- **Provider 稳定性**：上游提供方变更导致“突然不可用”的大规模事故（Broken Claude Max，#7410）。
- **Plan mode 细节**：Plan mode 里“会问问题/收集约束”是核心体验点（#3844）。
- **多目录/工作区扩展**：支持把多个目录加入同一 session / workspace（#1543）。
- **IDE/编辑器集成**：IDE 内联 diff、上下文同步等（#216），以及对 Cursor 等工具链的支持（#2072）。
- **输入体验**：vim motions（#1764）、语音输入（#4695）、粘贴文本折叠但可展开编辑（#8501）。
- **跨平台**：Windows 支持长期议题（#631）。
- **可配置系统提示词**：global/project/custom 目录的 system prompt（#7101）。

## Qwen Code（QwenLM/qwen-code）

- **本地模型 tool calling 可靠性**：self-hosted / ollama 下工具调用“几乎不可用/不执行/无错误”（#124/#176/#187）。
- **Token 成本异常**：token 消耗远高于同类工具（#83）。
- **Streaming setup timeout**：长输入或网络情况下流式初始化超时（#239）。
- **Loop/长输入边界**：输入长度范围、死循环检测导致中断（#350）。
- **Schema 兼容性/变更破坏**：参数 required/类型变更导致全量失败（例如 is_background，#472）。
- **ACP & IDE 集成**：ACP 集成问题聚合（#987）。

## Cross-cutting（值得转成 oneAgent 的测试/变更）

- **Plan vs Execute 的硬切分**：至少要能“先列计划 + 人工确认/Hard Gate + 再执行”。
- **Hooks/扩展点**：before/after 事件钩子是工业落地必需品（可观测 + 可自动化）。
- **敏感路径屏蔽**：repo/global ignore（规则可共享、可审计）是安全底线。
- **环境一致性**：shell 是否 login、PATH 继承策略、工具链探测与修复建议。
- **多目录 workspace**：支持会话内扩展工作区，且有清晰的权限边界与证据。
- **Provider 事故演练**：断连/超时/配额/协议变更的回归用例 + 降级策略。
- **本地模型工具调用**：对“tool calling 不执行/不报错/超时”要有可回归 bad cases 与诊断面板。

