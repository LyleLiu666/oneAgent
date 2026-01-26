package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/scope"
)

const (
	defaultPlanDir  = ".oneagent"
	defaultPlanFile = "PLAN.md"
)

var (
	ErrPlanNotFound   = errors.New("plan file not found")
	ErrTaskNotFound   = errors.New("task not found")
	ErrInvalidTaskID  = errors.New("task_id is required")
	ErrEmptyAcceptance = errors.New("acceptance criteria is empty")
)

func DefaultPlanPath(root string) string {
	return filepath.Join(root, defaultPlanDir, defaultPlanFile)
}

type Plan struct {
	Tasks []Task `json:"tasks"`
}

type Task struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"` // todo|doing|done (doing reserved)
	Scope      []string   `json:"scope,omitempty"`
	Acceptance Acceptance `json:"acceptance,omitempty"`

	LineIndex int `json:"-"`
}

type Acceptance struct {
	Files       []string      `json:"files,omitempty"`
	MustContain []MustContain `json:"must_contain,omitempty"`
}

type MustContain struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type MarkDoneResult struct {
	PlanPath string `json:"plan_path"`
	TaskID   string `json:"task_id"`
	Pass     bool   `json:"pass"`
	Updated  bool   `json:"updated"`
	Message  string `json:"message,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func EnsurePlanFile(root string, template string, overwrite bool) (string, bool, error) {
	if strings.TrimSpace(root) == "" {
		return "", false, errors.New("root is required")
	}

	planPath := DefaultPlanPath(root)
	if _, err := os.Stat(planPath); err == nil && !overwrite {
		return planPath, true, nil
	}

	dir := filepath.Dir(planPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("create plan directory: %w", err)
	}

	content := strings.TrimSpace(template)
	if content == "" {
		content = strings.TrimSpace(DefaultTemplate)
	}
	content = content + "\n"

	if err := atomicWriteFile(planPath, []byte(content), 0o644); err != nil {
		return "", false, err
	}
	return planPath, false, nil
}

func Load(root string) (Plan, string, string, error) {
	if strings.TrimSpace(root) == "" {
		return Plan{}, "", "", errors.New("root is required")
	}

	planPath := DefaultPlanPath(root)
	data, err := os.ReadFile(planPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Plan{}, planPath, "", ErrPlanNotFound
		}
		return Plan{}, planPath, "", fmt.Errorf("read plan: %w", err)
	}
	raw := string(data)
	parsed := Parse(raw)
	return parsed, planPath, raw, nil
}

func MarkDone(root string, taskID string) (MarkDoneResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return MarkDoneResult{}, ErrInvalidTaskID
	}

	parsed, planPath, raw, err := Load(root)
	if err != nil {
		return MarkDoneResult{}, err
	}

	task, ok := findTaskByID(parsed, taskID)
	if !ok {
		return MarkDoneResult{}, ErrTaskNotFound
	}

	if strings.EqualFold(task.Status, "done") {
		return MarkDoneResult{
			PlanPath: planPath,
			TaskID:   taskID,
			Pass:     true,
			Updated:  false,
			Message:  "task already done",
		}, nil
	}

	pass, reason, err := ValidateTask(root, task)
	if err != nil {
		return MarkDoneResult{}, err
	}
	if !pass {
		msg := fmt.Sprintf("【plan中某个任务标记done失败】 %s", reason)
		return MarkDoneResult{
			PlanPath: planPath,
			TaskID:   taskID,
			Pass:     false,
			Updated:  false,
			Message:  msg,
			Reason:   reason,
		}, nil
	}

	updatedContent, changed, err := SetTaskDone(raw, taskID, true)
	if err != nil {
		return MarkDoneResult{}, err
	}
	if !changed {
		return MarkDoneResult{
			PlanPath: planPath,
			TaskID:   taskID,
			Pass:     true,
			Updated:  false,
		}, nil
	}

	if err := atomicWriteFile(planPath, []byte(updatedContent), 0o644); err != nil {
		return MarkDoneResult{}, err
	}

	return MarkDoneResult{
		PlanPath: planPath,
		TaskID:   taskID,
		Pass:     true,
		Updated:  true,
	}, nil
}

func ValidateTask(root string, task Task) (bool, string, error) {
	if len(task.Acceptance.Files) == 0 && len(task.Acceptance.MustContain) == 0 {
		return false, ErrEmptyAcceptance.Error(), nil
	}

	root = strings.TrimSpace(root)
	if root == "" {
		return false, "", errors.New("root is required")
	}

	for _, rel := range task.Acceptance.Files {
		path := strings.TrimSpace(rel)
		if path == "" {
			continue
		}
		abs, err := scope.ResolveWritePath(root, path, task.Scope)
		if err != nil {
			return false, fmt.Sprintf("invalid acceptance file path %q: %v", path, err), nil
		}
		info, statErr := os.Stat(abs)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return false, fmt.Sprintf("missing required file: %s", path), nil
			}
			return false, "", fmt.Errorf("stat %s: %w", abs, statErr)
		}
		if info.IsDir() {
			return false, fmt.Sprintf("expected file but got directory: %s", path), nil
		}
	}

	for _, mc := range task.Acceptance.MustContain {
		path := strings.TrimSpace(mc.Path)
		if path == "" {
			continue
		}
		text := mc.Text
		if strings.TrimSpace(text) == "" {
			continue
		}

		abs, err := scope.ResolveWritePath(root, path, task.Scope)
		if err != nil {
			return false, fmt.Sprintf("invalid must_contain path %q: %v", path, err), nil
		}
		ok, err := fileContains(abs, text, 2*1024*1024)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, fmt.Sprintf("missing required file: %s", path), nil
			}
			return false, "", err
		}
		if !ok {
			return false, fmt.Sprintf("file %s does not contain %q", path, text), nil
		}
	}

	return true, "", nil
}

