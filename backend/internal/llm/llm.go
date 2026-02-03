// Package llm provides an OpenAI-compatible API client for LLM interactions.
//
// EXTENSION POINTS:
// - Implement the Client interface to add new LLM providers (e.g., Anthropic, local models)
// - Modify ChatCompletionOptions for provider-specific parameters
// - Add new message types in ChatMessage for multi-modal support
//
// USAGE:
//
//	client := llm.NewOpenAIClient(cfg)
//	response, err := client.ChatCompletion(ctx, messages, nil)
//
//	// Or with streaming:
//	err := client.ChatCompletionStream(ctx, messages, nil, func(chunk string) error {
//	    fmt.Print(chunk)
//	    return nil
//	})
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// INTERFACES - Implement these to add new LLM providers
// ============================================================================

// Client defines the interface for LLM interactions.
// EXTENSION: Implement this interface to add support for new LLM providers.
type Client interface {
	// ChatCompletion performs a non-streaming chat completion request.
	// Returns the assistant's response content.
	ChatCompletion(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (string, error)

	// ChatCompletionStream performs a streaming chat completion request.
	// The callback is invoked for each content chunk received.
	ChatCompletionStream(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) error
}

// StreamCallback is called for each chunk during streaming.
// Return an error to stop the stream.
type StreamCallback func(chunk string) error

// TraceCallback defines callbacks for monitoring LLM interactions.
type TraceCallback struct {
	OnStart      func(ctx context.Context, input []ChatMessage)
	OnFirstToken func(ctx context.Context)
	OnToken      func(ctx context.Context, token string)
	OnComplete   func(ctx context.Context, fullOutput string, err error)
}

// ============================================================================
// DATA TYPES
// ============================================================================

// ChatMessage represents a message in the conversation.
// EXTENSION: Add fields like `images`, `tool_calls` for multi-modal/tool support.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant", "tool"
	Content string `json:"content"` // Message content
	// Provider-specific cache hints (set internally when KV cache is enabled).
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	CachePoint   *CacheControl `json:"cachePoint,omitempty"`

	// Volatile marks messages that must not be treated as part of the stable prefix.
	// It is not serialized to provider payloads; it only guides cache selection.
	Volatile bool `json:"-"`

	// ForceCacheable marks messages that MUST be included in the cacheable selection
	// for providers that require explicit cache markers (e.g., long session summaries).
	ForceCacheable bool `json:"-"`

	// EXTENSION FIELDS (optional, for tool calls):
	Name       string     `json:"name,omitempty"`         // Function name for tool messages
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // Tool calls from assistant
	ToolCallID string     `json:"tool_call_id,omitempty"` // ID for tool response
}

