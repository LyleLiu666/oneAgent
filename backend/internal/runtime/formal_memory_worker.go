package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"

	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
	postgresstore "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/store/postgres"
	memoryworker "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/worker"
)

const (
	formalMemoryWorkerLoopKey                 = "formalmemory.worker"
	formalMemoryWorkerPollInterval            = time.Second
	formalMemoryWorkerLeaseDuration           = 30 * time.Second
	formalMemoryContinuityDraftID             = "thread-continuity"
	formalMemoryConsolidationDuplicateReason  = "superseded_by_newer_session_summary"
	formalMemoryConsolidationSummaryConflict  = "active_session_summary_sdk_conflict"
	formalMemoryConsolidationUniqueConstraint = "uq_memory_jobs_active_consolidation"
	formalMemoryContinuityConfidence          = 0.72
	formalMemoryMaxSummaryText                = 240
)

type formalMemoryTurnJobPayload struct {
	TurnRef      string `json:"turn_ref"`
	RunlogRef    string `json:"runlog_ref"`
	RunID        string `json:"run_id"`
	TurnID       string `json:"turn_id"`
	SessionID    string `json:"session_id"`
	ToolProtocol string `json:"tool_protocol"`
	Status       string `json:"status"`
	AgentID      string `json:"agent_id"`
	Error        string `json:"error"`
}

type formalMemoryConversationSnapshot struct {
	SessionID      string
	UserMessage    *model.ChatMessage
	AssistantReply *model.ChatMessage
}

type formalMemoryExtractProcessor struct {
	sessions  *sessionstore.Store
	extractor *memoryworker.Extractor
	jobStore  core.JobStateStore
}

type formalMemoryConsolidationPlanner struct {
	pool *pgxpool.Pool
}

func StartFormalMemoryWorker(rt *Runtime) {
	if rt == nil || rt.Config == nil || rt.Sessions == nil {
		return
	}
	if !rt.Config.MemorySDKEnableTurnEndJobs || rt.FormalMemory == nil {
		return
	}
	if strings.TrimSpace(rt.Config.MemorySDKPostgresDSN) == "" {
		return
	}

	rt.GoOnceKey(formalMemoryWorkerLoopKey, func(ctx context.Context) {
		runFormalMemoryWorkerLoop(ctx, rt)
	})
}

func runFormalMemoryWorkerLoop(ctx context.Context, rt *Runtime) {
	store, err := postgresstore.Open(ctx, rt.Config.MemorySDKPostgresDSN)
	if err != nil {
		log.Printf("formalmemory worker disabled: open store: %v", err)
		return
	}
	defer store.Close()

	pool, err := pgxpool.New(ctx, rt.Config.MemorySDKPostgresDSN)
	if err != nil {
		log.Printf("formalmemory worker disabled: open query pool: %v", err)
		return
	}
	defer pool.Close()

	runner := memoryworker.NewRunner(store, formalMemoryWorkerOwner(), formalMemoryWorkerLeaseDuration, nil)
	processor := &formalMemoryExtractProcessor{
		sessions:  rt.Sessions,
		extractor: memoryworker.NewExtractor(store, nil),
		jobStore:  store,
	}
	planner := &formalMemoryConsolidationPlanner{pool: pool}
	consolidator := memoryworker.NewConsolidator(store, nil)

	for {
		if ctx.Err() != nil {
			return
		}

		worked := false

		found, err := runner.RunNextExtractJob(ctx, processor)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("formalmemory worker extract failed: %v", err)
		}
		if found {
			worked = true
		}

		found, err = runner.RunNextConsolidationJob(ctx, planner, consolidator)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("formalmemory worker consolidation failed: %v", err)
		}
		if found {
			worked = true
		}

		if worked {
			continue
		}

		timer := time.NewTimer(formalMemoryWorkerPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func formalMemoryWorkerOwner() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("oneagent:%s:%d", host, os.Getpid())
}

func (p *formalMemoryExtractProcessor) ProcessExtractJob(ctx context.Context, job core.Job) error {
	payload, err := decodeFormalMemoryTurnJobPayload(job.PayloadJSON)
	if err != nil {
		return err
	}

	input, err := p.buildExtractInput(job, payload)
	if err != nil {
		return err
	}
	if len(input.Drafts) == 0 {
		return nil
	}

	result, err := p.extractor.ExtractTurnCandidates(ctx, input)
	if err != nil {
		return err
	}
	if len(result.Candidates) == 0 {
		return nil
	}

	if err := enqueueFormalMemoryConsolidationJob(ctx, p.jobStore, job, payload); err != nil {
		return err
	}
	return nil
}

