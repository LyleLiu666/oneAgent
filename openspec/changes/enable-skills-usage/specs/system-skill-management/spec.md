# 规范: 系统技能管理 (System Skill Management)

## ADDED Requirements

### Requirement: 系统技能发现 (System Skill Discovery)
系统必须 (MUST) 检测位于全局系统配置目录（例如 `~/.oneagent/skills`）中的技能，而不是本地项目目录中的技能。

#### Scenario: 忽略本地技能 (Scenario: Ignore Local Skills)
假如项目根目录下存在文件 `./skills/local-skill.md`
当聊天会话开始时
那么系统提示词中**不得**包含 "local-skill"

#### Scenario: 检测全局技能 (Scenario: Detect Global Skills)
假如存在文件 `~/.oneagent/skills/global-master/SKILL.md` (模拟路径)
并且文件名为 "MasterArchitect"
当聊天会话开始时
那么系统提示词中**必须**包含 "MasterArchitect"

### Requirement: 中文上下文注入 (Chinese Context Injection)
系统必须 (MUST) 使用中文提示词提供技能列表，以符合用户偏好。

#### Scenario: 中文提示词标题 (Scenario: Chinese Prompt Header)
假如存在至少一个全局技能
当生成系统提示词时
那么它**必须**包含 "## 可用技能"
并且它**必须**包含 "你需要先读取技能文件"

#### Scenario: 列表中的技能描述 (Scenario: Skill Description in List)
假如存在一个全局技能 "Translator"，描述为 "Expert in translation"
当生成系统提示词时
那么它**必须**包含 "- Translator: Expert in translation"