// Tool describes an OpenAI-compatible tool definition.
type Tool struct {
	ID       string       `json:"-"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines a function-style tool.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCall represents a tool invocation returned by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction contains the arguments for a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionOptions configures the chat completion request.
// EXTENSION: Add provider-specific options as needed.
type ChatCompletionOptions struct {
	Model                      string   `json:"model,omitempty"`             // Model override
	Temperature                *float64 `json:"temperature,omitempty"`       // 0.0 to 2.0
	MaxTokens                  *int     `json:"max_tokens,omitempty"`        // Max response tokens
	TopP                       *float64 `json:"top_p,omitempty"`             // Nucleus sampling
	FrequencyPenalty           *float64 `json:"frequency_penalty,omitempty"` // -2.0 to 2.0
	PresencePenalty            *float64 `json:"presence_penalty,omitempty"`  // -2.0 to 2.0
	Stop                       []string `json:"stop,omitempty"`              // Stop sequences
	PromptCacheKey             string   `json:"-"`
	EnablePromptCache          bool     `json:"-"`
	PromptCacheDowngraded      bool     `json:"-"`
	PromptCacheDowngradeReason string   `json:"-"`
	Tools                      []Tool   `json:"-"`
	ToolChoice                 any      `json:"-"`

	// Trace options
	Trace *TraceCallback `json:"-"` // Not sent to API

	// EXTENSION: Add for tool/function calling support.
}

// ============================================================================
// OPENAI-COMPATIBLE CLIENT IMPLEMENTATION
// ============================================================================

// OpenAIClient implements the Client interface for OpenAI-compatible APIs.
// This works with OpenAI, Azure OpenAI, DeepSeek, and other compatible endpoints.
type OpenAIClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
	cacheStyle cacheControlStyle
}

// ClientConfig holds configuration for creating an LLM client.
type ClientConfig struct {
	Endpoint string // API endpoint (e.g., "https://api.openai.com/v1")
	APIKey   string // API key
	Model    string // Default model
	Timeout  time.Duration
}

// NewOpenAIClient creates a new OpenAI-compatible client.
// EXTENSION: Create similar constructors for other providers.
func NewOpenAIClient(cfg ClientConfig) *OpenAIClient {
	return newOpenAIClient(cfg, cacheControlStyleNone)
}

func newOpenAIClient(cfg ClientConfig, cacheStyle cacheControlStyle) *OpenAIClient {
	return &OpenAIClient{
		endpoint:   strings.TrimSuffix(cfg.Endpoint, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: newHTTPClient(cfg.Timeout),
		cacheStyle: cacheStyle,
	}
}

// NewOpenAIClientFromEnv creates a client using standard config structure.
// This is a convenience method for typical usage patterns.
func NewOpenAIClientFromEnv(endpoint, apiKey, model string) *OpenAIClient {
	return NewOpenAIClient(ClientConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}

// chatCompletionRequest is the request body for chat completions.
type chatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Stream           bool          `json:"stream"`
	Temperature      *float64      `json:"temperature,omitempty"`
	MaxTokens        *int          `json:"max_tokens,omitempty"`
	TopP             *float64      `json:"top_p,omitempty"`
	FrequencyPenalty *float64      `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64      `json:"presence_penalty,omitempty"`
	Stop             []string      `json:"stop,omitempty"`
	PromptCacheKey   string        `json:"prompt_cache_key,omitempty"`
	Tools            []Tool        `json:"tools,omitempty"`
	ToolChoice       any           `json:"tool_choice,omitempty"`
}

