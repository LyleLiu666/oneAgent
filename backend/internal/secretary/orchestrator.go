package secretary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/memorydb"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type ResolveModelFunc func(ctx context.Context, userID, modelID string) (client llm.Client, resolvedModelID string, err error)

type Orchestrator struct {
	Sessions *sessionstore.Store
	Tasks    *taskqueue.Store
	Runner   *taskqueue.TaskRunner
	Memory   *memorydb.DB

	ResolveModel ResolveModelFunc

	DefaultWorkspacePoolRoot string

	muBySession sync.Map // map[string]*sync.Mutex (best-effort triage mutex)
}

const (
	secretaryModuleSU = "secretary"
	secretaryModuleSW = "secretary_sw"
)

func (o *Orchestrator) lock(sessionID string) *sync.Mutex {
	val, _ := o.muBySession.LoadOrStore(sessionID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func deriveSWSessionID(suSessionID string) string {
	suSessionID = strings.TrimSpace(suSessionID)
	if suSessionID == "" {
		return ""
	}
	return suSessionID + "-sw"
}

func buildTextLLMHistory(dbMessages []model.ChatMessage) []llm.ChatMessage {
	out := make([]llm.ChatMessage, 0, len(dbMessages))
	for _, msg := range dbMessages {
		if msg.Type != model.MessageTypeText {
			continue
		}
		if msg.Role != model.MessageRoleUser && msg.Role != model.MessageRoleAssistant {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		out = append(out, llm.ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	return out
}

func (o *Orchestrator) AppendInboxMessage(ctx context.Context, userID, sessionID, content, workspace string) (InboxAppendResult, error) {
	if o == nil || o.Sessions == nil {
		return InboxAppendResult{}, errors.New("sessions store not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return InboxAppendResult{}, errors.New("content is required")
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	title := truncateString(content, 100)
	if _, err := o.Sessions.GetOrCreateSession(sessionID, userID, secretaryModuleSU, title); err != nil {
		return InboxAppendResult{}, err
	}

	session, msgs, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		return InboxAppendResult{}, err
	}

	workspace = strings.TrimSpace(workspace)
	if workspace != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(workspace)
		if err != nil {
			return InboxAppendResult{}, err
		}

		meta := session.Metadata
		if meta == nil {
			meta = model.JSONB{}
		}
		if existing, ok := meta["workspace"].(string); ok && strings.TrimSpace(existing) != "" && existing != normalized {
			return InboxAppendResult{}, fmt.Errorf("cannot change workspace for an existing session")
		}
		meta["workspace"] = normalized
		_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)
		session.Metadata = meta
	}

	// Best-effort: compress the SU session before appending a new message so the
	// triage cursor can advance safely even when IDs are rewritten.
	if o.ResolveModel != nil && o.Sessions != nil {
		modelID := ""
		if v, ok := session.Metadata["model_id"].(string); ok {
			modelID = strings.TrimSpace(v)
		}
		if client, _, err := o.ResolveModel(ctx, userID, modelID); err == nil && client != nil {
			llmMessages := make([]llm.ChatMessage, 0, 2+len(msgs))
			llmMessages = append(llmMessages, llm.BuildSystemMessage("ONEAGENT_SECRETARY_SU_CONTEXT"))
			llmMessages = append(llmMessages, buildTextLLMHistory(msgs)...)
			llmMessages = append(llmMessages, llm.BuildUserMessage(content))

			compressed, _, err := sessioncompress.CompressSessionIfNeeded(ctx, o.Sessions, sessionID, msgs, llmMessages, client, sessioncompress.DefaultOptions())
			if err == nil && compressed {
				// Reload after rewrite.
				session, msgs, err = o.Sessions.GetSessionWithMessages(sessionID, userID)
				if err != nil {
					return InboxAppendResult{}, err
				}

				// Advance triage cursor to the end so we don't re-dispatch old work after compression.
				endID := uint(0)
				if len(msgs) > 0 {
					endID = msgs[len(msgs)-1].ID
				}
				st, _ := decodeState(session.Metadata)
				st.CursorMessageID = endID
				st.TriageRuns = nil
				meta := upsertState(session.Metadata, st)
				_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)
				session.Metadata = meta
			}
		}
	}

	userMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: content,
	})
	if err != nil {
		return InboxAppendResult{}, err
	}

	ackText, resolvedModelID, err := o.generateQuickAck(ctx, userID, session.Metadata, content)
	if err != nil || strings.TrimSpace(ackText) == "" {
		// Quick ack is best-effort. Persist a deterministic fallback so the UI
		// stays responsive even when the LLM returns no content or the provider
		// doesn't stream SSE.
		ackText = fallbackQuickAckText(content)
		resolvedModelID = ""
	}

	// Persist resolved model id for consistent behavior across refreshes (best-effort).
	if resolvedModelID != "" {
		meta := session.Metadata
		if meta == nil {
			meta = model.JSONB{}
		}
		if _, ok := meta["model_id"]; !ok {
			meta["model_id"] = resolvedModelID
			_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)
			session.Metadata = meta
		}
	}

	parentID := userMsg.ID
	ackMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:     model.MessageRoleAssistant,
		Type:     model.MessageTypeText,
		Content:  ackText,
		ParentID: &parentID,
	})
	if err != nil {
		return InboxAppendResult{}, err
	}

	// Best-effort: record a user-facing worklog entry so SW (or future agents) can sync it.
	if o.Memory != nil {
		_, _ = o.Memory.AppendEntry(ctx, memorydb.Entry{
			PrincipalID: userID,
			Writer:      "SU",
			Type:        "worklog",
			Workspace:   workspace,
			Title:       "user_message",
			Content:     content,
		})
	}

	return InboxAppendResult{
		SessionID:    sessionID,
		MessageID:    userMsg.ID,
		AckMessageID: ackMsg.ID,
		AckText:      ackText,
	}, nil
}

