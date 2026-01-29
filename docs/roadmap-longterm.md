# oneAgent Roadmap（愿景 → 支柱 → 路线图 / Backlog）

更新时间：2026-01-29

目的：把「我们要做什么、为什么做、先做什么、做完如何验收」固化成一份长期可执行文档。  
原则：**没有文档就没有执行**；每完成一项工作，必须回到愿景做一次对照检查（Simple/Beautiful/Progressive + Evidence）。

> 相关文档：
> - UX 愿景（心理与优先级）：`docs/ux-vision.md`
> - 项目约束与完成定义：`openspec/project.md`
> - 变更进度（Done/Doing）：`openspec list`
> - 变更规范（OpenSpec）：`openspec/changes/review.md`

---

## 0) 状态快照（Done / Doing / Next）

以下以 `openspec list` 为准；Roadmap 只保留一个入口，避免分叉维护。

### 0.1 Doing（已写 Spec，待实现/进行中）
- [ ] `add-worktree-attempt-isolation`（2/8 tasks）：git worktree 隔离 attempt（执行根目录 + 生命周期管理）
- [ ] `add-mcp-server`（2/7 tasks）：对外暴露 MCP server（local-only + auth/policy + events）
- [ ] `add-secretary-mode-chat`（0/5 tasks）：提供“秘书模式”纯聊天体验（独立路由 + 低噪声 + 菜单可回完整模式）
- [ ] `fix-skill-read-not-found-ux`（0/7 tasks）：skill.read not-found 的可行动 UX

### 0.2 Done（已交付）
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

### 0.3 Next（未拆 Spec：需要先建 OpenSpec change）
- [ ] Digest/通知/批处理的体验增强（收割、聚合、失败聚类、批量 follow-up）
- [ ] 多 workspace 队列策略、资源治理、自动化（排队策略/资源限制/日程化）
- [ ] SOP/skills 的自动学习管线、强制证据、去重合并、过时淘汰

---

## 1) 北极星与成功标准（What “Good” looks like）

### 1.1 北极星体验
用户只需要安装一个“客户端式 agent 产品”，选择一个 workspace，就能下达任务并离开；数小时后回来能拿到 **可交付结果 + 可验证证据**。  
默认对大多数用户“简单好用、界面好看”，高级用户才逐步展开更多控制与治理。

### 1.2 成功判定（结果层）
- **交付物可用**：输出文件/变更可以直接被使用（代码可构建/测试通过；文档可导出/可发出）。
- **证据可追溯**：每次 attempt 至少有 `summary + findings + trace`，并能定位关键变更与失败原因。
- **失败可续**：失败/中断不会“卡死”，用户能 resume 并复用已有工作。
- **默认体验简单**：主路径不强迫用户理解治理/配置/底层概念（advanced 隐藏但可发现）。

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
- 已有 OpenSpec：`add-mcp-server`（未实现）。

**迭代点（典型）**：
- MCP server 先只读，再逐步加入 create/cancel/resume（严格复用 auth/policy）。
- 事件流（events）与证据链打通，为通知/日报提供通用入口。

---

## 4) 路线图（按“地基 → 上层”的依赖关系）

这不是“排期承诺”，而是依赖关系与建议顺序（做完再滚动更新）。

### 4.0 地基 → 上层（执行顺序与依赖）
为了保证“先把地基打牢再上楼”，我们把近期主线拆成层级（L0→L3）。  
**规则**：默认只推进最靠前、且能形成闭环交付的那一层；后续层级的工作必须建立在前一层的稳定性之上。

| 层级 | 目标 | 主要 changes | 依赖/门槛 |
| --- | --- | --- | --- |
| **L0** | 项目可运行/可测试（可复现） | `add-project-scripts`（已完成） | tool permissions 不可绕过；失败留痕完整 |
| **L1** | 交付可审查（评论→下一轮） | `add-diff-review-loop`（已完成） | 具备 diff/test evidence 的可打开指针 |
| **L2** | 执行隔离 + 易回滚 | `add-worktree-attempt-isolation` | git repo 检测稳定；生命周期清理可解释 |
| **L3** | 对外编排入口 | `add-mcp-server` | local-only + auth/policy；事件流与证据链打通 |

