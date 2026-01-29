# Change: Add toolcalling reliability specs (from expert advice) + long-text protocol tests

## Why
`docs/oneAgent_toolcall_advice/docs/` 里是一组“工具调用顺滑度/成功率”的专家建议。当前这些建议以文档形式存在，但缺少：
- 统一的 OpenSpec 入口（便于在项目内被发现、讨论与拆解）
- 可回归的专项测试（避免基于直觉/口碑做协议决策，尤其是 XML vs JSON 在长文本入参场景）

同时，关于“XML 一定比 JSON 调用成功率低”的结论存在争议：在入参是长文本（例如 ~3000 字）且包含大量换行/引号/代码片段时，XML+CDATA 可能比 JSON string 更容易保持结构正确并减少转义负担。

## What Changes
- 新增 OpenSpec capability：`system-toolcalling-reliability`
  - 将专家建议文档整理为可发现的 spec 入口（允许在 spec 内引用原始文档以避免过度细化）
  - 明确“先测后改”的原则：在对协议做默认/废弃等决策前，必须有专项回归测试
- 增加后端专项测试（优先于任何协议改动）：
  - XML：`<content><![CDATA[...]]></content>` 长文本（>=3000 字）解析与 args 构建的无损性
  - JSON：长文本参数的 JSON 编码/解码无损性，以及与 XML 的编码体积对比（用于评估长文本场景的转义负担）

## Impact
- Affected specs: `system-toolcalling-reliability` (new)
- Affected code (tests only): `backend/internal/toolxml/*_test.go` and/or `backend/internal/handler/*_test.go`
- Reference docs (source of truth): `docs/oneAgent_toolcall_advice/docs/*.md`

