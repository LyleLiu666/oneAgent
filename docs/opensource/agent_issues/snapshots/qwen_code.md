# Qwen Code — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): 2026-01-30T14:44:25Z
- Repo: https://github.com/QwenLM/qwen-code
- Query: `repo:QwenLM/qwen-code is:issue is:open sort:reactions-desc`
- Top N: 10

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

### [ollama qwen coder model support](https://github.com/QwenLM/qwen-code/issues/187) (#187)
- Reactions: **26** | Comments: **18** | Updated: 2026-01-05T01:52:07Z

### What would you like to be added?

Please add support for ollama self hosted models ,
Right now local models either just work as chat or dont work at all .
I tried changing ollama context setting but it didnt help.

### Why is this needed?

The same models in cloud work just fine but when you try self host it something breaks and model are unable to work in agent mode.

### Additional context

_No response_

---

### [[bug] qwen code tokens 消耗不正常](https://github.com/QwenLM/qwen-code/issues/83) (#83)
- Reactions: **8** | Comments: **17** | Updated: 2025-10-17T10:57:06Z
- Labels: `type/bug`

昨天修了三个故障，消耗了2千多万tokens，联系商务和阿里云技术协助排查后给退代金券了

今天试了下cline，改了一个和昨天类似的故障，cline消费了60万token（58万输入，2万输出），昨天qwen code消耗了800万token。qwen code肯定存在问题，消耗token是其他插件的十倍都不止

---

### [✕ [API Error: Streaming setup timeout after 64s. Try reducing input length or increasing timeout in config.](https://github.com/QwenLM/qwen-code/issues/239) (#239)
- Reactions: **8** | Comments: **14** | Updated: 2025-11-12T01:35:03Z
- Labels: `status/need-information`, `type/bug`

### What happened?

✕ [API Error: Streaming setup timeout after 64s. Try reducing input length or increasing timeout in config.

  Streaming setup timeout troubleshooting:
  - Reduce input length or complexity
  - Increase timeout in config: contentGenerator.timeout
  - Check network connectivity and firewall settings
  - Consider using non-streaming mode for very long inputs]

### What did you expect to happen?

不要 报告这个错误，自己处理好

### Client information

<details>

```console
$ qwen /about
# paste output here
```

</details>

### Login information

_No response_

### Anything else we need to kn…

---

### [Better ACP & IDE integration](https://github.com/QwenLM/qwen-code/issues/987) (#987)
- Reactions: **8** | Comments: **2** | Updated: 2026-01-26T08:01:39Z
- Labels: `category/integration`

This task tracks all ACP integration issues.

---

### [Tool calling does not work with local model qwen3-30b-a3b](https://github.com/QwenLM/qwen-code/issues/176) (#176)
- Reactions: **7** | Comments: **22** | Updated: 2025-10-17T10:53:14Z
- Labels: `type/bug`

### What happened?

I was trying to get qwen code running with a local instance of the small coder model. I can see the model responds with what look like appropriate tool calls to me, but the tool calls seem to not be executed. Unfortunately I don't see any errors, so I don't why they don't work as expected.

If needed, I can provide further logs of the server requests. 

### What did you expect to happen?

When I ask qwen code to init a git repository and the response from the locally hosted server is:

```console
[format_partial_response_oaicompat] DEBUG: Streaming finish_reason check | tid…

---

### [Tool calls with self-hosted Qwen3-Coder fail (almost always)](https://github.com/QwenLM/qwen-code/issues/124) (#124)
- Reactions: **6** | Comments: **5** | Updated: 2025-10-17T10:55:12Z
- Labels: `type/bug`

### What happened?

Self hosting Qwen3-Coder-480B-A35B-Instruct was a success, using something like this, but tool calls don't work:

**Without a jinja template**

```bash
 lama-box --host 0.0.0.0 --embeddings --gpu-layers 63 --parallel 4 --ctx-size 8192 --port 40003 --model /mnt/nfs/models/Qwen3-Coder-480B-A35B-Instruct-Q4_K_M-00001-of-00006.gguf --alias Qwen3-Coder-480B-A35B-Instruct --no-mmap --no-warmup --tensor-split 182335,182335 --ctx-size 262144 --flash-attn --parallel 4 --mmap --mlock --verbose --top-k 20 --temp 0.7 --top-p 0.8 --repeat-penalty 1.05 --min-p 0.00
```

**With a jinja te…

---

### [claude agent skills](https://github.com/QwenLM/qwen-code/issues/965) (#965)
- Reactions: **6** | Comments: **4** | Updated: 2026-01-13T06:17:13Z
- Labels: `priority/P1`

Are there any plans to support anthropic agent skills?
https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills

---

### [is_background missing property and isn't boolean](https://github.com/QwenLM/qwen-code/issues/472) (#472)
- Reactions: **5** | Comments: **13** | Updated: 2025-12-30T14:54:35Z
- Labels: `status/in-review`, `type/bug`, `type/feature-request`

### What happened?

Since you merged the #445 all I get is "params/is_background must be boolean" and "params must have required property 'is_background" even if I update the memory with [https://github.com/QwenLM/qwen-code/blob/main/docs/tools/shell.md](shell.md) it still gives the errors.

I'm using Qwen/Qwen3-Coder-30B-A3B-Instruct

### What did you expect to happen?

Execute the shell commands successfully after prompting with examples how to use the new is_background parameter for the shell.

### Client information

<details>

```console
$ qwen /about
# paste output here
```

</details>

…

---

### [[API Error: 400 <400> InternalError.Algo.InvalidParameter: Range of input length should be [1, 1048576]], A potential loop was detected. This can happen due to repetitive tool calls or other model behavior. The request has been halted.](https://github.com/QwenLM/qwen-code/issues/350) (#350)
- Reactions: **5** | Comments: **3** | Updated: 2025-10-17T10:01:00Z
- Labels: `status/need-information`, `type/bug`

### What happened?

出现死循环

### What did you expect to happen?

出现死循环

### Client information

* **CLI Version:** 0.0.7
* **Git Commit:** 14e6d3c0
* **Operating System:** darwin v22.17.1
* **Sandbox Environment:** no sandbox
* **Model Version:** qwen3-coder-480b-a35b-instruct
* **Memory Usage:** 134.8 MB

### Login information

_No response_

### Anything else we need to know?

_No response_

---

### [ERR_MODULE_NOT_FOUND error from my debug console](https://github.com/QwenLM/qwen-code/issues/193) (#193)
- Reactions: **5** | Comments: **2** | Updated: 2025-10-17T10:52:34Z
- Labels: `type/bug`

### What happened?

I got these error messages when I was working with qwen:
```
╭───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮                              
│ Debug Console (ctrl+o to close)                                                                                                                                   │                              
│                                                                                                                              …

---

