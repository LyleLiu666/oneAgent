package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIResponsesClient implements the Client interface for OpenAI Responses API.
type OpenAIResponsesClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIResponsesClient creates a new client for the Responses API.
func NewOpenAIResponsesClient(cfg ClientConfig) *OpenAIResponsesClient {
	return &OpenAIResponsesClient{
		endpoint:   strings.TrimSuffix(cfg.Endpoint, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: newHTTPClient(cfg.Timeout),
	}
}

type responsesMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesFunctionCall struct {
	Type      string `json:"type"` // "function_call"
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type responsesFunctionCallOutput struct {
	Type   string `json:"type"` // "function_call_output"
	CallID string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type responsesRequest struct {
	Model           string          `json:"model"`
	Input           any             `json:"input,omitempty"`
	Instructions    string          `json:"instructions,omitempty"`
	Stream          bool            `json:"stream"`
	Temperature     *float64        `json:"temperature,omitempty"`
	MaxOutputTokens *int            `json:"max_output_tokens,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	PromptCacheKey  string          `json:"prompt_cache_key,omitempty"`
	Tools           []responsesTool `json:"tools,omitempty"`
	ToolChoice      any             `json:"tool_choice,omitempty"`
}

type responsesResponse struct {
	ID     string `json:"id,omitempty"`
	Output []struct {
		Type      string `json:"type"`
		Role      string `json:"role,omitempty"`
		Name      string `json:"name,omitempty"`
		CallID    string `json:"call_id,omitempty"`
		Arguments string `json:"arguments,omitempty"`
		Content   []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func normalizeResponsesTools(tools []Tool) []responsesTool {
	ordered := normalizeTools(tools)
	if len(ordered) == 0 {
		return nil
	}

	out := make([]responsesTool, 0, len(ordered))
	for _, tool := range ordered {
		if strings.TrimSpace(tool.Type) != "function" {
			continue
		}
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			continue
		}
		out = append(out, responsesTool{
			Type:        "function",
			Name:        name,
			Description: strings.TrimSpace(tool.Function.Description),
			Parameters:  tool.Function.Parameters,
		})
	}
	return out
}

// ChatCompletion performs a non-streaming response request.
func (c *OpenAIResponsesClient) ChatCompletion(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (string, error) {
	var fullContent strings.Builder
	if err := c.ChatCompletionStream(ctx, messages, opts, func(chunk string) error {
		fullContent.WriteString(chunk)
		return nil
	}); err != nil {
		return "", err
	}
	return fullContent.String(), nil
}

// ChatCompletionStream performs a streaming response request.
func (c *OpenAIResponsesClient) ChatCompletionStream(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) error {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	defaultMaxTokens := 8192
	instructions, input := buildResponsesInstructionsAndInput(messages)
	reqBody := responsesRequest{
		Model:        model,
		Input:        input,
		Instructions: instructions,
		Stream:       true,
	}
	if opts != nil && opts.MaxTokens != nil {
		reqBody.MaxOutputTokens = opts.MaxTokens
	} else {
		reqBody.MaxOutputTokens = &defaultMaxTokens
	}
	if opts != nil {
		reqBody.Temperature = opts.Temperature
		reqBody.TopP = opts.TopP
		reqBody.PromptCacheKey = opts.PromptCacheKey
		if len(opts.Tools) > 0 {
			reqBody.Tools = normalizeResponsesTools(opts.Tools)
		}
		if opts.ToolChoice != nil {
			reqBody.ToolChoice = opts.ToolChoice
		}
	}

	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/responses", bytes.NewReader(body))
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
			reqBody.PromptCacheKey = ""
			continue
		}

		err = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return err
	}
	defer resp.Body.Close()

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
		if line == "" {
			continue
		}

		data, ok := extractSSEDataLine(line)
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}

		chunk := extractResponsesStreamChunk(data)
		if chunk == "" {
			continue
		}

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

	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		opts.Trace.OnComplete(ctx, fullContent.String(), nil)
	}

	return nil
}

func (c *OpenAIResponsesClient) ChatCompletionWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (ChatCompletionResult, error) {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	defaultMaxTokens := 8192
	instructions, input := buildResponsesInstructionsAndInput(messages)
	reqBody := responsesRequest{
		Model:        model,
		Input:        input,
		Instructions: instructions,
		Stream:       false,
	}
	if opts != nil && opts.MaxTokens != nil {
		reqBody.MaxOutputTokens = opts.MaxTokens
	} else {
		reqBody.MaxOutputTokens = &defaultMaxTokens
	}
	if opts != nil {
		reqBody.Temperature = opts.Temperature
		reqBody.TopP = opts.TopP
		reqBody.PromptCacheKey = opts.PromptCacheKey
		if len(opts.Tools) > 0 {
			reqBody.Tools = normalizeResponsesTools(opts.Tools)
		}
		if opts.ToolChoice != nil {
			reqBody.ToolChoice = opts.ToolChoice
		}
	}

	var resp *http.Response
	var body []byte
	for attempt := 0; attempt < 2; attempt++ {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return ChatCompletionResult{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/responses", bytes.NewReader(payload))
		if err != nil {
			return ChatCompletionResult{}, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
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

		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))

		if resp.StatusCode == http.StatusOK {
			break
		}

		if attempt == 0 && maybeDowngradePromptCaching(opts, resp.StatusCode, body) {
			reqBody.PromptCacheKey = ""
			continue
		}

		err = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}

	var result responsesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		err = fmt.Errorf("failed to decode response: %w", err)
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}
	if result.Error != nil {
		err := fmt.Errorf("API error: %s", result.Error.Message)
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return ChatCompletionResult{}, err
	}

	content, toolCalls := extractResponsesContentAndToolCalls(result)
	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		opts.Trace.OnComplete(ctx, content, nil)
	}
	return ChatCompletionResult{
		Content:   content,
		ToolCalls: toolCalls,
	}, nil
}

func (c *OpenAIResponsesClient) ChatCompletionStreamWithTools(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions, callback StreamCallback) (ChatCompletionResult, error) {
	// Best-effort downgrade: stream=true + tools is not fully implemented yet for Responses.
	// Run a non-stream request and emit the full content once.
	result, err := c.ChatCompletionWithTools(ctx, messages, opts)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	if callback != nil && result.Content != "" {
		if err := callback(result.Content); err != nil {
			return result, err
		}
	}
	return result, nil
}

func buildResponsesInstructionsAndInput(messages []ChatMessage) (instructions string, input []any) {
	if len(messages) == 0 {
		return "", nil
	}

	input = make([]any, 0, len(messages))
	for idx, msg := range messages {
		if idx == 0 && strings.TrimSpace(msg.Role) == "system" {
			instructions = msg.Content
			continue
		}

		switch strings.TrimSpace(msg.Role) {
		case "tool":
			if strings.TrimSpace(msg.ToolCallID) != "" {
				input = append(input, responsesFunctionCallOutput{
					Type:   "function_call_output",
					CallID: msg.ToolCallID,
					Output: msg.Content,
				})
				continue
			}
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				if strings.TrimSpace(msg.Content) != "" {
					input = append(input, responsesMessage{
						Role:    "assistant",
						Content: msg.Content,
					})
				}
				for _, call := range msg.ToolCalls {
					input = append(input, responsesFunctionCall{
						Type:      "function_call",
						CallID:    call.ID,
						Name:      call.Function.Name,
						Arguments: call.Function.Arguments,
					})
				}
				continue
			}
		}

		input = append(input, responsesMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return instructions, input
}

func extractResponsesContentAndToolCalls(result responsesResponse) (content string, toolCalls []ToolCall) {
	var out strings.Builder
	toolCalls = make([]ToolCall, 0)

	for _, item := range result.Output {
		switch item.Type {
		case "function_call":
			callID := strings.TrimSpace(item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(item.Name)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   callID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		case "message":
			for _, block := range item.Content {
				if strings.TrimSpace(block.Text) != "" {
					out.WriteString(block.Text)
				}
			}
			if strings.TrimSpace(item.Text) != "" {
				out.WriteString(item.Text)
			}
		case "output_text":
			if strings.TrimSpace(item.Text) != "" {
				out.WriteString(item.Text)
			}
			for _, block := range item.Content {
				if strings.TrimSpace(block.Text) != "" {
					out.WriteString(block.Text)
				}
			}
		default:
			if strings.TrimSpace(item.Text) != "" {
				out.WriteString(item.Text)
			}
			for _, block := range item.Content {
				if strings.TrimSpace(block.Text) != "" {
					out.WriteString(block.Text)
				}
			}
		}
	}

	return out.String(), toolCalls
}

func extractResponsesStreamChunk(raw string) string {
	var event map[string]any
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return ""
	}

	if typ, ok := event["type"].(string); ok {
		switch typ {
		case "response.output_text.delta":
			if delta, ok := event["delta"].(string); ok {
				return delta
			}
			if text, ok := event["text"].(string); ok {
				return text
			}
		case "response.output_text":
			if text, ok := event["text"].(string); ok {
				return text
			}
		case "response.completed":
			return ""
		}
	}

	if delta, ok := event["delta"].(string); ok {
		return delta
	}

	return ""
}
