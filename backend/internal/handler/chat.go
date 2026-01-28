// Package handler provides HTTP handlers for the API.
//
// CHAT HANDLER:
// This file implements the chat API endpoints with real LLM integration.
// It uses the llm package for OpenAI-compatible API calls.
//
// EXTENSION POINTS:
// - Add new chat endpoints for different modules (e.g., ReaderChat, GuideChat)
// - Implement tool/function calling by extending the streaming handler
// - Add custom trace collection for observability
//
// MULTI-MODULE PATTERN:
// To create a new chat module, copy this handler and:
// 1. Change the Module constant
// 2. Customize the system prompt
// 3. Add module-specific logic (e.g., context injection)
package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/llmlog"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/prompt"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
)

// ============================================================================
// CONSTANTS - Customize for your modules
// ============================================================================

// ChatModule defines the module identifier for this chat handler.
// EXTENSION: Create new handlers with different module identifiers.
const ChatModule = "assistant"

// DefaultSystemPrompt is the default system prompt for the assistant.
// EXTENSION: Customize this for your use case or make it configurable.
const DefaultSystemPrompt = `- 总是以TDD的思想完成任务，主动验证任务的效果，再给到用户汇报；以干完就走为耻，以保证结果为荣
- 碰到关键问题多询问，多反思，多换位思考，多运用通识，多深入用户思考；以想当然为耻，以理解任务背景信息为荣

你是一个agentic的助手，帮助用户完成他的目标。
总是以专业的角度，提供生产级的解决方案，总是在行动前先识别假设，主动向用户提问验证。

## 任务管理
- 频繁使用一个plan.md文件来管理和规划任务
- 用于规划任务和将复杂任务分解为小步骤
- 立即标记已完成的待办事项,不要批量处理

## 提问机制:
- 澄清问题
- 验证假设
- 做出不确定的决策

## 子 Agent（subagent）使用建议
- 仅当某个步骤边界清晰、能独立交付（代码/文档/文件）且不需要频繁回看大量历史时，才考虑调用 subagent
- 不要把强耦合、需要频繁交互/反复回看上下文的工作交给子 Agent
- 调用 subagent 时：提供清晰 task；仅传递前序步骤“短总结 + findings/trace 引用（路径）”，不要塞入全量过程

## 执行任务

推荐步骤:

1. **在修改文件前先阅读已经存在的文件**
2. 反问用户以收集信息
3. **避免过度工程化**:
    - 只做直接要求或明确必要的更改
    - 不添加未请求的功能、重构或"改进"
    - 不添加不必要的错误处理、验证或抽象
    - 避免为假设的未来需求设计
`

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// ChatRequest represents an incoming chat message.
// EXTENSION: Add fields for temperature, model override, etc.
type ChatRequest struct {
	Message      string   `json:"message" binding:"required"`
	SessionID    string   `json:"session_id"`
	SystemPrompt string   `json:"system_prompt,omitempty"` // EXTENSION: Custom system prompt
	ModelID      string   `json:"model_id,omitempty"`
	ToolIDs      []string `json:"tool_ids,omitempty"`
	ToolProtocol string   `json:"tool_protocol,omitempty"` // "json" (default) or "xml"
	Workspace    string   `json:"workspace,omitempty"`
}

// StreamEvent represents a Server-Sent Event.
// EXTENSION: Add new event types for tool calls, progress, etc.
type StreamEvent struct {
	Type string `json:"type"` // "session", "content", "trace", "done", "error"
	Data string `json:"data"`
}

type streamMsg struct {
	Op         string                      `json:"op"` // "start" | "delta" | "final" | "insert"
	ID         string                      `json:"id"`
	Role       string                      `json:"role,omitempty"`
	MsgType    string                      `json:"msg_type,omitempty"` // "text" | "tool_call" | "tool_result"
	Delta      string                      `json:"delta,omitempty"`
	Error      string                      `json:"error,omitempty"`
	ToolCall   *persistedToolCallMessage   `json:"tool_call,omitempty"`
	ToolResult *persistedToolResultMessage `json:"tool_result,omitempty"`
}

func broadcastMsg(b *StreamBroadcaster, msg streamMsg) {
	if b == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	b.Broadcast(StreamEvent{Type: "msg", Data: string(data)})
}

// TruncateSessionRequest represents a request to truncate a session's messages.
// This is used for "retry/regenerate" flows where we discard messages from a given point.
type TruncateSessionRequest struct {
	FromMessageID uint `json:"from_message_id" binding:"required"`
}

// ============================================================================
// HANDLER
// ============================================================================

// ChatHandler handles chat-related API endpoints.
type ChatHandler struct {
	rt            *oneruntime.Runtime
	streamManager *StreamManager
}

// NewChatHandler creates a new ChatHandler with LLM client.
func NewChatHandler(rt *oneruntime.Runtime) *ChatHandler {
	return &ChatHandler{
		rt:            rt,
		streamManager: NewStreamManager(),
	}
}

// ============================================================================
// STREAMING CHAT ENDPOINT
// ============================================================================

