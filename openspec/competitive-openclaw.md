# 竞争拆解：如何超过 OpenClaw（以 oneAgent 为基底）

日期：2026-02-06

## 范围与取舍（已决策）

本轮明确选择：先把 oneAgent 做成“本地工程交付助手”，专注解决【简单问题 + 复杂问题】，并且全链路留痕（可审查交付物 + 证据 + 回滚 + 少打断）。

明确不做（至少不在第一阶段做）：

- IM/多渠道接入与网关工程（WhatsApp/Telegram/iMessage/Slack/Discord 等）
- 语音/移动端节点/Canvas 等设备侧能力

## 0. 结论先行（避免空谈）

如果“超过 OpenClaw”不先被定义为可量化目标，我们会永远陷在主观体感里。

基于目前公开信息（OpenClaw 的 README/Docs 导航 + oneAgent 仓库现状）：

- OpenClaw 在「多渠道入口 + always-on 网关 + 设备节点（语音/Canvas/移动端）」上领先非常明显。
- oneAgent 的可赢定位不是“渠道数量”，而是「工程交付质量」：更少打断、更稳定的可审查交付物、更强的回滚与证据链、更低的污染成本。
- 所以“超过”的可行路径是：明确一个用户画像与任务集合，用 head-to-head 基准评测把差距量化，然后用 OpenSpec 把差距拆成可验证的 changes，按依赖顺序推进。

## 0.1 我们要打穿的主路径：简单问题 + 复杂问题（都要留痕）

简单问题（同步）：

- 在 chat 内快速完成，不强迫进入 task queue
- 仍然必须留下可审查的产物与证据：summary/findings/trace/diff/test report（可得时）
- 不要为了“灵活”牺牲可回归性：所有降级都要可解释（为什么没有 diff/为什么没跑测试）

复杂问题（异步）：

- 自动 handoff 到 Task Queue，支持 progress/中断/恢复
- 默认可回滚边界（worktree 隔离或等价隔离）与防污染
- 默认交付“可审查包”（manifest + 指针稳定），失败也要留下完整证据框架

## 1. OpenClaw 是什么（我们到底在对标什么）

从 `openclaw/openclaw` 的 README 可提炼出的产品事实（非价值判断）：

- 交付形态：Node.js CLI + onboarding wizard + gateway daemon（control plane），推荐 `openclaw onboard`，并可安装为常驻服务。
- 入口面：覆盖大量聊天渠道（WhatsApp/Telegram/Slack/Discord/Google Chat/Signal/iMessage/Microsoft Teams/WebChat 等），强调“在你已用的渠道里回复你”。
- 设备能力：语音（说/听），Canvas（可控可渲染），并提到 macOS/iOS/Android 等节点。
- 安全默认：对 DM 做 pairing/allowlist，默认 fail-closed（未知发信人不处理）。
- 模型与鉴权：除 API key 外，强调 OAuth 订阅（Anthropic / OpenAI）与 failover。

一句话：OpenClaw 是 messaging-first 的 personal assistant 平台。

## 2. oneAgent 现在是什么（我们真实能交付什么）

从 `README.md` 与 `openspec/` 的事实（加上本仓库实现）可以归纳：

- 交付形态：本地工具形应用，一个 `oneagent` 可执行文件启动（Go 后端内嵌 Vue 前端）。
- 核心能力：workspace 边界 + 工具调用（文件/命令等）+ 长任务 Task Queue（可取消/可恢复）+ Outcome Observer 验收 + skills。
- 产品心智：更像“本地工程助理/跑腿执行器”，主要在本地 UI 内闭环，不是“多渠道聊天 bot”。

一句话：oneAgent 是 local-first 的 agentic 交付工具。

## 3. “超过”必须变成可回归指标（否则你永远不知道有没有变强）

把“超过”定义为：在一组固定任务集上，oneAgent 的端到端交付质量指标 >= OpenClaw（或显著优于）。

建议最小指标集（和 `openspec/changes/add-head2head-benchmark-suite/` 一致）：

