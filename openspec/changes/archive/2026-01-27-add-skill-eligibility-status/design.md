## Context
skills 本质是“给 AI 的说明书”，但说明书经常隐含外部依赖（CLI/bin、环境变量、仅 macOS 可用等）。缺少机器可读元数据会导致召回/推荐无法判断可用性，模型只能在执行阶段失败。

## Goals / Non-Goals
- Goals:
  - 允许 skill 以机器可读方式声明依赖与安装建议
  - 自动推荐只推荐当前环境可用（eligible）的 skill
  - 提供一条命令快速查看缺失依赖与建议安装方式
- Non-Goals:
  - 不做自动安装/自动执行安装脚本（只做提示与可见性）
  - 不引入 embedding/vector index（仍保持离线、本地可解释）

## Decisions
- Frontmatter schema（可选）：
  - `requires`:
    - `os`: ["darwin"|"linux"|"windows"]（非空则限制 OS）
    - `bins`: 必需存在的可执行文件名列表（全部满足）
    - `any_bins`: 至少存在其一的可执行文件名列表（满足其一）
    - `env`: 必需存在的环境变量名列表
  - `install`: 安装建议列表（机器可读，便于 status 输出），字段支持：
    - `kind`: "brew"|"go"|"node"|"uv"|"download"|"command"
    - `label`: 可选显示名
    - `formula`/`module`/`package`/`url`/`command`: 对应 kind 的主要参数
    - `bins`: 该安装方式提供的 bin（可选，用于提示）
    - `os`: 该安装方式适用 OS（可选）

## Risks / Trade-offs
- 解析 schema 后会增加输出字段与检查逻辑：通过 `omitempty` 与 CLI 专用输出控制膨胀。
- “eligible” 判断仅基于本机 PATH/env/OS：无法覆盖更深层依赖（例如权限、外部服务）。