func (o *Orchestrator) Triage(ctx context.Context, userID, sessionID string, cursorOverride *uint) (TriageResult, error) {
	if o == nil || o.Sessions == nil {
		return TriageResult{}, errors.New("sessions store not initialized")
	}
	if o.Tasks == nil || o.Runner == nil {
		return TriageResult{}, errors.New("task queue not initialized")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return TriageResult{}, errors.New("sessionID is required")
	}

	// Best-effort per-session mutex.
	mu := o.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, msgs, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		return TriageResult{}, err
	}

	state, _ := decodeState(session.Metadata)
	cursor := state.CursorMessageID
	if cursorOverride != nil {
		cursor = *cursorOverride
	}

	newUserMsgs := make([]model.ChatMessage, 0, 8)
	for _, m := range msgs {
		if m.ID <= cursor {
			continue
		}
		if m.Role != model.MessageRoleUser || m.Type != model.MessageTypeText {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		newUserMsgs = append(newUserMsgs, m)
	}

	if len(newUserMsgs) == 0 {
		return TriageResult{
			SummaryMessage:    "",
			SummaryMessageID:  0,
			CursorMessageID:   cursor,
			CreatedTaskIDs:    []string{},
			Questions:         []string{},
			WorkspacesCreated: []string{},
		}, nil
	}

	toID := newUserMsgs[len(newUserMsgs)-1].ID
	if run, ok := findRun(state, cursor, toID); ok {
		return TriageResult{
			SummaryMessage:    run.SummaryMessage,
			SummaryMessageID:  run.SummaryMessageID,
			CursorMessageID:   state.CursorMessageID,
			CreatedTaskIDs:    append([]string{}, run.CreatedTaskIDs...),
			Questions:         append([]string{}, run.Questions...),
			WorkspacesCreated: append([]string{}, run.WorkspacesCreated...),
		}, nil
	}

	sessionWorkspace := ""
	if raw, ok := session.Metadata["workspace"]; ok {
		if v, ok := raw.(string); ok {
			sessionWorkspace = strings.TrimSpace(v)
		}
	}

	// If the user is asking progress/status, answer directly instead of dispatching workers.
	if len(newUserMsgs) == 1 && looksLikeProgressQuery(newUserMsgs[0].Content) {
		summary, questions, err := o.buildProgressReply(userID, sessionWorkspace, state)
		if err != nil {
			return TriageResult{}, err
		}

		summaryMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeText,
			Content: summary,
		})
		if err != nil {
			return TriageResult{}, err
		}

		state.CursorMessageID = toID
		state.TriageRuns = append(state.TriageRuns, TriageRun{
			FromCursor:        cursor,
			ToMessageID:       toID,
			InputMessageIDs:   messageIDs(newUserMsgs),
			SummaryMessageID:  summaryMsg.ID,
			SummaryMessage:    summary,
			CreatedTaskIDs:    []string{},
			Questions:         append([]string{}, questions...),
			WorkspacesCreated: []string{},
			CreatedAt:         time.Now().UTC(),
		})
		if len(state.TriageRuns) > 20 {
			state.TriageRuns = state.TriageRuns[len(state.TriageRuns)-20:]
		}

		meta := upsertState(session.Metadata, state)
		_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)

		if o.Memory != nil {
			_, _ = o.Memory.AppendEntry(ctx, memorydb.Entry{
				PrincipalID: userID,
				Writer:      "SU",
				Type:        "findings",
				Workspace:   sessionWorkspace,
				Title:       "progress_reply",
				Content:     summary,
			})
		}

		return TriageResult{
			SummaryMessage:    summary,
			SummaryMessageID:  summaryMsg.ID,
			CursorMessageID:   state.CursorMessageID,
			CreatedTaskIDs:    []string{},
			Questions:         questions,
			WorkspacesCreated: []string{},
		}, nil
	}

	// SW: dispatch planning + worker coordination (best-effort).
	plan, resolvedModelID, err := o.generateDispatchPlan(ctx, userID, sessionID, session.Metadata, newUserMsgs)
	if err != nil {
		return TriageResult{}, err
	}

	// Persist resolved model id for consistent behavior across refreshes (best-effort).
	if resolvedModelID != "" {
		meta := session.Metadata
		if meta == nil {
			meta = model.JSONB{}
		}
		if _, ok := meta["model_id"]; !ok {
			meta["model_id"] = resolvedModelID
			_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)
			session.Metadata = meta
		}
	}

	dispatch, err := o.dispatchPlanAsSW(ctx, userID, sessionID, sessionWorkspace, plan)
	if err != nil {
		return TriageResult{}, err
	}
	createdTaskIDs := dispatch.CreatedTaskIDs
	questions := dispatch.Questions
	workspacesCreated := dispatch.WorkspacesCreated

	summary := buildTriageSummary(plan.SummaryMessage, len(createdTaskIDs), questions)

	summaryMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleAssistant,
		Type:    model.MessageTypeText,
		Content: summary,
	})
	if err != nil {
		return TriageResult{}, err
	}

	state.CursorMessageID = toID
	state.TriageRuns = append(state.TriageRuns, TriageRun{
		FromCursor:        cursor,
		ToMessageID:       toID,
		InputMessageIDs:   messageIDs(newUserMsgs),
		SummaryMessageID:  summaryMsg.ID,
		SummaryMessage:    summary,
		CreatedTaskIDs:    append([]string{}, createdTaskIDs...),
		Questions:         append([]string{}, questions...),
		WorkspacesCreated: append([]string{}, workspacesCreated...),
		CreatedAt:         time.Now().UTC(),
	})
	if len(state.TriageRuns) > 20 {
		state.TriageRuns = state.TriageRuns[len(state.TriageRuns)-20:]
	}

	meta := upsertState(session.Metadata, state)
	_ = o.Sessions.UpdateSessionMetadata(sessionID, meta)

	if o.Memory != nil {
		_, _ = o.Memory.AppendEntry(ctx, memorydb.Entry{
			PrincipalID: userID,
			Writer:      "SU",
			Type:        "findings",
			Workspace:   sessionWorkspace,
			Title:       "triage_summary",
			Content:     summary,
		})
	}

	return TriageResult{
		SummaryMessage:    summary,
		SummaryMessageID:  summaryMsg.ID,
		CursorMessageID:   state.CursorMessageID,
		CreatedTaskIDs:    createdTaskIDs,
		Questions:         questions,
		WorkspacesCreated: workspacesCreated,
	}, nil
}