func (p *formalMemoryExtractProcessor) buildExtractInput(job core.Job, payload formalMemoryTurnJobPayload) (memoryworker.ExtractTurnInput, error) {
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(job.ScopeID)
	}
	if sessionID == "" {
		return memoryworker.ExtractTurnInput{}, fmt.Errorf("formal memory extract job missing session_id")
	}

	runID := strings.TrimSpace(payload.RunID)
	if runID == "" {
		runID = "chat:" + sessionID
	}
	turnID := strings.TrimSpace(payload.TurnID)
	if turnID == "" {
		return memoryworker.ExtractTurnInput{}, fmt.Errorf("formal memory extract job missing turn_id")
	}

	_, messages, err := p.sessions.GetSessionWithMessages(sessionID, "")
	if err != nil {
		return memoryworker.ExtractTurnInput{}, fmt.Errorf("load session %s for formal memory: %w", sessionID, err)
	}

	cutoff := job.CreatedAt
	if cutoff.IsZero() {
		cutoff = time.Now()
	}
	snapshot := latestConversationSnapshotBefore(sessionID, messages, cutoff)
	draft := buildTurnContinuityDraft(payload, snapshot)

	return memoryworker.ExtractTurnInput{
		RunID:     runID,
		TurnID:    turnID,
		ScopeKind: job.ScopeKind,
		ScopeID:   sessionID,
		Drafts: []memoryworker.CandidateDraft{
			draft,
		},
	}, nil
}

func enqueueFormalMemoryConsolidationJob(ctx context.Context, jobStore core.JobStateStore, job core.Job, payload formalMemoryTurnJobPayload) error {
	if jobStore == nil {
		return fmt.Errorf("formal memory job store required")
	}

	now := time.Now().UTC()
	consolidationJob := core.Job{
		ID:          fmt.Sprintf("consolidate:%s:%s:%s", job.ScopeKind, job.ScopeID, strings.TrimSpace(payload.TurnID)),
		Type:        core.JobTypeConsolidateScope,
		ScopeKind:   job.ScopeKind,
		ScopeID:     job.ScopeID,
		State:       core.JobStateQueued,
		PayloadJSON: append([]byte(nil), job.PayloadJSON...),
		NotBefore:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := jobStore.Enqueue(ctx, consolidationJob); err != nil {
		if isActiveConsolidationDuplicate(err) {
			return nil
		}
		return err
	}
	return nil
}

func isActiveConsolidationDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.TrimSpace(pgErr.ConstraintName) == formalMemoryConsolidationUniqueConstraint
	}
	return false
}

func (p *formalMemoryConsolidationPlanner) PlanConsolidation(ctx context.Context, job core.Job) (memoryworker.ConsolidationPlan, error) {
	candidates, err := p.loadProposedCandidates(ctx, job.ScopeKind, job.ScopeID)
	if err != nil {
		return memoryworker.ConsolidationPlan{}, err
	}

	activeSummary, err := p.loadActiveSessionSummary(ctx, job.ScopeKind, job.ScopeID)
	if err != nil {
		return memoryworker.ConsolidationPlan{}, err
	}

	return buildFormalMemoryConsolidationPlan(job, candidates, activeSummary)
}

