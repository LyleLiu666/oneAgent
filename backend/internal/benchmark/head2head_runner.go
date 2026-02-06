package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/server"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type Head2HeadRunOptions struct {
	RepoRoot     string
	DatasetPath  string
	OutDir       string
	BaselinePath string
	Limit        int
	CaseIDs      []string
}

type Head2HeadReport struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	Mode          string    `json:"mode"`
	DatasetID     string    `json:"dataset_id"`
	DatasetVersion int      `json:"dataset_version"`
	GeneratedAt   time.Time `json:"generated_at"`

	RepoRoot string `json:"repo_root,omitempty"`
	RunRoot  string `json:"run_root"`

	Summary Head2HeadSummary     `json:"summary"`
	Cases   []Head2HeadCaseResult `json:"cases"`

	Baseline *Head2HeadBaselineDelta `json:"baseline,omitempty"`
}

type Head2HeadSummary struct {
	TotalCases int `json:"total_cases"`

	FirstPassSuccesses   int     `json:"first_pass_successes"`
	FirstPassSuccessRate float64 `json:"first_pass_success_rate"`

	RecoveryAttempts   int     `json:"recovery_attempts"`
	RecoverySuccesses  int     `json:"recovery_successes"`
	RecoverySuccessRate float64 `json:"recovery_success_rate"`

	HumanInterventionCount int     `json:"human_intervention_count"`
	EvidenceCompletenessAvg float64 `json:"evidence_completeness_avg"`

	CostPerSuccessfulDeliveryUSD float64 `json:"cost_per_successful_delivery_usd"`
}

type Head2HeadBaselineDelta struct {
	BaselineRunID string `json:"baseline_run_id,omitempty"`

	FirstPassSuccessRateDelta       float64 `json:"first_pass_success_rate_delta"`
	RecoverySuccessRateDelta        float64 `json:"recovery_success_rate_delta"`
	HumanInterventionCountDelta     int     `json:"human_intervention_count_delta"`
	EvidenceCompletenessAvgDelta    float64 `json:"evidence_completeness_avg_delta"`
	CostPerSuccessfulDeliveryUSDDelta float64 `json:"cost_per_successful_delivery_usd_delta"`
}

type Head2HeadAttemptResult struct {
	AttemptID string `json:"attempt_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`

	ObserverPass   *bool  `json:"observer_pass,omitempty"`
	ObserverReason string `json:"observer_reason,omitempty"`

	CheckpointPath     string `json:"checkpoint_path,omitempty"`
	FindingsPath       string `json:"findings_path,omitempty"`
	TraceLogPath       string `json:"trace_log_path,omitempty"`
	TestReportPath     string `json:"test_report_path,omitempty"`
	DiffPatchPath      string `json:"diff_patch_path,omitempty"`
	ChangedFilesPath   string `json:"changed_files_path,omitempty"`
	ReviewCommentsPath string `json:"review_comments_path,omitempty"`
}

type Head2HeadCaseResult struct {
	CaseID  string   `json:"case_id"`
	Title   string   `json:"title"`
	Labels  []string `json:"labels,omitempty"`
	TaskID  string   `json:"task_id"`
	WorkspaceRoot string `json:"workspace_root"`

	FirstPassSuccess bool `json:"first_pass_success"`
	Recovered        bool `json:"recovered"`
	Interventions    int  `json:"interventions"`

	EvidenceRequired        []string `json:"evidence_required,omitempty"`
	EvidenceCompleteness    float64  `json:"evidence_completeness"`
	MissingEvidence         []string `json:"missing_evidence,omitempty"`

	Attempt1 Head2HeadAttemptResult  `json:"attempt_1"`
	Attempt2 *Head2HeadAttemptResult `json:"attempt_2,omitempty"`
}

