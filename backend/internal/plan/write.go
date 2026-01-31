package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var checkboxLineRe = regexp.MustCompile(`^(\s*-\s*\[)([xX>\s])(\].*)$`)

func SetTaskDone(content string, taskID string, done bool) (string, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", false, ErrInvalidTaskID
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	parsed := Parse(normalized)

	task, ok := findTaskByID(parsed, taskID)
	if !ok {
		return "", false, ErrTaskNotFound
	}

	if task.LineIndex < 0 || task.LineIndex >= len(lines) {
		return "", false, fmt.Errorf("task line index out of range: %d", task.LineIndex)
	}

	line := lines[task.LineIndex]
	updated, changed, err := setCheckbox(line, done)
	if err != nil {
		return "", false, err
	}
	if changed {
		lines[task.LineIndex] = updated
	}

	return strings.Join(lines, "\n"), changed, nil
}

func setCheckbox(line string, done bool) (string, bool, error) {
	match := checkboxLineRe.FindStringSubmatch(line)
	if match == nil {
		return "", false, fmt.Errorf("invalid task line: %q", line)
	}

	want := " "
	if done {
		want = "x"
	}

	current := match[2]
	if strings.EqualFold(current, want) {
		return line, false, nil
	}

	return match[1] + want + match[3], true, nil
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("rename temp file: %w", err)
		}
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
