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

