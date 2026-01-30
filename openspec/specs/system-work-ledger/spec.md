# system-work-ledger Specification

## Purpose
TBD - created by archiving change add-work-ledger-sop-learning. Update Purpose after archive.
## Requirements
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

### Requirement: 学习/提炼/治理必须是独立管线（不影响交付主链路）
系统必须 (MUST) 将 Work Ledger 的“交付主链路”与“学习旁路”解耦：
- 交付主链路：attempt 完成后生成 receipt 并落盘
- 学习旁路：消费 receipts 生成 Digest、SOP suggestions 与治理操作（去重/合并/废弃等）

系统必须 (MUST) 保证学习旁路的失败不会影响 attempt 的终态与可查询性；学习旁路应 (SHOULD) 支持幂等与断点续跑。

#### Scenario: 学习失败不影响 receipt 与 attempt 终态
- **GIVEN** 一次 attempt 已进入终态并生成 receipt
- **WHEN** 学习旁路（Digest/SOP 提炼/治理）在处理该 receipt 时失败
- **THEN** receipt 仍可被查询与检索
- **THEN** attempt 的终态不受影响（不得被回滚或改写为失败）
- **THEN** 用户可在稍后重试学习旁路（best-effort）

### Requirement: SOP Suggestion 必须基于“可验证证据”的成功交付（Evidence-first）
系统必须 (MUST) 将 SOP Suggestion 的生成限定在“可学习 receipts”集合内；可学习 receipts 至少满足：
- `status=succeeded`
- 具备可追溯证据引用（至少包含 `findings_path` 与 `trace_log_path`）
- 若任务需要命令验收，则必须包含“由主/子 agent 生成的测试报告文件”或等价证据（Outcome Observer 只读读取判定，不执行命令）

#### Scenario: 缺少证据的 receipts 不参与 SOP 提炼
- **GIVEN** receipt A 的 `status=succeeded` 但缺少 `findings_path`（或 trace/e2e 证据缺失）
- **WHEN** 系统运行 SOP 提炼
- **THEN** receipt A 不得进入 SOP Suggestion 的 evidence_receipt_ids

### Requirement: 自动学习的 SOP/Skill 必须包含复杂 know-how（可复用流程 + 验收）
系统必须 (MUST) 确保 SOP Suggestion 的 `draft_skill` 不是“显而易见的一句话提示”，而应包含可复用 know-how 的最小结构：
- 使用时机/边界（WHEN to use / WHEN NOT to use）
- 分步流程（步骤化、可复用）
- 失败处理/常见坑（failure handling）
- 验收方式（如何证明有效：文件/测试报告/产物路径）

#### Scenario: draft_skill 至少包含关键段落
- **GIVEN** 系统生成一个 `status=proposed` 的 SOP Suggestion
- **WHEN** 用户查看其 `draft_skill`
- **THEN** draft_skill 包含“使用时机/边界”“分步流程”“失败处理/常见坑”“验收方式”等段落（或等价结构）

### Requirement: SOP/Skills 必须可治理（去重/合并/废弃）
系统必须 (MUST) 提供 SOP Suggestion 与已批准个人 skills 的治理能力，至少包括：
- 去重（dedupe）：检测近似 suggestions/skills 并合并为 canonical 条目
- 合并（merge）：保留 `merged_from`（或等价字段）与证据映射，避免丢失解释链
- 废弃/归档（deprecated/archived）：从召回集合移除但保留审计记录与证据引用

#### Scenario: 合并后仅 canonical 参与召回
- **GIVEN** 两条高度相似的 SOP Suggestion 被治理流程合并为 canonical（其中一条标记为 merged/deprecated）
- **WHEN** 用户批准并启用 SOP skills
- **THEN** 仅 canonical skill 参与 skills discovery/recall（merged/deprecated/archived 不得参与）

### Requirement: SOP Suggestion Inbox（待治理工作台）默认最多 10 条且采用“Top-10 竞赛”替换策略
系统必须 (MUST) 为每个 `principal_id` 提供一个 SOP Suggestion Inbox（列表视图即可，不要求按日期分组），用于集中展示 `status=proposed` 的待审阅建议。

系统必须 (MUST) 对每天进入 Inbox 的 `proposed` 建议数量施加默认上限为 10 条；当当天已达到 10 条后，每新增 1 条候选建议，系统必须 (MUST) 按“可解释排序”保留更好的 10 条并将较差者标记为 `parked`（或等价状态），以避免无限打扰与噪声膨胀。

#### Scenario: 达到 10 条后新增候选触发替换
- **GIVEN** 某用户当天已有 10 条 `status=proposed` 的 suggestions 在 Inbox
- **WHEN** 系统产生第 11 条候选 suggestion
- **THEN** 系统对 11 条进行排序并仅保留 10 条在 Inbox
- **THEN** 被挤出的 suggestion 被标记为 `parked`（或等价状态）且保留证据引用（不得丢失）

