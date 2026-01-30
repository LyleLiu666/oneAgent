# Kode CLI — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:26Z
- Repo: https://github.com/shareAI-lab/Kode-cli
- Query: `repo:shareAI-lab/Kode-cli is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [哈哈哈，大佬下周回国贾跃亭吗](https://github.com/shareAI-lab/Kode-cli/issues/9) (#9)
- Reactions: **7** | Comments: **4** | Updated: 2025-09-06T09:39:04Z

期待大佬的结果

---

### [Waiting for release?](https://github.com/shareAI-lab/Kode-cli/issues/2) (#2)
- Reactions: **5** | Comments: **0** | Updated: 2025-07-18T00:43:30Z

It said it will be released tonight 5 days ago!?

---

### [Feature request: 支持 OpenAI Responses API previous_response_id + 分支会话（rewind/fork）](https://github.com/shareAI-lab/Kode-cli/issues/139) (#139)
- Reactions: **2** | Comments: **1** | Updated: 2025-11-27T20:19:02Z


背景 / Background

回退到第 N 轮，从那里开始一个新的思路，相当于在对话时间线上「开分支」。

在 OpenAI 的新 Responses API 中，引入了 previous_response_id 机制，可以让服务端记住整条对话链（包括输入和输出），后续请求只需要发送「新一轮的用户输入 + previous_response_id」即可继续会话，甚至可以从同一个 previous_response_id 开出多条不同分支。

需求 / What I’d like Kode to support

希望 Kode 在对接 OpenAI（尤其是 GPT-5.x / Codex 系列 / Responses API-only 模型）时：
	1.	优先使用 Responses API + previous_response_id 来维持上下文，而不是每次都在本地手动拼接完整历史；
	2.	在 CLI 的交互层面上：
	•	保持现在「可以回退到任意历史轮次」的 UX；
	•	当用户从某一轮回退并继续对话时，内部使用这一轮对应的 response.id 作为新的 previous_response_id，在服务端开出一个新的会话分支；
	3.	在用户显式编辑历史内容 / 压缩上下文 / 超出保留期时，能够自动退回到「本地构造 input[]、不传 previous_res…

---

### [请问有计划支持ACP吗？](https://github.com/shareAI-lab/Kode-cli/issues/99) (#99)
- Reactions: **2** | Comments: **0** | Updated: 2025-09-22T12:24:02Z

我在使用Zed，支持外部的命令行智能体，CC和Gemini都可以，但是貌似Kode不支持[Agent Client Protocol (ACP)](https://agentclientprotocol.com/)，请问有计划做不？

---

### [Excited about the release!](https://github.com/shareAI-lab/Kode-cli/issues/7) (#7)
- Reactions: **1** | Comments: **0** | Updated: 2025-07-22T05:29:12Z



---

### [请问大佬，目前咱们是和官方Claude code 保持同步更新的吗？](https://github.com/shareAI-lab/Kode-cli/issues/130) (#130)
- Reactions: **1** | Comments: **2** | Updated: 2025-12-23T02:07:00Z

请问大佬，目前咱们是和官方Claude code 保持同步更新的吗？

---

### [Kode cli upgrade](https://github.com/shareAI-lab/Kode-cli/issues/140) (#140)
- Reactions: **0** | Comments: **0** | Updated: 2025-11-30T11:42:42Z



---

### [[BUG] Local develop mode start failed](https://github.com/shareAI-lab/Kode-cli/issues/141) (#141)
- Reactions: **0** | Comments: **3** | Updated: 2025-12-02T12:18:44Z

For the current main branch, use bun install && bun run build && bun run dev:
```
✅ yoga.wasm copied to dist
✅ cli.js made executable
✅ Build completed for cross-platform compatibility!
📋 Generated files:
  - dist/ (ESM modules)
  - dist/index.js (main entrypoint)
  - dist/entrypoints/cli.js (CLI main)
  - cli.js (cross-platform wrapper)
  - .npmrc (npm configuration)
➜  kode-cli git:(main) bun run dev
$ bun run ./src/entrypoints/cli.tsx --verbose
2078 |     .m(function (Command, cs, config, o) {
2079 |     return [middlewareEndpoint.getEndpointPlugin(config, Command.getEndpointParameterInstru…

---

### [前排支持](https://github.com/shareAI-lab/Kode-cli/issues/6) (#6)
- Reactions: **0** | Comments: **0** | Updated: 2025-07-21T12:25:02Z



---

### [期待复现，很想学习下源码](https://github.com/shareAI-lab/Kode-cli/issues/8) (#8)
- Reactions: **0** | Comments: **0** | Updated: 2025-07-24T01:19:00Z



---

