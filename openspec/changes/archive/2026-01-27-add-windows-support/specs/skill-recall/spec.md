## MODIFIED Requirements
### Requirement: 支持多来源发现并基于本地文件召回（grep/ripgrep）
技能召回工具必须 (MUST) 能从以下来源发现技能并进行召回（对以上来源取并集后按 `name` 去重）：
1) `<workspace>/.oneagent/skills`
2) `<workspace>/skills`
3) `<workspace>/.claude/skills`
4) `~/.claude/skills`
5) `~/.codex/skills`
6) 内置 skills（随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

技能召回工具应该 (SHOULD) 优先使用 `rg`（ripgrep）对 `SKILL.md` 内容进行匹配计分；在 `rg` 不可用时系统必须 (MUST) 降级为 `grep -R` **或等价实现** 以保持基本可用，并通过 `doctor` 明确提示安装/启用更快的搜索后端；整个流程不得依赖外部网络。

#### Scenario: rg 缺失时仍可通过等价降级召回
- **GIVEN** 运行环境未安装 `rg`
- **WHEN** 执行技能召回（基于本地文件）
- **THEN** 工具仍可返回技能候选列表（允许性能下降）
- **THEN** 系统能提供可操作的提示（例如提示安装/启用 `rg`）

#### Scenario: Windows 无 rg/grep 时仍可用
- **GIVEN** Windows 环境未安装 `rg` 且不存在 `grep`
- **WHEN** 执行技能召回（基于本地文件）
- **THEN** 工具仍可返回技能候选列表（通过等价实现降级）

#### Scenario: ~/.claude skills 为 symlink 目录
- **GIVEN** `~/.claude/skills/code-review-excellence` 是一个 symlink，指向某个真实目录且该目录内包含 `SKILL.md`
- **WHEN** 执行技能召回
- **THEN** 召回结果不得因 symlink 而漏掉该技能