func RunHead2HeadBenchmark(ctx context.Context, opts Head2HeadRunOptions) (Head2HeadReport, error) {
	ds, err := LoadHead2HeadDataset(opts.DatasetPath)
	if err != nil {
		return Head2HeadReport{}, err
	}

	selected, err := selectCases(ds.Cases, opts.Limit, opts.CaseIDs)
	if err != nil {
		return Head2HeadReport{}, err
	}

	runID := uuid.NewString()
	runRoot := strings.TrimSpace(opts.OutDir)
	if runRoot == "" {
		repoRoot := strings.TrimSpace(opts.RepoRoot)
		if repoRoot == "" {
			return Head2HeadReport{}, errors.New("repo_root is required when out_dir is empty")
		}
		runRoot = filepath.Join(repoRoot, ".oneagent", "tmp", "benchmarks", runID)
	}
	if err := os.MkdirAll(runRoot, 0o700); err != nil {
		return Head2HeadReport{}, err
	}

	cfg, err := config.Load(config.LoadOptions{
		Home:     runRoot,
		Profile:  "dev",
		AuthMode: "none",
	})
	if err != nil {
		return Head2HeadReport{}, err
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		return Head2HeadReport{}, err
	}
	defer func() { _ = rt.Close() }()

	userID := "local"
	providerID := "bench-provider-" + runID
	modelID := "bench-model-" + runID
	if _, err := rt.Settings.CreateProvider(ctx, settingsdb.Provider{
		ID:           providerID,
		UserID:       userID,
		Name:         "benchmark-mock",
		ProviderType: llm.ProviderTypeOpenAIResponse,
		BaseURL:      "http://127.0.0.1.invalid",
		APIKey:       "sk-benchmark",
	}); err != nil {
		return Head2HeadReport{}, err
	}
	if _, err := rt.Settings.CreateModel(ctx, settingsdb.Model{
		ID:         modelID,
		ProviderID: providerID,
		UserID:     userID,
		Name:       "benchmark-mock",
		Model:      "gpt-benchmark",
		IsDefault:  true,
	}); err != nil {
		return Head2HeadReport{}, err
	}

	if err := server.EnsureTaskQueue(rt); err != nil {
		return Head2HeadReport{}, err
	}

	caseByTaskID := make(map[string]Head2HeadCase, len(selected))
	rt.TaskRunner.DecideOutcome = buildObjectiveDecider(caseByTaskID)

	report := Head2HeadReport{
		SchemaVersion:  1,
		RunID:          runID,
		Mode:           "mock",
		DatasetID:      ds.ID,
		DatasetVersion: ds.Version,
		GeneratedAt:    time.Now().UTC(),
		RepoRoot:       strings.TrimSpace(opts.RepoRoot),
		RunRoot:        runRoot,
		Cases:          make([]Head2HeadCaseResult, 0, len(selected)),
	}

	var (
		firstPassOK int
		recoveryAttempts int
		recoveryOK int
		interventions int
		evidenceSum float64
		successCostSum float64
		successCount int
	)

	workspacesRoot := filepath.Join(runRoot, "workspaces")
	_ = os.MkdirAll(workspacesRoot, 0o700)

	for _, c := range selected {

		workspaceRoot := filepath.Join(workspacesRoot, c.ID)
		if err := prepareWorkspace(workspaceRoot, c.Workspace); err != nil {
			return Head2HeadReport{}, fmt.Errorf("prepare workspace %s: %w", c.ID, err)
		}
		if c.Workspace.GitInit {
			if err := initGitWorkspace(ctx, workspaceRoot); err != nil {
				return Head2HeadReport{}, fmt.Errorf("init git workspace %s: %w", c.ID, err)
			}
		}

		attemptIdx := 0
		mock := StartMockOpenAIResponsesServer(selectAttemptResponses(c, attemptIdx))
		if mock == nil {
			return Head2HeadReport{}, fmt.Errorf("mock server not started for case %s", c.ID)
		}
		if _, err := rt.Settings.UpdateProvider(ctx, userID, providerID, map[string]any{
			"base_url": mock.URL(),
		}); err != nil {
			mock.Close()
			return Head2HeadReport{}, err
		}

		task, err := rt.Tasks.CreateTask(userID, workspaceRoot, c.Title, c.Prompt, modelID, taskqueue.Limits{})
		if err != nil {
			mock.Close()
			return Head2HeadReport{}, err
		}
		caseByTaskID[task.ID] = c

		if err := rt.TaskRunner.Enqueue(task.ID); err != nil {
			mock.Close()
			return Head2HeadReport{}, err
		}

		updated, err := waitForTerminalAttempt(rt.Tasks, task.ID, 60*time.Second)
		mock.Close()
		if err != nil {
			return Head2HeadReport{}, err
		}

		a1 := updated.LatestAttempt()
		if a1 == nil {
			return Head2HeadReport{}, fmt.Errorf("missing attempt for task %s", task.ID)
		}

		cr := Head2HeadCaseResult{
			CaseID:        c.ID,
			Title:         c.Title,
			Labels:        append([]string(nil), c.Labels...),
			TaskID:        task.ID,
			WorkspaceRoot: workspaceRoot,
			EvidenceRequired: append([]string(nil), c.EvidenceRequired...),
			Attempt1:      toAttemptResult(*a1),
		}

		firstPass := a1.Status == taskqueue.AttemptSucceeded
		cr.FirstPassSuccess = firstPass
		if firstPass {
			firstPassOK++
		}

		// Best-effort recovery: resume once when mock provides a second attempt script.
		var attempt2 *Head2HeadAttemptResult
		recovered := false
		if !firstPass && len(c.Mock.Attempts) > 1 {
			recoveryAttempts++
			interventions++
			attemptIdx = 1
			mock2 := StartMockOpenAIResponsesServer(selectAttemptResponses(c, attemptIdx))
			if _, err := rt.Settings.UpdateProvider(ctx, userID, providerID, map[string]any{
				"base_url": mock2.URL(),
			}); err != nil {
				mock2.Close()
				return Head2HeadReport{}, err
			}
			if _, err := rt.TaskRunner.Resume(task.ID, "benchmark resume"); err != nil {
				mock2.Close()
				return Head2HeadReport{}, err
			}
			updated2, err := waitForTerminalAttempt(rt.Tasks, task.ID, 60*time.Second)
			mock2.Close()
			if err != nil {
				return Head2HeadReport{}, err
			}
			a2 := updated2.LatestAttempt()
			if a2 == nil || a2.ResumedFromAttemptID == "" {
				return Head2HeadReport{}, fmt.Errorf("expected resumed attempt for task %s", task.ID)
			}
			ar := toAttemptResult(*a2)
			attempt2 = &ar
			if a2.Status == taskqueue.AttemptSucceeded {
				recovered = true
				recoveryOK++
			}
		}
		cr.Attempt2 = attempt2
		cr.Recovered = recovered
		cr.Interventions = 0
		if attempt2 != nil {
			cr.Interventions = 1
		}

		// Evidence completeness is measured on the final attempt (attempt2 if present).
		finalAttempt := a1
		if attempt2 != nil {
			cur, _ := rt.Tasks.GetTask(task.ID)
			finalAttempt = cur.LatestAttempt()
		}
		score, missing := computeEvidenceCompleteness(c.EvidenceRequired, finalAttempt)
		cr.EvidenceCompleteness = score
		cr.MissingEvidence = missing
		evidenceSum += score

		if finalAttempt != nil && finalAttempt.Status == taskqueue.AttemptSucceeded && finalAttempt.Usage != nil {
			successCostSum += finalAttempt.Usage.CostUSD
		}
		if finalAttempt != nil && finalAttempt.Status == taskqueue.AttemptSucceeded {
			successCount++
		}

		report.Cases = append(report.Cases, cr)
	}

	report.Summary.TotalCases = len(report.Cases)
	report.Summary.FirstPassSuccesses = firstPassOK
	if report.Summary.TotalCases > 0 {
		report.Summary.FirstPassSuccessRate = float64(firstPassOK) / float64(report.Summary.TotalCases)
		report.Summary.EvidenceCompletenessAvg = evidenceSum / float64(report.Summary.TotalCases)
	}
	report.Summary.RecoveryAttempts = recoveryAttempts
	report.Summary.RecoverySuccesses = recoveryOK
	if recoveryAttempts > 0 {
		report.Summary.RecoverySuccessRate = float64(recoveryOK) / float64(recoveryAttempts)
	}
	report.Summary.HumanInterventionCount = interventions
	if successCount > 0 {
		report.Summary.CostPerSuccessfulDeliveryUSD = successCostSum / float64(successCount)
	}

	if strings.TrimSpace(opts.BaselinePath) != "" {
		if base, err := loadReport(opts.BaselinePath); err == nil {
			report.Baseline = &Head2HeadBaselineDelta{
				BaselineRunID: base.RunID,
				FirstPassSuccessRateDelta: report.Summary.FirstPassSuccessRate - base.Summary.FirstPassSuccessRate,
				RecoverySuccessRateDelta: report.Summary.RecoverySuccessRate - base.Summary.RecoverySuccessRate,
				HumanInterventionCountDelta: report.Summary.HumanInterventionCount - base.Summary.HumanInterventionCount,
				EvidenceCompletenessAvgDelta: report.Summary.EvidenceCompletenessAvg - base.Summary.EvidenceCompletenessAvg,
				CostPerSuccessfulDeliveryUSDDelta: report.Summary.CostPerSuccessfulDeliveryUSD - base.Summary.CostPerSuccessfulDeliveryUSD,
			}
		}
	}

	if err := writeReportFiles(runRoot, report); err != nil {
		return Head2HeadReport{}, err
	}
	return report, nil
}

