# Aider Top Issues (Open & High Reactions)

Fetched at: Fri Jan 30 22:32:32 CST 2026
Repo: https://github.com/Aider-AI/aider

## [Uncaught AttributeError in nova-aider.py line 76](https://github.com/Aider-AI/aider/issues/4804) (#4804)
**Created**: 2026-01-28T07:16:37Z

Aider version: 0.86.1
Python version: 3.12.12
Platform: macOS-26.2-arm64-arm-64bit
Python implementation: CPython
Virtual environment: Yes
OS: Darwin 25.2.0 (64bit)
Git version: git version 2.50.1 (Apple Git-155)

An uncaught exception occurred:

```
Traceback (most recent call last):
  File "nova-aider.py", line 79, in <module>
    begin()
  File "nova-aider.py", line 76, in begin
    message = coder.get_input()
              ^^^^^^^^^^^^^^^
AttributeError: 'int' object has no attribute 'get_input'

```

---

## [Dependency failure attempting to install langchain-community](https://github.com/Aider-AI/aider/issues/4801) (#4801)
**Created**: 2026-01-26T17:42:27Z

When attempting to install `langchain-community` with `aider-chat` v.0.86.1, I receive the following error:

```
❯ uv add langchain-community
  × No solution found when resolving dependencies for split (python_full_version >= '3.13'):
  ╰─▶ Because ai-devtools:dev depends on aider-chat>=0.86.1 and only aider-chat<=0.86.1 is available, we can conclude that ai-devtools:dev depends
      on aider-chat==0.86.1.
      And because aider-chat==0.86.1 depends on requests==2.32.4, we can conclude that ai-devtools:dev depends on requests==2.32.4.
      And because langchain-community>=0.3.28 depends on requests>=2.32.5 and langchain-community==0.2.14 was yanked (reason: Overly restrictive
      langchain-core dependency. Fixed in 0.2.15), we can conclude that all of:
          langchain-community==0.2.14
          langchain-community>=0.3.28
       and ai-devtools:dev are incompatible. (1)

      Because only the following versions of langchain-community are available:
          langchain-community==0.0.1
          langchain-community==0.0.2
          langchain-community==0.0.3
          langchain-community==0.0.4
          langchain-community==0.0.5
          langchain-community==0.0.6
          langchain-community==0.0.7
          langchain-community==0.0.8
          langchain-community==0.0.9
          langchain-community==0.0.10
          langchain-community==0.0.11
          langchain-community==0.0.12
          langchain-community==0.0.13
          langchain-community==0.0.14
          langchain-community==0.0.15
          langchain-community==0.0.16
          langchain-community==0.0.17
          langchain-community==0.0.18
          langchain-community==0.0.19
          langchain-community==0.0.20
          langchain-community==0.0.21
          langchain-community==0.0.22
          langchain-community==0.0.23
          langchain-community==0.0.24
          langchain-community==0.0.25
          langchain-community==0.0.26
          langchain-community==0.0.27
          langchain-community==0.0.28
          langchain-community==0.0.29
          langchain-community==0.0.30
          langchain-community==0.0.31
          langchain-community==0.0.32
          langchain-community==0.0.33
          langchain-community==0.0.34
          langchain-community==0.0.35
          langchain-community==0.0.36
          langchain-community==0.0.37
          langchain-community==0.0.38
          langchain-community==0.2.0
          langchain-community==0.2.1
          langchain-community==0.2.2
          langchain-community==0.2.3
          langchain-community==0.2.4
          langchain-community==0.2.5
          langchain-community==0.2.6
          langchain-community==0.2.7
          langchain-community==0.2.9
          langchain-community==0.2.10
          langchain-community==0.2.11
          langchain-community==0.2.12
          langchain-community==0.2.13
          langchain-community==0.2.14
          langchain-community==0.2.15
          langchain-community==0.2.16
          langchain-community==0.2.17
          langchain-community==0.2.18
          langchain-community==0.2.19
          langchain-community==0.3.0
          langchain-community==0.3.1
          langchain-community==0.3.2
          langchain-community==0.3.3
          langchain-community==0.3.4
          langchain-community==0.3.5
          langchain-community==0.3.6
          langchain-community==0.3.7
          langchain-community==0.3.8
          langchain-community==0.3.9
          langchain-community==0.3.10
          langchain-community==0.3.11
          langchain-community==0.3.12
          langchain-community==0.3.13
          langchain-community==0.3.14
          langchain-community==0.3.15
          langchain-community==0.3.16
          langchain-community==0.3.17
          langchain-community==0.3.18
          langchain-community==0.3.19
          langchain-community==0.3.20
          langchain-community==0.3.21
          langchain-community==0.3.22
          langchain-community==0.3.23
          langchain-community==0.3.24
          langchain-community==0.3.25
          langchain-community==0.3.26
          langchain-community==0.3.27
          langchain-community==0.3.28
          langchain-community==0.3.29
          langchain-community==0.3.30
          langchain-community==0.3.31
          langchain-community==0.4
          langchain-community==0.4.1
      and langchain-community==0.0.1 depends on langchain-core>=0.0.13,<0.1, we can conclude that langchain-community<0.0.2 depends on
      langchain-core>=0.0.13,<0.1.
      And because langchain-community>=0.0.2,<=0.0.7 depends on langchain-core>=0.1,<0.2 and langchain-core>=0.1.5,<0.2, we can conclude that
      langchain-community<0.0.9 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2

      And because langchain-community==0.0.9 was yanked (reason: bad imports from langchain-core) and langchain-community>=0.0.10,<=0.0.11 depends
      on langchain-core>=0.1.8,<0.2, we can conclude that langchain-community<0.0.12 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2

      And because langchain-community>=0.0.12,<=0.0.13 depends on langchain-core>=0.1.9,<0.2 and langchain-core>=0.1.14,<0.2, we can conclude that
      langchain-community<0.0.16 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2

      And because langchain-community>=0.0.16,<=0.0.17 depends on langchain-core>=0.1.16,<0.2 and langchain-core>=0.1.19,<0.2, we can conclude that
      langchain-community<0.0.19 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2

      And because langchain-community>=0.0.19,<=0.0.20 depends on langchain-core>=0.1.21,<0.2 and langchain-core>=0.1.24,<0.2, we can conclude that
      langchain-community<0.0.22 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2

      And because langchain-community>=0.0.22,<=0.0.24 depends on langchain-core>=0.1.26,<0.2 and langchain-core>=0.1.28,<0.2.0, we can conclude
      that langchain-community<0.0.26 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community==0.0.26 depends on langchain-core>=0.1.29,<0.2.0 and langchain-core>=0.1.30,<0.2.0, we can conclude that
      langchain-community<0.0.28 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community==0.0.28 depends on langchain-core>=0.1.31,<0.2.0 and langchain-core>=0.1.33,<0.2.0, we can conclude that
      langchain-community<0.0.30 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community>=0.0.30,<=0.0.31 depends on langchain-core>=0.1.37,<0.2.0 and langchain-core>=0.1.41,<0.2.0, we can conclude
      that langchain-community<0.0.33 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community==0.0.33 depends on langchain-core>=0.1.43,<0.2.0 and langchain-core>=0.1.45,<0.2.0, we can conclude that
      langchain-community<0.0.35 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community==0.0.35 depends on langchain-core>=0.1.47,<0.2.0 and langchain-core>=0.1.48,<0.2.0, we can conclude that
      langchain-community<0.0.37 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain-community==0.0.37 depends on langchain-core>=0.1.51,<0.2.0 and langchain-core>=0.1.52,<0.2.0, we can conclude that
      langchain-community<0.2.0 depends on one of:
          langchain-core>=0.0.13,<0.1
          langchain-core>=0.1,<0.2.0

      And because langchain==1.2.7 depends on langchain-core>=1.2.7,<2.0.0 and langchain-community>=0.3.26,<=0.3.27 depends on
      langchain>=0.3.26,<1.0.0, we can conclude that langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.25,<0.3.28
       are incompatible.
      And because langchain-community>=0.3.24,<=0.3.25 depends on langchain>=0.3.25,<1.0.0 and langchain>=0.3.24,<1.0.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.21,<0.3.28
       are incompatible.
      And because langchain-community==0.3.21 depends on langchain>=0.3.23,<1.0.0 and langchain>=0.3.21,<1.0.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.19,<0.3.28
       are incompatible.
      And because langchain-community==0.3.19 depends on langchain>=0.3.20,<1.0.0 and langchain>=0.3.19,<1.0.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.17,<0.3.28
       are incompatible.
      And because langchain-community==0.3.17 depends on langchain>=0.3.18,<1.0.0 and langchain>=0.3.16,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.15,<0.3.28
       are incompatible.
      And because langchain-community==0.3.15 depends on langchain>=0.3.15,<0.4.0 and langchain>=0.3.14,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.13,<0.3.28
       are incompatible.
      And because langchain-community==0.3.13 depends on langchain>=0.3.13,<0.4.0 and langchain>=0.3.12,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.11,<0.3.28
       are incompatible.
      And because langchain-community==0.3.11 depends on langchain>=0.3.11,<0.4.0 and langchain>=0.3.10,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.9,<0.3.28
       are incompatible.
      And because langchain-community>=0.3.8,<=0.3.9 depends on langchain>=0.3.8,<0.4.0 and langchain>=0.3.7,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.5,<0.3.28
       are incompatible.
      And because langchain-community>=0.3.4,<=0.3.5 depends on langchain>=0.3.6,<0.4.0 and langchain>=0.3.4,<0.4.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.3.2,<0.3.28
       are incompatible.
      And because langchain-community==0.3.2 depends on langchain>=0.3.3,<0.4.0 and langchain>=0.3.1,<0.4.0, we can conclude that langchain==1.2.7
      and all of:
          langchain-community<0.2.0
          langchain-community>0.3.0,<0.3.28
       are incompatible.
      And because langchain-community==0.3.0 depends on langchain>=0.3.0,<0.4.0 and langchain>=0.2.17,<0.3.0, we can conclude that langchain==1.2.7
      and all of:
          langchain-community<0.2.0
          langchain-community>0.2.17,<0.3.28
       are incompatible.
      And because langchain-community>=0.2.16,<=0.2.17 depends on langchain>=0.2.16,<0.3.0 and langchain>=0.2.15,<0.3.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.2.12,<0.2.14
          langchain-community>0.2.14,<0.3.28
       are incompatible.
      And because langchain-community==0.2.12 depends on langchain>=0.2.13,<0.3.0 and langchain>=0.2.12,<0.3.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.2.10,<0.2.14
          langchain-community>0.2.14,<0.3.28
       are incompatible.
      And because langchain-community>=0.2.9,<=0.2.10 depends on langchain>=0.2.9,<0.3.0 and langchain>=0.2.7,<0.3.0, we can conclude that
      langchain==1.2.7 and all of:
          langchain-community<0.2.0
          langchain-community>0.2.6,<0.2.14
          langchain-community>0.2.14,<0.3.28
       are incompatible.
      And because langchain-community==0.2.6 depends on langchain>=0.2.6,<0.3.0 and langchain>=0.2.5,<0.3.0, we can conclude that langchain==1.2.7
      and all of:
          langchain-community<0.2.0
          langchain-community>0.2.4,<0.2.14
          langchain-community>0.2.14,<0.3.28
       are incompatible.
      And because langchain-community>=0.2.0,<=0.2.4 depends on langchain>=0.2.0,<0.3.0 and only langchain<=1.2.7 is available, we can conclude
      that langchain>=1.2.7 and all of:
          langchain-community<0.2.14
          langchain-community>0.2.14,<0.3.28
       are incompatible.
      And because your project depends on langchain>=1.2.7 and langchain-community, we can conclude that your project depends on one of:
          langchain-community==0.2.14
          langchain-community>=0.3.28

      And because we know from (1) that all of:
          langchain-community==0.2.14
          langchain-community>=0.3.28
       and ai-devtools:dev are incompatible, we can conclude that your project and ai-devtools:dev are incompatible.
      And because your project requires your project and ai-devtools:dev, we can conclude that your project's requirements are unsatisfiable.

      hint: Pre-releases are available for `langchain-community` in the requested range (e.g., 1.0.0a1), but pre-releases weren't enabled (try:
      `--prerelease=allow`)
  help: If you want to add the package regardless of the failed resolution, provide the `--frozen` flag to skip locking and syncing.
```

