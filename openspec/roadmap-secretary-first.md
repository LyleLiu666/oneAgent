# Secretary-first Roadmap（微信心智：只聊天，其它全自动）

更新时间：2026-02-01

目的：把「默认只保留秘书模式、隐藏管理系统细节、把复杂度变成自动化」拆成可执行的迭代序列，并明确每一步的验收与证据。

相关文档：
- 长期 Roadmap（全局）：`openspec/roadmap-longterm.md`
- UX 愿景（用户心理与优先级）：`docs/ux-vision.md`
- OpenSpec 项目约束与 DoD：`openspec/project.md`
- 活跃变更列表：`openspec list`

---

## 1) 北极星体验（What “Good” looks like）

**默认体验**：用户打开 oneAgent，只看到一个类似微信对话窗口的“秘书”，可以直接说事；系统自动帮用户做拆解、执行、迭代、交付与留证据。

**关键感受**：
- 像在微信里和秘书说话：没有“管理系统”心智负担
- 结果可交付：最终落到文件/变更/报告等可验证产物
- 过程可追溯：需要排障时能找到证据（trace/receipt/findings），但默认不强迫用户看

---

## 2) 产品定义（模式与行为）

### 2.1 两种模式（Progressive disclosure）

- **秘书模式（默认）**：只保留对话所需最小要素；全局菜单/侧边栏隐藏；高级信息与治理入口默认不可见但可恢复。
- **完全模式（高级）**：恢复左侧侧边栏与所有工作台页面（Tasks/Governance/Ledger/Workflows…）。

### 2.2 秘书模式的硬性 UX 约束（必须满足）
- 默认不展示任何“治理/配置/系统状态”面板（除非用户显式切到完全模式）
- 允许用户一键回到完全模式（可发现、可逆）
- 不牺牲证据链：隐藏不等于丢失；需要时可定位到 trace/findings/artifacts（best-effort）

---

## 3) 现状核对（事实依据，避免想当然）

### 3.1 工具循环（迭代轮数）已有上限
- Chat 工具 loop 已有 `max_steps` 上限：默认 `200`，可用环境变量覆盖：`ONEAGENT_CHAT_TOOL_MAX_STEPS`（并有 cap `ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP`）。对应实现：`backend/internal/toolcalling/limits.go`。
- 这解决了“无上限导致无限迭代/烧钱/卡死”的基础风险；后续更像是“默认值/可配置性/可观测性”的产品化问题。

### 3.2 会话自动压缩：存在实现，但缺少可回归验证
- 触发阈值：当 prompt 近似长度（runes）`> 80_000` 时触发压缩。对应实现：`backend/internal/handler/session_compress.go`。
- 压缩行为：将早期历史摘要为一条以 `【会话压缩】` 开头的 assistant 消息，保留最近两轮对话（4 条 text message），并重建 prompt（system + summary + tail + 当前 user）。
- 现状缺口：缺少专项测试与“如何触发/如何确认生效”的可操作路径，导致体感为“很少触发/不知道有没有生效”。

### 3.3 前端“秘书模式”已具备雏形，但全局侧边栏仍暴露
- 已存在独立路由：`/secretary`（`frontend/src/views/Secretary.vue`）。
- `ChatBox` 支持 `secretary/full` 两种 UI 模式并持久化：`frontend/src/components/ChatBox.vue`（localStorage key：`oneagent-chat-ui-mode`）。
- 秘书模式下已隐藏：历史面板、模型/工具/工作区选择、trace、TaskQueuePanel 等（best-effort）。
- 当前关键缺口：`Sidebar` 在登录后全局渲染（`frontend/src/App.vue`），导致“秘书模式仍像管理系统”。

---

## 4) 迭代路线图（从现在到“微信秘书”）

说明：Roadmap 不等于 OpenSpec change；每个可交付迭代都必须落成一个或多个 OpenSpec change（proposal + tasks + spec deltas），并具备可回归验证。

### Phase S0：护栏与可验证性（先把“长跑能力”做实）

目标：避免无限迭代、避免失忆、避免卡死；并让这些能力“可证明”。

