# Change: Introduce memorySdk As oneAgent's Optional Formal Memory Layer

## Why
`oneAgent` 现在已经有一套本地 `memorydb`，但它承担的是“秘书共享记忆”这类 append-only 本地流水，不等同于外部 `memorySdk` 的“正式记忆平台”。

如果直接把两者混成一个东西，会有两个明显问题：

1. 会把当前已经稳定的 SU/SW 共享记忆链路打断。
2. 会迫使 `oneAgent` 在宿主层继续复制 `memorySdk` 的 scope、pre-recall、remember/forget 规则，回到双维护。

用户的目标已经很明确：希望 `oneAgent` 直接引用外部 SDK，尽量不要在本项目里再复制一遍同类逻辑。因此这次应该先把边界放对，再把第一阶段真正接起来。

## What Changes
- 在 `backend` 中直接依赖外部 `memorySdk`，并在本地开发阶段通过 `replace` 指向 `/Users/liu_y/code/goProject/AgentAll/memorySdk`
- 明确区分两类 memory：
  - 现有 `memorydb`：继续承担秘书 SU/SW 的本地 append-only 共享记忆
  - 新增 `memorySdk`：作为“正式记忆层（formal memory）”，负责 recall / remember / forget / consolidation 这一类通用能力
- 第一期只接入最稳、最值得先落地的能力：
  - runtime 可选初始化 `memorySdk`
  - chat 在每轮发送前执行 pre-recall
  - pre-recall 结果只以 `TurnContext` 注入，不污染 stable prefix
- Docker 调试链路补齐外部 Postgres 接线，但不改变 oneAgent “本地默认无外部数据库也可启动”的基本行为

## Impact
- Affected specs:
  - `data-storage`
  - `system-prompt-assembly`
- Affected code:
  - `/Users/liu_y/code/goProject/oneAgent/backend/go.mod`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/config/config.go`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/runtime/runtime.go`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/handler/chat.go`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/formalmemory/*`
  - `/Users/liu_y/code/goProject/oneAgent/docker-compose.yml`
