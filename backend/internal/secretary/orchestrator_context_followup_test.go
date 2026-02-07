package secretary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func scriptedTriageResponses() []string {
	return []string{
		`<tool_data>
<call>
  <tool_name>does_not_exist</tool_name>
</call>
</tool_data>`,
		`<secretary_triage_plan>
  <intent>progress</intent>
  <summary_message>ok</summary_message>
  <tasks></tasks>
  <task_actions></task_actions>
  <questions></questions>
</secretary_triage_plan>`,
	}
}

func findTriagePromptFromCalls(t *testing.T, calls [][]llm.ChatMessage) string {
	t.Helper()
	for _, call := range calls {
		for _, m := range call {
			if m.Role != model.MessageRoleUser {
				continue
			}
			if strings.Contains(m.Content, "session_workspace_root:") {
				return m.Content
			}
		}
	}
	t.Fatalf("missing triage user prompt in calls: %+v", calls)
	return ""
}

func TestTriage_FollowupPromptContainsContinuityAnchors(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}

	client := &scriptedStreamClient{responses: scriptedTriageResponses()}
	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   &taskqueue.TaskRunner{},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	const sessionID = "s-followup"
	if _, err := sessions.GetOrCreateSession(sessionID, "local", secretaryModuleSU, "t"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := sessions.AppendMessage(sessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "帮我找一下 Go 项目在哪个目录"}); err != nil {
		t.Fatalf("append first msg: %v", err)
	}

	st := State{
		CursorMessageID: 1,
		TriageRuns: []TriageRun{{
			FromCursor:       0,
			ToMessageID:      1,
			SummaryMessage:   "我需要项目路径才能继续。",
			Questions:        []string{"请提供项目路径"},
			SummaryMessageID: 2,
			CreatedAt:        time.Now().UTC(),
		}},
	}
	if err := sessions.UpdateSessionMetadata(sessionID, upsertState(model.JSONB{}, st)); err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if _, err := sessions.AppendMessage(sessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "在 user home 下面找一下"}); err != nil {
		t.Fatalf("append second msg: %v", err)
	}

	if _, err := o.Triage(context.Background(), "local", sessionID, nil); err != nil {
		t.Fatalf("triage: %v", err)
	}

	prompt := findTriagePromptFromCalls(t, client.calls)
	if !strings.Contains(prompt, "CONTINUITY_CONTEXT") {
		t.Fatalf("missing continuity context marker in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "continuity_pending_questions") {
		t.Fatalf("missing pending questions marker in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "请提供项目路径") {
		t.Fatalf("missing previous question content in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "continuity_followup_hint") {
		t.Fatalf("missing followup hint in prompt: %q", prompt)
	}
}

func TestGenerateTriagePlanAsSU_ContextBudgetCapsAt80k(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	client := &scriptedStreamClient{responses: scriptedTriageResponses()}
	o := &Orchestrator{
		Sessions: sessions,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	anchors := make([]string, 0, 400)
	for i := 0; i < 400; i++ {
		anchors = append(anchors, strings.Repeat("a", 300))
	}
	carry := triageCarryContext{
		PendingQuestions: []string{"请提供项目路径"},
		LastSummary:      strings.Repeat("summary", 5000),
		SemanticAnchors:  anchors,
	}

	_, _, err = o.generateTriagePlanAsSU(context.Background(), "local", "session-1", "", "", model.JSONB{}, []model.ChatMessage{{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: "继续",
	}}, carry)
	if err != nil {
		t.Fatalf("generateTriagePlanAsSU: %v", err)
	}

	seenOmitted := false
	for _, call := range client.calls {
		total := sessioncompress.ApproximateContextRunes(call)
		if total > sessioncompress.DefaultMaxContextRunes {
			t.Fatalf("context budget exceeded: got=%d want<=%d", total, sessioncompress.DefaultMaxContextRunes)
		}
		for _, m := range call {
			if m.Role == model.MessageRoleUser && strings.Contains(m.Content, "continuity_omitted_count") {
				seenOmitted = true
			}
		}
	}
	if !seenOmitted {
		t.Fatalf("expected omitted marker in prompt after compaction")
	}
}

func TestGenerateTriagePlanAsSU_InjectsDefaultHomeSearchRoot(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	client := &scriptedStreamClient{responses: scriptedTriageResponses()}
	o := &Orchestrator{
		Sessions: sessions,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	carry := triageCarryContext{SearchPolicy: secretarySearchPolicy{DefaultReadonlySearchRoot: filepath.Clean(home)}}
	_, _, err = o.generateTriagePlanAsSU(context.Background(), "local", "session-1", "", "", model.JSONB{}, []model.ChatMessage{{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: "帮我找项目目录",
	}}, carry)
	if err != nil {
		t.Fatalf("generateTriagePlanAsSU: %v", err)
	}

	prompt := findTriagePromptFromCalls(t, client.calls)
	if !strings.Contains(prompt, "default_readonly_search_root: "+filepath.Clean(home)) {
		t.Fatalf("missing default readonly home root in prompt: %q", prompt)
	}
}

func TestExecuteLayeredReadonlySearch_ExpandsBeyondHomeWhenNeeded(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	phase2 := filepath.Join(base, "phase2")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	projectDir := filepath.Join(phase2, "repo-go")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	policy := secretarySearchPolicy{
		DefaultReadonlySearchRoot: home,
		Phase1HomeRoots:           []string{home},
		Phase2CommonDevRoots:      []string{phase2},
		TimeoutPerPhase:           2 * time.Second,
		MaxVisitedDirs:            2000,
		MaxDepth:                  4,
		MaxCandidates:             10,
		ExcludeHidden:             true,
		ExcludedDirNames:          map[string]struct{}{},
	}

	snap := executeLayeredReadonlySearch(policy, "找 Go 项目")
	if snap.FinalPhase != "common-dev" {
		t.Fatalf("expected common-dev phase, got %q (snapshot=%+v)", snap.FinalPhase, snap)
	}
	if !snap.Expanded {
		t.Fatalf("expected expanded=true when phase2 matched")
	}
	found := false
	for _, c := range snap.Candidates {
		if filepath.Clean(c) == filepath.Clean(projectDir) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected project candidate %q in %+v", projectDir, snap.Candidates)
	}
}

func TestExecuteLayeredReadonlySearch_ExcludesHiddenAndSystemDirsByDefault(t *testing.T) {
	root := t.TempDir()
	hiddenProject := filepath.Join(root, ".hidden", "repo-hidden")
	systemProject := filepath.Join(root, "System", "repo-system")
	normalProject := filepath.Join(root, "work", "repo-normal")
	for _, dir := range []string{hiddenProject, systemProject, normalProject} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
			t.Fatalf("write go.mod in %s: %v", dir, err)
		}
	}

	policy := secretarySearchPolicy{
		DefaultReadonlySearchRoot: root,
		Phase1HomeRoots:           []string{root},
		TimeoutPerPhase:           2 * time.Second,
		MaxVisitedDirs:            3000,
		MaxDepth:                  5,
		MaxCandidates:             20,
		ExcludeHidden:             true,
		ExcludedDirNames: map[string]struct{}{
			"System": {},
		},
	}

	snap := executeLayeredReadonlySearch(policy, "go 项目")
	for _, c := range snap.Candidates {
		if strings.Contains(c, ".hidden") {
			t.Fatalf("expected hidden dir excluded, got candidate %q", c)
		}
		if strings.Contains(c, string(filepath.Separator)+"System"+string(filepath.Separator)) {
			t.Fatalf("expected system dir excluded, got candidate %q", c)
		}
	}
	want := filepath.Clean(normalProject)
	found := false
	for _, c := range snap.Candidates {
		if filepath.Clean(c) == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected normal project %q in %+v", want, snap.Candidates)
	}
}
