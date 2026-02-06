package taskqueue

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ArtifactManifestVersionV1 = "v1"

type ArtifactUnavailable struct {
	Code string `json:"code"`
	Hint string `json:"hint,omitempty"`
}

type ArtifactManifestV1 struct {
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`

	TaskID        string        `json:"task_id"`
	AttemptID     string        `json:"attempt_id"`
	AttemptStatus AttemptStatus `json:"attempt_status"`

	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`

	FindingsPath        string               `json:"findings_path"`
	FindingsUnavailable *ArtifactUnavailable `json:"findings_unavailable,omitempty"`

	TraceLogPath        string               `json:"trace_log_path"`
	TraceLogUnavailable *ArtifactUnavailable `json:"trace_log_unavailable,omitempty"`

	TestReportPath        string               `json:"test_report_path"`
	TestReportUnavailable *ArtifactUnavailable `json:"test_report_unavailable,omitempty"`

	ChangedFilesPath        string               `json:"changed_files_path"`
	ChangedFilesUnavailable *ArtifactUnavailable `json:"changed_files_unavailable,omitempty"`

	DiffPatchPath        string               `json:"diff_patch_path"`
	DiffPatchUnavailable *ArtifactUnavailable `json:"diff_patch_unavailable,omitempty"`

	ReviewCommentsPath        string               `json:"review_comments_path"`
	ReviewCommentsUnavailable *ArtifactUnavailable `json:"review_comments_unavailable,omitempty"`
}

type changedFilesReportMeta struct {
	GitWorkspace bool
	Note         string
	HasNone      bool
	Ok           bool
}

func readChangedFilesReportMeta(path string) changedFilesReportMeta {
	path = strings.TrimSpace(path)
	if path == "" {
		return changedFilesReportMeta{}
	}
	f, err := os.Open(path)
	if err != nil {
		return changedFilesReportMeta{}
	}
	defer f.Close()

	var meta changedFilesReportMeta
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# git_workspace:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "# git_workspace:"))
			meta.GitWorkspace = raw == "true"
			meta.Ok = true
			continue
		}
		if strings.HasPrefix(line, "# note:") {
			meta.Note = strings.TrimSpace(strings.TrimPrefix(line, "# note:"))
			meta.Ok = true
			continue
		}
		if line == "- (none)" {
			meta.HasNone = true
			meta.Ok = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return changedFilesReportMeta{}
	}
	return meta
}

