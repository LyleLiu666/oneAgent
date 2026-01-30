# OpenCode — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:23Z
- Repo: https://github.com/anomalyco/opencode
- Query: `repo:anomalyco/opencode is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [Broken Claude Max](https://github.com/anomalyco/opencode/issues/7410) (#7410)
- Reactions: **413** | Comments: **385** | Updated: 2026-01-29T06:05:56Z
- Labels: `bug`

### Description

As of a few moments ago, usage of claude max stopped with the following error: 

<img width="831" height="66" alt="Image" src="https://github.com/user-attachments/assets/ecc0f211-a883-4a6a-b1c8-816d7ea449a2" />

I did try to reconnect, but got the same error.

### Plugins

_No response_

### OpenCode version

1.1.8

### Steps to reproduce

_No response_

### Screenshot and/or share link

_No response_

### Operating System

mac

### Terminal

_No response_

---

### [Support for Cursor?](https://github.com/anomalyco/opencode/issues/2072) (#2072)
- Reactions: **119** | Comments: **48** | Updated: 2026-01-28T13:54:21Z

As Cursor has released their own CLI (https://cursor.com/cli), I'm wondering if Opencode could support it. It's just a thought, as I'm guessing the API for it is not made for public use or documented in any way, but it's worth a shot.

---

### [[FEATURE]: Plan mode questions like claude code](https://github.com/anomalyco/opencode/issues/3844) (#3844)
- Reactions: **108** | Comments: **8** | Updated: 2025-12-22T14:18:03Z
- Labels: `discussion`

### Feature hasn't been suggested before.

- [x] I have verified this feature I'm about to request hasn't been suggested before.

### Describe the enhancement you want to request

In the plan mode of claude code, it sometimes asks questions. Would be nice to have it here. 

---

### [Windows Support](https://github.com/anomalyco/opencode/issues/631) (#631)
- Reactions: **103** | Comments: **193** | Updated: 2026-01-28T20:28:23Z
- Labels: `windows`

our windows support isn't really there - creating this one super issue to track all problems

---

### [[Feature Request] Adding directories / creating workspaces](https://github.com/anomalyco/opencode/issues/1543) (#1543)
- Reactions: **74** | Comments: **19** | Updated: 2026-01-26T11:11:13Z

One of the features I utilize frequently in Claude Code is adding additional directories to a session that are outside the scope of my working directory. Are there currently plans to implement such a feature or to create a workspace similar to VS code?

---

### [[FEATURE]: vim motions in input box](https://github.com/anomalyco/opencode/issues/1764) (#1764)
- Reactions: **73** | Comments: **14** | Updated: 2026-01-30T07:53:02Z
- Labels: `tui`, `discussion`

Please add the option of using vim keyboard shortcuts when writing the prompt (ClaudeCode has it, see attached)

<img width="369" height="96" alt="Image" src="https://github.com/user-attachments/assets/717db25e-9df6-4bd5-a6d2-91444f8be2fd" />

<img width="777" height="511" alt="Image" src="https://github.com/user-attachments/assets/1cddb038-dda9-4618-971c-c3db0ae21e34" />

---

### [[FEATURE]: Speech-to-Text Voice Input for Lazy People in OpenCode](https://github.com/anomalyco/opencode/issues/4695) (#4695)
- Reactions: **72** | Comments: **11** | Updated: 2026-01-28T08:10:21Z
- Labels: `discussion`

### Feature hasn't been suggested before.

- [x] I have verified this feature I'm about to request hasn't been suggested before.

### Describe the enhancement you want to request

Hi! First of all, congratulations on the amazing project.

I've been working on a Speech-to-Text voice input feature that integrates directly into the TUI. It allows users to start audio recording with a keybind, automatically transcribe speech using different providers, and insert the resulting text directly into the prompt.

I've built an initial working version, currently tested only on macOS, and the system inclu…

---

### [Editor integrations](https://github.com/anomalyco/opencode/issues/216) (#216)
- Reactions: **63** | Comments: **29** | Updated: 2026-01-22T06:52:51Z

Editor integrations like Claude Code's VS Code extension or [claudecode.nvim](https://github.com/coder/claudecode.nvim) show diffs in the user's editor and sync context between the editor and agent.

I'm sure this is on the roadmap for opencode, but I couldn't find an existing issue so opening this as a feature request just to track it.

I don't think opencode ever needs to live inside say, neovim, but it would be nice if it could see what you have open and selected in neovim, and if it could display diffs in neovim.

---

### [[FEATURE]: Allow to expand the pasted text (e.g. `[Pasted ~1 lines]`)](https://github.com/anomalyco/opencode/issues/8501) (#8501)
- Reactions: **56** | Comments: **2** | Updated: 2026-01-29T17:15:11Z
- Labels: `opentui`, `discussion`

### Feature hasn't been suggested before.

- [x] I have verified this feature I'm about to request hasn't been suggested before.

### Describe the enhancement you want to request

#### The problem
I love that pasted text is summarized to avoid bloating the prompt, but I usually want to edit it or at least view it before sending the prompt.

This is especially inconvenient when using voice-to-text.

<img width="875" height="201" alt="Image" src="https://github.com/user-attachments/assets/95b53a50-f2e4-4b89-bea6-eeecb9542679" />

#### The current workarounds
- I know I can use my editor (leader …

---

### [[FEATURE]: Allow custom system prompts in global, project or custom directories](https://github.com/anomalyco/opencode/issues/7101) (#7101)
- Reactions: **53** | Comments: **12** | Updated: 2026-01-29T04:14:51Z
- Labels: `discussion`

### Feature hasn't been suggested before.

- [x] I have verified this feature I'm about to request hasn't been suggested before.

### Describe the enhancement you want to request

Based on a [reddit discussion on shorter system prompts](https://www.reddit.com/r/opencodeCLI/comments/1p6lxd4/shortened_system_prompts_in_opencode/), I checked how it could be implemented in opencode, to allow custom prompts. 

This would allow experimenting with the system prompt (especially on low-end models with limited context window) 

Perfect would be a similar approach as for opencode.json: 

Lowest to highes…

---

