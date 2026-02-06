package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestServer_TaskAttemptFiles_Read_ReturnsSnapshottedContent(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	workspace := t.TempDir()
	userID := "local"
	task, err := rt.Tasks.CreateTask(userID, workspace, "t", "p", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if len(task.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(task.Attempts))
	}
	attemptID := task.Attempts[0].ID

	reviewDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attemptID, "review")
	filesDir := filepath.Join(reviewDir, "files")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		t.Fatalf("mkdir files dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "changed_files.txt"), []byte("# changed_files\n\nreport.md\n"), 0o600); err != nil {
		t.Fatalf("write changed_files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "report.md"), []byte("HELLO\n"), 0o600); err != nil {
		t.Fatalf("write snapshot file: %v", err)
	}

	u := fmt.Sprintf(
		"%s/api/tasks/%s/attempts/%s/files/read?path=%s",
		srv.URL,
		url.PathEscape(task.ID),
		url.PathEscape(attemptID),
		url.QueryEscape("report.md"),
	)

	resp, data := doJSON(t, http.MethodGet, u, rt.AuthToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if strings.TrimSpace(out.Path) == "" || !strings.Contains(filepath.ToSlash(out.Path), "/review/files/report.md") {
		t.Fatalf("unexpected path: %q", out.Path)
	}
	if out.Content != "HELLO\n" {
		t.Fatalf("unexpected content: %q", out.Content)
	}
	if out.Truncated {
		t.Fatalf("expected truncated=false, got true")
	}
}

func TestServer_TaskAttemptFiles_Read_RejectsPathTraversal(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	workspace := t.TempDir()
	userID := "local"
	task, err := rt.Tasks.CreateTask(userID, workspace, "t", "p", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	attemptID := task.Attempts[0].ID
	reviewDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attemptID, "review")
	if err := os.MkdirAll(reviewDir, 0o700); err != nil {
		t.Fatalf("mkdir review dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "changed_files.txt"), []byte("# changed_files\n\nreport.md\n"), 0o600); err != nil {
		t.Fatalf("write changed_files: %v", err)
	}

	u := fmt.Sprintf(
		"%s/api/tasks/%s/attempts/%s/files/read?path=%s",
		srv.URL,
		url.PathEscape(task.ID),
		url.PathEscape(attemptID),
		url.QueryEscape("../secrets.txt"),
	)

	resp, data := doJSON(t, http.MethodGet, u, rt.AuthToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
}
