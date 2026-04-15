package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"

	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
)

func TestE2E_Chat_FormalMemoryTurnEndEnqueuesJob(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeOpenAITextDelta(w, "cmpl-1", "Hello")
		writeOpenAITextDelta(w, "cmpl-1", " world")
		writeOpenAIDone(w)
	}))
	t.Cleanup(mock.Close)

	rt, jobStore := newRuntimeWithFormalMemoryTurnEnd(t, &testFormalMemoryJobStore{})
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	configureDefaultOpenAIModel(t, srv.URL, rt.AuthToken, mock.URL)

	reqBody := map[string]any{
		"message":       "hi",
		"tool_protocol": "json",
		"tool_ids":      []string{},
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new chat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	sessionID, assistantText := readSessionIDAndAssistantTextFromChat(t, resp.Body)
	if strings.TrimSpace(assistantText) != "Hello world" {
		t.Fatalf("expected assistant text %q, got %q", "Hello world", assistantText)
	}

	waitUntilSessionHasAtLeastMessages(t, srv.URL, rt.AuthToken, sessionID, 2*time.Second, 2)

	jobs := waitUntilFormalMemoryJobs(t, jobStore, 2*time.Second, 1)
	job := jobs[0]
	if job.Type != core.JobTypeExtractTurnCandidates {
		t.Fatalf("expected extract job type, got %s", job.Type)
	}
	if job.ScopeKind != core.ScopeKindThread || job.ScopeID != sessionID {
		t.Fatalf("expected job scope thread/%s, got %s/%s", sessionID, job.ScopeKind, job.ScopeID)
	}

	var payload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal job payload: %v", err)
	}
	runID, _ := payload["run_id"].(string)
	turnID, _ := payload["turn_id"].(string)
	turnRef, _ := payload["turn_ref"].(string)
	runlogRef, _ := payload["runlog_ref"].(string)
	boundaryKind, _ := payload["boundary_kind"].(string)
	if runID != "chat:"+sessionID {
		t.Fatalf("expected run_id=%q, got %q", "chat:"+sessionID, runID)
	}
	if strings.TrimSpace(turnID) == "" {
		t.Fatalf("expected non-empty turn_id, payload=%+v", payload)
	}
	if turnRef != fmt.Sprintf("chat:%s:%s", sessionID, turnID) {
		t.Fatalf("unexpected turn_ref: %q", turnRef)
	}
	if runlogRef != fmt.Sprintf("runlog:%s:%s", runID, turnID) {
		t.Fatalf("unexpected runlog_ref: %q", runlogRef)
	}
	if boundaryKind != "context_compaction" {
		t.Fatalf("expected boundary_kind=context_compaction, got %q", boundaryKind)
	}
	if payload["session_id"] != sessionID {
		t.Fatalf("expected session_id=%q, got %+v", sessionID, payload["session_id"])
	}
	if payload["status"] != "completed" {
		t.Fatalf("expected status=completed, got %+v", payload["status"])
	}
	if payload["tool_protocol"] != "json" {
		t.Fatalf("expected tool_protocol=json, got %+v", payload["tool_protocol"])
	}
	if _, ok := payload["workspace_root"]; ok {
		t.Fatalf("workspace_root should not be persisted, got %+v", payload["workspace_root"])
	}
	if _, ok := payload["llm_log_path_hint"]; ok {
		t.Fatalf("llm_log_path_hint should not be persisted, got %+v", payload["llm_log_path_hint"])
	}
	if _, ok := payload["session_messages_path_hint"]; ok {
		t.Fatalf("session_messages_path_hint should not be persisted, got %+v", payload["session_messages_path_hint"])
	}
}