type dispatchResult struct {
	CreatedTaskIDs    []string
	Questions         []string
	WorkspacesCreated []string
}

// dispatchPlanAsSW performs side effects: workspace creation + task creation/enqueue.
// SU is user-facing and should stay read-mostly; SW owns dispatch (best-effort).
func (o *Orchestrator) dispatchPlanAsSW(ctx context.Context, userID, sessionID, sessionWorkspace string, plan triagePlan) (dispatchResult, error) {
	if o == nil {
		return dispatchResult{}, errors.New("orchestrator is nil")
	}
	if o.Tasks == nil || o.Runner == nil {
		return dispatchResult{}, errors.New("task queue not initialized")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return dispatchResult{}, errors.New("sessionID is required")
	}
	sessionWorkspace = strings.TrimSpace(sessionWorkspace)

	createdTaskIDs := make([]string, 0, len(plan.Tasks))
	questions := append([]string{}, plan.Questions...)
	workspacesCreated := make([]string, 0, 2)

	for _, task := range plan.Tasks {
		title := strings.TrimSpace(task.Title)
		prompt := strings.TrimSpace(task.Prompt)
		if prompt == "" {
			continue
		}
		if title == "" {
			title = deriveTaskTitle(prompt)
		}

		strategy := strings.ToLower(strings.TrimSpace(task.WorkspaceStrategy))
		workspace := ""

		switch strategy {
		case "new":
			ws, err := o.createWorkspace(ctx, sessionID)
			if err != nil {
				return dispatchResult{}, err
			}
			workspacesCreated = append(workspacesCreated, ws)
			workspace = ws

		case "session":
			if sessionWorkspace == "" {
				questions = append(questions, fmt.Sprintf("要继续「%s」，我需要你告诉我在哪个项目目录里做（发我仓库根目录路径即可）。", title))
				continue
			}
			workspace = sessionWorkspace

		case "ask":
			questions = append(questions, fmt.Sprintf("「%s」需要你指定要操作的项目目录。你把目录路径发我就行。", title))
			continue

		default:
			questions = append(questions, fmt.Sprintf("「%s」我暂时判断不出要用哪个项目目录。你发我一个目录路径（仓库根目录）我就继续。", title))
			continue
		}

		limits := taskqueue.ResolveLimits(taskqueue.Limits{})
		created, err := o.Tasks.CreateTask(userID, workspace, title, prompt, "", limits)
		if err != nil {
			return dispatchResult{}, err
		}
		createdTaskIDs = append(createdTaskIDs, created.ID)

		if err := o.Runner.Enqueue(created.ID); err != nil {
			return dispatchResult{}, err
		}
	}

	// Best-effort: record SW dispatch worklog for SU sync later.
	if o.Memory != nil && (len(createdTaskIDs) > 0 || len(questions) > 0 || len(workspacesCreated) > 0) {
		var b strings.Builder
		if len(createdTaskIDs) > 0 {
			b.WriteString("created_task_ids:\n")
			for _, id := range createdTaskIDs {
				b.WriteString("- " + strings.TrimSpace(id) + "\n")
			}
		}
		if len(workspacesCreated) > 0 {
			b.WriteString("workspaces_created:\n")
			for _, ws := range workspacesCreated {
				b.WriteString("- " + strings.TrimSpace(ws) + "\n")
			}
		}
		if len(questions) > 0 {
			b.WriteString("questions:\n")
			for _, q := range questions {
				q = strings.TrimSpace(q)
				if q == "" {
					continue
				}
				b.WriteString("- " + q + "\n")
			}
		}
		content := strings.TrimSpace(b.String())
		if content != "" {
			_, _ = o.Memory.AppendEntry(ctx, memorydb.Entry{
				PrincipalID: userID,
				Writer:      "SW",
				Type:        "worklog",
				Workspace:   sessionWorkspace,
				Title:       "triage_dispatch",
				Content:     content,
			})
		}
	}

	return dispatchResult{
		CreatedTaskIDs:    createdTaskIDs,
		Questions:         questions,
		WorkspacesCreated: workspacesCreated,
	}, nil
}

