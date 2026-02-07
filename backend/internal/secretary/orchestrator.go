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
	"github.com/liu_y/oneAgent/backend/internal/channelrelay"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/memorydb"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
)

type ResolveModelFunc func(ctx context.Context, userID, modelID string) (client llm.Client, resolvedModelID string, err error)

type Orchestrator struct {
	Sessions *sessionstore.Store
	Tasks    *taskqueue.Store
	Runner   *taskqueue.TaskRunner
	Memory   *memorydb.DB
	Settings *settingsdb.DB

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
	return o.AppendInboxMessageWithTrace(ctx, userID, sessionID, content, workspace, nil)
}

func (o *Orchestrator) AppendInboxMessageWithTrace(ctx context.Context, userID, sessionID, content, workspace string, traceMeta map[string]any) (InboxAppendResult, error) {
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

	trace := model.TraceDataJSON{}
	if len(traceMeta) > 0 {
		entry := model.NewTraceEntry(model.TraceTypeCustom, "inbox_message")
		entry.Metadata = traceMeta
		entry.Complete()
		trace = model.TraceDataJSON{TraceData: model.TraceData{Entries: []model.TraceEntry{entry}}}
	}

	userMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: content,
		Trace:   trace,
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
			CanceledTaskIDs:   []string{},
			ResumedTaskIDs:    []string{},
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
			CanceledTaskIDs:   append([]string{}, run.CanceledTaskIDs...),
			ResumedTaskIDs:    append([]string{}, run.ResumedTaskIDs...),
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

	// Fast path (deterministic): progress questions should not require LLM availability.
	// Only trigger this when there is a single new user message to reduce false positives.
	if len(newUserMsgs) == 1 && isLikelyProgressQuestion(newUserMsgs[0].Content) && strings.TrimSpace(progressSnapshot) != "" {
		summaryMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeText,
			Content: progressSnapshot,
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
			SummaryMessage:    progressSnapshot,
			CreatedTaskIDs:    []string{},
			CanceledTaskIDs:   []string{},
			ResumedTaskIDs:    []string{},
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
				Title:       "triage_summary",
				Content:     progressSnapshot,
			})
		}

		return TriageResult{
			SummaryMessage:    progressSnapshot,
			SummaryMessageID:  summaryMsg.ID,
			CursorMessageID:   state.CursorMessageID,
			CreatedTaskIDs:    []string{},
			CanceledTaskIDs:   []string{},
			ResumedTaskIDs:    []string{},
			Questions:         []string{},
			WorkspacesCreated: []string{},
		}, nil
	}

	// SU: plan + short-path direct answers (best-effort).
	plan, resolvedModelID, err := o.generateTriagePlanAsSU(ctx, userID, sessionID, sessionWorkspace, progressSnapshot, session.Metadata, newUserMsgs)
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

	dispatch := dispatchResult{
		Questions: append([]string{}, plan.Questions...),
	}
	if len(plan.Tasks) > 0 || len(plan.TaskActions) > 0 {
		var dispatchErr error
		dispatch, dispatchErr = o.dispatchPlanAsSW(ctx, userID, sessionID, sessionWorkspace, plan)
		if dispatchErr != nil {
			return TriageResult{}, dispatchErr
		}
	}
	createdTaskIDs := dispatch.CreatedTaskIDs
	canceledTaskIDs := dispatch.CanceledTaskIDs
	resumedTaskIDs := dispatch.ResumedTaskIDs
	questions := dispatch.Questions
	workspacesCreated := dispatch.WorkspacesCreated

	summary := buildTriageSummary(plan.SummaryMessage, len(createdTaskIDs), questions)
	// Note: the SU triage plan is already user-facing. Avoid a second LLM call unless needed.

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
		CanceledTaskIDs:   append([]string{}, canceledTaskIDs...),
		ResumedTaskIDs:    append([]string{}, resumedTaskIDs...),
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
		CanceledTaskIDs:   canceledTaskIDs,
		ResumedTaskIDs:    resumedTaskIDs,
		Questions:         questions,
		WorkspacesCreated: workspacesCreated,
	}, nil
}