// StreamChat handles streaming chat responses.
// POST /api/chat
//
// EXTENSION POINTS:
// - Add context injection (RAG, user preferences)
// - Implement tool/function calling
// - Add rate limiting per user
func (h *ChatHandler) StreamChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.rt == nil || h.rt.Sessions == nil || h.rt.Settings == nil || h.rt.LLMLog == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	policySnap, err := h.rt.ResolveToolPolicySnapshot(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create or get session
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	baseOverride := strings.TrimSpace(req.SystemPrompt)
	systemPrompt := ""

	// Ensure session exists and get history
	title := truncateString(req.Message, 100)
	if _, err := h.rt.Sessions.GetOrCreateSession(sessionID, userID, ChatModule, title); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	session, persistedMessages, err := h.rt.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load session"})
		return
	}

	workspaceFromReq := strings.TrimSpace(req.Workspace)
	workspaceSet := workspaceFromReq != ""
	workspaceValue := workspaceFromReq
	if !workspaceSet {
		if stored, ok := extractWorkspace(session.Metadata); ok {
			workspaceValue = stored
		}
	}
	workspaceValue = strings.TrimSpace(workspaceValue)

	workspaceRoot := ""
	if workspaceValue != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(workspaceValue)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		workspaceRoot = normalized
	}

	selectedModelID := strings.TrimSpace(req.ModelID)
	if selectedModelID == "" && session.Metadata != nil {
		if raw, ok := session.Metadata["model_id"]; ok {
			if modelID, ok := raw.(string); ok {
				selectedModelID = modelID
			}
		}
	}

	toolIDsSet := req.ToolIDs != nil
	selectedToolIDs := req.ToolIDs
	if !toolIDsSet {
		if stored, ok := extractToolIDs(session.Metadata); ok {
			selectedToolIDs = stored
		}
		selectedToolIDs = filterKnownToolIDs(selectedToolIDs)
	}

	toolProtocol := strings.TrimSpace(req.ToolProtocol)
	toolProtocolSet := toolProtocol != ""
	if !toolProtocolSet {
		if stored, ok := extractToolProtocol(session.Metadata); ok {
			toolProtocol = stored
		}
	}
	toolProtocol = strings.ToLower(strings.TrimSpace(toolProtocol))
	if toolProtocol == "" {
		toolProtocol = "json"
	}
	if toolProtocol != "json" && toolProtocol != "xml" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tool_protocol"})
		return
	}

	resolvedModel, err := h.resolveModel(c.Request.Context(), userID, selectedModelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if resolvedModel.SafetyTier != safetyTierHigh && userID != "local" {
		blocked := intersectToolIDs(selectedToolIDs, []string{tool.ToolIDBash, tool.ToolIDRunCommand})
		if len(blocked) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("tool(s) %s require safety_tier=%s (current=%s)", strings.Join(blocked, ","), safetyTierHigh, resolvedModel.SafetyTier),
			})
			return
		}
	}

	toolDefs, err := tool.MountWithSnapshot(selectedToolIDs, policySnap)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	selectedToolIDs = toolIDsFromDefinitions(toolDefs)

	toolNames := make([]string, 0, len(toolDefs))
	for _, def := range toolDefs {
		toolNames = append(toolNames, def.Spec.Function.Name)
	}
	assembled, err := prompt.AssembleStablePrefix(prompt.AssembleInput{
		BaseOverride: baseOverride,
		ToolNames:    toolNames,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	systemPrompt = assembled.StablePrefix

	// Build conversation history (include persisted tool calls/results for KV cache and correctness).
	messages := make([]llm.ChatMessage, 0, 1+len(persistedMessages)+1)
	messages = append(messages, llm.BuildSystemMessage(systemPrompt))
	if len(persistedMessages) > 0 {
		messages = append(messages, buildLLMHistoryFromMessages(persistedMessages, toolProtocol)...)
	}

	// TurnContext (volatile): dynamic per-turn context MUST NOT be injected into the stable prefix.
	// This is intentionally appended after persisted history and excluded from cache selection.
	turnContext := ""
	for _, def := range toolDefs {
		if def.ID == tool.ToolIDSkillRead {
			turnContext = strings.TrimSpace(buildSkillSuggestionTurnContext(c.Request.Context(), h.rt.Skills, workspaceRoot, req.Message))
			break
		}
	}
	if msg, ok := llm.BuildTurnContextMessage(turnContext); ok {
		messages = append(messages, msg)
	}

	// Add current user message (always last)
	messages = append(messages, llm.BuildUserMessage(req.Message))

	if toolProtocol == "xml" && len(toolDefs) > 0 && len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content = strings.TrimSpace(messages[0].Content) + "\n\n" + toolxml.SystemPrompt(toolDefs)
	}

	if resolvedModel.ModelID != "" {
		sessionMetadata := session.Metadata
		if sessionMetadata == nil {
			sessionMetadata = model.JSONB{}
		}

		epoch := extractPromptCacheEpoch(sessionMetadata)

		prevSystemPrompt, _ := sessionMetadata["system_prompt"].(string)
		prevModelID, _ := sessionMetadata["model_id"].(string)
		prevToolIDs, prevToolIDsOK := extractToolIDs(sessionMetadata)
		prevToolProtocol, prevToolProtocolOK := extractToolProtocol(sessionMetadata)

		needsEpochBump := false
		if strings.TrimSpace(req.SystemPrompt) != "" && strings.TrimSpace(prevSystemPrompt) != "" && prevSystemPrompt != systemPrompt {
			needsEpochBump = true
		}
		if strings.TrimSpace(req.ModelID) != "" && strings.TrimSpace(prevModelID) != "" && prevModelID != resolvedModel.ModelID {
			needsEpochBump = true
		}
		if toolIDsSet && prevToolIDsOK && !equalStringSlices(prevToolIDs, selectedToolIDs) {
			needsEpochBump = true
		}
		if toolProtocolSet && prevToolProtocolOK && prevToolProtocol != toolProtocol {
			needsEpochBump = true
		}

		shouldUpdate := false
		if workspaceSet {
			if existing, ok := sessionMetadata["workspace"].(string); ok && strings.TrimSpace(existing) != "" && existing != workspaceRoot {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change workspace for an existing session"})
				return
			}
			if workspaceRoot != "" {
				sessionMetadata["workspace"] = workspaceRoot
				shouldUpdate = true
			}
		}
		if _, ok := sessionMetadata["system_prompt"]; !ok || req.SystemPrompt != "" {
			sessionMetadata["system_prompt"] = systemPrompt
			shouldUpdate = true
		}
		if _, ok := sessionMetadata["model_id"]; !ok || req.ModelID != "" {
			sessionMetadata["model_id"] = resolvedModel.ModelID
			shouldUpdate = true
		}
		if toolIDsSet {
			sessionMetadata["tool_ids"] = selectedToolIDs
			shouldUpdate = true
		}
		if toolProtocolSet {
			sessionMetadata["tool_protocol"] = toolProtocol
			shouldUpdate = true
		}

		// Persist policy snapshot identifiers for UX/debugging.
		if prevHash, _ := sessionMetadata["policy_hash"].(string); strings.TrimSpace(prevHash) != strings.TrimSpace(policySnap.PolicyHash) {
			sessionMetadata["principal_id"] = policySnap.PrincipalID
			sessionMetadata["policy_id"] = policySnap.Policy.ID
			sessionMetadata["policy_hash"] = policySnap.PolicyHash
			sessionMetadata["policy_resolved_at"] = policySnap.ResolvedAt
			shouldUpdate = true
		}

		if needsEpochBump {
			epoch++
		}
		if needsEpochBump || sessionMetadata["prompt_cache_epoch"] == nil {
			sessionMetadata["prompt_cache_epoch"] = epoch
			shouldUpdate = true
		}

		if shouldUpdate {
			_ = h.rt.Sessions.UpdateSessionMetadata(sessionID, sessionMetadata)
		}

		session.Metadata = sessionMetadata
	}

	sessionMetadataForEpoch := session.Metadata
	if sessionMetadataForEpoch == nil {
		sessionMetadataForEpoch = model.JSONB{}
	}
	cacheEpoch := extractPromptCacheEpoch(sessionMetadataForEpoch)

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// Ensure headers are sent immediately and push an initial padding comment to reduce proxy buffering.
	c.Writer.WriteHeaderNow()
	fmt.Fprintf(c.Writer, ":%s\n\n", strings.Repeat(" ", 2048))
	flusher.Flush()

	// Send session_id first
	sendSSE(c.Writer, flusher, StreamEvent{
		Type: "session",
		Data: sessionID,
	})

	// Get or create broadcater for this session
	broadcaster := h.streamManager.GetOrCreate(sessionID)

	// Subscribe to the stream
	// The channel size should be large enough to hold history if we implement replay,
	// but for now we just want a realtime stream.
	clientChan := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(clientChan)

	// If this is the PRIMARY (first) request for this session that triggered creation,
	// start the generation process in background.
	// Otherwise, we just listen.
	if broadcaster.StartGeneration() {
		go func() {
			defer broadcaster.Finish()

			// Compress long sessions before persisting this user turn.
			// This avoids deleting the freshly-created user message during compression.
			{
				ctx := context.Background()
				before := approximateContextRunes(messages)
				if before > sessionCompressionMaxContextRunes {
					broadcaster.Broadcast(StreamEvent{
						Type: "trace",
						Data: fmt.Sprintf("Context length %d > %d, compressing history...", before, sessionCompressionMaxContextRunes),
					})
				}

				compressed, compressedMessages, err := compressSessionIfNeeded(ctx, h.rt.Sessions, sessionID, persistedMessages, messages, resolvedModel.Client)
				if err != nil {
					broadcaster.Broadcast(StreamEvent{
						Type: "trace",
						Data: fmt.Sprintf("Context compression failed: %v", err),
					})

					// Fallback: keep the last two rounds and continue without DB mutation.
					if before > sessionCompressionMaxContextRunes {
						_, toKeep := splitForCompression(persistedMessages, sessionCompressionKeepTextMsgs)
						placeholder := fmt.Sprintf("【会话压缩】摘要生成失败（%v），已仅保留最近两轮对话。", err)

						fallback := make([]llm.ChatMessage, 0, 2+len(toKeep)+1)
						if len(messages) > 0 {
							fallback = append(fallback, messages[0])
						}
						fallback = append(fallback, llm.BuildAssistantMessage(placeholder))
						for _, msg := range toKeep {
							fallback = append(fallback, llm.ChatMessage{Role: msg.Role, Content: msg.Content})
						}
						if len(messages) > 0 {
							fallback = append(fallback, messages[len(messages)-1])
						}
						messages = fallback
					}
				} else if compressed {
					after := approximateContextRunes(compressedMessages)
					broadcaster.Broadcast(StreamEvent{
						Type: "trace",
						Data: fmt.Sprintf("Context compressed: %d → %d", before, after),
					})
					messages = compressedMessages

					// Compression rewrites the stable prefix; bump prompt cache epoch for prompt_cache_key providers.
					cacheEpoch++
					sessionMetadataForEpoch["prompt_cache_epoch"] = cacheEpoch
					_ = h.rt.Sessions.UpdateSessionMetadata(sessionID, sessionMetadataForEpoch)
				}
			}

			// Save user message only after compression succeeds (or is skipped).
			if _, err := h.rt.Sessions.AppendMessage(sessionID, model.ChatMessage{
				Role:    model.MessageRoleUser,
				Type:    model.MessageTypeText,
				Content: req.Message,
			}); err != nil {
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Failed to persist user message: %v", err),
				})
				return
			}

			// Start trace collection
			traceStart := time.Now()
			var traceEntries []model.TraceEntry

			// Trace callback integration
			traceCallback := &llm.TraceCallback{
				OnStart: func(ctx context.Context, input []llm.ChatMessage) {
					broadcaster.Broadcast(StreamEvent{
						Type: "trace",
						Data: "Connected to AI...",
					})
				},
				OnFirstToken: func(ctx context.Context) {
					broadcaster.Broadcast(StreamEvent{
						Type: "trace",
						Data: "Generating response...",
					})
				},
				OnComplete: func(ctx context.Context, output string, err error) {
					if err != nil {
						broadcaster.Broadcast(StreamEvent{
							Type: "error",
							Data: fmt.Sprintf("LLM error: %v", err),
						})
					} else {
						broadcaster.Broadcast(StreamEvent{
							Type: "trace",
							Data: "Response complete.",
						})
					}
				},
				OnToken: func(ctx context.Context, token string) {
					// Count tokens (approximate relying on stream chunks)
					// In a real implementation we might use a tokenizer, but counting chunks/words is a naive proxy
					// or if the provider sends raw token usage we'd use that.
					// For this feature request, we just want "activity" to be visible.
					// We'll increment a counter and broadcast occasionally.

					// Hack: use a closure variable for state since we can't easily change the method signature
					// We need to define the counter outside this struct literal.
					// See below for the implementation integration.
				},
			}

			// Token tracking state
			var tokenCount int
			var lastBroadcast time.Time

			// UTF-8 buffering state
			var incompleteUTF8 []byte

			// Update the callback to use the state
			traceCallback.OnToken = func(ctx context.Context, token string) {
				tokenCount += utf8.RuneCountInString(token)

				// Throttle updates: every 20 characters or 100ms
				now := time.Now()
				if tokenCount%20 == 0 || now.Sub(lastBroadcast) > 100*time.Millisecond {
					broadcaster.Broadcast(StreamEvent{
						Type: "usage",
						Data: fmt.Sprintf(`{"response_tokens": %d}`, tokenCount),
					})
					lastBroadcast = now
				}
			}

			// Trace internal steps (fake ones purely for UI experience if config enabled)
			if h.rt.Config.EnableTrace {
				traceEntry := model.NewTraceEntry(model.TraceTypeLLMCall, "ChatCompletion")
				traceEntry.Input = map[string]any{
					"message_count": len(messages),
					"tool_protocol": toolProtocol,
					"tool_ids":      selectedToolIDs,
				}
				traceEntry.Model = resolvedModel.ModelName
				traceEntries = append(traceEntries, traceEntry)

				broadcaster.Broadcast(StreamEvent{
					Type: "trace",
					Data: "Preparing conversation context...",
				})
				time.Sleep(50 * time.Millisecond)
			}

			// Call LLM with streaming
			var fullContent string
			ctx := context.Background() // Use background context so generation survives request cancellation
			ctx = tool.ContextWithSessionID(ctx, sessionID)
			ctx = tool.ContextWithUserID(ctx, userID)
			ctx = tool.ContextWithPolicySnapshot(ctx, policySnap)
			ctx = tool.ContextWithSettingsDB(ctx, h.rt.Settings)
			ctx = tool.ContextWithSkillManager(ctx, h.rt.Skills)
			ctx = tool.ContextWithRuntimeLayout(ctx, h.rt.Layout)
			ctx = tool.ContextWithWorkLedger(ctx, h.rt.WorkLedger)
			ctx = tool.ContextWithLLMClient(ctx, resolvedModel.Client)
			ctx = tool.ContextWithModelName(ctx, resolvedModel.ModelName)
			ctx = tool.ContextWithSystemPrompt(ctx, systemPrompt)
			ctx = tool.ContextWithWorkspace(ctx, tool.WorkspaceConfig{
				Enabled: strings.TrimSpace(workspaceRoot) != "",
				Root:    workspaceRoot,
			})
			ctx = tool.ContextWithOCC(ctx, strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_OCC")) != "1")

			opts := &llm.ChatCompletionOptions{
				Trace: traceCallback,
			}
			if toolProtocol == "json" && len(toolDefs) > 0 {
				opts.Tools = tool.ToolsForLLM(toolDefs)
			}
			if resolvedModel.EnableKVCache {
				opts.EnablePromptCache = true
				if llm.SupportsPromptCacheKey(resolvedModel.ProviderType) {
					key, keyErr := llm.BuildPromptCacheKey(llm.PromptCacheKeyInput{
						SessionID:    sessionID,
						Epoch:        cacheEpoch,
						Model:        resolvedModel.ModelName,
						ToolProtocol: toolProtocol,
						Messages:     messages,
						Tools:        opts.Tools,
					})
					if keyErr == nil {
						opts.PromptCacheKey = key
					}
				}
			}

			llmCallID := uuid.NewString()
			callRecord := llmlog.CallRecord{
				ID:        llmCallID,
				SessionID: sessionID,
				UserID:    userID,
				Model:     resolvedModel.ModelName,
				Provider:  resolvedModel.ProviderType,
				CreatedAt: time.Now(),
				Request: map[string]any{
					"messages":                  messages,
					"tool_protocol":             toolProtocol,
					"tool_ids":                  selectedToolIDs,
					"cacheable_message_indexes": llm.CacheableMessageIndexes(messages),
				},
				PromptCacheEnabled: opts.EnablePromptCache,
				PromptCacheEpoch:   cacheEpoch,
			}
			if opts.EnablePromptCache && strings.TrimSpace(opts.PromptCacheKey) != "" {
				callRecord.PromptCacheKeyHash = sha256Hex(opts.PromptCacheKey)
			}

			if opts.EnablePromptCache {
				msg := "KV cache: enabled=true"
				if callRecord.PromptCacheKeyHash != "" {
					msg += " key_hash=" + callRecord.PromptCacheKeyHash
				}
				msg += fmt.Sprintf(" epoch=%d", cacheEpoch)
				broadcaster.Broadcast(StreamEvent{Type: "trace", Data: msg})
			}

			var err error
			var toolLoopPersisted bool
			if len(toolDefs) > 0 && toolProtocol == "xml" {
				var currentStepID string
				fullContent, err = toolxml.RunLoop(
					ctx,
					resolvedModel.Client,
					messages,
					opts,
					toolDefs,
					userID,
					func(chunk string) error {
						broadcastMsg(broadcaster, streamMsg{
							Op:      "delta",
							ID:      currentStepID,
							Role:    model.MessageRoleAssistant,
							MsgType: model.MessageTypeText,
							Delta:   chunk,
						})
						return nil
					},
					func(msg string) {
						broadcaster.Broadcast(StreamEvent{Type: "trace", Data: msg})
					},
					func(msg string) {
						broadcaster.Broadcast(StreamEvent{Type: "error", Data: msg})
					},
					func(toolName, toolCallID, args string, toolErr error) {
						recordToolFailure(sessionID, userID, resolvedModel, toolName, toolCallID, args, toolErr)
					},
					func(step toolxml.StepRecord) {
						var parentID uint
						content, err := marshalPersistedToolCall("xml", step.VisibleContent, step.AssistantContent, step.ToolCalls)
						if err != nil {
							return
						}
						broadcastMsg(broadcaster, streamMsg{
							Op:      "final",
							ID:      currentStepID,
							Role:    model.MessageRoleAssistant,
							MsgType: model.MessageTypeToolCall,
							ToolCall: &persistedToolCallMessage{
								Protocol:   "xml",
								Content:    step.VisibleContent,
								LLMContent: step.AssistantContent,
								ToolCalls:  step.ToolCalls,
							},
						})

						var traceData model.TraceDataJSON
						if h.rt.Config.EnableTrace {
							entries := make([]model.TraceEntry, 0, len(step.ToolCalls)*2)
							resultByID := make(map[string]toolxml.ToolResult, len(step.ToolResults))
							for _, r := range step.ToolResults {
								resultByID[r.ToolCallID] = r
							}

							for _, call := range step.ToolCalls {
								if r, ok := resultByID[call.ID]; ok {
									if call.Function.Name == "subagent" {
										var parsed struct {
											OK           bool   `json:"ok"`
											Summary      string `json:"summary"`
											FindingsPath string `json:"findings_path"`
											TraceLogPath string `json:"trace_log_path"`
											RunID        string `json:"run_id"`
											DurationMs   int64  `json:"duration_ms"`
											Error        string `json:"error"`
										}
										_ = json.Unmarshal([]byte(r.OutputJSON), &parsed)

										entry := model.NewTraceEntry(model.TraceTypeSubAgent, call.Function.Name)
										entry.Input = map[string]any{
											"tool_call_id": call.ID,
											"arguments":    call.Function.Arguments,
										}
										entry.Output = map[string]any{
											"ok":             parsed.OK,
											"summary":        parsed.Summary,
											"findings_path":  parsed.FindingsPath,
											"trace_log_path": parsed.TraceLogPath,
											"run_id":         parsed.RunID,
											"duration_ms":    parsed.DurationMs,
											"error":          parsed.Error,
										}
										entry.Metadata["tool_call_id"] = call.ID
										entry.Metadata["protocol"] = "xml"
										entry.Metadata["parent_session_id"] = sessionID
										if strings.TrimSpace(parsed.RunID) != "" {
											entry.Metadata["run_id"] = strings.TrimSpace(parsed.RunID)
										}
										if strings.TrimSpace(parsed.FindingsPath) != "" {
											entry.Metadata["findings_path"] = strings.TrimSpace(parsed.FindingsPath)
										}
										if strings.TrimSpace(parsed.TraceLogPath) != "" {
											entry.Metadata["trace_log_path"] = strings.TrimSpace(parsed.TraceLogPath)
										}
										if r.Error != "" {
											entry.Error = r.Error
										} else if strings.TrimSpace(parsed.Error) != "" {
											entry.Error = strings.TrimSpace(parsed.Error)
										} else if !parsed.OK {
											entry.Error = "subagent failed"
										}
										entry.Complete()
										entries = append(entries, entry)
										continue
									}

									entry := model.NewTraceEntry(model.TraceTypeToolCall, call.Function.Name)
									entry.Input = map[string]any{
										"tool_call_id": call.ID,
										"arguments":    call.Function.Arguments,
									}
									entry.Metadata["tool_call_id"] = call.ID
									entry.Metadata["protocol"] = "xml"
									entry.Complete()
									entries = append(entries, entry)

									resEntry := model.NewTraceEntry(model.TraceTypeToolResult, call.Function.Name)
									resEntry.Output = r.OutputJSON
									if r.Error != "" {
										resEntry.Error = r.Error
									}
									resEntry.Metadata["tool_call_id"] = r.ToolCallID
									resEntry.Metadata["protocol"] = "xml"
									resEntry.Complete()
									entries = append(entries, resEntry)
									continue
								}

								entry := model.NewTraceEntry(model.TraceTypeToolCall, call.Function.Name)
								entry.Input = map[string]any{
									"tool_call_id": call.ID,
									"arguments":    call.Function.Arguments,
								}
								entry.Metadata["tool_call_id"] = call.ID
								entry.Metadata["protocol"] = "xml"
								entry.Complete()
								entries = append(entries, entry)
							}

							traceData = model.TraceDataJSON{
								TraceData: model.TraceData{
									Entries: entries,
									Model:   resolvedModel.ModelName,
								},
							}
						}

						callMsg := model.ChatMessage{
							Role:    model.MessageRoleAssistant,
							Type:    model.MessageTypeToolCall,
							Content: content,
							Trace:   traceData,
						}
						if persisted, err := h.rt.Sessions.AppendMessage(sessionID, callMsg); err == nil {
							parentID = persisted.ID
							toolLoopPersisted = true
						}

						argsByID := make(map[string]string, len(step.ToolCalls))
						for _, call := range step.ToolCalls {
							argsByID[call.ID] = call.Function.Arguments
						}

						structured := make([]persistedToolResult, 0, len(step.ToolResults))
						for _, r := range step.ToolResults {
							structured = append(structured, persistedToolResult{
								ToolName:   r.ToolName,
								ToolCallID: r.ToolCallID,
								Arguments:  argsByID[r.ToolCallID],
								OK:         r.OK,
								Output:     r.OutputJSON,
								Error:      r.Error,
							})
						}

						resultContent, err := marshalPersistedToolResult("xml", "", "", "", step.ToolResultMessage, structured)
						if err != nil {
							return
						}
						broadcastMsg(broadcaster, streamMsg{
							Op:      "insert",
							ID:      uuid.NewString(),
							Role:    model.MessageRoleUser,
							MsgType: model.MessageTypeToolResult,
							ToolResult: &persistedToolResultMessage{
								Protocol: "xml",
								Content:  step.ToolResultMessage,
								Results:  structured,
							},
						})
						resultMsg := model.ChatMessage{
							Role:    model.MessageRoleUser,
							Type:    model.MessageTypeToolResult,
							Content: resultContent,
						}
						if parentID != 0 {
							resultMsg.ParentID = &parentID
						}
						if _, err := h.rt.Sessions.AppendMessage(sessionID, resultMsg); err == nil {
							toolLoopPersisted = true
						}
					},
					func(visibleContent, assistantContent string) {
						content := strings.TrimSpace(assistantContent)
						if content == "" {
							content = visibleContent
						}
						broadcastMsg(broadcaster, streamMsg{
							Op:      "final",
							ID:      currentStepID,
							Role:    model.MessageRoleAssistant,
							MsgType: model.MessageTypeText,
						})
						msg := model.ChatMessage{
							Role:    model.MessageRoleAssistant,
							Type:    model.MessageTypeText,
							Content: content,
						}
						if _, err := h.rt.Sessions.AppendMessage(sessionID, msg); err == nil {
							toolLoopPersisted = true
						}
					},
					func(step int) {
						currentStepID = uuid.NewString()
						broadcastMsg(broadcaster, streamMsg{
							Op:      "start",
							ID:      currentStepID,
							Role:    model.MessageRoleAssistant,
							MsgType: model.MessageTypeText,
						})
					},
				)
			} else if len(toolDefs) > 0 {
				toolClient, ok := resolvedModel.Client.(interface {
					ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error)
				})
				if !ok {
					broadcaster.Broadcast(StreamEvent{
						Type: "error",
						Data: "Tool calling not supported for this provider",
					})
					err = fmt.Errorf("tool calling not supported for this provider")
				} else {
					fullContent, toolLoopPersisted, err = runToolLoop(ctx, toolClient, messages, opts, toolDefs, broadcaster, sessionID, userID, resolvedModel, resolvedModel.ModelName, h.rt.Config.EnableTrace, h.rt.Sessions)
				}
			} else {
				assistantStreamID := uuid.NewString()
				broadcastMsg(broadcaster, streamMsg{
					Op:      "start",
					ID:      assistantStreamID,
					Role:    model.MessageRoleAssistant,
					MsgType: model.MessageTypeText,
				})
				err = resolvedModel.Client.ChatCompletionStream(ctx, messages, opts, func(chunk string) error {
					fullContent += chunk

					// Buffer content to ensure we only broadcast complete UTF-8 runes
					incompleteUTF8 = append(incompleteUTF8, chunk...)
					valid, rest := splitBuffer(incompleteUTF8)
					incompleteUTF8 = rest

					if len(valid) > 0 {
						broadcastMsg(broadcaster, streamMsg{
							Op:      "delta",
							ID:      assistantStreamID,
							Role:    model.MessageRoleAssistant,
							MsgType: model.MessageTypeText,
							Delta:   string(valid),
						})
					}
					return nil
				})
				broadcastMsg(broadcaster, streamMsg{
					Op:      "final",
					ID:      assistantStreamID,
					Role:    model.MessageRoleAssistant,
					MsgType: model.MessageTypeText,
					Error: func() string {
						if err != nil {
							return err.Error()
						}
						return ""
					}(),
				})
			}

			if toolProtocol == "xml" && err != nil && toolLoopPersisted {
				// Ensure an assistant "air bubble" exists on XML tool failures (history + trace).
				entry := model.NewTraceEntry(model.TraceTypeCustom, "Error")
				entry.Error = err.Error()
				entry.Complete()
				trace := model.TraceDataJSON{
					TraceData: model.TraceData{
						Entries: []model.TraceEntry{entry},
						Model:   resolvedModel.ModelName,
					},
				}
				msg := model.ChatMessage{
					SessionID: sessionID,
					Role:      model.MessageRoleAssistant,
					Type:      model.MessageTypeText,
					Content:   "",
					Trace:     trace,
				}
				_, _ = h.rt.Sessions.AppendMessage(sessionID, msg)
			}

			if err != nil {
				log.Printf("LLM stream error: %v", err)
				// Error broadcast handled in OnComplete trace or here
			}

			callRecord.Response = fullContent
			callRecord.PromptCacheEnabled = opts.EnablePromptCache
			callRecord.PromptCacheDowngraded = opts.PromptCacheDowngraded
			callRecord.PromptCacheDowngradeReason = strings.TrimSpace(opts.PromptCacheDowngradeReason)
			if err != nil {
				callRecord.Error = err.Error()
			}
			if callRecord.PromptCacheDowngraded && callRecord.PromptCacheDowngradeReason != "" {
				broadcaster.Broadcast(StreamEvent{
					Type: "trace",
					Data: "KV cache downgraded: " + callRecord.PromptCacheDowngradeReason,
				})
			}
			logPath := h.rt.LLMLog.PathForCall(time.Now(), sessionID, llmCallID)
			if writeErr := h.rt.LLMLog.WriteCall(logPath, callRecord); writeErr != nil {
				log.Printf("LLM log write error: %v", writeErr)
			}

			// Complete trace
			if h.rt.Config.EnableTrace && len(traceEntries) > 0 {
				traceEntries[0].Metadata["llm_log_path"] = logPath
				traceEntries[0].Metadata["prompt_cache_enabled"] = opts.EnablePromptCache
				traceEntries[0].Metadata["prompt_cache_epoch"] = cacheEpoch
				traceEntries[0].Metadata["prompt_cache_downgraded"] = opts.PromptCacheDowngraded
				if strings.TrimSpace(opts.PromptCacheDowngradeReason) != "" {
					traceEntries[0].Metadata["prompt_cache_downgrade_reason"] = strings.TrimSpace(opts.PromptCacheDowngradeReason)
				}
				if callRecord.PromptCacheKeyHash != "" {
					traceEntries[0].Metadata["prompt_cache_key_hash"] = callRecord.PromptCacheKeyHash
				}

				traceEntries[0].Complete()
				traceEntries[0].Output = fullContent
				if err != nil {
					traceEntries[0].Error = err.Error()
				}
			}

			// Save assistant message (also persist error cases so history shows an "air bubble" + trace).
			if !toolLoopPersisted && (fullContent != "" || err != nil) {
				entries := traceEntries
				if err != nil && len(entries) == 0 {
					entry := model.NewTraceEntry(model.TraceTypeCustom, "Error")
					entry.Error = err.Error()
					entry.Complete()
					entries = []model.TraceEntry{entry}
				}

				traceData := model.TraceDataJSON{
					TraceData: model.TraceData{
						Entries:  entries,
						Model:    resolvedModel.ModelName,
						Duration: time.Since(traceStart).Milliseconds(),
					},
				}

				assistantMsg := model.ChatMessage{
					Role:    model.MessageRoleAssistant,
					Type:    model.MessageTypeText,
					Content: fullContent,
					Trace:   traceData,
				}
				if _, persistErr := h.rt.Sessions.AppendMessage(sessionID, assistantMsg); persistErr != nil {
					log.Printf("Failed to persist assistant message: %v", persistErr)
				}
			}
		}()
	}

	// Listen for events and push to SSE
	// This blocks until the channel is closed (generation finished) or client disconnects
	notify := c.Request.Context().Done()