I've tried the usual things, like `--frozen`, and `--prerelease=allow`, but those only led to more errors.

I also tried cloning `main`, and referencing it, but also received dependency errors.

Am I doing something wrong?

Any help is appreciated.

---

## [Integration: Screenpipe for screen context in coding sessions](https://github.com/Aider-AI/aider/issues/4800) (#4800)
**Created**: 2026-01-25T15:48:34Z

## Integration Idea

Add **[Screenpipe](https://github.com/mediar-ai/screenpipe)** support to provide screen context during coding sessions!

## What is Screenpipe?

Screenpipe (16k+ stars) is a 24/7 local screen & mic recording tool:
- Captures screen via OCR
- Transcribes audio
- Local storage + REST API

## Why This Matters for Aider

When coding, developers constantly:
- Read documentation
- Look at Stack Overflow
- View error messages
- Attend meetings discussing features

Screenpipe captures all of this. Aider could use it to:

1. **Reference docs you viewed**: "Use the API I was reading about"
2. **Understand error context**: Auto-include recent error messages
3. **Meeting context**: "Implement what we discussed"

## Integration Ideas

1. **Context command**: `/context` pulls recent screen content
2. **Auto-context**: Include relevant screen content in prompts
3. **Meeting mode**: Reference recent meeting transcripts

## Example

```
> /context last 30 minutes
[Screenpipe shows: Error stack trace, API docs, Slack discussion]

> Fix the error I saw and use the API pattern from the docs
```

Both tools are **local-first** - no data leaves your machine!

Would love to discuss feasibility.

---

## [Aider is not using the .aider.model.settings.yml file and seems to take the user's configuration as the default.](https://github.com/Aider-AI/aider/issues/4798) (#4798)
**Created**: 2026-01-25T13:37:19Z

Aider is not using the .aider.model.settings.yml file and seems to take the user's configuration as the default.

```console
$ cat .aider.model.settings.yml
- name: ollama_chat/phi3.5:3.8b-mini-instruct-q4_K_M
  extra_params:
    num_ctx: 65536

$ aider
Aider v0.86.1
Model: codestral/codestral-latest with whole edit format, infinite output

$ aider --model-settings-file .aider.model.settings.yml
Aider v0.86.1
Model: codestral/codestral-latest with whole edit format, infinite output

$ aider --model ollama/phi3.5:3.8b-mini-instruct-q4_K_M
Aider v0.86.1
Model: ollama/phi3.5:3.8b-mini-instruct-q4_K_M with whole edit format
```


---

## [enable z.ai GLM 4.7 with aider](https://github.com/Aider-AI/aider/issues/4797) (#4797)
**Created**: 2026-01-25T06:04:48Z

### Issue

Hello
How are you? 

Trying to use aider with GLM 4.7 or the flash but I get this error msg.

I do see it listed in models do, so not sure what it needs.



```

aider --model zai/glm-4.7
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Aider v0.86.1
Model: zai/glm-4.7 with whole edit format
Git repo: .git with 862 files
Repo-map: using 4096 tokens, auto refresh
Added README.md to the chat (read-only).
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Readonly: README.md                                                                                                   
> hello                                                                                                               


litellm.BadRequestError: LLM Provider NOT provided. Pass in the LLM provider you are trying to call. You passed 
model=zai/glm-4.7
 Pass model as E.g. For 'Huggingface' inference endpoints pass in `completion(model='huggingface/starcoder',..)` Learn
more: https://docs.litellm.ai/docs/providers

https://docs.litellm.ai/docs/providers
Open URL for more info? (Y)es/(N)o/(D)on't ask again [Yes]:                         
```

### Version and model info

_No response_

---

