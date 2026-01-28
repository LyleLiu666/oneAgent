# Design: Defense-in-depth security

## Goals
- 默认安全（secure-by-default）：开箱即可在“最小暴露、最小权限”下工作
- 纵深防御：任何单点失败不导致系统失控（尤其是 prompt injection / 越权工具调用）
- 可解释、可审计：重要决策（allow/deny/风险提示）能追溯到规则与证据
- 可控放开：管理员/用户能显式提升能力（但要有清晰风险提示与审计线索）

## Non-goals（v1）
- 追求“绝对隔离/绝对安全”（跨平台与可用性成本过高）
- 一次性引入完整企业级 SSO/RBAC（先以 token+principal+policy 解决 80%）

## Pragmatic threat model
- 未授权访问：服务被暴露在 LAN/公网、token 泄露、浏览器/恶意站点诱导访问
- Prompt injection：外部内容（web/邮件/粘贴）试图把“指令”伪装成“数据”
- 数据外传：通过工具读取 workspace 外文件、通过命令/网络向外发送数据
- 破坏性操作：误执行 `rm -rf`、误改关键文件、越界写入
- 模型差异：不同 provider/model 的对齐能力差异导致越权更容易发生

## Layer mapping（from outside to inside）
| Layer | oneAgent 方向 | Spec anchor |
| --- | --- | --- |
| 1. 入口控制 | 配对/发 token、token allowlist、principal/门控、可回收 | `auth-mode` |
| 2. 身份验证 | Gateway Bearer token（可扩展 password/SSO） | `auth-mode` |
| 3. 外部内容包装 | 非可信内容边界标记 + UI 警告 | `system-prompt-assembly` |
| 4. 可疑模式检测 | 注入/外传/破坏性模式 regex 检测 + 记录/告警 | `system-prompt-assembly` |
| 5. 模型选择 | 模型“安全档位”+ 默认推荐 + 高风险工具挂载门槛 | `llm-provider-management` |
| 6. 工具权限 | per-principal allow/deny + constraints + 审计 | `system-tool-permissions` |
| 7. 沙箱隔离 | 命令类工具可选 docker sandbox + FS 隔离 | `system-tool-permissions` |
| 8. 网络隔离 | loopback/Tailscale + 暴露风险显式提示 | `local-runtime` |

## Implementation sketch

### 1) Entry control（token 发放/门控）
- 以 `principal_id` 作为权限与审计的主键（现有基础已具备）。
- 将“管理面”能力（token 管理、tool policy 管理）定义为 admin-only。
- 配对/发 token 的 UX（二维码/短码/一次性链接）作为 v1.1：避免把 admin token 当“共享密码”传播。

### 2) Identity verification（网关认证）
- `AUTH_MODE=token` 继续作为默认；`AUTH_MODE=none` 明确标红风险并仅用于开发/离线极简。
- 将风险提示从“文档说明”提升到“产品内可见”（登录页/启动 banner/doctor 输出）。

### 3) External content packaging（软隔离，抗注入）
- 定义“非可信内容”来源：web 搜索/HTTP、剪贴板、workspace 外文件（当策略允许读取时）、任何工具返回的长文本。
- 系统注入到模型前统一包装：
  - 边界标记（`BEGIN_UNTRUSTED_CONTENT`/`END_UNTRUSTED_CONTENT`）
  - 来源元数据（tool id、URL/路径、抓取时间）
- Stable prefix 增加强约束：将边界内内容视为“数据”，不得将其当作指令执行；遇到越权/冲突时向用户确认。

### 4) Suspicious pattern detection（监控与可追溯）
- 维护一组可迭代的模式（regex/heuristics），覆盖：
  - “ignore previous/system prompt/工具越权”等注入特征
  - token/密钥格式、外传意图、破坏性命令意图
- 触发后：
  - 写入 trace/receipt（命中规则、来源、摘要）
  - UI 对该段内容打“可疑”标签（best-effort）
  - 对高风险动作触发二次确认或策略性阻断（由 policy 决定）

### 5) Model selection（软隔离，默认推荐）
- 在 model 配置中引入 `safety_tier`（例如 `high|standard`）。
- 高风险工具的默认挂载需要 `safety_tier=high`（除非管理员策略显式放开）。
- UI 在模型列表中显式展示“推荐安全模型”，并对低档位给出提示。

### 6) Tool permissions（硬隔离，最小权限）
- per-principal allow/deny 作为最外层 hard gate（现有能力）。
- 对命令工具维持 allowlist/profile 策略，并要求决策可解释/可审计。

### 7) Sandbox isolation（硬隔离，命令执行）
- 为 `bash/run_command` 提供可选 `docker` sandbox：
  - workspace 挂载到容器内固定路径（默认只读；写入需显式放开）
  - 默认 `--network none`（由 policy 决定是否放开网络）
  - 资源限制（时间/CPU/内存）避免 runaway
- 当 policy 要求 sandbox 但环境不可用：返回可操作错误（安装/启用建议或策略降级方案）。

### 8) Network isolation（硬隔离，最小暴露）
需要明确默认取舍（可访问性 vs 默认安全）：
- Option A（更安全，可能 breaking）：默认 bind loopback；显式 `--bind 0.0.0.0` 才暴露到 LAN
- Option B（兼容现状）：保留 local profile 默认 LAN，但提供 `--secure` 或 `--bind 127.0.0.1` 的一键安全模式，并对非 loopback 强提示风险

跨设备访问优先推荐：loopback + Tailscale（可审计、可撤销、避免“误暴露到公网”）。

## Open questions
- 默认 bind 策略选 Option A 还是 B？（这会影响“开箱即用”的可访问性）
- 配对发 token 的目标 UX 是什么？（二维码/短码/一次性链接/手动复制？）
- Docker sandbox 是否可接受作为 optional 依赖？Windows 场景如何兜底？