loop:
	for {
		select {
		case <-notify:
			// Client disconnected
			break loop
		case event, ok := <-clientChan:
			if !ok {
				// Broadcast channel closed (generation finished)
				break loop
			}
			sendSSE(c.Writer, flusher, event)
		}
	}

	// Send done event if we finished normally
	sendSSE(c.Writer, flusher, StreamEvent{
		Type: "done",
		Data: "",
	})
}

// ============================================================================
// SESSION MANAGEMENT ENDPOINTS
// ============================================================================

// GetSessions returns all chat sessions for the current user.
// GET /api/sessions
// EXTENSION: Add pagination, filtering by module
func (h *ChatHandler) GetSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	sessions, err := h.rt.Sessions.ListSessions(userID, ChatModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load sessions"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSession returns a specific session with messages.
// GET /api/sessions/:id
func (h *ChatHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)
	if h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	session, msgs, err := h.rt.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load session"})
		return
	}
	session.Messages = msgs
	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes a chat session.
// DELETE /api/sessions/:id
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)
	if h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	if _, _, err := h.rt.Sessions.GetSessionWithMessages(sessionID, userID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load session"})
		return
	}

	if err := h.rt.Sessions.DeleteSession(sessionID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}
	c.Status(http.StatusNoContent)
}

// TruncateSession deletes messages in a session starting from a specific message ID (inclusive).
// POST /api/sessions/:id/truncate
func (h *ChatHandler) TruncateSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)

	var req TruncateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	if err := h.rt.Sessions.TruncateFromMessageID(sessionID, userID, req.FromMessageID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to truncate messages"})
		return
	}

	c.Status(http.StatusNoContent)
}

