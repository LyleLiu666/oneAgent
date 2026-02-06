# oneAgent Roadmap（愿景 → 支柱 → 路线图 / Backlog）

更新时间：2026-02-06

目的：把「我们要做什么、为什么做、先做什么、做完如何验收」固化成一份长期可执行文档。  
原则：**没有文档就没有执行**；每完成一项工作，必须回到愿景做一次对照检查（Simple/Beautiful/Progressive + Evidence）。

> 相关文档：
> - UX 愿景（心理与优先级）：`docs/ux-vision.md`
> - Secretary-first（短期专项：默认只保留秘书模式）：`openspec/roadmap-secretary-first.md`
> - 项目约束与完成定义：`openspec/project.md`
> - 变更进度（Done/Doing）：`openspec list`
> - 变更规范（OpenSpec）：`openspec/changes/review.md`

---

## 0) 状态快照（Doing / Complete / Archived / Next）

以下以 `openspec list` 为准；Roadmap 只保留一个入口，避免分叉维护。

### 0.1 Doing（已写 Spec，待实现/进行中）
- 当前 active changes 请直接运行 `openspec list` 查看（本段不再手工维护列表，避免漂移）

### 0.2 Complete（已完成，待归档到 `openspec/changes/archive/`）
- `update-app-shell-secretary-first`：默认只保留秘书模式；仅完全模式显示 Sidebar/菜单
- `add-secretary-chat-task-handoff`：秘书模式下长任务 handoff 到 Task Queue（对话低噪声展示）
- `update-secretary-task-handoff-ux`：handoff 确认与回执更像“微信聊天”
- `add-secretary-deliverable-cards`：对话内交付卡片（仅任务产物 artifacts，可点开预览）
- `add-secretary-recovery-actions`：失败任务的一键恢复/排障入口（best-effort）
- `add-secretary-task-completion-notifications`：后台任务完成后自动在对话里追加低噪声通知
- `add-secretary-status-hints`：秘书模式低噪声状态提示（不引入管理系统外观）
- `add-secretary-task-status-hints`：秘书模式任务状态提示（运行中/完成等）
- `add-chat-stream-recovery-and-stop`：Chat streaming 可 stop + 刷新后恢复（best-effort）
- `add-session-context-compression-verification`：会话自动压缩可回归验证（确认生效）
- `update-tool-loop-limits-and-write-file-no-truncate`：tool loop 上限可配置 + `write_file` 超限 fail-fast
- `update-chat-ux-tool-progress-indicator`：工具执行进度提示（含秘书模式）
- `add-high-risk-command-approvals`：高风险命令强制审批（安全护栏）
- `add-native-command-sandbox`：跨平台 native sandbox（workspaceRoot 边界、真删除）
- `add-trash-file-tool`：软删除（回收站）工具（move-to-trash + 7 天保留清理）
- `add-workflow-orchestration-graph`：工作流编排（显式图）：节点=工作型 agent；交付物=文件集；Hard/Soft Gate

### 0.3 Archived（已交付，且已归档到 `openspec/changes/archive/`）
- [x] `add-project-scripts`
- [x] `add-diff-review-loop`
- [x] `add-observer-remediation-loop`
- [x] `add-execution-safety-invariants`
- [x] `update-workbench-ux-fold-advanced`
- [x] `add-skill-governance-workbench`
- [x] `add-sop-governance-workbench`
- [x] `add-skill-governance-duplicates`
- [x] `add-skill-governance-edit`
- [x] `add-sidebar-ledger-status-badges`

### 0.4 Next（未拆 Spec：需要先建 OpenSpec change）
- [ ] Digest/通知/批处理的体验增强（收割、聚合、失败聚类、批量 follow-up）
- [ ] 多 workspace 队列策略、资源治理、自动化（排队策略/资源限制/日程化）
- [ ] SOP/skills 的自动学习管线、强制证据、去重合并、过时淘汰

---

## 1) 北极星与成功标准（What “Good” looks like）

### 1.1 北极星体验
用户只需要安装一个“客户端式 agent 产品”，选择一个 workspace，就能下达任务并离开；数小时后回来能拿到 **可交付结果 + 可验证证据**。  
默认对大多数用户“简单好用、界面好看”，高级用户才逐步展开更多控制与治理。

长期形态：像 FastGPT/N8N 一样可编排工作流，但**每个节点都是一个工作型 Agent**（类似 Claude Code/Codex），节点间以“文件集交付物”传递，并支持多条工作线并行推进；秘书作为系统级 orchestrator 把自然语言委托自动转为工作流。

### 1.2 成功判定（结果层）
- **交付物可用**：输出文件/变更可以直接被使用（代码可构建/测试通过；文档可导出/可发出）。
- **证据可追溯**：每次 attempt 至少有 `summary + findings + trace`，并能定位关键变更与失败原因。
- **失败可续**：失败/中断不会“卡死”，用户能 resume 并复用已有工作。
- **默认体验简单**：主路径不强迫用户理解治理/配置/底层概念（advanced 隐藏但可发现）。
- **双层验收可回归**：节点/任务的完成由 Hard Gate（可代码校验的客观事实）+ Soft Gate（按预设维度的模型评分；best-effort）共同决定，并把验收报告作为证据产物留存。

