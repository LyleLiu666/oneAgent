package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_WorkflowsAPI_CreatePublishRunExecute(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
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

	workspace := "/tmp/ws-demo"

	// Create workflow.
	body := bytes.NewReader([]byte(`{"workspace_root":"` + workspace + `","name":"Demo"}`))
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/workflows", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/workflows: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/workflows status=%d", res.StatusCode)
	}
	var wf map[string]any
	_ = json.NewDecoder(res.Body).Decode(&wf)
	workflowID, _ := wf["workflow_id"].(string)
	if workflowID == "" {
		t.Fatalf("expected workflow_id, got %+v", wf)
	}

	// Publish version.
	publishPayload := map[string]any{
		"workspace_root": workspace,
		"graph": map[string]any{
			"nodes": []map[string]any{{"node_id": "a", "title": "A"}},
			"edges": []map[string]any{},
		},
	}
	b, _ := json.Marshal(publishPayload)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/workflows/"+workflowID+"/publish", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST publish: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST publish status=%d", res.StatusCode)
	}
	var ver map[string]any
	_ = json.NewDecoder(res.Body).Decode(&ver)
	versionID, _ := ver["version_id"].(string)
	if versionID == "" {
		t.Fatalf("expected version_id, got %+v", ver)
	}

	// Create run.
	createRunPayload := map[string]any{
		"workspace_root": workspace,
		"version_id":     versionID,
	}
	b, _ = json.Marshal(createRunPayload)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/workflows/"+workflowID+"/runs", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST create run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST create run status=%d", res.StatusCode)
	}
	var run map[string]any
	_ = json.NewDecoder(res.Body).Decode(&run)
	runID, _ := run["run_id"].(string)
	if runID == "" {
		t.Fatalf("expected run_id, got %+v", run)
	}

	// Execute run (noop).
	execPayload := map[string]any{
		"workspace_root": workspace,
		"concurrency":    1,
	}
	b, _ = json.Marshal(execPayload)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/workflows/"+workflowID+"/runs/"+runID+"/execute", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST execute: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST execute status=%d", res.StatusCode)
	}
	var executed map[string]any
	_ = json.NewDecoder(res.Body).Decode(&executed)
	if executed["status"] != "succeeded" {
		t.Fatalf("expected succeeded, got %+v", executed)
	}

	// Get run.
	res, err = http.Get(srv.URL + "/api/workflows/" + workflowID + "/runs/" + runID + "?workspace=" + workspace)
	if err != nil {
		t.Fatalf("GET run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET run status=%d", res.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(res.Body).Decode(&got)
	if got["status"] != "succeeded" {
		t.Fatalf("expected succeeded, got %+v", got)
	}
}

