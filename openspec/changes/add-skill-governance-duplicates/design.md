# Design: Skill duplicates discovery for governance

## Goals
- 运行时 `skill.Discover` 继续保持“按优先级去重只保留最终生效版本”的行为（不改动召回语义）
- 治理场景提供“全量 candidates”视图：同一 `skill_id` 可能有多个候选版本
- 标记最终生效版本：与 `skill.Discover` 的优先级规则一致

## Approach
1) 新增 `skill.DiscoverCandidates`（或等价）：
   - 扫描与 `Discover` 相同的 source roots（含 builtin）
   - **不去重**，返回所有解析成功的 `Skill` 列表，并附带：
     - `precedence_rank`：source roots 的扫描顺序（越小优先级越高）
     - `effective`：同一 `skill_id` 中 precedence 最小者为 `true`
2) 新增 API：`GET /api/skills/duplicates`
   - 仅返回存在冲突（同 `skill_id` candidates >= 2）的分组
3) 前端在 Skill Governance 页面新增 “Duplicates”
   - 展示每组冲突的所有 candidates
   - 复用现有 archive（个人 skills）能力，用于解除污染/切换生效版本

## Pitfalls / notes
- `skill_id` 来源于 name normalized；同名冲突组的语义清晰，但不同命名的“语义相似”不在本 change 处理范围
- builtin skills 没有真实文件路径，不能 archive/edit；UI 需要区分
- macOS `/var` vs `/private/var` 的路径归一化问题：archivable 判断仍以 `EvalSymlinks` 为准