func (o *Orchestrator) GetState(_ context.Context, userID, sessionID string) (StateResult, error) {
	if o == nil || o.Sessions == nil {
		return StateResult{}, errors.New("sessions store not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return StateResult{}, errors.New("sessionID is required")
	}

	session, _, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		return StateResult{}, err
	}
	state, _ := decodeState(session.Metadata)
	return StateResult{CursorMessageID: state.CursorMessageID, TriageRuns: state.TriageRuns}, nil
}

type triagePlan struct {
	SummaryMessage string       `json:"summary_message"`
	Tasks          []triageTask `json:"tasks,omitempty"`
	Questions      []string     `json:"questions,omitempty"`
}

type triageTask struct {
	Title             string `json:"title"`
	Prompt            string `json:"prompt"`
	WorkspaceStrategy string `json:"workspace_strategy"`
}

func (o *Orchestrator) generateQuickAck(ctx context.Context, userID string, metadata model.JSONB, userContent string) (ackText string, resolvedModelID string, err error) {
	if o.ResolveModel == nil {
		return "", "", errors.New("ResolveModel is required")
	}
	modelID := ""
	if v, ok := metadata["model_id"].(string); ok {
		modelID = strings.TrimSpace(v)
	}
	client, resolvedID, err := o.ResolveModel(ctx, userID, modelID)
	if err != nil {
		return "", "", err
	}

	sys := strings.TrimSpace(secretaryAckSystemPrompt)
	msgs := []llm.ChatMessage{llm.BuildSystemMessage(sys)}
	if injected := o.buildMemorySyncPrompt(ctx, userID, "SU", "SW"); strings.TrimSpace(injected) != "" {
		msgs = append(msgs, llm.BuildUserMessage(injected))
	}
	msgs = append(msgs, llm.BuildUserMessage(userContent))

	temp := 0.2
	maxTokens := 80
	out, err := client.ChatCompletion(ctx, msgs, &llm.ChatCompletionOptions{
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return "", "", err
	}

	ack := strings.TrimSpace(out)
	ack = strings.Trim(ack, "\"")
	ack = sanitizeQuickAckText(userContent, ack)
	if ack == "" {
		return "", "", errors.New("empty quick ack")
	}
	return ack, resolvedID, nil
}

func (o *Orchestrator) generateDispatchPlan(ctx context.Context, userID, sessionID string, metadata model.JSONB, msgs []model.ChatMessage) (triagePlan, string, error) {
	if o.ResolveModel == nil {
		return triagePlan{}, "", errors.New("ResolveModel is required")
	}
	if o.Sessions == nil {
		return triagePlan{}, "", errors.New("sessions store not initialized")
	}
	modelID := ""
	if v, ok := metadata["model_id"].(string); ok {
		modelID = strings.TrimSpace(v)
	}
	client, resolvedID, err := o.ResolveModel(ctx, userID, modelID)
	if err != nil {
		return triagePlan{}, "", err
	}
	if client == nil {
		return triagePlan{}, "", errors.New("resolved model client is nil")
	}

	var b strings.Builder
	for i, m := range msgs {
		b.WriteString(fmt.Sprintf("%d) %s\n", i+1, strings.TrimSpace(m.Content)))
	}
	input := strings.TrimSpace(b.String())
	if input == "" {
		return triagePlan{}, "", errors.New("no messages to triage")
	}

	sys := strings.TrimSpace(secretaryDispatchSystemPromptSW)
	user := "以下是用户自上次归并以来的消息列表（按时间顺序）：\n" + input + "\n\n请按 schema 输出 JSON。"
	// SW: before dispatching, pull-sync the latest SU entries (best-effort).
	if injected := strings.TrimSpace(o.buildMemorySyncPrompt(ctx, userID, "SW", "SU")); injected != "" {
		user = injected + "\n\n" + user
	}

	temp := 0.2
	maxTokens := 1500
	requestMsgs := []llm.ChatMessage{llm.BuildSystemMessage(sys)}

	swSessionID := deriveSWSessionID(sessionID)
	if swSessionID != "" {
		_, _ = o.Sessions.GetOrCreateSession(swSessionID, userID, secretaryModuleSW, "Secretary(Worker)")
		_, swMsgs, _ := o.Sessions.GetSessionWithMessages(swSessionID, userID)
		requestMsgs = append(requestMsgs, buildTextLLMHistory(swMsgs)...)

		// Apply fixed 80k compression per SW channel (best-effort).
		cOpts := sessioncompress.DefaultOptions()
		cOpts.SummaryPrefix = sessioncompress.DefaultSummaryPrefix
		llmMessages := append(append([]llm.ChatMessage{}, requestMsgs...), llm.BuildUserMessage(user))
		compressed, compressedMsgs, compErr := sessioncompress.CompressSessionIfNeeded(ctx, o.Sessions, swSessionID, swMsgs, llmMessages, client, cOpts)
		if compErr != nil {
			fallback := sessioncompress.BuildFallbackMessages(swMsgs, llmMessages, compErr, cOpts)
			if len(fallback) > 0 {
				// Preserve system + tail, but re-append the user prompt once below.
				requestMsgs = fallback[:len(fallback)-1]
			}
		} else if compressed {
			requestMsgs = compressedMsgs[:len(compressedMsgs)-1]
		}
	}

	requestMsgs = append(requestMsgs, llm.BuildUserMessage(user))

	out, err := client.ChatCompletion(ctx, requestMsgs, &llm.ChatCompletionOptions{
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return triagePlan{}, "", err
	}

	raw := strings.TrimSpace(out)
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return triagePlan{}, "", err
	}

	var plan triagePlan
	if err := json.Unmarshal([]byte(jsonText), &plan); err != nil {
		return triagePlan{}, "", fmt.Errorf("parse triage json: %w", err)
	}

	plan.SummaryMessage = strings.TrimSpace(plan.SummaryMessage)
	for i := range plan.Tasks {
		plan.Tasks[i].Title = strings.TrimSpace(plan.Tasks[i].Title)
		plan.Tasks[i].Prompt = strings.TrimSpace(plan.Tasks[i].Prompt)
		plan.Tasks[i].WorkspaceStrategy = strings.TrimSpace(plan.Tasks[i].WorkspaceStrategy)
	}
	for i := range plan.Questions {
		plan.Questions[i] = strings.TrimSpace(plan.Questions[i])
	}

	// Best-effort: record SW decision into the SW session (not user-facing).
	if swSessionID := deriveSWSessionID(sessionID); swSessionID != "" {
		_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
			Role:    model.MessageRoleUser,
			Type:    model.MessageTypeText,
			Content: strings.TrimSpace("triage_input:\n" + input),
		})
		_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeText,
			Content: jsonText,
		})
	}

	return plan, resolvedID, nil
}

