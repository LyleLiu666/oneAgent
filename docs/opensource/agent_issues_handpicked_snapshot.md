# Agent/Coding-Agent Competitor Issues Snapshot (Handpicked)

> Generated: 2026-01-30  

> Focus repos: openai/codex, anomalyco/opencode, QwenLM/qwen-code, shareAI-lab/Kode-cli, openclaw/openclaw, anthropics/claude-code


## openai/codex (weight: S)

工业级踩坑最密集：Windows/权限沙箱/patch/edit工具/网络与runner交互/CLI-TUI 体验

- #9920 **Codex randomly Freezes in 0.91.0 (Windows)**  
  https://github.com/openai/codex/issues/9920  
  Signals: CLI卡死/无响应 / 后台任务仍在跑 / Windows

- #9914 **apply_patch mixed CRLF causes fallback to slow python**  
  https://github.com/openai/codex/issues/9914  
  Signals: patch引擎不稳 / CRLF/LF 混用 / token/性能暴涨 / Windows

- #9906 **Async SQLite hang in Codex runtime (aiosqlite + SQLAlchemy)**  
  https://github.com/openai/codex/issues/9906  
  Signals: runner/沙箱环境交互 / async event-loop/线程交互 / network toggle 影响行为

- #7344 **Keeps prompting approvals + excessive `cd` prepended commands**  
  https://github.com/openai/codex/issues/7344  
  Signals: 权限弹窗风暴 / sandbox scope 误判 / workdir vs cd 语义错误

- #9872 **stream disconnected before completion: Transport error**  
  https://github.com/openai/codex/issues/9872  
  Signals: 流式/网络不稳定 / 长任务中断恢复

- #9814 **TUI drops MCP image output when text precedes image**  
  https://github.com/openai/codex/issues/9814  
  Signals: MCP输出渲染 / 多模态输出顺序/协议

- #9912 **Configurable Maximum Agent Recursion Depth**  
  https://github.com/openai/codex/issues/9912  
  Signals: 递归/子任务深度控制 / 防止失控循环

- #9908 **CLI wrapper exits 0/hangs after child terminates by signal**  
  https://github.com/openai/codex/issues/9908  
  Signals: 进程管理 / signal/退出码 / wrapper健壮性


## anthropics/claude-code (weight: S)

同量级：沙箱/工具链/子代理编排/TUI/Windows安装与Bun生态

- #21942 **Sandbox: Operation not permitted on /tmp/... due to macOS com.apple.provenance xattr**  
  https://github.com/anthropics/claude-code/issues/21942  
  Signals: macOS xattr / sandbox 文件系统权限误报

- #21941 **Subagents complete multiple times causing duplicate processing**  
  https://github.com/anthropics/claude-code/issues/21941  
  Signals: 子代理去重/幂等 / 重复完成事件

- #21939 **Shell snapshot doesn't capture zoxide dependency functions, breaking cd alias**  
  https://github.com/anthropics/claude-code/issues/21939  
  Signals: shell snapshot 不完整 / 依赖/函数/别名丢失

- #21930 **Add hook event for plan mode transitions (ExitPlanMode/PlanApproved)**  
  https://github.com/anthropics/claude-code/issues/21930  
  Signals: Plan mode 生命周期hook / 可扩展性/插件化

- #21937 **Terminal rendering glitches, infinite scroll, background task deadlock, missing clipboard image paste**  
  https://github.com/anthropics/claude-code/issues/21937  
  Signals: TUI渲染/性能 / 死锁 / 剪贴板多模态输入

- #21931 **ReferenceError: Bun is not defined (Windows npm install) causes crashes**  
  https://github.com/anthropics/claude-code/issues/21931  
  Signals: Windows安装/打包 / 运行时依赖


## anomalyco/opencode (weight: A)

用户规模大：Windows支持、TUI keybind/交互磨损高频

- #631 **Windows Support (super issue)**  
  https://github.com/anomalyco/opencode/issues/631  
  Signals: Windows兼容性系统性问题

- #4997 **Keybinds (TUI navigation, multiline input, remap, Ctrl-C)**  
  https://github.com/anomalyco/opencode/issues/4997  
  Signals: TUI键位一致性 / 跨平台快捷键 / 可配置/可重映射


## QwenLM/qwen-code (weight: B)

快速增长：技能/扩展、文件操作工具、CLI界面与多语言

- #1669 **IDEA acp 文件修改逻辑有问题**  
  https://github.com/QwenLM/qwen-code/issues/1669  
  Signals: 文件修改逻辑/patch策略问题 / IDEA集成

- #1666 **YAML Formatter Error for Skills Bundled with Extensions**  
  https://github.com/QwenLM/qwen-code/issues/1666  
  Signals: 扩展/技能打包 / formatter/校验链路

- #1619 **read_many_files 工具并未实际返回文件内容**  
  https://github.com/QwenLM/qwen-code/issues/1619  
  Signals: 工具输出不可信 / 协议/返回值缺失

- #1646 **Supports multi-language switching capabilities**  
  https://github.com/QwenLM/qwen-code/issues/1646  
  Signals: 多语言UI/输入


## shareAI-lab/Kode-cli (weight: B)

