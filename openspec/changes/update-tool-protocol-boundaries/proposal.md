# Change: Define tool protocol boundaries (JSON default, XML fallback)

## Why
当前系统同时支持两套工具协议（JSON 原生 tool calling / XML `<tool_data>`），但两者的能力边界与可用工具集合并没有被显式固化：
- 用户/前端一旦切到 XML 协议，选择了 XML engine 未实现的工具（例如部分 `lsp.*` / `document.export`），会直接炸在运行期
- XML 协议对长文本与特殊字符更敏感，截断/标签破坏会导致解析失败，进而进入自愈循环

需要明确“默认走 JSON、XML 仅做 fallback”的策略，并把两套协议的可用工具与失败模式变成可测试的约束，避免把不必要的脆弱性暴露给用户。

参考资料（source-of-truth）：
- `docs/oneAgent_toolcall_advice/docs/03_XML工具调用_如果你必须保留它_怎么让它更稳.md`
- `docs/oneAgent_toolcall_advice/docs/08_Roadmap_分层次推进_由简单到长远.md`（L2-2）

## What Changes
- 固化协议选择策略（best-effort）：
  - provider 支持原生 tools → 默认使用 JSON tool calling
  - provider 不支持 → 自动 fallback 到 XML（或返回明确错误；best-effort）
- 固化 XML 协议的能力边界：
  - XML 协议下只暴露 XML engine 支持的工具集合；不支持的工具不得被挂载
  - 对“检测到 `<tool_data` 但缺失闭合标签”的截断，视为无效 step 并触发更强 retry 指令（best-effort）

## Impact
- Affected specs: `system-toolcalling-reliability`
- Affected code (expected): `backend/internal/handler/chat.go`, `backend/internal/toolxml/*`, `backend/internal/tool/*`
- Tests (expected): tool protocol selection tests + xml engine tests

