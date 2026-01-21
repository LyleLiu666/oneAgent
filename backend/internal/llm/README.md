# LLM Package

OpenAI-compatible LLM client for the base application.

## Quick Start

```go
import "github.com/liu_y/oneAgent/backend/internal/llm"

// Create client
client := llm.NewOpenAIClientFromEnv(
    "https://api.openai.com/v1",  // or DeepSeek, Azure, etc.
    "sk-your-api-key",
    "gpt-4",
)

// Non-streaming
response, err := client.ChatCompletion(ctx, []llm.ChatMessage{
    llm.BuildSystemMessage("You are helpful."),
    llm.BuildUserMessage("Hello!"),
}, nil)

// Streaming
err := client.ChatCompletionStream(ctx, messages, nil, func(chunk string) error {
    fmt.Print(chunk)
    return nil
})
```

## Extension Points

1. **New Providers**: Implement `llm.Client` interface
2. **Tool Calling**: Extend `ChatMessage` with ToolCalls field
3. **Custom Options**: Add fields to `ChatCompletionOptions`

## Configuration

LLM providers and models are now configured through the Settings UI in your
application. Navigate to Settings → LLM Providers to add new providers and
models.

## Prompt caching (KV cache)

KV cache behavior is provider-specific and is toggled per model via
`enable_kv_cache` in the Settings UI.

- Claude (Anthropic): marks the first two system messages and the most recent
  two messages with `cache_control: { type: "ephemeral" }`, and adds the
  `anthropic-beta: prompt-caching` header.
- OpenAI Chat Completions / Responses, ZhipuAI, MiniMax, Antigravity, Codex,
  DeepSeek: set `prompt_cache_key` to the session ID when KV cache is enabled.
- OpenRouter: marks the first two system messages and the most recent two
  messages with `cache_control: { type: "ephemeral" }`.
- Bedrock: marks the first two system messages and the most recent two messages
  with `cachePoint: { type: "ephemeral" }`.

Some providers also perform automatic prefix caching; the session cache key is
still sent for consistency when supported.

Best practices:

- Keep system prompts stable across a session.
- Put volatile context in user messages to maximize cache hits.