type resolvedModel struct {
	Client        llm.Client
	ProviderType  string
	ProviderID    string
	ModelID       string
	ModelName     string
	EnableKVCache bool
	SafetyTier    string
}

func (h *ChatHandler) resolveModel(ctx context.Context, userID, modelID string) (*resolvedModel, error) {
	return h.resolveModelWithSettings(ctx, userID, modelID)
}

func (h *ChatHandler) resolveModelWithSettings(ctx context.Context, userID, modelID string) (*resolvedModel, error) {
	if h == nil || h.rt == nil || h.rt.Settings == nil {
		return nil, fmt.Errorf("settings not available")
	}

	var m settingsdb.Model
	if strings.TrimSpace(modelID) != "" {
		got, err := h.rt.Settings.GetModel(ctx, userID, strings.TrimSpace(modelID))
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("model not found")
			}
			return nil, err
		}
		m = got
	} else {
		models, err := h.rt.Settings.ListModels(ctx, userID, "")
		if err != nil {
			return nil, err
		}
		for _, candidate := range models {
			if candidate.IsDefault {
				m = candidate
				break
			}
		}
	}

	if m.ID == "" {
		if strings.TrimSpace(modelID) != "" {
			return nil, fmt.Errorf("model not found")
		}
		return nil, fmt.Errorf("no LLM model configured")
	}

	provider, err := h.rt.Settings.GetProvider(ctx, userID, m.ProviderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("provider not found")
		}
		return nil, err
	}

	if strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.APIKey) == "" {
		return nil, fmt.Errorf("provider base_url or api_key is missing")
	}

	client, err := llm.NewClientForProvider(llm.ProviderConfig{
		ProviderType: provider.ProviderType,
		Endpoint:     provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        m.Model,
	})
	if err != nil {
		return nil, err
	}

	return &resolvedModel{
		Client:        client,
		ProviderType:  provider.ProviderType,
		ProviderID:    provider.ID,
		ModelID:       m.ID,
		ModelName:     m.Model,
		EnableKVCache: m.EnableKVCache,
		SafetyTier:    safetyTierFromOptions(m.Options),
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// sendSSE sends a Server-Sent Event.
func sendSSE(w io.Writer, flusher http.Flusher, event StreamEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type toolCaller interface {
	ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error)
}

type toolStreamingCaller interface {
	ChatCompletionStreamWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) (llm.ChatCompletionResult, error)
}