func (o *Orchestrator) buildMemorySyncPrompt(ctx context.Context, principalID, channel, peerWriter string) string {
	if o == nil || o.Memory == nil {
		return ""
	}
	res, err := o.Memory.PullSync(ctx, principalID, channel, peerWriter, 10)
	if err != nil {
		return ""
	}
	if len(res.Entries) == 0 && res.Omitted == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[MEMORY_SYNC from=%s omitted=%d]\n", peerWriter, res.Omitted))
	for _, e := range res.Entries {
		title := strings.TrimSpace(e.Title)
		if title == "" {
			title = e.Type
		}
		content := strings.TrimSpace(e.Content)
		if len([]rune(content)) > 500 {
			content = truncateString(content, 500)
		}
		b.WriteString(fmt.Sprintf("- (%s) %s: %s\n", e.Writer, title, content))
	}
	return strings.TrimSpace(b.String())
}

func decodeState(meta model.JSONB) (State, bool) {
	if meta == nil {
		return State{}, false
	}
	raw, ok := meta["secretary"]
	if !ok || raw == nil {
		return State{}, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return State{}, false
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, false
	}
	return st, true
}

func upsertState(meta model.JSONB, st State) model.JSONB {
	if meta == nil {
		meta = model.JSONB{}
	}
	meta["secretary"] = st
	return meta
}

