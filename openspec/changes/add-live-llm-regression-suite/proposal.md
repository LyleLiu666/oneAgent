# Change: Add live LLM regression suite (real provider + JSON vs XML)

## Why
当前我们需要优先把“核心 agent 回路”做得丝滑：成功率高、速度快、损耗小。

Mock/单元测试不足以覆盖真实网络、真实 provider、真实长文本入参等场景；因此需要一套**可重复、可度量、可产出报告**的“真实跑一遍”回归套件，直接读取本机 `.oneagent/settings.db` 中已配置的 LLM provider/model，用真实调用来评估稳定性，并为后续体验优化提供证据。

## What Changes
- 新增一个 **opt-in** 的 Go E2E 回归测试套件：
  - 自动读取并复制当前 repo 的 `.oneagent/settings.db` 到临时 home（避免污染真实数据）
  - 启动本地 server（AuthMode=none），通过 `/api/chat` 真实跑多组用例
  - 覆盖 `tool_protocol=json` 与 `tool_protocol=xml` 两条链路，并包含 **3000+ 字长文本**入参的专项对比用例
- 运行结束产出测试报告（JSON + Markdown），写入 `.oneagent/tmp/`（默认不入 git）
- 报告中严格避免泄漏敏感信息（如 API key）

## Impact
- Affected specs: `project-tooling`
- Affected code: `backend/internal/server`（新增 live 回归测试与报告生成）
- Operational: 该套件为 opt-in；默认 `go test ./...` 不会触发真实调用（避免 CI 成本与不稳定）

