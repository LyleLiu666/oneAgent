# system-toolcalling-reliability Specification

## Purpose
TBD - created by archiving change update-tool-protocol-boundaries. Update Purpose after archive.
## Requirements
### Requirement: The system MUST define tool protocol boundaries and selection
系统必须 (MUST) 明确并固化两套工具协议（JSON 原生 tool calling / XML `<tool_data>`）的能力边界，并提供可预测的协议选择策略（best-effort）：
- 当 provider 支持原生 tools 时，系统默认使用 JSON tool calling
- 当 provider 不支持原生 tools 但 tools 被请求时，系统自动 fallback 到 XML（或返回明确错误；best-effort）

#### Scenario: Provider without tools triggers XML fallback
- **GIVEN** 用户请求启用 tools 且当前 provider 不支持原生 tools
- **WHEN** 系统准备进入工具 loop
- **THEN** 系统使用 XML tool protocol 继续推进（best-effort）
- **OR** 返回清晰错误，提示切换 provider/关闭 tools（best-effort）

### Requirement: XML protocol MUST only expose supported tools
当 tool protocol 为 XML 时，系统必须 (MUST) 只向模型暴露 XML engine 已实现参数构建与执行的工具；任何未实现的工具不得被挂载或不得被接受执行（fail-closed）。

#### Scenario: Unsupported tools are not available under XML protocol
- **GIVEN** 当前 tool protocol 为 XML
- **WHEN** 系统构建本轮可用 tools 列表
- **THEN** `lsp.*` / `document.export` 等未被 XML engine 支持的工具不会被挂载（best-effort）

### Requirement: XML protocol MUST detect truncated tool_data and request retry
当模型输出中出现 `<tool_data` 但未闭合 `</tool_data>`（截断/格式破坏）时，系统必须 (MUST) 将该 step 视为无效工具调用，并触发一次更强约束的重试指令（best-effort）。

#### Scenario: Missing closing tag triggers a stronger retry instruction
- **GIVEN** 模型输出包含 `<tool_data` 但缺失 `</tool_data>`
- **WHEN** XML tool parser 尝试解析
- **THEN** 系统判定为截断/无效结构并触发 retry（best-effort）

### Requirement: The system MUST normalize/repair tool arguments (best-effort)
当 provider/模型输出的 tool call arguments 不是严格合法 JSON 时，系统必须 (MUST) 做 best-effort 的 normalization/repair，以尽量将其修复为可 `json.Unmarshal` 的 JSON 字符串，减少无意义的 tool loop 浪费。

修复应至少覆盖（best-effort）：
- ```json 围栏（去除围栏后再校验）
- 前后夹杂解释文本（抽取最外层 JSON object/array）
- 整体为 JSON string 的情况（先 unquote 再解析）

#### Scenario: Code-fenced JSON arguments are repaired
- **GIVEN** tool arguments 形如 ```json { ... } ```
- **WHEN** 系统进行 arguments normalization
- **THEN** 得到合法 JSON 字符串（best-effort）

#### Scenario: Arguments with prefix/suffix text are repaired
- **GIVEN** tool arguments 形如 `Here is JSON: { ... } thanks`
- **WHEN** 系统进行 arguments normalization
- **THEN** 抽取并返回合法 JSON（best-effort）

#### Scenario: Quoted JSON arguments are repaired
- **GIVEN** tool arguments 是一个 JSON string（内容为 `{...}` 或 `[...]`）
- **WHEN** 系统进行 arguments normalization
- **THEN** 返回被解码后的合法 JSON（best-effort）

### Requirement: Tool-enabled chat MUST default to temperature=0 (unless explicitly set)
当本轮启用了工具调用（例如 `tool_ids` 非空且使用原生 tools 协议）时，系统必须 (MUST) 使用低温度以提升结构化输出稳定性。若未显式配置温度，系统必须 (MUST) 默认将 `temperature` 设为 `0`（best-effort；若 provider 不支持 0，可采用最接近的等价低温度）。

#### Scenario: Tool-enabled request sends temperature=0
- **GIVEN** 本轮启用了 tools（tool_ids 非空）
- **WHEN** 系统向 provider 发起一次 LLM 调用
- **THEN** 请求体包含 `temperature=0`（best-effort）

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

