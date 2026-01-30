# Dify Top Issues (Open & High Reactions)

Fetched at: Fri Jan 30 22:32:30 CST 2026
Repo: https://github.com/langgenius/dify

## [Support Manual Input of <thinking>...</thinking> Tags in Output Node](https://github.com/langgenius/dify/issues/31774) (#31774)
**Created**: 2026-01-30T12:40:12Z

### Self Checks

- [x] I have read the [Contributing Guide](https://github.com/langgenius/dify/blob/main/CONTRIBUTING.md) and [Language Policy](https://github.com/langgenius/dify/issues/1542).
- [x] I have searched for existing issues [search for existing issues](https://github.com/langgenius/dify/issues), including closed ones.
- [x] I confirm that I am using English to submit this report, otherwise it will be closed.
- [x] Please do not modify this template :) and fill in all the required fields.

### 1. Is this request related to a challenge you're experiencing? Tell me about your story.

Dify currently parses tags from large model outputs to display content in "thinking" style, but users cannot manually input <thinking>...</thinking> tags in the output node to distinguish between "thinking" content and normal conversational content. So:
Add a feature to the Dify output node to:
1. Allow users to manually input content in the format: <thinking>thinking content</thinking> normal content
2. Dify will parse the <thinking>...</thinking> tags like it does for large model outputs, displaying the content inside the tags in "thinking" style and the content outside in normal conversational style.


### 2. Additional context or comments

Example
**Input:**
```

< thinking > 
1. Query cargo 12345 status 
2. Retrieve data from PostgreSQL 
3. Verify data integrity 
< /thinking > 
Cargo 12345 is delivered as of 2024-05-10.

```

Expected Output:

- Content inside <thinking>...</thinking>: displayed in "thinking" style

- Content outside: displayed in normal conversational style

### 3. Can you help us with this feature?

- [x] I am interested in contributing to this feature.

---

## [output.type.slice(...).toLocaleUpperCase is not a function](https://github.com/langgenius/dify/issues/31773) (#31773)
**Created**: 2026-01-30T12:31:39Z

### Self Checks

- [x] I have read the [Contributing Guide](https://github.com/langgenius/dify/blob/main/CONTRIBUTING.md) and [Language Policy](https://github.com/langgenius/dify/issues/1542).
- [x] This is only for bug report, if you would like to ask a question, please head to [Discussions](https://github.com/langgenius/dify/discussions/categories/general).
- [x] I have searched for existing issues [search for existing issues](https://github.com/langgenius/dify/issues), including closed ones.
- [x] I confirm that I am using English to submit this report, otherwise it will be closed.
- [x] 【中文用户 & Non English User】请使用英语提交，否则会被关闭 ：）
- [x] Please do not modify this template :) and fill in all the required fields.

### Dify version

1.11.4

### Cloud or Self Hosted

Self Hosted (Docker)

### Steps to reproduce

1. create mcp tool，mcp tool output schema has union type
2. create workflow and add mcp tool, it broken

<img width="1078" height="1154" alt="Image" src="https://github.com/user-attachments/assets/c7647376-a150-4936-bf91-13df7b73b8a6" />

### ✔️ Expected Behavior

no error

### ❌ Actual Behavior

<img width="1078" height="1154" alt="Image" src="https://github.com/user-attachments/assets/060b9782-e47a-4e65-96c2-623a91ecc2bf" />

---

## [[Tracker] Issues of Human in the Loop feature](https://github.com/langgenius/dify/issues/31765) (#31765)
**Created**: 2026-01-30T10:56:08Z

- [ ] #31648:
- [ ] #31749: reverted
- [ ] #31631: reverted
- [ ] #31646: reverted
- [ ] Potential High CPU usage caused by HITL code

---

## [Bug: server identifier returned instead of provider ID in console API `workspaces/current/tools/mcp`](https://github.com/langgenius/dify/issues/31763) (#31763)
**Created**: 2026-01-30T10:40:25Z

### Self Checks

- [x] I have read the [Contributing Guide](https://github.com/langgenius/dify/blob/main/CONTRIBUTING.md) and [Language Policy](https://github.com/langgenius/dify/issues/1542).
- [x] This is only for bug report, if you would like to ask a question, please head to [Discussions](https://github.com/langgenius/dify/discussions/categories/general).
- [x] I have searched for existing issues [search for existing issues](https://github.com/langgenius/dify/issues), including closed ones.
- [x] I confirm that I am using English to submit this report, otherwise it will be closed.
- [x] 【中文用户 & Non English User】请使用英语提交，否则会被关闭 ：）
- [x] Please do not modify this template :) and fill in all the required fields.

### Dify version

1.11.3

### Cloud or Self Hosted

Self Hosted (Source)

### Steps to reproduce

1. Call console API for the list of MCP servers `/console/api/workspaces/current/tools/mcp`.
2. Inspect the response, notice that on every single entry field `id` is the same as field `server_identifier`.
3. Inspect the file class `ToolMCPListAllApi`, notice that the function `list_providers` always return object where ID is `server_identifier`.

<img width="1886" height="2337" alt="Image" src="https://github.com/user-attachments/assets/1b16df31-8ffc-4673-bfad-7839f94926dc" />

### ✔️ Expected Behavior

**The `id` field in each MCP server entry should be the provider ID.**

Otherwise we cannot make use of other APIs such as `/workspaces/current/tool-provider/mcp/update/<path:provider_id>`, since we have no way to obtain the provider ID (without removing and adding the MCP server instance).

### ❌ Actual Behavior

- The `id` and `server_identifier` fields are effectively the same.
- We have no way of obtaining the provider ID without removing and re-adding MCP servers.
- As such we cannot easily make use of related APIs that depends on provider ID.

---

## [The same content can be analyzed in the workflow's LLM, but in the ReAct mode (using the same model), it often results in a "data too long" error.](https://github.com/langgenius/dify/issues/31762) (#31762)
**Created**: 2026-01-30T10:14:32Z

### Self Checks

- [x] I have read the [Contributing Guide](https://github.com/langgenius/dify/blob/main/CONTRIBUTING.md) and [Language Policy](https://github.com/langgenius/dify/issues/1542).
- [x] This is only for bug report, if you would like to ask a question, please head to [Discussions](https://github.com/langgenius/dify/discussions/categories/general).
- [x] I have searched for existing issues [search for existing issues](https://github.com/langgenius/dify/issues), including closed ones.
- [x] I confirm that I am using English to submit this report, otherwise it will be closed.
- [x] 【中文用户 & Non English User】请使用英语提交，否则会被关闭 ：）
- [x] Please do not modify this template :) and fill in all the required fields.

### Dify version

v1.11.4

### Cloud or Self Hosted

Self Hosted (Docker)

### Steps to reproduce

NONE

### ✔️ Expected Behavior

I hope the output from the workflow and the output from the ReAct mode are consistent.

### ❌ Actual Behavior

These images all pertain to the same issue, and the tool has returned identical data in each case.

Workflow LLM output:

<img width="2090" height="1392" alt="Image" src="https://github.com/user-attachments/assets/67e0e2e5-1dcc-4f7f-9bff-7433a2870408" />

ReAct output(Unstable)：
success_output:

<img width="2882" height="1776" alt="Image" src="https://github.com/user-attachments/assets/ecbe0ee3-df8c-45c4-b6f6-cb0e326cab53" />

data too long:

<img width="2886" height="1358" alt="Image" src="https://github.com/user-attachments/assets/f9b98594-d0cf-4f93-a8e8-dfeff5b9cec1" />

---