---

## 2) 当前阶段定位（Where we are）

### 2.1 已有“地基”
我们已经有较完整的底盘闭环（以 OpenSpec 与实现为准）：
- Task Queue / Work Ledger / Outcome Observer（长任务、证据链、只读验收）
- Skills/SOP：发现、召回、治理（列表/编辑/归档/duplicates）
- Tool permissions / workspace 边界（默认安全 + 可控）

### 2.2 最可能仍未完成的主方向（当前最大的缺口）
**“项目化工作流 + 可审查交付（从聊天到工程闭环）”** 仍未完成到“默认好用”的程度：
- 进入可运行态仍容易卡在环境准备（setup/test/dev server/复制 `.env` 等）
- 交付缺少“一眼审查”的默认路径（diff/变更摘要 + review→follow-up）
- 失败后的清理/隔离/回滚成本偏高（尤其是代码类任务）
- Chat 仍偏“工作台式”信息密度：当用户只想把任务委托给“秘书”时，缺少一个足够低噪声的纯对话模式

这是一个“产品形态层”的缺口：它决定用户是否真的把 oneAgent 当作可长期依赖的 client，而不是“偶尔聊两句的工具”。

---

## 3) 六大产品支柱（Pillars）与迭代方向

下面每个支柱都按：**目标 → 当前 → 迭代点** 写清楚，便于长期复盘。

### P1. 默认简单 + 美观（Client-like UX）
**目标**：主路径极短、界面低噪声；复杂度只在需要时出现（progressive disclosure）。

**当前**：
- 已默认中文；工作台部分低频内容已折叠（advanced）。

**迭代点（典型）**：
- 统一信息架构：避免“功能堆叠”导致的拥挤（布局、密度、对齐、溢出）。
- 默认视图只显示：workspace、任务入口、交付入口；其余都可展开但不干扰。
- “展开后仍好看”：展开态要像一个专业工具，而不是堆叠面板。
- 引入“秘书模式”（像 Moltbot 的 WebChat 一样低噪声）：当用户只想对话时，界面仅保留消息流 + 输入框 + 必要状态；模型/工具/trace/任务面板等统一折叠到“详情/高级”。提供独立路由直接进入（例如 `/secretary`），完整模式仍可从菜单进入，并支持一键切换回完整模式。

### P2. 项目化工作流（开箱即用地进入可运行态）
**目标**：真实项目里，agent 能在 1 分钟内进入“可运行/可测试/可预览”状态，并留下证据。

**当前**：
- 已完成：`add-project-scripts`（baseline 已实现）。

**迭代点（典型）**：
- `.oneagent/project.json` 支持 setup/test/cleanup/dev_server/copy_files，并且失败留痕。
- UI 呈现“项目脚本状态/最近日志”，让用户快速排障。

### P3. 可审查交付（Diff review loop）
**目标**：交付默认可审查；用户评论可直接成为下一轮 attempt 的输入（短反馈闭环）。

**当前**：
- 已完成：`add-diff-review-loop`（baseline 已实现）。

**迭代点（典型）**：
- 每次 attempt 产出 diff artifacts（git 优先，非 git 降级）。
- UI 提供 review 入口 + comments；comments 进入证据链并注入 follow-up。

### P4. 隔离执行与可回滚（Worktree / 防污染）
**目标**：把风险从“共享文件系统状态”降维成“可合并的变更”，减少污染与清理成本。

**当前**：
- 已有 OpenSpec：`add-worktree-attempt-isolation`（未实现）。

**迭代点（典型）**：
- attempt 在 git worktree 执行，记录 base SHA / worktree_root / diff / tests。
- worktree 生命周期管理（清理/孤儿回收）要稳定、可解释。
- 对非 git / 不适合复制的大型 workspace：提供 `sandbox_mode=native` 作为硬边界执行后端（workspaceRoot 内可真删除，如 `rm -rf`）。

### P5. 长任务自治与收割体验（挂机、日报、通知）
**目标**：用户把任务丢给 agent 后可以离开；回来能“收割成果”，失败点也能集中处理。

**当前**：
- Task Queue/Work Ledger 已有基础；UI 有工作台与状态入口。

**迭代点（典型）**：
- Work Ledger/Digest 的“收割”体验：聚合、过滤、失败原因聚类、批量 follow-up。
- 多 workspace 并行是基础能力；未来可增加“排队策略/资源限制/日程化”。

### P6. 生态与集成（MCP / 私有部署）
**目标**：oneAgent 不只是 UI，更是可编排运行时；外部客户端可以标准协议接入。

**当前**：
- 已完成并归档：`add-mcp-server`（只读入口）
- 可选后续：`update-mcp-action-plane-v1`（受控动作面：create/resume/cancel；复用 auth/policy/approval）

