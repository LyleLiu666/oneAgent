package workledger

import (
	"bufio"
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
	baseDir string

	mu         sync.Mutex
	receiptMux map[string]*sync.Mutex
	suggestionMux map[string]*sync.Mutex
}

func NewStore(baseDir string) (*Store, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, errors.New("baseDir is required")
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create baseDir: %w", err)
	}
	return &Store{
		baseDir:    baseDir,
		receiptMux: make(map[string]*sync.Mutex),
		suggestionMux: make(map[string]*sync.Mutex),
	}, nil
}

func (s *Store) ReceiptsDir() string {
	return filepath.Join(s.baseDir, "receipts")
}

func (s *Store) SuggestionsDir() string {
	return filepath.Join(s.baseDir, "sop_suggestions")
}

func (s *Store) receiptDir(receiptID string) string {
	return filepath.Join(s.ReceiptsDir(), receiptID)
}

func (s *Store) receiptJSONPath(receiptID string) string {
	return filepath.Join(s.receiptDir(receiptID), "receipt.json")
}

func (s *Store) receiptMDPath(receiptID string) string {
	return filepath.Join(s.receiptDir(receiptID), "receipt.md")
}

func (s *Store) receiptLock(receiptID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.receiptMux == nil {
		s.receiptMux = make(map[string]*sync.Mutex)
	}
	m, ok := s.receiptMux[receiptID]
	if ok {
		return m
	}
	m = &sync.Mutex{}
	s.receiptMux[receiptID] = m
	return m
}

func (s *Store) suggestionLock(suggestionID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.suggestionMux == nil {
		s.suggestionMux = make(map[string]*sync.Mutex)
	}
	m, ok := s.suggestionMux[suggestionID]
	if ok {
		return m
	}
	m = &sync.Mutex{}
	s.suggestionMux[suggestionID] = m
	return m
}

type CreateReceiptInput struct {
	PrincipalID   string
	WorkspaceRoot string

	Kind   ReceiptKind
	Status ReceiptStatus

	StartedAt  time.Time
	FinishedAt time.Time

	Summary   string
	Artifacts ReceiptArtifacts
	Signals   ReceiptSignals
}

func (s *Store) CreateReceipt(in CreateReceiptInput) (Receipt, error) {
	if s == nil {
		return Receipt{}, errors.New("store is nil")
	}
	if strings.TrimSpace(in.PrincipalID) == "" {
		return Receipt{}, errors.New("principal_id is required")
	}
	if strings.TrimSpace(in.Summary) == "" {
		return Receipt{}, errors.New("summary is required")
	}
	if strings.TrimSpace(string(in.Kind)) == "" {
		return Receipt{}, errors.New("kind is required")
	}
	if strings.TrimSpace(string(in.Status)) == "" {
		return Receipt{}, errors.New("status is required")
	}

	id := uuid.NewString()
	now := time.Now()
	started := in.StartedAt
	if started.IsZero() {
		started = now
	}
	finished := in.FinishedAt
	if finished.IsZero() {
		finished = now
	}
	if finished.Before(started) {
		finished = started
	}

	r := Receipt{
		ReceiptID:     id,
		PrincipalID:   strings.TrimSpace(in.PrincipalID),
		WorkspaceRoot: strings.TrimSpace(in.WorkspaceRoot),
		Kind:          in.Kind,
		Status:        in.Status,
		StartedAt:     started.UTC(),
		FinishedAt:    finished.UTC(),
		Summary:       strings.TrimSpace(in.Summary),
		Artifacts:     in.Artifacts,
		Signals:       in.Signals,
	}

	mu := s.receiptLock(id)
	mu.Lock()
	defer mu.Unlock()

	dir := s.receiptDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Receipt{}, fmt.Errorf("create receipt dir: %w", err)
	}

	jsonPath := s.receiptJSONPath(id)
	if err := writeJSONAtomic(jsonPath, r, 0o600); err != nil {
		return Receipt{}, err
	}

	mdPath := s.receiptMDPath(id)
	if err := writeTextAtomic(mdPath, buildReceiptMarkdown(r), 0o644); err != nil {
		return Receipt{}, err
	}

	return r, nil
}

func (s *Store) GetReceipt(receiptID string) (Receipt, error) {
	if s == nil {
		return Receipt{}, errors.New("store is nil")
	}
	receiptID = strings.TrimSpace(receiptID)
	if receiptID == "" {
		return Receipt{}, errors.New("receipt_id is required")
	}

	mu := s.receiptLock(receiptID)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(s.receiptJSONPath(receiptID))
	if err != nil {
		return Receipt{}, err
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return Receipt{}, fmt.Errorf("decode receipt: %w", err)
	}
	return r, nil
}

type ListReceiptsQuery struct {
	PrincipalID string
	Workspace   string
	Status      ReceiptStatus
	Q           string

	FinishedAfter  time.Time
	FinishedBefore time.Time

	Limit int
}

