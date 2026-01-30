# Oh My OpenCode — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:23Z
- Repo: https://github.com/code-yeongyu/oh-my-opencode
- Query: `repo:code-yeongyu/oh-my-opencode is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [websearch_exa MCP timeout after opencode reinstall](https://github.com/code-yeongyu/oh-my-opencode/issues/493) (#493)
- Reactions: **23** | Comments: **16** | Updated: 2026-01-27T00:58:01Z
- Labels: `bug`

## Description

After reinstalling opencode and oh-my-opencode, the `websearch_exa` MCP server fails to connect with a timeout error.

## Important Context

**Before the reinstall, `websearch_exa` was working without any API key configuration.** After reinstalling both opencode and oh-my-opencode, the MCP now times out.

## Error

```
operation timeout after 5000ms
```

## Environment

- OpenCode version: 1.1.1
- oh-my-opencode: latest
- OS: Linux
- Node: v24.12.0

## Logs

From OpenCode logs:
```
INFO  service=mcp key=websearch_exa type=remote found
```

The MCP is being detected as `type=rem…

---

### [[Feature]: Multi-provider model fallback for agents](https://github.com/code-yeongyu/oh-my-opencode/issues/1114) (#1114)
- Reactions: **20** | Comments: **2** | Updated: 2026-01-27T10:04:15Z
- Labels: `enhancement`

### Prerequisites

- [x] I have searched existing issues and discussions to avoid duplicates
- [x] This feature request is specific to oh-my-opencode (not OpenCode core)
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Problem Description

As a user with access to multiple AI providers (e.g., Google Antigravity free tier + personal Anthropic API), I currently have no way to configure automatic failover between providers.
When my free quota from Antigravity is exhausted, I must manually edit the configuration to switch to my personal API key. Thi…

---

### [[Bug]:  please stop overengineering shit](https://github.com/code-yeongyu/oh-my-opencode/issues/1189) (#1189)
- Reactions: **18** | Comments: **0** | Updated: 2026-01-28T01:56:28Z
- Labels: `bug`

### Prerequisites

- [x] I have searched existing issues to avoid duplicates
- [x] I am using the latest version of oh-my-opencode
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Bug Description

<img width="951" height="224" alt="Image" src="https://github.com/user-attachments/assets/e13bcd56-3cfe-4208-8250-e9c499d3b3a8" />

### Steps to Reproduce

open pc after a few days
the thing updates
configs break, spend hours fixing config till it is perfect as per the "new improved" 3.x

agent forgets who he is

auth plugin breaks, spamming 50x auth.t…

---

### [[Feature]: Support multiple fallback models for a single Agent (Model Failover Strategy)](https://github.com/code-yeongyu/oh-my-opencode/issues/703) (#703)
- Reactions: **17** | Comments: **1** | Updated: 2026-01-18T10:54:45Z
- Labels: `enhancement`

### Prerequisites

- [x] I have searched existing issues and discussions to avoid duplicates
- [x] This feature request is specific to oh-my-opencode (not OpenCode core)
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Problem Description

### Problem Description
Currently, each Agent in `oh-my-opencode` is bound to a single specific model configuration.

I often encounter situations where the primary high-performance model (e.g., Claude-3.5-Opus) hits rate limits (429) or runs out of quota/credits during a task. When this happens, the Agent sto…

---

### [[Feature] Add Mode Task Planning with beads](https://github.com/code-yeongyu/oh-my-opencode/issues/359) (#359)
- Reactions: **11** | Comments: **1** | Updated: 2026-01-27T01:05:40Z
- Labels: `enhancement`

Have beads integrated so I can use opus for this to architect and then glm 4.7 to have these executed of each task https://github.com/steveyegge/beads

---

### [Share your opinion](https://github.com/code-yeongyu/oh-my-opencode/issues/37) (#37)
- Reactions: **11** | Comments: **111** | Updated: 2026-01-29T08:56:51Z
- Labels: `discussions`

When I first envisioned Oh My OpenCode and decided to release it, here was my thought process:

There aren't a decent general coding agent harness. Such a setup needs to:
- Be free of cost concerns.
- Require no tedious configuration.
- Just work, and work well.
- Allow existing power users to switch over immediately.
- Solve "general" problems (depending on how you define them), such as preventing excessive comments or overly defensive code.
- Remain easily extensible and customizable.
- *And since contributing directly to a massive project like OpenCode takes time, let's approach this as a p…

---

### [[Feature]: Add `/omo-spec` — A Unified Spec‑Driven Workflow for Oh‑My‑OpenCode](https://github.com/code-yeongyu/oh-my-opencode/issues/854) (#854)
- Reactions: **11** | Comments: **3** | Updated: 2026-01-30T14:27:45Z
- Labels: `enhancement`

### Prerequisites

- [x] I have searched existing issues and discussions to avoid duplicates
- [x] This feature request is specific to oh-my-opencode (not OpenCode core)
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Problem Description

Oh‑My‑OpenCode (OMO) provides an extremely powerful multi‑agent execution environment, including:

- The **Sisyphus** orchestrator with multi‑model reasoning  
- Specialist agents like **oracle**, **librarian**, and **explore**  
- 20+ workflow hooks (todo‑continuation‑enforcer, session-recovery, context-windo…

---

### [[Feature]: Disable oh-my-opencode via cli command](https://github.com/code-yeongyu/oh-my-opencode/issues/673) (#673)
- Reactions: **10** | Comments: **4** | Updated: 2026-01-30T02:58:31Z
- Labels: `enhancement`

### Prerequisites

- [x] I have searched existing issues and discussions to avoid duplicates
- [x] This feature request is specific to oh-my-opencode (not OpenCode core)
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Problem Description

Sometimes I would like to disable oh-my-opencode plugin temporarly and not delete it manually from the opencode json config.

### Proposed Solution

/settings -> disable plugin

### Alternatives Considered

_No response_

### Doctor Output (Optional)

```shell

```

### Additional Context

_No response_

### F…

---

### [[Bug]: bun crashed (oh-my-opencode-windows-x64)](https://github.com/code-yeongyu/oh-my-opencode/issues/1175) (#1175)
- Reactions: **10** | Comments: **11** | Updated: 2026-01-28T10:02:46Z
- Labels: `bug`

### Prerequisites

- [x] I have searched existing issues to avoid duplicates
- [x] I am using the latest version of oh-my-opencode
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Bug Description

```sh
bun i -g oh-my-opencode-windows-x64
bun add v1.3.6 (d530ed99)

installed oh-my-opencode-windows-x64@3.1.0 with binaries:
 - oh-my-opencode

1 package installed [1.87s]

bunx oh-my-opencode doctor
============================================================
Bun v1.3.6 (d530ed99) Windows x64
Windows v.win11_dt
CPU: sse42 avx avx2
Args: "C:\Users\Ad…

---

### [[Bug]: Installation failed](https://github.com/code-yeongyu/oh-my-opencode/issues/872) (#872)
- Reactions: **9** | Comments: **2** | Updated: 2026-01-27T02:52:54Z
- Labels: `bug`

### Prerequisites

- [x] I have searched existing issues to avoid duplicates
- [x] I am using the latest version of oh-my-opencode
- [x] I have read the [documentation](https://github.com/code-yeongyu/oh-my-opencode#readme)

### Bug Description

Wanting to try this out I executed the following command:

    npx oh-my-opencode install

I ge the following result after I confirm the installation:

```
Need to install the following packages:
oh-my-opencode@2.14.0
Ok to proceed? (y) 

env: bun: No such file or directory
```

I have NodeJS v22.16.0 installed and I'm not using bun at all. Also, openc…

---