func TestE2E_Chat_FormalMemoryTurnEndEnqueueFailureDoesNotBreakChat(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeOpenAITextDelta(w, "cmpl-1", "still")
		writeOpenAITextDelta(w, "cmpl-1", " works")
		writeOpenAIDone(w)
	}))
	t.Cleanup(mock.Close)

	jobStore := &testFormalMemoryJobStore{enqueueErr: errors.New("queue down")}
	rt, _ := newRuntimeWithFormalMemoryTurnEnd(t, jobStore)
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	configureDefaultOpenAIModel(t, srv.URL, rt.AuthToken, mock.URL)

	reqBody := map[string]any{
		"message":       "hi",
		"tool_protocol": "json",
		"tool_ids":      []string{},
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new chat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	sessionID, assistantText := readSessionIDAndAssistantTextFromChat(t, resp.Body)
	if strings.TrimSpace(assistantText) != "still works" {
		t.Fatalf("expected assistant text %q, got %q", "still works", assistantText)
	}

	msgs := waitUntilSessionHasAtLeastMessages(t, srv.URL, rt.AuthToken, sessionID, 2*time.Second, 2)
	if len(msgs) < 2 {
		t.Fatalf("expected persisted user+assistant messages, got %v", msgs)
	}
	if got := jobStore.snapshot(); len(got) != 0 {
		t.Fatalf("expected no jobs recorded on enqueue failure, got %d", len(got))
	}
}

func TestE2E_Chat_FormalMemoryPreRecallDegradedEmitsTrace(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeOpenAITextDelta(w, "cmpl-1", "trace")
		writeOpenAITextDelta(w, "cmpl-1", " ok")
		writeOpenAIDone(w)
	}))
	t.Cleanup(mock.Close)

	rt, err := newRuntimeWithPreRecallDegrade(t)
	if err != nil {
		t.Fatalf("new runtime with prerecall degrade: %v", err)
	}
	rt.Config.EnableTrace = true
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	configureDefaultOpenAIModel(t, srv.URL, rt.AuthToken, mock.URL)

	reqBody := map[string]any{
		"message":       "hi",
		"tool_protocol": "json",
		"tool_ids":      []string{},
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new chat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	sessionID, assistantText, traces := readSessionIDAssistantTextAndTracesFromChat(t, resp.Body)
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("expected non-empty session id")
	}
	if strings.TrimSpace(assistantText) != "trace ok" {
		t.Fatalf("expected assistant text %q, got %q", "trace ok", assistantText)
	}
	if !containsTrace(traces, "Formal memory pre-recall degraded; continuing without recalled context.") {
		t.Fatalf("expected prerecall degraded trace, got %+v", traces)
	}
}

func newRuntimeWithPreRecallDegrade(t *testing.T) (*oneruntime.Runtime, error) {
	t.Helper()

	home := t.TempDir()
	cfg, err := config.Load(config.LoadOptions{
		Home:     home,
		Profile:  "dev",
		AuthMode: "token",
	})
	if err != nil {
		return nil, err
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		return nil, err
	}

	svc, err := formalmemory.NewServiceWithJobStore(&degradingFormalMemoryStore{}, nil, formalmemory.Config{
		PreRecallPolicy: "auto",
	})
	if err != nil {
		_ = rt.Close()
		return nil, err
	}
	rt.FormalMemory = svc
	return rt, nil
}

func newRuntimeWithFormalMemoryTurnEnd(t *testing.T, jobStore *testFormalMemoryJobStore) (*oneruntime.Runtime, *testFormalMemoryJobStore) {
	t.Helper()

	home := t.TempDir()
	cfg, err := config.Load(config.LoadOptions{
		Home:     home,
		Profile:  "dev",
		AuthMode: "token",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}

	rt.Config.MemorySDKEnableTurnEndJobs = true
	svc, err := formalmemory.NewServiceWithJobStore(&noopFormalMemoryStore{}, jobStore, formalmemory.Config{
		PreRecallPolicy: "auto",
	})
	if err != nil {
		_ = rt.Close()
		t.Fatalf("new formal memory service: %v", err)
	}
	rt.FormalMemory = svc
	return rt, jobStore
}

func configureDefaultOpenAIModel(t *testing.T, baseURL, token, mockURL string) {
	t.Helper()

	var providerResp createProviderResp
	mustPostJSON(t, baseURL, token, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mockURL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, baseURL, token, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)
}

func readSessionIDAndAssistantTextFromChat(t *testing.T, body io.Reader) (string, string) {
	t.Helper()

	var (
		sessionID string
		out       strings.Builder
	)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var evt streamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		if evt.Type == "session" && strings.TrimSpace(evt.Data) != "" {
			sessionID = strings.TrimSpace(evt.Data)
			continue
		}
		if evt.Type != "msg" {
			continue
		}
		var msg streamMsg
		if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
			continue
		}
		if msg.MsgType == "text" && msg.Role == "assistant" && msg.Op == "delta" {
			out.WriteString(msg.Delta)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat stream: %v", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("expected chat stream to include session id")
	}
	return sessionID, out.String()
}

