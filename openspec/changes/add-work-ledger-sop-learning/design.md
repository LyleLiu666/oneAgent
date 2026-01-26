## Context
oneAgent 的“可交付”能力正在从一次性对话升级为“持续数小时推进的任务交付”。在此过程中，用户信任的来源不应该是“模型说完成了”，而是可复核的证据（diff/测试报告/产物路径/trace）。

另一方面，用户留存的根本是“经验复利”：当某类任务反复出现，系统应能从历史交付中总结出 SOP，并在用户确认后复用到未来任务中。

## Goals / Non-Goals
- Goals
  - 每次交付都能自动形成 Receipt（机器可读 + 人可读），进入可检索的 Work Ledger。
  - 日报/Digest 将“成果 + 待决策点 + 风险变更 + 证据链接”聚合出来，减少用户逐条查看成本。
  - SOP 提炼存在一个明确的中间态：**默认不启用**，只有人工确认才启用（避免误学/污染）。
  - 数据归属为 **个人**：SOP/习惯默认只影响当前用户（非组织共享）。
  - v1 先做**单机版（单用户）**：不引入账号体系，避免过早复杂化。
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

## Decisions
- Decision: 数据默认按“个人”隔离（v1=单机单用户）
  - v1 单机版将 `principal_id` 视为隐式常量（例如 `local`），API 无需引入多用户身份识别。
  - 未来托管/私有部署多用户化时，再将 `principal_id` 作为一等字段贯穿存储与 API（单独变更）。
- Decision: SOP 必须人工确认才启用（中间态）
  - 任何自动生成的 SOP suggestion **不得**默认生效，避免错误经验污染未来任务。
- Decision: 启用后的 SOP 以“个人 skill 包”物化
  - 优先落盘到 oneAgent 管理的 personal skills 路径（而不是写入 workspace），以符合“个人资产”的语义。
  - skills discovery/recall 通过新增 source 支持该路径。

## Open Questions
- SOP 提炼策略：v1 采用启发式聚类（基于 tags、文件类型、工具序列），还是允许用 LLM 辅助总结（需要成本与隐私边界）？
- Digest 触发：按自然日定时生成 vs 每次任务完成增量更新？