func (s *Store) ListReceipts(q ListReceiptsQuery) ([]Receipt, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	if strings.TrimSpace(q.PrincipalID) == "" {
		return nil, errors.New("principal_id is required")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	receiptsDir := s.ReceiptsDir()
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Receipt{}, nil
		}
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(q.Q))
	workspace := strings.TrimSpace(q.Workspace)
	status := strings.TrimSpace(string(q.Status))
	after := q.FinishedAfter
	before := q.FinishedBefore

	out := make([]Receipt, 0, minInt(limit, len(entries)))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()

		data, err := os.ReadFile(s.receiptJSONPath(id))
		if err != nil {
			continue
		}
		var r Receipt
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		if strings.TrimSpace(r.PrincipalID) != strings.TrimSpace(q.PrincipalID) {
			continue
		}
		if workspace != "" && strings.TrimSpace(r.WorkspaceRoot) != workspace {
			continue
		}
		if status != "" && strings.TrimSpace(string(r.Status)) != status {
			continue
		}
		if !after.IsZero() {
			if r.FinishedAt.IsZero() || r.FinishedAt.Before(after) {
				continue
			}
		}
		if !before.IsZero() {
			if r.FinishedAt.IsZero() || !r.FinishedAt.Before(before) {
				continue
			}
		}
		if needle != "" {
			if !receiptMatchesQuery(r, needle, s.receiptMDPath(id)) {
				continue
			}
		}
		out = append(out, r)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FinishedAt.After(out[j].FinishedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func receiptMatchesQuery(r Receipt, needle string, mdPath string) bool {
	if strings.Contains(strings.ToLower(r.Summary), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(string(r.Kind)), needle) {
		return true
	}

	if mdPath != "" {
		f, err := os.Open(mdPath)
		if err != nil {
			return false
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.Contains(strings.ToLower(scanner.Text()), needle) {
				return true
			}
		}
	}
	return false
}

func buildReceiptMarkdown(r Receipt) string {
	var b strings.Builder
	b.WriteString("# Receipt\n\n")
	b.WriteString("- receipt_id: ")
	b.WriteString(r.ReceiptID)
	b.WriteString("\n- kind: ")
	b.WriteString(string(r.Kind))
	b.WriteString("\n- status: ")
	b.WriteString(string(r.Status))
	b.WriteString("\n- principal_id: ")
	b.WriteString(r.PrincipalID)
	b.WriteString("\n")
	if strings.TrimSpace(r.WorkspaceRoot) != "" {
		b.WriteString("- workspace_root: ")
		b.WriteString(r.WorkspaceRoot)
		b.WriteString("\n")
	}
	b.WriteString("- started_at: ")
	b.WriteString(r.StartedAt.Format(time.RFC3339))
	b.WriteString("\n- finished_at: ")
	b.WriteString(r.FinishedAt.Format(time.RFC3339))
	b.WriteString("\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString(strings.TrimSpace(r.Summary))
	b.WriteString("\n\n")

	if strings.TrimSpace(r.Artifacts.FindingsPath) != "" || strings.TrimSpace(r.Artifacts.TraceLogPath) != "" || strings.TrimSpace(r.Artifacts.TestReportPath) != "" || strings.TrimSpace(r.Artifacts.DiffRef) != "" {
		b.WriteString("## Artifacts\n\n")
		if strings.TrimSpace(r.Artifacts.FindingsPath) != "" {
			b.WriteString("- findings_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.FindingsPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.TraceLogPath) != "" {
			b.WriteString("- trace_log_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.TraceLogPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.TestReportPath) != "" {
			b.WriteString("- test_report_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.TestReportPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.DiffRef) != "" {
			b.WriteString("- diff_ref: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.DiffRef))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if r.Signals.DurationMs > 0 || r.Signals.TotalTokens > 0 || r.Signals.Calls > 0 || r.Signals.CostUSD > 0 {
		b.WriteString("## Signals\n\n")
		if r.Signals.DurationMs > 0 {
			b.WriteString("- duration_ms: ")
			b.WriteString(fmt.Sprintf("%d", r.Signals.DurationMs))
			b.WriteString("\n")
		}
		if r.Signals.Calls > 0 {
			b.WriteString("- calls: ")
			b.WriteString(fmt.Sprintf("%d", r.Signals.Calls))
			b.WriteString("\n")
		}
		if r.Signals.PromptTokens > 0 {
			b.WriteString("- prompt_tokens: ")
			b.WriteString(fmt.Sprintf("%d", r.Signals.PromptTokens))
			b.WriteString("\n")
		}
		if r.Signals.CompletionTokens > 0 {
			b.WriteString("- completion_tokens: ")
			b.WriteString(fmt.Sprintf("%d", r.Signals.CompletionTokens))
			b.WriteString("\n")
		}
		if r.Signals.TotalTokens > 0 {
			b.WriteString("- total_tokens: ")
			b.WriteString(fmt.Sprintf("%d", r.Signals.TotalTokens))
			b.WriteString("\n")
		}
		if r.Signals.CostUSD > 0 {
			b.WriteString("- cost_usd: ")
			b.WriteString(fmt.Sprintf("%.4f", r.Signals.CostUSD))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return writeTextAtomic(path, string(b)+"\n", mode)
}

func writeTextAtomic(path string, content string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}
	tmp := path + ".tmp." + uuid.NewString()
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