func runToolLoop(
	ctx context.Context,
	client toolCaller,
	messages []llm.ChatMessage,
	opts *llm.ChatCompletionOptions,
	defs []tool.Definition,
	broadcaster *StreamBroadcaster,
	sessionID string,
	userID string,
	resolved *resolvedModel,
	modelName string,
	enableTrace bool,
	sessions *sessionstore.Store,
) (string, bool, error) {
	if len(defs) == 0 {
		return "", false, fmt.Errorf("no tools configured")
	}

	if sessions == nil {
		return "", false, fmt.Errorf("session store not configured")
	}
	handlers := make(map[string]tool.Handler)
	for _, def := range defs {
		handlers[def.Spec.Function.Name] = def.Handler
	}

	// Inject userID into context for tool handlers to access user-specific settings
	if userID != "" {
		ctx = tool.ContextWithUserID(ctx, userID)
	}

	var combined strings.Builder
	var persisted bool

	const maxSteps = 20
	for step := 0; step < maxSteps; step++ {
		stepStreamID := uuid.NewString()
		broadcastMsg(broadcaster, streamMsg{
			Op:      "start",
			ID:      stepStreamID,
			Role:    model.MessageRoleAssistant,
			MsgType: model.MessageTypeText,
		})

		var (
			result llm.ChatCompletionResult
			err    error
		)

		if streamClient, ok := client.(toolStreamingCaller); ok {
			var stepContent strings.Builder
			var stepBuffer []byte
			result, err = streamClient.ChatCompletionStreamWithTools(ctx, messages, opts, func(chunk string) error {
				stepContent.WriteString(chunk)
				combined.WriteString(chunk)

				stepBuffer = append(stepBuffer, chunk...)
				valid, rest := splitBuffer(stepBuffer)
				stepBuffer = rest

				if len(valid) > 0 {
					broadcastMsg(broadcaster, streamMsg{
						Op:      "delta",
						ID:      stepStreamID,
						Role:    model.MessageRoleAssistant,
						MsgType: model.MessageTypeText,
						Delta:   string(valid),
					})
				}
				return nil
			})
			if err == nil && stepContent.Len() == 0 && result.Content != "" {
				combined.WriteString(result.Content)
				broadcastMsg(broadcaster, streamMsg{
					Op:      "delta",
					ID:      stepStreamID,
					Role:    model.MessageRoleAssistant,
					MsgType: model.MessageTypeText,
					Delta:   result.Content,
				})
			}
			if result.Content == "" {
				result.Content = stepContent.String()
			}
		} else {
			result, err = client.ChatCompletionWithTools(ctx, messages, opts)
			if err == nil && result.Content != "" {
				combined.WriteString(result.Content)
				broadcastMsg(broadcaster, streamMsg{
					Op:      "delta",
					ID:      stepStreamID,
					Role:    model.MessageRoleAssistant,
					MsgType: model.MessageTypeText,
					Delta:   result.Content,
				})
			}
		}
		if err != nil {
			broadcastMsg(broadcaster, streamMsg{
				Op:      "final",
				ID:      stepStreamID,
				Role:    model.MessageRoleAssistant,
				MsgType: model.MessageTypeText,
				Error:   err.Error(),
			})
			recordToolFailure(sessionID, userID, resolved, "", "", "", err)
			entry := model.NewTraceEntry(model.TraceTypeCustom, "Error")
			entry.Error = err.Error()
			entry.Complete()
			trace := model.TraceDataJSON{
				TraceData: model.TraceData{
					Entries: []model.TraceEntry{entry},
					Model:   modelName,
				},
			}
			msg := model.ChatMessage{
				Role:    model.MessageRoleAssistant,
				Type:    model.MessageTypeText,
				Content: "",
				Trace:   trace,
			}
			if _, persistErr := sessions.AppendMessage(sessionID, msg); persistErr == nil {
				persisted = true
			}
			return combined.String(), persisted, err
		}

		if len(result.ToolCalls) == 0 {
			broadcastMsg(broadcaster, streamMsg{
				Op:      "final",
				ID:      stepStreamID,
				Role:    model.MessageRoleAssistant,
				MsgType: model.MessageTypeText,
			})
			assistantMsg := model.ChatMessage{
				Role:    model.MessageRoleAssistant,
				Type:    model.MessageTypeText,
				Content: result.Content,
			}
			if _, persistErr := sessions.AppendMessage(sessionID, assistantMsg); persistErr == nil {
				persisted = true
			}

			// Return the content already streamed/broadcast across all steps so llm_calls response matches realtime output.
			return combined.String(), persisted, nil
		}

		var toolCallMsgID uint
		stepTraceEntries := make([]model.TraceEntry, 0, len(result.ToolCalls)*2)
		serialized, marshalErr := marshalPersistedToolCall("json", result.Content, result.Content, result.ToolCalls)
		if marshalErr != nil {
			serialized = result.Content
		}
		callMsg := model.ChatMessage{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeToolCall,
			Content: serialized,
		}
		if persistedCall, err := sessions.AppendMessage(sessionID, callMsg); err == nil {
			toolCallMsgID = persistedCall.ID
			persisted = true
		}
		broadcastMsg(broadcaster, streamMsg{
			Op:      "final",
			ID:      stepStreamID,
			Role:    model.MessageRoleAssistant,
			MsgType: model.MessageTypeToolCall,
			ToolCall: &persistedToolCallMessage{
				Protocol:   "json",
				Content:    result.Content,
				LLMContent: result.Content,
				ToolCalls:  result.ToolCalls,
			},
		})

		messages = append(messages, llm.ChatMessage{
			Role:      model.MessageRoleAssistant,
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		for _, call := range result.ToolCalls {
			isSubagent := call.Function.Name == "subagent"
			if enableTrace && !isSubagent {
				entry := model.NewTraceEntry(model.TraceTypeToolCall, call.Function.Name)
				entry.Input = map[string]any{
					"tool_call_id": call.ID,
					"arguments":    call.Function.Arguments,
				}
				entry.Metadata["tool_call_id"] = call.ID
				entry.Metadata["protocol"] = "json"
				entry.Complete()
				stepTraceEntries = append(stepTraceEntries, entry)
			}

			handler, ok := handlers[call.Function.Name]
			var (
				payload    any
				toolErr    error
				marshalErr error
			)
			if !ok {
				toolErr = fmt.Errorf("unknown tool: %s", call.Function.Name)
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Unknown tool: %s", call.Function.Name),
				})
				recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, toolErr)
				payload = map[string]string{"error": toolErr.Error()}
			} else {
				broadcaster.Broadcast(StreamEvent{
					Type: "trace",
					Data: fmt.Sprintf("Running tool: %s", call.Function.Name),
				})

				payload, toolErr = handler(ctx, json.RawMessage(call.Function.Arguments))
				if toolErr != nil {
					broadcaster.Broadcast(StreamEvent{
						Type: "error",
						Data: fmt.Sprintf("Tool %s failed: %v", call.Function.Name, toolErr),
					})
					recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, toolErr)

					// FEEDBACK: Return error to LLM so it can retry
					payload = map[string]string{
						"error": fmt.Sprintf("Tool execution failed: %v", toolErr),
					}
				}
			}

			response, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Tool %s response error: %v", call.Function.Name, marshalErr),
				})
				recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, marshalErr)

				// If marshaling fails, send a plain text error
				response = []byte(fmt.Sprintf(`{"error": "Failed to marshal tool response: %v"}`, marshalErr))
			}

			serialized, serializeErr := marshalPersistedToolResult("json", call.ID, call.Function.Name, call.Function.Arguments, string(response), nil)
			if serializeErr != nil {
				serialized = string(response)
			}
			resultMsg := model.ChatMessage{
				Role:    model.MessageRoleTool,
				Type:    model.MessageTypeToolResult,
				Content: serialized,
			}
			if toolCallMsgID != 0 {
				resultMsg.ParentID = &toolCallMsgID
			}
			if _, persistErr := sessions.AppendMessage(sessionID, resultMsg); persistErr == nil {
				persisted = true
			}
			broadcastMsg(broadcaster, streamMsg{
				Op:      "insert",
				ID:      uuid.NewString(),
				Role:    model.MessageRoleTool,
				MsgType: model.MessageTypeToolResult,
				ToolResult: &persistedToolResultMessage{
					Protocol:   "json",
					ToolCallID: call.ID,
					Name:       call.Function.Name,
					Arguments:  call.Function.Arguments,
					Content:    string(response),
				},
			})

			if enableTrace && isSubagent {
				var parsed struct {
					OK           bool   `json:"ok"`
					Summary      string `json:"summary"`
					FindingsPath string `json:"findings_path"`
					TraceLogPath string `json:"trace_log_path"`
					RunID        string `json:"run_id"`
					DurationMs   int64  `json:"duration_ms"`
					Error        string `json:"error"`
				}
				_ = json.Unmarshal(response, &parsed)

				entry := model.NewTraceEntry(model.TraceTypeSubAgent, call.Function.Name)
				entry.Input = map[string]any{
					"tool_call_id": call.ID,
					"arguments":    call.Function.Arguments,
				}
				entry.Output = map[string]any{
					"ok":             parsed.OK,
					"summary":        parsed.Summary,
					"findings_path":  parsed.FindingsPath,
					"trace_log_path": parsed.TraceLogPath,
					"run_id":         parsed.RunID,
					"duration_ms":    parsed.DurationMs,
					"error":          parsed.Error,
				}
				entry.Metadata["tool_call_id"] = call.ID
				entry.Metadata["protocol"] = "json"
				entry.Metadata["parent_session_id"] = sessionID
				if strings.TrimSpace(parsed.RunID) != "" {
					entry.Metadata["run_id"] = strings.TrimSpace(parsed.RunID)
				}
				if strings.TrimSpace(parsed.FindingsPath) != "" {
					entry.Metadata["findings_path"] = strings.TrimSpace(parsed.FindingsPath)
				}
				if strings.TrimSpace(parsed.TraceLogPath) != "" {
					entry.Metadata["trace_log_path"] = strings.TrimSpace(parsed.TraceLogPath)
				}
				if toolErr != nil {
					entry.Error = toolErr.Error()
				} else if marshalErr != nil {
					entry.Error = marshalErr.Error()
				} else if strings.TrimSpace(parsed.Error) != "" {
					entry.Error = strings.TrimSpace(parsed.Error)
				} else if !parsed.OK {
					entry.Error = "subagent failed"
				}
				entry.Complete()
				stepTraceEntries = append(stepTraceEntries, entry)
			} else if enableTrace {
				entry := model.NewTraceEntry(model.TraceTypeToolResult, call.Function.Name)
				entry.Output = string(response)
				if toolErr != nil {
					entry.Error = toolErr.Error()
				}
				if marshalErr != nil {
					if entry.Error == "" {
						entry.Error = marshalErr.Error()
					} else {
						entry.Error = entry.Error + "; " + marshalErr.Error()
					}
				}
				entry.Metadata["tool_call_id"] = call.ID
				entry.Metadata["protocol"] = "json"
				entry.Complete()
				stepTraceEntries = append(stepTraceEntries, entry)
			}

			rawToolContent := string(response)
			recordSuspiciousMatches(broadcaster, &stepTraceEntries, sessionID, userID, "tool_output", call.Function.Name, call.ID, rawToolContent)
			messages = append(messages, llm.ChatMessage{
				Role:       model.MessageRoleTool,
				Content:    wrapUntrustedToolOutput(call.Function.Name, call.ID, rawToolContent),
				ToolCallID: call.ID,
				Name:       call.Function.Name,
			})
		}

		if enableTrace && toolCallMsgID != 0 && len(stepTraceEntries) > 0 {
			trace := model.TraceDataJSON{
				TraceData: model.TraceData{
					Entries: stepTraceEntries,
					Model:   modelName,
				},
			}
			_ = sessions.UpdateMessageTrace(sessionID, toolCallMsgID, trace)
		}
	}

	recordToolFailure(sessionID, userID, resolved, "", "", "", fmt.Errorf("tool call limit reached"))
	err := fmt.Errorf("tool call limit reached")
	entry := model.NewTraceEntry(model.TraceTypeCustom, "Error")
	entry.Error = err.Error()
	entry.Complete()
	trace := model.TraceDataJSON{
		TraceData: model.TraceData{
			Entries: []model.TraceEntry{entry},
			Model:   modelName,
		},
	}
	msg := model.ChatMessage{
		Role:    model.MessageRoleAssistant,
		Type:    model.MessageTypeText,
		Content: "",
		Trace:   trace,
	}
	if _, persistErr := sessions.AppendMessage(sessionID, msg); persistErr == nil {
		persisted = true
	}
	return combined.String(), persisted, err
}

