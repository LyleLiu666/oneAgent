# Change: Add skill eligibility metadata + status CLI

## Why
当 skills 数量上千时，“召回到的 skill 是否真的能在当前环境执行”成为主要摩擦点：模型读到了 SKILL.md，但缺少依赖/不支持当前 OS，最终只能失败重试，浪费时间与上下文预算。

参考 Clawdbot 的 skill 机制，本变更为 oneAgent skill 引入机器可读的 `requires/install` 元数据，并提供 `oneagent skills status/check` 来让用户快速定位缺失与安装方式；同时让“自动推荐技能”仅推荐 **eligible** 的 skill。

## What Changes
- 在 `SKILL.md` YAML frontmatter 中支持可选 `requires` 与 `install` 字段（机器可读）
- 增加 skill eligibility 计算：根据 `requires` 判断当前环境是否可用
- 自动技能推荐（chat TurnContext）与子 agent Top-K 技能注入仅使用 eligible skills
- 新增 CLI：`oneagent skills status`（`check` 作为别名）展示技能可用性/缺失依赖/安装建议

## Impact
- Affected specs: `skill-recall`, `system-skill-management`, `system-subagent-orchestration`
- Affected code: `backend/internal/skill`, `backend/internal/handler`, `backend/internal/tool`, `backend/cmd/oneagent`
