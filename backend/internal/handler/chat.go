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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/database"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
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
const DefaultSystemPrompt = `You are a helpful AI assistant. Be concise and helpful.`

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
}

// StreamEvent represents a Server-Sent Event.
// EXTENSION: Add new event types for tool calls, progress, etc.
type StreamEvent struct {
	Type string `json:"type"` // "session", "content", "trace", "done", "error"
	Data string `json:"data"`
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
	cfg           *config.Config
	streamManager *StreamManager
}

// NewChatHandler creates a new ChatHandler with LLM client.
func NewChatHandler(cfg *config.Config) *ChatHandler {
	return &ChatHandler{
		cfg:           cfg,
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

	userID := middleware.GetUserID(c)
	db := database.GetDB()

	// Create or get session
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Build conversation history
	var messages []llm.ChatMessage

	// Add system prompt
	systemPrompt := DefaultSystemPrompt
	if req.SystemPrompt != "" {
		systemPrompt = req.SystemPrompt
	}
	messages = append(messages, llm.BuildSystemMessage(systemPrompt))

	// Ensure session exists and get history
	var session model.ChatSession
	if db != nil {
		result := db.Where("id = ?", sessionID).First(&session)
		if result.Error != nil {
			// Create new session
			sessionMetadata := model.JSONB{}
			session = model.ChatSession{
				ID:       sessionID,
				UserID:   userID,
				Title:    truncateString(req.Message, 100),
				Module:   ChatModule,
				Metadata: sessionMetadata,
			}
			db.Create(&session)
		}

		// Load message history for context
		// EXTENSION: Add sliding window or summarization for long conversations
		var dbMessages []model.ChatMessage
		db.Where("session_id = ?", sessionID).Order("id ASC").Find(&dbMessages)
		for _, msg := range dbMessages {
			messages = append(messages, llm.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Add current user message
	messages = append(messages, llm.BuildUserMessage(req.Message))

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

	toolDefs, err := tool.Mount(selectedToolIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	selectedToolIDs = toolIDsFromDefinitions(toolDefs)

	if toolProtocol == "xml" && len(toolDefs) > 0 && len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content = strings.TrimSpace(messages[0].Content) + "\n\n" + toolxml.SystemPrompt(toolDefs)
	}

	resolvedModel, err := h.resolveModel(db, userID, selectedModelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if db != nil && resolvedModel.ModelID != "" {
		metadata := session.Metadata
		if metadata == nil {
			metadata = model.JSONB{}
		}

		shouldUpdate := false
		if _, ok := metadata["model_id"]; !ok || req.ModelID != "" {
			metadata["model_id"] = resolvedModel.ModelID
			shouldUpdate = true
		}
		if toolIDsSet {
			metadata["tool_ids"] = selectedToolIDs
			shouldUpdate = true
		}
		if toolProtocolSet {
			metadata["tool_protocol"] = toolProtocol
			shouldUpdate = true
		}

		if shouldUpdate {
			db.Model(&model.ChatSession{}).Where("id = ?", sessionID).Update("metadata", metadata)
		}
	}

	if db != nil {
		// Save user message only after model resolution succeeds.
		userMsg := model.ChatMessage{
			SessionID: sessionID,
			Role:      model.MessageRoleUser,
			Type:      model.MessageTypeText,
			Content:   req.Message,
		}
		db.Create(&userMsg)
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
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
				tokenCount++

				// Throttle updates: every 5 tokens or 100ms
				now := time.Now()
				if tokenCount%5 == 0 || now.Sub(lastBroadcast) > 100*time.Millisecond {
					broadcaster.Broadcast(StreamEvent{
						Type: "usage",
						Data: fmt.Sprintf(`{"response_tokens": %d}`, tokenCount),
					})
					lastBroadcast = now
				}
			}

			// Trace internal steps (fake ones purely for UI experience if config enabled)
			if h.cfg.EnableTrace {
				traceEntry := model.NewTraceEntry(model.TraceTypeLLMCall, "ChatCompletion")
				traceEntry.Input = messages
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

			opts := &llm.ChatCompletionOptions{
				Trace: traceCallback,
			}
			if resolvedModel.EnableKVCache {
				opts.EnablePromptCache = true
				if llm.SupportsPromptCacheKey(resolvedModel.ProviderType) {
					opts.PromptCacheKey = sessionID
				}
			}
			if toolProtocol == "json" && len(toolDefs) > 0 {
				opts.Tools = tool.ToolsForLLM(toolDefs)
			}

			var llmCallID uint
			if db != nil {
				messagesJSON, err := json.Marshal(messages)
				if err == nil {
					call := model.LLMCall{
						SessionID:  sessionID,
						UserID:     userID,
						ProviderID: resolvedModel.ProviderID,
						ModelID:    resolvedModel.ModelID,
						ModelName:  resolvedModel.ModelName,
						Messages:   string(messagesJSON),
						CreatedAt:  time.Now(),
					}
					if err := db.Create(&call).Error; err == nil {
						llmCallID = call.ID
					}
				}
			}

			var err error
			if len(toolDefs) > 0 && toolProtocol == "xml" {
				fullContent, err = toolxml.RunLoop(
					ctx,
					resolvedModel.Client,
					messages,
					opts,
					toolDefs,
					func(chunk string) error {
						broadcaster.Broadcast(StreamEvent{Type: "content", Data: chunk})
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
					fullContent, err = runToolLoop(ctx, toolClient, messages, opts, toolDefs, broadcaster, sessionID, userID, resolvedModel)
				}
			} else {
				err = resolvedModel.Client.ChatCompletionStream(ctx, messages, opts, func(chunk string) error {
					fullContent += chunk

					// Buffer content to ensure we only broadcast complete UTF-8 runes
					incompleteUTF8 = append(incompleteUTF8, chunk...)
					valid, rest := splitBuffer(incompleteUTF8)
					incompleteUTF8 = rest

					if len(valid) > 0 {
						broadcaster.Broadcast(StreamEvent{
							Type: "content",
							Data: string(valid),
						})
					}
					return nil
				})
			}

			if err != nil {
				log.Printf("LLM stream error: %v", err)
				// Error broadcast handled in OnComplete trace or here
			}

			if db != nil && llmCallID != 0 {
				update := map[string]any{
					"response": fullContent,
				}
				if err != nil {
					update["error"] = err.Error()
				}
				db.Model(&model.LLMCall{}).Where("id = ?", llmCallID).Updates(update)
			}

			// Complete trace
			if h.cfg.EnableTrace && len(traceEntries) > 0 {
				traceEntries[0].Complete()
				traceEntries[0].Output = fullContent
				if err != nil {
					traceEntries[0].Error = err.Error()
				}
			}

			// Save assistant message to database (also persist error cases so history shows an "air bubble" + trace).
			if db != nil && (fullContent != "" || err != nil) {
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
					SessionID: sessionID,
					Role:      model.MessageRoleAssistant,
					Type:      model.MessageTypeText,
					Content:   fullContent,
					Trace:     traceData,
				}
				db.Create(&assistantMsg)

				// Update session timestamp
				db.Model(&model.ChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now())
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
	db := database.GetDB()

	if db == nil {
		c.JSON(http.StatusOK, []model.ChatSession{})
		return
	}

	// Filter by module for multi-module support
	// EXTENSION: Make module a query parameter
	var sessions []model.ChatSession
	db.Where("user_id = ? AND module = ?", userID, ChatModule).
		Order("updated_at DESC").
		Find(&sessions)

	c.JSON(http.StatusOK, sessions)
}

// GetSession returns a specific session with messages.
// GET /api/sessions/:id
func (h *ChatHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)
	db := database.GetDB()

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	var session model.ChatSession
	result := db.Where("id = ? AND user_id = ?", sessionID, userID).
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		First(&session)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes a chat session.
// DELETE /api/sessions/:id
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)
	db := database.GetDB()

	if db == nil {
		c.Status(http.StatusNoContent)
		return
	}

	// Delete messages first
	db.Where("session_id = ?", sessionID).Delete(&model.ChatMessage{})

	// Delete session
	result := db.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&model.ChatSession{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// TruncateSession deletes messages in a session starting from a specific message ID (inclusive).
// POST /api/sessions/:id/truncate
func (h *ChatHandler) TruncateSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := middleware.GetUserID(c)
	db := database.GetDB()

	if db == nil {
		c.Status(http.StatusNoContent)
		return
	}

	var req TruncateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Ensure session belongs to user.
	var session model.ChatSession
	if err := tx.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Delete messages from the specified message (inclusive).
	if err := tx.Where("session_id = ? AND id >= ?", sessionID, req.FromMessageID).Delete(&model.ChatMessage{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to truncate messages"})
		return
	}

	// Update session timestamp.
	if err := tx.Model(&model.ChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now()).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
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
}

func (h *ChatHandler) resolveModel(db *gorm.DB, userID, modelID string) (*resolvedModel, error) {
	if db == nil {
		if modelID != "" {
			return nil, fmt.Errorf("database not configured for model lookup")
		}
		return nil, fmt.Errorf("no LLM model configured")
	}

	var llmModel model.LLMModel
	if modelID != "" {
		if err := db.Where("id = ? AND user_id = ?", modelID, userID).First(&llmModel).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("model not found")
			}
			return nil, err
		}
	} else {
		if err := db.Where("user_id = ? AND is_default = ?", userID, true).
			Order("updated_at DESC").
			First(&llmModel).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
	}

	if llmModel.ID != "" {
		var provider model.LLMProvider
		if err := db.Where("id = ? AND user_id = ?", llmModel.ProviderID, userID).First(&provider).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("provider not found")
			}
			return nil, err
		}

		if provider.BaseURL == "" || provider.APIKey == "" {
			return nil, fmt.Errorf("provider base_url or api_key is missing")
		}

		client, err := llm.NewClientForProvider(llm.ProviderConfig{
			ProviderType: provider.ProviderType,
			Endpoint:     provider.BaseURL,
			APIKey:       provider.APIKey,
			Model:        llmModel.Model,
		})
		if err != nil {
			return nil, err
		}

		return &resolvedModel{
			Client:        client,
			ProviderType:  provider.ProviderType,
			ProviderID:    provider.ID,
			ModelID:       llmModel.ID,
			ModelName:     llmModel.Model,
			EnableKVCache: llmModel.EnableKVCache,
		}, nil
	}

	if modelID != "" {
		return nil, fmt.Errorf("model not found")
	}

	return nil, fmt.Errorf("no LLM model configured")
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
) (string, error) {
	if len(defs) == 0 {
		return "", fmt.Errorf("no tools configured")
	}

	handlers := make(map[string]tool.Handler)
	for _, def := range defs {
		handlers[def.Spec.Function.Name] = def.Handler
	}

	var combined strings.Builder

	const maxSteps = 20
	for step := 0; step < maxSteps; step++ {
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
					broadcaster.Broadcast(StreamEvent{
						Type: "content",
						Data: string(valid),
					})
				}
				return nil
			})
			if err == nil && stepContent.Len() == 0 && result.Content != "" {
				combined.WriteString(result.Content)
				broadcaster.Broadcast(StreamEvent{
					Type: "content",
					Data: result.Content,
				})
			}
			if result.Content == "" {
				result.Content = stepContent.String()
			}
		} else {
			result, err = client.ChatCompletionWithTools(ctx, messages, opts)
			if err == nil && result.Content != "" {
				combined.WriteString(result.Content)
				broadcaster.Broadcast(StreamEvent{
					Type: "content",
					Data: result.Content,
				})
			}
		}
		if err != nil {
			recordToolFailure(sessionID, userID, resolved, "", "", "", err)
			return combined.String(), err
		}

		if len(result.ToolCalls) == 0 {
			// Return the content already streamed/broadcast across all steps so history matches realtime output.
			return combined.String(), nil
		}

		messages = append(messages, llm.ChatMessage{
			Role:      model.MessageRoleAssistant,
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		for _, call := range result.ToolCalls {
			handler, ok := handlers[call.Function.Name]
			if !ok {
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Unknown tool: %s", call.Function.Name),
				})
				recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, fmt.Errorf("unknown tool"))
				return combined.String(), fmt.Errorf("unknown tool: %s", call.Function.Name)
			}

			broadcaster.Broadcast(StreamEvent{
				Type: "trace",
				Data: fmt.Sprintf("Running tool: %s", call.Function.Name),
			})

			payload, err := handler(ctx, json.RawMessage(call.Function.Arguments))
			if err != nil {
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Tool %s failed: %v", call.Function.Name, err),
				})
				recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, err)

				// FEEDBACK: Return error to LLM so it can retry
				payload = map[string]string{
					"error": fmt.Sprintf("Tool execution failed: %v", err),
				}
			}

			response, err := json.Marshal(payload)
			if err != nil {
				broadcaster.Broadcast(StreamEvent{
					Type: "error",
					Data: fmt.Sprintf("Tool %s response error: %v", call.Function.Name, err),
				})
				recordToolFailure(sessionID, userID, resolved, call.Function.Name, call.ID, call.Function.Arguments, err)

				// If marshaling fails, send a plain text error
				response = []byte(fmt.Sprintf(`{"error": "Failed to marshal tool response: %v"}`, err))
			}

			messages = append(messages, llm.ChatMessage{
				Role:       model.MessageRoleTool,
				Content:    string(response),
				ToolCallID: call.ID,
				Name:       call.Function.Name,
			})
		}
	}

	recordToolFailure(sessionID, userID, resolved, "", "", "", fmt.Errorf("tool call limit reached"))
	return combined.String(), fmt.Errorf("tool call limit reached")
}

func recordToolFailure(sessionID, userID string, resolved *resolvedModel, toolName, toolCallID, args string, err error) {
	if err == nil {
		return
	}
	db := database.GetDB()
	if db == nil {
		return
	}

	failure := model.ToolCallFailure{
		SessionID:  sessionID,
		UserID:     userID,
		ToolName:   toolName,
		ToolCallID: toolCallID,
		Arguments:  args,
		Error:      err.Error(),
	}
	if resolved != nil {
		failure.ProviderID = resolved.ProviderID
		failure.ModelID = resolved.ModelID
		failure.ModelName = resolved.ModelName
	}

	_ = db.Create(&failure).Error
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
