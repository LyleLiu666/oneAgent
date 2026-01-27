## ADDED Requirements
### Requirement: Skill Eligibility Metadata (requires/install)
系统必须 (MUST) 支持在 `SKILL.md` 的 YAML frontmatter 中声明可选的 `requires` 与 `install` 元数据，用于判断技能在当前环境是否可用（eligible）以及向用户展示安装建议。

`requires` 支持以下字段：
- `os`: 允许的 OS 列表（非空时必须包含当前 OS）
- `bins`: 必需存在的可执行文件名列表（全部满足）
- `any_bins`: 至少存在其一的可执行文件名列表
- `env`: 必需存在的环境变量名列表

`install` 支持结构化安装建议（例如 brew/go/node/download/command），用于 status/check 输出。

#### Scenario: 缺少依赖的 skill 不应被自动推荐
- **GIVEN** 存在 skill A，声明 `requires.bins=["memo"]`
- **AND** 当前运行环境缺少 `memo` 可执行文件
- **WHEN** 系统构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 系统不得把 skill A 作为自动推荐技能

### Requirement: Skill Status/Check CLI
系统必须 (MUST) 提供 CLI 用于检查技能可用性，并展示缺失依赖与安装建议：
- `oneagent skills status`：列出技能可用性信息
- `oneagent skills check`：`status` 的别名

#### Scenario: status 输出包含 missing 与 install hints
- **GIVEN** 存在 skill B，声明 `requires.bins=["memo"]` 且 `install` 提供 brew 安装方式
- **AND** 当前环境缺少 `memo`
- **WHEN** 用户运行 `oneagent skills status`
- **THEN** 输出中包含 skill B 的 missing 信息（包含 `memo`）
- **THEN** 输出中包含与 brew 对应的安装提示（例如 `brew install ...` 或 command）