func selectAttemptResponses(c Head2HeadCase, attemptIndex int) []MockResponse {
	if attemptIndex < 0 {
		attemptIndex = 0
	}
	if len(c.Mock.Attempts) == 0 {
		return nil
	}
	if attemptIndex >= len(c.Mock.Attempts) {
		attemptIndex = len(c.Mock.Attempts) - 1
	}
	return c.Mock.Attempts[attemptIndex].Responses
}

func selectCases(all []Head2HeadCase, limit int, caseIDs []string) ([]Head2HeadCase, error) {
	if len(all) == 0 {
		return nil, errors.New("dataset has no cases")
	}

	if len(caseIDs) > 0 {
		want := make(map[string]struct{}, len(caseIDs))
		for _, id := range caseIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			want[id] = struct{}{}
		}
		if len(want) == 0 {
			return nil, errors.New("case_ids is empty after trimming")
		}
		out := make([]Head2HeadCase, 0, len(want))
		for _, c := range all {
			if _, ok := want[c.ID]; !ok {
				continue
			}
			out = append(out, c)
			delete(want, c.ID)
		}
		if len(want) > 0 {
			missing := make([]string, 0, len(want))
			for id := range want {
				missing = append(missing, id)
			}
			return nil, fmt.Errorf("unknown case_ids: %s", strings.Join(missing, ", "))
		}
		return out, nil
	}

	if limit <= 0 || limit > len(all) {
		limit = len(all)
	}
	out := make([]Head2HeadCase, 0, limit)
	out = append(out, all[:limit]...)
	return out, nil
}