产出（建议拆成 changes）：
- **会话压缩可回归**：`add-session-context-compression-verification`（补齐单测/可观测信号/可复现实验）。
- **工具循环上限产品化**：确认默认值策略（例如 150/200）+ 在 doctor/config 中能看见当前生效值（best-effort）。
- **中断/恢复**：对齐 `add-chat-stream-recovery-and-stop`（当前 active change）以支撑“像微信一样随时停/继续”的体验。

验收（Hard Gate 示例）：
- 存在测试能稳定触发一次压缩并断言：summary 写入 session 且 prompt 被重建（不依赖真实 LLM）
- UI/trace 可看到“Context compressed: before → after”（或等价信号）

### Phase S1：秘书模式成为唯一入口（隐藏管理系统外观）

目标：默认进入秘书模式；只有切回完全模式才展示左侧侧边栏与全量菜单。

产出（建议 1 个 change）：
- `update-app-shell-secretary-first`：全局 Sidebar 根据模式显示/隐藏（secretary=隐藏，full=显示）
- 默认启动行为调整：首次进入应用默认是秘书模式（或默认路由跳转至 `/secretary`）
- 模式切换持久化（已存在 localStorage；需变成“全局一致”）

验收（Hard Gate 示例）：
- 在秘书模式下，任意页面不渲染 Sidebar（含移动端菜单按钮）
- 切到完全模式后 Sidebar 恢复，且能正常导航到 Tasks/Governance/Ledger 等页面

### Phase S2：把“管理系统”变成隐形自动化（只在聊天里交付）

目标：用户在秘书模式下不需要知道 Task Queue / Ledger / 工具权限等概念；系统自动使用它们并把关键结果以对话形式交付。

产出（建议拆 2–4 个 changes，按可回归粒度）：
- **聊天驱动的任务化执行**：秘书模式下长任务自动落到 Task Queue（异步），并以对话流展示进度与结果（而不是把所有过程塞进 chat turn）。
- **交付物卡片**：在对话里以“可点击卡片”的方式交付 artifacts（文件路径、diff、导出文档、report 链接）（best-effort）。
- **失败可恢复**：失败时对话给出“下一步按钮”（重试/继续/缩小范围/切 full mode 排障）（best-effort）。

验收（Hard Gate 示例）：
- 用户在秘书模式发起一个耗时任务后，可以刷新页面并看到任务仍在进行/已完成（基于 Task Queue）
- 完成后对话中至少出现一个可验证的交付入口（文件存在/报告可打开）

### Phase S3：全能秘书（长期：多线程与复利）

目标：秘书能同时接住多条委托，自动拆解并行推进；把重复 know-how 沉淀为 Skills/SOP 并可治理，但不打扰默认体验。

对应长期方向（与 `openspec/roadmap-longterm.md` 对齐）：
- Workflow orchestration graph（显式工作流图）
- SOP/Skills 学习管线 + 治理复利
- 多 workspace 队列策略与资源治理

---

## 5) 关键决策点（需要你确认/补充）

1) **工具循环上限默认值**：你希望默认是多少？（目前实现默认 200；你提议 150 是“至少要这么大”的意思，还是必须改成 150？）
2) **秘书模式下是否允许访问非 chat 页面**：仅隐藏入口（可 URL 直达），还是要路由级强制重定向到 `/secretary`？
3) **秘书模式的“回归完全模式”入口形态**：保留当前 Chat header 按钮，还是要更像微信（放在“⋯更多”里/长按/命令面板）？
4) **秘书模式下的可观测性露出**：你希望默认完全不可见，还是要保留一个极轻的“状态/进度”提示（例如一行提示或角标）？

---

## 6) 执行顺序（地基 → 上层，避免搅和在一起）

目标：把后续工作固化成一个“按依赖顺序推进”的队列；每一步都对应一个 OpenSpec change（或补齐现有 change 的残留任务），完成后按 TDD 验证并提交，再进入下一步。

> 状态说明：
> - ✅：已完成（spec+实现+验证）
> - 🟡：进行中/未收尾
> - ⏭️：下一步（建议新建 change）

