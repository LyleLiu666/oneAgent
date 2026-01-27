## Context
oneAgent 的“可交付”能力正在从一次性对话升级为“持续数小时推进的任务交付”。在此过程中，用户信任的来源不应该是“模型说完成了”，而是可复核的证据（diff/测试报告/产物路径/trace）。

另一方面，用户留存的根本是“经验复利”：当某类任务反复出现，系统应能从历史交付中总结出 SOP，并在用户确认后复用到未来任务中。

本变更也承认一个现实：大模型会持续进化，但仍无法脱离提示词工作。提示词像给精密仪器照射的一束光——角度、强弱、范围不同，会显著影响输出。**Skills 从技术上是“被验证可用的提示词/流程知识”的持久化载体；从业务上是 SOP/know-how 的资产化。**

## Goals / Non-Goals
- Goals
  - 每次交付都能自动形成 Receipt（机器可读 + 人可读），进入可检索的 Work Ledger。
  - 日报/Digest 将“成果 + 待决策点 + 风险变更 + 证据链接”聚合出来，减少用户逐条查看成本。
  - SOP 提炼存在一个明确的中间态：**默认不启用**，只有人工确认才启用（避免误学/污染）。
  - 数据归属为 **个人**：SOP/习惯默认只影响当前用户（非组织共享）。
  - v1 先做**单机版（单用户）**：不引入账号体系，避免过早复杂化。
  - 学习/提炼/治理是 **独立管线**：不得把学习逻辑耦合进 Task/Subagent 的主执行链路；学习失败不影响交付。
- Non-Goals
  - 立即做组织级 SOP 共享/审批流/RBAC（后续单独变更）。
  - 追求“提示词不可逆防复制”的强 DRM（本阶段更关注可交付与可复核）。

## Key Concepts
### Receipt（收据）
一次“可交付工作尝试”的标准化产物，用于信任与审计。最小字段建议：
- `receipt_id`
- `principal_id`（个人身份；本地模式可视为单用户）
- `workspace_root`（可空）
- `kind`：例如 `task_attempt` / `subagent_run` / `chat_session`
- `status`：`succeeded|failed|canceled|timed_out|interrupted`
- `started_at/finished_at`
- `summary`（面向用户的短总结）
- `artifacts`：`findings_path/trace_log_path/test_report_path/diff_ref` 等指针
- `signals`：耗时、成本、使用模型、关键工具、风险等级（可选）

Receipt 应同时生成：
- `receipt.json`（机器可读，便于聚合/检索/学习）
- `receipt.md`（人可读，便于直接交付/转发）

### Work Ledger
按个人维度汇总 receipts，并提供：
- 列表/过滤（workspace、日期、状态、标签）
- 全文检索（summary、结论、关键文件）
- 与任务/会话/trace 的互链

### Digest（日汇总/通知）
从 receipts 生成的日级聚合视图（默认 in-app；可扩展 webhook/邮件/Slack）。
Digest 的核心不是“流水账”，而是：
- 今日完成了什么（可交付清单）
- 哪些失败需要决策/人工介入
- 哪些变更风险较高（例如 touching auth/payment）
- 每条都必须附证据链接（receipt/trace/findings/diff）

### SOP Suggestion（中间态）
系统从历史 receipts 中发现重复模式后生成的“建议 SOP”，但默认不启用。
建议至少包含：
- `suggestion_id`
- `title/description`
- `evidence_count` + `evidence_receipt_ids[]`
- `draft_skill`（可直接转为 SKILL 包的草稿内容）
- `risk_notes`（可能的误用/边界）
- `status`: `proposed|approved|rejected`

启用机制：
1) proposed：系统生成，**不参与召回**，仅展示给用户 review
2) approved：用户确认（可编辑）后，系统落盘为个人 skill 包并接入召回
3) rejected：用户拒绝，系统保留记录但不再打扰（可配置）

## Architecture: Independent Learning Pipeline（学习独立管线）

原则：学习必须“旁路化”，不影响任何既有功能的正确性与稳定性。

推荐将 Work Ledger 分成两条链路：

1) **Delivery Path（交付主链路）**：Task/Subagent 完成后生成 Receipt 并落盘（必须快、可靠、可恢复）。
2) **Learning Path（学习旁路）**：消费 receipts 生成 digest、SOP suggestions、以及治理操作（可慢、best-effort、可重跑）。

关键约束：
- Receipt 生成必须 (MUST) **不阻塞** Task/Subagent 的终态落盘；学习链路失败不得影响 attempt status。
- 学习链路应 (SHOULD) 具备幂等与断点续跑：同一 receipt 重放不会产生重复记录。
- 学习链路的 LLM 调用是可选增强：未配置 provider 时系统仍应可生成 receipt 与 ledger 查询；SOP 提炼可降级为“暂不生成建议”。

## Evidence-first（强制证据与 e2e 有效性）

系统必须把“可学习”与“可留痕”区分开：
- **可留痕**：所有 attempts（成功/失败/取消/中断）都可生成 receipt，用于复盘与审计。
- **可学习**：只有满足“可交付 + 可验收 + 有证据”的 receipts 才能进入 SOP 提炼的候选集合。

建议的 v1 可学习门槛：
- receipt.status == `succeeded`
- 至少包含 `findings_path` 与 `trace_log_path`
- 若任务需要命令验收，则必须包含“由主/子 agent 生成的测试报告文件”或等价证据（Observer 只读读取判定，不执行命令）

## Governance（SOP/Skills 治理：去重/合并/过时淘汰）

目标：自动学习的技能应该是“复杂 know-how”，且必须可治理，避免资产膨胀与召回污染。

### 1) 什么才算“可学习的复杂 skill”

