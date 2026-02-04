package secretary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/agent"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/memorydb"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolcalling"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
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
		content := msg.Content
		if msg.Role == model.MessageRoleAssistant && strings.HasPrefix(strings.TrimSpace(content), sessioncompress.DefaultSummaryPrefix) {
			out = append(out, llm.BuildSessionSummaryMessage(content))
			continue
		}
		out = append(out, llm.ChatMessage{Role: msg.Role, Content: content})
	}
	return out
}

const secretaryToolMaxSteps = 5

func secretaryReadOnlyPolicy() permissions.Policy {
	deny := []string{
		tool.ToolIDBash,
		tool.ToolIDRunCommand,
		tool.ToolIDSubagent,

		tool.ToolIDWriteFile,
		tool.ToolIDEdit,
		tool.ToolIDEditV2,
		tool.ToolIDMultiEdit,
		tool.ToolIDTrashFile,
		tool.ToolIDDocumentExport,
		tool.ToolIDPlan,
	}

	denySet := make(map[string]struct{}, len(deny))
	rules := make([]permissions.Rule, 0, len(deny)+16)
	for _, id := range deny {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		denySet[id] = struct{}{}
		rules = append(rules, permissions.Rule{
			ID:     "deny-" + id,
			Effect: permissions.EffectDeny,
			ToolID: id,
		})
	}

	// Default deny, then allow every known read-only tool (except those explicitly denied above).
	for _, def := range tool.All() {
		id := strings.TrimSpace(def.ID)
		if id == "" {
			continue
		}
		if _, denied := denySet[id]; denied {
			continue
		}
		safety := tool.SafetyForToolID(id)
		if safety.Effect != tool.SafetyEffectReadOnly {
			continue
		}
		rules = append(rules, permissions.Rule{
			ID:     "allow-" + id,
			Effect: permissions.EffectAllow,
			ToolID: id,
		})
	}

	return permissions.Policy{
		ID:            "secretary_read_only",
		DefaultEffect: permissions.EffectDeny,
		Rules:         rules,
	}
}

func secretaryDefaultToolIDs(policySnapshot permissions.Snapshot) []string {
	infos := tool.InfosWithSnapshot(policySnapshot)
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		id := strings.TrimSpace(info.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
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

	// NOTE: Secretary SU session is treated as an append-only log. Avoid rewriting
	// message history here (e.g., compression that renumbers IDs) to preserve
	// traceability and maximize provider KV-cache effectiveness.

	userMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: content,
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
		AckMessageID: 0,
		AckText:      "",
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

	progressSnapshot := ""
	if snap, _, snapErr := o.buildProgressReply(userID, sessionWorkspace, state); snapErr == nil {
		progressSnapshot = strings.TrimSpace(snap)
	}

	// SW: dispatch planning + worker coordination (best-effort).
	plan, resolvedModelID, err := o.generateDispatchPlan(ctx, userID, sessionID, sessionWorkspace, progressSnapshot, session.Metadata, newUserMsgs)
	if err != nil {
		// Best-effort fallback: some deployments/tests may not have an LLM configured for secretary triage.
		// If we have a non-empty progress snapshot, answer with it instead of failing hard.
		if progressSnapshotHasWork(progressSnapshot) {
			summaryMsg, persistErr := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
				Role:    model.MessageRoleAssistant,
				Type:    model.MessageTypeText,
				Content: progressSnapshot,
			})
			if persistErr != nil {
				return TriageResult{}, persistErr
			}

			state.CursorMessageID = toID
			state.TriageRuns = append(state.TriageRuns, TriageRun{
				FromCursor:        cursor,
				ToMessageID:       toID,
				InputMessageIDs:   messageIDs(newUserMsgs),
				SummaryMessageID:  summaryMsg.ID,
				SummaryMessage:    progressSnapshot,
				CreatedTaskIDs:    []string{},
				Questions:         []string{},
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
					Title:       "progress_reply_fallback",
					Content:     progressSnapshot,
				})
			}

			return TriageResult{
				SummaryMessage:    progressSnapshot,
				SummaryMessageID:  summaryMsg.ID,
				CursorMessageID:   state.CursorMessageID,
				CreatedTaskIDs:    []string{},
				Questions:         []string{},
				WorkspacesCreated: []string{},
			}, nil
		}

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

	summary := ""
	intent := strings.ToLower(strings.TrimSpace(plan.Intent))
	if intent == "progress" && progressSnapshotHasWork(progressSnapshot) {
		summary = progressSnapshot
	} else {
		summary = buildTriageSummary(plan.SummaryMessage, len(createdTaskIDs), questions)
		if len(createdTaskIDs) > 0 || len(questions) > 0 {
			if suSummary, _, suErr := o.generateSUReport(ctx, userID, sessionID, sessionWorkspace, session.Metadata, newUserMsgs, plan, dispatch, progressSnapshot); suErr == nil {
				if trimmed := strings.TrimSpace(suSummary); trimmed != "" {
					summary = trimmed
				}
			}
		}
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
	skippedNeedWorkspace := make([]string, 0, 2)

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
				skippedNeedWorkspace = append(skippedNeedWorkspace, title)
				continue
			}
			workspace = sessionWorkspace

		case "ask":
			skippedNeedWorkspace = append(skippedNeedWorkspace, title)
			continue

		default:
			skippedNeedWorkspace = append(skippedNeedWorkspace, title)
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

	// Safety fallback: if we could not dispatch some tasks due to missing workspace info,
	// but SW didn't provide any user-facing questions, emit one minimal actionable ask.
	if len(skippedNeedWorkspace) > 0 && len(questions) == 0 {
		if len(skippedNeedWorkspace) == 1 && strings.TrimSpace(skippedNeedWorkspace[0]) != "" {
			questions = append(questions, fmt.Sprintf("要继续「%s」，请把项目目录（仓库根目录）的路径发我。", strings.TrimSpace(skippedNeedWorkspace[0])))
		} else {
			questions = append(questions, "要继续推进，我需要你发我项目目录（仓库根目录）的路径。")
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
	return StateResult{CursorMessageID: state.CursorMessageID, TriageRuns: state.TriageRuns, RecoveryFocus: state.RecoveryFocus}, nil
}

func (o *Orchestrator) SetRecoveryFocus(_ context.Context, userID, sessionID, taskID, attemptID string) (StateResult, error) {
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

	taskID = strings.TrimSpace(taskID)
	attemptID = strings.TrimSpace(attemptID)

	// Best-effort per-session mutex.
	mu := o.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	// Ensure secretary session exists for bootstrap.
	if _, err := o.Sessions.GetOrCreateSession(sessionID, userID, secretaryModuleSU, "Secretary"); err != nil {
		return StateResult{}, err
	}

	session, _, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		return StateResult{}, err
	}

	state, _ := decodeState(session.Metadata)
	if taskID != "" && attemptID != "" {
		state.RecoveryFocus = &RecoveryFocus{TaskID: taskID, AttemptID: attemptID}
	} else {
		state.RecoveryFocus = nil
	}

	meta := upsertState(session.Metadata, state)
	if err := o.Sessions.UpdateSessionMetadata(sessionID, meta); err != nil {
		return StateResult{}, err
	}

	return StateResult{CursorMessageID: state.CursorMessageID, TriageRuns: state.TriageRuns, RecoveryFocus: state.RecoveryFocus}, nil
}

