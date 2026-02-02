package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	root string

	mu    sync.Mutex
	wsMux map[string]*sync.Mutex
}

func NewStore(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("workflow store root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create workflow root: %w", err)
	}
	return &Store{root: root, wsMux: map[string]*sync.Mutex{}}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) lockWorkspace(workspaceRoot string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wsMux == nil {
		s.wsMux = map[string]*sync.Mutex{}
	}
	key := workspaceKey(workspaceRoot)
	m, ok := s.wsMux[key]
	if !ok {
		m = &sync.Mutex{}
		s.wsMux[key] = m
	}
	return m
}

func (s *Store) workspaceDir(workspaceRoot string) string {
	return filepath.Join(s.root, workspaceKey(workspaceRoot))
}

func (s *Store) workflowsDir(workspaceRoot string) string {
	return filepath.Join(s.workspaceDir(workspaceRoot), "workflows")
}

func (s *Store) workflowDir(workspaceRoot string, workflowID string) string {
	return filepath.Join(s.workflowsDir(workspaceRoot), workflowID)
}

func (s *Store) workflowJSONPath(workspaceRoot string, workflowID string) string {
	return filepath.Join(s.workflowDir(workspaceRoot, workflowID), "workflow.json")
}

func (s *Store) versionsDir(workspaceRoot string, workflowID string) string {
	return filepath.Join(s.workflowDir(workspaceRoot, workflowID), "versions")
}

func (s *Store) versionJSONPath(workspaceRoot string, workflowID string, versionID string) string {
	return filepath.Join(s.versionsDir(workspaceRoot, workflowID), versionID+".json")
}

func (s *Store) runsDir(workspaceRoot string, workflowID string) string {
	return filepath.Join(s.workflowDir(workspaceRoot, workflowID), "runs")
}

func (s *Store) runDir(workspaceRoot string, workflowID string, runID string) string {
	return filepath.Join(s.runsDir(workspaceRoot, workflowID), runID)
}

func (s *Store) runJSONPath(workspaceRoot string, workflowID string, runID string) string {
	return filepath.Join(s.runDir(workspaceRoot, workflowID, runID), "run.json")
}