draft_skill（最终会物化为 `SKILL.md`）在 v1 至少应包含：
- 使用时机/边界（WHEN to use / WHEN NOT to use）
- 分步流程（可复用步骤，包含关键工具选择）
- 常见坑/异常处理（failure handling）
- 验收方式（如何证明有效：文件/测试报告/产物路径）

同时要避免把“复杂”误解为“啰嗦”。参考 `skill-creator` 的核心原则：
- 默认假设模型已经很聪明：只写模型**缺**的内容（能显著改变执行质量的 know-how / guardrails / 验收方法），不要堆常识。
- 通过“合适的自由度”提升可靠性：越脆弱/越危险的流程，越应落为确定性的脚本/模板（可选）。

### 1.1) Depth 的一个可操作判据：Prompt Compressibility Test（不可压缩性）

直觉：如果一个 SOP/skill 的核心提示词改成一句“简单提问”也能获得近似效果，那么它更可能是“蜻蜓点水/常识性”，不值得沉淀为长期资产。

因此 v1 建议为每个 suggestion 做一个 LLM 辅助的压缩测试（结果仅用于排序与人审辅助，不自动生效）：
1) 生成一个最小化的“简单提问版 prompt”（compression_prompt）
2) 让评估器判断：在相同 workspace/约束/工具集合下，`compression_prompt` 是否能达到与 draft_skill SOP 近似的交付效果
3) 若“基本能达到”，则 depth 降低（更像可被常规对话替代）；若“明显不能”，则 depth 更高（更像真正 know-how）

这与“模型会越来越强但仍需要提示词作为光束”的观点一致：我们记录的是“能稳定改变执行质量的那束光”，而不是随便一句话也能得到的输出。

### 2) 生命周期状态（建议）

- Suggestion: `proposed | approved | rejected | merged | deprecated`
- Skill（个人）：`active | deprecated | archived`

### 3) 治理操作（必须具备）

- **Dedup**：检测近似 suggestions/skills（标题/结构/关键步骤/证据集合相似），合并为单一 canonical 条目。
- **Merge**：保留 `merged_from_ids[]` 与证据映射，避免丢失“为什么学到它”的解释链。
- **Drop/Archive**：当 skill 长期不再适用或被新流程替代时，可被标记 deprecated 并从召回集合移除（但保留审计记录与历史证据）。
  - 推荐实现：将已归档的个人 skill 包从 `ONEAGENT_HOME/.oneagent/skills/` 移动到 `ONEAGENT_HOME/.oneagent/skills-archived/`，避免文件扫描阶段进入 discovery/recall。

### 4) Inbox（待治理工作台）与“Top-10 竞赛”策略

目标：把“主观审阅带宽”当作稀缺资源来保护。v1 采用一个简单的 inbox：
- 仅展示列表（不强制按日期分组）
- 每个 principal 每天最多保留 10 条 `proposed`（可配置但默认 10）

当当天已达到 10 条时，每新增 1 条候选，不是简单追加，而是触发一次“11 选 10”竞赛：按排序保留更好的 10 条（被挤出者进入 backlog/parked，不丢失证据）。

排序建议使用“可解释打分”（用于替换决策与 UI 展示），核心维度：
- **Scarcity（稀缺性/新颖性）**：与已存在 skills/suggestions 的相似度越低越稀缺
- **Depth（know-how 深度）**：使用 Prompt Compressibility Test（越不可压缩越深）
- **Evidence Strength（证据强度）**：证据条数/覆盖面（尤其包含失败→恢复→成功的轨迹）

重要：分数只用于辅助“挑剔审阅”的排序与替换，不直接决定自动启用；启用仍必须人工确认。

## Integration Points（与现有能力对齐）

- Task Queue：把每次 task attempt 的交付件（findings/trace/可选测试报告）纳入 receipt artifacts。
- Subagent：至少为 subagent run 生成 receipt（作为 v1 的最小闭环）。
- Skills discovery/recall：只有 **approved/active** 的个人 SOP skills 才进入 discovery/recall；proposed/rejected/archived 不得污染召回。

## Decisions
- Decision: 数据默认按“个人”隔离（v1=单机单用户）
  - v1 单机版将 `principal_id` 视为隐式常量（例如 `local`），API 无需引入多用户身份识别。
  - 未来托管/私有部署多用户化时，再将 `principal_id` 作为一等字段贯穿存储与 API（单独变更）。
- Decision: SOP 必须人工确认才启用（中间态）
  - 任何自动生成的 SOP suggestion **不得**默认生效，避免错误经验污染未来任务。
- Decision: 启用后的 SOP 以“个人 skill 包”物化
  - 优先落盘到 oneAgent 管理的 personal skills 路径（而不是写入 workspace），以符合“个人资产”的语义。
  - skills discovery/recall 通过新增 source 支持该路径。
 - Decision: 学习管线必须独立于交付管线
  - receipt/ledger 的落盘与查询是主链路；digest/sop-learning/governance 是旁路，失败不得影响交付。
 - Decision: 证据是硬门槛
  - v1 的 SOP 提炼仅消费通过验收的 succeeded receipts，并强制附带证据引用；否则宁可不学。

## Open Questions
- SOP 提炼策略：v1 采用启发式聚类（基于 tags、文件类型、工具序列），还是允许用 LLM 辅助总结（需要成本与隐私边界）？
- Digest 触发：按自然日定时生成 vs 每次任务完成增量更新？
- 去重/合并策略：v1 先基于“结构化字段 + 近似文本”做 deterministic merge，还是用 LLM 做相似度判定（成本/可解释性权衡）？
- 过时判定：以“长期未被引用/用户主动标记/多次失败证据”为准，还是加入自动规则（避免误删）？