辅助但低风险的“体验修补”（可穿插）：
- `add-secretary-mode-chat`：不阻塞主线（L0/L1/L2/L3），但能显著降低“只想对话委托”的噪声。
- `fix-skill-read-not-found-ux`：不阻塞主线，但每次碰到都值得顺手修掉（减少治理摩擦）。

### Phase A：把“项目任务”做到默认可用（地基主线）
1) `add-project-scripts`（已完成：进入可运行态 + 日志证据）
2) `add-diff-review-loop`（已完成：可审查交付 + review→follow-up）
3) `add-worktree-attempt-isolation`（隔离执行 + 可回滚边界）

### Phase A2：体验线（可与地基并行推进）
4) `add-secretary-mode-chat`（低噪声纯对话入口）
5) `fix-skill-read-not-found-ux`（小而关键的摩擦修复）

### Phase B：把“运行时”开放出去（集成主线）
6) `add-mcp-server`（local-only + auth/policy + events）

### Phase C：把“挂机收割”做到极致（留存主线）
7) Digest/通知/批处理的体验增强（按需拆 change）
8) 多 workspace 的队列策略、资源治理、自动化（按需拆 change）

### Phase D：学习与治理的复利升级（资产主线）
9) SOP/skills 的自动学习管线、强制证据、去重合并、过时淘汰（按需拆 change）

### 4.1 近期 backlog（已写 OpenSpec：保留关键价值/风险点）

> 进入实现阶段前，必须把工作区未提交变更整理为可回滚提交（否则“可复现/可推广”不成立）。

#### P0（地基 / 必须先做）

##### `add-worktree-attempt-isolation`（隔离执行）
**价值**：把“并发/污染/回滚”问题降维为 git 合并问题，天然支持审查与回退。

**关键坑**
- Windows 文件占用/路径长度：worktree 清理与回收要足够鲁棒。
- 非 git workspace：必须给出明确错误或按配置退化（不得 silent fallback）。
- 生命周期：孤儿 worktree 的识别与清理需要证据与可操作提示。

##### `fix-skill-read-not-found-ux`（可穿插：低成本高收益）
**价值**：减少治理/学习阶段的“读不到 skill 却不知道怎么办”的摩擦，提升可用性。  
**定位**：不阻塞主线（L0/L1/L2），但每次碰到都值得顺手修掉。

#### P1（核心体验：低噪声纯对话）

##### `add-secretary-mode-chat`（像“只和秘书说话”一样简单）
**价值**：在不牺牲“复杂交互窗口（Tasks/Governance/Ledger）”的前提下，提供一个极低噪声的纯对话入口，让用户可以像使用 Moltbot/WebChat 那样只通过聊天完成委托与收割。

**关键坑**
- 不能把“简单”做成“功能缺失”：只是把复杂度折叠/隐藏，并确保一键回到完整模式。
- 默认路径不得耦合 UI 文案：测试必须使用 `data-testid`，避免 i18n/措辞变更导致脆弱。
- 证据链不丢：秘书模式隐藏 trace/工具细节，但必须可发现地打开查看（否则排障困难）。

#### P2（生态/集成：把 oneAgent 变成可编排运行时）

##### `add-mcp-server`（标准协议入口）
**价值**：让外部客户端通过 MCP 读取/订阅/管理任务与账本，支撑通知/日报与“挂机收割”场景。

**关键坑**
- 默认必须 local-only；远程访问必须显式开启且复用 auth/policy。
- MCP 调用必须进入证据链（否则排障与审计断裂）。

#### 执行顺序（建议）
按“地基 → 上层（L0→L3）”综合排序：
1) `add-worktree-attempt-isolation`
2) `add-mcp-server`（建议先只读，逐步扩展）
3) `add-secretary-mode-chat`（体验线：可并行推进）
4) `fix-skill-read-not-found-ux`（穿插做，随时可落地）

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