- First pass success rate：一次成功交付的比例
- Recovery success rate：失败后无需人肉重做能恢复到成功的比例
- Human intervention count：需要用户介入的次数（追问、手工修环境、手动给参数等）
- Evidence completeness：证据是否齐全（summary/findings/trace/diff/test report 等）
- Cost per successful delivery：每次成功交付的成本（best-effort；至少先做 token/调用次数）

注意：这些指标不解决所有问题，但能把“体感”变成“可对比的数值曲线”。

## 4. 定位选择：想赢，就别同时打所有战场

你要的是“超过他”，不是“长得像他”。最现实的打法是：先选一个你能赢的战场，把它打穿。

推荐的赢面战场（oneAgent 的结构更适配）：

- 工程交付：代码/文档/运维类任务，“给出可审查交付物 + 可复核证据 + 可回滚边界”
- 低打断自治：秘书模式“先自愈后升级”，减少无意义追问
- 简单/复杂任务的统一交付体验：简单问题不打断，复杂问题可挂机，且两者都能稳定留痕

不建议第一阶段硬拼的战场：

- 20+ 渠道适配数量（成本极高，且会拖垮核心交付闭环）
- 语音/移动端节点（需要产品与平台工程投入，先确认目标用户是否真的需要）

## 5. OpenSpec 已经怎么拆（你要的“拆解”就在仓库里）

OpenSpec 在本仓库的“可执行拆解”由三层组成：

- 真相层（What IS）：`openspec/specs/**/spec.md` 定义能力与验收场景。
- 变更层（What SHOULD change）：`openspec/changes/<change-id>/` 用 proposal + tasks + specs delta 拆解“要做什么”。
- 路线图层（Why/Order）：`openspec/roadmap-*.md` 只保留一个入口，避免分叉维护。

当前 active changes（优先级与快照）：`openspec/changes/review.md`。

### 5.1 地基：OpenSpec 真相对齐（已完成）

Change：`openspec/changes/update-openspec-truth-alignment/`

- 目的：防止 spec/doc/roadmap 与真实状态漂移。
- 交付：`scripts/openspec_truth_check.sh` + `make openspec-truth` + CI 接入（`scripts/ci_build.sh`）。
- 验收：本地/CI 同一入口能 fail-fast；capability Purpose 不允许占位；review 快照不漏变更。

### 5.2 P0：衡量“超过”的标尺（head-to-head 基准）

Change：`openspec/changes/add-head2head-benchmark-suite/`

- 目的：把“超过 OpenClaw”变成可回归的指标曲线。
- 核心交付：
  - 固定任务集（先 MVP 20，再扩到 100）
  - runner（可重复、可中断恢复）
  - JSON 报告 + Markdown 报告
  - nightly 与手动触发（不阻塞常规 PR）
- 硬门槛（建议写进实现时的测试）：
  - >=3 条任务本地跑通并产出报告
  - baseline 对比能输出 delta

### 5.3 P0：交付物契约化（让用户“总能拿到可审查包”）

Change：`openspec/changes/update-task-deliverable-contract-v1/`

- 目的：稳定“交付入口”与证据字段，避免 UI/工具链每次都碰运气。
- 核心交付：
  - artifact manifest v1（schema-versioned）
  - 缺失字段必须给 reason code（而不是静默缺失）
  - receipt 与 artifacts 映射一致性校验
- 硬门槛：
  - API 合约测试覆盖成功/失败/降级路径
  - 抽样核对历史 attempts 的兼容与迁移策略

### 5.4 P0：worktree 隔离鲁棒化（可回滚边界 + 防污染）

Change：`openspec/changes/update-worktree-attempt-isolation-v2/`

- 目的：让 mutating attempt 默认在可清理、可回滚的隔离根目录里跑，降低“把仓库跑脏”的概率。
- 核心交付：
  - attempt artifacts 记录 `worktree_root/base_commit_sha/worktree_mode`
  - non-git workspace 在 worktree mode 下 fail-closed（禁止 silent fallback）
  - 清理失败可追溯、可重试；orphan sweeper（best-effort）
