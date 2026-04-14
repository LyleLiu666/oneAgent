## Context
当前 workspace 选择链路分成两种：

- Windows / macOS：调用本机原生目录选择器
- Linux / Docker：直接返回“不支持”，只能手填

这导致 Docker 场景虽然拥有容器内文件系统视角，却没有一个可视化的目录选择入口。

## Goals / Non-Goals
- Goals:
  - 让 Docker / Linux 场景能在网页里浏览并选择服务端可见目录
  - 保持手动输入仍可用，作为兜底路径
  - 保持默认 UI 简单，不做完整文件管理器
  - 默认只暴露明确允许的根目录，避免把整个容器文件系统暴露给前端
- Non-Goals:
  - 不支持浏览文件，只展示目录
  - 不做多选、搜索、收藏、最近目录等增强能力
  - 不替换 macOS / Windows 的原生 picker

## Decisions
- Decision: 新增独立的 workspace browse API，而不是改写原有 `/api/workspace/choose`
  - Why: 原有接口语义是“调用服务端原生 picker 并直接返回结果”；网页浏览需要逐级列目录，交互模型不同

- Decision: 目录浏览仅限于允许根目录集合
  - Why: 容器里可能存在大量系统目录，不应该默认全部暴露
  - Initial roots:
    - `ONEAGENT_HOME`
    - `DEFAULT_WORKSPACE`（若存在）
    - `BASH_ROOT_DIR`（若显式配置）
    - 当前实现中存在且可访问的去重结果

- Decision: 前端统一走一个轻量弹窗
  - Why: 当前多个页面都有“选择工作区”入口，抽成统一弹窗能减少行为漂移

## Risks / Trade-offs
- 风险: 允许根目录过少时，用户可能仍需要手动输入
  - 缓解: 保留手动输入；提示“这里只展示服务端允许浏览的目录”

- 风险: 目录存在但当前进程无权限读取
  - 缓解: 后端返回明确错误，前端展示错误横幅，不静默失败

- 风险: symlink 或 `..` 导致越界浏览
  - 缓解: 后端统一做绝对路径、真实路径与根目录包含关系校验

## Migration Plan
1. 新增 browse API 与 config capability
2. 新增前端浏览弹窗并接入现有 workspace 入口
3. 保留原生 picker 和手动输入
4. 在 Docker 中重建并验证 `/data` 浏览路径

## Open Questions
- 第一版是否需要把 workflow artifacts root 也加入允许根目录
  - 当前先不加，避免把内部运行产物目录当成默认工作区入口
