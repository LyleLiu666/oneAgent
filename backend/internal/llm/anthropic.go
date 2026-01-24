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
)

// AnthropicClient implements the Client interface for Claude (Anthropic).
type AnthropicClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(cfg ClientConfig) *AnthropicClient {
	return &AnthropicClient{
		endpoint:   strings.TrimSuffix(cfg.Endpoint, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: newHTTPClient(cfg.Timeout),
	}
}

type anthropicCacheControl struct {
	Type string `json:"type"`
}

type anthropicContent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Input        map[string]any         `json:"input,omitempty"`
	ToolUseID    string                 `json:"tool_use_id,omitempty"`
	Content      string                 `json:"content,omitempty"`
	IsError      bool                   `json:"is_error,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	System        []anthropicContent `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    any                `json:"tool_choice,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type anthropicResponse struct {
	Type    string             `json:"type,omitempty"`
	Content []anthropicContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type anthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type anthropicStreamContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicStreamEvent struct {
	Type         string                       `json:"type"`
	Delta        *anthropicStreamDelta        `json:"delta,omitempty"`
	ContentBlock *anthropicStreamContentBlock `json:"content_block,omitempty"`
	Index        *int                         `json:"index,omitempty"`
	Error        *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ChatCompletion performs a non-streaming completion with Claude.
func (c *AnthropicClient) ChatCompletion(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (string, error) {
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

// ChatCompletionWithTools performs a non-streaming completion and returns tool calls.
func (c *AnthropicClient) ChatCompletionWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (ChatCompletionResult, error) {
	return c.ChatCompletionStreamWithTools(ctx, messages, opts, func(string) error {
		return nil
	})
}

// ChatCompletionStream performs a streaming completion with Claude.
func (c *AnthropicClient) ChatCompletionStream(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) error {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	system, anthropicMessages := buildAnthropicPayload(messages, opts != nil && opts.EnablePromptCache)
	maxTokens := 8192
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}

	reqBody := anthropicRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		Messages:      anthropicMessages,
		System:        system,
		Temperature:   nil,
		TopP:          nil,
		StopSequences: nil,
		Stream:        true,
	}
	if opts != nil {
		reqBody.Temperature = opts.Temperature
		reqBody.TopP = opts.TopP
		reqBody.StopSequences = opts.Stop
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	if opts != nil && opts.EnablePromptCache {
		req.Header.Set("anthropic-beta", "prompt-caching")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("request failed: %w", err)
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return err
	}

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	var firstTokenReceived bool

	for {
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

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "error" && event.Error != nil {
			err = fmt.Errorf("API error: %s", event.Error.Message)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, fullContent.String(), err)
			}
			return err
		}

		chunk := ""
		if event.Delta != nil && event.Delta.Text != "" {
			chunk = event.Delta.Text
		} else if event.ContentBlock != nil && event.ContentBlock.Text != "" {
			chunk = event.ContentBlock.Text
		}

		if chunk != "" {
			if !firstTokenReceived {
				firstTokenReceived = true
				if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
					opts.Trace.OnFirstToken(ctx)
				}
			}

			fullContent.WriteString(chunk)
			if err := callback(chunk); err != nil {
				if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
					opts.Trace.OnComplete(ctx, fullContent.String(), err)
				}
				return err
			}
		}
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		opts.Trace.OnComplete(ctx, fullContent.String(), nil)
	}

	return nil
}

// ChatCompletionStreamWithTools performs a streaming completion and returns tool calls.
func (c *AnthropicClient) ChatCompletionStreamWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) (ChatCompletionResult, error) {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	system, anthropicMessages := buildAnthropicPayload(messages, opts != nil && opts.EnablePromptCache)
	maxTokens := 8192
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}

	reqBody := anthropicRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		Messages:      anthropicMessages,
		System:        system,
		Temperature:   nil,
		TopP:          nil,
		StopSequences: nil,
		Stream:        true,
	}
	if opts != nil {
		reqBody.Temperature = opts.Temperature
		reqBody.TopP = opts.TopP
		reqBody.StopSequences = opts.Stop
		if len(opts.Tools) > 0 {
			reqBody.Tools = buildAnthropicTools(opts.Tools)
		}
		if opts.ToolChoice != nil {
			reqBody.ToolChoice = opts.ToolChoice
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ChatCompletionResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	if opts != nil && opts.EnablePromptCache {
		req.Header.Set("anthropic-beta", "prompt-caching")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("request failed: %w", err)
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	var firstTokenReceived bool
	toolCalls := make(map[int]*ToolCall)
	toolArgs := make(map[int]*strings.Builder)
	mergeToolArgs := func(builder *strings.Builder, fragment string) {
		if fragment == "" {
			return
		}
		if builder.Len() == 0 {
			builder.WriteString(fragment)
			return
		}

		// Be tolerant if a provider sends full JSON-so-far chunks instead of incremental deltas.
		current := builder.String()
		if strings.HasPrefix(fragment, current) {
			if len(fragment) == len(current) {
				return
			}
			builder.Reset()
			builder.WriteString(fragment)
			return
		}

		builder.WriteString(fragment)
	}

	for {
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

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Error != nil {
			err = fmt.Errorf("API error: %s", event.Error.Message)
			if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
				opts.Trace.OnComplete(ctx, fullContent.String(), err)
			}
			return ChatCompletionResult{}, err
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock == nil || event.Index == nil {
				continue
			}
			if event.ContentBlock.Type == "tool_use" {
				index := *event.Index
				toolCalls[index] = &ToolCall{
					ID:   event.ContentBlock.ID,
					Type: "function",
					Function: ToolCallFunction{
						Name: event.ContentBlock.Name,
					},
				}
				if event.ContentBlock.Input != nil {
					args, err := json.Marshal(event.ContentBlock.Input)
					if err == nil {
						builder := &strings.Builder{}
						builder.Write(args)
						toolArgs[index] = builder
					}
				}
			}
		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text == "" {
					continue
				}
				if !firstTokenReceived {
					firstTokenReceived = true
					if opts != nil && opts.Trace != nil && opts.Trace.OnFirstToken != nil {
						opts.Trace.OnFirstToken(ctx)
					}
				}
				fullContent.WriteString(event.Delta.Text)

				// Trace: OnToken
				if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
					opts.Trace.OnToken(ctx, event.Delta.Text)
				}

				if err := callback(event.Delta.Text); err != nil {
					if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
						opts.Trace.OnComplete(ctx, fullContent.String(), err)
					}
					return ChatCompletionResult{}, err
				}
			case "input_json_delta":
				if event.Index == nil || event.Delta.PartialJSON == "" {
					continue
				}
				builder, ok := toolArgs[*event.Index]
				if !ok {
					builder = &strings.Builder{}
					toolArgs[*event.Index] = builder
				}
				mergeToolArgs(builder, event.Delta.PartialJSON)

				// Trace: OnToken for tool call arguments
				if opts != nil && opts.Trace != nil && opts.Trace.OnToken != nil {
					opts.Trace.OnToken(ctx, event.Delta.PartialJSON)
				}
			}
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
			call.Function.Arguments = normalizeToolArguments(builder.String())
		} else {
			call.Function.Arguments = normalizeToolArguments(call.Function.Arguments)
		}
		if strings.TrimSpace(call.Function.Arguments) == "" {
			call.Function.Arguments = "{}"
		}
		finalCalls = append(finalCalls, *call)
	}

	return ChatCompletionResult{
		Content:   fullContent.String(),
		ToolCalls: finalCalls,
	}, nil
}

func buildAnthropicPayload(messages []ChatMessage, enableCache bool) ([]anthropicContent, []anthropicMessage) {
	cacheIndexes := map[int]bool{}
	if enableCache {
		cacheIndexes = cacheMessageIndexes(messages)
	}

	system := make([]anthropicContent, 0)
	converted := make([]anthropicMessage, 0)

	for idx, msg := range messages {
		if msg.Role == "system" {
			block := anthropicContent{
				Type: "text",
				Text: msg.Content,
			}
			if enableCache && cacheIndexes[idx] {
				block.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
			}
			system = append(system, block)
			continue
		}

		switch msg.Role {
		case "assistant":
			blocks := make([]anthropicContent, 0, 1+len(msg.ToolCalls))
			if msg.Content != "" {
				block := anthropicContent{
					Type: "text",
					Text: msg.Content,
				}
				if enableCache && cacheIndexes[idx] {
					block.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
				}
				blocks = append(blocks, block)
			}

			for _, call := range msg.ToolCalls {
				blocks = append(blocks, anthropicContent{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: parseToolInput(call.Function.Arguments),
				})
			}

			converted = append(converted, anthropicMessage{
				Role:    "assistant",
				Content: blocks,
			})
		case "tool":
			converted = append(converted, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
		default:
			block := anthropicContent{
				Type: "text",
				Text: msg.Content,
			}
			if enableCache && cacheIndexes[idx] {
				block.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
			}
			converted = append(converted, anthropicMessage{
				Role:    msg.Role,
				Content: []anthropicContent{block},
			})
		}
	}

	return system, converted
}

func buildAnthropicTools(tools []Tool) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	return out
}

func parseToolInput(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return map[string]any{"_raw": raw}
	}
	if input == nil {
		return map[string]any{}
	}
	return input
}

func extractAnthropicContent(blocks []anthropicContent) (string, []ToolCall) {
	var contentBuilder strings.Builder
	toolCalls := make([]ToolCall, 0)

	for _, block := range blocks {
		switch block.Type {
		case "text":
			contentBuilder.WriteString(block.Text)
		case "tool_use":
			args, err := json.Marshal(block.Input)
			if err != nil {
				args = []byte("{}")
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(args),
				},
			})
		}
	}

	return contentBuilder.String(), toolCalls
}