type triagePlan struct {
	Intent         string       `json:"intent,omitempty"`
	SummaryMessage string       `json:"summary_message"`
	Tasks          []triageTask `json:"tasks,omitempty"`
	Questions      []string     `json:"questions,omitempty"`
}

type triageTask struct {
	Title             string `json:"title"`
	Prompt            string `json:"prompt"`
	WorkspaceStrategy string `json:"workspace_strategy"`
}

func normalizeTriagePlan(plan *triagePlan) {
	if plan == nil {
		return
	}
	plan.Intent = strings.TrimSpace(plan.Intent)
	plan.SummaryMessage = strings.TrimSpace(plan.SummaryMessage)

	if len(plan.Tasks) > 0 {
		out := make([]triageTask, 0, len(plan.Tasks))
		for _, t := range plan.Tasks {
			t.Title = strings.TrimSpace(t.Title)
			t.Prompt = strings.TrimSpace(t.Prompt)
			t.WorkspaceStrategy = strings.TrimSpace(t.WorkspaceStrategy)
			if t.Title == "" && t.Prompt == "" && t.WorkspaceStrategy == "" {
				continue
			}
			out = append(out, t)
		}
		plan.Tasks = out
	}

	if len(plan.Questions) > 0 {
		out := make([]string, 0, len(plan.Questions))
		for _, q := range plan.Questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			out = append(out, q)
		}
		plan.Questions = out
	}
}

func buildSecretaryTriagePlanTool() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "secretary_triage_plan",
			Description: "Return the secretary triage plan (intent + summary_message + tasks + questions).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"intent": map[string]any{
						"type": "string",
						"enum": []string{"progress", "dispatch", "clarify"},
					},
					"summary_message": map[string]any{"type": "string"},
					"tasks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"title":              map[string]any{"type": "string"},
								"prompt":             map[string]any{"type": "string"},
								"workspace_strategy": map[string]any{"type": "string", "enum": []string{"new", "session", "ask"}},
							},
							"required": []string{"title", "prompt", "workspace_strategy"},
						},
					},
					"questions": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"intent", "summary_message", "tasks", "questions"},
			},
		},
	}
}

func buildSecretaryTriagePlanToolInstruction() string {
	return strings.TrimSpace(`
请立刻调用 tool：secretary_triage_plan。
要求：
- 只通过 tool arguments 返回结构化字段，不要输出任何额外文本
- intent 只能是：progress | dispatch | clarify
- tasks/questions 允许为空，但字段必须存在（为空则用 []）
`)
}

func buildSecretaryTriagePlanTagsInstruction() string {
	return strings.TrimSpace(`
请只输出一个宽松的 tags block（不要输出 JSON / 代码块 / 额外解释），格式如下：

<secretary_triage_plan>
  <intent>progress|dispatch|clarify</intent>
  <summary_message>...</summary_message>
  <tasks>
    <task>
      <title>...</title>
      <prompt>...</prompt>
      <workspace_strategy>new|session|ask</workspace_strategy>
    </task>
  </tasks>
  <questions>
    <item>...</item>
  </questions>
</secretary_triage_plan>
`)
}