func findRun(st State, fromCursor, toMessageID uint) (TriageRun, bool) {
	for i := len(st.TriageRuns) - 1; i >= 0; i-- {
		r := st.TriageRuns[i]
		if r.FromCursor == fromCursor && r.ToMessageID == toMessageID {
			return r, true
		}
	}
	return TriageRun{}, false
}

func messageIDs(msgs []model.ChatMessage) []uint {
	ids := make([]uint, 0, len(msgs))
	for _, m := range msgs {
		if m.ID == 0 {
			continue
		}
		ids = append(ids, m.ID)
	}
	return ids
}

func buildTriageSummary(planSummary string, createdTaskCount int, questions []string) string {
	summary := strings.TrimSpace(planSummary)

	normalizedQuestions := make([]string, 0, len(questions))
	seen := make(map[string]struct{}, len(questions))
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		// Keep the user-facing summary low-noise.
		if len([]rune(q)) > 240 {
			q = truncateString(q, 240) + "…"
		}
		if _, ok := seen[q]; ok {
			continue
		}
		seen[q] = struct{}{}
		normalizedQuestions = append(normalizedQuestions, q)
	}

	if summary == "" {
		switch {
		case createdTaskCount > 0 && len(normalizedQuestions) > 0:
			summary = "我先把不依赖你决定的部分开始推进了。接下来需要你拍板："
		case createdTaskCount > 0:
			summary = "我先开始推进了，进展我会随时告诉你。"
		case len(normalizedQuestions) > 0:
			summary = "继续之前我需要你拍板："
		default:
			summary = "收到，我正在整理下一步。"
		}
	}

	if len(normalizedQuestions) == 0 {
		return strings.TrimSpace(summary)
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(summary))
	b.WriteString("\n")

	// Only append questions that are not already included in the summary.
	qIndex := 0
	for _, q := range normalizedQuestions {
		if strings.Contains(summary, q) {
			continue
		}
		qIndex++
		b.WriteString(fmt.Sprintf("%d) %s\n", qIndex, q))
	}

	if qIndex == 0 {
		return strings.TrimSpace(b.String())
	}

	b.WriteString("你直接回复编号/答案就行，我继续推进。")
	return strings.TrimSpace(b.String())
}

