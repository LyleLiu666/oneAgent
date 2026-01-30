# Continue Top Issues (Open & High Reactions)

Fetched at: Fri Jan 30 22:32:31 CST 2026
Repo: https://github.com/continuedev/continue

## [Lack of support for Colorblindness themes](https://github.com/continuedev/continue/issues/10037) (#10037)
**Created**: 2026-01-30T14:00:08Z

### Before submitting your bug report

- [x] I've tried using the "Ask AI" feature on the [Continue docs site](https://docs.continue.dev/) to see if the docs have an answer
- [x] I'm not able to find a related conversation on [GitHub discussions](https://github.com/continuedev/continue/discussions) that reports the same bug
- [x] I'm not able to find an [open issue](https://github.com/continuedev/continue/issues?q=is%3Aopen+is%3Aissue) that reports the same bug
- [x] I've seen the [troubleshooting guide](https://docs.continue.dev/troubleshooting) on the Continue Docs

### Relevant environment info

```Markdown
- OS: Windows
- Continue version: v1.2.14
- IDE version: VS Code 1.108.1
- Model: qwen2.5-coder:3b
- config:
  
name: Local Config
version: 1.0.0
schema: v1
models:
  - name: Llama 3.1 8B
    provider: ollama
    model: llama3.1:8b
    roles:
      - chat
      - edit
      - apply
  - name: Qwen2.5-Coder 1.5B
    provider: ollama
    model: qwen2.5-coder:1.5b-base
    roles:
      - autocomplete
  - name: Nomic Embed
    provider: ollama
    model: nomic-embed-text:latest
    roles:
      - embed
  - name: Autodetect
    provider: ollama
    model: AUTODETECT

  
  OR link to agent in Continue hub:
```

### Description

The code snippets put in chat present colors that compete with the themes.   I'm using Blinds(Light) and the function names match the background exactly (it may be because I'm red/green colorblind if you can see the function names).

<img width="408" height="296" alt="Image" src="https://github.com/user-attachments/assets/a32aaf92-348a-4273-8d2a-65e09defcb02" />

<img width="412" height="306" alt="Image" src="https://github.com/user-attachments/assets/7b1ccd11-fd1a-45db-9d98-06a726bdb579" />



### To reproduce

Install and use Blinds(Light).   Then ask the chat to generate a python function.  The function name will be the same color as the background.

### Log output

```Shell

```

---

## [Error: GPT-5 - Unknown error](https://github.com/continuedev/continue/issues/10035) (#10035)
**Created**: 2026-01-30T13:37:13Z

**Error Details**

Model: GPT-5
Provider: openai
Status Code: N/A

**Error Output**
```
Error streaming response: You exceeded your current quota, please check your plan and billing details. For more information on this error, read the docs: https://platform.openai.com/docs/guides/error-codes/api-errors.
```

**Additional Context**
Please add any additional context about the error here


---

## [Error: Claude Sonnet 4.5 - 402](https://github.com/continuedev/continue/issues/10033) (#10033)
**Created**: 2026-01-30T10:34:07Z

**Error Details**

Model: Claude Sonnet 4.5
Provider: continue-proxy
Status Code: 402

**Error Output**
```
"You have no credits remaining on your Continue account. Please go to https://hub.continue.dev/settings/billing to add credits. You can also add your own ANTHROPIC_API_KEY secret at https://continue.dev/settings/secrets%3FsecretName=ANTHROPIC_API_KEY"
```

**Additional Context**
Please add any additional context about the error here


---

## [Error: Gemini 2.5 Pro - Unknown error](https://github.com/continuedev/continue/issues/10032) (#10032)
**Created**: 2026-01-30T10:28:09Z

**Error Details**

Model: Gemini 2.5 Pro
Provider: gemini
Status Code: N/A

**Error Output**
```
{"error":{"message":"{\n  \"error\": {\n    \"code\": 429,\n    \"message\": \"You exceeded your current quota, please check your plan and billing details. For more information on this error, head to: https://ai.google.dev/gemini-api/docs/rate-limits. To monitor your current usage, head to: https://ai.dev/rate-limit. \\n* Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 0, model: gemini-2.5-pro\\n* Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 0, model: gemini-2.5-pro\\n* Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_input_token_count, limit: 0, model: gemini-2.5-pro\\n* Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_input_token_count, limit: 0, model: gemini-2.5-pro\\nPlease retry in 32.256763749s.\",\n    \"status\": \"RESOURCE_EXHAUSTED\",\n    \"details\": [\n      {\n        \"@type\": \"type.googleapis.com/google.rpc.Help\",\n        \"links\": [\n          {\n            \"description\": \"Learn more about Gemini API quotas\",\n            \"url\": \"https://ai.google.dev/gemini-api/docs/rate-limits\"\n          }\n        ]\n      },\n      {\n        \"@type\": \"type.googleapis.com/google.rpc.QuotaFailure\",\n        \"violations\": [\n          {\n            \"quotaMetric\": \"generativelanguage.googleapis.com/generate_content_free_tier_requests\",\n            \"quotaId\": \"GenerateRequestsPerDayPerProjectPerModel-FreeTier\",\n            \"quotaDimensions\": {\n              \"location\": \"global\",\n              \"model\": \"gemini-2.5-pro\"\n            }\n          },\n          {\n            \"quotaMetric\": \"generativelanguage.googleapis.com/generate_content_free_tier_requests\",\n            \"quotaId\": \"GenerateRequestsPerMinutePerProjectPerModel-FreeTier\",\n            \"quotaDimensions\": {\n              \"location\": \"global\",\n              \"model\": \"gemini-2.5-pro\"\n            }\n          },\n          {\n            \"quotaMetric\": \"generativelanguage.googleapis.com/generate_content_free_tier_input_token_count\",\n            \"quotaId\": \"GenerateContentInputTokensPerModelPerMinute-FreeTier\",\n            \"quotaDimensions\": {\n              \"location\": \"global\",\n              \"model\": \"gemini-2.5-pro\"\n            }\n          },\n          {\n            \"quotaMetric\": \"generativelanguage.googleapis.com/generate_content_free_tier_input_token_count\",\n            \"quotaId\": \"GenerateContentInputTokensPerModelPerDay-FreeTier\",\n            \"quotaDimensions\": {\n              \"location\": \"global\",\n              \"model\": \"gemini-2.5-pro\"\n            }\n          }\n        ]\n      },\n      {\n        \"@type\": \"type.googleapis.com/google.rpc.RetryInfo\",\n        \"retryDelay\": \"32s\"\n      }\n    ]\n  }\n}\n","code":429,"status":"Too Many Requests"}}
```

**Additional Context**
Please add any additional context about the error here


---

## [Error: Claude Sonnet 4.5 - 402](https://github.com/continuedev/continue/issues/10031) (#10031)
**Created**: 2026-01-30T10:20:34Z

**Error Details**

Model: Claude Sonnet 4.5
Provider: continue-proxy
Status Code: 402

**Error Output**
```
"You have no credits remaining on your Continue account. Please go to https://hub.continue.dev/settings/billing to add credits. You can also add your own ANTHROPIC_API_KEY secret at https://continue.dev/settings/secrets%3FsecretName=ANTHROPIC_API_KEY"
```

**Additional Context**
Please add any additional context about the error here


---