func parseSecretaryTriagePlanTags(text string) (triagePlan, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return triagePlan{}, false
	}

	block, ok := agent.ExtractLatestTagBlock(text, "secretary_triage_plan")
	if !ok {
		block, ok = agent.ExtractLatestTagBlock(text, "triage_plan")
	}
	if !ok {
		return triagePlan{}, false
	}

	// NOTE: For container tags (tasks/questions), avoid using nested tag names as stop-tags.
	// Otherwise missing closing tags (common in model output) could truncate the inner payload.
	intent, _ := agent.ExtractTagValue(block, "intent", []string{"summary_message", "tasks", "questions"})
	summary, _ := agent.ExtractTagValue(block, "summary_message", []string{"tasks", "questions"})
	tasksRaw, _ := agent.ExtractTagValue(block, "tasks", []string{"questions"})
	questionsRaw, _ := agent.ExtractTagValue(block, "questions", nil)

	plan := triagePlan{
		Intent:         strings.TrimSpace(intent),
		SummaryMessage: strings.TrimSpace(summary),
	}

	for _, taskBlock := range extractAllTagBlocks(tasksRaw, "task") {
		title, _ := agent.ExtractTagValue(taskBlock, "title", []string{"prompt", "workspace_strategy"})
		promptText, _ := agent.ExtractTagValue(taskBlock, "prompt", []string{"title", "workspace_strategy"})
		strategy, _ := agent.ExtractTagValue(taskBlock, "workspace_strategy", []string{"title", "prompt"})
		t := triageTask{
			Title:             strings.TrimSpace(title),
			Prompt:            strings.TrimSpace(promptText),
			WorkspaceStrategy: strings.TrimSpace(strategy),
		}
		if t.Title == "" && t.Prompt == "" && t.WorkspaceStrategy == "" {
			continue
		}
		plan.Tasks = append(plan.Tasks, t)
	}

	for _, item := range extractAllTagBlocks(questionsRaw, "item") {
		v, _ := agent.ExtractTagValue(item, "item", nil)
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		plan.Questions = append(plan.Questions, v)
	}

	normalizeTriagePlan(&plan)
	if plan.SummaryMessage == "" {
		return triagePlan{}, false
	}
	return plan, true
}

func extractAllTagBlocks(text string, tag string) []string {
	text = strings.TrimSpace(text)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if text == "" || tag == "" {
		return nil
	}

	lower := strings.ToLower(text)
	closeTag := "</" + tag + ">"

	var out []string
	search := 0
	for {
		openRel := strings.Index(lower[search:], "<"+tag)
		if openRel == -1 {
			break
		}
		open := search + openRel
		closeRel := strings.Index(lower[open:], closeTag)
		if closeRel == -1 {
			out = append(out, strings.TrimSpace(text[open:]))
			break
		}
		end := open + closeRel + len(closeTag)
		out = append(out, strings.TrimSpace(text[open:end]))
		search = end
	}
	return out
}