MCP/代理模式/权限模型与网络代理问题集中

- #166 **[BUG] MCP返回的文本处理报错**  
  https://github.com/shareAI-lab/Kode-cli/issues/166  
  Signals: MCP文本处理/协议健壮性

- #164 **[BUG] Synchronous Unix command loading blocks UI during completion**  
  https://github.com/shareAI-lab/Kode-cli/issues/164  
  Signals: UI阻塞 / 同步调用导致卡顿

- #160 **windows下使用gitBash貌似不走代理**  
  https://github.com/shareAI-lab/Kode-cli/issues/160  
  Signals: 代理/网络 / Windows终端差异

- #159 **安全模式/YOLO 模式权限切换诉求**  
  https://github.com/shareAI-lab/Kode-cli/issues/159  
  Signals: 权限模型可配置 / 用户心智

- #156 **能支持codex auth登入嗎?**  
  https://github.com/shareAI-lab/Kode-cli/issues/156  
  Signals: 认证/登录 / 多提供商账号体系


## openclaw/openclaw (weight: A)

最近爆火：多平台Bot/多provider工具调用一致性、子代理可靠性问题明显

- #4600 **Terminated assistant mid-toolCall causes infinite 'unexpected tool_use_id' loop**  
  https://github.com/openclaw/openclaw/issues/4600  
  Signals: tool_use/tool_result 配对 / 中断恢复 / transcript repair

- #4599 **Sub-agent runs silently die on TypeError: fetch failed — no retry, no cleanup**  
  https://github.com/openclaw/openclaw/issues/4599  
  Signals: 子代理失败处理 / 重试/清理/可观测

- #4607 **imageModel fallback not used when primary model falls back to non-vision provider**  
  https://github.com/openclaw/openclaw/issues/4607  
  Signals: 能力协商 / 多模型/多provider fallback

- #4601 **Add PDF Support to Read Tool**  
  https://github.com/openclaw/openclaw/issues/4601  
  Signals: 工业落地：PDF/文档输入 / read tool 能力

- #4611 **Add systemPrompt support for WhatsApp groups**  
  https://github.com/openclaw/openclaw/issues/4611  
  Signals: 渠道/群组差异化系统提示 / 多tenant/多场景

- #4596 **Failed to start CLI: missing native module '@mariozechner/clipboard-linux-arm-gnueabihf'**  
  https://github.com/openclaw/openclaw/issues/4596  
  Signals: 跨平台原生依赖 / 打包/分发


---

## Badcase Taxonomy (agent 落地高频坑位)

1) **权限/沙箱边界误判 → 弹窗风暴 / 卡工作流**
   - 典型表现：反复请求授权、把合法操作当“越狱”；或不同 OS 上权限语义不一致。
   - 设计要点：
     - 把“workspace roots / allowed paths”做成显式、可审计的数据结构；工具接口强制传 `workdir`（避免在 command string 里 `cd && ...`）。
     - 审批缓存要“按 scope + 动作类型 + 会话”分层；给用户明确的“安全模式/YOLO 模式”开关与可视化范围。

2) **tool_use / tool_result 协议与转录修复不健壮 → 中断后无限失败循环**
   - 典型表现：中断/超时后 session 被写入“孤儿 tool_result”，之后每次请求都被 provider 拒绝。
   - 设计要点：
     - Transcript 层做成状态机：tool_use 必须在 tool_result 前出现；中断时要么补齐 tool_use，要么丢弃对应 tool_result（幂等修复）。
     - “恢复”逻辑要可重放：把每个 tool call 的生命周期（pending/success/error/aborted）持久化。

3) **跨平台（Windows/macOS/Linux）差异 → 80% 工程时间都在修边角**
   - 典型表现：CRLF、shell/编码、代理、Bun/Node 原生依赖、clipboard 等。
   - 设计要点：
     - 明确 OS 兼容层：EOL、路径、权限、终端能力、代理/证书、原生模块 fallback。
     - 安装/升级过程加自检（依赖版本、权限、网络、证书），失败给出可执行的修复建议。

4) **Runner/执行环境与 async/线程/网络开关交互 → “在我本机能跑，进 agent 就挂/卡”**
   - 典型表现：async DB hang、子进程信号处理异常、后台任务死锁。
   - 设计要点：
     - 所有 exec 都要有 watchdog（超时、SIGTERM/SIGKILL 策略、退出码/信号映射）。
     - 环境隔离要可观测：把“网络开关/沙箱策略/资源限制”写入每次执行的 trace。

5) **TUI/IDE 体验磨损（keybind、滚动、渲染、阻塞） → “能用但很难长期用”**
   - 设计要点：
     - Keybind 统一层 + 可重映射；不要把 Ctrl-C 这类“系统级”键位当成业务键。
     - UI 线程绝不做同步 IO；所有长任务异步 + 可取消；渲染性能要有基准测试。

6) **子代理编排（重复完成、静默失败） → 结果不可信 / 资源泄漏**
   - 设计要点：
     - 子代理输出要“幂等 + 去重”：按 task_id / run_id 归并。
     - 子代理失败必须显式上报（错误、重试次数、清理动作），默认要有 retry/backoff。
