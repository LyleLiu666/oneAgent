# system-toolcalling-reliability Spec Delta

## ADDED Requirements

### Requirement: Structured outputs MUST NOT rely on plain-text JSON as the only channel (best-effort)
当系统需要模型产出结构化结果（例如“triage plan / decision / report payload”）时，系统必须 (MUST) 提供可靠的结构化返回通道（best-effort），并不得 (MUST NOT) 仅依赖“要求模型只输出纯文本 JSON”作为唯一方式。

系统必须 (MUST) 优先使用：
1) tool-call（原生 function calling；best-effort），或
2) 宽松 tags（XML-like；best-effort）
来承载结构化字段。

原因（best-effort）：
- 强制纯文本 JSON 往往会牺牲内容质量（模型把注意力放在格式合规上）
- 即使使用原生 tools，仍可能出现 arguments JSON string 轻微不合法导致 400 的情况；需要可恢复的 fallback

#### Scenario: Invalid tool arguments fall back to tags-based structured output (best-effort)
- **GIVEN** 系统优先使用原生 tools 获取结构化输出（best-effort）
- **AND** provider 返回 “invalid function arguments json” 类错误（best-effort）
- **WHEN** 系统进入 best-effort 恢复流程（best-effort）
- **THEN** 系统切换到宽松 tags 协议让模型重试输出结构化字段（best-effort）
- **AND** 若仍失败，系统返回明确错误与可操作建议（best-effort）

### Requirement: XML-like tags for structured output MUST NOT require CDATA (best-effort)
当系统使用 XML-like tags 承载结构化输出（best-effort）时，系统不得 (MUST NOT) 要求模型必须使用 `<![CDATA[...]]>` 才算“合规”；默认应允许 tag 内容为普通文本（best-effort）。

系统可以 (MAY) 保留对 CDATA 的兼容解析（best-effort），用于长文本或包含大量特殊字符的场景，但不应强迫模型每次都输出 CDATA（best-effort）。

#### Scenario: Plain tag content is accepted without CDATA (best-effort)
- **GIVEN** 模型输出结构化字段时使用 `<summary_message>...</summary_message>`（无 CDATA；best-effort）
- **WHEN** 系统解析该 tags payload（best-effort）
- **THEN** 系统成功提取字段并继续流程（best-effort）