func prepareWorkspace(root string, ws Head2HeadWorkspace) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("workspace root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	for _, sf := range ws.SeedFiles {
		p := filepath.Join(root, filepath.FromSlash(filepath.Clean(sf.Path)))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(sf.Content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func initGitWorkspace(ctx context.Context, root string) error {
	// Ensure git is installed (best-effort).
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "benchmark@example.invalid"},
		{"git", "config", "user.name", "Benchmark"},
		{"git", "add", "-A"},
		{"git", "commit", "-m", "seed"},
	}
	for _, args := range cmds {
		c := exec.CommandContext(ctx, args[0], args[1:]...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func waitForTerminalAttempt(store *taskqueue.Store, taskID string, timeout time.Duration) (taskqueue.Task, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := store.GetTask(taskID)
		if err != nil {
			return taskqueue.Task{}, err
		}
		a := task.LatestAttempt()
		if a != nil {
			switch a.Status {
			case taskqueue.AttemptSucceeded, taskqueue.AttemptFailed, taskqueue.AttemptCanceled, taskqueue.AttemptTimedOut, taskqueue.AttemptInterrupted, taskqueue.AttemptLimitExceeded:
				return task, nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	task, _ := store.GetTask(taskID)
	return taskqueue.Task{}, fmt.Errorf("timeout waiting for terminal attempt for task=%s latest=%+v", taskID, task.LatestAttempt())
}

func buildObjectiveDecider(caseByTaskID map[string]Head2HeadCase) func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
	return func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
		_ = ctx
		c, ok := caseByTaskID[task.ID]
		if !ok {
			return taskqueue.ObserverDecision{Pass: true, Reason: "no_case_config"}, nil
		}

		failures := validateAcceptance(c, task.Workspace, attempt)
		if len(failures) == 0 {
			return taskqueue.ObserverDecision{Pass: true, Reason: "ok"}, nil
		}
		return taskqueue.ObserverDecision{
			Pass:   false,
			Reason: "acceptance_failed: " + strings.Join(failures, "; "),
		}, nil
	}
}

func validateAcceptance(c Head2HeadCase, workspaceRoot string, attempt taskqueue.Attempt) []string {
	out := make([]string, 0, 8)
	workspaceRoot = strings.TrimSpace(workspaceRoot)

	readWorkspaceFile := func(rel string) (string, error) {
		p := filepath.Join(workspaceRoot, filepath.FromSlash(filepath.Clean(rel)))
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	for _, r := range c.Acceptance {
		switch strings.TrimSpace(r.Type) {
		case "file_exists":
			p := filepath.Join(workspaceRoot, filepath.FromSlash(filepath.Clean(r.Path)))
			if st, err := os.Stat(p); err != nil || st.IsDir() {
				out = append(out, "file_exists:"+r.Path)
			}
		case "file_not_exists":
			p := filepath.Join(workspaceRoot, filepath.FromSlash(filepath.Clean(r.Path)))
			if _, err := os.Stat(p); err == nil {
				out = append(out, "file_not_exists:"+r.Path)
			}
		case "file_contains":
			text, err := readWorkspaceFile(r.Path)
			if err != nil || !strings.Contains(text, r.Contains) {
				out = append(out, "file_contains:"+r.Path)
			}
		case "file_not_contains":
			text, err := readWorkspaceFile(r.Path)
			if err != nil || strings.Contains(text, r.Contains) {
				out = append(out, "file_not_contains:"+r.Path)
			}
		case "file_equals":
			text, err := readWorkspaceFile(r.Path)
			if err != nil || text != r.Equals {
				out = append(out, "file_equals:"+r.Path)
			}
		case "json_valid":
			text, err := readWorkspaceFile(r.Path)
			if err != nil {
				out = append(out, "json_valid:"+r.Path)
				continue
			}
			var v any
			if err := json.Unmarshal([]byte(text), &v); err != nil {
				out = append(out, "json_valid:"+r.Path)
			}
		case "changed_files_contains":
			if strings.TrimSpace(attempt.ChangedFilesPath) == "" {
				out = append(out, "changed_files_contains:"+r.Path)
				continue
			}
			b, err := os.ReadFile(attempt.ChangedFilesPath)
			if err != nil {
				out = append(out, "changed_files_contains:"+r.Path)
				continue
			}
			if !changedFilesHasPath(string(b), strings.TrimSpace(r.Path)) {
				out = append(out, "changed_files_contains:"+r.Path)
			}
		default:
			out = append(out, "unknown_rule:"+strings.TrimSpace(r.Type))
		}
	}
	return out
}

func changedFilesHasPath(report string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	lines := strings.Split(report, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		if line == want {
			return true
		}
	}
	return false
}

func toAttemptResult(a taskqueue.Attempt) Head2HeadAttemptResult {
	out := Head2HeadAttemptResult{
		AttemptID: strings.TrimSpace(a.ID),
		Status:    string(a.Status),
		Error:     strings.TrimSpace(a.Error),

		CheckpointPath:     strings.TrimSpace(a.CheckpointPath),
		FindingsPath:       strings.TrimSpace(a.FindingsPath),
		TraceLogPath:       strings.TrimSpace(a.TraceLogPath),
		TestReportPath:     strings.TrimSpace(a.TestReportPath),
		DiffPatchPath:      strings.TrimSpace(a.DiffPatchPath),
		ChangedFilesPath:   strings.TrimSpace(a.ChangedFilesPath),
		ReviewCommentsPath: strings.TrimSpace(a.ReviewCommentsPath),
	}
	if a.Observer != nil {
		pass := a.Observer.Pass
		out.ObserverPass = &pass
		out.ObserverReason = strings.TrimSpace(a.Observer.Reason)
	}
	return out
}

func computeEvidenceCompleteness(required []string, attempt *taskqueue.Attempt) (float64, []string) {
	if len(required) == 0 {
		return 1.0, nil
	}
	if attempt == nil {
		return 0, append([]string(nil), required...)
	}
	missing := make([]string, 0, len(required))
	for _, k := range required {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		path := evidencePathForKind(k, *attempt)
		if !fileExists(path) {
			missing = append(missing, k)
		}
	}
	score := 1.0
	if len(required) > 0 {
		score = float64(len(required)-len(missing)) / float64(len(required))
	}
	return score, missing
}

func evidencePathForKind(kind string, a taskqueue.Attempt) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "checkpoint":
		return strings.TrimSpace(a.CheckpointPath)
	case "findings":
		return strings.TrimSpace(a.FindingsPath)
	case "trace":
		return strings.TrimSpace(a.TraceLogPath)
	case "test_report":
		return strings.TrimSpace(a.TestReportPath)
	case "diff_patch":
		return strings.TrimSpace(a.DiffPatchPath)
	case "changed_files":
		return strings.TrimSpace(a.ChangedFilesPath)
	case "review_comments":
		return strings.TrimSpace(a.ReviewCommentsPath)
	default:
		return ""
	}
}

func fileExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func writeReportFiles(runRoot string, report Head2HeadReport) error {
	jsonPath := filepath.Join(runRoot, "report.json")
	mdPath := filepath.Join(runRoot, "report.md")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(mdPath, []byte(buildMarkdownReport(report)), 0o600); err != nil {
		return err
	}
	return nil
}

func buildMarkdownReport(r Head2HeadReport) string {
	var b strings.Builder
	b.WriteString("# Head-to-Head Benchmark Report\n\n")
	b.WriteString(fmt.Sprintf("- run_id: `%s`\n", r.RunID))
	b.WriteString(fmt.Sprintf("- dataset: `%s` v%d\n", r.DatasetID, r.DatasetVersion))
	b.WriteString(fmt.Sprintf("- generated_at: `%s`\n", r.GeneratedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- run_root: `%s`\n", r.RunRoot))
	if strings.TrimSpace(r.RepoRoot) != "" {
		b.WriteString(fmt.Sprintf("- repo_root: `%s`\n", r.RepoRoot))
	}
	b.WriteString("\n## Summary\n\n")
	b.WriteString(fmt.Sprintf("- total_cases: %d\n", r.Summary.TotalCases))
	b.WriteString(fmt.Sprintf("- first_pass_success_rate: %.3f (%d/%d)\n", r.Summary.FirstPassSuccessRate, r.Summary.FirstPassSuccesses, r.Summary.TotalCases))
	if r.Summary.RecoveryAttempts > 0 {
		b.WriteString(fmt.Sprintf("- recovery_success_rate: %.3f (%d/%d)\n", r.Summary.RecoverySuccessRate, r.Summary.RecoverySuccesses, r.Summary.RecoveryAttempts))
	} else {
		b.WriteString("- recovery_success_rate: n/a (no recovery attempts)\n")
	}
	b.WriteString(fmt.Sprintf("- human_intervention_count: %d\n", r.Summary.HumanInterventionCount))
	b.WriteString(fmt.Sprintf("- evidence_completeness_avg: %.3f\n", r.Summary.EvidenceCompletenessAvg))
	b.WriteString(fmt.Sprintf("- cost_per_successful_delivery_usd: %.4f\n", r.Summary.CostPerSuccessfulDeliveryUSD))

	if r.Baseline != nil {
		b.WriteString("\n## Baseline Delta (best-effort)\n\n")
		if strings.TrimSpace(r.Baseline.BaselineRunID) != "" {
			b.WriteString(fmt.Sprintf("- baseline_run_id: `%s`\n", r.Baseline.BaselineRunID))
		}
		b.WriteString(fmt.Sprintf("- first_pass_success_rate_delta: %+0.3f\n", r.Baseline.FirstPassSuccessRateDelta))
		b.WriteString(fmt.Sprintf("- recovery_success_rate_delta: %+0.3f\n", r.Baseline.RecoverySuccessRateDelta))
		b.WriteString(fmt.Sprintf("- human_intervention_count_delta: %+d\n", r.Baseline.HumanInterventionCountDelta))
		b.WriteString(fmt.Sprintf("- evidence_completeness_avg_delta: %+0.3f\n", r.Baseline.EvidenceCompletenessAvgDelta))
		b.WriteString(fmt.Sprintf("- cost_per_successful_delivery_usd_delta: %+0.4f\n", r.Baseline.CostPerSuccessfulDeliveryUSDDelta))
	}

	b.WriteString("\n## Cases\n\n")
	for _, c := range r.Cases {
		b.WriteString(fmt.Sprintf("### %s\n\n", c.CaseID))
		b.WriteString(fmt.Sprintf("- title: %s\n", c.Title))
		b.WriteString(fmt.Sprintf("- status: %s\n", c.Attempt1.Status))
		if c.Attempt2 != nil {
			b.WriteString(fmt.Sprintf("- resume_status: %s\n", c.Attempt2.Status))
		}
		b.WriteString(fmt.Sprintf("- first_pass_success: %t\n", c.FirstPassSuccess))
		b.WriteString(fmt.Sprintf("- recovered: %t\n", c.Recovered))
		b.WriteString(fmt.Sprintf("- interventions: %d\n", c.Interventions))
		b.WriteString(fmt.Sprintf("- evidence_completeness: %.3f\n", c.EvidenceCompleteness))
		if len(c.MissingEvidence) > 0 {
			b.WriteString(fmt.Sprintf("- missing_evidence: %s\n", strings.Join(c.MissingEvidence, ", ")))
		}
		if c.Attempt1.ObserverPass != nil {
			b.WriteString(fmt.Sprintf("- observer_pass: %t\n", *c.Attempt1.ObserverPass))
		}
		if strings.TrimSpace(c.Attempt1.ObserverReason) != "" {
			b.WriteString(fmt.Sprintf("- observer_reason: %s\n", c.Attempt1.ObserverReason))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func loadReport(path string) (Head2HeadReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Head2HeadReport{}, err
	}
	var r Head2HeadReport
	if err := json.Unmarshal(data, &r); err != nil {
		return Head2HeadReport{}, err
	}
	if r.SchemaVersion <= 0 || strings.TrimSpace(r.RunID) == "" {
		return Head2HeadReport{}, errors.New("invalid baseline report")
	}
	return r, nil
}
