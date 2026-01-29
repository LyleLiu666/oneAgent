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

type responsesRequest struct {
	Model           string             `json:"model"`
	Input           []responsesMessage `json:"input"`
	Stream          bool               `json:"stream"`
	Temperature     *float64           `json:"temperature,omitempty"`
	MaxOutputTokens *int               `json:"max_output_tokens,omitempty"`
	TopP            *float64           `json:"top_p,omitempty"`
	PromptCacheKey  string             `json:"prompt_cache_key,omitempty"`
	Tools           []Tool             `json:"tools,omitempty"`
	ToolChoice      any                `json:"tool_choice,omitempty"`
}

type responsesResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Text string `json:"text"`
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
	reqBody := responsesRequest{
		Model:  model,
		Input:  buildResponsesInput(messages),
		Stream: true,
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
			reqBody.Tools = normalizeTools(opts.Tools)
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

func buildResponsesInput(messages []ChatMessage) []responsesMessage {
	converted := make([]responsesMessage, 0, len(messages))
	for _, msg := range messages {
		converted = append(converted, responsesMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return converted
}

func extractResponsesOutput(result responsesResponse) string {
	var builder strings.Builder
	for _, output := range result.Output {
		if output.Text != "" {
			builder.WriteString(output.Text)
		}
		for _, content := range output.Content {
			if content.Text != "" {
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
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
