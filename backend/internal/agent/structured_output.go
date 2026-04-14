package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type StructuredOutputMode string

const (
	StructuredOutputModeToolCall StructuredOutputMode = "tool_call"
	StructuredOutputModeTags     StructuredOutputMode = "tags"
)

type StructuredOutputMeta struct {
	Mode                 StructuredOutputMode
	FellBackFromToolCall bool
	ToolCallID           string
	RawOutput            string
}

type StructuredOutputSpec[T any] struct {
	// Tool is the preferred structured-output channel (function calling).
	Tool llm.Tool

	// ToolName overrides the expected tool name (defaults to Tool.Function.Name).
	ToolName string

	// ToolInstruction, when non-empty, is appended as a user message for tool-call attempts.
	ToolInstruction string

	// TagsInstruction, when non-empty, is appended as a user message for tags attempts.
	TagsInstruction string

	// ParseToolArgs parses a tool call's function.arguments JSON into a payload.
	ParseToolArgs func(raw json.RawMessage) (T, error)

	// ParseTags parses a tags-based payload from model text output.
	ParseTags func(text string) (T, bool)

	MaxToolAttempts int
	MaxTagAttempts  int
}

func RequestStructuredOutput[T any](
	ctx context.Context,
	client llm.Client,
	baseMessages []llm.ChatMessage,
	baseOpts *llm.ChatCompletionOptions,
	spec StructuredOutputSpec[T],
) (T, StructuredOutputMeta, error) {
	var zero T

	if client == nil {
		return zero, StructuredOutputMeta{}, errors.New("llm client is required")
	}
	if spec.ParseToolArgs == nil {
		return zero, StructuredOutputMeta{}, errors.New("ParseToolArgs is required")
	}
	if spec.ParseTags == nil {
		return zero, StructuredOutputMeta{}, errors.New("ParseTags is required")
	}

	toolName := strings.TrimSpace(spec.ToolName)
	if toolName == "" {
		toolName = strings.TrimSpace(spec.Tool.Function.Name)
	}

	maxToolAttempts := spec.MaxToolAttempts
	if maxToolAttempts <= 0 {
		maxToolAttempts = 1
	}
	maxTagAttempts := spec.MaxTagAttempts
	if maxTagAttempts <= 0 {
		maxTagAttempts = 1
	}

	// 1) Tool-call channel (preferred when supported).
	if toolName != "" {
		if toolClient, ok := client.(toolCaller); ok {
			var lastToolErr error
			for attempt := 0; attempt < maxToolAttempts; attempt++ {
				callMessages := append([]llm.ChatMessage(nil), baseMessages...)
				if strings.TrimSpace(spec.ToolInstruction) != "" {
					callMessages = append(callMessages, llm.BuildUserMessage(strings.TrimSpace(spec.ToolInstruction)))
				}

				opts := cloneChatCompletionOptions(baseOpts)
				opts.Tools = []llm.Tool{spec.Tool}
				if opts.Temperature == nil {
					temp := 0.0
					opts.Temperature = &temp
				}

				res, err := toolClient.ChatCompletionWithTools(ctx, callMessages, opts)
				if err != nil {
					lastToolErr = err
					if shouldFallbackToTags(err) {
						break
					}
					continue
				}

				for _, call := range res.ToolCalls {
					if !llm.ToolNamesEquivalent(call.Function.Name, toolName) {
						continue
					}
					raw := json.RawMessage(call.Function.Arguments)
					payload, parseErr := spec.ParseToolArgs(raw)
					if parseErr == nil {
						return payload, StructuredOutputMeta{
							Mode:       StructuredOutputModeToolCall,
							ToolCallID: strings.TrimSpace(call.ID),
						}, nil
					}

					lastToolErr = fmt.Errorf("tool call arguments parse failed: %w", parseErr)
					break
				}

				if lastToolErr == nil {
					lastToolErr = errors.New("expected tool call not found in response")
				}
			}

			// Fall back to tags mode below (best-effort).
			_ = lastToolErr
		}
	}

	// 2) Tags fallback channel.
	var lastTagsErr error
	for attempt := 0; attempt < maxTagAttempts; attempt++ {
		callMessages := append([]llm.ChatMessage(nil), baseMessages...)
		if strings.TrimSpace(spec.TagsInstruction) != "" {
			callMessages = append(callMessages, llm.BuildUserMessage(strings.TrimSpace(spec.TagsInstruction)))
		}

		opts := cloneChatCompletionOptions(baseOpts)
		opts.Tools = nil

		out, err := client.ChatCompletion(ctx, callMessages, opts)
		if err != nil {
			lastTagsErr = err
			continue
		}
		if payload, ok := spec.ParseTags(out); ok {
			return payload, StructuredOutputMeta{
				Mode:                 StructuredOutputModeTags,
				FellBackFromToolCall: toolName != "" && supportsToolCalling(client),
				RawOutput:            truncateString(strings.TrimSpace(out), 800),
			}, nil
		}
		lastTagsErr = errors.New("tags output parse failed")
	}

	if lastTagsErr == nil {
		lastTagsErr = errors.New("structured output failed")
	}
	return zero, StructuredOutputMeta{Mode: StructuredOutputModeTags}, lastTagsErr
}

func supportsToolCalling(client llm.Client) bool {
	if client == nil {
		return false
	}
	_, ok := client.(toolCaller)
	return ok
}

func shouldFallbackToTags(err error) bool {
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode != 400 {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(apiErr.Message))
	if msg == "" {
		msg = strings.ToLower(strings.TrimSpace(apiErr.Raw))
	}
	if msg == "" {
		msg = strings.ToLower(err.Error())
	}
	return strings.Contains(msg, "invalid function arguments") || strings.Contains(msg, "function arguments json")
}

func cloneChatCompletionOptions(in *llm.ChatCompletionOptions) *llm.ChatCompletionOptions {
	if in == nil {
		return &llm.ChatCompletionOptions{}
	}
	out := *in
	if in.MaxTokens != nil {
		v := *in.MaxTokens
		out.MaxTokens = &v
	}
	if in.Temperature != nil {
		v := *in.Temperature
		out.Temperature = &v
	}
	if in.TopP != nil {
		v := *in.TopP
		out.TopP = &v
	}
	if in.FrequencyPenalty != nil {
		v := *in.FrequencyPenalty
		out.FrequencyPenalty = &v
	}
	if in.PresencePenalty != nil {
		v := *in.PresencePenalty
		out.PresencePenalty = &v
	}
	if in.Stop != nil {
		out.Stop = append([]string{}, in.Stop...)
	}
	if in.Tools != nil {
		out.Tools = append([]llm.Tool{}, in.Tools...)
	}
	return &out
}
