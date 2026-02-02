package secretary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type ResolveModelFunc func(ctx context.Context, userID, modelID string) (client llm.Client, resolvedModelID string, err error)

type Orchestrator struct {
	Sessions *sessionstore.Store
	Tasks    *taskqueue.Store
	Runner   *taskqueue.TaskRunner

	ResolveModel ResolveModelFunc

	DefaultWorkspacePoolRoot string

	muBySession sync.Map // map[string]*sync.Mutex (best-effort triage mutex)
}

func (o *Orchestrator) lock(sessionID string) *sync.Mutex {
	val, _ := o.muBySession.LoadOrStore(sessionID, &sync.Mutex{})
	return val.(*sync.Mutex)
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
	if _, err := o.Sessions.GetOrCreateSession(sessionID, userID, "assistant", title); err != nil {
		return InboxAppendResult{}, err
	}

	session, _, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
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

	plan, resolvedModelID, err := o.generateTriagePlan(ctx, userID, session.Metadata, newUserMsgs)
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

	sessionWorkspace := ""
	if raw, ok := session.Metadata["workspace"]; ok {
		if v, ok := raw.(string); ok {
			sessionWorkspace = strings.TrimSpace(v)
		}
	}

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
				return TriageResult{}, err
			}
			workspacesCreated = append(workspacesCreated, ws)
			workspace = ws

		case "session":
			if sessionWorkspace == "" {
				questions = append(questions, fmt.Sprintf("这项任务需要一个代码仓库 workspace：%s。请指定/选择 workspace 后我再派工。", title))
				continue
			}
			workspace = sessionWorkspace

		case "ask":
			questions = append(questions, fmt.Sprintf("这项任务需要你确认 workspace：%s。请告诉我用哪个目录来执行。", title))
			continue

		default:
			questions = append(questions, fmt.Sprintf("无法确定 workspace（strategy=%s）：%s。请告诉我用哪个目录来执行。", strategy, title))
			continue
		}

		limits := taskqueue.ResolveLimits(taskqueue.Limits{})
		created, err := o.Tasks.CreateTask(userID, workspace, title, prompt, "", limits)
		if err != nil {
			return TriageResult{}, err
		}
		createdTaskIDs = append(createdTaskIDs, created.ID)

		if err := o.Runner.Enqueue(created.ID); err != nil {
			return TriageResult{}, err
		}
	}

	summary := strings.TrimSpace(plan.SummaryMessage)
	if summary == "" {
		summary = buildFallbackSummary(len(createdTaskIDs), len(questions))
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

	return TriageResult{
		SummaryMessage:    summary,
		SummaryMessageID:  summaryMsg.ID,
		CursorMessageID:   state.CursorMessageID,
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
	msgs := []llm.ChatMessage{
		llm.BuildSystemMessage(sys),
		llm.BuildUserMessage(userContent),
	}

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
	if ack == "" {
		return "", "", errors.New("empty quick ack")
	}
	return ack, resolvedID, nil
}

func (o *Orchestrator) generateTriagePlan(ctx context.Context, userID string, metadata model.JSONB, msgs []model.ChatMessage) (triagePlan, string, error) {
	if o.ResolveModel == nil {
		return triagePlan{}, "", errors.New("ResolveModel is required")
	}
	modelID := ""
	if v, ok := metadata["model_id"].(string); ok {
		modelID = strings.TrimSpace(v)
	}
	client, resolvedID, err := o.ResolveModel(ctx, userID, modelID)
	if err != nil {
		return triagePlan{}, "", err
	}

	var b strings.Builder
	for i, m := range msgs {
		b.WriteString(fmt.Sprintf("%d) %s\n", i+1, strings.TrimSpace(m.Content)))
	}
	input := strings.TrimSpace(b.String())
	if input == "" {
		return triagePlan{}, "", errors.New("no messages to triage")
	}

	sys := strings.TrimSpace(secretaryTriageSystemPrompt)
	user := "以下是用户自上次归并以来的消息列表（按时间顺序）：\n" + input + "\n\n请按 schema 输出 JSON。"

	temp := 0.2
	maxTokens := 1500
	out, err := client.ChatCompletion(ctx, []llm.ChatMessage{
		llm.BuildSystemMessage(sys),
		llm.BuildUserMessage(user),
	}, &llm.ChatCompletionOptions{
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

	return plan, resolvedID, nil
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

func buildFallbackSummary(taskCount, questionCount int) string {
	switch {
	case taskCount > 0 && questionCount > 0:
		return fmt.Sprintf("我已先安排 %d 个后台 worker 开始执行，同时有 %d 个问题需要你确认。", taskCount, questionCount)
	case taskCount > 0:
		return fmt.Sprintf("我已安排 %d 个后台 worker 开始执行。", taskCount)
	case questionCount > 0:
		return fmt.Sprintf("我先记下了需求，但有 %d 个问题需要你确认后才能派工。", questionCount)
	default:
		return "我已记下需求，正在整理下一步安排。"
	}
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

func fallbackQuickAckText(userContent string) string {
	text := strings.TrimSpace(userContent)
	// Keep it short and natural; avoid repeating a fixed phrase like “已记下”.
	templates := []string{
		"收到，我来处理。",
		"好的，我安排一下。",
		"明白，我继续跟进。",
		"了解，我马上处理。",
		"收到，我继续推进。",
	}
	if text == "" {
		return templates[0]
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
- 8~18 个字，尽量点出/复述 1 个关键信息，避免固定模板
`

const secretaryTriageSystemPrompt = `
ONEAGENT_SECRETARY_TRIAGE
你是用户的秘书（中层管理者），负责把多条消息归并为少量任务并派发后台 worker。

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

workspace_strategy 规则：
- new：与 repo 无关的泛化任务（报告/整理/写文档等），允许系统创建新 workspace 并行执行
- session：需要在会话 workspace（代码仓库）内执行的任务（改代码/跑测试等）
- ask：无法判断 workspace 或需要用户明确指定时使用
`
