package plan

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

type UpsertTaskResult struct {
	PlanPath string `json:"plan_path"`
	TaskID   string `json:"task_id"`
	Existed  bool   `json:"existed"`
	Updated  bool   `json:"updated"`
}

type DeleteTaskResult struct {
	PlanPath string `json:"plan_path"`
	TaskID   string `json:"task_id"`
	Updated  bool   `json:"updated"`
}

func UpsertTask(root string, task Task) (UpsertTaskResult, error) {
	taskID := strings.TrimSpace(task.ID)
	if taskID == "" {
		return UpsertTaskResult{}, ErrInvalidTaskID
	}
	task.ID = taskID

	task.Title = strings.TrimSpace(task.Title)
	if task.Title == "" {
		return UpsertTaskResult{}, errors.New("title is required")
	}

	status := strings.ToLower(strings.TrimSpace(task.Status))
	if status == "" {
		status = "todo"
	}
	switch status {
	case "todo", "doing", "done":
		task.Status = status
	default:
		return UpsertTaskResult{}, fmt.Errorf("invalid status: %s", status)
	}

	parsed, planPath, raw, err := Load(root)
	if err != nil {
		return UpsertTaskResult{}, err
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	block := renderTaskBlock(task)

	if existing, ok := findTaskByID(parsed, taskID); ok {
		start := existing.LineIndex
		if start < 0 || start >= len(lines) {
			return UpsertTaskResult{}, fmt.Errorf("task line index out of range: %d", start)
		}

		end := start + 1
		for end < len(lines) {
			if taskLineRe.MatchString(lines[end]) {
				break
			}
			end++
		}

		out := make([]string, 0, len(lines)-(end-start)+len(block))
		out = append(out, lines[:start]...)
		out = append(out, block...)
		out = append(out, lines[end:]...)

		next := strings.Join(out, "\n")
		if !strings.HasSuffix(next, "\n") {
			next += "\n"
		}
		changed := next != normalized
		if changed {
			if err := atomicWriteFile(planPath, []byte(next), 0o644); err != nil {
				return UpsertTaskResult{}, err
			}
		}
		return UpsertTaskResult{
			PlanPath: planPath,
			TaskID:   taskID,
			Existed:  true,
			Updated:  changed,
		}, nil
	}

	// Append as a new task near the end.
	if !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	sep := ""
	if strings.TrimSpace(normalized) != "" && !strings.HasSuffix(normalized, "\n\n") {
		sep = "\n"
	}
	next := normalized + sep + strings.Join(block, "\n") + "\n"
	if err := atomicWriteFile(planPath, []byte(next), 0o644); err != nil {
		return UpsertTaskResult{}, err
	}
	return UpsertTaskResult{
		PlanPath: planPath,
		TaskID:   taskID,
		Existed:  false,
		Updated:  true,
	}, nil
}

func DeleteTask(root string, taskID string) (DeleteTaskResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return DeleteTaskResult{}, ErrInvalidTaskID
	}

	parsed, planPath, raw, err := Load(root)
	if err != nil {
		return DeleteTaskResult{}, err
	}
	task, ok := findTaskByID(parsed, taskID)
	if !ok {
		return DeleteTaskResult{}, ErrTaskNotFound
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	start := task.LineIndex
	if start < 0 || start >= len(lines) {
		return DeleteTaskResult{}, fmt.Errorf("task line index out of range: %d", start)
	}
	end := start + 1
	for end < len(lines) {
		if taskLineRe.MatchString(lines[end]) {
			break
		}
		end++
	}

	out := make([]string, 0, len(lines)-(end-start))
	out = append(out, lines[:start]...)
	out = append(out, lines[end:]...)

	next := strings.Join(out, "\n")
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	changed := next != normalized
	if changed {
		if err := atomicWriteFile(planPath, []byte(next), 0o644); err != nil {
			return DeleteTaskResult{}, err
		}
	}

	return DeleteTaskResult{
		PlanPath: planPath,
		TaskID:   taskID,
		Updated:  changed,
	}, nil
}

func SuggestTaskID(p Plan, title string) string {
	base := slugifyTaskID(title)
	if base == "" {
		base = "task"
	}
	seen := make(map[string]struct{}, len(p.Tasks))
	for _, t := range p.Tasks {
		id := strings.TrimSpace(t.ID)
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	if _, ok := seen[base]; !ok {
		return base
	}
	for i := 2; i < 10_000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, ok := seen[candidate]; !ok {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, len(seen)+1)
}

func slugifyTaskID(title string) string {
	var b strings.Builder
	b.Grow(len(title))

	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if lastHyphen {
			continue
		}
		b.WriteByte('-')
		lastHyphen = true
	}

	out := strings.Trim(b.String(), "-")
	out = strings.TrimSpace(out)
	return out
}

func renderTaskBlock(task Task) []string {
	mark := " "
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "done":
		mark = "x"
	case "doing":
		mark = ">"
	default:
		mark = " "
	}

	title := strings.TrimSpace(task.Title)
	id := strings.TrimSpace(task.ID)

	lines := []string{
		fmt.Sprintf("- [%s] %s <!-- id: %s -->", mark, title, id),
	}

	scopeLines := renderScopeLines(task.Scope)
	if len(scopeLines) > 0 {
		lines = append(lines, scopeLines...)
	}

	acceptLines := renderAcceptanceLines(task.Acceptance)
	if len(acceptLines) > 0 {
		lines = append(lines, acceptLines...)
	}

	return lines
}

func renderScopeLines(scope []string) []string {
	out := make([]string, 0, 2+len(scope))
	clean := make([]string, 0, len(scope))
	for _, p := range scope {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return nil
	}
	out = append(out, "  - scope:")
	for _, p := range clean {
		out = append(out, "    - "+p)
	}
	return out
}

func renderAcceptanceLines(acc Acceptance) []string {
	files := make([]string, 0, len(acc.Files))
	for _, f := range acc.Files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		files = append(files, f)
	}
	mc := make([]MustContain, 0, len(acc.MustContain))
	for _, item := range acc.MustContain {
		path := strings.TrimSpace(item.Path)
		text := strings.TrimSpace(item.Text)
		if path == "" || text == "" {
			continue
		}
		mc = append(mc, MustContain{Path: path, Text: text})
	}
	if len(files) == 0 && len(mc) == 0 {
		return nil
	}

	out := []string{"  - acceptance:"}
	if len(files) > 0 {
		out = append(out, "    - files:")
		for _, f := range files {
			out = append(out, "      - "+f)
		}
	}
	if len(mc) > 0 {
		out = append(out, "    - must_contain:")
		for _, item := range mc {
			out = append(out, fmt.Sprintf(`      - %s: %q`, item.Path, item.Text))
		}
	}
	return out
}
