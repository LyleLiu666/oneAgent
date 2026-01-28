package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type ObserveInput struct {
	TaskID    string
	AttemptID string

	WorkspaceRoot string
	Prompt        string

	Summary      string
	FindingsPath string
	TraceLogPath string
	TestReportPath string
}

type OutcomeObserver struct {
	Client llm.Client

	Model string

	MaxFindingsBytes int
	MaxTraceTailBytes int
}

func (o *OutcomeObserver) Decide(ctx context.Context, in ObserveInput) (ObserverDecision, error) {
	if o == nil {
		return ObserverDecision{}, errors.New("observer is nil")
	}
	if o.Client == nil {
		return ObserverDecision{}, errors.New("observer client is required")
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return ObserverDecision{}, errors.New("task_id is required")
	}
	if strings.TrimSpace(in.AttemptID) == "" {
		return ObserverDecision{}, errors.New("attempt_id is required")
	}
	if strings.TrimSpace(in.WorkspaceRoot) == "" {
		return ObserverDecision{}, errors.New("workspace_root is required")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return ObserverDecision{}, errors.New("prompt is required")
	}

	findingsMax := o.MaxFindingsBytes
	if findingsMax <= 0 {
		findingsMax = 128 * 1024
	}
	traceTailMax := o.MaxTraceTailBytes
	if traceTailMax <= 0 {
		traceTailMax = 32 * 1024
	}

	findingsText, findingsMeta := readFileHead(in.FindingsPath, findingsMax)
	traceTail, traceMeta := readFileTail(in.TraceLogPath, traceTailMax)
	testReportText, testReportMeta := readFileHead(in.TestReportPath, findingsMax)

	system := strings.TrimSpace(`
You are an Outcome Observer for an autonomous coding agent.

Goal: Decide if the attempt satisfies the user's original expectations.

Rules:
- You MUST be evidence-based. Prefer citing file paths and excerpts from the provided artifacts.
- You MUST NOT rely solely on the presence/absence of TODOs or a plan checklist.
- You are read-only: you cannot run commands or assume tests were executed unless evidence is provided.
- If evidence is insufficient, FAIL and explain what evidence is missing.

Output:
Return ONLY a JSON object with keys:
- pass: boolean
- reason: string (actionable; what is missing or what is done)
- evidence: array of strings (file paths / short excerpts)
`)

	var user strings.Builder
	user.WriteString("## Task\n")
	user.WriteString(fmt.Sprintf("- task_id: %s\n", in.TaskID))
	user.WriteString(fmt.Sprintf("- attempt_id: %s\n", in.AttemptID))
	user.WriteString(fmt.Sprintf("- workspace_root: %s\n", in.WorkspaceRoot))
	user.WriteString("\n### User expectation (prompt)\n")
	user.WriteString(in.Prompt)
	user.WriteString("\n")

	if strings.TrimSpace(in.Summary) != "" {
		user.WriteString("\n### Attempt summary\n")
		user.WriteString(in.Summary)
		user.WriteString("\n")
	}

	user.WriteString("\n## Artifacts\n")
	user.WriteString(fmt.Sprintf("- findings_path: %s (%s)\n", in.FindingsPath, findingsMeta))
	user.WriteString(fmt.Sprintf("- trace_log_path: %s (%s)\n", in.TraceLogPath, traceMeta))
	user.WriteString(fmt.Sprintf("- test_report_path: %s (%s)\n", in.TestReportPath, testReportMeta))

	if strings.TrimSpace(findingsText) != "" {
		user.WriteString("\n### Findings excerpt\n")
		user.WriteString("```markdown\n")
		user.WriteString(findingsText)
		if !strings.HasSuffix(findingsText, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}
	if strings.TrimSpace(traceTail) != "" {
		user.WriteString("\n### Trace tail excerpt\n")
		user.WriteString("```text\n")
		user.WriteString(traceTail)
		if !strings.HasSuffix(traceTail, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}
	if strings.TrimSpace(testReportText) != "" {
		user.WriteString("\n### Test report excerpt\n")
		user.WriteString("```markdown\n")
		user.WriteString(testReportText)
		if !strings.HasSuffix(testReportText, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}

	messages := []llm.ChatMessage{
		llm.BuildSystemMessage(system),
		llm.BuildUserMessage(user.String()),
	}

	var temp float64 = 0
	maxTokens := 600
	opts := &llm.ChatCompletionOptions{
		Model:       strings.TrimSpace(o.Model),
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	out, err := o.Client.ChatCompletion(ctx, messages, opts)
	if err != nil {
		return ObserverDecision{}, err
	}

	decision, err := parseObserverDecision(out)
	if err != nil {
		return ObserverDecision{}, err
	}
	return decision, nil
}

func parseObserverDecision(out string) (ObserverDecision, error) {
	raw := strings.TrimSpace(out)
	var d ObserverDecision
	if err := json.Unmarshal([]byte(raw), &d); err == nil {
		return d, nil
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		candidate := strings.TrimSpace(raw[start : end+1])
		if err := json.Unmarshal([]byte(candidate), &d); err == nil {
			return d, nil
		}
	}

	return ObserverDecision{}, fmt.Errorf("invalid observer output (expected JSON): %q", truncateString(raw, 300))
}

func readFileHead(path string, maxBytes int) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "missing"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "not_found"
		}
		return "", "error"
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return string(data[:maxBytes]), fmt.Sprintf("exists,size=%d,head=%d", len(data), maxBytes)
	}
	return string(data), fmt.Sprintf("exists,size=%d", len(data))
}

func readFileTail(path string, maxBytes int) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "missing"
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "not_found"
		}
		return "", "error"
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", "error"
	}
	size := info.Size()
	if size <= 0 {
		return "", "empty"
	}
	if maxBytes <= 0 || int64(maxBytes) >= size {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "error"
		}
		return string(data), fmt.Sprintf("exists,size=%d", size)
	}

	start := size - int64(maxBytes)
	if _, err := f.Seek(start, 0); err != nil {
		return "", "error"
	}
	buf := make([]byte, maxBytes)
	n, _ := f.Read(buf)
	return string(buf[:n]), fmt.Sprintf("exists,size=%d,tail=%d", size, maxBytes)
}

func truncateString(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