func (s *Store) CreateWorkflow(ctx context.Context, workspaceRoot string, name string) (Workflow, error) {
	if s == nil {
		return Workflow{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	name = strings.TrimSpace(name)
	if workspaceRoot == "" {
		return Workflow{}, errors.New("workspace_root is required")
	}
	if name == "" {
		return Workflow{}, errors.New("name is required")
	}

	_ = ctx

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().UTC()
	wf := Workflow{
		WorkflowID:    uuid.NewString(),
		WorkspaceRoot: workspaceRoot,
		Name:          name,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := os.MkdirAll(s.workflowDir(workspaceRoot, wf.WorkflowID), 0o700); err != nil {
		return Workflow{}, fmt.Errorf("create workflow dir: %w", err)
	}
	if err := writeJSONAtomic(s.workflowJSONPath(workspaceRoot, wf.WorkflowID), wf, 0o600); err != nil {
		return Workflow{}, err
	}
	return wf, nil
}

func (s *Store) RenameWorkflow(ctx context.Context, workspaceRoot string, workflowID string, name string) (Workflow, error) {
	if s == nil {
		return Workflow{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	name = strings.TrimSpace(name)
	if workspaceRoot == "" {
		return Workflow{}, errors.New("workspace_root is required")
	}
	if workflowID == "" {
		return Workflow{}, errors.New("workflow_id is required")
	}
	if name == "" {
		return Workflow{}, errors.New("name is required")
	}
	_ = ctx

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	p := s.workflowJSONPath(workspaceRoot, workflowID)
	b, err := os.ReadFile(p)
	if err != nil {
		return Workflow{}, err
	}
	var wf Workflow
	if err := json.Unmarshal(b, &wf); err != nil {
		return Workflow{}, err
	}
	wf.Name = name
	wf.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(p, wf, 0o600); err != nil {
		return Workflow{}, err
	}
	return wf, nil
}

func (s *Store) DeleteWorkflow(ctx context.Context, workspaceRoot string, workflowID string) error {
	if s == nil {
		return errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if workflowID == "" {
		return errors.New("workflow_id is required")
	}
	_ = ctx

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	return os.RemoveAll(s.workflowDir(workspaceRoot, workflowID))
}

func (s *Store) ListWorkflows(ctx context.Context, workspaceRoot string) ([]Workflow, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, errors.New("workspace_root is required")
	}

	_ = ctx

	root := s.workflowsDir(workspaceRoot)
	ents, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Workflow{}, nil
		}
		return nil, err
	}

	out := make([]Workflow, 0, len(ents))
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		id := strings.TrimSpace(ent.Name())
		if id == "" {
			continue
		}
		b, err := os.ReadFile(s.workflowJSONPath(workspaceRoot, id))
		if err != nil {
			continue
		}
		var wf Workflow
		if err := json.Unmarshal(b, &wf); err != nil {
			continue
		}
		if strings.TrimSpace(wf.WorkflowID) == "" {
			continue
		}
		out = append(out, wf)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].WorkflowID < out[j].WorkflowID
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) PublishVersion(ctx context.Context, workspaceRoot string, workflowID string, graph Graph) (WorkflowVersion, error) {
	if s == nil {
		return WorkflowVersion{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	if workspaceRoot == "" {
		return WorkflowVersion{}, errors.New("workspace_root is required")
	}
	if workflowID == "" {
		return WorkflowVersion{}, errors.New("workflow_id is required")
	}

	_ = ctx

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	// Ensure workflow exists.
	if _, err := os.Stat(s.workflowJSONPath(workspaceRoot, workflowID)); err != nil {
		return WorkflowVersion{}, err
	}

	now := time.Now().UTC()
	v := WorkflowVersion{
		VersionID:   uuid.NewString(),
		WorkflowID:  workflowID,
		Published:   true,
		PublishedAt: now,
		Graph:       graph,
		CreatedAt:   now,
	}
	if err := os.MkdirAll(s.versionsDir(workspaceRoot, workflowID), 0o700); err != nil {
		return WorkflowVersion{}, err
	}
	if err := writeJSONAtomic(s.versionJSONPath(workspaceRoot, workflowID, v.VersionID), v, 0o600); err != nil {
		return WorkflowVersion{}, err
	}
	return v, nil
}

func (s *Store) GetVersion(ctx context.Context, workspaceRoot, workflowID, versionID string) (WorkflowVersion, error) {
	if s == nil {
		return WorkflowVersion{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	versionID = strings.TrimSpace(versionID)
	if workspaceRoot == "" || workflowID == "" || versionID == "" {
		return WorkflowVersion{}, errors.New("workspace_root, workflow_id, version_id are required")
	}
	_ = ctx
	b, err := os.ReadFile(s.versionJSONPath(workspaceRoot, workflowID, versionID))
	if err != nil {
		return WorkflowVersion{}, err
	}
	var v WorkflowVersion
	if err := json.Unmarshal(b, &v); err != nil {
		return WorkflowVersion{}, err
	}
	return v, nil
}

func (s *Store) CreateRun(ctx context.Context, workspaceRoot string, workflowID string, versionID string, inputs map[string]any) (WorkflowRun, error) {
	if s == nil {
		return WorkflowRun{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	versionID = strings.TrimSpace(versionID)
	if workspaceRoot == "" {
		return WorkflowRun{}, errors.New("workspace_root is required")
	}
	if workflowID == "" {
		return WorkflowRun{}, errors.New("workflow_id is required")
	}
	if versionID == "" {
		return WorkflowRun{}, errors.New("version_id is required")
	}
	_ = ctx

	ver, err := s.GetVersion(ctx, workspaceRoot, workflowID, versionID)
	if err != nil {
		return WorkflowRun{}, err
	}

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().UTC()
	runID := uuid.NewString()

	nodeRuns := make(map[string]NodeRun, len(ver.Graph.Nodes))
	for _, n := range ver.Graph.Nodes {
		id := strings.TrimSpace(n.NodeID)
		if id == "" {
			continue
		}
		nodeRuns[id] = NodeRun{NodeID: id, Status: NodeStatusQueued}
	}

	run := WorkflowRun{
		RunID:         runID,
		WorkflowID:    workflowID,
		VersionID:     versionID,
		WorkspaceRoot: workspaceRoot,
		GraphSnapshot: ver.Graph,
		Inputs:        inputs,
		NodeRuns:      nodeRuns,
		Status:        RunStatusQueued,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := os.MkdirAll(s.runDir(workspaceRoot, workflowID, runID), 0o700); err != nil {
		return WorkflowRun{}, fmt.Errorf("create run dir: %w", err)
	}
	if err := writeJSONAtomic(s.runJSONPath(workspaceRoot, workflowID, runID), run, 0o600); err != nil {
		return WorkflowRun{}, err
	}
	return run, nil
}

func (s *Store) GetRun(ctx context.Context, workspaceRoot string, workflowID string, runID string) (WorkflowRun, error) {
	if s == nil {
		return WorkflowRun{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	runID = strings.TrimSpace(runID)
	if workspaceRoot == "" || workflowID == "" || runID == "" {
		return WorkflowRun{}, errors.New("workspace_root, workflow_id, run_id are required")
	}
	_ = ctx
	b, err := os.ReadFile(s.runJSONPath(workspaceRoot, workflowID, runID))
	if err != nil {
		return WorkflowRun{}, err
	}
	var run WorkflowRun
	if err := json.Unmarshal(b, &run); err != nil {
		return WorkflowRun{}, err
	}
	return run, nil
}

func (s *Store) UpdateRun(ctx context.Context, workspaceRoot string, workflowID string, runID string, fn func(*WorkflowRun) error) (WorkflowRun, error) {
	if s == nil {
		return WorkflowRun{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	runID = strings.TrimSpace(runID)
	if workspaceRoot == "" || workflowID == "" || runID == "" {
		return WorkflowRun{}, errors.New("workspace_root, workflow_id, run_id are required")
	}
	if fn == nil {
		return WorkflowRun{}, errors.New("update fn is required")
	}

	mu := s.lockWorkspace(workspaceRoot)
	mu.Lock()
	defer mu.Unlock()

	cur, err := s.GetRun(ctx, workspaceRoot, workflowID, runID)
	if err != nil {
		return WorkflowRun{}, err
	}
	before := cur

	if cur.NodeRuns == nil {
		cur.NodeRuns = map[string]NodeRun{}
	}

	if err := fn(&cur); err != nil {
		return WorkflowRun{}, err
	}

	now := time.Now().UTC()
	if cur.CreatedAt.IsZero() {
		cur.CreatedAt = before.CreatedAt
	}
	cur.UpdatedAt = now

	if err := writeJSONAtomic(s.runJSONPath(workspaceRoot, workflowID, runID), cur, 0o600); err != nil {
		return WorkflowRun{}, err
	}
	return cur, nil
}

func (s *Store) ListRuns(ctx context.Context, workspaceRoot string, workflowID string, limit int) ([]WorkflowRun, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	if workspaceRoot == "" || workflowID == "" {
		return nil, errors.New("workspace_root and workflow_id are required")
	}
	_ = ctx

	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	root := s.runsDir(workspaceRoot, workflowID)
	ents, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []WorkflowRun{}, nil
		}
		return nil, err
	}

	out := make([]WorkflowRun, 0, len(ents))
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		id := strings.TrimSpace(ent.Name())
		if id == "" {
			continue
		}
		r, err := s.GetRun(ctx, workspaceRoot, workflowID, id)
		if err != nil {
			continue
		}
		out = append(out, r)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].RunID < out[j].RunID
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func workspaceKey(workspaceRoot string) string {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return "no-workspace"
	}
	p := filepath.Clean(workspaceRoot)
	if resolved, err := filepath.EvalSymlinks(p); err == nil && strings.TrimSpace(resolved) != "" {
		p = filepath.Clean(resolved)
	}
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:8])
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}
	tmp := path + ".tmp." + uuid.NewString()
	if err := os.WriteFile(tmp, append(b, '\n'), mode); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// RunRoot returns the durable directory for a workflow run.
//
// Layout:
//
//	<storeRoot>/<workspaceKey>/workflows/<workflow_id>/runs/<run_id>/
func (s *Store) RunRoot(workspaceRoot, workflowID, runID string) string {
	if s == nil {
		return ""
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	runID = strings.TrimSpace(runID)
	if workspaceRoot == "" || workflowID == "" || runID == "" {
		return ""
	}
	return s.runDir(workspaceRoot, workflowID, runID)
}

// NodeRoot returns the durable directory for a node run within a workflow run.
//
// Layout:
//
//	<RunRoot>/nodes/<node_id>/
func (s *Store) NodeRoot(workspaceRoot, workflowID, runID, nodeID string) string {
	if s == nil {
		return ""
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	runID = strings.TrimSpace(runID)
	nodeID = strings.TrimSpace(nodeID)
	if workspaceRoot == "" || workflowID == "" || runID == "" || nodeID == "" {
		return ""
	}
	return filepath.Join(s.runDir(workspaceRoot, workflowID, runID), "nodes", nodeID)
}

func (s *Store) EnsureNodeRoot(workspaceRoot, workflowID, runID, nodeID string) (string, error) {
	root := s.NodeRoot(workspaceRoot, workflowID, runID, nodeID)
	if root == "" {
		return "", errors.New("node root is required")
	}
	if err := os.MkdirAll(filepath.Join(root, "deliverables"), 0o700); err != nil {
		return "", fmt.Errorf("ensure deliverables dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "gates"), 0o700); err != nil {
		return "", fmt.Errorf("ensure gates dir: %w", err)
	}
	return root, nil
}

// CleanupOldRuns removes finished workflow runs older than retentionDays.
//
// It is best-effort: individual deletion failures are ignored.
func (s *Store) CleanupOldRuns(ctx context.Context, retentionDays int, now time.Time) error {
	if s == nil {
		return errors.New("store is nil")
	}
	_ = ctx

	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	wsDirs, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, wsEnt := range wsDirs {
		if !wsEnt.IsDir() {
			continue
		}
		wsDir := filepath.Join(s.root, wsEnt.Name())
		wfsDir := filepath.Join(wsDir, "workflows")
		wfs, err := os.ReadDir(wfsDir)
		if err != nil {
			continue
		}
		for _, wfEnt := range wfs {
			if !wfEnt.IsDir() {
				continue
			}
			wfDir := filepath.Join(wfsDir, wfEnt.Name())
			runsDir := filepath.Join(wfDir, "runs")
			runs, err := os.ReadDir(runsDir)
			if err != nil {
				continue
			}
			for _, runEnt := range runs {
				if !runEnt.IsDir() {
					continue
				}
				runDir := filepath.Join(runsDir, runEnt.Name())
				runJSON := filepath.Join(runDir, "run.json")

				data, err := os.ReadFile(runJSON)
				if err != nil {
					continue
				}
				var run WorkflowRun
				if err := json.Unmarshal(data, &run); err != nil {
					continue
				}
				if run.FinishedAt.IsZero() {
					continue
				}
				if run.Status == RunStatusRunning || run.Status == RunStatusQueued {
					continue
				}
				if run.FinishedAt.Before(cutoff) {
					_ = os.RemoveAll(runDir)
				}
			}
		}
	}

	return nil
}
