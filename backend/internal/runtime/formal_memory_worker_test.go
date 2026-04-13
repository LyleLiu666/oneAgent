package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/model"

	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
	memoryworker "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/worker"
)

func TestLatestConversationSnapshotBefore_SelectsTurnBoundedMessages(t *testing.T) {
	base := time.Date(2026, time.April, 13, 10, 0, 0, 0, time.UTC)
	messages := []model.ChatMessage{
		{ID: 1, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "第一轮问题", CreatedAt: base.Add(1 * time.Minute)},
		{ID: 2, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "第一轮回答", CreatedAt: base.Add(2 * time.Minute)},
		{ID: 3, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "第二轮问题", CreatedAt: base.Add(3 * time.Minute)},
		{ID: 4, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "第二轮回答", CreatedAt: base.Add(4 * time.Minute)},
	}

	snapshot := latestConversationSnapshotBefore("session-1", messages, base.Add(2*time.Minute+30*time.Second))
	if snapshot.UserMessage == nil || snapshot.UserMessage.ID != 1 {
		t.Fatalf("expected first-turn user message, got %+v", snapshot.UserMessage)
	}
	if snapshot.AssistantReply == nil || snapshot.AssistantReply.ID != 2 {
		t.Fatalf("expected first-turn assistant message, got %+v", snapshot.AssistantReply)
	}
}

func TestBuildTurnContinuityDraft_UsesAssistantSourceAndTraceableRefs(t *testing.T) {
	payload := formalMemoryTurnJobPayload{
		TurnRef:   "chat:session-1:turn-1",
		RunlogRef: "runlog:chat:session-1:turn-1",
	}
	snapshot := formalMemoryConversationSnapshot{
		SessionID: "session-1",
		UserMessage: &model.ChatMessage{
			ID:      1,
			Role:    model.MessageRoleUser,
			Type:    model.MessageTypeText,
			Content: "请继续把 memory worker 做完",
		},
		AssistantReply: &model.ChatMessage{
			ID:      2,
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeText,
			Content: "我会继续补齐 worker，让 turn-end job 真正被消费。",
		},
	}

	draft := buildTurnContinuityDraft(payload, snapshot)
	if draft.CandidateType != core.CandidateTypeSemantic {
		t.Fatalf("unexpected candidate type: %s", draft.CandidateType)
	}
	if draft.SourceKind != core.SourceKindAssistantMessage {
		t.Fatalf("expected assistant source kind, got %s", draft.SourceKind)
	}
	if draft.SourceRef != "chat:session-1:msg-2" {
		t.Fatalf("unexpected source ref: %q", draft.SourceRef)
	}

	summary, ok := draft.Payload.(core.SemanticMemoryPayload)
	if !ok {
		t.Fatalf("expected semantic continuity payload, got %T", draft.Payload)
	}
	if !strings.Contains(summary.Summary, "memory worker") {
		t.Fatalf("expected summary to mention worker context, got %q", summary.Summary)
	}
	if summary.Title == "" {
		t.Fatalf("expected non-empty title, got %+v", summary)
	}
	if len(summary.TopicKeys) == 0 || summary.TopicKeys[0] != "thread.continuity" {
		t.Fatalf("expected continuity topic key, got %+v", summary.TopicKeys)
	}
}