### Requirement: “稀缺性 + know-how 深度”排序必须包含 Prompt Compressibility Test（不可压缩性）
系统必须 (MUST) 为每条候选 SOP Suggestion 计算一个用于排序/替换的人类可解释评分（不用于自动启用），至少包含：
- `scarcity_score`：与已存在 skills/suggestions 的相似度越低越稀缺
- `depth_score`：衡量是否包含不可被“简单提问”替代的 know-how
- `evidence_score`：证据强度（证据条数/覆盖面）

系统应该 (SHOULD) 使用一个 LLM 辅助的 Prompt Compressibility Test 来估计 `depth_score`：
1) 基于该 suggestion 的 draft_skill 生成一个最小化“简单提问版 prompt”
2) 评估器判断：简单提问是否能达到与 draft_skill 近似的交付效果
3) 若“基本能达到”，则 depth 降低；若“明显不能”，则 depth 提高

#### Scenario: 可被简单提问替代的候选 depth 更低
- **GIVEN** 一个候选 suggestion 的核心内容可被一句简单提问替代并预计达到近似效果
- **WHEN** 系统对该 suggestion 进行 Prompt Compressibility Test
- **THEN** 该 suggestion 的 `depth_score` 低于不可压缩的候选

### Requirement: Parked/Backlog 可被用户手动“生成更多”拉回 Inbox
系统必须 (MUST) 为被挤出 Inbox 的候选 suggestions 提供一个 `parked`（或等价）状态，用于保留证据与后续审阅机会（不得丢失 evidence_receipt_ids 与评分信息）。

系统必须 (MUST) 提供一个用户可触发的“生成更多（Load More）”机制，使用户可以从 `parked` 中按排序规则追加拉回 Inbox（列表即可，不要求日期分组）。

系统必须 (MUST) 仅在用户主动触发时才将 `parked` 拉回 Inbox；系统不得 (MUST NOT) 自动把 `parked` 重新塞回 Inbox（避免持续打扰）。

#### Scenario: 用户手动拉回 parked suggestions
- **GIVEN** 用户当天的 Inbox 已满（10 条）且存在若干 `status=parked` 的 suggestions
- **WHEN** 用户在 Inbox 中点击“生成更多”并请求 count=3
- **THEN** 系统从 `parked` 中按排序规则选择 3 条拉回 Inbox（状态变为 `proposed` 或等价）
- **THEN** Inbox 仍遵守“最多 10 条”的上限（必要时触发替换并把被挤出者标记回 `parked`）

### Requirement: Receipts MUST reference test reports when available
系统必须 (MUST) 在存在测试报告时，将其引用写入 Receipt（例如 `test_report_path`），以便用户在 digest/ledger 中直接打开证据。

#### Scenario: Receipt includes test_report_path
- **GIVEN** 某次 attempt 产生了测试报告文件
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含 `test_report_path`

#### Scenario: Receipt remains valid without test_report_path
- **GIVEN** 某次 attempt 未产生测试报告文件（例如无 tests/依赖缺失）
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt 仍然是有效的证据条目（包含 summary/findings/trace 等其他字段）

### Requirement: Receipt MUST reference diff artifacts when available
系统必须 (MUST) 在 attempt 产生“变更证据”（diff/变更摘要）时，将其引用写入 Receipt（例如 `diff_patch_path` / `changed_files_path`），以便用户在 Work Ledger/Digest 中直接打开审查证据。

#### Scenario: Receipt includes diff artifact pointers
- **GIVEN** 某次 attempt artifacts 中包含 `diff_patch_path`（或等价字段）
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含对应 diff 指针字段

#### Scenario: Receipt remains valid without diff artifacts
- **GIVEN** 某次 attempt 未产生 diff artifacts（例如无文件改动）
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt 仍然是有效证据条目（包含 summary/findings/trace 等其他字段）

### Requirement: Review comments MUST be preserved in the evidence chain
系统必须 (MUST) 支持用户对某次 attempt/receipt 提交 review comments，并将其作为 append-only 的证据写入 Work Ledger（例如 `review_comments.jsonl` 或等价结构），以保证：
- 复盘时可看到“用户审查意见如何影响后续交付”
- follow-up attempt 可引用并注入这些 comments

#### Scenario: User submits a review comment and it is persisted
- **GIVEN** 用户打开某条 receipt 的 review 页面
- **WHEN** 用户提交一条 review comment（文本即可）
- **THEN** 系统持久化该 comment 并关联到对应 receipt_id/attempt_id
- **AND** comment 可在后续查询 receipt 详情时被读取（best-effort）