func (o *Orchestrator) generateDispatchPlan(ctx context.Context, userID, sessionID, sessionWorkspace, progressSnapshot string, metadata model.JSONB, msgs []model.ChatMessage) (triagePlan, string, error) {
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
	factory := agent.NewFactory()
	policySnapshot := permissions.ResolveSnapshot(userID, secretaryReadOnlyPolicy(), time.Now())
	agentRuntime, err := factory.Build(agent.BuildRequest{
		Spec: agent.AgentSpec{
			ID:           "secretary-sw",
			BaseOverride: sys,
			ToolIDs:      secretaryDefaultToolIDs(policySnapshot),
			ToolProtocol: agent.ToolProtocolXML,
		},
		Client:         client,
		PolicySnapshot: policySnapshot,
	})
	if err != nil {
		return triagePlan{}, "", err
	}
	sys = agentRuntime.FullSystemPrompt()
	sessionWorkspace = strings.TrimSpace(sessionWorkspace)
	workspaceHint := "(unset)"
	if sessionWorkspace != "" {
		workspaceHint = sessionWorkspace
	}

	userPrompt := "Context:\n- session_workspace_root: " + workspaceHint + "\n\n以下是用户自上次归并以来的消息列表（按时间顺序）：\n" + input

	turnContextParts := make([]string, 0, 2)
	if injected := strings.TrimSpace(o.buildMemorySyncPrompt(ctx, userID, "SW", "SU")); injected != "" {
		turnContextParts = append(turnContextParts, injected)
	}
	if snap := strings.TrimSpace(progressSnapshot); snap != "" {
		turnContextParts = append(turnContextParts, "## 任务看板快照\n"+snap)
	}
	turnContext := strings.TrimSpace(strings.Join(turnContextParts, "\n\n"))
	turnCtxMsg, hasTurnCtx := llm.BuildTurnContextMessage(turnContext)

	temp := 0.2
	maxTokens := 1500
	requestMsgs := []llm.ChatMessage{llm.BuildSystemMessage(sys)}

	swSessionID := deriveSWSessionID(sessionID)
	if swSessionID != "" {
		_, _ = o.Sessions.GetOrCreateSession(swSessionID, userID, secretaryModuleSW, "Secretary(Work)")
		_, swMsgs, _ := o.Sessions.GetSessionWithMessages(swSessionID, userID)

		lastSummaryIdx := -1
		lastSummaryContent := ""
		for i := len(swMsgs) - 1; i >= 0; i-- {
			msg := swMsgs[i]
			if msg.Type != model.MessageTypeText || msg.Role != model.MessageRoleAssistant {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(msg.Content), sessioncompress.DefaultSummaryPrefix) {
				lastSummaryIdx = i
				lastSummaryContent = msg.Content
				break
			}
		}

		// Prefer a stable summary + tail for KV-cache. Keep SW stored messages append-only:
		// never rewrite or renumber IDs.
		if lastSummaryIdx >= 0 {
			requestMsgs = append(requestMsgs, llm.BuildSessionSummaryMessage(lastSummaryContent))
			if lastSummaryIdx+1 < len(swMsgs) {
				requestMsgs = append(requestMsgs, buildTextLLMHistory(swMsgs[lastSummaryIdx+1:])...)
			}
		} else {
			requestMsgs = append(requestMsgs, buildTextLLMHistory(swMsgs)...)
		}

		// Soft compression for the SW prompt (no ReplaceMessages). If the prompt is too large,
		// append a new stable summary message and rebuild the prompt as:
		// system + summary + turn context + current user prompt.
		cOpts := sessioncompress.DefaultOptions()
		cOpts.SummaryPrefix = sessioncompress.DefaultSummaryPrefix
		llmMessages := append([]llm.ChatMessage{}, requestMsgs...)
		if hasTurnCtx {
			llmMessages = append(llmMessages, turnCtxMsg)
		}
		llmMessages = append(llmMessages, llm.BuildUserMessage(userPrompt))
		if sessioncompress.ApproximateContextRunes(llmMessages) > cOpts.MaxContextRunes {
			base := swMsgs
			if lastSummaryIdx >= 0 && lastSummaryIdx < len(swMsgs) {
				base = swMsgs[lastSummaryIdx:]
			}
			toSummarize, _ := sessioncompress.SplitForCompression(base, 0)
			if len(toSummarize) > 0 {
				input := sessioncompress.FormatForSummaryInput(toSummarize, cOpts)
				summary, sumErr := sessioncompress.BuildCompressionSummary(ctx, client, input)
				if sumErr == nil {
					summary = strings.TrimSpace(summary)
				}
				if summary == "" {
					summary = "流水账:\n- （摘要生成为空）\n\nFindings:\n- （摘要生成为空）"
				}

				summaryHeader := fmt.Sprintf("%s已压缩 %d 条历史消息\n\n", cOpts.SummaryPrefix, len(toSummarize))
				summaryContent := summaryHeader + summary

				// Best-effort: persist the SW summary as an append-only internal message.
				_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
					Role:    model.MessageRoleAssistant,
					Type:    model.MessageTypeText,
					Content: summaryContent,
				})

				requestMsgs = []llm.ChatMessage{
					llm.BuildSystemMessage(sys),
					llm.BuildSessionSummaryMessage(summaryContent),
				}
				if hasTurnCtx {
					requestMsgs = append(requestMsgs, turnCtxMsg)
				}
				requestMsgs = append(requestMsgs, llm.BuildUserMessage(userPrompt))
			} else {
				requestMsgs = sessioncompress.BuildFallbackMessages(swMsgs, llmMessages, errors.New("SW prompt too large"), cOpts)
			}
		} else {
			requestMsgs = llmMessages
		}
	} else {
		if hasTurnCtx {
			requestMsgs = append(requestMsgs, turnCtxMsg)
		}
		requestMsgs = append(requestMsgs, llm.BuildUserMessage(userPrompt))
	}

	tagsInstruction := buildSecretaryTriagePlanTagsInstruction()

	var (
		plan triagePlan
		meta agent.StructuredOutputMeta
	)
	if agentRuntime.ToolProtocol == agent.ToolProtocolXML && len(agentRuntime.ToolDefs) > 0 {
		loopMsgs := append([]llm.ChatMessage(nil), requestMsgs...)
		loopMsgs = append(loopMsgs, llm.BuildUserMessage(tagsInstruction))

		toolCtx := toolcalling.ContextWithChatToolMaxSteps(ctx, secretaryToolMaxSteps)
		toolCtx = tool.ContextWithPolicySnapshot(toolCtx, policySnapshot)
		if ws := strings.TrimSpace(sessionWorkspace); ws != "" {
			toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{
				Enabled: true,
				Root:    ws,
			})
		}

		observeStep := func(rec toolxml.StepRecord) {
			swSessionID := deriveSWSessionID(sessionID)
			if swSessionID == "" {
				return
			}
			if o == nil || o.Sessions == nil {
				return
			}

			type toolCallEnvelope struct {
				Protocol string         `json:"protocol"`
				Calls    []llm.ToolCall `json:"tool_calls,omitempty"`
			}
			type toolResultEnvelope struct {
				Protocol string               `json:"protocol"`
				Results  []toolxml.ToolResult `json:"results,omitempty"`
			}

			callBody := ""
			if len(rec.ToolCalls) > 0 {
				if b, err := json.Marshal(toolCallEnvelope{Protocol: "xml", Calls: rec.ToolCalls}); err == nil {
					callBody = string(b)
				}
			}
			var callID *uint
			if strings.TrimSpace(callBody) != "" {
				if msg, err := o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
					Role:    model.MessageRoleAssistant,
					Type:    model.MessageTypeToolCall,
					Content: callBody,
				}); err == nil && msg.ID != 0 {
					callID = &msg.ID
				}
			}

			resultBody := ""
			if len(rec.ToolResults) > 0 {
				if b, err := json.Marshal(toolResultEnvelope{Protocol: "xml", Results: rec.ToolResults}); err == nil {
					resultBody = string(b)
				}
			}
			if strings.TrimSpace(resultBody) != "" {
				toolMsg := model.ChatMessage{
					Role:    model.MessageRoleTool,
					Type:    model.MessageTypeToolResult,
					Content: resultBody,
				}
				if callID != nil {
					toolMsg.ParentID = callID
				}
				_, _ = o.Sessions.AppendMessage(swSessionID, toolMsg)
			}
		}

		out, loopErr := toolxml.RunLoop(
			toolCtx,
			client,
			loopMsgs,
			&llm.ChatCompletionOptions{
				Temperature: &temp,
				MaxTokens:   &maxTokens,
			},
			agentRuntime.ToolDefs,
			userID,
			nil,
			nil,
			nil,
			nil,
			observeStep,
			nil,
			nil,
		)
		if loopErr == nil {
			if p, ok := parseSecretaryTriagePlanTags(out); ok {
				plan = p
				meta = agent.StructuredOutputMeta{Mode: agent.StructuredOutputModeTags}
			}
		}
	}

	if strings.TrimSpace(plan.SummaryMessage) == "" {
		toolSpec := buildSecretaryTriagePlanTool()
		toolInstruction := buildSecretaryTriagePlanToolInstruction()

		parsed, parsedMeta, err := agent.RequestStructuredOutput[triagePlan](ctx, client, requestMsgs, &llm.ChatCompletionOptions{
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		}, agent.StructuredOutputSpec[triagePlan]{
			Tool:            toolSpec,
			ToolName:        toolSpec.Function.Name,
			ToolInstruction: toolInstruction,
			TagsInstruction: tagsInstruction,
			ParseToolArgs: func(raw json.RawMessage) (triagePlan, error) {
				var p triagePlan
				if err := json.Unmarshal(raw, &p); err != nil {
					return triagePlan{}, err
				}
				normalizeTriagePlan(&p)
				return p, nil
			},
			ParseTags: func(text string) (triagePlan, bool) {
				p, ok := parseSecretaryTriagePlanTags(text)
				if !ok {
					return triagePlan{}, false
				}
				normalizeTriagePlan(&p)
				return p, true
			},
		})
		if err != nil {
			return triagePlan{}, "", err
		}
		plan = parsed
		meta = parsedMeta
	}

	// Best-effort: record SW decision into the SW session (not user-facing).
	if swSessionID := deriveSWSessionID(sessionID); swSessionID != "" {
		_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
			Role:    model.MessageRoleUser,
			Type:    model.MessageTypeText,
			Content: strings.TrimSpace("triage_input:\n" + input),
		})
		decision := ""
		if b, err := json.MarshalIndent(plan, "", "  "); err == nil && len(b) > 0 {
			decision = string(b)
		}
		if decision == "" {
			decision = fmt.Sprintf("intent=%s\nsummary_message=%s\n(mode=%s)", strings.TrimSpace(plan.Intent), strings.TrimSpace(plan.SummaryMessage), meta.Mode)
		}

		_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeText,
			Content: decision,
		})
	}

	return plan, resolvedID, nil
}

