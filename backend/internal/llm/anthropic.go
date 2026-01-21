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
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
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

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ChatCompletion performs a non-streaming completion with Claude.
func (c *AnthropicClient) ChatCompletion(ctx context.Context, messages []ChatMessage, opts *ChatCompletionOptions) (string, error) {
	model := c.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	if opts != nil && opts.Trace != nil && opts.Trace.OnStart != nil {
		opts.Trace.OnStart(ctx, messages)
	}

	system, anthropicMessages := buildAnthropicPayload(messages, opts != nil && opts.EnablePromptCache)
	maxTokens := 4096
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
		Stream:        false,
	}

	if opts != nil {
		reqBody.Temperature = opts.Temperature
		reqBody.TopP = opts.TopP
		reqBody.StopSequences = opts.Stop
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	if opts != nil && opts.EnablePromptCache {
		req.Header.Set("anthropic-beta", "prompt-caching")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Type == "error" || result.Error != nil {
		msg := "unknown error"
		if result.Error != nil && result.Error.Message != "" {
			msg = result.Error.Message
		}
		err := fmt.Errorf("API error: %s", msg)
		if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
			opts.Trace.OnComplete(ctx, "", err)
		}
		return "", err
	}

	var contentBuilder strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			contentBuilder.WriteString(block.Text)
		}
	}

	content := contentBuilder.String()
	if opts != nil && opts.Trace != nil && opts.Trace.OnComplete != nil {
		if opts.Trace.OnFirstToken != nil {
			opts.Trace.OnFirstToken(ctx)
		}
		opts.Trace.OnComplete(ctx, content, nil)
	}

	return content, nil
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
	maxTokens := 4096
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

func buildAnthropicPayload(messages []ChatMessage, enableCache bool) ([]anthropicContent, []anthropicMessage) {
	cacheIndexes := map[int]bool{}
	if enableCache {
		cacheIndexes = cacheMessageIndexes(messages)
	}

	system := make([]anthropicContent, 0)
	converted := make([]anthropicMessage, 0)

	for idx, msg := range messages {
		block := anthropicContent{
			Type: "text",
			Text: msg.Content,
		}
		if enableCache && cacheIndexes[idx] {
			block.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
		}

		if msg.Role == "system" {
			system = append(system, block)
			continue
		}

		converted = append(converted, anthropicMessage{
			Role:    msg.Role,
			Content: []anthropicContent{block},
		})
	}

	return system, converted
}