### L0 地基：运行时边界与安全（跨平台能力）
1) 🟡 `add-native-command-sandbox`：补齐 Linux/Windows native sandbox（Landlock/Restricted Token 等，best-effort）
   - **模块边界**：仅触碰 `backend/internal/sandbox/**`（或等价）、命令工具执行层、doctor；不要把平台细节泄漏到 handler/业务层。
   - **Hard Gate**：集成测试“workspace 内可删、越界拒绝”，并在不可用平台上可预测 skip。

### L1 地基：对话主循环可靠性（长跑能力）
2) ✅ `update-tool-loop-limits-and-write-file-no-truncate`：工具 loop max_steps 可配置 + write_file fail-fast
3) ✅ `add-session-context-compression-verification`：会话压缩可回归 + 阈值可配置（dev/诊断）
4) 🟡 `add-chat-stream-recovery-and-stop`：补齐 QA 收尾（把 4.1/4.2 变成可回归验证或至少可复现实验脚本）
   - **模块边界**：Chat streaming 的可靠性验证优先在 backend integration test + frontend component test；避免把“测试逻辑/调试逻辑”混进生产代码。
   - **Hard Gate**：可回归验证“刷新后可 attach 继续”、“Stop 后不落盘 reply 且后端 generation 终止”。

### L2 壳层：默认就是秘书（入口心智一致）
5) ✅ `update-app-shell-secretary-first`：全局 `ui_mode`（secretary/full）+ Sidebar 仅 full 可见 + deep-link 提示
   - **模块边界**：`ui_mode` 只由 `frontend/src/stores/ui.ts` 负责；任何页面/组件不得各自维护第二份 mode 状态。

### L3 上层：秘书模式的“低噪声可观测性”（不等于把侧边栏搬进来）
6) ⏭️ `add-secretary-status-hints`（建议新建 change）
   - **做什么**：在 `ui_mode=secretary` 下提供极轻的状态提示（例如 SOP 待治理数、任务运行中），且不引入“管理系统外观”。
   - **模块边界**：
     - 状态拉取逻辑抽成 composable（例如 `useLedgerStatusToday`），Sidebar/SecretaryBar 复用，避免重复定时器逻辑。
     - UI 作为 App Shell 的一部分（例如 `SecretaryStatusBar`），不要继续膨胀 `ChatBox.vue`。
   - **Hard Gate**：unit tests 覆盖 badge 出现/隐藏、点击进入 full mode 的路径；不依赖 UI 文案选择器。

### L4 上层：把“管理系统”变成自动化（任务化执行 + 交付）
7) ⏭️ `add-secretary-chat-task-handoff`（建议新建 change）
   - **做什么**：秘书模式下将“长任务”自动落入 Task Queue（异步），对话仅展示进度与最终交付入口（而不是把全过程塞进一轮 chat）。
   - **模块边界**：
     - 后端新增“编排层”（secretary/orchestrator）负责判定/入队/回传状态；不要把 Task Queue 逻辑揉进 `chat.go`。
     - Chat 仍是即时对话；Task Queue 仍是长跑执行；二者通过明确的事件/引用桥接（receipt/task_id）。
   - **Hard Gate**：集成测试：发起一个模拟长任务 → 产生 task + events → 刷新后可恢复显示进度 → 完成后可打开 artifacts。

8) ⏭️ `add-secretary-deliverable-cards`（建议新建 change）
   - **做什么**：对话里结构化交付（文件路径/diff/导出文档/report 链接），形成“可点击卡片”。
   - **模块边界**：渲染组件独立（`DeliverableCard`），后端输出结构化引用（不要让前端从纯文本里用正则猜）。

9) ⏭️ `add-secretary-recovery-actions`（建议新建 change）
   - **做什么**：失败时给出下一步（重试/继续/缩小范围/进入 full 排障），把“可恢复”做成默认体验。

### L5 长期上层：多线程与复利（秘书=编排器）
10) ✅/🟡 对齐 `add-workflow-orchestration-graph` 等长期方向，把“并行工作线/交付物传递/Hard&Soft Gate”落到 Secretary orchestration 上。
