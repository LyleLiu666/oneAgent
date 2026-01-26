# Change: Work Ledger + Receipt-driven SOP learning (personal, staged enablement)

## Why
随着 oneAgent 进入“可交付、可托管、可复用”的商业化阶段，仅有对话与工具调用不足以形成长期价值：用户一旦离开，若没有积累的数据与使用习惯，迁移成本会很低。

我们需要把“交付过程”沉淀为资产：
- **可验证的交付证据**：让非技术用户也能理解并信任（避免“感觉被忽悠/被骗”）。
- **可复用的 SOP**：从重复出现的交付模式中自动提炼出可复用流程，形成个人经验复利。
- **可持续的习惯**：默认模型/技能/流程偏好随使用自然沉淀，减少下次上手成本。

## What Changes
- 新增一个 **Work Ledger** 能力：为每次“可交付工作尝试（attempt）”生成并持久化 Receipt（收据），包含 summary + 证据引用（diff/测试报告/findings/trace）。
- 提供按用户（个人）维度的 ledger 查询与检索能力（列表、过滤、全文检索）。
- v1 先做**单机版（单用户）**：不引入账号/SSO；`principal_id` 可视为隐式常量（例如 `local`）。
- 提供 “完成通知/日报（Digest）” 的基础能力：从 ledger 生成日级汇总，让用户定期回来“收割成果”。
- 增加 “Receipt → SOP” 的自动提炼：当系统检测到重复模式时生成 **SOP Suggestion（建议）**，进入**中间态**（draft/proposed），只有用户人工确认后才会启用为 Active SOP。
- 启用后的 SOP 以 “个人 skills 包” 的形式落盘并接入 skills discovery/recall（便于在后续任务中复用）。

## Impact
- Affected specs:
  - `data-storage`（新增 receipts/digests/sop-suggestions 的落盘规则）
  - `system-skill-management`（新增 oneAgent-managed personal skills source）
  - `skill-recall`（召回来源扩展到 personal skills）
  - **New**: `system-work-ledger`（Receipt/Ledger/Digest/SOP-staging 规格）
- Affected code (expected):
  - Backend: receipt 生成、ledger store/index、digest 生成、SOP suggestion store、审批/启用 API
  - Frontend: Work Ledger 页面（列表/搜索/详情）、SOP Suggestions（review/approve/edit/reject）、Digest 入口
