# Aider — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:28Z
- Repo: https://github.com/Aider-AI/aider
- Query: `repo:Aider-AI/aider is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [MCP SUPPORT](https://github.com/Aider-AI/aider/issues/3314) (#3314)
- Reactions: **234** | Comments: **20** | Updated: 2025-08-26T11:28:33Z

### Issue

Is there plane to add mcp server call?

### Version and model info

_No response_

---

### [Please add support for model context protocol from anthropic ](https://github.com/Aider-AI/aider/issues/2525) (#2525)
- Reactions: **138** | Comments: **26** | Updated: 2025-06-12T22:07:43Z
- Labels: `enhancement`

### Issue

Please add support for model context protocol from anthropic 

### Version and model info

latest

---

### [Feature: Add GitHub Copilot as model provider](https://github.com/Aider-AI/aider/issues/2227) (#2227)
- Reactions: **119** | Comments: **212** | Updated: 2025-12-15T21:22:58Z
- Labels: `enhancement`, `priority`

### Issue

Hello!

Please add GitHub Copilot as model provider.

Should be possible like this: https://github.com/olimorris/codecompanion.nvim/blob/5c5a5c759b8c925e81f8584a0279eefc8a6c6643/lua/codecompanion/adapters/copilot.lua

Idea taken from: https://github.com/cline/cline/discussions/660

Thank you!

### Version and model info

_No response_

---

### [Config file location should follow "modern" specifications](https://github.com/Aider-AI/aider/issues/216) (#216)
- Reactions: **77** | Comments: **14** | Updated: 2026-01-14T18:13:25Z
- Labels: `enhancement`

Writing dotfiles into the root of user's home dir is not really a thing anymore.

For linux, there's the XDG base directory specification. This would make the location by default something like `~/.config/aider/config.yml`.  See https://gist.github.com/roalcantara/107ba66dfa3b9d023ac9329e639bc58c#file-xdg-cheat-sheet-md

On a mac `~/.config` is probably better for a terminal app than going to the "correct" `~/Library/Application Support/aider/config.yaml`. Tools like git, github copilot, gh cli, shells, neovim and many others use it or at least will make a symlink from there to Library.

On wi…

---

### [Add claude 4](https://github.com/Aider-AI/aider/issues/4063) (#4063)
- Reactions: **62** | Comments: **12** | Updated: 2025-05-28T18:45:18Z

### Issue

Released today (2025-05-22) 🎉 
- https://www.anthropic.com/news/claude-4

Please include the openrouter models in addition to the usual anthropic/bedrock/etc. ones :)
- https://openrouter.ai/anthropic/claude-sonnet-4
- https://openrouter.ai/anthropic/claude-opus-4

### Version and model info

_No response_

---

### [Aider integration with emacs](https://github.com/Aider-AI/aider/issues/1913) (#1913)
- Reactions: **61** | Comments: **0** | Updated: 2024-10-04T05:06:41Z
- Labels: `enhancement`

### Aider integration with emacs

Emacs have good capability to integrate with CLI tool and might be a natural fit to use aider to help coding.

I have some initial work to integrate aider into emacs at https://github.com/tninja/aider.el (mostly written by aider / aider.el itself), right now it support:
    - Pop-up menu for frequently used command, it can get detail user instruction from mini-buffer and send to aider
    - Git repo specific aider sessions in emacs: automatically identify your git repo of current file, and create a new aider session for it. Multiple aider sessions can exist fo…

---

### [Inspiration From Claude Code](https://github.com/Aider-AI/aider/issues/3362) (#3362)
- Reactions: **56** | Comments: **45** | Updated: 2025-07-23T05:53:31Z
- Labels: `enhancement`

I am the author of [aidermacs](https://github.com/MatthewZMD/aidermacs) and [emigo](https://github.com/MatthewZMD/emigo).

Given Claude has released their proprietary CLI tool [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview), I wonder if there are any cool ideas/features aider should implement.

Some that come to my mind:
1. Encourage deeper thinking
2. `> rebase on main and resolve any merge conflicts`

---

### [Feature request: vscode extension](https://github.com/Aider-AI/aider/issues/68) (#68)
- Reactions: **56** | Comments: **36** | Updated: 2025-05-08T19:28:28Z
- Labels: `enhancement`

Would be amazing to have this as an extension to vscode (and Visual Studio if possible) similar to Github Copilot chat. Would really make using this very seamless. Either way, this is an amazingly helpful tool. Thank you

---

### [Feature Request: Integration of LLM-Supported Tools and Function Calls in Aider](https://github.com/Aider-AI/aider/issues/2672) (#2672)
- Reactions: **42** | Comments: **4** | Updated: 2025-09-27T00:30:17Z

### Issue

Currently, Aider does not support integrating tools or function calls commonly supported by LLMs. It would be highly beneficial to enable tool integration with Aider to extend its capabilities. A few potential use cases include:

Building and testing applications
Packaging and deployment workflows
GitOps tasks, such as creating repositories, managing PRs, and commits
Operational tasks
Additionally, it would be valuable to support standard tooling protocols like MCP to ensure broader compatibility and efficiency.

### Version and model info

_No response_

---

### [PyCharm Support](https://github.com/Aider-AI/aider/issues/483) (#483)
- Reactions: **40** | Comments: **13** | Updated: 2025-01-22T10:54:52Z
- Labels: `enhancement`

It would be awesome to support PyCharm or other Jetbrains IDEs such as IntelliJ with a plugin.  aider could add all or selected projects in the Projects tab to its context.

---