func TestBuildFormalMemoryConsolidationPlan_PromotesSemanticCandidates(t *testing.T) {
	now := time.Date(2026, time.April, 13, 10, 0, 0, 0, time.UTC)
	payloadBytes, err := json.Marshal(formalMemoryTurnJobPayload{TurnID: "turn-2"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	first := core.CandidateMemory{
		ID:            "cand_first",
		CandidateType: core.CandidateTypeSemantic,
		ScopeKind:     core.ScopeKindThread,
		ScopeID:       "session-1",
		SourceKind:    core.SourceKindAssistantMessage,
		SourceRef:     "chat:session-1:msg-2",
		Confidence:    0.7,
		Payload: core.SemanticMemoryPayload{
			Title:     "线程主题",
			Summary:   "第一次连续性摘要",
			TopicKeys: []string{"thread.continuity"},
		},
		Status:         core.CandidateStatusProposed,
		IdempotencyKey: "run:turn-1:extract:continuity",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	second := first
	second.ID = "cand_second"
	second.SourceRef = "chat:session-1:msg-4"
	second.IdempotencyKey = "run:turn-2:extract:continuity"
	second.UpdatedAt = now.Add(time.Minute)
	second.Payload = core.SemanticMemoryPayload{
		Title:     "线程主题",
		Summary:   "第二次连续性摘要",
		TopicKeys: []string{"thread.continuity"},
	}

	plan, err := buildFormalMemoryConsolidationPlan(core.Job{
		ID:          "job-1",
		Type:        core.JobTypeConsolidateScope,
		ScopeKind:   core.ScopeKindThread,
		ScopeID:     "session-1",
		PayloadJSON: payloadBytes,
	}, []core.CandidateMemory{first, second}, nil)
	if err != nil {
		t.Fatalf("build consolidation plan: %v", err)
	}
	if plan.NextWatermark != "turn-2" {
		t.Fatalf("unexpected watermark: %q", plan.NextWatermark)
	}
	if len(plan.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(plan.Decisions))
	}
	if plan.Decisions[0].Action != memoryworker.ConsolidationActionPromote || plan.Decisions[0].Candidate.ID != "cand_first" {
		t.Fatalf("expected first semantic candidate promoted, got %+v", plan.Decisions[0])
	}
	if plan.Decisions[1].Action != memoryworker.ConsolidationActionPromote || plan.Decisions[1].Candidate.ID != "cand_second" {
		t.Fatalf("expected second semantic candidate promoted, got %+v", plan.Decisions[1])
	}
}

func TestBuildFormalMemoryConsolidationPlan_RejectsSessionSummaryWhenActiveSummaryExists(t *testing.T) {
	payloadBytes, err := json.Marshal(formalMemoryTurnJobPayload{TurnID: "turn-2"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	candidate := core.CandidateMemory{
		ID:            "cand_summary",
		CandidateType: core.CandidateTypeSessionSummary,
		ScopeKind:     core.ScopeKindThread,
		ScopeID:       "session-1",
		SourceKind:    core.SourceKindAssistantMessage,
		SourceRef:     "chat:session-1:msg-4",
		Confidence:    0.7,
		Payload: core.SessionSummaryPayload{
			CurrentState:      "新摘要",
			ValidatedFindings: []string{"新发现"},
			NextActions:       []string{"新动作"},
		},
		Status:         core.CandidateStatusProposed,
		IdempotencyKey: "run:turn-2:extract:summary",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	plan, err := buildFormalMemoryConsolidationPlan(core.Job{
		ID:          "job-2",
		Type:        core.JobTypeConsolidateScope,
		ScopeKind:   core.ScopeKindThread,
		ScopeID:     "session-1",
		PayloadJSON: payloadBytes,
	}, []core.CandidateMemory{candidate}, &core.SessionSummary{
		FormalCommon: core.FormalCommon{
			ID:            "mem_active",
			Type:          core.FormalTypeSessionSummary,
			ScopeKind:     core.ScopeKindThread,
			ScopeID:       "session-1",
			SourceKind:    core.SourceKindAssistantMessage,
			SourceRef:     "chat:session-1:msg-1",
			Confidence:    0.6,
			ValidityState: core.ValidityStateActive,
			ValidFrom:     time.Now().Add(-time.Hour),
			CreatedAt:     time.Now().Add(-time.Hour),
			UpdatedAt:     time.Now().Add(-time.Hour),
		},
		ThreadID:          "session-1",
		CurrentState:      "已存在摘要",
		ValidatedFindings: []string{"已有上下文"},
		NextActions:       []string{"继续"},
	})
	if err != nil {
		t.Fatalf("build consolidation plan: %v", err)
	}
	if len(plan.Decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(plan.Decisions))
	}
	if plan.Decisions[0].Action != memoryworker.ConsolidationActionReject {
		t.Fatalf("expected session summary candidate rejected, got %+v", plan.Decisions[0])
	}
	if plan.Decisions[0].Reason != formalMemoryConsolidationSummaryConflict {
		t.Fatalf("unexpected rejection reason: %q", plan.Decisions[0].Reason)
	}
}