func (o *Orchestrator) workspacePoolRoot() (string, error) {
	root := strings.TrimSpace(os.Getenv("ONEAGENT_WORKSPACE_POOL_DIR"))
	if root == "" {
		root = strings.TrimSpace(o.DefaultWorkspacePoolRoot)
	}
	if root == "" {
		return "", errors.New("workspace pool root is not configured")
	}

	expanded, err := expandHome(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(expanded, 0o700); err != nil {
		return "", err
	}
	return expanded, nil
}

func (o *Orchestrator) createWorkspace(_ context.Context, sessionID string) (string, error) {
	root, err := o.workspacePoolRoot()
	if err != nil {
		return "", err
	}

	short := strings.ReplaceAll(sessionID, "-", "")
	if len(short) > 8 {
		short = short[:8]
	}

	name := fmt.Sprintf("ws-%s-%s-%s", time.Now().UTC().Format("20060102-150405"), short, uuid.NewString()[:8])
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	// NormalizeWorkspaceRoot expects the directory to exist.
	return scope.NormalizeWorkspaceRoot(path)
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func extractJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty LLM output")
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < 0 || end <= start {
		return "", errors.New("no json object found in output")
	}
	return raw[start : end+1], nil
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func looksLikeProgressQuery(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// Only treat clear progress/status questions as progress queries.
	keywords := []string{
		"写了多少",
		"写到哪",
		"写好了吗",
		"写完了吗",
		"进度",
		"做到哪",
		"做完了吗",
		"完成了吗",
		"跑完了吗",
		"还在跑吗",
		"还在运行吗",
		"现在怎么样",
		"进展如何",
		"有结果吗",
	}
	for _, kw := range keywords {
		if kw != "" && strings.Contains(t, kw) {
			return true
		}
	}
	if strings.HasSuffix(t, "了吗") || strings.HasSuffix(t, "了没") {
		return true
	}
	return false
}

type workspaceTextStats struct {
	FilesTotal     int
	TextFilesTotal int
	CharsTotal     int
	Truncated      bool
}

func collectWorkspaceTextStats(root string) (workspaceTextStats, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return workspaceTextStats{}, errors.New("root is required")
	}

	const (
		maxFiles  = 800
		maxBytes  = int64(8 * 1024 * 1024) // total bytes read
		maxSingle = int64(2 * 1024 * 1024)
	)

	var (
		stats     workspaceTextStats
		bytesRead int64
	)

	extAllowed := map[string]bool{
		".md":       true,
		".markdown": true,
		".txt":      true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".tmp" || name == ".git" || name == "node_modules" {
				return fs.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(name, ".") {
			return nil
		}

		stats.FilesTotal++
		if stats.FilesTotal > maxFiles {
			stats.Truncated = true
			return fs.SkipAll
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !extAllowed[ext] {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		size := info.Size()
		if size <= 0 {
			return nil
		}
		if size > maxSingle || bytesRead+size > maxBytes {
			stats.Truncated = true
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		bytesRead += int64(len(data))
		stats.TextFilesTotal++

		for _, r := range string(data) {
			if unicode.IsSpace(r) {
				continue
			}
			stats.CharsTotal++
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		return workspaceTextStats{}, err
	}
	return stats, nil
}

func formatApproxChineseChars(n int) string {
	if n <= 0 {
		return "0字"
	}
	if n < 10_000 {
		return fmt.Sprintf("%d字", n)
	}
	w := (n + 5000) / 10_000
	if w <= 0 {
		w = 1
	}
	return fmt.Sprintf("%dw字", w)
}

func looksLikeQuestion(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if strings.ContainsAny(t, "？?") {
		return true
	}
	if strings.HasSuffix(t, "吗") || strings.HasSuffix(t, "么") {
		return true
	}
	keywords := []string{
		"多少",
		"进度",
		"写到哪",
		"写好",
		"完成",
		"到哪",
		"跑得怎么样",
		"跑了吗",
		"状态",
		"现在怎么样",
		"情况如何",
	}
	for _, kw := range keywords {
		if kw != "" && strings.Contains(t, kw) {
			return true
		}
	}
	return false
}

func sanitizeQuickAckText(userContent, ack string) string {
	text := strings.TrimSpace(ack)
	if text == "" {
		return ""
	}

	// Normalize common LLM “assistant-y” patterns to avoid looking dumb.
	bannedPrefixes := []string{"已记下", "已记录", "收到您的", "收到你", "收到您"}
	for _, p := range bannedPrefixes {
		if strings.HasPrefix(text, p) {
			return fallbackQuickAckText(userContent)
		}
	}
	// Avoid colon-style receipts like "已记下：xxx".
	if strings.ContainsAny(text, "：:") && strings.Contains(text, "记") {
		return fallbackQuickAckText(userContent)
	}

	// Keep it short.
	return truncateString(text, 60)
}

func (o *Orchestrator) buildProgressReply(userID, workspace string, st State) (summary string, questions []string, err error) {
	if o == nil {
		return "", nil, errors.New("orchestrator is nil")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}

	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		q := "我还不知道你在用哪个 workspace。请先指定/选择一个目录，我才能查看进度。"
		return q, []string{q}, nil
	}

	normalized, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		return "", nil, err
	}
	workspace = normalized

	running := 0
	queued := 0
	succeeded := 0
	needsAttention := 0

	if o.Tasks != nil {
		tasks, _ := o.Tasks.ListTasks(userID, workspace)

		created := make(map[string]bool)
		for _, run := range st.TriageRuns {
			for _, id := range run.CreatedTaskIDs {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				created[id] = true
			}
		}

		relevant := tasks
		if len(created) > 0 {
			tmp := make([]taskqueue.Task, 0, len(tasks))
			for _, t := range tasks {
				if created[t.ID] {
					tmp = append(tmp, t)
				}
			}
			if len(tmp) > 0 {
				relevant = tmp
			}
		}

		for _, t := range relevant {
			a := t.LatestAttempt()
			if a == nil {
				continue
			}
			switch a.Status {
			case taskqueue.AttemptQueued:
				queued++
			case taskqueue.AttemptRunning:
				running++
			case taskqueue.AttemptSucceeded:
				succeeded++
			case taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
				needsAttention++
			default:
				// ignore
			}
		}
	}

	// Only scan files for ephemeral workspaces to avoid accidentally reading huge repos.
	shouldScan := false
	wsPath := workspace
	if real, err := filepath.EvalSymlinks(wsPath); err == nil && strings.TrimSpace(real) != "" {
		wsPath = real
	}
	wsClean := filepath.Clean(wsPath) + string(os.PathSeparator)
	if strings.Contains(wsClean, string(os.PathSeparator)+".oneagent"+string(os.PathSeparator)+"workspaces"+string(os.PathSeparator)) {
		shouldScan = true
	} else if root, err := o.workspacePoolRoot(); err == nil {
		rootPath := root
		if real, err := filepath.EvalSymlinks(rootPath); err == nil && strings.TrimSpace(real) != "" {
			rootPath = real
		}
		rootClean := filepath.Clean(rootPath) + string(os.PathSeparator)
		if strings.HasPrefix(wsClean, rootClean) {
			shouldScan = true
		}
	}

	stats := workspaceTextStats{}
	if shouldScan {
		if got, err := collectWorkspaceTextStats(workspace); err == nil {
			stats = got
		}
	}

	statusLine := ""
	switch {
	case running > 0 || queued > 0:
		statusLine = "我看了下，还在运行中。"
	case needsAttention > 0:
		statusLine = "我看了下，有任务卡住了，需要你确认。"
	case succeeded > 0:
		statusLine = "我看了下，已经跑完了。"
	default:
		statusLine = "我看了下，暂时没看到在跑的任务。"
	}

	var details []string
	if shouldScan {
		if stats.FilesTotal > 0 && stats.CharsTotal > 0 {
			details = append(details, fmt.Sprintf("一共有 %d 个文件，累计约 %s。", stats.FilesTotal, formatApproxChineseChars(stats.CharsTotal)))
		} else if stats.FilesTotal > 0 {
			details = append(details, fmt.Sprintf("一共有 %d 个文件。", stats.FilesTotal))
		}
		if stats.Truncated {
			details = append(details, "（统计已截断）")
		}
	}

	if len(details) == 0 {
		// Always provide at least one actionable hint.
		if running > 0 || queued > 0 {
			details = append(details, "我会继续盯着，有更新再告诉你。")
		} else {
			details = append(details, "如需我统计字数/文件，请把任务放在系统创建的 workspace 里。")
		}
	}

	return strings.TrimSpace(statusLine + strings.Join(details, "")), nil, nil
}

func fallbackQuickAckText(userContent string) string {
	text := strings.TrimSpace(userContent)
	// Keep it short and natural; avoid repeating a fixed phrase like “已记下”.
	templates := []string{"收到，我来处理。", "好的，我安排一下。", "明白，我继续跟进。", "了解，我马上处理。", "收到，我继续推进。"}
	questionTemplates := []string{"我看看。", "我查一下。", "我确认一下。", "我看下进度。"}
	if text == "" {
		return templates[0]
	}
	looksQuestion := looksLikeQuestion(text)
	if looksQuestion {
		idx := int(crc32.ChecksumIEEE([]byte(text)) % uint32(len(questionTemplates)))
		return questionTemplates[idx]
	}
	idx := int(crc32.ChecksumIEEE([]byte(text)) % uint32(len(templates)))
	return templates[idx]
}

// deriveTaskTitle matches handler/tasks.go logic (kept local to avoid circular deps).
func deriveTaskTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "Task"
	}
	// Take the first line, then truncate.
	line := prompt
	if idx := strings.IndexAny(line, "\r\n"); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "Task"
	}
	return truncateString(line, 60)
}

const secretaryAckSystemPrompt = `
ONEAGENT_SECRETARY_ACK
你是用户的秘书。请对用户刚刚的消息做一个“快速确认”，满足：
- 只输出一句中文，不要分析，不要列清单
- 不要调用任何工具
- 6~14 个字，语气自然（像真人助理），不要使用“已记下/已记录/收到您的…请求”等机械句式
- 如果用户在问进度/状态/数量，优先回复“我看看/我查一下”这类短句
- 尽量点出/复述 1 个关键信息，避免固定模板
`

const secretaryDispatchSystemPromptSW = `
ONEAGENT_SECRETARY_TRIAGE
你是用户的秘书（SW：Secretary(Worker)，中层管理者），负责把多条消息归并为少量任务并派发后台 worker。
约束：
- 你不直接和用户对话（SU 负责对话）；你只产出派工计划（summary_message/tasks/questions）。
- 你不执行工具，不写/改文件；只做“派工/排队/需要用户确认的问题”的决策。

请严格只输出 JSON（不要代码块，不要额外解释），schema：
{
  "summary_message": "给用户的低噪声汇报（中文）",
  "tasks": [
    {
      "title": "任务标题（短）",
      "prompt": "交给 worker 的工作说明（包含验收标准，鼓励先写测试再改动）",
      "workspace_strategy": "new|session|ask"
    }
  ],
  "questions": ["需要用户确认的问题（可为空）"]
}

summary_message 写作要求（非常重要）：
- 这是“用户会看到的一段话”，要像真人秘书在说话：自然、具体、可执行
- 不要使用内部术语：不要出现 worker/task/派工/workspace/后台 等词
- 不要只说“有 N 个问题/需要确认后才能继续”这种空话；如果需要确认，一定要把要确认的点写清楚
- 至少给出下一步：要么你将继续推进什么；要么用户现在只需要回复什么（最好能“回复 1/2/3”）
- 即使 tasks/questions 都为空，也要输出一条不空的 summary_message（例如“我先把需求梳理一下，马上回来”）

questions 写作要求：
- 每条都要能让用户直接回答（最好附 2~3 个选项或所需信息格式）
- 尽量用“项目目录/仓库根目录/路径”等用户听得懂的说法，不要说 workspace

workspace_strategy 规则：
- new：与 repo 无关的泛化任务（报告/整理/写文档等），允许系统创建新 workspace 并行执行
- session：需要在会话 workspace（代码仓库）内执行的任务（改代码/跑测试等）
- ask：无法判断 workspace 或需要用户明确指定时使用
`