func recordToolFailure(sessionID, userID string, resolved *resolvedModel, toolName, toolCallID, args string, err error) {
	if err == nil {
		return
	}
	parts := []string{
		fmt.Sprintf("session_id=%s", sessionID),
		fmt.Sprintf("user_id=%s", userID),
		fmt.Sprintf("tool=%s", toolName),
		fmt.Sprintf("tool_call_id=%s", toolCallID),
		fmt.Sprintf("error=%v", err),
	}
	if strings.TrimSpace(args) != "" {
		parts = append(parts, fmt.Sprintf("args=%s", truncateString(args, 500)))
	}
	if resolved != nil {
		parts = append(parts,
			fmt.Sprintf("provider_id=%s", resolved.ProviderID),
			fmt.Sprintf("model_id=%s", resolved.ModelID),
			fmt.Sprintf("model=%s", resolved.ModelName),
		)
	}
	log.Printf("tool_failure: %s", strings.Join(parts, " "))
}

func extractToolIDs(metadata model.JSONB) ([]string, bool) {
	if metadata == nil {
		return nil, false
	}
	raw, ok := metadata["tool_ids"]
	if !ok {
		return nil, false
	}

	switch value := raw.(type) {
	case []string:
		return value, true
	case []any:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			if str, ok := item.(string); ok {
				ids = append(ids, str)
			}
		}
		return ids, true
	default:
		return nil, false
	}
}

