## Context
OpenSpec 已成为本项目执行中枢；一旦“文档状态”与“真实实现状态”漂移，后续 change 的优先级、依赖关系和验收边界都会失真。

## Goals / Non-Goals
- Goals:
  - 建立可自动验证的 OpenSpec 真相对齐基线
  - 将“状态快照来源”从人工口径切到命令可回放
  - 降低 roadmap 维护的人肉成本与主观误差
- Non-Goals:
  - 本 change 不引入新业务功能
  - 本 change 不重写已有 capability 内容（仅补齐缺口和校验）

## Decisions
- Decision: 以 `project-tooling` 承载 OpenSpec 治理校验能力。
  - Reason: 该能力属于工程质量门禁，适配 CI/本地脚本语义。
- Decision: 优先做“可失败”的硬校验（Purpose 占位、状态快照来源），暂不做复杂语义 lint。
  - Reason: 先解决高风险漂移点，避免一次性过重治理。
- Decision: truth check 脚本不依赖 `openspec` CLI；active changes 以文件系统为准（`openspec/changes/*`）。
  - Reason: CI 与本地都能稳定运行，不引入额外安装依赖；并且“真实存在的变更目录”是最底层可回放来源。

## Risks / Trade-offs
- 风险：早期会暴露大量历史存量问题，短期增加修复成本。
- 缓解：允许分批修复，但新增变更必须立刻满足校验。

## Migration Plan
1. 引入校验脚本并作为 fail-fast 门禁（本地 + CI）
2. 补齐已识别的 Purpose 与 roadmap “状态来源”说明
3. 后续按需扩展校验项（保持最小且高信噪比）

## Open Questions
- roadmap 状态一致性校验是否需要结构化输出（JSON）作为中间层？
