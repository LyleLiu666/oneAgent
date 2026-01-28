## ADDED Requirements

### Requirement: Model configuration MUST include a safety tier
系统必须 (MUST) 允许为每个 Model 配置一个“安全档位”（例如 `safety_tier=high|standard` 或等价字段），用于表达该模型在安全对齐/拒绝越权方面的相对强弱。

该字段用于：
- UI 提示与默认推荐
- 工具暴露/权限策略的参考（高风险工具默认需要 `safety_tier=high`，除非管理员显式放开）

#### Scenario: High-risk tools are gated by safety tier
- **GIVEN** 当前会话选择的 model 的 `safety_tier=standard`
- **WHEN** 系统决定是否向模型挂载高风险工具（例如 `bash`）
- **THEN** 系统默认不挂载（或返回明确提示需要切换到 `safety_tier=high` 的模型/策略放开）

#### Scenario: UI surfaces recommended safe models
- **GIVEN** 系统存在多个 models，其中一部分 `safety_tier=high`
- **WHEN** 用户在 UI 选择模型
- **THEN** UI 对 `safety_tier=high` 的模型显示推荐/提示（best-effort）