func extractToolProtocol(metadata model.JSONB) (string, bool) {
	if metadata == nil {
		return "", false
	}
	raw, ok := metadata["tool_protocol"]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func extractWorkspace(metadata model.JSONB) (string, bool) {
	if metadata == nil {
		return "", false
	}
	raw, ok := metadata["workspace"]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func extractPromptCacheEpoch(metadata model.JSONB) int {
	if metadata == nil {
		return 0
	}
	raw, ok := metadata["prompt_cache_epoch"]
	if !ok || raw == nil {
		return 0
	}

	switch v := raw.(type) {
	case int:
		if v < 0 {
			return 0
		}
		return v
	case int64:
		if v < 0 {
			return 0
		}
		return int(v)
	case float64:
		if v < 0 {
			return 0
		}
		return int(v)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return 0
		}
		return n
	default:
		return 0
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func filterKnownToolIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	known := make(map[string]struct{})
	for _, def := range tool.All() {
		known[def.ID] = struct{}{}
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := known[trimmed]; ok {
			out = append(out, trimmed)
		}
	}
	return out
}

func intersectToolIDs(ids []string, targets []string) []string {
	if len(ids) == 0 || len(targets) == 0 {
		return nil
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		targetSet[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := targetSet[trimmed]; ok {
			out = append(out, trimmed)
		}
	}
	return out
}

func toolIDsFromDefinitions(defs []tool.Definition) []string {
	if len(defs) == 0 {
		return nil
	}

	ids := make([]string, 0, len(defs))
	for _, def := range defs {
		ids = append(ids, def.ID)
	}
	return ids
}

// ============================================================================
// STREAM MANAGER
// ============================================================================

// StreamManager manages active chat streams for SSE recovery.
type StreamManager struct {
	streams sync.Map // map[string]*StreamBroadcaster
}

func NewStreamManager() *StreamManager {
	return &StreamManager{}
}

func (sm *StreamManager) GetOrCreate(sessionID string) *StreamBroadcaster {
	// If exists, return it
	if val, ok := sm.streams.Load(sessionID); ok {
		return val.(*StreamBroadcaster)
	}

	// Create new
	sb := &StreamBroadcaster{
		sessionID: sessionID,
		clients:   make(map[chan StreamEvent]bool),
		manager:   sm,
	}
	// Use LoadOrStore to handle race conditions
	act, loaded := sm.streams.LoadOrStore(sessionID, sb)
	if loaded {
		return act.(*StreamBroadcaster)
	}
	return sb
}

func (sm *StreamManager) Remove(sessionID string) {
	sm.streams.Delete(sessionID)
}

// StreamBroadcaster handles broadcasting events to multiple clients (tabs) for the same session.
type StreamBroadcaster struct {
	sessionID string
	clients   map[chan StreamEvent]bool
	mu        sync.RWMutex
	manager   *StreamManager
	started   bool
	startMu   sync.Mutex
	history   []StreamEvent // Optional: store recent events for catch-up (not full replay)
}

// Subscribe adds a new client channel.
func (sb *StreamBroadcaster) Subscribe() chan StreamEvent {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	ch := make(chan StreamEvent, 50) // Buffered to prevent blocking
	sb.clients[ch] = true
	return ch
}

// Unsubscribe removes a client channel.
func (sb *StreamBroadcaster) Unsubscribe(ch chan StreamEvent) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if _, ok := sb.clients[ch]; !ok {
		return
	}
	delete(sb.clients, ch)
	close(ch)
}

// Broadcast sends an event to all connected clients.
func (sb *StreamBroadcaster) Broadcast(event StreamEvent) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	for ch := range sb.clients {
		select {
		case ch <- event:
		default:
			// If client is blocked, we skip (it's likely disconnected or too slow)
			// Ideally we should disconnect slow clients.
		}
	}
}

// StartGeneration atomically checks if this broadcaster should start generation.
// Returns true only for the first caller.
func (sb *StreamBroadcaster) StartGeneration() bool {
	sb.startMu.Lock()
	defer sb.startMu.Unlock()

	if sb.started {
		return false
	}
	sb.started = true
	return true
}

// Finish closes the broadcaster and removes it from manager.
func (sb *StreamBroadcaster) Finish() {
	sb.manager.Remove(sb.sessionID)
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for ch := range sb.clients {
		close(ch)
	}
	sb.clients = nil
}

// splitBuffer splits the byte slice at the last valid rune boundary.
// It returns the valid prefix (to be sent) and the remaining suffix (to be buffered).
func splitBuffer(b []byte) (toSend, toKeep []byte) {
	if len(b) == 0 {
		return nil, nil
	}

	// Fast path: if last byte is ASCII, we are safe.
	if b[len(b)-1] < 0x80 {
		return b, nil
	}

	// Walk backwards from the end (up to UTFMax bytes)
	// to find the start of the last sequence.
	limit := len(b)
	if limit > utf8.UTFMax {
		limit = utf8.UTFMax
	}

	for i := 1; i <= limit; i++ {
		start := len(b) - i
		// Check if the sequence starting here is a rune start
		if utf8.RuneStart(b[start]) {
			// Check if it forms a complete rune from here to end
			if utf8.FullRune(b[start:]) {
				return b, nil // Complete rune at end
			}
			// Incomplete rune at end
			return b[:start], b[start:]
		}
	}

	// If we exhausted lookback and found no start, it's either:
	// 1. Just continuation bytes (invalid UTF-8 independently)
	// 2. A long sequence of garbage
	// In strict mode, we might want to buffer, but if it's invalid, it stays invalid.
	// We treat it as "complete" so it gets flushed (and replaced by replacement chars).
	return b, nil
}