func (o *Orchestrator) generateSUReport(ctx context.Context, userID, sessionID, sessionWorkspace string, metadata model.JSONB, userMsgs []model.ChatMessage, plan triagePlan, dispatch dispatchResult, progressSnapshot string) (string, string, error) {
	if o == nil {
		return "", "", errors.New("orchestrator is nil")
	}
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
	if client == nil {
		return "", "", errors.New("resolved model client is nil")
	}

	sys := strings.TrimSpace(secretaryReportSystemPromptSU)
	factory := agent.NewFactory()
	agentRuntime, err := factory.Build(agent.BuildRequest{
		Spec: agent.AgentSpec{
			ID:           "secretary-su",
			BaseOverride: sys,
			ToolIDs:      nil,
			ToolProtocol: agent.ToolProtocolNone,
		},
		Client: client,
	})
	if err != nil {
		return "", "", err
	}
	sys = agentRuntime.FullSystemPrompt()

	var b strings.Builder
	b.WriteString("【本轮输入】\n")
	for i, m := range userMsgs {
		line := strings.TrimSpace(m.Content)
		if line == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%d) %s\n", i+1, line))
	}

	b.WriteString("\n【SW 规划结果】\n")
	if v := strings.TrimSpace(plan.Intent); v != "" {
		b.WriteString("intent: " + v + "\n")
	}
	if v := strings.TrimSpace(plan.SummaryMessage); v != "" {
		b.WriteString("summary_message: " + v + "\n")
	}
	if len(plan.Tasks) > 0 {
		b.WriteString("tasks:\n")
		for _, t := range plan.Tasks {
			title := strings.TrimSpace(t.Title)
			prompt := strings.TrimSpace(t.Prompt)
			strategy := strings.TrimSpace(t.WorkspaceStrategy)
			if title == "" {
				title = deriveTaskTitle(prompt)
			}
			b.WriteString("- title: " + title + "\n")
			if prompt != "" {
				b.WriteString("  prompt: " + prompt + "\n")
			}
			if strategy != "" {
				b.WriteString("  workspace_strategy: " + strategy + "\n")
			}
		}
	}
	if len(dispatch.CreatedTaskIDs) > 0 {
		b.WriteString(fmt.Sprintf("created_task_count: %d\n", len(dispatch.CreatedTaskIDs)))
	}
	if len(dispatch.Questions) > 0 {
		b.WriteString("questions:\n")
		for i, q := range dispatch.Questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, q))
		}
	}
	if snap := strings.TrimSpace(progressSnapshot); snap != "" {
		b.WriteString("\n【任务看板快照】\n")
		b.WriteString(snap)
		b.WriteString("\n")
	}
	sessionWorkspace = strings.TrimSpace(sessionWorkspace)
	if sessionWorkspace != "" {
		b.WriteString("\n【会话目录（内部）】\n")
		b.WriteString(sessionWorkspace)
		b.WriteString("\n")
	}
	b.WriteString("\n请输出你要发给用户的一段话。")

	temp := 0.2
	maxTokens := 600
	out, err := client.ChatCompletion(ctx, []llm.ChatMessage{
		llm.BuildSystemMessage(sys),
		llm.BuildUserMessage(strings.TrimSpace(b.String())),
	}, &llm.ChatCompletionOptions{
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return "", "", err
	}
	text := strings.TrimSpace(out)
	text = strings.Trim(text, "\"")
	if text == "" {
		return "", "", errors.New("empty SU report")
	}

	// Best-effort: record SU report into SW session for traceability, without altering the SU history.
	if o.Sessions != nil {
		if swSessionID := deriveSWSessionID(sessionID); swSessionID != "" {
			_, _ = o.Sessions.AppendMessage(swSessionID, model.ChatMessage{
				Role:    model.MessageRoleAssistant,
				Type:    model.MessageTypeText,
				Content: strings.TrimSpace("su_report:\n" + text),
			})
		}
	}

	return text, resolvedID, nil
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
			summary = "我在整理下一步。"
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

	b.WriteString("你直接回复编号/答案，我就往下安排。")
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

func progressSnapshotHasWork(snapshot string) bool {
	snapshot = strings.TrimSpace(snapshot)
	if snapshot == "" {
		return false
	}

	// The summary starts with "我查了下：运行 X，排队 Y，已完成 Z，需要处理 W。"
	// If all counts are 0, treat it as "no active work" (avoid answering unrelated questions with a blank board).
	allZero := "运行 0，排队 0，已完成 0，需要处理 0"
	return !strings.Contains(snapshot, allZero)
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

func (o *Orchestrator) buildProgressReply(userID, workspace string, st State) (summary string, questions []string, err error) {
	if o == nil {
		return "", nil, errors.New("orchestrator is nil")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}

	workspace = strings.TrimSpace(workspace)
	normalizedWorkspace := ""
	if workspace != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(workspace)
		if err != nil {
			return "", nil, err
		}
		normalizedWorkspace = normalized
	}

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

	running := 0
	queued := 0
	succeeded := 0
	needsAttention := 0

	var runningTasks []taskqueue.Task
	var queuedTasks []taskqueue.Task
	var succeededTasks []taskqueue.Task
	var needsAttentionTasks []taskqueue.Task

	if o.Tasks != nil {
		tasks, _ := o.Tasks.ListTasks(userID, normalizedWorkspace)
		relevant := tasks
		filteredByCreated := false
		if len(created) > 0 {
			tmp := make([]taskqueue.Task, 0, len(tasks))
			for _, t := range tasks {
				if created[t.ID] {
					tmp = append(tmp, t)
				}
			}
			if len(tmp) > 0 {
				relevant = tmp
				filteredByCreated = true
			}
		}

		// If the session has no workspace bound, keep the summary low-noise by only
		// counting likely-active tasks: queued/running/needs-attention, plus recently
		// finished ones. This avoids reporting the user's entire task history.
		if normalizedWorkspace == "" && !filteredByCreated {
			cutoff := time.Now().Add(-24 * time.Hour)
			active := make([]taskqueue.Task, 0, len(relevant))
			for _, t := range relevant {
				a := t.LatestAttempt()
				if a == nil {
					continue
				}
				switch a.Status {
				case taskqueue.AttemptQueued, taskqueue.AttemptRunning,
					taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
					active = append(active, t)
				case taskqueue.AttemptSucceeded:
					if t.UpdatedAt.After(cutoff) {
						active = append(active, t)
					}
				default:
					// ignore
				}
			}
			if len(active) > 0 {
				relevant = active
			}
		}

		sort.Slice(relevant, func(i, j int) bool {
			return relevant[i].UpdatedAt.After(relevant[j].UpdatedAt)
		})

		for _, t := range relevant {
			a := t.LatestAttempt()
			if a == nil {
				continue
			}
			switch a.Status {
			case taskqueue.AttemptQueued:
				queued++
				queuedTasks = append(queuedTasks, t)
			case taskqueue.AttemptRunning:
				running++
				runningTasks = append(runningTasks, t)
			case taskqueue.AttemptSucceeded:
				succeeded++
				succeededTasks = append(succeededTasks, t)
			case taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
				needsAttention++
				needsAttentionTasks = append(needsAttentionTasks, t)
			default:
				// ignore
			}
		}

		// If we don't have a workspace from session metadata, try to infer a single
		// workspace from the relevant tasks so we can provide safe file/word stats.
		if normalizedWorkspace == "" {
			uniq := make(map[string]struct{}, 2)
			for _, t := range relevant {
				ws := strings.TrimSpace(t.Workspace)
				if ws == "" {
					continue
				}
				uniq[ws] = struct{}{}
				if len(uniq) > 1 {
					break
				}
			}
			if len(uniq) == 1 {
				for ws := range uniq {
					normalizedWorkspace = ws
				}
			}
		}
	}

	// Only scan files for ephemeral workspaces to avoid accidentally reading huge repos.
	shouldScan := false
	wsPath := normalizedWorkspace
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
		if got, err := collectWorkspaceTextStats(normalizedWorkspace); err == nil {
			stats = got
		}
	}

	statusLine := fmt.Sprintf("我查了下：运行 %d，排队 %d，已完成 %d，需要处理 %d。", running, queued, succeeded, needsAttention)

	trimWithEllipsis := func(s string, maxLen int) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		runes := []rune(s)
		if len(runes) <= maxLen {
			return s
		}
		return string(runes[:maxLen]) + "…"
	}
	shortID := func(id string) string {
		id = strings.TrimSpace(id)
		if id == "" {
			return ""
		}
		if len(id) <= 8 {
			return id
		}
		return id[:8]
	}
	taskLabel := func(t taskqueue.Task) string {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = deriveTaskTitle(t.Prompt)
		}
		id := shortID(t.ID)
		if id == "" {
			return title
		}
		return fmt.Sprintf("%s（追踪号 %s）", title, id)
	}
	artifactsLabel := func(a *taskqueue.Attempt) string {
		if a == nil {
			return ""
		}
		parts := make([]string, 0, 4)
		if strings.TrimSpace(a.FindingsPath) != "" {
			parts = append(parts, "findings")
		}
		if strings.TrimSpace(a.DiffPatchPath) != "" {
			parts = append(parts, "diff")
		}
		if strings.TrimSpace(a.TraceLogPath) != "" {
			parts = append(parts, "trace")
		}
		if strings.TrimSpace(a.TestReportPath) != "" {
			parts = append(parts, "test")
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "/")
	}

	// Always provide at least one actionable hint.
	var b strings.Builder
	b.WriteString(strings.TrimSpace(statusLine))

	appendList := func(title string, tasks []taskqueue.Task, lineFn func(taskqueue.Task) string) {
		if len(tasks) == 0 {
			return
		}
		b.WriteString("\n")
		b.WriteString(title)
		b.WriteString("\n")

		limit := 3
		if len(tasks) < limit {
			limit = len(tasks)
		}
		for i := 0; i < limit; i++ {
			line := strings.TrimSpace(lineFn(tasks[i]))
			if line == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, line))
		}
		if extra := len(tasks) - limit; extra > 0 {
			b.WriteString(fmt.Sprintf("（还有 %d 个未展开）\n", extra))
		}
	}

	appendList("正在进行：", runningTasks, func(t taskqueue.Task) string {
		a := t.LatestAttempt()
		line := taskLabel(t)
		if a != nil && a.StartedAt != nil {
			line = line + "，已运行 " + time.Since(*a.StartedAt).Truncate(time.Second).String()
		}
		return line
	})
	appendList("排队中：", queuedTasks, func(t taskqueue.Task) string {
		return taskLabel(t)
	})
	appendList("已完成：", succeededTasks, func(t taskqueue.Task) string {
		a := t.LatestAttempt()
		line := taskLabel(t)
		if a == nil {
			return line
		}
		if summary := trimWithEllipsis(a.Summary, 60); summary != "" {
			line = line + " — " + summary
		}
		if arts := artifactsLabel(a); arts != "" {
			line = line + "（产出：" + arts + "）"
		}
		return line
	})
	appendList("需要处理：", needsAttentionTasks, func(t taskqueue.Task) string {
		a := t.LatestAttempt()
		line := taskLabel(t)
		if a == nil {
			return line
		}
		msg := strings.TrimSpace(a.Error)
		if msg == "" && a.Observer != nil {
			msg = strings.TrimSpace(a.Observer.Reason)
		}
		if msg != "" {
			line = line + " — " + trimWithEllipsis(msg, 80)
		}
		return line
	})

	if shouldScan && (stats.FilesTotal > 0 || stats.CharsTotal > 0) {
		b.WriteString("\n")
		if stats.FilesTotal > 0 && stats.CharsTotal > 0 {
			b.WriteString(fmt.Sprintf("目录里一共有 %d 个文件，累计约 %s。", stats.FilesTotal, formatApproxChineseChars(stats.CharsTotal)))
		} else if stats.FilesTotal > 0 {
			b.WriteString(fmt.Sprintf("目录里一共有 %d 个文件。", stats.FilesTotal))
		}
		if stats.Truncated {
			b.WriteString("（统计已截断）")
		}
	}

	b.WriteString("\n")

	switch {
	case running > 0 || queued > 0:
		b.WriteString("我会继续盯着，有更新再告诉你。")
	case needsAttention > 0:
		b.WriteString("你也可以点「查看 Trace」看看卡在哪一步，然后把报错贴给我，我帮你继续处理。")
	case succeeded > 0:
		b.WriteString("你也可以点「查看 Trace」或打开 findings/diff 看产出细节。")
	default:
		b.WriteString("如果你要我跟踪某个条目的进度，把标题或追踪号发我就行。")
	}

	if strings.TrimSpace(b.String()) != "" {
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String()), nil, nil
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