- 硬门槛：
  - 单元测试覆盖 git/non-git、越界、base SHA
  - 集成测试覆盖并发、终态清理、orphan 回收

### 5.5 P1：秘书自治（少打断，先自愈后升级）

Change：`openspec/changes/update-secretary-autonomy-selfheal-v2/`

- 目的：把“可自愈的问题”从用户身上拿走，让用户只在真正需要时才介入。
- 核心交付：
  - progress 意图：基于系统快照直接答复（单任务时禁止追问）
  - 自愈：协议解析/参数归一化类错误 bounded repair + retry
  - 升级：预算耗尽才升级给用户，且必须给“下一步动作”
- 硬门槛：
  - 后端测试覆盖单任务进度问答、多任务聚焦、自愈重试上限
  - 前端测试覆盖恢复回执与低噪声提示

### 5.6 P1：多 workspace 调度治理（可解释、可观测、可恢复）

Change：`openspec/changes/update-queue-governance-scheduling-v2/`

- 目的：让任务不会因为规模增长而变成“玄学调度”，并且用户能解释“为什么没跑”。
- 核心交付：
  - 公平性（防饥饿）、priority/并发上限一致性
  - schedule misfire 策略 + 幂等 key
  - 治理决策写入事件流 + 低噪声 UI 汇总
- 硬门槛：
  - 调度测试：多 workspace、公平性、pause/resume、priority
  - schedule 测试：misfire、幂等、重复触发

### 5.7 P2：外部动作面（MCP 从只读到可控写）

Change：`openspec/changes/update-mcp-action-plane-v1/`

- 目的：把 oneAgent 变成“可被外部系统触发的运行时”，而不是只能在本地 UI 点点点。
- 核心交付：
  - MCP 方法：create/resume/cancel task
  - 复用 principal auth/policy/approval（禁止绕过）
  - 审计字段与事件双向引用
- 硬门槛：
  - MCP 集成测试：未授权拒绝、授权通过、审批流
  - 状态机一致性：MCP 与 HTTP API 语义一致

### 5.8 P2：最小渠道中继（先补“入口够用”，再谈全渠道）

Change：`openspec/changes/add-channel-relay-v1/`

- 说明：本轮已明确“第一阶段不做 IM/多渠道接入”，所以该 change 暂时不在主线上推进（保留为后续可选项）。
- 目的：用最小成本把“入站委托/出站通知”跑通，形成渠道闭环。
- 核心交付：
  - ingress schema + webhook adapter（v1 先支持 1 个渠道）
  - provider message id 幂等去重
  - task 终态通知回推来源 thread（可追溯引用）
  - webhook 签名鉴权 fail-closed + 审计日志
- 硬门槛：
  - 端到端测试：入站 -> secretary -> task -> 出站
  - 幂等测试：重复 webhook 不重复创建任务

## 6. 推荐执行顺序（依赖先行）

如果目标是“更快超过”，建议顺序是：

1) `add-head2head-benchmark-suite`（先把尺子立起来）
2) `update-task-deliverable-contract-v1`（稳定交付物口径）
3) `update-worktree-attempt-isolation-v2`（降低污染与回滚成本）
4) `update-secretary-autonomy-selfheal-v2`（减少用户介入）
5) `update-queue-governance-scheduling-v2`（规模化稳定性）
6) （可选后续）`update-mcp-action-plane-v1`（对外触发能力；不影响本地交付主线）
7) （更后）`add-channel-relay-v1`（渠道接入；本轮明确不做 IM/多渠道）

理由：前 3 个 change 决定了“交付可信度”，有了它们，后面的入口扩展才不会把系统带崩。

## 7. 结论：本轮选择的竞争路线

已选择路线：先打穿 “本地工程交付助手（可审查交付物 + 证据 + 回滚 + 少打断）”，并用 head-to-head 基准把进步做成可回归曲线；渠道/IM/设备节点不作为第一阶段目标。
