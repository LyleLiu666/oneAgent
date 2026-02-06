package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHead2HeadRunner_Mock_Smoke3(t *testing.T) {
	repoRoot := findRepoRootByDataset(t)
	datasetPath := filepath.Join(repoRoot, "benchmarks", "datasets", "head2head_mvp.json")

	outDir := filepath.Join(t.TempDir(), "bench")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir outdir: %v", err)
	}

	report, err := RunHead2HeadBenchmark(context.Background(), Head2HeadRunOptions{
		RepoRoot:    repoRoot,
		DatasetPath: datasetPath,
		OutDir:      outDir,
		Limit:       3,
	})
	if err != nil {
		t.Fatalf("RunHead2HeadBenchmark: %v", err)
	}
	if report.Summary.TotalCases != 3 {
		t.Fatalf("expected total_cases=3, got %d", report.Summary.TotalCases)
	}
	if report.Summary.FirstPassSuccesses != 3 {
		for _, c := range report.Cases {
			t.Logf("case=%s attempt1_status=%s observer_reason=%q", c.CaseID, c.Attempt1.Status, c.Attempt1.ObserverReason)
		}
		t.Fatalf("expected first_pass_successes=3, got %d", report.Summary.FirstPassSuccesses)
	}
	if report.Summary.FirstPassSuccessRate != 1.0 {
		t.Fatalf("expected first_pass_success_rate=1.0, got %.3f", report.Summary.FirstPassSuccessRate)
	}

	if _, err := os.Stat(filepath.Join(outDir, "report.json")); err != nil {
		t.Fatalf("missing report.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "report.md")); err != nil {
		t.Fatalf("missing report.md: %v", err)
	}
}

func TestHead2HeadRunner_Mock_RecoveryResume(t *testing.T) {
	repoRoot := findRepoRootByDataset(t)
	datasetPath := filepath.Join(repoRoot, "benchmarks", "datasets", "head2head_mvp.json")

	outDir := filepath.Join(t.TempDir(), "bench")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir outdir: %v", err)
	}

	report, err := RunHead2HeadBenchmark(context.Background(), Head2HeadRunOptions{
		RepoRoot:    repoRoot,
		DatasetPath: datasetPath,
		OutDir:      outDir,
		CaseIDs:     []string{"014_coding_recovery_wrong_then_fix"},
	})
	if err != nil {
		t.Fatalf("RunHead2HeadBenchmark: %v", err)
	}
	if report.Summary.TotalCases != 1 {
		t.Fatalf("expected total_cases=1, got %d", report.Summary.TotalCases)
	}
	if report.Summary.FirstPassSuccesses != 0 {
		t.Fatalf("expected first_pass_successes=0, got %d", report.Summary.FirstPassSuccesses)
	}
	if report.Summary.RecoveryAttempts != 1 || report.Summary.RecoverySuccesses != 1 {
		t.Fatalf("expected recovery 1/1, got %d/%d", report.Summary.RecoverySuccesses, report.Summary.RecoveryAttempts)
	}
	if report.Summary.RecoverySuccessRate != 1.0 {
		t.Fatalf("expected recovery_success_rate=1.0, got %.3f", report.Summary.RecoverySuccessRate)
	}
	if report.Summary.HumanInterventionCount != 1 {
		t.Fatalf("expected human_intervention_count=1, got %d", report.Summary.HumanInterventionCount)
	}
	if len(report.Cases) != 1 || report.Cases[0].Attempt2 == nil {
		t.Fatalf("expected attempt2 to be present: %+v", report.Cases)
	}
	if !report.Cases[0].Recovered || report.Cases[0].Interventions != 1 {
		t.Fatalf("expected recovered=true interventions=1, got recovered=%t interventions=%d", report.Cases[0].Recovered, report.Cases[0].Interventions)
	}

	if _, err := os.Stat(filepath.Join(outDir, "report.json")); err != nil {
		t.Fatalf("missing report.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "report.md")); err != nil {
		t.Fatalf("missing report.md: %v", err)
	}
}