func (p *formalMemoryConsolidationPlanner) loadProposedCandidates(ctx context.Context, scopeKind core.ScopeKind, scopeID string) ([]core.CandidateMemory, error) {
	rows, err := p.pool.Query(ctx, `
SELECT
	id,
	candidate_type,
	scope_kind,
	scope_id,
	source_kind,
	source_ref,
	confidence,
	status,
	COALESCE(idempotency_key, ''),
	COALESCE(derived_formal_id, ''),
	COALESCE(rejection_reason, ''),
	payload_json,
	created_at,
	updated_at
FROM memory_candidates
WHERE scope_kind = $1 AND scope_id = $2 AND status = 'proposed'
ORDER BY updated_at ASC, id ASC
`, string(scopeKind), scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]core.CandidateMemory, 0)
	for rows.Next() {
		var (
			id              string
			candidateType   string
			rowScopeKind    string
			rowScopeID      string
			sourceKind      string
			sourceRef       string
			confidence      float64
			status          string
			idempotencyKey  string
			derivedFormalID string
			rejectionReason string
			payloadJSON     []byte
			createdAt       time.Time
			updatedAt       time.Time
		)
		if err := rows.Scan(
			&id,
			&candidateType,
			&rowScopeKind,
			&rowScopeID,
			&sourceKind,
			&sourceRef,
			&confidence,
			&status,
			&idempotencyKey,
			&derivedFormalID,
			&rejectionReason,
			&payloadJSON,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		payload, err := decodeCandidatePayload(core.CandidateType(candidateType), payloadJSON)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, core.CandidateMemory{
			ID:              id,
			CandidateType:   core.CandidateType(candidateType),
			ScopeKind:       core.ScopeKind(rowScopeKind),
			ScopeID:         rowScopeID,
			SourceKind:      core.SourceKind(sourceKind),
			SourceRef:       sourceRef,
			Confidence:      confidence,
			Payload:         payload,
			Status:          core.CandidateStatus(status),
			IdempotencyKey:  idempotencyKey,
			DerivedFormalID: derivedFormalID,
			RejectionReason: rejectionReason,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (p *formalMemoryConsolidationPlanner) loadActiveSessionSummary(ctx context.Context, scopeKind core.ScopeKind, scopeID string) (*core.SessionSummary, error) {
	if scopeKind != core.ScopeKindThread {
		return nil, nil
	}

	var (
		id            string
		formalType    string
		rowScopeKind  string
		rowScopeID    string
		sourceKind    string
		sourceRef     string
		confidence    float64
		validityState string
		validFrom     time.Time
		validTo       *time.Time
		reasonCode    string
		supersedesIDs []string
		payloadJSON   []byte
		threadID      string
		createdAt     time.Time
		updatedAt     time.Time
	)
	err := p.pool.QueryRow(ctx, `
SELECT
	id,
	type,
	scope_kind,
	scope_id,
	source_kind,
	source_ref,
	confidence,
	validity_state,
	valid_from,
	valid_to,
	COALESCE(reason_code, ''),
	COALESCE(supersedes_ids, '{}'),
	payload_json,
	COALESCE(thread_id, ''),
	created_at,
	updated_at
FROM memory_formal
WHERE scope_kind = $1
  AND scope_id = $2
  AND type = 'session_summary'
  AND validity_state = 'active'
ORDER BY updated_at DESC, id DESC
LIMIT 1
`, string(scopeKind), scopeID).Scan(
		&id,
		&formalType,
		&rowScopeKind,
		&rowScopeID,
		&sourceKind,
		&sourceRef,
		&confidence,
		&validityState,
		&validFrom,
		&validTo,
		&reasonCode,
		&supersedesIDs,
		&payloadJSON,
		&threadID,
		&createdAt,
		&updatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var payload core.SessionSummaryPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("decode active session summary payload: %w", err)
	}

	return &core.SessionSummary{
		FormalCommon: core.FormalCommon{
			ID:            id,
			Type:          core.FormalType(formalType),
			ScopeKind:     core.ScopeKind(rowScopeKind),
			ScopeID:       rowScopeID,
			SourceKind:    core.SourceKind(sourceKind),
			SourceRef:     sourceRef,
			Confidence:    confidence,
			ValidityState: core.ValidityState(validityState),
			ValidFrom:     validFrom,
			ValidTo:       validTo,
			ReasonCode:    reasonCode,
			SupersedesIDs: supersedesIDs,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		},
		ThreadID:          threadID,
		CurrentState:      payload.CurrentState,
		ValidatedFindings: append([]string(nil), payload.ValidatedFindings...),
		NextActions:       append([]string(nil), payload.NextActions...),
		Blockers:          append([]string(nil), payload.Blockers...),
		KeyRefs:           append([]string(nil), payload.KeyRefs...),
	}, nil
}

func buildFormalMemoryConsolidationPlan(job core.Job, candidates []core.CandidateMemory, activeSummary *core.SessionSummary) (memoryworker.ConsolidationPlan, error) {
	payload, err := decodeFormalMemoryTurnJobPayload(job.PayloadJSON)
	if err != nil {
		return memoryworker.ConsolidationPlan{}, err
	}

	latestSummaryID := latestSessionSummaryCandidateID(candidates)
	decisions := make([]memoryworker.ConsolidationDecision, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.CandidateType == core.CandidateTypeSessionSummary {
			switch {
			case activeSummary != nil:
				decisions = append(decisions, memoryworker.ConsolidationDecision{
					Action:    memoryworker.ConsolidationActionReject,
					Candidate: candidate,
					Reason:    formalMemoryConsolidationSummaryConflict,
				})
				continue
			case candidate.ID != latestSummaryID:
				decisions = append(decisions, memoryworker.ConsolidationDecision{
					Action:    memoryworker.ConsolidationActionReject,
					Candidate: candidate,
					Reason:    formalMemoryConsolidationDuplicateReason,
				})
				continue
			}
		}
		decisions = append(decisions, memoryworker.ConsolidationDecision{
			Action:    memoryworker.ConsolidationActionPromote,
			Candidate: candidate,
		})
	}

	return memoryworker.ConsolidationPlan{
		ScopeKind:            job.ScopeKind,
		ScopeID:              job.ScopeID,
		ActiveSessionSummary: activeSummary,
		Decisions:            decisions,
		NextWatermark:        formalMemoryNextWatermark(job, payload),
	}, nil
}

func latestSessionSummaryCandidateID(candidates []core.CandidateMemory) string {
	latest := ""
	for _, candidate := range candidates {
		if candidate.CandidateType == core.CandidateTypeSessionSummary {
			latest = candidate.ID
		}
	}
	return latest
}

func formalMemoryNextWatermark(job core.Job, payload formalMemoryTurnJobPayload) string {
	for _, value := range []string{payload.TurnID, payload.TurnRef, job.ID} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "unknown"
}

func decodeFormalMemoryTurnJobPayload(raw json.RawMessage) (formalMemoryTurnJobPayload, error) {
	var payload formalMemoryTurnJobPayload
	if len(raw) == 0 {
		return payload, fmt.Errorf("formal memory job payload is empty")
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, fmt.Errorf("decode formal memory job payload: %w", err)
	}
	return payload, nil
}

func latestConversationSnapshotBefore(sessionID string, messages []model.ChatMessage, cutoff time.Time) formalMemoryConversationSnapshot {
	snapshot := formalMemoryConversationSnapshot{SessionID: sessionID}
	assistantIndex := -1

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if !messageAtOrBefore(msg, cutoff) {
			continue
		}
		if assistantIndex == -1 {
			if msg.Role == model.MessageRoleAssistant && msg.Type == model.MessageTypeText && strings.TrimSpace(msg.Content) != "" {
				assistantIndex = i
				snapshot.AssistantReply = &messages[i]
			}
			continue
		}
		if msg.Role == model.MessageRoleUser && msg.Type == model.MessageTypeText && strings.TrimSpace(msg.Content) != "" {
			snapshot.UserMessage = &messages[i]
			break
		}
	}

	if snapshot.UserMessage == nil {
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			if !messageAtOrBefore(msg, cutoff) {
				continue
			}
			if msg.Role == model.MessageRoleUser && msg.Type == model.MessageTypeText && strings.TrimSpace(msg.Content) != "" {
				snapshot.UserMessage = &messages[i]
				break
			}
		}
	}

	return snapshot
}

func messageAtOrBefore(msg model.ChatMessage, cutoff time.Time) bool {
	if cutoff.IsZero() || msg.CreatedAt.IsZero() {
		return true
	}
	return !msg.CreatedAt.After(cutoff)
}

func buildTurnContinuityDraft(payload formalMemoryTurnJobPayload, snapshot formalMemoryConversationSnapshot) memoryworker.CandidateDraft {
	userText := ""
	if snapshot.UserMessage != nil {
		userText = snapshot.UserMessage.Content
	}
	assistantText := ""
	if snapshot.AssistantReply != nil {
		assistantText = snapshot.AssistantReply.Content
	}

	sourceKind := core.SourceKindRunlog
	sourceRef := strings.TrimSpace(payload.RunlogRef)
	if snapshot.AssistantReply != nil {
		sourceKind = core.SourceKindAssistantMessage
		sourceRef = chatMessageSourceRef(snapshot.SessionID, *snapshot.AssistantReply)
	} else if snapshot.UserMessage != nil {
		sourceKind = core.SourceKindUserMessage
		sourceRef = chatMessageSourceRef(snapshot.SessionID, *snapshot.UserMessage)
	}
	if strings.TrimSpace(sourceRef) == "" {
		sourceRef = strings.TrimSpace(payload.TurnRef)
	}

	return memoryworker.CandidateDraft{
		DraftID:       formalMemoryContinuityDraftID,
		CandidateType: core.CandidateTypeSemantic,
		SourceKind:    sourceKind,
		SourceRef:     sourceRef,
		Confidence:    formalMemoryContinuityConfidence,
		Payload: core.SemanticMemoryPayload{
			Title:       buildTurnContinuityTitle(userText),
			Summary:     buildSessionSummaryCurrentState(userText, assistantText, payload),
			TopicKeys:   []string{"thread.continuity"},
			EvidenceIDs: nil,
		},
	}
}

func buildTurnContinuityTitle(userText string) string {
	userText = truncateMemoryText(compactMemoryText(userText), 80)
	if userText == "" {
		return "当前线程连续性"
	}
	return "线程主题：" + userText
}

func buildSessionSummaryCurrentState(userText, assistantText string, payload formalMemoryTurnJobPayload) string {
	userText = truncateMemoryText(compactMemoryText(userText), formalMemoryMaxSummaryText)
	assistantText = truncateMemoryText(compactMemoryText(assistantText), formalMemoryMaxSummaryText)
	turnErr := truncateMemoryText(compactMemoryText(payload.Error), formalMemoryMaxSummaryText)

	switch {
	case turnErr != "" && userText != "":
		return fmt.Sprintf("当前线程围绕“%s”推进，但上一轮执行出现错误：%s", userText, turnErr)
	case turnErr != "":
		return "当前线程上一轮执行出现错误：" + turnErr
	case userText != "" && assistantText != "":
		return fmt.Sprintf("当前线程围绕“%s”推进，最近回复要点：%s", userText, assistantText)
	case assistantText != "":
		return "最近回复要点：" + assistantText
	case userText != "":
		return "当前线程的最新用户目标：" + userText
	default:
		return "当前线程最近一轮对话已完成，等待继续推进。"
	}
}

func buildSessionSummaryFindings(userText, assistantText string) []string {
	userText = truncateMemoryText(compactMemoryText(userText), formalMemoryMaxSummaryText)
	assistantText = truncateMemoryText(compactMemoryText(assistantText), formalMemoryMaxSummaryText)

	switch {
	case userText != "" && assistantText != "":
		return []string{
			"最近一轮用户请求：" + userText,
			"最近一轮 assistant 回复：" + assistantText,
		}
	case userText != "":
		return []string{"最近一轮用户请求：" + userText}
	case assistantText != "":
		return []string{"最近一轮 assistant 回复：" + assistantText}
	default:
		return []string{"最近一轮对话已记录。"}
	}
}

func buildSessionSummaryNextActions(userText, assistantText string, payload formalMemoryTurnJobPayload) []string {
	userText = truncateMemoryText(compactMemoryText(userText), formalMemoryMaxSummaryText)
	assistantText = truncateMemoryText(compactMemoryText(assistantText), formalMemoryMaxSummaryText)

	if strings.TrimSpace(payload.Error) != "" {
		return []string{"先处理上一轮错误，再继续当前线程。"}
	}
	if assistantText != "" {
		return []string{"继续基于最近回复的方向推进当前线程。"}
	}
	if userText != "" {
		return []string{"继续围绕用户最新提出的目标推进当前线程。"}
	}
	return []string{"继续沿着当前线程上下文推进。"}
}

func buildSessionSummaryBlockers(payload formalMemoryTurnJobPayload) []string {
	errText := truncateMemoryText(compactMemoryText(payload.Error), formalMemoryMaxSummaryText)
	if errText == "" {
		return nil
	}
	return []string{"上一轮错误：" + errText}
}

func collectSessionSummaryRefs(payload formalMemoryTurnJobPayload) []string {
	refs := make([]string, 0, 2)
	for _, ref := range []string{payload.TurnRef, payload.RunlogRef} {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

func chatMessageSourceRef(sessionID string, msg model.ChatMessage) string {
	if strings.TrimSpace(sessionID) == "" || msg.ID == 0 {
		return ""
	}
	return fmt.Sprintf("chat:%s:msg-%d", sessionID, msg.ID)
}

func compactMemoryText(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func truncateMemoryText(raw string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(raw))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func decodeCandidatePayload(candidateType core.CandidateType, payloadJSON []byte) (any, error) {
	switch candidateType {
	case core.CandidateTypeSessionSummary:
		var payload core.SessionSummaryPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("decode session summary candidate payload: %w", err)
		}
		return payload, nil
	case core.CandidateTypeSemantic:
		var payload core.SemanticMemoryPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("decode semantic candidate payload: %w", err)
		}
		return payload, nil
	case core.CandidateTypeEvidence:
		var payload core.EvidenceMemoryPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("decode evidence candidate payload: %w", err)
		}
		return payload, nil
	case core.CandidateTypeTemporalFact:
		var payload core.TemporalFactMemoryPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("decode temporal fact candidate payload: %w", err)
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported candidate type %q", candidateType)
	}
}
