# OpenHands — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:29Z
- Repo: https://github.com/OpenHands/OpenHands
- Query: `repo:OpenHands/OpenHands is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [Simple directions for dockerless install](https://github.com/OpenHands/OpenHands/issues/8632) (#8632)
- Reactions: **23** | Comments: **4** | Updated: 2025-11-20T19:56:49Z
- Labels: `enhancement`, `roadmap`

**What problem or use case are you trying to solve?**

It is relatively easy to get started if you have docker, but it's not easy to get started without docker.

**Describe the UX or technical implementation you have in mind**

We can start with our local runtime directions: https://docs.all-hands.dev/modules/usage/runtimes/local
But cover the entirety of the "getting started" process.

Additionally, we should consider making vscode, jupyter, and the browser optional dependencies.

### If you find this feature request or enhancement useful, make sure to add a 👍 to the issue


---

### [[PRD] Planning Agent](https://github.com/OpenHands/OpenHands/issues/8964) (#8964)
- Reactions: **13** | Comments: **9** | Updated: 2026-01-05T17:57:17Z
- Labels: `enhancement`, `roadmap`

**A planning agent has been implemented. The planning agent has read-only tools except for one file that it can write in: a** `Plan.md` at the root of its workspace.

**Missing features**

* Switch from "Plan" to "Code" mode within a conversation. (probably CLI-only)
* The user should be able to save their model preferences for "Plan" and "Code" mode. For example:
  * When in "Plan" mode, a developer may want to use GPT-5 for it's reasoning abilities
  * When in "Code" mode, a developer may prefer Claude Sonnet 4 for it's coding ability
* Two planning modes
  * CLI-planning = light planning be…

---

### [[Agent] Implement Critic Model](https://github.com/OpenHands/OpenHands/issues/8963) (#8963)
- Reactions: **11** | Comments: **9** | Updated: 2026-01-27T00:29:25Z
- Labels: `enhancement`, `roadmap`

**What problem or use case are you trying to solve?**
This idea of using choosing the best of multiple solutions has been tried by other SWE-bench submissions, but these strategies were generally based on prompting an existing model like Claude. Rather than using this prompt-based reranking strategy, we trained a dedicated critic model, which we found provided more effective results.

The goal of this issue is to implement a critic model in OpenHands.

**Additional context**
Read more about the OpenHands Critic model here: https://www.all-hands.dev/blog/sota-on-swe-bench-verified-with-inferenc…

---

### [Multiple LLM working in sync ](https://github.com/OpenHands/OpenHands/issues/2075) (#2075)
- Reactions: **6** | Comments: **21** | Updated: 2026-01-05T15:00:26Z
- Labels: `enhancement`


@mroch @li-boxuan @jeremi @penberg @JensRoland 

integrate a feature that can allow user to use multiple llm models in the project with their special expertise 

for example :

when user add 3 LLM models into opendevin with specific usage 

first LLM should only be use research and browsing like GPT-3.5, Mixtral , 

second LLM model can be used for code generation like GPT-4o, deepseeker , code llama

third LLM model can be used for any reasoning thinking or any other task or role assign by user like GPT-4o, llama3-70b

user can change the model or role anytime in the middle of project or at …

---

### [Proposal: Separate Repositories](https://github.com/OpenHands/OpenHands/issues/10649) (#10649)
- Reactions: **6** | Comments: **10** | Updated: 2026-01-06T17:42:13Z
- Labels: `enhancement`

## The Problem

This repository has grown like a weed. We have many different things mingled in here:
* Prompts and microagents
* Logic for running agents
* A web server
* A frontend
* Logic for bash runtime environments
* A CLI package
* Evaluation logic

This creates all sorts of headaches:
* It makes the repo very unapproachable for new users
* CI/CD takes forEVER (and is often flaky w/ runtime and frontend tests)
* We have weird interdependencies that shouldn't exist
* Package/image sizes are massive.

## The Solution

I'd like to propose that we break things up into the following reposito…

---

### [Improve timeout handling and feedback for slow local LLMs](https://github.com/OpenHands/OpenHands/issues/8768) (#8768)
- Reactions: **6** | Comments: **7** | Updated: 2026-01-13T21:51:28Z
- Labels: `enhancement`, `configuration`, `settings`

**What problem or use case are you trying to solve?**

When using OpenHands with LM Studio, long generations from local models can exceed the default timeout. This causes OpenHands to disconnect before the response is ready, resulting in a loop of retry attempts with no clear explanation or feedback.

**Describe the UX or technical implementation you have in mind**

- Expose an **LLM timeout** setting in the **LLM settings** panel when **Advanced mode** is enabled  
- Improve retry messages to clearly indicate the cause of failure (e.g. **“LLM timeout exceeded”**)

**Additional context**

I fi…

---

### [[PRD] Fork a Conversation](https://github.com/OpenHands/OpenHands/issues/8560) (#8560)
- Reactions: **6** | Comments: **21** | Updated: 2026-01-26T02:40:16Z
- Labels: `enhancement`, `roadmap`, `Needs Design`

Any part of the conversational thread should be editable, so we can guide the agent better, including its own responses and actions.

Forking a thread is fairly slow and cumbersome, re: https://github.com/All-Hands-AI/OpenHands/issues/8555 (that's when I start a new thread, and copy paste only some context I care about, to re-bootstrap the agent on a specific task)

What I would like is more control over the conversational thread, because the whole thing gets fed back on every follow-up response. But, most of the time the agent goes all over the place, and that's not helping it act better on t…

---

### [Chat Reset](https://github.com/OpenHands/OpenHands/issues/8989) (#8989)
- Reactions: **6** | Comments: **6** | Updated: 2026-01-25T21:24:19Z
- Labels: `enhancement`, `chat`, `backlog`

**What problem or use case are you trying to solve?**

A feature to reset all chat history in a current session would be good to have. In some projects initializing the environment takes a lot of time (installing all requirements, building the project, training models ect). In these projects its not efficient to start every task in a new session.

However executing all tasks in the same chat, the agent tends to repeat initial tasks like "install all requirements and build the project". It would therefore be better to have a chat reset feature, which keeps the environment while resetting the ch…

---

### [[Bug]: 500 error new conversation](https://github.com/OpenHands/OpenHands/issues/12083) (#12083)
- Reactions: **5** | Comments: **25** | Updated: 2026-01-16T01:04:05Z
- Labels: `bug`

### Is there an existing issue for the same bug? (If one exists, thumbs up or comment on the issue instead).

- [x] I have checked the existing issues.

### Describe the bug and reproduction steps

Hello everyone,

I'm new to using OpenHands, and I'm encountering a problem when I try to create a conversation:

Server error '500 Internal Server Error' for url 'http://host.docker.internal:38693/api/conversations'
For more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500
I installed it via Docker using the official documentation.

I'm using ollama with mistral:7b-in…

---

### [Support for `external_url` config.
](https://github.com/OpenHands/OpenHands/issues/6356) (#6356)
- Reactions: **5** | Comments: **10** | Updated: 2026-01-15T14:48:57Z
- Labels: `enhancement`, `configuration`

I would like to host this on a server other than localhost but have run into various issues with what appear to be hard coded localhost urls. It would be great to have a config option to set the external url so that these urls resolve properly.

**What problem or use case are you trying to solve?**

* hosting open hands on a server other than localhost

**Describe the UX of the solution you'd like**

Config option 

### If you find this feature request or enhancement useful, make sure to add a 👍 to the issue



---

