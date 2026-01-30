# Microsoft AutoGen — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:30Z
- Repo: https://github.com/microsoft/autogen
- Query: `repo:microsoft/autogen is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [[Feature Request]: Golang/Rust implementation](https://github.com/microsoft/autogen/issues/1700) (#1700)
- Reactions: **34** | Comments: **4** | Updated: 2026-01-02T20:30:24Z

### Is your feature request related to a problem? Please describe.

Introducing Rust and Golang implementations for Microsoft Autogen can significantly enhance its asynchronous capabilities and data integration features. These languages offer unique advantages in system programming, concurrency, and performance, which can be particularly beneficial in the context of Autogen's functionality for generating and managing code. Below, I'll outline the potential benefits and use cases for incorporating Rust and Golang into Autogen's ecosystem.

### Describe the solution you'd like

### Rust Implemen…

---

### [autogen-magentic-one ModuleNotFoundError: No module named 'autogen_core'](https://github.com/microsoft/autogen/issues/4079) (#4079)
- Reactions: **6** | Comments: **27** | Updated: 2025-02-25T15:04:15Z
- Labels: `proj-magentic-one`

Hi,
  I am trying to run example for autogen-magentic-one and followed all the steps in your docs but getting following error:

python autogen-magentic-one/examples/example_coder.py --logs_dir ./my_logs
Traceback (most recent call last):
  File "/home/Ubuntu/autogen/python/packages/autogen-magentic-one/examples/example_coder.py", line 10, in <module>
    from autogen_core.application import SingleThreadedAgentRuntime
ModuleNotFoundError: No module named 'autogen_core'

The docs mentioned to run example.py but there is no example.py in the repo. 

I am running this in conda environment. Thanks.…

---

### [[Roadmap] AutoGen Studio Roadmap](https://github.com/microsoft/autogen/issues/4006) (#4006)
- Reactions: **5** | Comments: **0** | Updated: 2025-07-16T17:03:07Z
- Labels: `proj-studio`, `proj-agentchat`, `size-large`

### What feature would you like to be added?

Current version of AutoGen Studio (AGS) is based on the AutoGen 0.2x api.
As the AgentChat api becomes more stable, this issue will track efforts to rebase AutoGen Studio on the AgentChat api.

#### Issues  

Issues for AGS are tracked here. (It is recommended that issues with the `needs-design` label be discussed before any implementation).

- [x] #5773
- [ ] #5891
- [ ] #5751
- [ ] Component Generation Support in AGS 
- [x] #6163
- [ ] Enable background tasks in AGS 
- [ ] Enable simple evals in AGS
- [ ] Data collection support for FT 
- [x] #62…

---

### [MCP tool JSON serialization lacks ensure_ascii=False, degrades LLM performance for Japanese text](https://github.com/microsoft/autogen/issues/6995) (#6995)
- Reactions: **5** | Comments: **0** | Updated: 2025-09-06T02:50:27Z
- Labels: `needs-triage`

### What happened?

**Describe the bug**
In the MCP tool implementation ([autogen_ext/tools/mcp/_base.py](https://github.com/microsoft/autogen/blob/main/python/packages/autogen-ext/src/autogen_ext/tools/mcp/_base.py#L190)), JSON serialization does not set `ensure_ascii=False`. This causes all non-ASCII characters, including Japanese, to be escaped as Unicode codepoints (e.g., "\u65e5\u672c"), which often degrades LLM understanding and generation quality in Japanese environments.

### Which packages was the bug in?

Python Extensions (autogen-ext)

### AutoGen library version.

Python dev (main…

---

### [Unify tool functions and user defined functions](https://github.com/microsoft/autogen/issues/2101) (#2101)
- Reactions: **4** | Comments: **5** | Updated: 2025-02-25T15:04:14Z
- Labels: `code-execution`, `tool-usage`

Currently, tool functions and user defined functions in executors behave fairly differently. We would like to unify the behavior and usage of these so that they are easier to understand and use.

More details to come in this issue.

---

### [Adding Memory Components (useful for RAG workflows) in AGS](https://github.com/microsoft/autogen/issues/4707) (#4707)
- Reactions: **4** | Comments: **2** | Updated: 2025-03-02T23:44:51Z
- Labels: `proj-studio`, `size-large`


Add the ability to attach memory to agents in AGS.

## What 

A common pattern (or requirement) for agentic apps is to support the ability to add memory to agents.
Here memory refers to the ability to augment model context based on the state of the agent (previous message). 

It is valuable to support this in a tool like AGS. An example would be that users can:

- Define a Memory component (similar to how tools, models, etc can be defined today). Memory might be a simple list backed by a file on disc, or a vector 
- Attach the memory to an agent .. similar to how you can drag a tool into an a…

---

### [Gemini Model Client in `autogen-ext`](https://github.com/microsoft/autogen/issues/3741) (#3741)
- Reactions: **4** | Comments: **6** | Updated: 2025-02-07T22:25:12Z
- Labels: `proj-extensions`

### What feature would you like to be added?

Model client for Gemini mirroring the one in v0.2

### Why is this needed?

Gemini is a notable provider

---

### [[Feature Request]: Skills in other languages than Python](https://github.com/microsoft/autogen/issues/2834) (#2834)
- Reactions: **3** | Comments: **0** | Updated: 2024-10-24T14:58:07Z
- Labels: `0.2`, `needs-triage`

### Is your feature request related to a problem? Please describe.

I'm not very familiar with Python, so I have trouble creating advanced Skill-functions.

I would love to create Skills using PHP or JS, but could not find a way to instruct the agent to use `php-cli`or `nodejs` to execute the skills. Apparently, they are currently limited to only run python code.

### Describe the solution you'd like

Allow running skills in any language that is currently available on the system.

To do this, I suggest to respect the Shebang line that defines the interpreter.
If no Shebang is present, assume t…

---

### [Typescript language support in new architecture](https://github.com/microsoft/autogen/issues/3858) (#3858)
- Reactions: **3** | Comments: **1** | Updated: 2024-10-24T14:57:58Z

- Ability to implement agents in Typescript
- In process runtime 

---

### [Model Costs and Cached Tokens](https://github.com/microsoft/autogen/issues/4835) (#4835)
- Reactions: **3** | Comments: **8** | Updated: 2025-08-19T07:45:57Z

### What feature would you like to be added?

Previously there where fields `client.total_usage_summary` and `planner.client.actual_usage_summary` with the amount of tokens and the costs. There is a class 

```
@dataclass
class RequestUsage:
    prompt_tokens: int
    completion_tokens: int
```

but I think apart from the logic being flawed (see https://github.com/microsoft/autogen/issues/4769, https://github.com/microsoft/autogen/issues/4719) it also lacks important fields. Most notably the costs and the cached tokens.

I think this should also be mentioned in the Migration Guide.

### Why is…

---