**迭代点（典型）**：
- MCP 保持 local-only 为默认；远程访问必须显式开启且复用 auth/policy。
- MCP 调用必须进入证据链（events/receipt/trace），否则排障与审计断裂。
- 事件流（events）与证据链打通，为通知/日报提供通用入口。

---

## 4) 路线图（按“地基 → 上层”的依赖关系）

这不是“排期承诺”，而是依赖关系与建议顺序（做完再滚动更新）。

### 4.0 地基 → 上层（执行顺序与依赖）
为了保证“先把地基打牢再上楼”，路线图拆成层级（L0→L4；必要时扩展）。  
**规则**：默认只推进最靠前、且能形成闭环交付的那一层；后续层级必须建立在前一层稳定性之上。  
**状态来源**：active changes 的真实进度以 `openspec list` 为准；本文件只维护“依赖顺序与验收门槛”（避免漂移）。

#### 4.0.1 已完成的地基（归档变更，作为背景）
我们已经完成了本地交付闭环的基础层（详见 `openspec/changes/archive/`）：
- `add-project-scripts`：进入可运行/可测试（可复现）
- `add-diff-review-loop`：交付可审查（review → follow-up）
- `add-worktree-attempt-isolation` + `add-native-command-sandbox`：隔离执行与硬边界（降低污染与破坏面）
- `add-mcp-server`：MCP 只读入口（集成基底）

#### 4.0.2 当前主线：本地工程交付助手（简单问题 + 复杂问题，都要留痕）
目标：用户可以把简单问题当场解决，把复杂问题丢给后台挂机；两者都能拿到“可审查交付物 + 证据 + 可回滚边界（当有写操作时）”，且尽量少打断用户。

| 层级 | 目标 | 对应 changes（按依赖） | 验收门槛（Hard Gate，示例） |
| --- | --- | --- | --- |
| **L0** | 真相对齐 + 量尺 | `update-openspec-truth-alignment`（✓ Complete）→ `add-head2head-benchmark-suite` | truth check 可在本地/CI fail-fast；benchmark 本地最小集（>=3）可产出 JSON+MD 报告 |
| **L1** | 交付物口径稳定（简单/复杂统一留痕） | `update-task-deliverable-contract-v1` | manifest v1 版本化；缺失字段有 reason code；API 合约测试覆盖成功/失败/降级 |
| **L2** | 执行隔离 + 易回滚（复杂问题默认安全） | `update-worktree-attempt-isolation-v2` | worktree 元信息落盘；non-git fail-closed；清理失败可追溯/可重试；orphan 回收（best-effort） |
| **L3** | 少打断自治（先自愈后升级） | `update-secretary-autonomy-selfheal-v2` | 单任务进度问答不追问；协议/参数类错误 bounded repair+retry；超过预算才升级且给下一步 |
| **L4** | 规模化稳定性（多 workspace 调度治理） | `update-queue-governance-scheduling-v2` | 公平性/防饥饿；schedule misfire+幂等 key；治理决策写入事件；覆盖关键测试 |
| **L5（可选）** | 外部触发动作面 | `update-mcp-action-plane-v1` | MCP 集成测试覆盖 auth/policy/approval；审计字段可追溯 |
| **L6（更后）** | 渠道/IM 接入 | `add-channel-relay-v1` | v1 先 1 个渠道；webhook 签名 fail-closed；幂等去重；端到端测试 |

#### 4.0.3 线性执行顺序（按“地基 → 上层”排好）
1) `add-head2head-benchmark-suite`（先把尺子立起来）
2) `update-task-deliverable-contract-v1`（把“留痕/可审查”做成稳定契约）
3) `update-worktree-attempt-isolation-v2`（把“回滚/防污染”做成默认）
4) `update-secretary-autonomy-selfheal-v2`（减少用户介入，把可自愈问题拿回来）
5) `update-queue-governance-scheduling-v2`（规模化后仍可解释/可恢复）
6) `update-mcp-action-plane-v1`（可选后续：外部触发，不阻塞本地交付主线）
7) `add-channel-relay-v1`（更后：渠道/IM，本阶段明确不做）

---

## 5) 执行机制（如何把计划落成可交付）

### 5.1 Spec-first 工作流（强制）
每个可交付工作都必须先形成 OpenSpec change：
- `proposal.md`（Why/What/Impact）
- `tasks.md`（最小可交付拆解）
- `specs/*/spec.md`（ADDED/MODIFIED requirements + 场景）

### 5.2 Definition of Done（强制）
每次提交必须同时满足：
- `openspec validate <change-id> --strict --no-interactive` 通过
- 单测通过（前后端）
- e2e smoke 通过（如果涉及关键链路）
- “愿景对照检查”通过：默认简单、界面美观、展开后仍清爽、证据链不断裂

---

## 6) 维护：文档与计划的更新规则
- 每新增/完成一个 change：同步更新本文件的「0) 状态快照」（Done / Doing / Next）
- 每完成一个 change：在本文件补充“产出/经验/下一轮迭代点”
- 每月至少回顾一次：愿景是否变化、北极星是否偏移、哪些指标在退化
