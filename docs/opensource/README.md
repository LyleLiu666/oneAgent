# Open Source 参考库（能抄就抄）

目的：维护一份“可直接抄”的开源参考清单，帮助 oneAgent 把愿景拆解为可执行的工程细节。

我们的原则：

- **愿景容易，细节难**：workflow/agent/评测/权限/回退/可观测性这些“工业级细节”最耗耐心。
- **能抄就抄**：优先复用成熟项目里已经被踩坑验证过的设计与实现。
- **必须遵守 License**：允许“借鉴/复刻设计”，但复制代码前要确认许可并保留归属与必要的 NOTICE（best-effort）。

本参考库的本地镜像目录：

- `/Users/liu_y/code/opensource`

> 说明：本文件只记录“我们为什么看它、抄什么、从哪里抄”；具体落地仍需在 oneAgent 里拆 OpenSpec change 与测试。
>
> 🔗 **[Competitor Issues Snapshot](./issues/README.md)**: 我们爬取了重点项目的 Top Issues (High Reactions) 以供避坑参考。
> 🔗 **[Agent Issues Snapshot](./agent_issues/README.md)**: 额外聚焦 coding-agent/agent 产品（Codex/OpenCode/…）的 Top Issues。

---

## 1) 关注点 → 该抄什么

### A. 工作流编排（像 FastGPT / n8n，但节点=Agent）

我们要抄的不是“画布 UI”，而是：

- workflow graph 的数据结构（node/edge、versioning、运行快照）
- 节点执行模型（timeouts/retries/cancel/resume）
- **节点交付物是文件集**（artifact manifest、产物指针、可回放）
- **Hard/Soft Gate 验收**（客观校验 + 模型评分；预算与迭代上限）

### B. coding-agent 的交付闭环（像 Claude Code / Codex）

我们要抄的不是“聊天”，而是：

- 工具路由策略（默认低温、参数修复、错误可行动）
- 可观测性（事件瀑布、trace、diff/test_report 指针）
- 安全与隔离（workspace 边界、sandbox、命令 profile）
- 可回退（worktree/checkpoint/rollback）

### C. 评测与回归（把“顺滑/成功率”变成数字）

我们要抄的包括：

- 固定任务集 + 可重复跑（脚本化）
- 指标定义（args_valid_rate / tool_success_rate / steps_per_task / retry_success_rate）
- 报告产物（JSON/JSONL/SQLite）与对比展示

---

## 2) 现有本地镜像（已存在于 `/Users/liu_y/code/opensource`）

> 这些目录已存在，可直接用 `rg`/`README`/`docs` 开始扫。

### 2.1 Coding agents / CLI（参考交互与工具链路）

- `codex`：参考 CLI 形态、tool loop、事件输出（对齐 Codex 风格）
- `opencode` / `oh-my-opencode` / `qwen-code` / `Kode-cli`：参考“开源 coding agent”的交互、工具封装与工程取舍
- `learn-claude-code`：参考 Claude Code 类体验的提示词与工作流拆解（best-effort）

### 2.2 多 Agent / Context / Skills

- `moltbot`：参考“秘书式低噪声对话入口/多任务委托”形态（best-effort）
- `agentic-context-engine`：参考上下文压缩/记忆/证据组织
- `agent-skills` / `skill-from-masters` / `seo-geo-claude-skills`：参考 skills 的结构与治理方式（best-effort）
- `openagentic-sdk`：参考 agent SDK 抽象（best-effort）

### 2.3 交付与知识库（可选）

- `WeKnora` / `talebook` / `MinerU`：参考文档/知识处理与导出（如果要做“多文件交付/资料汇编”可借鉴）

---

## 3) 推荐新增镜像（尚未在本地，建议 clone）

> 这些是“工业级细节”最值得抄的项目：workflow 引擎、agent 平台、IDE/CLI agent、评测与可观测性。

### 3.1 工作流引擎 / 编排（强相关）

- n8n：`git clone --depth 1 https://github.com/n8n-io/n8n.git n8n`
  - 抄：节点/运行记录/失败重试/连接器生态、可视化编排（以运行模型为主）
- FastGPT：`git clone --depth 1 https://github.com/labring/FastGPT.git FastGPT`
  - 抄：面向业务的“工作流 + LLM 应用”的工程化边界与 UI 组织
- Dify：`git clone --depth 1 https://github.com/langgenius/dify.git dify`
  - 抄：应用/工作流/知识库/观察面板的产品化拼装
- Flowise：`git clone --depth 1 https://github.com/FlowiseAI/Flowise.git flowise`
  - 抄：图形化链路 + node 生态（对比 n8n/FastGPT 的取舍）
- Langflow：`git clone --depth 1 https://github.com/langflow-ai/langflow.git langflow`
  - 抄：工作流/组件模型与扩展点（best-effort）

### 3.2 Agent 平台 / 自动化执行（强相关）

- OpenHands（OpenDevin 系）：`git clone --depth 1 https://github.com/All-Hands-AI/OpenHands.git OpenHands`
  - 抄：SWE/代码任务的执行闭环、sandbox、评测与回放（best-effort）
- Microsoft AutoGen：`git clone --depth 1 https://github.com/microsoft/autogen.git autogen`
  - 抄：多 agent 协作抽象、message routing、tooling（best-effort）

### 3.3 Coding assistant（补齐“像 Codex/Claude Code”细节）

- Continue：`git clone --depth 1 https://github.com/continuedev/continue.git continue`
  - 抄：IDE 集成、上下文选择、工具/模型切换体验（best-effort）
- aider：`git clone --depth 1 https://github.com/Aider-AI/aider.git aider`
  - 抄：diff 驱动的改代码闭环、提示词与回归（best-effort）

---

## 4) 使用方式（建议流程）

1. 先在本文件里给一个项目加条目：**为什么看它 / 抄什么 / 对应 oneAgent 哪个能力**。
2. clone 到 `/Users/liu_y/code/opensource/<name>`（尽量 `--depth 1`）。
3. 用 `rg` 快速定位关键实现点（例如：workflow run、artifact、retry、observer/eval、permissions）。
4. 回到 oneAgent：拆 OpenSpec change（先测后改），再逐步实现。