func BuildArtifactManifestV1(tasksDir string, task Task, attempt Attempt) ArtifactManifestV1 {
	now := Now().UTC()

	manifest := ArtifactManifestV1{
		Version:        ArtifactManifestVersionV1,
		GeneratedAt:    now,
		TaskID:         strings.TrimSpace(task.ID),
		AttemptID:      strings.TrimSpace(attempt.ID),
		AttemptStatus:  attempt.Status,
		Summary:        strings.TrimSpace(attempt.Summary),
		Error:          strings.TrimSpace(attempt.Error),
		FindingsPath:   strings.TrimSpace(attempt.FindingsPath),
		TraceLogPath:   strings.TrimSpace(attempt.TraceLogPath),
		TestReportPath: strings.TrimSpace(attempt.TestReportPath),
	}

	reviewDir := ""
	if strings.TrimSpace(tasksDir) != "" && strings.TrimSpace(task.ID) != "" && strings.TrimSpace(attempt.ID) != "" {
		reviewDir = filepath.Join(tasksDir, task.ID, "attempts", attempt.ID, "review")
	}

	changedFilesPath := strings.TrimSpace(attempt.ChangedFilesPath)
	if changedFilesPath == "" && reviewDir != "" {
		changedFilesPath = filepath.Join(reviewDir, "changed_files.txt")
	}
	manifest.ChangedFilesPath = changedFilesPath

	diffPatchPath := strings.TrimSpace(attempt.DiffPatchPath)
	if diffPatchPath == "" && reviewDir != "" {
		diffPatchPath = filepath.Join(reviewDir, "diff.patch")
	}
	manifest.DiffPatchPath = diffPatchPath

	reviewCommentsPath := strings.TrimSpace(attempt.ReviewCommentsPath)
	if reviewCommentsPath == "" && reviewDir != "" {
		reviewCommentsPath = filepath.Join(reviewDir, "review_comments.jsonl")
	}
	manifest.ReviewCommentsPath = reviewCommentsPath

	// Findings.
	if strings.TrimSpace(manifest.FindingsPath) == "" {
		manifest.FindingsUnavailable = &ArtifactUnavailable{
			Code: "missing",
			Hint: "findings_path is empty",
		}
	} else if !fileExists(manifest.FindingsPath) {
		manifest.FindingsUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("findings_path not found: %s", manifest.FindingsPath),
		}
	}

	// Trace.
	if strings.TrimSpace(manifest.TraceLogPath) == "" {
		manifest.TraceLogUnavailable = &ArtifactUnavailable{
			Code: "missing",
			Hint: "trace_log_path is empty",
		}
	} else if !fileExists(manifest.TraceLogPath) {
		manifest.TraceLogUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("trace_log_path not found: %s", manifest.TraceLogPath),
		}
	}

	// Test report (best-effort).
	if strings.TrimSpace(manifest.TestReportPath) == "" {
		code := "not_generated"
		hint := "test_report_path is empty"
		if strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_TEST_REPORT")) == "1" {
			code = "disabled"
			hint = "test report generation disabled (ONEAGENT_DISABLE_TEST_REPORT=1)"
		} else if strings.TrimSpace(task.Workspace) != "" {
			ws := strings.TrimSpace(task.Workspace)
			if !fileExists(filepath.Join(ws, "go.mod")) &&
				!fileExists(filepath.Join(ws, "package.json")) &&
				!fileExists(filepath.Join(ws, "pyproject.toml")) &&
				!fileExists(filepath.Join(ws, "requirements.txt")) {
				code = "no_tests_detected"
				hint = "no test runner detected (go.mod/package.json/pyproject.toml/requirements.txt)"
			}
		}
		manifest.TestReportUnavailable = &ArtifactUnavailable{Code: code, Hint: hint}
	} else if !fileExists(manifest.TestReportPath) {
		manifest.TestReportUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("test_report_path not found: %s", manifest.TestReportPath),
		}
	}

	// Changed files.
	changedMeta := changedFilesReportMeta{}
	if strings.TrimSpace(manifest.ChangedFilesPath) == "" {
		manifest.ChangedFilesUnavailable = &ArtifactUnavailable{
			Code: "missing",
			Hint: "changed_files_path is empty",
		}
	} else if !fileExists(manifest.ChangedFilesPath) {
		manifest.ChangedFilesUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("changed_files_path not found: %s", manifest.ChangedFilesPath),
		}
	} else {
		changedMeta = readChangedFilesReportMeta(manifest.ChangedFilesPath)
	}

	// Diff patch.
	if strings.TrimSpace(manifest.DiffPatchPath) == "" {
		manifest.DiffPatchUnavailable = &ArtifactUnavailable{
			Code: "missing",
			Hint: "diff_patch_path is empty",
		}
	} else if fileExists(manifest.DiffPatchPath) {
		// ok
	} else if changedMeta.Ok {
		code := "diff_unavailable"
		hint := "diff patch not available"
		note := strings.ToLower(strings.TrimSpace(changedMeta.Note))
		switch {
		case !changedMeta.GitWorkspace:
			code = "non_git_workspace"
			hint = "workspace is not a git repository; diff.patch unavailable"
		case changedMeta.HasNone || strings.Contains(note, "no changes detected") || strings.Contains(note, "empty patch"):
			code = "no_changes_detected"
			hint = "no changes detected; diff.patch omitted"
		case strings.Contains(note, "patch too large"):
			code = "patch_too_large"
			hint = "diff patch omitted due to size; review changed_files"
		case strings.Contains(note, "git not found"):
			code = "git_not_found"
			hint = "git not found; install git to enable diff.patch"
		case strings.HasPrefix(note, "git diff omitted:"):
			code = "git_diff_error"
			hint = strings.TrimSpace(changedMeta.Note)
		case note != "":
			hint = strings.TrimSpace(changedMeta.Note)
		}
		manifest.DiffPatchUnavailable = &ArtifactUnavailable{Code: code, Hint: hint}
	} else {
		manifest.DiffPatchUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("diff_patch_path not found: %s", manifest.DiffPatchPath),
		}
	}

	// Review comments.
	if strings.TrimSpace(manifest.ReviewCommentsPath) == "" {
		manifest.ReviewCommentsUnavailable = &ArtifactUnavailable{
			Code: "missing",
			Hint: "review_comments_path is empty",
		}
	} else if !fileExists(manifest.ReviewCommentsPath) {
		manifest.ReviewCommentsUnavailable = &ArtifactUnavailable{
			Code: "file_missing",
			Hint: fmt.Sprintf("review_comments_path not found: %s", manifest.ReviewCommentsPath),
		}
	}

	return manifest
}

func artifactManifestV1Path(tasksDir, taskID, attemptID string) string {
	tasksDir = strings.TrimSpace(tasksDir)
	taskID = strings.TrimSpace(taskID)
	attemptID = strings.TrimSpace(attemptID)
	if tasksDir == "" || taskID == "" || attemptID == "" {
		return ""
	}
	return filepath.Join(tasksDir, taskID, "attempts", attemptID, "artifact_manifest.v1.json")
}

func EnsureArtifactManifestV1(tasksDir string, task Task, attempt *Attempt) error {
	if attempt == nil {
		return fmt.Errorf("attempt is nil")
	}
	path := artifactManifestV1Path(tasksDir, task.ID, attempt.ID)
	if path == "" {
		return fmt.Errorf("manifest path not available")
	}
	manifest := BuildArtifactManifestV1(tasksDir, task, *attempt)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir manifest dir: %w", err)
	}
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	attempt.ArtifactManifestVersion = ArtifactManifestVersionV1
	attempt.ArtifactManifestPath = path

	// Normalize stable review artifact pointers when they exist on disk (back-compat for older attempts).
	reviewDir := filepath.Join(strings.TrimSpace(tasksDir), strings.TrimSpace(task.ID), "attempts", strings.TrimSpace(attempt.ID), "review")
	defaultChangedFiles := filepath.Join(reviewDir, "changed_files.txt")
	defaultDiffPatch := filepath.Join(reviewDir, "diff.patch")
	defaultReviewComments := filepath.Join(reviewDir, "review_comments.jsonl")

	if strings.TrimSpace(attempt.ChangedFilesPath) == "" && fileExists(defaultChangedFiles) {
		attempt.ChangedFilesPath = defaultChangedFiles
	}
	if strings.TrimSpace(attempt.DiffPatchPath) == "" && fileExists(defaultDiffPatch) {
		attempt.DiffPatchPath = defaultDiffPatch
	}
	if strings.TrimSpace(attempt.ReviewCommentsPath) == "" && fileExists(defaultReviewComments) {
		attempt.ReviewCommentsPath = defaultReviewComments
	}

	return nil
}
