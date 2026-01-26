## ADDED Requirements
### Requirement: Receipt（交付收据）必须生成且可追溯
系统必须 (MUST) 为每次“可交付的工作尝试（attempt）”生成一个 Receipt，并持久化为：
- 机器可读：`receipt.json`
- 人类可读：`receipt.md`

Receipt 必须包含至少以下字段：`receipt_id`、`principal_id`、`workspace_root`（可空）、`kind`、`status`、`started_at`、`finished_at`、`summary`、以及可追溯证据引用（至少包含 `findings_path` 与 `trace_log_path` 或等价指针）。

#### Scenario: 成功交付生成 receipt.json 与 receipt.md
- **GIVEN** 一次工作尝试执行完成并进入 `succeeded`
- **WHEN** 系统持久化该次尝试的交付结果
- **THEN** 系统生成 `receipt.json` 与 `receipt.md`
- **THEN** `receipt.json` 包含 `summary` 与 `findings_path/trace_log_path`（或等价指针）

#### Scenario: 失败交付仍生成 receipt（用于复盘）
- **GIVEN** 一次工作尝试执行完成并进入 `failed`
- **WHEN** 系统持久化该次尝试的交付结果
- **THEN** 系统仍生成 `receipt.json` 与 `receipt.md`
- **THEN** `receipt.json` 的 `status=failed` 且包含可操作的失败原因与证据引用

### Requirement: Work Ledger（个人）支持查询、过滤与检索
系统必须 (MUST) 提供一个 Work Ledger，将 receipts 按 `principal_id` 归集，并提供查询能力（API 或等价接口），至少支持：
- 按 workspace 过滤（可空/可多值）
- 按状态过滤（succeeded/failed/...）
- 按时间范围过滤（date range）
- 关键字检索（至少覆盖 receipt 的 `summary` 与 `receipt.md` 文本）

#### Scenario: 查询某 workspace 的最近 receipts
- **GIVEN** 某用户在 workspace A 上存在多条 receipts
- **WHEN** 用户查询 workspace=A 且 limit=10 的 receipts 列表
- **THEN** 系统返回最多 10 条结果且按时间倒序稳定排序
- **THEN** 每条结果包含可打开的 receipt 详情引用（例如 receipt_id 或详情 URL）

#### Scenario: 单机版默认单用户（隐式 principal_id）
- **GIVEN** oneAgent 以单机版运行（无账号/SSO）
- **WHEN** 用户查询 receipts 列表
- **THEN** 系统使用隐式的 `principal_id`（例如固定为 `local`）进行归集与过滤

### Requirement: Digest（日汇总）可用于“定期收割成果”
系统必须 (MUST) 为每个 `principal_id` 提供日级 Digest 的生成与查看能力，用于将一天内的 receipts 聚合为：
- 完成清单（可交付项）
- 失败/待决策点清单（需要用户介入）
- 每条都附证据链接（指向 receipt 或其 artifacts）

#### Scenario: 日汇总包含完成与失败清单
- **GIVEN** 某用户当天产生了 succeeded 与 failed 的 receipts
- **WHEN** 系统为该用户生成当天 Digest
- **THEN** Digest 同时包含完成清单与失败/待决策点清单
- **THEN** Digest 中每条条目都包含可追溯引用（例如 receipt_id 或 artifacts 指针）

### Requirement: Receipt → SOP Suggestion（中间态，不默认启用）
系统必须 (MUST) 基于 Work Ledger 支持自动发现重复出现的交付模式，并生成 SOP Suggestion（建议 SOP），且必须进入“中间态”（proposed），默认不启用。

SOP Suggestion 必须包含至少：`suggestion_id`、`title`、`description`、`evidence_count`、`evidence_receipt_ids[]`、`draft_skill`（或等价草稿内容）、以及 `status=proposed`。

#### Scenario: 重复模式产生 SOP Suggestion
- **GIVEN** 某用户在一段时间内产生了多条高度相似的 receipts（系统判定为重复模式）
- **WHEN** 系统运行 SOP 提炼
- **THEN** 系统创建一条 `status=proposed` 的 SOP Suggestion
- **THEN** SOP Suggestion 包含 evidence_receipt_ids 指向这些 receipts

### Requirement: 人工确认后才启用 SOP（proposed → approved）
系统必须 (MUST) 要求用户对 SOP Suggestion 进行人工确认后才启用：
- `proposed` 的 SOP 不得 (MUST NOT) 自动参与 skills recall/推荐
- 用户确认时系统应该 (SHOULD) 允许用户编辑（例如名称、描述、scope、步骤）
- 用户确认后，SOP 状态变为 `approved`（或等价终态），并进入可复用路径

#### Scenario: proposed 不参与召回，approved 才参与
- **GIVEN** 存在一个 `status=proposed` 的 SOP Suggestion
- **WHEN** 系统执行 skills recall 或生成技能推荐
- **THEN** 该 SOP 不出现在可选技能集合中
- **WHEN** 用户将该 SOP Suggestion 设为 `approved`
- **THEN** 该 SOP 作为可复用能力出现在可选技能集合中

### Requirement: approved SOP 物化为个人 skill 包
系统必须 (MUST) 在 SOP Suggestion 被用户批准后，将其物化为一个个人 skill 包（包含 `SKILL.md`，可选包含 scripts/templates），并保证其可被技能发现与 `skill.read` 读取。

#### Scenario: 批准后可通过 skill.read 读取
- **GIVEN** 用户批准了一个 SOP Suggestion（进入 `approved`）
- **WHEN** 系统完成物化流程
- **THEN** 系统创建一个可发现的 skill 包
- **THEN** agent 可通过 `skill.read` 读取该 skill 的 `SKILL.md`