### Requirement: Learning jobs MUST produce governance hints for dedupe/merge (best-effort)
系统必须 (MUST) 在学习管线（learning job）中为每条 SOP suggestion 产出可用于治理的线索（best-effort），以降低人工去重成本：
- 至少包含一个 `similar_suggestion_ids[]` 或等价结构（best-effort）
- 当系统能找到明显的 merge 目标时，输出 `recommended_merge_target_id`（best-effort）
- hints 不得替代证据：suggestion 仍必须包含 `evidence_receipt_ids[]`（best-effort）

#### Scenario: Suggestion includes similar hints and preserves evidence
- **GIVEN** 系统生成了一条 SOP suggestion（best-effort）
- **WHEN** 客户端获取该 suggestion 详情
- **THEN** 返回包含 `evidence_receipt_ids[]`（best-effort）
- **AND** 返回包含 `similar_*` 治理线索字段（best-effort）

### Requirement: Digest MUST persist a structured representation (in addition to markdown)
系统必须 (MUST) 在生成 Digest 时，除 `markdown` 外还产出结构化 Digest（best-effort），用于 UI 的筛选/聚类/批处理动作：
- 至少包含条目列表（items），每条指向一个 receipt（receipt_id）并含可展示的摘要字段（best-effort）
- 结构化 Digest 必须可被读取而不触发 refresh（best-effort）

#### Scenario: Developer can fetch a structured digest for a day
- **GIVEN** 某天存在 Digest（best-effort）
- **WHEN** 客户端请求该天的结构化 Digest
- **THEN** 返回包含 `items[]` 的 JSON（best-effort）
- **AND** 不会隐式触发 digest refresh（best-effort）

### Requirement: Digest MUST provide failure clustering for harvest mode (best-effort)
系统必须 (MUST) 在结构化 Digest 中提供失败聚类（best-effort），用于用户快速定位“同类问题批量处理”：
- clusters 至少可按 `error_code` / `failure_reason` / `tool_name` 等维度聚合（best-effort）
- cluster 条目包含 count 与 receipt_ids 指针（best-effort）

#### Scenario: Failures are grouped into clusters
- **GIVEN** 当天存在多条 failed receipts，且部分失败原因相同（best-effort）
- **WHEN** 系统生成结构化 Digest
- **THEN** 返回的 `clusters[]` 至少包含一个 cluster，`count>=2`（best-effort）

### Requirement: The system MUST support batch follow-up creation from receipts
系统必须 (MUST) 支持用户从一组 receipts 创建 follow-up（批处理下一轮）：
- 输入是 `receipt_ids[]` + 用户补充指令（可选）（best-effort）
- 输出是一个已入队的 task（或等价执行单元），并保留这次 follow-up 的证据（best-effort）

#### Scenario: User creates a follow-up task from selected receipts
- **GIVEN** 用户选中若干 receipt_ids（>=2 best-effort）
- **WHEN** 用户提交 batch follow-up
- **THEN** 系统创建并入队一个新 task（best-effort）
- **AND** 该 task 的上下文包含所选 receipts 的证据指针（best-effort）

### Requirement: SOP governance UI MUST avoid “ghost actions” and provide helpful empty states
系统必须 (MUST) 在 SOP 治理页面对空列表/未选中状态提供友好的空状态，并避免展示无意义或不可用的“幽灵按钮”（例如无数据时的加载更多）。

#### Scenario: Empty SOP list shows guidance and hides irrelevant actions
- **GIVEN** SOP 建议列表为空
- **WHEN** 用户打开 SOP 治理页面
- **THEN** 页面展示引导性空状态（例如说明何时产生建议）
- **AND** 无意义的操作按钮默认不展示或明确禁用（best-effort）

### Requirement: SOP governance copy MUST be user-centered by default
系统必须 (MUST) 在 SOP 治理页面默认文案中避免暴露实现术语（例如“稀缺性/know-how/证据排序”）；可将解释放入可选的帮助入口（例如 tooltip/info，best-effort）。

#### Scenario: Default subtitle is understandable without implementation jargon
- **WHEN** 用户打开 SOP 治理页面
- **THEN** 页面默认副标题使用用户可理解的表述（best-effort）

### Requirement: Receipt MUST reference worktree evidence when used
系统必须 (MUST) 在 attempt 使用 worktree mode 时，将 worktree 相关证据写入 Receipt，以便复盘与恢复：
- `worktree_root`（路径）
- `base_commit_sha`（或等价字段）

#### Scenario: Receipt includes worktree_root and base_commit_sha
- **GIVEN** 某次 attempt 在 worktree mode 下运行且 artifacts 中包含 `worktree_root/base_commit_sha`
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含 `worktree_root`
- **AND** receipt artifacts 包含 `base_commit_sha`