type dispatchResult struct {
	CreatedTaskIDs    []string
	CanceledTaskIDs   []string
	ResumedTaskIDs    []string
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

	relayLink, relayOK := channelrelay.TaskLink{}, false
	if o.Sessions != nil {
		if session, _, err := o.Sessions.GetSessionWithMessages(sessionID, userID); err == nil {
			relayLink, relayOK = extractChannelRelayLink(session.Metadata, userID, sessionID)
		}
	}

	createdTaskIDs := make([]string, 0, len(plan.Tasks))
	canceledTaskIDs := make([]string, 0, len(plan.TaskActions))
	resumedTaskIDs := make([]string, 0, len(plan.TaskActions))
	questions := append([]string{}, plan.Questions...)
	workspacesCreated := make([]string, 0, 2)
	skippedNeedWorkspace := make([]string, 0, 2)

	// Best-effort: handle task queue adjustments (cancel/resume). These are user-facing controls
	// and SHOULD NOT delete any task data/evidence.
	if len(plan.TaskActions) > 0 {
		const maxBulkTargets = 20

		allTasks, err := o.Tasks.ListTasks(userID, "")
		if err != nil {
			return dispatchResult{}, err
		}

		sessionTasks := allTasks
		sessionScoped := false
		if sessionWorkspace != "" {
			inWS, err := o.Tasks.ListTasks(userID, sessionWorkspace)
			if err != nil {
				return dispatchResult{}, err
			}
			sessionTasks = inWS
			sessionScoped = true
		}

		inferBulkWorkspace := func(action string) string {
			action = strings.ToLower(strings.TrimSpace(action))
			if action == "" {
				return ""
			}

			uniq := make(map[string]struct{}, 2)
			for _, t := range allTasks {
				ws := strings.TrimSpace(t.Workspace)
				if ws == "" {
					continue
				}
				a := t.LatestAttempt()
				if a == nil {
					continue
				}

				switch action {
				case "cancel":
					switch a.Status {
					case taskqueue.AttemptQueued, taskqueue.AttemptRunning,
						taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
						uniq[ws] = struct{}{}
					}
				case "resume":
					switch a.Status {
					case taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
						uniq[ws] = struct{}{}
					}
				}

				if len(uniq) > 1 {
					return ""
				}
			}
			if len(uniq) != 1 {
				return ""
			}
			for ws := range uniq {
				return ws
			}
			return ""
		}

		// Design principle (Secretary agentic): LLM-first semantics.
		//
		// `task_actions[].task_id` may be a natural-language reference (e.g. "取消那个开发管理系统的任务").
		// Orchestrator code should avoid rules-first heuristics that "guess" what the user meant.
		// Instead: build a candidate set within hard safety boundaries (same user/workspace, action-relevant),
		// ask the LLM to resolve single/ambiguous/not_found, and only do validation/permission checks here.
		//
		// See docs/secretary-agentic-design-review.md ("语义解析同样必须 LLM-first").
		//
		// NOTE: The current implementation still contains deterministic ID/prefix matching fast-paths
		// (token extraction, exact/prefix match). These convenience heuristics are kept for now, but
		// they are not aligned with the LLM-first principle.
		resolveByRef := func(ref string, action string) (taskqueue.Task, bool, string, []taskqueue.Task) {
			rawRef := strings.TrimSpace(ref)
			if rawRef == "" {
				return taskqueue.Task{}, false, "", nil
			}

			// Extract a best-effort opaque ID token when present (users/LLMs may wrap it in brackets).
			// For natural-language references, keep the original string so we can resolve semantically.
			extractIDToken := func(raw string) string {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					return ""
				}

				trimmed := strings.Trim(raw, " \t\r\n()（）[]【】{}<>《》\"'“”‘’.,，。;；:：#")
				if trimmed == "" {
					trimmed = raw
				}

				isIDChar := func(r rune) bool {
					switch {
					case r >= 'a' && r <= 'z':
						return true
					case r >= 'A' && r <= 'Z':
						return true
					case r >= '0' && r <= '9':
						return true
					case r == '-' || r == '_':
						return true
					default:
						return false
					}
				}

				isHex := func(s string) bool {
					if s == "" {
						return false
					}
					for _, r := range s {
						switch {
						case r >= '0' && r <= '9':
							// ok
						case r >= 'a' && r <= 'f':
							// ok
						case r >= 'A' && r <= 'F':
							// ok
						default:
							return false
						}
					}
					return true
				}

				hasHexLetters := func(s string) bool {
					for _, r := range s {
						switch {
						case r >= 'a' && r <= 'f':
							return true
						case r >= 'A' && r <= 'F':
							return true
						}
					}
					return false
				}

				score := func(tok string) int {
					tok = strings.Trim(tok, "-_")
					if tok == "" {
						return 0
					}
					if _, err := uuid.Parse(tok); err == nil {
						return 1000 + len(tok)
					}
					// UUID prefix / attempt prefix (common). Avoid treating pure numbers as IDs (e.g. "写2026文章").
					if len(tok) >= 8 && isHex(tok) {
						return 500 + len(tok)
					}
					if len(tok) >= 4 && isHex(tok) && hasHexLetters(tok) {
						return 100 + len(tok)
					}
					return 0
				}

				best := ""
				bestScore := 0
				start := -1
				for i, r := range trimmed {
					if isIDChar(r) {
						if start == -1 {
							start = i
						}
						continue
					}
					if start != -1 {
						tok := trimmed[start:i]
						if s := score(tok); s > bestScore {
							best = tok
							bestScore = s
						}
						start = -1
					}
				}
				if start != -1 {
					tok := trimmed[start:]
					if s := score(tok); s > bestScore {
						best = tok
						bestScore = s
					}
				}

				return strings.Trim(best, "-_")
			}

			idToken := extractIDToken(rawRef)
			idRef := rawRef
			if idToken != "" {
				idRef = idToken
			}

			matchesByRef := func(tasks []taskqueue.Task, needle string, prefix bool) []taskqueue.Task {
				out := make([]taskqueue.Task, 0, 2)
				seen := make(map[string]struct{}, 2)
				for _, t := range tasks {
					id := strings.TrimSpace(t.ID)
					if id == "" {
						continue
					}
					match := false
					if prefix {
						if strings.HasPrefix(id, needle) {
							match = true
						} else {
							for _, a := range t.Attempts {
								aid := strings.TrimSpace(a.ID)
								if aid != "" && strings.HasPrefix(aid, needle) {
									match = true
									break
								}
							}
						}
					} else {
						if id == needle {
							match = true
						} else {
							for _, a := range t.Attempts {
								if strings.TrimSpace(a.ID) == needle {
									match = true
									break
								}
							}
						}
					}
					if !match {
						continue
					}
					if _, ok := seen[id]; ok {
						continue
					}
					seen[id] = struct{}{}
					out = append(out, t)
				}
				return out
			}

			// Exact match first (prefer session workspace scope).
			if matches := matchesByRef(sessionTasks, idRef, false); len(matches) == 1 {
				return matches[0], true, "", nil
			} else if len(matches) > 1 {
				return taskqueue.Task{}, false, "ambiguous", matches
			}
			if sessionScoped && idToken != "" {
				if matches := matchesByRef(allTasks, idRef, false); len(matches) == 1 {
					return matches[0], true, "", nil
				} else if len(matches) > 1 {
					return taskqueue.Task{}, false, "ambiguous", matches
				}
			}

			// Prefix match (best-effort; prefer session workspace scope).
			if matches := matchesByRef(sessionTasks, idRef, true); len(matches) == 1 {
				return matches[0], true, "", nil
			} else if len(matches) > 1 {
				return taskqueue.Task{}, false, "ambiguous", matches
			}
			if sessionScoped && idToken != "" {
				if matches := matchesByRef(allTasks, idRef, true); len(matches) == 1 {
					return matches[0], true, "", nil
				} else if len(matches) > 1 {
					return taskqueue.Task{}, false, "ambiguous", matches
				}
			}

			// Agentic resolution: let the model map natural-language references to known tasks.
			// Scope: prefer session workspace when bound.
			scopeTasks := sessionTasks
			if scopeTasks == nil {
				scopeTasks = []taskqueue.Task{}
			}

			isRelevantForAction := func(t taskqueue.Task, action string) bool {
				a := t.LatestAttempt()
				if a == nil {
					return false
				}
				switch strings.ToLower(strings.TrimSpace(action)) {
				case "cancel":
					switch a.Status {
					case taskqueue.AttemptQueued, taskqueue.AttemptRunning,
						taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
						return true
					default:
						return false
					}
				case "resume":
					switch a.Status {
					case taskqueue.AttemptFailed, taskqueue.AttemptLimitExceeded, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted:
						return true
					default:
						return false
					}
				default:
					return true
				}
			}

			relevant := make([]taskqueue.Task, 0, len(scopeTasks))
			for _, t := range scopeTasks {
				if isRelevantForAction(t, action) {
					relevant = append(relevant, t)
				}
			}
			sort.Slice(relevant, func(i, j int) bool {
				return relevant[i].UpdatedAt.After(relevant[j].UpdatedAt)
			})
			if len(relevant) > 40 {
				relevant = relevant[:40]
			}

			suggestions := relevant
			if len(suggestions) > 5 {
				suggestions = suggestions[:5]
			}

			if o.ResolveModel == nil || len(relevant) == 0 {
				return taskqueue.Task{}, false, "not_found", suggestions
			}

			client, _, err := o.ResolveModel(ctx, userID, "")
			if err != nil || client == nil {
				return taskqueue.Task{}, false, "not_found", suggestions
			}

			type cand struct {
				ID        string `json:"id"`
				Title     string `json:"title"`
				Prompt    string `json:"prompt,omitempty"`
				Status    string `json:"status"`
				Error     string `json:"error,omitempty"`
				UpdatedAt string `json:"updated_at,omitempty"`
			}

			deriveTitle := func(t taskqueue.Task) string {
				title := strings.TrimSpace(t.Title)
				if title != "" {
					return title
				}
				return deriveTaskTitle(t.Prompt)
			}
			snippet := func(s string, maxRunes int) string {
				s = strings.TrimSpace(s)
				if s == "" {
					return ""
				}
				r := []rune(s)
				if len(r) <= maxRunes {
					return s
				}
				return string(r[:maxRunes]) + "…"
			}

			candidates := make([]cand, 0, len(relevant))
			byID := make(map[string]taskqueue.Task, len(relevant))
			for _, t := range relevant {
				a := t.LatestAttempt()
				if a == nil {
					continue
				}
				id := strings.TrimSpace(t.ID)
				if id == "" {
					continue
				}
				byID[id] = t
				errMsg := strings.TrimSpace(a.Error)
				if errMsg == "" && a.Observer != nil {
					errMsg = strings.TrimSpace(a.Observer.Reason)
				}
				candidates = append(candidates, cand{
					ID:        id,
					Title:     snippet(deriveTitle(t), 80),
					Prompt:    snippet(t.Prompt, 120),
					Status:    string(a.Status),
					Error:     snippet(errMsg, 120),
					UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339),
				})
			}

			type resolution struct {
				Match      string   `json:"match"`
				TaskID     string   `json:"task_id,omitempty"`
				Candidates []string `json:"candidates,omitempty"`
			}

			sys := strings.TrimSpace(`
ONEAGENT_SECRETARY_TASK_REF_RESOLVER
You resolve a user's natural-language task reference to a task id.
Return ONLY a JSON object with:
- match: "single" | "ambiguous" | "not_found"
- task_id: string (when match="single")
- candidates: string[] of task ids (when match="ambiguous")
Rules:
- Use ONLY ids from the provided candidates list.
- If unsure, return match="ambiguous" with up to 3 candidate ids.
`)

			var ub strings.Builder
			ub.WriteString("action: " + strings.ToLower(strings.TrimSpace(action)) + "\n")
			ub.WriteString("ref: " + strings.TrimSpace(rawRef) + "\n")
			ub.WriteString("candidates:\n")
			for i, c := range candidates {
				line, _ := json.Marshal(c)
				ub.WriteString(fmt.Sprintf("%d) %s\n", i+1, string(line)))
			}

			temp := 0.0
			maxTokens := 350
			innerCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			out, err := client.ChatCompletion(innerCtx, []llm.ChatMessage{
				llm.BuildSystemMessage(sys),
				llm.BuildUserMessage(strings.TrimSpace(ub.String())),
			}, &llm.ChatCompletionOptions{
				Temperature: &temp,
				MaxTokens:   &maxTokens,
			})
			if err != nil {
				return taskqueue.Task{}, false, "not_found", suggestions
			}

			obj, err := extractJSONObject(out)
			if err != nil {
				return taskqueue.Task{}, false, "not_found", suggestions
			}

			var res resolution
			if err := json.Unmarshal([]byte(obj), &res); err != nil {
				return taskqueue.Task{}, false, "not_found", suggestions
			}

			switch strings.ToLower(strings.TrimSpace(res.Match)) {
			case "single":
				id := strings.TrimSpace(res.TaskID)
				if id == "" {
					return taskqueue.Task{}, false, "not_found", suggestions
				}
				if t, ok := byID[id]; ok {
					return t, true, "", nil
				}
				return taskqueue.Task{}, false, "not_found", suggestions
			case "ambiguous":
				var matches []taskqueue.Task
				seen := make(map[string]struct{}, 4)
				for _, id := range res.Candidates {
					id = strings.TrimSpace(id)
					if id == "" {
						continue
					}
					if _, ok := seen[id]; ok {
						continue
					}
					seen[id] = struct{}{}
					if t, ok := byID[id]; ok {
						matches = append(matches, t)
					}
					if len(matches) >= 3 {
						break
					}
				}
				if len(matches) == 0 {
					return taskqueue.Task{}, false, "not_found", suggestions
				}
				return taskqueue.Task{}, false, "ambiguous", matches
			default:
				return taskqueue.Task{}, false, "not_found", suggestions
			}
		}

		selectBulk := func(action string) ([]taskqueue.Task, string) {
			if strings.TrimSpace(sessionWorkspace) == "" {
				if inferred := inferBulkWorkspace(action); inferred != "" {
					if inWS, err := o.Tasks.ListTasks(userID, inferred); err == nil {
						sessionWorkspace = inferred
						sessionTasks = inWS
						sessionScoped = true
					}
				}
			}
			if sessionTasks == nil {
				return nil, "no_tasks"
			}

			out := make([]taskqueue.Task, 0, 8)
			for _, t := range sessionTasks {
				a := t.LatestAttempt()
				if a == nil {
					continue
				}
				switch strings.ToLower(strings.TrimSpace(action)) {
				case "cancel":
					if a.Status == taskqueue.AttemptQueued || a.Status == taskqueue.AttemptRunning ||
						a.Status == taskqueue.AttemptFailed || a.Status == taskqueue.AttemptLimitExceeded || a.Status == taskqueue.AttemptTimedOut || a.Status == taskqueue.AttemptInterrupted {
						out = append(out, t)
					}
				case "resume":
					if a.Status == taskqueue.AttemptFailed || a.Status == taskqueue.AttemptLimitExceeded || a.Status == taskqueue.AttemptTimedOut || a.Status == taskqueue.AttemptInterrupted {
						out = append(out, t)
					}
				}
			}
			if len(out) > maxBulkTargets {
				out = out[:maxBulkTargets]
				return out, "truncated"
			}
			return out, ""
		}

		for _, a := range plan.TaskActions {
			act := strings.ToLower(strings.TrimSpace(a.Action))
			ref := strings.TrimSpace(a.TaskID)
			notes := strings.TrimSpace(a.ReviewNotes)

			if act != "cancel" && act != "resume" {
				questions = append(questions, fmt.Sprintf("不支持的任务操作：%q（只支持 cancel/resume；不支持 delete）。", strings.TrimSpace(a.Action)))
				continue
			}

			var targets []taskqueue.Task
			if ref != "" {
				if t, ok, why, matches := resolveByRef(ref, act); ok {
					targets = []taskqueue.Task{t}
				} else {
					label := func(t taskqueue.Task) string {
						title := strings.TrimSpace(t.Title)
						if title == "" {
							title = deriveTaskTitle(t.Prompt)
						}
						id := strings.TrimSpace(t.ID)
						if len(id) > 8 {
							id = id[:8]
						}
						if id == "" {
							return title
						}
						return fmt.Sprintf("%s（追踪号 %s）", title, id)
					}

					switch why {
					case "ambiguous":
						var b strings.Builder
						b.WriteString(fmt.Sprintf("「%s」匹配到多个任务，你指的是哪一个？\n", strings.TrimSpace(ref)))
						limit := len(matches)
						if limit > 3 {
							limit = 3
						}
						for i := 0; i < limit; i++ {
							b.WriteString(fmt.Sprintf("%d) %s\n", i+1, label(matches[i])))
						}
						b.WriteString("回复 1/2/3，或再补一句更具体的描述。")
						questions = append(questions, strings.TrimSpace(b.String()))
					default:
						var b strings.Builder
						b.WriteString(fmt.Sprintf("我没法确定你说的「%s」是哪一个任务。\n", strings.TrimSpace(ref)))
						if len(matches) > 0 {
							limit := len(matches)
							if limit > 3 {
								limit = 3
							}
							for i := 0; i < limit; i++ {
								b.WriteString(fmt.Sprintf("%d) %s\n", i+1, label(matches[i])))
							}
							b.WriteString("回复 1/2/3，或再补一句更具体的描述。")
						} else {
							b.WriteString("你可以再描述一下任务的标题/目的（例如“写 XX 文章的那个”），我就能帮你取消/继续。")
						}
						questions = append(questions, strings.TrimSpace(b.String()))
					}
					continue
				}
			} else {
				bulk, why := selectBulk(act)
				if why == "truncated" {
					questions = append(questions, "任务较多：我这次先处理前 20 个，其余的你再说一声我继续。")
				}
				targets = bulk
			}

			for _, t := range targets {
				if strings.TrimSpace(t.UserID) != userID {
					continue
				}
				switch act {
				case "cancel":
					updated, err := o.Runner.Cancel(t.ID)
					if err != nil {
						questions = append(questions, fmt.Sprintf("我没法取消任务「%s」：%v", strings.TrimSpace(t.ID), err))
						continue
					}
					latest := updated.LatestAttempt()
					if latest == nil {
						questions = append(questions, fmt.Sprintf("我没法取消任务「%s」：任务没有 attempt 记录", strings.TrimSpace(t.ID)))
						continue
					}
					if latest.Status != taskqueue.AttemptCanceled && latest.Status != taskqueue.AttemptRunning {
						questions = append(questions, fmt.Sprintf("任务「%s」当前状态为 %q，取消未生效（仅 queued/running 及失败态可取消）。", strings.TrimSpace(t.ID), latest.Status))
						continue
					}
					canceledTaskIDs = append(canceledTaskIDs, t.ID)
				case "resume":
					if _, err := o.Runner.ResumeWithSource(t.ID, notes, "secretary"); err != nil {
						questions = append(questions, fmt.Sprintf("我没法继续任务「%s」：%v", strings.TrimSpace(t.ID), err))
						continue
					}
					resumedTaskIDs = append(resumedTaskIDs, t.ID)
				default:
					// defensive: should have been filtered above.
					questions = append(questions, fmt.Sprintf("不支持的任务操作：%q（只支持 cancel/resume；不支持 delete）。", strings.TrimSpace(a.Action)))
				}
			}
		}
	}

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

		if relayOK {
			attemptID := ""
			if latest := created.LatestAttempt(); latest != nil {
				attemptID = latest.ID
			}
			_ = o.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    created.ID,
				AttemptID: attemptID,
				Type:      channelrelay.TaskLinkEventType,
				Message:   "task linked to channel relay source",
				Data: map[string]any{
					"provider":      strings.TrimSpace(relayLink.Provider),
					"principal_id":  strings.TrimSpace(relayLink.PrincipalID),
					"channel_id":    strings.TrimSpace(relayLink.ChannelID),
					"thread_id":     strings.TrimSpace(relayLink.ThreadID),
					"message_id":    strings.TrimSpace(relayLink.MessageID),
					"session_id":    strings.TrimSpace(relayLink.SessionID),
					"dispatch_mode": "secretary",
				},
			})
		}

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
	if o.Memory != nil && (len(createdTaskIDs) > 0 || len(canceledTaskIDs) > 0 || len(resumedTaskIDs) > 0 || len(questions) > 0 || len(workspacesCreated) > 0) {
		var b strings.Builder
		if len(createdTaskIDs) > 0 {
			b.WriteString("created_task_ids:\n")
			for _, id := range createdTaskIDs {
				b.WriteString("- " + strings.TrimSpace(id) + "\n")
			}
		}
		if len(canceledTaskIDs) > 0 {
			b.WriteString("canceled_task_ids:\n")
			for _, id := range canceledTaskIDs {
				b.WriteString("- " + strings.TrimSpace(id) + "\n")
			}
		}
		if len(resumedTaskIDs) > 0 {
			b.WriteString("resumed_task_ids:\n")
			for _, id := range resumedTaskIDs {
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
		CanceledTaskIDs:   canceledTaskIDs,
		ResumedTaskIDs:    resumedTaskIDs,
		Questions:         questions,
		WorkspacesCreated: workspacesCreated,
	}, nil
}

func extractChannelRelayLink(meta model.JSONB, userID, sessionID string) (channelrelay.TaskLink, bool) {
	if meta == nil {
		return channelrelay.TaskLink{}, false
	}

	raw, ok := meta["channel_relay"].(map[string]any)
	if !ok {
		if v, ok := meta["channel_relay"].(model.JSONB); ok {
			raw = map[string]any(v)
		}
	}
	if len(raw) == 0 {
		return channelrelay.TaskLink{}, false
	}

	provider := strings.TrimSpace(anyString(raw["provider"]))
	if provider == "" {
		provider = channelrelay.ProviderWebhookV1
	}
	channelID := strings.TrimSpace(anyString(raw["channel_id"]))
	threadID := strings.TrimSpace(anyString(raw["thread_id"]))
	if channelID == "" || threadID == "" {
		return channelrelay.TaskLink{}, false
	}

	principal := strings.TrimSpace(anyString(raw["principal_id"]))
	if principal == "" {
		principal = strings.TrimSpace(userID)
		if principal == "" {
			principal = "local"
		}
	}

	return channelrelay.TaskLink{
		Provider:    provider,
		PrincipalID: principal,
		ChannelID:   channelID,
		ThreadID:    threadID,
		MessageID:   strings.TrimSpace(anyString(raw["message_id"])),
		SessionID:   strings.TrimSpace(sessionID),
	}, true
}

func anyString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return ""
	}
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
	Intent         string             `json:"intent,omitempty"`
	SummaryMessage string             `json:"summary_message"`
	Tasks          []triageTask       `json:"tasks,omitempty"`
	TaskActions    []triageTaskAction `json:"task_actions,omitempty"`
	Questions      []string           `json:"questions,omitempty"`
}