func readSessionIDAssistantTextAndTracesFromChat(t *testing.T, body io.Reader) (string, string, []string) {
	t.Helper()

	var (
		sessionID string
		out       strings.Builder
		traces    []string
	)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var evt streamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		switch evt.Type {
		case "session":
			if strings.TrimSpace(evt.Data) != "" {
				sessionID = strings.TrimSpace(evt.Data)
			}
		case "trace":
			traces = append(traces, strings.TrimSpace(evt.Data))
		case "msg":
			var msg streamMsg
			if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
				continue
			}
			if msg.MsgType == "text" && msg.Role == "assistant" && msg.Op == "delta" {
				out.WriteString(msg.Delta)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat stream: %v", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("expected chat stream to include session id")
	}
	return sessionID, out.String(), traces
}

func containsTrace(traces []string, want string) bool {
	for _, trace := range traces {
		if strings.TrimSpace(trace) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func waitUntilFormalMemoryJobs(t *testing.T, store *testFormalMemoryJobStore, timeout time.Duration, min int) []core.Job {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for formal memory jobs")
		default:
		}

		jobs := store.snapshot()
		if len(jobs) >= min {
			return jobs
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type noopFormalMemoryStore struct{}

func (n *noopFormalMemoryStore) Recall(context.Context, core.RecallQuery) (core.RecallResult, error) {
	return core.RecallResult{}, nil
}

func (n *noopFormalMemoryStore) InsertCandidate(context.Context, core.RememberRequest) (core.RememberResult, error) {
	return core.RememberResult{}, nil
}

type degradingFormalMemoryStore struct{}

func (d *degradingFormalMemoryStore) Recall(context.Context, core.RecallQuery) (core.RecallResult, error) {
	return core.RecallResult{}, context.DeadlineExceeded
}

func (d *degradingFormalMemoryStore) InsertCandidate(context.Context, core.RememberRequest) (core.RememberResult, error) {
	return core.RememberResult{}, nil
}

func (d *degradingFormalMemoryStore) GetFormalByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (d *degradingFormalMemoryStore) GetCandidateByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (d *degradingFormalMemoryStore) InvalidateFormal(context.Context, core.ForgetRequest) error {
	return nil
}

func (d *degradingFormalMemoryStore) DeleteCandidate(context.Context, core.ForgetRequest) error {
	return nil
}

func (n *noopFormalMemoryStore) GetFormalByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (n *noopFormalMemoryStore) GetCandidateByID(context.Context, string) (core.MemoryTarget, bool, error) {
	return core.MemoryTarget{}, false, nil
}

func (n *noopFormalMemoryStore) InvalidateFormal(context.Context, core.ForgetRequest) error {
	return nil
}

func (n *noopFormalMemoryStore) DeleteCandidate(context.Context, core.ForgetRequest) error {
	return nil
}

type testFormalMemoryJobStore struct {
	mu         sync.Mutex
	enqueued   []core.Job
	enqueueErr error
}

func (s *testFormalMemoryJobStore) Enqueue(_ context.Context, job core.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enqueueErr != nil {
		return s.enqueueErr
	}
	s.enqueued = append(s.enqueued, job)
	return nil
}

func (s *testFormalMemoryJobStore) Claim(context.Context, core.JobType, string, time.Time, time.Time) (core.Job, bool, error) {
	return core.Job{}, false, nil
}

func (s *testFormalMemoryJobStore) Complete(context.Context, string, string, time.Time) error {
	return nil
}

func (s *testFormalMemoryJobStore) Fail(context.Context, string, string, string, *time.Time, time.Time) (core.JobState, error) {
	return core.JobStateFailed, nil
}

func (s *testFormalMemoryJobStore) GetWatermark(context.Context, core.ScopeRef, core.JobType) (string, bool, error) {
	return "", false, nil
}

func (s *testFormalMemoryJobStore) UpsertWatermark(context.Context, core.ScopeRef, core.JobType, string, time.Time) error {
	return nil
}

func (s *testFormalMemoryJobStore) snapshot() []core.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]core.Job, len(s.enqueued))
	copy(out, s.enqueued)
	return out
}
