## ADDED Requirements

### Requirement: Toolcalling reliability advice MUST be discoverable from OpenSpec
本项目必须 (MUST) 在 OpenSpec 中提供一个统一入口，整理并引用 `docs/oneAgent_toolcall_advice/docs/` 里的专家建议，避免建议散落导致的“知道但找不到 / 找到但无法拆解”的情况。

允许在本 spec 中引用原始文档作为细节来源（best-effort），但本 spec 必须给出可执行的落地抓手（例如回归测试与指标）。

参考资料（source-of-truth）：
- `docs/oneAgent_toolcall_advice/docs/00_总览_为什么你的工具调用不够顺滑.md`
- `docs/oneAgent_toolcall_advice/docs/01_根因清单_对比ClaudeCode与Codex差距.md`
- `docs/oneAgent_toolcall_advice/docs/02_JSON工具调用_可靠性提升方案_可落地改动.md`
- `docs/oneAgent_toolcall_advice/docs/03_XML工具调用_如果你必须保留它_怎么让它更稳.md`
- `docs/oneAgent_toolcall_advice/docs/04_OpenAI_Responses_API_补齐工具调用_对齐Codex关键.md`
- `docs/oneAgent_toolcall_advice/docs/05_命令沙箱与权限_为什么这会拉低成功率_以及怎么改.md`
- `docs/oneAgent_toolcall_advice/docs/06_测试与指标_把成功率变成可回归的数字.md`
- `docs/oneAgent_toolcall_advice/docs/07_建议补齐的工具手册清单_模板.md`
- `docs/oneAgent_toolcall_advice/docs/08_Roadmap_分层次推进_由简单到长远.md`

#### Scenario: Developer can find the canonical advice set
- **GIVEN** 开发者需要改进工具调用顺滑度/成功率
- **WHEN** 开发者阅读本 spec
- **THEN** 能找到上述文档路径并按主题深入（best-effort）

### Requirement: The project MUST provide long-text protocol regression tests (XML vs JSON)
在对工具调用协议（例如默认协议、是否保留 XML、或对 XML 做编码约束）做任何行为变更前，项目必须 (MUST) 先提供可回归的专项测试，覆盖**长文本入参**（例如约 3000 字）场景。

该专项测试的目标是验证“协议链路的工程鲁棒性与编码负担”，而非直接对某个模型/provider 的真实成功率下结论（best-effort）。

#### Scenario: XML CDATA preserves long content losslessly
- **GIVEN** 一个 XML `<tool_data>` tool call，其中 `<content><![CDATA[...]]></content>` 包含 >=3000 字且含换行/引号/类似标签的片段（best-effort）
- **WHEN** 系统解析该 `<tool_data>` 并将字段构造成工具参数（JSON args）
- **THEN** `content` 字段的值在解析/构造过程中保持无损（lossless）

#### Scenario: JSON tool arguments preserve long content when properly encoded
- **GIVEN** 一个 JSON tool call arguments payload，其中包含 >=3000 字的 `content` 字段（best-effort）
- **WHEN** 系统对该 arguments 进行 JSON 解码并交由工具处理
- **THEN** `content` 字段保持无损（lossless）
- **AND** 对于明显不合法（例如未正确转义）的 arguments，系统应返回可理解的失败（best-effort）

#### Scenario: Size comparison is available for code-like long text
- **GIVEN** 一个包含大量换行/引号/反斜杠的 code-like 长文本 payload（>=3000 字）
- **WHEN** 比较 XML+CDATA 与 JSON string 在“参数载荷”层面的编码体积（best-effort）
- **THEN** 项目保留对比结果（例如通过测试日志/基准测试输出）以支持协议取舍讨论（best-effort）