const secretaryReportSystemPromptSU = `
ONEAGENT_SECRETARY_SU_REPORT
你是用户的秘书（SU：Secretary(User)），负责把“本轮归并/派工/进度/需要确认的点”讲清楚，并让用户知道下一步怎么做。
约束：
- 你要用自然中文（像真人助理），不要用机械话术
- 不要出现内部术语：不要出现 worker/task/workspace/派工/后台/工具调用 等词
- 默认更 agentic：只要能推进，就先做决定并推进；把你的“默认假设/默认选择”写清楚，方便用户随时纠偏
- 遇到重复/相近事项：默认先帮用户去重合并（保留最相关/最新的一个继续推进），并说明你怎么处理的；用户若想保留多个，再让他一句话纠偏
- 如果需要用户确认：不要只说“有 N 个问题/需要确认后才能继续”，必须把要确认的点写清楚；并尽量只问 1 个最关键的问题（避免连珠炮）
- 如果不需要用户确认：说明你将继续推进什么，并承诺“有更新就告诉你”（不要编造进度）
- 输出一段消息即可：不要输出标题、不要代码块、不要输出 JSON
`

const secretaryDispatchSystemPromptSW = `
ONEAGENT_SECRETARY_TRIAGE
你是用户的秘书（SW：Secretary(Work)，中层管理者），负责把多条消息归并为少量任务并派发后台 worker。
约束：
- 你不直接和用户对话（SU 负责对话）；你只产出派工计划（summary_message/tasks/questions）。
- 你可以使用系统提供的工具做查询/解释/排障（best-effort），但你是“只读”：不得使用任何会写入/改动/删除文件的工具；如果必须写文件/改文件/删文件，请把工作拆成后台任务（tasks）。
- 简单问题（预计 <= 5 轮工具调用、且不涉及写文件/改文件/删文件）尽量直接在 summary_message 里给出结论与下一步，不要为了“看起来在干活”而派新任务。
- 你会收到一个“任务看板快照”（如果存在）。当用户在问进度/已完成/卡住/报错时，优先用该快照直接回答；不要为此新建任务或追加无意义的问题。
输出：
- 你将通过系统提供的“结构化输出通道”返回 triage plan（intent/summary_message/tasks/questions）。
- 不要尝试输出纯文本 JSON 来满足 schema（这很脆弱且容易降智）；用 tool-call 或宽松 tags（由系统约束与解析）。

summary_message 写作要求（非常重要）：
- 这是“用户会看到的一段话”，要像真人秘书在说话：自然、具体、可执行
- 不要使用内部术语：不要出现 worker/task/派工/workspace/后台 等词
- 不要只说“有 N 个问题/需要确认后才能继续”这种空话；如果需要确认，一定要把要确认的点写清楚
- 默认更 agentic：能做决定就先做决定并推进（优先选可逆/低风险动作）；把你的“默认假设/默认选择”写清楚，让用户可以一句话纠偏
- 遇到重复/相近事项：默认先去重合并（保留最相关/最新的一个继续推进），不要为了确认而卡住
- 至少给出下一步：要么你将继续推进什么；要么用户现在只需要回复什么（避免让用户自己猜）
- 即使 tasks/questions 都为空，也要输出一条不空的 summary_message（例如“我先把需求梳理一下，马上回来”）

questions 写作要求：
- questions[] 只用于“硬阻塞”：没有用户输入就无法继续推进、或存在明显不可逆风险的点；否则不要放进 questions[]（写进 summary_message 的默认假设即可）
- 每轮最多 1 条关键问题（宁可默认推进 + 允许纠偏，也不要事无巨细地问用户）
- 每条都要能让用户直接回答（给出所需信息格式即可；不要为了“显得专业”而硬塞选项菜单）
- 尽量用“项目目录/仓库根目录/路径”等用户听得懂的说法，不要说 workspace

创作/写作类请求（例如写小说/写文案/写报告）额外要求：
- 优先直接产出一个可交付的初稿/大纲/小样并继续迭代；不要先问一堆设定
- 只有在会明显影响方向时才问 1 个关键偏好（例如文风/受众/长度），否则按通用偏好默认推进

workspace_strategy 规则：
- new：与 repo 无关的泛化任务（报告/整理/写文档等），允许系统创建新 workspace 并行执行
- session：需要在会话 workspace（代码仓库）内执行的任务（改代码/跑测试等）；仅当 session_workspace_root 已设置时使用
- ask：无法判断 workspace 或需要用户明确指定时使用
- 如果 session_workspace_root 是 (unset)，不要输出 workspace_strategy=session；改用 ask 并在 questions 里问清楚要用哪个项目目录
	`