type triageTask struct {
	Title             string `json:"title"`
	Prompt            string `json:"prompt"`
	WorkspaceStrategy string `json:"workspace_strategy"`
}

type triageTaskAction struct {
	Action      string `json:"action"`
	TaskID      string `json:"task_id,omitempty"`
	ReviewNotes string `json:"review_notes,omitempty"`
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

	if len(plan.TaskActions) > 0 {
		out := make([]triageTaskAction, 0, len(plan.TaskActions))
		for _, a := range plan.TaskActions {
			a.Action = strings.TrimSpace(a.Action)
			a.TaskID = strings.TrimSpace(a.TaskID)
			a.ReviewNotes = strings.TrimSpace(a.ReviewNotes)
			if a.Action == "" && a.TaskID == "" && a.ReviewNotes == "" {
				continue
			}
			out = append(out, a)
		}
		plan.TaskActions = out
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
			Description: "Return the secretary triage plan (intent + summary_message + tasks + task_actions + questions).",
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
					"task_actions": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"action": map[string]any{"type": "string", "enum": []string{"cancel", "resume"}},
								"task_id": map[string]any{
									"type":        "string",
									"description": "Optional. Full task id, short prefix, or a natural-language reference (e.g. '写XX文章的任务'). When omitted, apply to relevant tasks (prefer session workspace when bound; otherwise apply across all workspaces, best-effort).",
								},
								"review_notes": map[string]any{"type": "string"},
							},
							"required": []string{"action"},
						},
					},
					"questions": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"intent", "summary_message", "tasks", "task_actions", "questions"},
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
	- tasks/task_actions/questions 允许为空，但字段必须存在（为空则用 []）
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
	<task_actions>
	  <task_action>
	    <action>cancel|resume</action>
	    <task_id>...</task_id>
	    <review_notes>...</review_notes>
	  </task_action>
	</task_actions>
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
	intent, _ := agent.ExtractTagValue(block, "intent", []string{"summary_message", "tasks", "task_actions", "questions"})
	summary, _ := agent.ExtractTagValue(block, "summary_message", []string{"tasks", "task_actions", "questions"})
	tasksRaw, _ := agent.ExtractTagValue(block, "tasks", []string{"task_actions", "questions"})
	actionsRaw, _ := agent.ExtractTagValue(block, "task_actions", []string{"questions"})
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

	for _, actionBlock := range extractAllTagBlocks(actionsRaw, "task_action") {
		act, _ := agent.ExtractTagValue(actionBlock, "action", []string{"task_id", "review_notes"})
		taskID, _ := agent.ExtractTagValue(actionBlock, "task_id", []string{"action", "review_notes"})
		reviewNotes, _ := agent.ExtractTagValue(actionBlock, "review_notes", []string{"action", "task_id"})
		a := triageTaskAction{
			Action:      strings.TrimSpace(act),
			TaskID:      strings.TrimSpace(taskID),
			ReviewNotes: strings.TrimSpace(reviewNotes),
		}
		if a.Action == "" && a.TaskID == "" && a.ReviewNotes == "" {
			continue
		}
		plan.TaskActions = append(plan.TaskActions, a)
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

func (o *Orchestrator) generateTriagePlanAsSU(ctx context.Context, userID, sessionID, sessionWorkspace, progressSnapshot string, metadata model.JSONB, msgs []model.ChatMessage) (triagePlan, string, error) {
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

	sys := strings.TrimSpace(secretaryTriageSystemPromptSU)
	factory := agent.NewFactory()
	policySnapshot := permissions.ResolveSnapshot(userID, secretaryReadOnlyPolicy(), time.Now())
	agentRuntime, err := factory.Build(agent.BuildRequest{
		Spec: agent.AgentSpec{
			ID:           "secretary-su-triage",
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
	if injected := strings.TrimSpace(o.buildMemorySyncPrompt(ctx, userID, "SU", "SW")); injected != "" {
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
	if hasTurnCtx {
		requestMsgs = append(requestMsgs, turnCtxMsg)
	}
	requestMsgs = append(requestMsgs, llm.BuildUserMessage(userPrompt))

	tagsInstruction := buildSecretaryTriagePlanTagsInstruction()

	var plan triagePlan
	if agentRuntime.ToolProtocol == agent.ToolProtocolXML && len(agentRuntime.ToolDefs) > 0 {
		loopMsgs := append([]llm.ChatMessage(nil), requestMsgs...)
		loopMsgs = append(loopMsgs, llm.BuildUserMessage(tagsInstruction))

		toolCtx := ctx
		if toolCtx == nil {
			toolCtx = context.Background()
		}
		toolCtx = tool.ContextWithPolicySnapshot(toolCtx, policySnapshot)
		if o != nil && o.Settings != nil {
			toolCtx = tool.ContextWithSettingsDB(toolCtx, o.Settings)
		}
		if ws := strings.TrimSpace(sessionWorkspace); ws != "" {
			toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{
				Enabled: true,
				Root:    ws,
			})
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
			nil,
			nil,
			nil,
		)
		if loopErr == nil {
			if p, ok := parseSecretaryTriagePlanTags(out); ok {
				plan = p
			}
		}
	}

	if strings.TrimSpace(plan.SummaryMessage) == "" {
		toolSpec := buildSecretaryTriagePlanTool()
		toolInstruction := buildSecretaryTriagePlanToolInstruction()

		parsed, _, err := agent.RequestStructuredOutput[triagePlan](ctx, client, requestMsgs, &llm.ChatCompletionOptions{
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
	}

	_ = sessionID // reserved for future trace hooks (best-effort)
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

	b.WriteString("\n【规划结果】\n")
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
	if len(plan.TaskActions) > 0 {
		b.WriteString("task_actions:\n")
		for _, a := range plan.TaskActions {
			act := strings.TrimSpace(a.Action)
			id := strings.TrimSpace(a.TaskID)
			notes := strings.TrimSpace(a.ReviewNotes)
			line := "- action: " + act
			if id != "" {
				line += ", task_id: " + id
			}
			b.WriteString(line + "\n")
			if notes != "" {
				b.WriteString("  review_notes: " + notes + "\n")
			}
		}
	}
	if len(dispatch.CreatedTaskIDs) > 0 {
		b.WriteString(fmt.Sprintf("created_task_count: %d\n", len(dispatch.CreatedTaskIDs)))
	}
	if len(dispatch.CanceledTaskIDs) > 0 {
		b.WriteString(fmt.Sprintf("canceled_task_count: %d\n", len(dispatch.CanceledTaskIDs)))
	}
	if len(dispatch.ResumedTaskIDs) > 0 {
		b.WriteString(fmt.Sprintf("resumed_task_count: %d\n", len(dispatch.ResumedTaskIDs)))
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

func isLikelyProgressQuestion(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)

	containsAny := func(needles []string) bool {
		for _, needle := range needles {
			needle = strings.TrimSpace(needle)
			if needle == "" {
				continue
			}
			if strings.Contains(lower, needle) {
				return true
			}
		}
		return false
	}

	// Strong progress/status intent.
	if containsAny([]string{
		"progress",
		"status",
		"进度",
		"进展",
		"状态",
	}) {
		return true
	}

	// Task-centric progress intent (avoid matching generic "任务" mentions).
	if strings.Contains(lower, "任务") && containsAny([]string{
		"完成",
		"运行",
		"排队",
		"在跑",
		"还在",
		"几个",
		"多少",
		"进行",
	}) {
		return true
	}

	return false
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
		for _, id := range run.CanceledTaskIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			created[id] = true
		}
		for _, id := range run.ResumedTaskIDs {
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
		b.WriteString("你也可以点「查看 Trace」看看卡在哪一步，然后把报错贴给我，我帮你继续处理。要是这些都不要了，也可以告诉我你想取消哪些任务（或全部取消），我会把它们关掉（相当于取消，不删证据）。")
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
# Role: 用户的得力业务特助 (Executive Assistant)

## Core Philosophy (核心理念)
你是用户注意力的守护者。你的目标是**“最小化用户的决策成本，最大化事项的推进速度”**。
请站在“业务现场”，用通俗、干练、有人情味的语言汇报进展。

## Mental Framework (思维框架)

1.  **信息降噪与重组 (Synthesize & Filter)**
    * **翻译价值**：将后台技术术语（Task, Worker, Slots）转化为业务状态。
    * **合并同类项**：自动归并重复或相似的事务，只汇报最关键的一条主线。

2.  **基于证据的推进 (Evidence-Based Action)**
    * **事实锚定**：汇报进度时，必须在上下文（Context）中找到对应证据（日志、状态码、快照）。
    * **不做无源之水**：如果上下文中没有明确证据表明“已完成”，请使用推测性语气（“预计”、“建议”），并明确告知用户这是你的推断，而非既定事实。

3.  **风险敏感性决策 (Risk-Aware Autonomy)**
    * **低风险默认推进**：对于查询、重试、整理信息等**可逆**操作，请大胆做决定并事后同步。
    * **高风险刹车机制**：对于删除数据、覆盖配置、对外发送消息等**不可逆**操作，**必须**显式请求用户确认，并简述风险点。

4.  **极简交互 (Minimal Friction)**
    * **结果优先**：先说结论/动作，再简述背景。
    * **按需展开**：默认只提供核心信息。文末可隐含（Implicitly）表达“如需查看具体日志/证据可随时吩咐”，无需每次都把日志贴出来。

## Tone & Style (语调与风格)
* **自然对话**：像真人在 IM 软件上汇报工作。
* **高信息密度**：拒绝废话，直击要点。

## Output Instruction
请根据输入的上下文信息，运用上述思维框架进行决策与汇报。直接输出内容，无需标题或解释。
`

const secretaryTriageSystemPromptSU = `
ONEAGENT_SECRETARY_SU_TRIAGE
# Role: 用户的得力业务特助 (SU - Secretary User)
你是用户注意力的守护者。你的目标是**用最小成本把事情推进到可交付**。
你拥有“只读”的高级分析能力（查看代码、搜索、推理）；任何涉及新建文件、修改代码、删除资源、跑耗时测试等变更性操作，都必须通过派发后台任务（Tasks）交给 Worker 执行。

# Output Protocol (最高优先级)
- 绝对禁止输出任何自然语言闲聊或 Markdown 正文。
- 必须且只能通过 Tool Call（secretary_triage_plan）或 XML Tags（<secretary_triage_plan>）返回结构化数据。
- 结构包含：intent（意图归类）、summary_message（给用户看的话）、tasks（后台工单，包含每个 task 的 workspace_strategy）、task_actions（对已有任务的 cancel/resume；不做物理 delete，用户说“删掉”默认用 cancel 关闭任务）、questions（阻塞问题）。

# Core Operating Rules (8条核心硬规则)

1. 默认推进原则 (Bias for Action)
   - 能做决定的不问用户：遇到非关键分支（如文风、非破坏性配置），直接按最佳实践“先斩后奏”。
   - 一句话纠偏：在 summary_message 中告知你的决定（“我将默认按 X 方案推进...”），让用户如果不满意只需回复一句即可修正。

2. 经济型排查 (Budget Awareness)
   - “约 5 次工具调用”只是用于判断是否自办的阈值（不是硬限制）：请先快速评估是否大概率能在约 5 次只读工具调用内闭环。
   - 如果你决定自办：可以根据需要继续调用只读工具，但要保持经济性，避免无意义自转/穷举。
   - 仅当需求涉及写/改/跑或明显需要长时/高成本执行时，才派发 tasks[] 给 Worker。

3. 读写分权 (Read/Write Separation)
   - 你只读：用工具看代码、查日志、读文档。
   - Worker 写：任何涉及新建文件、修改代码、删除资源、跑耗时测试的操作，必须封装进 tasks[]。
   - 你可以调整任务队列：仅允许在 task_actions[] 里发起 cancel/resume；不做物理 delete，用户说“删掉”默认用 cancel 关闭任务。

4. 创作交付分级 (Creation Delivery)
   - 短内容（<300字/大纲/小样）：直接在 summary_message 中输出，给用户即时反馈。
   - 长内容/文件产出：务必派发 tasks[] 给 Worker 生成，避免超时或上下文溢出。

5. Human-Like Communication
   - summary_message 必须自然、像人。严禁出现“根据系统查询”、“已派发 Task-ID”、“Worker 正在执行”等内部术语。
   - 说结果，不要说过程。例：“我已经为您安排了代码修复任务”(√) vs “正在调用 write_file 工具”(×)。

6. 反幻觉与事实性 (Anti-Hallucination)
   - 严禁编造进度：如果你只是派了单，只能说“已安排/已启动”，绝对不能说“已完成/已修复/已生成”（除非你能看到确切的产物）。

7. Questions 极简原则 (Hard Blockers Only)
   - questions[] 仅用于硬阻塞（没有此信息完全无法动工）或高风险不可逆操作。
   - 每轮最多问 1 个问题。
   - 避免“A/B 选项菜单”，直接请求原始信息。例：“请提供项目路径”(√) vs “你要选 A 路径还是 B 路径？”(×)。

8. Workspace 路由逻辑
   - new: 通用问答/无代码依赖的文档创作。
   - session: 明确需要在当前已打开的代码仓库（session_workspace_root）中操作。
   - ask: 意图涉及代码修改，但 session_workspace_root 为空且无法推断目标仓库时（此时必须并在 questions 里问路径）。
	`
