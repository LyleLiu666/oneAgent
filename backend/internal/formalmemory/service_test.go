package formalmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	agentsdkbridge "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/bridge/agentsdk"
	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
)

func TestOpen_DisabledWithoutDSN(t *testing.T) {
	svc, err := Open(context.Background(), Config{})
	if err != nil {
		t.Fatalf("expected disabled open without error, got %v", err)
	}
	if svc != nil {
		t.Fatalf("expected nil service when dsn is missing")
	}
}

func TestNewService_PreRecallTurnContextFormatsItems(t *testing.T) {
	store := &fakeMemoryStore{
		recallResult: core.RecallResult{
			Items: []core.RecallItem{
				{
					ID:        "mem_1",
					Type:      core.FormalTypeSemantic,
					ScopeKind: core.ScopeKindThread,
					ScopeID:   "session-1",
					Summary:   "用户当前在接 memorySdk。",
					SourceRef: "chat:session-1:msg-2",
				},
			},
		},
	}

	svc, err := NewService(store, Config{PreRecallPolicy: "auto"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	got, err := svc.PreRecallTurnContext(context.Background(), PreRecallRequest{
		UserID:        "u1",
		SessionID:     "session-1",
		WorkspaceRoot: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("prerecall turn context: %v", err)
	}
	if !strings.Contains(got.TurnContext, "FormalMemory 预召回") {
		t.Fatalf("expected formatted formal memory heading, got %q", got.TurnContext)
	}
	if !strings.Contains(got.TurnContext, "用户当前在接 memorySdk") {
		t.Fatalf("expected memory summary in output, got %q", got.TurnContext)
	}
	if got.Degraded {
		t.Fatalf("expected prerecall result not degraded, got %+v", got)
	}

	if len(store.recallQueries) != 1 {
		t.Fatalf("expected one recall query, got %d", len(store.recallQueries))
	}
	query := store.recallQueries[0]
	if len(query.ScopeOrder) != 3 {
		t.Fatalf("expected thread/project/user scope order, got %+v", query.ScopeOrder)
	}
	if query.ScopeOrder[0].Kind != core.ScopeKindThread || query.ScopeOrder[0].ID != "session-1" {
		t.Fatalf("unexpected first scope: %+v", query.ScopeOrder[0])
	}
	if query.ScopeOrder[1].Kind != core.ScopeKindProject || query.ScopeOrder[1].ID != projectScopeIDFromWorkspaceRoot("/tmp/workspace") {
		t.Fatalf("unexpected second scope: %+v", query.ScopeOrder[1])
	}
	if query.ScopeOrder[1].ID == "/tmp/workspace" {
		t.Fatalf("expected project scope id to be opaque, got raw workspace root")
	}
	if query.ScopeOrder[2].Kind != core.ScopeKindUser || query.ScopeOrder[2].ID != "u1" {
		t.Fatalf("unexpected third scope: %+v", query.ScopeOrder[2])
	}
}

func TestNewService_PreRecallTurnContextSessionOnlyPolicy(t *testing.T) {
	store := &fakeMemoryStore{
		recallResult: core.RecallResult{
			Items: []core.RecallItem{
				{
					ID:        "mem_1",
					Type:      core.FormalTypeSessionSummary,
					ScopeKind: core.ScopeKindThread,
					ScopeID:   "session-1",
					Summary:   "只查当前会话。",
				},
			},
		},
	}

	svc, err := NewService(store, Config{PreRecallPolicy: "session_only"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	got, err := svc.PreRecallTurnContext(context.Background(), PreRecallRequest{
		UserID:        "u1",
		SessionID:     "session-1",
		WorkspaceRoot: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("prerecall turn context: %v", err)
	}
	if got.Degraded {
		t.Fatalf("expected session-only prerecall not degraded, got %+v", got)
	}

	query := store.recallQueries[0]
	if len(query.ScopeOrder) != 1 {
		t.Fatalf("expected only thread scope, got %+v", query.ScopeOrder)
	}
	if query.ScopeOrder[0].Kind != core.ScopeKindThread || query.ScopeOrder[0].ID != "session-1" {
		t.Fatalf("unexpected thread scope: %+v", query.ScopeOrder[0])
	}
}

func TestNewService_PreRecallTurnContextDegradedReturnsEmpty(t *testing.T) {
	store := &fakeMemoryStore{recallErr: context.DeadlineExceeded}

	svc, err := NewService(store, Config{PreRecallPolicy: "auto"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	got, err := svc.PreRecallTurnContext(context.Background(), PreRecallRequest{
		UserID:    "u1",
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("expected degraded path without hard error, got %v", err)
	}
	if got.TurnContext != "" {
		t.Fatalf("expected empty turn context on degraded prerecall, got %q", got.TurnContext)
	}
	if !got.Degraded {
		t.Fatalf("expected degraded prerecall result, got %+v", got)
	}
}

func TestNewService_RejectsInvalidPolicy(t *testing.T) {
	_, err := NewService(&fakeMemoryStore{}, Config{PreRecallPolicy: "broken"})
	if err == nil {
		t.Fatalf("expected invalid policy error")
	}
}

func TestNewService_ExecuteToolRememberRequiresStableToolCallID(t *testing.T) {
	store := &fakeMemoryStore{}

	svc, err := NewService(store, Config{PreRecallPolicy: "auto"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.ExecuteTool(context.Background(), ToolInvokeRequest{
		Name:      "memory.remember",
		Protocol:  "json",
		RunID:     "chat:session-1",
		TurnID:    "turn-1",
		UserID:    "u1",
		SessionID: "session-1",
		Arguments: []byte(`{
			"candidate_type":"semantic",
			"scope_kind":"thread",
			"source_kind":"user_message",
			"source_ref":"chat:session-1:msg-1",
			"confidence":0.92,
			"payload":{"title":"语言偏好","summary":"用户偏好中文","topic_keys":["user.language"]}
		}`),
	})
	if agentsdkbridge.ErrorCode(err) != agentsdkbridge.ErrCodeRememberInvocationIDRequired {
		t.Fatalf("expected missing tool_call_id rejection, got %v", err)
	}
	if len(store.insertRequests) != 0 {
		t.Fatalf("expected no candidate insert on failed remember, got %d", len(store.insertRequests))
	}
}

func TestNewService_EnqueueTurnEndExtractJob(t *testing.T) {
	jobStore := &recordingJobStore{}

	svc, err := NewServiceWithJobStore(&fakeMemoryStore{}, jobStore, Config{PreRecallPolicy: "auto"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	notBefore := time.Date(2026, time.April, 13, 10, 0, 0, 0, time.UTC)
	payload := json.RawMessage(`{"boundary_kind":"context_compaction","turn_ref":"chat:session-1:turn-1","runlog_ref":"runlog:chat:session-1:turn-1"}`)
	result, err := svc.EnqueueTurnEndExtractJob(context.Background(), TurnEndRequest{
		RunID:         "chat:session-1",
		TurnID:        "turn-1",
		UserID:        "u1",
		SessionID:     "session-1",
		WorkspaceRoot: "/tmp/workspace",
		AgentID:       "oneagent.chat.assistant",
		Payload:       payload,
		NotBefore:     &notBefore,
	})
	if err != nil {
		t.Fatalf("enqueue turn end extract job: %v", err)
	}
	if !result.Enqueued || strings.TrimSpace(result.JobID) == "" {
		t.Fatalf("expected enqueued result, got %+v", result)
	}
	if len(jobStore.enqueued) != 1 {
		t.Fatalf("expected one enqueued job, got %d", len(jobStore.enqueued))
	}

	job := jobStore.enqueued[0]
	if job.Type != core.JobTypeExtractTurnCandidates {
		t.Fatalf("expected extract job type, got %s", job.Type)
	}
	if job.ScopeKind != core.ScopeKindThread || job.ScopeID != "session-1" {
		t.Fatalf("expected thread scope enqueue, got %s/%s", job.ScopeKind, job.ScopeID)
	}
	if string(job.PayloadJSON) != string(payload) {
		t.Fatalf("unexpected payload: %s", string(job.PayloadJSON))
	}
	if !job.NotBefore.Equal(notBefore) {
		t.Fatalf("unexpected not_before: %s", job.NotBefore)
	}
}

func TestNewService_EnqueueTurnEndExtractJobRequiresThreadScope(t *testing.T) {
	jobStore := &recordingJobStore{}

	svc, err := NewServiceWithJobStore(&fakeMemoryStore{}, jobStore, Config{PreRecallPolicy: "auto"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.EnqueueTurnEndExtractJob(context.Background(), TurnEndRequest{
		RunID:  "chat:session-1",
		TurnID: "turn-1",
		UserID: "u1",
	})
	if agentsdkbridge.ErrorCode(err) != agentsdkbridge.ErrCodeTurnEndThreadScopeRequired {
		t.Fatalf("expected thread scope required error, got %v", err)
	}
	if len(jobStore.enqueued) != 0 {
		t.Fatalf("expected no enqueued jobs on failure, got %d", len(jobStore.enqueued))
	}
}

func TestLogBridgeEvent_RedactsProjectHostPath(t *testing.T) {
	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	origPrefix := log.Prefix()
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
		log.SetPrefix(origPrefix)
	})

	err := logBridgeEvent(context.Background(), agentsdkbridge.BridgeEvent{
		EventName:       agentsdkbridge.EventNameWriteCommitted,
		TargetScopeKind: string(core.ScopeKindProject),
		TargetScopeID:   "/Users/demo/workspace",
		CandidateID:     "cand_1",
	})
	if err != nil {
		t.Fatalf("log bridge event: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "/Users/demo/workspace") {
		t.Fatalf("expected project host path to be redacted, got %q", got)
	}
	if !strings.Contains(got, "[redacted-host-path]") {
		t.Fatalf("expected redaction marker in log, got %q", got)
	}
}

func TestLogBridgeEvent_PreservesStableProjectScopeID(t *testing.T) {
	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	origPrefix := log.Prefix()
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
		log.SetPrefix(origPrefix)
	})

	err := logBridgeEvent(context.Background(), agentsdkbridge.BridgeEvent{
		EventName:       agentsdkbridge.EventNameWriteCommitted,
		TargetScopeKind: string(core.ScopeKindProject),
		TargetScopeID:   "project_123",
		CandidateID:     "cand_1",
	})
	if err != nil {
		t.Fatalf("log bridge event: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "project_123") {
		t.Fatalf("expected stable project id to remain visible, got %q", got)
	}
	if strings.Contains(got, "[redacted-host-path]") {
		t.Fatalf("did not expect redaction marker for stable project id, got %q", got)
	}
}

type fakeMemoryStore struct {
	recallResult   core.RecallResult
	recallErr      error
	recallQueries  []core.RecallQuery
	insertRequests []core.RememberRequest
}

func (f *fakeMemoryStore) Recall(_ context.Context, query core.RecallQuery) (core.RecallResult, error) {
	f.recallQueries = append(f.recallQueries, query)
	if f.recallErr != nil {
		return core.RecallResult{}, f.recallErr
	}
	return f.recallResult, nil
}

func (f *fakeMemoryStore) InsertCandidate(_ context.Context, req core.RememberRequest) (core.RememberResult, error) {
	f.insertRequests = append(f.insertRequests, req)
	return core.RememberResult{Candidate: req.Candidate}, nil
}

func (f *fakeMemoryStore) GetFormalByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (f *fakeMemoryStore) GetCandidateByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (f *fakeMemoryStore) InvalidateFormal(context.Context, core.ForgetRequest) error {
	return errors.New("not implemented")
}

func (f *fakeMemoryStore) DeleteCandidate(context.Context, core.ForgetRequest) error {
	return errors.New("not implemented")
}

type recordingJobStore struct {
	enqueued []core.Job
}

func (r *recordingJobStore) Enqueue(_ context.Context, job core.Job) error {
	r.enqueued = append(r.enqueued, job)
	return nil
}

func (r *recordingJobStore) Claim(context.Context, core.JobType, string, time.Time, time.Time) (core.Job, bool, error) {
	return core.Job{}, false, nil
}

func (r *recordingJobStore) Complete(context.Context, string, string, time.Time) error {
	return nil
}

func (r *recordingJobStore) Fail(context.Context, string, string, string, *time.Time, time.Time) (core.JobState, error) {
	return core.JobStateFailed, nil
}

func (r *recordingJobStore) GetWatermark(context.Context, core.ScopeRef, core.JobType) (string, bool, error) {
	return "", false, nil
}

func (r *recordingJobStore) UpsertWatermark(context.Context, core.ScopeRef, core.JobType, string, time.Time) error {
	return nil
}