// chatCompletionResponse is the response for non-streaming requests.
type chatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   *string    `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ChatCompletionResult includes text content plus any tool calls.
type ChatCompletionResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

func parseChatCompletionResult(result chatCompletionResponse) (ChatCompletionResult, error) {
	if result.Error != nil {
		return ChatCompletionResult{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return ChatCompletionResult{}, fmt.Errorf("no choices in response")
	}

	message := result.Choices[0].Message
	content := ""
	if message.Content != nil {
		content = *message.Content
	}

	return ChatCompletionResult{
		Content:      content,
		ToolCalls:    message.ToolCalls,
		FinishReason: result.Choices[0].FinishReason,
	}, nil
}

// streamChunk represents a chunk in the streaming response.
type streamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type streamToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

type streamChunkWithTools struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Role      string                `json:"role,omitempty"`
			Content   string                `json:"content,omitempty"`
			ToolCalls []streamToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ChatCompletion performs a non-streaming chat completion.
func (c *OpenAIClient) ChatCompletion(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (string, error) {
	var fullContent strings.Builder

	if opts != nil && len(opts.Tools) > 0 {
		result, err := c.ChatCompletionStreamWithTools(ctx, messages, opts, func(chunk string) error {
			fullContent.WriteString(chunk)
			return nil
		})
		if err != nil {
			return "", err
		}
		if result.Content != "" {
			return result.Content, nil
		}
		return fullContent.String(), nil
	}

	if err := c.ChatCompletionStream(ctx, messages, opts, func(chunk string) error {
		fullContent.WriteString(chunk)
		return nil
	}); err != nil {
		return "", err
	}

	return fullContent.String(), nil
}

// ChatCompletionWithTools performs a non-streaming chat completion and returns tool calls.
func (c *OpenAIClient) ChatCompletionWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (ChatCompletionResult, error) {
	return c.ChatCompletionStreamWithTools(ctx, messages, opts, func(string) error {
		return nil
	})
}

// ChatCompletionStream performs a streaming chat completion.
// The callback is invoked for each content chunk.
func (c *OpenAIClient) ChatCompletionStream(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) error {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	// Trace: OnStart
	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	var resp *http.Response
	for attempt := 0; attempt < defaultAPIRetryAttempts; attempt++ {
		reqMessages := messages
		if opts != nil && opts.EnablePromptCache && c.cacheStyle != cacheControlStyleNone {
			reqMessages = applyMessageCacheControl(messages, c.cacheStyle)
		}

		defaultMaxTokens := 8192
		reqBody := chatCompletionRequest{
			Model:    model,
			Messages: reqMessages,
			Stream:   true,
		}

		if opts != nil && opts.MaxTokens != nil {
			reqBody.MaxTokens = opts.MaxTokens
		} else {
			reqBody.MaxTokens = &defaultMaxTokens
		}

		if opts != nil {
			reqBody.Temperature = opts.Temperature
			reqBody.TopP = opts.TopP
			reqBody.FrequencyPenalty = opts.FrequencyPenalty
			reqBody.PresencePenalty = opts.PresencePenalty
			reqBody.Stop = opts.Stop
			reqBody.PromptCacheKey = opts.PromptCacheKey
			if len(opts.Tools) > 0 {
				reqBody.Tools = normalizeTools(opts.Tools)
			}
			if opts.ToolChoice != nil {
				reqBody.ToolChoice = opts.ToolChoice
			}
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			err = fmt.Errorf("request failed: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", err)
			}
			return err
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if attempt == 0 && maybeDowngradePromptCaching(opts, resp.StatusCode, respBody) {
			continue
		}

		apiErr := openAIAPIErrorFromResponse(resp, respBody)
		if isRetryableStatus(resp.StatusCode) && attempt < defaultAPIRetryAttempts-1 {
			continue
		}

		err = apiErr
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Some OpenAI-compatible providers ignore stream=true and return a regular JSON payload.
	// Detect that case early to avoid silently returning an empty string.
	var prefixLines []string
	firstNonEmpty := ""
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			prefixLines = append(prefixLines, line)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			firstNonEmpty = trimmed
			if err == nil || err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", readErr)
			}
			return readErr
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", readErr)
			}
			return readErr
		}
	}

	if firstNonEmpty != "" && !looksLikeSSELine(firstNonEmpty) {
		rest, _ := io.ReadAll(reader)
		raw := []byte(strings.Join(prefixLines, ""))
		raw = append(raw, rest...)

		var result chatCompletionResponse
		if err := json.Unmarshal(raw, &result); err != nil {
			parseErr := fmt.Errorf("failed to decode chat completion response: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", parseErr)
			}
			return parseErr
		}

		parsed, err := parseChatCompletionResult(result)
		if err != nil {
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", err)
			}
			return err
		}
		content := parsed.Content
		if strings.TrimSpace(content) == "" {
			err := fmt.Errorf("empty completion content")
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", err)
			}
			return err
		}

		if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
			opts.Trace.OnFirstToken(ctx)
		}
		if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
			opts.Trace.OnToken(ctx, content)
		}
		if callback != nil {
			if err := callback(content); err != nil {
				if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
					opts.Trace.OnComplete(ctx, content, err)
				}
				return err
			}
		}
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, content, nil)
		}
		return nil
	}

	var (
		fullContent        strings.Builder
		firstTokenReceived bool
	)

	processLine := func(line string) (bool, error) {
		line = strings.TrimSpace(line)
		if line == "" {
			return false, nil
		}

		data, ok := extractSSEDataLine(line)
		if !ok {
			return false, nil
		}
		if data == "[DONE]" {
			return true, nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks.
			return false, nil
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if !firstTokenReceived {
				firstTokenReceived = true
				if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
					opts.Trace.OnFirstToken(ctx)
				}
			}

			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)

			if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
				opts.Trace.OnToken(ctx, content)
			}

			if callback != nil {
				if err := callback(content); err != nil {
					if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
						opts.Trace.OnComplete(ctx, fullContent.String(), err)
					}
					return false, err
				}
			}
		}

		return false, nil
	}

	done := false
	for _, line := range prefixLines {
		var err error
		done, err = processLine(line)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}

	for !done {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, fullContent.String(), readErr)
			}
			return readErr
		}

		done, err = processLine(line)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(fullContent.String()) == "" {
		err := fmt.Errorf("empty completion stream")
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return err
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		opts.Trace.OnComplete(ctx, fullContent.String(), nil)
	}

	return nil
}

// ChatCompletionStreamWithTools performs a streaming chat completion and returns tool calls.
func (c *OpenAIClient) ChatCompletionStreamWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) (ChatCompletionResult, error) {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	var resp *http.Response
	for attempt := 0; attempt < defaultAPIRetryAttempts; attempt++ {
		reqMessages := messages
		if opts != nil && opts.EnablePromptCache && c.cacheStyle != cacheControlStyleNone {
			reqMessages = applyMessageCacheControl(messages, c.cacheStyle)
		}

		defaultMaxTokens := 8192
		reqBody := chatCompletionRequest{
			Model:    model,
			Messages: reqMessages,
			Stream:   true,
		}

		if opts != nil && opts.MaxTokens != nil {
			reqBody.MaxTokens = opts.MaxTokens
		} else {
			reqBody.MaxTokens = &defaultMaxTokens
		}

		if opts != nil {
			reqBody.Temperature = opts.Temperature
			reqBody.TopP = opts.TopP
			reqBody.FrequencyPenalty = opts.FrequencyPenalty
			reqBody.PresencePenalty = opts.PresencePenalty
			reqBody.Stop = opts.Stop
			reqBody.PromptCacheKey = opts.PromptCacheKey
			if len(opts.Tools) > 0 {
				reqBody.Tools = normalizeTools(opts.Tools)
			}
			if opts.ToolChoice != nil {
				reqBody.ToolChoice = opts.ToolChoice
			}
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return ChatCompletionResult{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return ChatCompletionResult{}, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			err = fmt.Errorf("request failed: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", err)
			}
			return ChatCompletionResult{}, err
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if attempt == 0 && maybeDowngradePromptCaching(opts, resp.StatusCode, respBody) {
			continue
		}

		apiErr := openAIAPIErrorFromResponse(resp, respBody)
		if isRetryableStatus(resp.StatusCode) && attempt < defaultAPIRetryAttempts-1 {
			continue
		}

		err = apiErr
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	var prefixLines []string
	firstNonEmpty := ""
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			prefixLines = append(prefixLines, line)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			firstNonEmpty = trimmed
			if err == nil || err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", readErr)
			}
			return ChatCompletionResult{}, readErr
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", readErr)
			}
			return ChatCompletionResult{}, readErr
		}
	}

	if firstNonEmpty != "" && !looksLikeSSELine(firstNonEmpty) {
		rest, _ := io.ReadAll(reader)
		raw := []byte(strings.Join(prefixLines, ""))
		raw = append(raw, rest...)

		var result chatCompletionResponse
		if err := json.Unmarshal(raw, &result); err != nil {
			parseErr := fmt.Errorf("failed to decode chat completion response: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", parseErr)
			}
			return ChatCompletionResult{}, parseErr
		}

		parsed, err := parseChatCompletionResult(result)
		if err != nil {
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, "", err)
			}
			return ChatCompletionResult{}, err
		}

		if parsed.Content != "" {
			if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
				opts.Trace.OnFirstToken(ctx)
			}
			if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
				opts.Trace.OnToken(ctx, parsed.Content)
			}
			if callback != nil {
				if err := callback(parsed.Content); err != nil {
					if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
						opts.Trace.OnComplete(ctx, parsed.Content, err)
					}
					return ChatCompletionResult{}, err
				}
			}
		}

		finalCalls := make([]ToolCall, 0, len(parsed.ToolCalls))
		for _, call := range parsed.ToolCalls {
			call.Function.Arguments = SanitizeToolArgumentsJSON(normalizeToolArguments(call.Function.Arguments))
			finalCalls = append(finalCalls, call)
		}

		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, parsed.Content, nil)
		}
		return ChatCompletionResult{
			Content:      parsed.Content,
			ToolCalls:    finalCalls,
			FinishReason: parsed.FinishReason,
		}, nil
	}

	var fullContent strings.Builder
	var firstTokenReceived bool
	toolCalls := make(map[int]*ToolCall)
	toolArgs := make(map[int]*strings.Builder)
	finishReason := ""
	mergeToolArgs := func(builder *strings.Builder, fragment string) string {
		if fragment == "" {
			return ""
		}
		if builder.Len() == 0 {
			builder.WriteString(fragment)
			return fragment
		}

		// Some OpenAI-compatible providers stream the full JSON-so-far each time instead of deltas.
		// If the fragment already contains the entire accumulated value as a prefix, treat it as a replacement.
		current := builder.String()
		if strings.HasPrefix(fragment, current) {
			if len(fragment) == len(current) {
				return ""
			}
			delta := fragment[len(current):]
			builder.Reset()
			builder.WriteString(fragment)
			return delta
		}

		builder.WriteString(fragment)
		return fragment
	}

	processLine := func(line string) (bool, error) {
		line = strings.TrimSpace(line)
		if line == "" {
			return false, nil
		}

		data, ok := extractSSEDataLine(line)
		if !ok {
			return false, nil
		}
		if data == "[DONE]" {
			return true, nil
		}

		var chunk streamChunkWithTools
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return false, nil
		}
		if len(chunk.Choices) == 0 {
			return false, nil
		}

		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			if !firstTokenReceived {
				firstTokenReceived = true
				if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
					opts.Trace.OnFirstToken(ctx)
				}
			}

			fullContent.WriteString(choice.Delta.Content)

			if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
				opts.Trace.OnToken(ctx, choice.Delta.Content)
			}

			if callback != nil {
				if err := callback(choice.Delta.Content); err != nil {
					if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
						opts.Trace.OnComplete(ctx, fullContent.String(), err)
					}
					return false, err
				}
			}
		}

		for _, delta := range choice.Delta.ToolCalls {
			acc, ok := toolCalls[delta.Index]
			if !ok {
				acc = &ToolCall{
					Type: delta.Type,
					Function: ToolCallFunction{
						Name: delta.Function.Name,
					},
				}
				toolCalls[delta.Index] = acc
			}
			if delta.ID != "" {
				acc.ID = delta.ID
			}
			if delta.Type != "" {
				acc.Type = delta.Type
			}
			if delta.Function.Name != "" {
				acc.Function.Name = delta.Function.Name
			}
			if delta.Function.Arguments != "" {
				builder, ok := toolArgs[delta.Index]
				if !ok {
					builder = &strings.Builder{}
					toolArgs[delta.Index] = builder
				}
				added := mergeToolArgs(builder, delta.Function.Arguments)

				if added != "" && opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
					opts.Trace.OnToken(ctx, added)
				}
			}
		}

		if choice.FinishReason != nil && *choice.FinishReason != "" {
			finishReason = *choice.FinishReason
		}

		return false, nil
	}

	done := false
	for _, line := range prefixLines {
		var err error
		done, err = processLine(line)
		if err != nil {
			return ChatCompletionResult{}, err
		}
		if done {
			break
		}
	}

	for !done {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			readErr := fmt.Errorf("stream read error: %w", err)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, fullContent.String(), readErr)
			}
			return ChatCompletionResult{}, readErr
		}

		done, err = processLine(line)
		if err != nil {
			return ChatCompletionResult{}, err
		}
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		opts.Trace.OnComplete(ctx, fullContent.String(), nil)
	}

	indices := make([]int, 0, len(toolCalls))
	for idx := range toolCalls {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	finalCalls := make([]ToolCall, 0, len(indices))
	for _, idx := range indices {
		call := toolCalls[idx]
		if call == nil {
			continue
		}
		if builder, ok := toolArgs[idx]; ok {
			call.Function.Arguments = SanitizeToolArgumentsJSON(normalizeToolArguments(builder.String()))
		} else {
			call.Function.Arguments = SanitizeToolArgumentsJSON(normalizeToolArguments(call.Function.Arguments))
		}
		finalCalls = append(finalCalls, *call)
	}

	return ChatCompletionResult{
		Content:      fullContent.String(),
		ToolCalls:    finalCalls,
		FinishReason: finishReason,
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// BuildSystemMessage creates a system message.
func BuildSystemMessage(content string) ChatMessage {
	return ChatMessage{Role: "system", Content: content}
}

// BuildUserMessage creates a user message.
func BuildUserMessage(content string) ChatMessage {
	return ChatMessage{Role: "user", Content: content}
}

// BuildAssistantMessage creates an assistant message.
func BuildAssistantMessage(content string) ChatMessage {
	return ChatMessage{Role: "assistant", Content: content}
}

func normalizeToolArguments(args string) string {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return ""
	}

	// Special-case: models sometimes double-quote the entire JSON payload, which becomes a JSON string.
	if json.Valid([]byte(trimmed)) {
		if strings.HasPrefix(trimmed, "\"") {
			var unquoted string
			if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
				unquoted = strings.TrimSpace(unquoted)
				if unquoted != "" && (strings.HasPrefix(unquoted, "{") || strings.HasPrefix(unquoted, "[")) && json.Valid([]byte(unquoted)) {
					return unquoted
				}
			}
		}
		return trimmed
	}

	if unfenced, ok := unwrapCodeFence(trimmed); ok {
		trimmed = strings.TrimSpace(unfenced)
		if trimmed == "" {
			return ""
		}
		// Re-run normalization on the unfenced payload (it may itself be quoted JSON).
		return normalizeToolArguments(trimmed)
	}

	// Best-effort: extract the last valid JSON value from noisy output.
	if extracted, ok := extractLastJSONValue(trimmed); ok {
		return extracted
	}

	return trimmed
}

func unwrapCodeFence(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return "", false
	}
	close := strings.LastIndex(s, "```")
	if close <= 0 {
		return "", false
	}

	start := 3
	if nl := strings.IndexByte(s, '\n'); nl != -1 {
		start = nl + 1
	} else {
		for start < len(s) && s[start] != ' ' && s[start] != '\t' && s[start] != '\r' && s[start] != '\n' {
			start++
		}
		for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
			start++
		}
	}

	if start >= close {
		return "", true
	}
	return s[start:close], true
}

func extractLastJSONValue(s string) (string, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != '{' && s[i] != '[' {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(s[i:]))
		var v any
		if err := dec.Decode(&v); err != nil {
			continue
		}
		end := i + int(dec.InputOffset())
		if end <= i {
			continue
		}
		candidate := strings.TrimSpace(s[i:end])
		if candidate == "" {
			continue
		}
		if json.Valid([]byte(candidate)) {
			return candidate, true
		}
	}
	return "", false
}
