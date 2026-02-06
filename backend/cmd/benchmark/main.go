package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/benchmark"
)

func main() {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	repoRoot := fs.String("repo-root", "..", "repo root (default: .. when running from backend/)")
	dataset := fs.String("dataset", "../benchmarks/datasets/head2head_mvp.json", "dataset path")
	outDir := fs.String("out", "", "output directory (default: <repo>/.oneagent/tmp/benchmarks/<run_id>/)")
	baseline := fs.String("baseline", "", "baseline report.json path (best-effort)")
	limit := fs.Int("limit", 0, "limit number of cases (0 = all)")
	maxFirstPassDrop := fs.Float64("max-first-pass-drop", 0, "fail if first_pass_success_rate drops more than this vs baseline (requires --baseline; 0 disables)")
	maxRecoveryDrop := fs.Float64("max-recovery-drop", 0, "fail if recovery_success_rate drops more than this vs baseline (requires --baseline; 0 disables)")
	maxEvidenceDrop := fs.Float64("max-evidence-drop", 0, "fail if evidence_completeness_avg drops more than this vs baseline (requires --baseline; 0 disables)")

	_ = fs.Parse(os.Args[1:])

	repo := strings.TrimSpace(*repoRoot)
	if repo != "" {
		if abs, err := filepath.Abs(repo); err == nil {
			repo = abs
		}
	}
	dsPath := strings.TrimSpace(*dataset)
	if dsPath != "" {
		if abs, err := filepath.Abs(dsPath); err == nil {
			dsPath = abs
		}
	}

	report, err := benchmark.RunHead2HeadBenchmark(context.Background(), benchmark.Head2HeadRunOptions{
		RepoRoot:     repo,
		DatasetPath:  dsPath,
		OutDir:       strings.TrimSpace(*outDir),
		BaselinePath: strings.TrimSpace(*baseline),
		Limit:        *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s\n", report.RunID)
	fmt.Printf("run_root=%s\n", report.RunRoot)
	fmt.Printf("total_cases=%d first_pass_success_rate=%.3f recovery_success_rate=%.3f evidence_avg=%.3f interventions=%d\n",
		report.Summary.TotalCases,
		report.Summary.FirstPassSuccessRate,
		report.Summary.RecoverySuccessRate,
		report.Summary.EvidenceCompletenessAvg,
		report.Summary.HumanInterventionCount,
	)
	fmt.Printf("report_json=%s\n", filepath.Join(report.RunRoot, "report.json"))
	fmt.Printf("report_md=%s\n", filepath.Join(report.RunRoot, "report.md"))

	if report.Baseline != nil {
		if *maxFirstPassDrop > 0 && report.Baseline.FirstPassSuccessRateDelta < -*maxFirstPassDrop {
			fmt.Fprintf(os.Stderr, "REGRESSION: first_pass_success_rate_delta=%+.3f < -%.3f\n", report.Baseline.FirstPassSuccessRateDelta, *maxFirstPassDrop)
			os.Exit(2)
		}
		if *maxRecoveryDrop > 0 && report.Baseline.RecoverySuccessRateDelta < -*maxRecoveryDrop {
			fmt.Fprintf(os.Stderr, "REGRESSION: recovery_success_rate_delta=%+.3f < -%.3f\n", report.Baseline.RecoverySuccessRateDelta, *maxRecoveryDrop)
			os.Exit(2)
		}
		if *maxEvidenceDrop > 0 && report.Baseline.EvidenceCompletenessAvgDelta < -*maxEvidenceDrop {
			fmt.Fprintf(os.Stderr, "REGRESSION: evidence_completeness_avg_delta=%+.3f < -%.3f\n", report.Baseline.EvidenceCompletenessAvgDelta, *maxEvidenceDrop)
			os.Exit(2)
		}
	}
}
