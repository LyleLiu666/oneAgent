# tool_call_failures 排查与提示词建议

本文基于 `tool_call_failures` 表中某天（示例：2026-01-22，Asia/Shanghai）记录，总结常见报错原因、哪些属于“模型不稳定/输出格式不稳”、哪些属于“工程约束导致的必然失败”，并给出提示词/工程侧的缓解建议。

## 一、典型失败类型（按出现频率聚类）

### 1) XML 工具协议：CDATA 未闭合导致的连锁失败（“格式不稳”）

现象：
- `bash` 报 `redirect target missing`
- `edit` 报 `invalid edit command ... unsupported command: <![CDATA[...`

根因：
- 模型输出了 `<command><![CDATA[...` 但漏了 `]]>`，导致解析后参数里残留 `<![CDATA[` 前缀；
- 对 `bash` 来说，命令开头的 `<` 会被当作重定向符号触发解析错误；
- 对 `edit` / `write_file` 来说，字段被污染后会导致参数校验失败或路径解析失败。

建议（提示词侧）：
- 只要使用 `<![CDATA[`，必须闭合为 `]]>`；不要把 `<![CDATA[` 当作普通文本写进字段值里。

工程侧缓解（已做）：
- XML 解析器对“未闭合 CDATA”做容错：会剥离 `<![CDATA[` 前缀，避免把 `<` 传给下游工具。

### 2) edit：命令格式不规范（“格式不稳/易偏航”）

现象：
- `edit requires filePath + oldcontent/newcontent`
- `use write_file for full file writes/creates`

建议（提示词侧）：
- **写文件/新建文件：优先用** `write_file`（`filePath + content`）。
- 超大文件：用 `write_file` 的 `append=true` 分段追加写入（建议每段 ≤3000 字；必要时先 `append=false` 写入空串以清空/创建）。
- **小范围替换：用** `edit`（`filePath + oldcontent/newcontent`）。
- `edit` 已禁用 `command`/`apply_edit` 脚本模式（历史遗留，容易被输出格式污染导致失败）。

工程侧缓解（已做）：
- 统一改为结构化参数：`edit` 仅处理 fuzzy replace；`write_file` 负责整文件写入。

### 3) bash 沙箱策略：命令/路径被禁止（“必然失败”）

现象（示例）：
- `command "sudo"/"apt-get"/"pip3"/"node"/"npm"/"find"/"touch" is not allowed`
- `heredoc redirection is not allowed`（禁止 `<<`）
- `path "... " is outside bash root`（绝对路径不在 `$BASH_ROOT_DIR` 下）

根因：
- bash 工具运行在受限沙箱中：禁止大量系统命令、安装器、解释器/编辑器、危险重定向等；
- 路径必须留在 `$BASH_ROOT_DIR` 内。

建议（提示词侧）：
- 避免使用被禁命令（尤其是 `sudo/apt-get/pip/node/npm/find/touch` 等）。
- 不要在 `bash` 里用重定向/ heredoc 写文件；写文件用 `write_file`，改文件用 `edit`。
- 路径使用相对路径（相对 `$BASH_ROOT_DIR`），不要写 `/tmp/...`、`/data/...`、`/dev/...`（除非明确允许）。

工程侧缓解（已做）：
- 允许将输出重定向到 `/dev/null`（常见且安全），避免不必要的失败。
- `bash` 工具描述/文档中补充了关键限制点，降低模型“盲撞”概率。

### 4) 上游 LLM/网络：流式读取失败（“服务不稳”）

现象：
- `stream read error: unexpected EOF`
- `stream read error: context deadline exceeded (Client.Timeout...)`

根因：
- 上游连接中断、服务端提前断流；
- HTTP 客户端超时 / 长输出导致读超时。

建议（提示词侧）：
- 对超长任务拆分：一次只做一个小目标、减少单次输出量。
- 工具结果尽量让模型“少回显、少粘贴大文件内容”，避免流式输出过长。

工程侧可选改进（未默认动）：
- 按 provider 配置更长的流式超时/重试策略（需要权衡卡死与成本）。

## 二、你可以怎么改提示词（最有效的几条）

- 强制 XML 工具调用时 CDATA 必须闭合：`<![CDATA[` 与 `]]>` 成对出现。
- 明确“写文件用 write_file、改文件用 edit；不要在 bash 里用 heredoc/echo 重定向写文件”。
- 明确 bash 沙箱禁用命令清单（至少列出最常见踩坑：`node/npm/find/touch/sudo/apt-get/pip`）。
- 强制写文件用 `write_file`（`filePath+content`），改文件用 `edit`（`filePath+oldcontent/newcontent`）；`edit` 不支持 `command`/`apply_edit`。
- 遇到工具报错时：先读错误信息并调整调用参数，不要重复同一个失败调用（减少 tool-call loop）。
