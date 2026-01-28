# Design: Pin canonical skill

## Core idea
当一个冲突 candidate 来自不可归档来源（如 `~/.claude`）时，最稳妥的治理手段是：
1) 将该 candidate 的内容复制到更高优先级的 oneAgent personal skills（`ONEAGENT_HOME/.oneagent/skills/<id>/SKILL.md`）
2) 让 discovery/recall 通过 precedence 自动选择该 personal canonical，从而“shadow”低优先级版本

## Pin semantics (v1)
- 输入：`skill_id` + `source/path`（用于定位 candidate）
- 输出：`canonical_path`（personal skill 路径）+ `shadowed_candidates`（可选列表）

## Optional follow-up
- “archive shadowed personal” 仅对 oneAgent personal duplicates 生效（移动到 `skills-archived/`）

